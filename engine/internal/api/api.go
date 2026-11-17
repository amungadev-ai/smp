// Package api exposes the engine's REST interface and the SSE log stream.
// Route patterns use Go 1.22 ServeMux wildcards.
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"drevanta/engine/internal/auth"
	"drevanta/engine/internal/jobs"
	"drevanta/engine/internal/model"
	"drevanta/engine/internal/serverinfo"
	"drevanta/engine/internal/store"
)

var (
	slugRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,48}$`)
	imageRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]*(:[a-zA-Z0-9._-]+)?$`)
	domainRe = regexp.MustCompile(`^(\*\.)?([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,24}$`)
)

type API struct {
	Engine *jobs.Engine
	Auth   *auth.Authenticator
	Store  *store.Store

	mu          sync.Mutex
	loginFails  map[string][]time.Time
}

func New(engine *jobs.Engine, a *auth.Authenticator, st *store.Store) *API {
	return &API{Engine: engine, Auth: a, Store: st, loginFails: map[string][]time.Time{}}
}

/* ------------------------- plumbing ------------------------- */

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	_ = enc.Encode(v)
}

func errJSON(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// readBody parses a JSON body and always consumes the stream so keep-alive
// connections stay in sync even when validation fails afterwards.
func readBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		errJSON(w, 400, "unreadable body")
		return false
	}
	_, _ = io.Copy(io.Discard, r.Body) // anything beyond the cap
	if len(body) == 0 {
		return true
	}
	if err := json.Unmarshal(body, dst); err != nil {
		errJSON(w, 400, "invalid JSON body")
		return false
	}
	return true
}

func clientIP(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		return strings.TrimSpace(strings.Split(xf, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

/* ------------------------- router ------------------------- */

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()

	// unauthenticated
	mux.HandleFunc("GET /api/v1/health", a.handleHealth)
	mux.HandleFunc("POST /api/v1/auth/login", a.handleLogin)

	// server discovery
	mux.HandleFunc("GET /api/v1/server", a.authed(a.handleServer))
	mux.HandleFunc("GET /api/v1/server/readiness", a.authed(a.handleReadiness))

	// projects
	mux.HandleFunc("GET /api/v1/projects", a.authed(a.handleProjects))
	mux.HandleFunc("POST /api/v1/projects/deploy", a.authed(a.handleDeploy))
	mux.HandleFunc("POST /api/v1/projects/{id}/deploy", a.authed(a.handleRedeploy))
	mux.HandleFunc("GET /api/v1/projects/{id}", a.authed(a.handleProject))
	mux.HandleFunc("DELETE /api/v1/projects/{id}", a.authed(a.handleProjectDelete))
	mux.HandleFunc("POST /api/v1/projects/{id}/restart", a.authed(a.handleRestart))
	mux.HandleFunc("POST /api/v1/projects/{id}/rollback", a.authed(a.handleRollback))
	mux.HandleFunc("GET /api/v1/projects/{id}/logs/stream", a.authed(a.handleLogsStream))

	// releases & jobs
	mux.HandleFunc("GET /api/v1/releases", a.authed(a.handleReleases))
	mux.HandleFunc("GET /api/v1/jobs", a.authed(a.handleJobs))
	mux.HandleFunc("GET /api/v1/jobs/{id}", a.authed(a.handleJob))

	// domains
	mux.HandleFunc("GET /api/v1/domains", a.authed(a.handleDomains))
	mux.HandleFunc("POST /api/v1/domains", a.authed(a.handleDomainAdd))
	mux.HandleFunc("DELETE /api/v1/domains/{id}", a.authed(a.handleDomainDelete))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		errJSON(w, 404, "not found")
	})
	return mux
}

func (a *API) authed(next http.HandlerFunc) http.HandlerFunc {
	return a.Auth.Middleware(func(w http.ResponseWriter, r *http.Request) {
		next(w, r)
	})
}

/* ------------------------- handlers ------------------------- */

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"ok": true, "version": a.Engine.Version, "mock_mode": a.Engine.Docker.Mode() == "mock",
	})
}

func (a *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if !readBody(w, r, &body) {
		return
	}
	ip := clientIP(r)
	a.mu.Lock()
	now := time.Now()
	window := a.loginFails[ip][:0]
	for _, t := range a.loginFails[ip] {
		if now.Sub(t) < time.Minute {
			window = append(window, t)
		}
	}
	a.loginFails[ip] = window
	rateLimited := len(window) >= 5
	a.mu.Unlock()
	if rateLimited {
		errJSON(w, 429, "too many login attempts — wait a minute")
		return
	}

	token, err := a.Auth.Login(body.Password, ip)
	if err != nil {
		if err == auth.ErrRateLimited {
			errJSON(w, 429, err.Error())
			return
		}
		a.mu.Lock()
		a.loginFails[ip] = append(a.loginFails[ip], now)
		a.mu.Unlock()
		errJSON(w, 401, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"token": token, "version": a.Engine.Version})
}

