// Command drevanta-engine is the Drevanta core server engine (M01).
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"drevanta/engine/internal/api"
	"drevanta/engine/internal/auth"
	"drevanta/engine/internal/jobs"
	"drevanta/engine/internal/proxy"
	"drevanta/engine/internal/runtime"
	"drevanta/engine/internal/serverinfo"
	"drevanta/engine/internal/store"
)

const engineVersion = "0.1.0"

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	dataDir := flag.String("data", "./data", "state directory")
	password := flag.String("password", "", "admin password (else DRVT_ADMIN_PASSWORD env, else generated)")
	webDir := flag.String("web", "./web", "mini console directory")
	forceMock := flag.Bool("mock", false, "force mock mode even if the docker socket exists")
	liveRequested := flag.Bool("live", false, "enable live Docker and Caddy operations (not yet available in M08A)")
	flag.Parse()

	log.Printf("Drevanta engine v%s — discovering local VPS…", engineVersion)

	// state
	st, err := store.Open(*dataDir)
	if err != nil {
		log.Fatalf("state: %v", err)
	}
	log.Printf("state: %s", st.Path())

	// M08A is a mock integration milestone. Refuse live operations until
	// the release-specific container / Caddy traffic-switch safety checks land.
	if *liveRequested && !*forceMock {
		log.Fatal("live Docker/Caddy operations are locked until M08C safety verification; use --mock for local testing")
	}
	d := runtime.NewMock()
	c := proxy.NewMock()
	log.Print("M08A safety gate: Docker and Caddy operations disabled (mock mode)")

	mode := d.Mode()
	log.Printf("runtime mode: %s (docker socket %s)", mode, func() string {
		if mode == runtime.ModeDocker {
			return "connected"
		}
		return "absent — deploys are simulated"
	}())
	log.Printf("proxy mode: %s (caddy admin api %s)", c.Mode(), func() string {
		if c.Mode() == proxy.ModeCaddy {
			return "connected"
		}
		return "not reachable — routing recorded but not applied"
	}())

	// Use the same administrator password after process restarts.
	// An explicit flag / environment override is never logged or persisted.
	pw, generated, err := loadOrCreatePassword(*dataDir, *password)
	if err != nil {
		log.Fatalf("admin credentials: %v", err)
	}
	if generated {
		log.Printf("new admin password saved in %s/admin-password (mode 0600)", *dataDir)
	}

	authn := auth.New(pw)
	engine := jobs.New(engineVersion, *dataDir, st, d, c)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	engine.StartWorkers(ctx)

	// expose probes for discovery, then log the readiness verdict
	serverinfo.SetProbes(d, c)
	info := serverinfo.Discover(engineVersion)
	log.Printf("host: %s · %s · %d cores · docker %s · caddy %s",
		info.Hostname, info.OS, info.CPUCores,
		boolStr(info.Docker.Installed), boolStr(info.Caddy.Installed))

	a := api.New(engine, authn, st)

	mux := http.NewServeMux()
	mux.Handle("/api/v1/", a.Handler())
	rootMounted := false
	if entries, err := os.ReadDir(*webDir); err == nil && len(entries) > 0 {
		fs := http.FileServer(http.Dir(*webDir))
		mux.Handle("/", fs)
		rootMounted = true
		log.Printf("mini console: %s", *webDir)
	}
	if !rootMounted {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "Drevanta engine "+engineVersion+" — API under /api/v1")
		})
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		log.Printf("shutting down…")
		shCtx, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()
		_ = srv.Shutdown(shCtx)
	}()

	log.Printf("listening on %s — login with the admin password, API under /api/v1", *addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("http: %v", err)
	}
}

func boolStr(ok bool) string {
	if ok {
		return "installed"
	}
	return "missing"
}

func generatePassword() (string, error) {
	b := make([]byte, 15)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// loadOrCreatePassword keeps credentials stable across restarts. Explicit
// overrides take precedence; generated secrets are persisted owner-only.
func loadOrCreatePassword(dir, explicit string) (string, bool, error) {
	if explicit != "" {
		return explicit, false, nil
	}
	if v := os.Getenv("DRVT_ADMIN_PASSWORD"); v != "" {
		return v, false, nil
	}
	path := filepath.Join(dir, "admin-password")
	if raw, err := os.ReadFile(path); err == nil {
		pw := strings.TrimSpace(string(raw))
		if pw == "" {
			return "", false, fmt.Errorf("existing password file is empty")
		}
		return pw, false, nil
	} else if !os.IsNotExist(err) {
		return "", false, err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", false, err
	}
	pw, err := generatePassword()
	if err != nil {
		return "", false, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if os.IsExist(err) {
		return loadOrCreatePassword(dir, "")
	}
	if err != nil {
		return "", false, err
	}
	if _, err = f.WriteString(pw + "\n"); err != nil {
		f.Close()
		return "", false, err
	}
	if err = f.Close(); err != nil {
		return "", false, err
	}
	return pw, true, nil
}
