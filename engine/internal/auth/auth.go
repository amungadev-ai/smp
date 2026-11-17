// Package auth issues unguessable, revocable, short-lived bearer sessions.
// Only SHA-256 token digests are held in memory. Restart invalidates sessions.
package auth

import (
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
 loginWindow=time.Minute
 loginMax=5
 tokenTTL=8*time.Hour
 maxSessions=128
)

var (
 ErrWrongPassword=errors.New("wrong password")
 ErrRateLimited=errors.New("too many login attempts — wait a minute")
 ErrInvalidToken=errors.New("invalid or expired session token")
)

type Authenticator struct{
 password string
 mu sync.Mutex
 fails map[string][]time.Time
 sessions map[[32]byte]time.Time
}

func New(password string)*Authenticator{
 return &Authenticator{password:password,fails:make(map[string][]time.Time),sessions:make(map[[32]byte]time.Time)}
}

func (a *Authenticator) prune(now time.Time){
 for key,expiry:=range a.sessions{if !now.Before(expiry){delete(a.sessions,key)}}
 // Prevent an unbounded set of distinct source IPs from consuming memory.
 if len(a.fails)>4096{
  for key,attempts:=range a.fails{
   if len(attempts)==0||now.Sub(attempts[len(attempts)-1])>loginWindow{delete(a.fails,key)}
  }
  if len(a.fails)>4096{a.fails=make(map[string][]time.Time)}
 }
}

// Login returns a fresh 256-bit bearer token; identical passwords never
// produce identical sessions. Each token is valid for up to eight hours.
func (a *Authenticator) Login(password,ip string)(string,error){
 a.mu.Lock()
 defer a.mu.Unlock()
 now:=time.Now()
 attempts:=a.fails[ip][:0]
 for _,t:=range a.fails[ip]{if now.Sub(t)<loginWindow{attempts=append(attempts,t)}}
 a.fails[ip]=attempts
 if len(attempts)>=loginMax{return "",ErrRateLimited}
 if subtle.ConstantTimeCompare([]byte(password),[]byte(a.password))!=1{
  a.fails[ip]=append(attempts,now)
  return "",ErrWrongPassword
 }
 delete(a.fails,ip)
 a.prune(now)
 if len(a.sessions)>=maxSessions{
  var oldest [32]byte
  var soonest time.Time
  for id,expiry:=range a.sessions{if soonest.IsZero()||expiry.Before(soonest){soonest=expiry;oldest=id}}
  delete(a.sessions,oldest)
 }
 var token [32]byte
 if _,err:=rand.Read(token[:]);err!=nil{return "",fmt.Errorf("session entropy: %w",err)}
 raw:=hex.EncodeToString(token[:])
 a.sessions[sha256.Sum256(token[:])]=now.Add(tokenTTL)
 return raw,nil
}

func digestToken(raw string)([32]byte,bool){
 var empty [32]byte
 if len(raw)!=64{return empty,false}
 decoded,err:=hex.DecodeString(raw)
 if err!=nil||len(decoded)!=32{return empty,false}
 return sha256.Sum256(decoded),true
}

// Verify checks server-side presence and expiration, without trusting a
// timestamp supplied by the client.
func (a *Authenticator) Verify(token string)bool{
 digest,ok:=digestToken(token)
 if !ok{return false}
 a.mu.Lock()
 defer a.mu.Unlock()
 expiry,found:=a.sessions[digest]
 if !found{return false}
 if !time.Now().Before(expiry){delete(a.sessions,digest);return false}
 return true
}

// Revoke invalidates this bearer session immediately. Tokens not present in
// memory (including after a restart) remain invalid.
func (a *Authenticator) Revoke(token string){
 digest,ok:=digestToken(token)
 if !ok{return}
 a.mu.Lock()
 delete(a.sessions,digest)
 a.mu.Unlock()
}

// TokenFromRequest accepts Authorization bearer headers only.
// Tokens in query strings are never accepted, including for SSE.
func TokenFromRequest(r *http.Request)string{
 h:=r.Header.Get("Authorization")
 if strings.HasPrefix(h,"Bearer "){return strings.TrimSpace(h[7:])}
 return ""
}

func (a *Authenticator) Middleware(next http.HandlerFunc)http.HandlerFunc{
 return func(w http.ResponseWriter,r *http.Request){
  if !a.Verify(TokenFromRequest(r)){
   w.Header().Set("Content-Type","application/json")
   w.Header().Set("Cache-Control","no-store")
   w.WriteHeader(http.StatusUnauthorized)
   fmt.Fprint(w,`{"error":"not authenticated"}`)
   return
  }
  next(w,r)
 }
}
