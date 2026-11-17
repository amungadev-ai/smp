// End-to-end tests of the M01 workflow in mock mode: login → deploy →
// health → routing → release history → rollback → restart → removal, plus an
// SSE log stream probe. The same flow runs against real Docker/Caddy on a VPS.
package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"drevanta/engine/internal/auth"
	"drevanta/engine/internal/jobs"
	"drevanta/engine/internal/proxy"
	"drevanta/engine/internal/runtime"
	"drevanta/engine/internal/store"
)

type harness struct {
	srv     *httptest.Server
	client  *http.Client
	token   string
	handler http.Handler
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	// engine writes land after jobs finish; register dir cleanup FIRST so it
	// runs LAST (after the worker cancel below), with a small grace period.
	stateDir, _ := os.MkdirTemp("", "drvt-state-*")
	dataDir, _ := os.MkdirTemp("", "drvt-data-*")
	t.Cleanup(func() {
		time.Sleep(400 * time.Millisecond)
		os.RemoveAll(stateDir)
		os.RemoveAll(dataDir)
	})

	st, err := store.Open(stateDir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	engine := jobs.New("test", dataDir, st, runtime.NewMock(), proxy.NewMock())
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	engine.StartWorkers(ctx)

	a := New(engine, auth.New("lab-password"), st)
	h := &harness{
		srv: httptest.NewServer(a.Handler()),
		client: &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		handler: a.Handler(),
	}
	t.Cleanup(h.srv.Close)

	// login
	token := h.post(t, "/api/v1/auth/login", map[string]string{"password": "lab-password"})
	var lr struct {
		Token string `json:"token"`
	}
	json.Unmarshal([]byte(token), &lr)
	if lr.Token == "" {
		t.Fatalf("login response: %s", token)
	}
	h.token = lr.Token
	return h
}

func (h *harness) do(t *testing.T, method, path string, body any) (int, string) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, h.srv.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+h.token)
	req.Header.Set("Content-Type", "application/json")
	res, err := h.client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	var buf bytes.Buffer
	buf.ReadFrom(res.Body)
	return res.StatusCode, buf.String()
}

func (h *harness) post(t *testing.T, path string, body any) string {
	t.Helper()
	code, out := h.do(t, "POST", path, body)
	if code >= 300 {
		t.Fatalf("POST %s → %d: %s", path, code, out)
	}
	return out
}

func (h *harness) get(t *testing.T, path string) (int, string) {
	return h.do(t, "GET", path, nil)
}

// waitJob polls until the job leaves the queued/running states.
func (h *harness) waitJob(t *testing.T, jobID string) (int, string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		code, out := h.get(t, "/api/v1/jobs/"+jobID)
		if code != 200 {
			t.Fatalf("job fetch %d: %s", code, out)
		}
		var j struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		}
		json.Unmarshal([]byte(out), &j)
		if j.Status == "succeeded" {
			return code, out
		}
		if j.Status == "failed" {
			t.Fatalf("job failed: %s", out)
		}
		time.Sleep(120 * time.Millisecond)
	}
	t.Fatal("job did not finish in 20s")
	return 0, ""
}

func TestFullDeploymentWorkflow(t *testing.T) {
	h := newHarness(t)

	// 1. deploy
	out := h.post(t, "/api/v1/projects/deploy", map[string]any{
		"name": "marketing-site", "image": "nginx:alpine", "port": 80, "domain": "app.example.com",
	})
	var dep struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
		Job struct {
			ID string `json:"id"`
		} `json:"job"`
	}
	if err := json.Unmarshal([]byte(out), &dep); err != nil {
		t.Fatalf("deploy response: %s", out)
	}
	h.waitJob(t, dep.Job.ID)

	// 2. project is running with a current release
	code, out := h.get(t, "/api/v1/projects/"+dep.Project.ID)
	if code != 200 {
		t.Fatalf("project fetch: %d %s", code, out)
	}
	var p struct {
		Status         string `json:"status"`
		CurrentRelease string `json:"current_release"`
		Domain         string `json:"domain"`
	}
	json.Unmarshal([]byte(out), &p)
	if p.Status != "running" || p.CurrentRelease == "" || p.Domain != "app.example.com" {
		t.Fatalf("project state wrong: %s", out)
	}

	// 3. release history shows one healthy release
	code, out = h.get(t, "/api/v1/releases?project="+dep.Project.ID)
	if code != 200 {
		t.Fatalf("releases: %d", code)
	}
	var rl struct {
		Releases []struct {
			Status   string `json:"status"`
			HealthOK *bool  `json:"health_ok"`
		} `json:"releases"`
	}
	json.Unmarshal([]byte(out), &rl)
	if len(rl.Releases) != 1 || rl.Releases[0].Status != "healthy" || rl.Releases[0].HealthOK == nil || !*rl.Releases[0].HealthOK {
		t.Fatalf("release history wrong: %s", out)
	}

	// 4. routing recorded
	code, out = h.get(t, "/api/v1/domains")
	if code != 200 || !strings.Contains(out, "app.example.com") {
		t.Fatalf("domains: %d %s", code, out)
	}

	// 5. redeploy marketing-site on a new tag, then roll back to the previous release
	out = h.post(t, fmt.Sprintf("/api/v1/projects/%s/deploy", dep.Project.ID), map[string]any{
		"image": "nginx:1.25",
	})
	var j1 struct {
		Job struct {
			ID string `json:"id"`
		} `json:"job"`
	}
	json.Unmarshal([]byte(out), &j1)
	h.waitJob(t, j1.Job.ID)

	out = h.post(t, fmt.Sprintf("/api/v1/projects/%s/rollback", dep.Project.ID), nil)
	var j2 struct {
		Job struct {
			ID string `json:"id"`
		} `json:"job"`
	}
	json.Unmarshal([]byte(out), &j2)
	h.waitJob(t, j2.Job.ID)

	code, out = h.get(t, "/api/v1/releases?project="+dep.Project.ID)
	if code != 200 {
		t.Fatal("releases after rollback")
	}
	var rl2 struct {
		Releases []struct {
			Status string `json:"status"`
		} `json:"releases"`
	}
	json.Unmarshal([]byte(out), &rl2)
	if len(rl2.Releases) < 2 {
		t.Fatalf("rollback should add a release: %s", out)
	}
	hasHealthy := false
	for _, r := range rl2.Releases {
		if r.Status == "healthy" {
			hasHealthy = true
		}
	}
	if !hasHealthy {
		t.Fatalf("no healthy release after rollback: %s", out)
	}
}