func (a *API) handleServer(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, serverinfo.Discover(a.Engine.Version))
}

func (a *API) handleReadiness(w http.ResponseWriter, r *http.Request) {
	info := serverinfo.Discover(a.Engine.Version)
	writeJSON(w, 200, map[string]any{
		"checks":    serverinfo.Readiness(info, a.Engine.DataDir),
		"mock_mode": a.Engine.Docker.Mode() == "mock",
	})
}

func (a *API) handleProjects(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"projects": a.Store.Projects()})
}

func (a *API) handleProject(w http.ResponseWriter, r *http.Request) {
	p, ok := a.Store.Project(r.PathValue("id"))
	if !ok {
		errJSON(w, 404, "project not found")
		return
	}
	writeJSON(w, 200, p)
}

func (a *API) handleDeploy(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name   string `json:"name"`
		Image  string `json:"image"`
		Port   int    `json:"port"`
		Domain string `json:"domain"`
		HTTPS  *bool  `json:"https"`
	}
	if !readBody(w, r, &body) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(body.Name))
	if !slugRe.MatchString(name) {
		errJSON(w, 400, "project name: lowercase letters, digits and dashes (max 49)")
		return
	}
	image := strings.TrimSpace(body.Image)
	if !imageRe.MatchString(image) {
		errJSON(w, 400, "image must look like registry/name:tag")
		return
	}
	if body.Port < 1 || body.Port > 65535 {
		errJSON(w, 400, "container port must be 1-65535")
		return
	}
	for _, p := range a.Store.Projects() {
		if p.Name == name {
			errJSON(w, 409, "project name already exists")
			return
		}
		if body.Domain != "" && p.Domain == body.Domain {
			errJSON(w, 409, "domain already routed to project "+p.Name)
			return
		}
	}
	https := true
	if body.HTTPS != nil {
		https = *body.HTTPS
	}
	project := model.Project{
		ID:      fmt.Sprintf("%s-%s", name, shortRand()),
		Name:    name,
		Image:   image,
		Port:    body.Port,
		Status:  model.StatusIdle,
		Created: model.Now(),
		HTTPS:   https && body.Domain != "",
	}
	if body.Domain != "" {
		d := domainRe.MatchString(strings.ToLower(body.Domain))
		if !d {
			errJSON(w, 400, "invalid domain name")
			return
		}
		project.Domain = strings.ToLower(body.Domain)
	}
	if err := a.Store.AddProject(project); err != nil {
		errJSON(w, 500, err.Error())
		return
	}
	if project.Domain != "" {
		_ = a.Store.AddDomain(model.Domain{
			ID: shortRand(), Name: project.Domain, ProjectID: project.ID,
			Scheme: map[bool]string{true: "https", false: "http"}[project.HTTPS],
			Created: model.Now(), Status: model.DomainPending,
		})
	}
	job, err := a.Engine.Enqueue(project.ID, model.JobDeploy)
	if err != nil {
		errJSON(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"project": project, "job": job})
}

func (a *API) handleProjectDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, ok := a.Store.Project(id)
	if !ok {
		errJSON(w, 404, "project not found")
		return
	}
	_ = a.Engine.Docker.Stop(p.Name)
	// detach routed domains and re-apply caddy
	for _, d := range a.Store.Domains() {
		if d.ProjectID == id {
			_ = a.Store.RemoveDomain(d.ID)
		}
	}
	_ = a.Store.UpdateProject(id, func(pp *model.Project) { pp.Status = model.StatusStopped })
	if err := a.Store.RemoveProject(id); err != nil {
		errJSON(w, 500, err.Error())
		return
	}
	_ = a.Engine.Caddy.ApplyConfig(a.Store.Domains(), a.Store.Projects())
	writeJSON(w, 200, map[string]bool{"removed": true})
}

// handleRedeploy rolls a project forward to a new image/tag — the everyday
// "Deploy application" action on an existing project.
func (a *API) handleRedeploy(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Image string `json:"image"`
	}
	if !readBody(w, r, &body) {
		return
	}
	image := strings.TrimSpace(body.Image)
	if !imageRe.MatchString(image) {
		errJSON(w, 400, "image must look like registry/name:tag")
		return
	}
	id := r.PathValue("id")
	if _, ok := a.Store.Project(id); !ok {
		errJSON(w, 404, "project not found")
		return
	}
	if err := a.Store.UpdateProject(id, func(p *model.Project) { p.Image = image }); err != nil {
		errJSON(w, 500, err.Error())
		return
	}
	job, err := a.Engine.Enqueue(id, model.JobDeploy)
	if err != nil {
		errJSON(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"job": job})
}

