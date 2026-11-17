package runtime
import (
 "context"
 "encoding/json"
 "fmt"
 "net"
 "net/http"
 "path/filepath"
 "strings"
 "testing"
 "time"
 "drevanta/engine/internal/model"
)
func fakeDocker(t *testing.T,hostIP string)(*Docker,*int){
 t.Helper()
 sock:=filepath.Join(t.TempDir(),"docker.sock")
 listener,err:=net.Listen("unix",sock);if err!=nil{t.Fatal(err)}
 removed:=0;projectID:="";releaseID:="";name:="";running:=false
 server:=&http.Server{Handler:http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  switch{
  case r.Method=="POST"&&strings.HasPrefix(r.URL.Path,"/containers/create"):
   name=r.URL.Query().Get("name")
   var cfg struct{Image string `json:"Image"`;Labels map[string]string `json:"Labels"`;HostConfig struct{PortBindings map[string][]map[string]string `json:"PortBindings"`} `json:"HostConfig"`}
   if err:=json.NewDecoder(r.Body).Decode(&cfg);err!=nil{t.Errorf("bad Docker JSON: %v",err)}
   if cfg.Image!="nginx:alpine"||cfg.HostConfig.PortBindings["80/tcp"][0]["HostIp"]!="127.0.0.1"{t.Errorf("unsafe Docker config: %+v",cfg)}
   projectID=cfg.Labels[labelProject];releaseID=cfg.Labels[labelRelease]
   w.WriteHeader(201);fmt.Fprint(w,`{"Id":"abc123"}`)
  case r.Method=="POST"&&strings.HasSuffix(r.URL.Path,"/start"):
   running=true;w.WriteHeader(204)
  case r.Method=="GET"&&strings.HasSuffix(r.URL.Path,"/json"):
   if !strings.Contains(r.URL.Path,name){w.WriteHeader(404);return}
   fmt.Fprintf(w,`{"Id":"abc123","Config":{"Labels":{"%s":%q,"%s":%q}},"State":{"Running":%t},"NetworkSettings":{"Ports":{"80/tcp":[{"HostIp":%q,"HostPort":"42343"}]}}}`,labelProject,projectID,labelRelease,releaseID,running,hostIP)
  case r.Method=="DELETE":
   removed++;w.WriteHeader(204)
  default:t.Errorf("unexpected Docker call %s %s",r.Method,r.URL.Path);w.WriteHeader(404)
  }
 })}
 go server.Serve(listener);t.Cleanup(func(){server.Close();listener.Close()})
 return NewWithSocket(sock),&removed
}
func TestReleaseBindsLoopbackAndRemovesOnlyOwnedContainer(t *testing.T){
 docker,n:=fakeDocker(t,"127.0.0.1")
 ctx,cancel:=context.WithTimeout(context.Background(),3*time.Second);defer cancel()
 p:=model.Project{ID:"project-1",Port:80};r:=model.Release{ID:"release-1"}
 id,port,err:=docker.StartRelease(ctx,p,r,"nginx:alpine")
 if err!=nil||id!="abc123"||port!=42343{t.Fatalf("start: %q %d %v",id,port,err)}
 if err:=docker.StopRelease(ctx,p.ID,r.ID);err!=nil{t.Fatal(err)}
 if *n!=1{t.Fatalf("unexpected removals: %d",*n)}
}
func TestPublicPortMappingFailsAndCleansCandidate(t *testing.T){
 docker,n:=fakeDocker(t,"0.0.0.0")
 ctx,cancel:=context.WithTimeout(context.Background(),3*time.Second);defer cancel()
 _,_,err:=docker.StartRelease(ctx,model.Project{ID:"p",Port:80},model.Release{ID:"r"},"nginx:alpine")
 if err==nil||!strings.Contains(err.Error(),"loopback"){t.Fatalf("expected rejection, got %v",err)}
 if *n!=1{t.Fatal("failed to clean unsafe candidate")}
}
