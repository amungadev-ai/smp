package auth

import (
	"errors"
	"net/http"
	"testing"
 "time"
)

func TestLoginAndVerify(t *testing.T) {
	a := New("s3cret-password")
	token, err := a.Login("s3cret-password", "10.0.0.1")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if !a.Verify(token) {
		t.Fatal("fresh token failed verification")
	}
	if a.Verify(token + "tampered") {
		t.Fatal("tampered token accepted")
	}
	if a.Verify("123.abc") {
		t.Fatal("forged token accepted")
	}
}

func TestWrongPassword(t *testing.T) {
	a := New("right")
	_, err := a.Login("wrong", "10.0.0.2")
	if !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("want ErrWrongPassword, got %v", err)
	}
}

func TestRateLimit(t *testing.T) {
	a := New("pw")
	for i := 0; i < 5; i++ {
		if _, err := a.Login("nope", "10.0.0.3"); err == nil {
			t.Fatalf("attempt %d unexpectedly succeeded", i)
		}
	}
	_, err := a.Login("pw", "10.0.0.3")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("6th attempt should be rate limited, got %v", err)
	}
	// a different IP is unaffected
	if _, err := a.Login("pw", "10.0.0.4"); err != nil {
		t.Fatalf("other ip rate limited: %v", err)
	}
}

func TestQueryTokenCannotAuthenticate(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://127.0.0.1/api/v1/projects?token=abc", nil)
	if TokenFromRequest(r) != "" {
		t.Fatal("URL token accepted")
	}
	r.Header.Set("Authorization", "Bearer good")
	if TokenFromRequest(r) != "good" {
		t.Fatal("bearer token rejected")
	}
}

func TestTokensUniqueAndRevoke(t *testing.T){
 a:=New("pw")
 one,err:=a.Login("pw","first")
 if err!=nil{t.Fatal(err)}
 two,err:=a.Login("pw","first")
 if err!=nil{t.Fatal(err)}
 if one==two||!a.Verify(one)||!a.Verify(two){t.Fatal("sessions were reused or invalid")}
 a.Revoke(one)
 if a.Verify(one)||!a.Verify(two){t.Fatal("logout affected the wrong session")}
 if New("pw").Verify(two){t.Fatal("token survived engine restart")}
}
func TestExpiredTokenRejected(t *testing.T){
 a:=New("pw")
 token,err:=a.Login("pw","first")
 if err!=nil{t.Fatal(err)}
 id,ok:=digestToken(token)
 if !ok{t.Fatal("invalid encoding")}
 a.mu.Lock()
 a.sessions[id]=time.Now().Add(-time.Minute)
 a.mu.Unlock()
 if a.Verify(token){t.Fatal("expired token accepted")}
}
func TestSessionsBounded(t *testing.T){
 a:=New("pw")
 for i:=0;i<maxSessions+5;i++{
  if _,err:=a.Login("pw","first");err!=nil{t.Fatal(err)}
 }
 a.mu.Lock()
 n:=len(a.sessions)
 a.mu.Unlock()
 if n!=maxSessions{t.Fatalf("expected %d sessions, got %d",maxSessions,n)}
}
