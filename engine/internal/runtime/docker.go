// Package runtime drives Docker through the Engine API unix socket.
// When the socket is absent the runtime degrades to mock mode so the whole
// deploy → health → HTTPS → logs → rollback workflow stays testable on any
// machine (and in CI) — mode is always visible through the API.
package runtime

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"drevanta/engine/internal/model"
)

const (
	ModeDocker = "docker"
	ModeMock   = "mock"
)

var defaultSocket = "/var/run/docker.sock"

type Docker struct {
	mode       string
	sock       string
	client     *http.Client
	logMu      sync.Mutex
	logFollows map[string]context.CancelFunc // project -> running follow goroutine
	logSubs    map[string][]chan model.LogLine
	subsMu     sync.Mutex
	mu         sync.Mutex // serializes container mutations
}

func New() *Docker {
	d := &Docker{sock: defaultSocket, logFollows: map[string]context.CancelFunc{}, logSubs: map[string][]chan model.LogLine{}}
	if _, err := os.Stat(defaultSocket); err == nil {
		if conn, err := net.DialTimeout("unix", defaultSocket, 500*time.Millisecond); err == nil {
			conn.Close()
			d.mode = ModeDocker
			d.client = &http.Client{
				Timeout: 0, // streaming endpoints manage their own deadlines
				Transport: &http.Transport{
					DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
						var nd net.Dialer
						return nd.DialContext(ctx, "unix", d.sock)
					},
				},
			}
			return d
		}
	}
	d.mode = ModeMock
	return d
}

func (d *Docker) Mode() string { return d.mode }

// NewMock forces simulated mode (lab/CI use, or --mock flag).
func NewMock() *Docker {
	return &Docker{mode: ModeMock, logFollows: map[string]context.CancelFunc{}, logSubs: map[string][]chan model.LogLine{}}
}

