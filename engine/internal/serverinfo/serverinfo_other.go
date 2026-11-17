//go:build !linux

// Non-linux fallback so the engine compiles everywhere (tests run on any host);
// discovery facts are minimal off Linux, which targets Ubuntu/Debian only.
package serverinfo

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"drevanta/engine/internal/model"
)

type DockerProbe interface {
	Ping() error
	Version() (string, error)
}

type CaddyProbe interface {
	Reachable() bool
}

var (
	dockerProbe DockerProbe
	caddyProbe  CaddyProbe
	probeMu     sync.Mutex
)

func SetProbes(d DockerProbe, c CaddyProbe) {
	probeMu.Lock()
	defer probeMu.Unlock()
	dockerProbe, caddyProbe = d, c
}

func Discover(version string) model.ServerInfo {
	info := model.ServerInfo{
		Hostname:      "unknown",
		OS:            runtime.GOOS,
		Kernel:        "unknown",
		Arch:          runtime.GOARCH,
		CPUCores:      runtime.NumCPU(),
		IP:            "127.0.0.1",
		EngineVersion: version,
	}
	if h, err := os.Hostname(); err == nil {
		info.Hostname = h
	}
	probeMu.Lock()
	d, c := dockerProbe, caddyProbe
	probeMu.Unlock()
	if d != nil {
		if err := d.Ping(); err == nil {
			v, _ := d.Version()
			info.Docker = model.RuntimeInfo{Installed: true, Running: true, Version: v}
		}
	}
	if c != nil && c.Reachable() {
		info.Caddy = model.RuntimeInfo{Installed: true, Running: true}
	}
	info.MockMode = !info.Docker.Installed
	return info
}

func Readiness(info model.ServerInfo, dataDir string) []model.ReadinessCheck {
	writable := false
	probe := filepath.Join(dataDir, ".drevanta-write-probe")
	if os.MkdirAll(dataDir, 0o755) == nil {
		if err := os.WriteFile(probe, []byte("ok"), 0o644); err == nil {
			os.Remove(probe)
			writable = true
		}
	}
	return []model.ReadinessCheck{
		{ID: "os", Label: "Operating system", OK: false, Detail: info.OS + " — production target is Ubuntu/Debian"},
		{ID: "docker", Label: "Docker Engine", OK: info.Docker.Installed, Detail: "discovery limited off Linux"},
		{ID: "caddy", Label: "Caddy reverse proxy", OK: info.Caddy.Installed, Detail: "discovery limited off Linux"},
		{ID: "datadir", Label: "State directory writable", OK: writable, Detail: dataDir},
	}
}

func portFree(port int) bool {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		return false
	}
	ln.Close()
	_ = time.Second
	return port != 80 && port != 443
}