func TestAuthGuardsAndValidation(t *testing.T) {
	h := newHarness(t)

	// unauthenticated requests are rejected
	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Get(h.srv.URL + "/api/v1/projects")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("unauthenticated /projects → %d, want 401", res.StatusCode)
	}

	// validation errors
	code, out := h.do(t, "POST", "/api/v1/projects/deploy", map[string]any{"name": "BAD NAME", "image": "nginx", "port": 80})
	if code != 400 {
		t.Fatalf("bad name → %d: %s", code, out)
	}
	code, _ = h.do(t, "POST", "/api/v1/projects/deploy", map[string]any{"name": "ok-name", "image": "not a valid image!!", "port": 80})
	if code != 400 {
		t.Fatalf("bad image → %d", code)
	}
	code, _ = h.do(t, "POST", "/api/v1/projects/deploy", map[string]any{"name": "ok-name", "image": "nginx", "port": 99999})
	if code != 400 {
		t.Fatalf("bad port → %d", code)
	}
	// duplicate name
	h.post(t, "/api/v1/projects/deploy", map[string]any{"name": "dup", "image": "nginx", "port": 80})
	code, _ = h.do(t, "POST", "/api/v1/projects/deploy", map[string]any{"name": "dup", "image": "nginx", "port": 80})
	if code != 409 {
		t.Fatalf("duplicate → %d, want 409", code)
	}
	// wrong password login
	client2 := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("POST", h.srv.URL+"/api/v1/auth/login", strings.NewReader(`{"password":"WRONG"}`))
	res2, err := client2.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res2.Body.Close()
	if res2.StatusCode != 401 {
		t.Fatalf("wrong password → %d, want 401", res2.StatusCode)
	}
}

func TestProjectRemoval(t *testing.T) {
	h := newHarness(t)
	out := h.post(t, "/api/v1/projects/deploy", map[string]any{"name": "todelete", "image": "nginx:alpine", "port": 80})
	var dep struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
		Job struct {
			ID string `json:"id"`
		} `json:"job"`
	}
	json.Unmarshal([]byte(out), &dep)
	h.waitJob(t, dep.Job.ID)

	code, _ := h.do(t, "DELETE", "/api/v1/projects/"+dep.Project.ID, nil)
	if code != 200 {
		t.Fatalf("delete → %d", code)
	}
	code, _ = h.get(t, "/api/v1/projects/"+dep.Project.ID)
	if code != 404 {
		t.Fatalf("project still present after delete: %d", code)
	}
}

func TestLogsStreamDeliversFrames(t *testing.T) {
	h := newHarness(t)
	out := h.post(t, "/api/v1/projects/deploy", map[string]any{"name": "logs", "image": "nginx:alpine", "port": 80})
	var dep struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
		Job struct {
			ID string `json:"id"`
		} `json:"job"`
	}
	json.Unmarshal([]byte(out), &dep)
	h.waitJob(t, dep.Job.ID)

	req, _ := http.NewRequest("GET", h.srv.URL+"/api/v1/projects/"+dep.Project.ID+"/logs/stream", nil)
	req.Header.Set("Authorization", "Bearer "+h.token)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	req = req.WithContext(ctx)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("stream connect: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("stream status %d", res.StatusCode)
	}

	gotFrame := false
	sc := bufio.NewScanner(res.Body)
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "data: ") {
			gotFrame = true
			break
		}
	}
	if !gotFrame {
		t.Fatal("no SSE data frame within 8s")
	}
}

func TestHealthEndpointShape(t *testing.T) {
	h := newHarness(t)
	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Get(h.srv.URL + "/api/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var body struct {
		OK       bool   `json:"ok"`
		Version  string `json:"version"`
		MockMode bool   `json:"mock_mode"`
	}
	json.NewDecoder(res.Body).Decode(&body)
	if !body.OK || body.Version == "" || !body.MockMode {
		t.Fatalf("health shape wrong: %+v", body)
	}
}
