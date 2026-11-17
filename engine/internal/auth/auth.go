// Package auth implements password login with HMAC session tokens and
// per-IP rate limiting. Standard library only.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	loginWindow = time.Minute
	loginMax    = 5
	tokenTTL    = 24 * time.Hour
)

var (
	ErrWrongPassword = errors.New("wrong password")
	ErrRateLimited   = errors.New("too many login attempts — wait a minute")
	ErrInvalidToken  = errors.New("invalid or expired session token")
)

type Authenticator struct {
	password string
	secret   []byte
	mu       sync.Mutex
	fails    map[string][]time.Time
}

func New(password string) *Authenticator {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		panic("auth: entropy unavailable: " + err.Error())
	}
	return &Authenticator{password: password, secret: secret, fails: map[string][]time.Time{}}
}

// Login validates a password for the given source IP and returns a session token.
func (a *Authenticator) Login(password, ip string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	window := a.fails[ip][:0]
	for _, t := range a.fails[ip] {
		if now.Sub(t) < loginWindow {
			window = append(window, t)
		}
	}
	a.fails[ip] = window
	if len(window) >= loginMax {
		return "", ErrRateLimited
	}
	if subtle.ConstantTimeCompare([]byte(password), []byte(a.password)) != 1 {
		a.fails[ip] = append(window, now)
		return "", ErrWrongPassword
	}
	delete(a.fails, ip)

	issued := now.Unix()
	payload := fmt.Sprintf("%d", issued)
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(payload))
	return payload + "." + hex.EncodeToString(mac.Sum(nil)), nil
}

// Verify checks a token's HMAC and expiry.
func (a *Authenticator) Verify(token string) bool {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return false
	}
	mac := hmac.New(sha256.New, a.secret)
	mac.Write([]byte(parts[0]))
	expected := hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(parts[1]), []byte(expected)) != 1 {
		return false
	}
	var issued int64
	if _, err := fmt.Sscanf(parts[0], "%d", &issued); err != nil {
		return false
	}
	return time.Since(time.Unix(issued, 0)) < tokenTTL
}

// TokenFromRequest accepts Authorization bearer headers only. Never embed
// credentials in URL query strings: proxies and access logs can record them.
func TokenFromRequest(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

// Middleware guards a handler, answering 401 without a valid token.
func (a *Authenticator) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.Verify(TokenFromRequest(r)) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":"not authenticated"}`)
			return
		}
		next(w, r)
	}
}
