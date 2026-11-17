// Package healthcheck verifies that a deployed container actually answers HTTP
// on its mapped port.
package healthcheck

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Check performs one GET against localhost:<port>.
func Check(ctx context.Context, port int, path string) (bool, int64, error) {
	if path == "" {
		path = "/"
	}
	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, path)
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, 0, err
	}
	transport := &http.Transport{DisableKeepAlives: true}
	client := &http.Client{Timeout: 2 * time.Second, Transport: transport}
	resp, err := client.Do(req)
	if err != nil {
		return false, 0, err
	}
	defer resp.Body.Close()
	latency := time.Since(start).Milliseconds()
	if resp.StatusCode >= 500 {
		return false, latency, fmt.Errorf("health endpoint returned %d", resp.StatusCode)
	}
	return resp.StatusCode < 400, latency, nil
}

// WaitUntilHealthy polls the port until it responds or attempts run out.
func WaitUntilHealthy(ctx context.Context, port int, attempts int, pause time.Duration) (bool, int64, error) {
	var lastErr error
	for i := 0; i < attempts; i++ {
		select {
		case <-ctx.Done():
			return false, 0, ctx.Err()
		default:
		}
		ok, latency, err := Check(ctx, port, "/")
		if ok {
			return true, latency, nil
		}
		if err != nil {
			lastErr = err
		}
		time.Sleep(pause)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no successful response after %d attempts", attempts)
	}
	return false, 0, lastErr
}