func (a *API) handleRestart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := a.Store.Project(id); !ok {
		errJSON(w, 404, "project not found")
		return
	}
	job, err := a.Engine.Enqueue(id, model.JobRestart)
	if err != nil {
		errJSON(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"job": job})
}

func (a *API) handleRollback(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Release string `json:"release"`
	}
	if !readBody(w, r, &body) {
		return
	}
	id := r.PathValue("id")
	if _, ok := a.Store.Project(id); !ok {
		errJSON(w, 404, "project not found")
		return
	}
	job, err := a.Engine.Enqueue(id, model.JobRollback)
	if err != nil {
		errJSON(w, 500, err.Error())
		return
	}
	_ = body.Release // specific-target selection lands with milestone 04; M01 rolls to previous healthy
	writeJSON(w, 200, map[string]any{"job": job})
}

func (a *API) handleReleases(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"releases": a.Store.Releases(r.URL.Query().Get("project"))})
}

func (a *API) handleJobs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"jobs": a.Store.Jobs(r.URL.Query().Get("project"))})
}

func (a *API) handleJob(w http.ResponseWriter, r *http.Request) {
	j, ok := a.Store.Job(r.PathValue("id"))
	if !ok {
		errJSON(w, 404, "job not found")
		return
	}
	writeJSON(w, 200, j)
}

func (a *API) handleDomains(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"domains": a.Store.Domains()})
}

func (a *API) handleDomainAdd(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name      string `json:"name"`
		ProjectID string `json:"project_id"`
		Scheme    string `json:"scheme"`
	}
	if !readBody(w, r, &body) {
		return
	}
	name := strings.ToLower(strings.TrimSpace(body.Name))
	if !domainRe.MatchString(name) {
		errJSON(w, 400, "invalid domain name")
		return
	}
	scheme := body.Scheme
	if scheme != "http" && scheme != "https" {
		scheme = "https"
	}
	if pid := body.ProjectID; pid != "" {
		if _, ok := a.Store.Project(pid); !ok {
			errJSON(w, 404, "project not found")
			return
		}
	}
	d := model.Domain{
		ID: shortRand(), Name: name, ProjectID: body.ProjectID,
		Scheme: scheme, Created: model.Now(), Status: model.DomainPending,
	}
	if err := a.Store.AddDomain(d); err != nil {
		errJSON(w, 409, err.Error())
		return
	}
	if d.ProjectID != "" {
		_ = a.Store.UpdateProject(d.ProjectID, func(p *model.Project) {
			p.Domain = d.Name
			p.HTTPS = scheme == "https"
		})
	}
	if err := a.Engine.Caddy.ApplyConfig(a.Store.Domains(), a.Store.Projects()); err != nil {
		log.Printf("caddy apply: %v", err)
	}
	writeJSON(w, 200, map[string]any{"domain": d})
}

func (a *API) handleDomainDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var removed model.Domain
	found := false
	for _, d := range a.Store.Domains() {
		if d.ID == id {
			removed, found = d, true
		}
	}
	if !found {
		errJSON(w, 404, "domain not found")
		return
	}
	if err := a.Store.RemoveDomain(id); err != nil {
		errJSON(w, 500, err.Error())
		return
	}
	if removed.ProjectID != "" {
		_ = a.Store.UpdateProject(removed.ProjectID, func(p *model.Project) { p.Domain = "" })
	}
	_ = a.Engine.Caddy.ApplyConfig(a.Store.Domains(), a.Store.Projects())
	writeJSON(w, 200, map[string]bool{"removed": true})
}

// handleLogsStream serves container output as Server-Sent Events.
func (a *API) handleLogsStream(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if _, ok := a.Store.Project(projectID); !ok {
		errJSON(w, 404, "project not found")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		errJSON(w, 500, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, ": drevanta log stream\n\n")
	flusher.Flush()

	lines, unsub := a.Engine.Docker.StreamLogs(projectID)
	defer unsub()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case line, ok := <-lines:
			if !ok {
				return
			}
			data, _ := json.Marshal(line)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func shortRand() string {
	b := make([]byte, 4)
	if _, err := cryptoRead(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano()%1e8)
	}
	return hexEncode(b)
}