func (d *Docker) Ping() error {
	if d.mode == ModeMock {
		return fmt.Errorf("docker socket not present")
	}
	resp, err := d.call(http.MethodGet, "/_ping", nil, 3*time.Second)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (d *Docker) Version() (string, error) {
	if d.mode == ModeMock {
		return "", fmt.Errorf("docker not available")
	}
	resp, err := d.call(http.MethodGet, "/version", nil, 5*time.Second)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var v struct {
		Version string `json:"Version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return "", err
	}
	return v.Version, nil
}

// call performs a request against the unix socket.
func (d *Docker) call(method, path string, body io.Reader, timeout time.Duration) (*http.Response, error) {
	req, err := http.NewRequest(method, "http://docker"+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := d.client
	if timeout > 0 {
		client = &http.Client{Timeout: timeout, Transport: d.client.Transport}
	}
	return client.Do(req)
}

// Pull fetches an image (streaming progress is summarized by the caller).
func (d *Docker) Pull(ctx context.Context, image string) error {
	if d.mode == ModeMock {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(400 * time.Millisecond): // simulate pull latency
		}
		return nil
	}
	image, tag := splitImage(image)
	reqPath := fmt.Sprintf("/images/create?fromImage=%s&tag=%s", image, tag)
	resp, err := d.call(http.MethodPost, reqPath, nil, 0)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return fmt.Errorf("docker pull failed: %s", strings_Sanitize(string(msg)))
	}
	// consume progress stream; surface only errors
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		var line struct {
			Error  string `json:"error"`
			Status string `json:"status"`
		}
		if err := json.Unmarshal(sc.Bytes(), &line); err == nil && line.Error != "" {
			return fmt.Errorf("docker pull error: %s", line.Error)
		}
	}
	return nil
}

func splitImage(image string) (name, tag string) {
	tag = "latest"
	if i := len(image) - 1; i > 0 {
		if j := lastColon(image); j > 0 && j < i {
			return image[:j], image[j+1:]
		}
	}
	return image, tag
}

func lastColon(s string) int {
	// avoid cutting on registry ports (host:port/image)
	slash := -1
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			slash = i
			break
		}
	}
	for i := len(s) - 1; i > slash; i-- {
		if s[i] == ':' {
			return i
		}
	}
	return -1
}

func strings_Sanitize(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= 32 && r < 127 {
			out = append(out, r)
		}
	}
	return string(out)
}

// EnsureRunning pulls nothing; it creates (or reuses) and starts the project's
// container, returning the container id and the mapped host port.
func (d *Docker) EnsureRunning(ctx context.Context, project model.Project) (string, int, error) {
	name := ContainerName(project.Name)
	if d.mode == ModeMock {
		d.mu.Lock()
		defer d.mu.Unlock()
		select {
		case <-ctx.Done():
			return "", 0, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
		return fmt.Sprintf("mock-%s-%d", project.Name, time.Now().Unix()%100000), project.Port, nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	// reuse existing container if present
	resp, err := d.call(http.MethodGet, "/containers/"+name+"/json", nil, 10*time.Second)
	if err == nil && resp.StatusCode == http.StatusOK {
		var inspect struct {
			ID    string `json:"Id"`
			State struct {
				Running bool `json:"Running"`
			} `json:"State"`
		}
		err := json.NewDecoder(resp.Body).Decode(&inspect)
		resp.Body.Close()
		if err == nil {
			if !inspect.State.Running {
				if r2, err2 := d.call(http.MethodPost, "/containers/"+name+"/start", nil, 30*time.Second); err2 != nil {
					return "", 0, err2
				} else {
					r2.Body.Close()
				}
			}
			port, perr := d.mappedPort(name, project.Port)
			return inspect.ID, port, perr
		}
		resp.Body.Close()
	} else if resp != nil {
		resp.Body.Close()
	}

	// create new container with an ephemeral host port
	createBody := map[string]any{
		"Image":        project.Image,
		"ExposedPorts": map[string]any{fmt.Sprintf("%d/tcp", project.Port): struct{}{}},
		"HostConfig": map[string]any{
			"PortBindings": map[string]any{
				fmt.Sprintf("%d/tcp", project.Port): []map[string]string{{"HostPort": ""}},
			},
		},
		"Labels": map[string]string{"drevanta.project": project.Name},
	}
	raw, _ := json.Marshal(createBody)
	resp, err = d.call(http.MethodPost, "/containers/create?name="+name, bytes.NewReader(raw), 30*time.Second)
	if err != nil {
		return "", 0, err
	}
	if resp.StatusCode != http.StatusCreated {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		resp.Body.Close()
		return "", 0, fmt.Errorf("container create failed: %s", strings_Sanitize(string(msg)))
	}
	var created struct {
		ID string `json:"Id"`
	}
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	if r2, err := d.call(http.MethodPost, "/containers/"+created.ID+"/start", nil, 60*time.Second); err != nil {
		return "", 0, err
	} else {
		r2.Body.Close()
	}
	port, err := d.mappedPort(name, project.Port)
	return created.ID, port, err
}

func (d *Docker) mappedPort(name string, containerPort int) (int, error) {
	resp, err := d.call(http.MethodGet, "/containers/"+name+"/json", nil, 10*time.Second)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var inspect struct {
		NetworkSettings struct {
			Ports map[string][]struct {
				HostPort string `json:"HostPort"`
			} `json:"Ports"`
		} `json:"NetworkSettings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&inspect); err != nil {
		return 0, err
	}
	bindings := inspect.NetworkSettings.Ports[fmt.Sprintf("%d/tcp", containerPort)]
	if len(bindings) == 0 || bindings[0].HostPort == "" {
		return 0, fmt.Errorf("no host port mapped for %d/tcp", containerPort)
	}
	var port int
	fmt.Sscanf(bindings[0].HostPort, "%d", &port)
	return port, nil
}

// Stop removes the container (create-again model keeps state simple).
func (d *Docker) Stop(project string) error {
	name := ContainerName(project)
	if d.mode == ModeMock {
		return nil
	}
	if r, err := d.call(http.MethodPost, "/containers/"+name+"/stop?t=10", nil, 30*time.Second); err == nil {
		r.Body.Close()
	}
	if r, err := d.call(http.MethodDelete, "/containers/"+name, nil, 30*time.Second); err == nil {
		r.Body.Close()
	}
	return nil
}

// Restart restarts the container in place.
func (d *Docker) Restart(project string) error {
	name := ContainerName(project)
	if d.mode == ModeMock {
		time.Sleep(300 * time.Millisecond)
		return nil
	}
	r, err := d.call(http.MethodPost, "/containers/"+name+"/restart?t=10", nil, 60*time.Second)
	if err != nil {
		return err
	}
	r.Body.Close()
	if r.StatusCode != http.StatusNoContent && r.StatusCode != http.StatusOK {
		return fmt.Errorf("restart returned %d", r.StatusCode)
	}
	return nil
}

// StreamLogs subscribes the caller to container output (or mock trace lines)
// and starts a follow/generator goroutine while subscribers exist.
func (d *Docker) StreamLogs(project string) (<-chan model.LogLine, func()) {
	ch := make(chan model.LogLine, 64)
	d.subsMu.Lock()
	d.logSubs[project] = append(d.logSubs[project], ch)
	first := len(d.logSubs[project]) == 1
	d.subsMu.Unlock()

	if first {
		ctx, cancel := context.WithCancel(context.Background())
		d.logMu.Lock()
		d.logFollows[project] = cancel
		d.logMu.Unlock()
		if d.mode == ModeMock {
			go d.mockLogGenerator(ctx, project)
		} else {
			go d.followLogs(ctx, project)
		}
	}

	unsub := func() {
		d.subsMu.Lock()
		subs := d.logSubs[project]
		for i, c := range subs {
			if c == ch {
				d.logSubs[project] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
		empty := len(d.logSubs[project]) == 0
		d.subsMu.Unlock()
		if empty {
			d.logMu.Lock()
			if cancel, ok := d.logFollows[project]; ok {
				cancel()
				delete(d.logFollows, project)
			}
			d.logMu.Unlock()
		}
	}
	return ch, unsub
}

func (d *Docker) broadcast(project string, line model.LogLine) {
	d.subsMu.Lock()
	defer d.subsMu.Unlock()
	for _, c := range d.logSubs[project] {
		select {
		case c <- line:
		default: // slow consumer — drop rather than block the container stream
		}
	}
}

// followLogs tails the container log stream and demultiplexes Docker's
// 8-byte-frame format (stream byte + 4-byte big-endian length).
func (d *Docker) followLogs(ctx context.Context, project string) {
	name := ContainerName(project)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		resp, err := d.call(http.MethodGet,
			"/containers/"+name+"/logs?stdout=1&stderr=1&follow=1&timestamps=1", nil, 0)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		br := bufio.NewReader(resp.Body)
		header := make([]byte, 8)
		for {
			if _, err := io.ReadFull(br, header); err != nil {
				break
			}
			length := int(binary.BigEndian.Uint32(header[4:8]))
			payload := make([]byte, length)
			if _, err := io.ReadFull(br, payload); err != nil {
				break
			}
			stream := "stdout"
			if header[0] == 2 {
				stream = "stderr"
			}
			d.broadcast(project, model.LogLine{Stream: stream, Message: string(payload)})
		}
		resp.Body.Close()
		if ctx.Err() != nil {
			return
		}
		time.Sleep(1 * time.Second) // container restart — reconnect
	}
}

// mockLogGenerator emits a small repeating trace so SSE clients see live data.
// It broadcasts to all current subscribers and stops when the context ends.
func (d *Docker) mockLogGenerator(ctx context.Context, project string) {
	messages := []string{
		"listening on port 80",
		"GET / 200 — health probe ok",
		"worker pool steady state",
		"cache warm-up complete",
	}
	t := time.NewTicker(700 * time.Millisecond)
	defer t.Stop()
	i := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			stream := "stdout"
			if i%7 == 5 {
				stream = "stderr"
			}
			d.broadcast(project, model.LogLine{Stream: stream, Message: fmt.Sprintf("[%s] %s", project, messages[i%len(messages)])})
			i++
		}
	}
}

// ContainerName is the deterministic container name for a project.
func ContainerName(project string) string { return "drevanta-" + project }
