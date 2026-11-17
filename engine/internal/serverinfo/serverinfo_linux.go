//go:build linux

// Package serverinfo discovers the local VPS — Drevanta manages the machine it
// runs on, so there is no add-server workflow; facts come from /proc and the
// runtime binaries.
package serverinfo

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"drevanta/engine/internal/model"
)

// Docker client over the unix socket (implemented in the runtime package).
type DockerProbe interface {
	Ping() error
	Version() (string, error)
}

// CaddyProbe checks the admin API (implemented in the proxy package).
type CaddyProbe interface {
	Reachable() bool
}

var (
	dockerProbe DockerProbe
	caddyProbe  CaddyProbe
	probeMu     sync.Mutex
)

// SetProbes lets main inject the concrete runtime implementations.
func SetProbes(d DockerProbe, c CaddyProbe) {
	probeMu.Lock()
	defer probeMu.Unlock()
	dockerProbe, caddyProbe = d, c
}

func Discover(version string) model.ServerInfo {
	info := model.ServerInfo{
		Hostname:      "unknown",
		OS:            "unknown",
		Kernel:        "unknown",
		Arch:          runtime.GOARCH,
		CPUCores:      runtime.NumCPU(),
		IP:            "127.0.0.1",
		EngineVersion: version,
	}
	if h, err := os.Hostname(); err == nil {
		info.Hostname = h
	}
	if pretty := osReleaseField("PRETTY_NAME"); pretty != "" {
		info.OS = pretty
	}
	if id := strings.ToLower(osReleaseField("ID")); id != "" {
		info.OS = info.OS + " (" + id + ")"
	}
	info.Kernel = readFirstLine("/proc/sys/kernel/osrelease")
	info.UptimeS = readUptime()
	info.MemTotal = readMemTotal()
	if st, err := statfs("/"); err == nil {
		info.DiskTotal = st.Blocks * uint64(st.Bsize)
		info.DiskUsed = (st.Blocks - st.Bavail) * uint64(st.Bsize)
	}
	if ip := primaryIP(); ip != "" {
		info.IP = ip
	}

	probeMu.Lock()
	d := dockerProbe
	c := caddyProbe
	probeMu.Unlock()

	if d != nil {
		if err := d.Ping(); err == nil {
			v, _ := d.Version()
			info.Docker = model.RuntimeInfo{Installed: true, Running: true, Version: v}
		}
	}
	if c != nil && c.Reachable() {
		v := caddyBinaryVersion()
		info.Caddy = model.RuntimeInfo{Installed: true, Running: true, Version: v}
	} else if path := lookPathCaddy(); path != "" {
		info.Caddy = model.RuntimeInfo{Installed: true, Running: false, Version: caddyBinaryVersion()}
	}
	info.MockMode = !info.Docker.Installed
	return info
}

// Readiness returns the M01 checklist: is this VPS ready to host projects.
func Readiness(info model.ServerInfo, dataDir string) []model.ReadinessCheck {
	checks := []model.ReadinessCheck{}
	id := strings.ToLower(osReleaseField("ID"))
	okOS := id == "ubuntu" || id == "debian" || id == "raspbian"
	checks = append(checks, model.ReadinessCheck{
		ID: "os", Label: "Operating system", OK: okOS,
		Detail: fmt.Sprintf("%s — Ubuntu/Debian supported", info.OS),
	})
	checks = append(checks, model.ReadinessCheck{
		ID: "docker", Label: "Docker Engine", OK: info.Docker.Installed && info.Docker.Running,
		Detail: boolDetail(info.Docker.Installed, info.Docker.Running,
			"socket responding", "install docker.io or docker-ce"),
	})
	checks = append(checks, model.ReadinessCheck{
		ID: "caddy", Label: "Caddy reverse proxy", OK: info.Caddy.Installed && info.Caddy.Running,
		Detail: boolDetail(info.Caddy.Installed, info.Caddy.Running,
			"admin API on 127.0.0.1:2019", "installed, start the caddy service"),
	})
	for _, p := range []int{80, 443} {
		free := portFree(p)
		checks = append(checks, model.ReadinessCheck{
			ID:    fmt.Sprintf("port-%d", p),
			Label: fmt.Sprintf("Port %d available", p),
			OK:    free,
			Detail: map[bool]string{true: "free", false: "in use — Caddy must own it"}[free],
		})
	}
	writable := dirWritable(dataDir)
	checks = append(checks, model.ReadinessCheck{
		ID: "datadir", Label: "State directory writable", OK: writable,
		Detail: dataDir,
	})
	return checks
}

/* ---------- helpers ---------- */

func boolDetail(installed, running bool, okDetail, fix string) string {
	switch {
	case installed && running:
		return okDetail
	case installed:
		return fix
	default:
		return "not installed — " + fix
	}
}

func osReleaseField(key string) string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.SplitN(sc.Text(), "=", 2)
		if len(parts) == 2 && parts[0] == key {
			return strings.Trim(parts[1], `"`)
		}
	}
	return ""
}

func readFirstLine(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if sc.Scan() {
		return strings.TrimSpace(sc.Text())
	}
	return ""
}

func readUptime() int64 {
	line := readFirstLine("/proc/uptime")
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return 0
	}
	v, _ := strconv.ParseFloat(fields[0], 64)
	return int64(v)
}

func readMemTotal() uint64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.Fields(sc.Text())
		if len(parts) >= 2 && parts[0] == "MemTotal:" {
			kb, _ := strconv.ParseUint(parts[1], 10, 64)
			return kb * 1024
		}
	}
	return 0
}

type fsStat struct {
	Blocks uint64
	Bsize  int64
	Bavail uint64
}

func statfs(path string) (fsStat, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return fsStat{}, err
	}
	return fsStat{Blocks: uint64(st.Blocks), Bsize: int64(st.Bsize), Bavail: uint64(st.Bavail)}, nil
}

func primaryIP() string {
	conn, err := net.DialTimeout("udp", "8.8.8.8:80", 300*time.Millisecond)
	if err != nil {
		return ""
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

func portFree(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

func dirWritable(dir string) bool {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}
	probe := filepath.Join(dir, ".drevanta-write-probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		return false
	}
	os.Remove(probe)
	return true
}

func lookPathCaddy() string {
	p, err := execLookPath("caddy")
	if err != nil {
		return ""
	}
	return p
}

func caddyBinaryVersion() string {
	out, err := runCmd("caddy", "version")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}
