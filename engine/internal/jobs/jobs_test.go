package jobs
import (
 "context"
 "net/http"
 "net/http/httptest"
 "sync"
 "strings"
 "testing"
 "time"
 "drevanta/engine/internal/model"
 "drevanta/engine/internal/proxy"
 "drevanta/engine/internal/runtime"
 "drevanta/engine/internal/store"
)
func fixture(t *testing.T)(*Engine,*store.Store){
 t.Helper();st,err:=store.Open(t.TempDir());if err!=nil{t.Fatal(err)}
 calls:=0
 caddy:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  calls++;if r.URL.Path!="/load"{http.Error(w,"not found",404);return}
  if calls==2{http.Error(w,"Caddy rejected update",503);return}
  w.WriteHeader(200)
 }))
 t.Cleanup(caddy.Close)
 st.AddProject(model.Project{ID:"app",Name:"app",Image:"nginx:alpine",Port:80,Status:model.StatusIdle})
 st.AddDomain(model.Domain{ID:"domain",Name:"app.example.com",ProjectID:"app",Scheme:"https",Status:model.DomainPending})
 e:=New("test",t.TempDir(),st,runtime.NewMock(),proxy.NewWithURL(caddy.URL))
 ctx,cancel:=context.WithCancel(context.Background());t.Cleanup(cancel);e.StartWorkers(ctx)
 return e,st
}
func wait(t *testing.T,st *store.Store,id string)model.Job{
 t.Helper();until:=time.Now().Add(8*time.Second)
 for time.Now().Before(until){j,ok:=st.Job(id);if !ok{t.Fatal("missing job")};if j.Status==model.JobFailed||j.Status==model.JobSucceeded{return j};time.Sleep(20*time.Millisecond)}
 t.Fatal("job timed out");return model.Job{}
}
func TestRejectedCaddySwitchPreservesActiveRelease(t *testing.T){
 e,st:=fixture(t)
 first,_:=e.Enqueue("app",model.JobDeploy)
 if x:=wait(t,st,first.ID);x.Status!=model.JobSucceeded{t.Fatalf("first deployment: %s",x.Error)}
 original,_:=st.Project("app")
 if original.CurrentRelease==""||original.UpstreamPort!=80{t.Fatalf("missing active target: %+v",original)}
 second,_:=e.Enqueue("app",model.JobDeploy,"nginx:1.25")
 if x:=wait(t,st,second.ID);x.Status!=model.JobFailed||!strings.Contains(x.Error,"Caddy routing"){t.Fatalf("expected route rejection: %+v",x)}
 after,_:=st.Project("app")
 if after.CurrentRelease!=original.CurrentRelease||after.Image!=original.Image||after.UpstreamPort!=original.UpstreamPort||after.Status!=model.StatusRunning{t.Fatalf("old release was disrupted: %+v",after)}
 third,_:=e.Enqueue("app",model.JobDeploy,"nginx:1.25")
 if x:=wait(t,st,third.ID);x.Status!=model.JobSucceeded{t.Fatalf("third deploy: %s",x.Error)}
 changed,_:=st.Project("app")
 if changed.Image!="nginx:1.25"||changed.CurrentRelease==original.CurrentRelease{t.Fatal("new release not activated after success")}
 rollback,_:=e.Enqueue("app",model.JobRollback)
 if x:=wait(t,st,rollback.ID);x.Status!=model.JobSucceeded{t.Fatalf("rollback: %s",x.Error)}
 restored,_:=st.Project("app")
 if restored.Image!="nginx:alpine"||restored.CurrentRelease==changed.CurrentRelease{t.Fatalf("wrong rollback: %+v",restored)}
 releases:=st.Releases("app");if releases[len(releases)-1].RolledBackTo==""{t.Fatal("lost rollback provenance")}
}
func TestQueuedImageDoesNotChangeActiveImage(t *testing.T){
 st,_:=store.Open(t.TempDir());st.AddProject(model.Project{ID:"p",Name:"p",Image:"nginx:alpine",Port:80})
 e:=New("test",t.TempDir(),st,runtime.NewMock(),proxy.NewMock())
 j,_:=e.Enqueue("p",model.JobDeploy,"nginx:1.25")
 saved,_:=st.Job(j.ID);p,_:=st.Project("p")
 if saved.Image!="nginx:1.25"||p.Image!="nginx:alpine"{t.Fatalf("immutable request failed: %+v %+v",saved,p)}
}
func TestInterruptedJobRequiresExplicitRetry(t *testing.T){
 st,_:=store.Open(t.TempDir());st.AddProject(model.Project{ID:"p",Name:"p",Image:"nginx:alpine",Port:80})
 st.AddJob(model.Job{ID:"interrupted",ProjectID:"p",Kind:model.JobDeploy,Status:model.JobRunning})
 e:=New("test",t.TempDir(),st,runtime.NewMock(),proxy.NewMock())
 ctx,cancel:=context.WithCancel(context.Background());defer cancel();e.StartWorkers(ctx)
 j,_:=st.Job("interrupted");if j.Status!=model.JobFailed{t.Fatalf("job silently replayed: %+v",j)}
}

func TestRejectedCaddySwitchRestoresPreviousRouting(t *testing.T){
 st,err:=store.Open(t.TempDir());if err!=nil{t.Fatal(err)}
 st.AddProject(model.Project{ID:"p1",Name:"p1",Image:"nginx:alpine",Port:80})
 st.AddDomain(model.Domain{ID:"d1",Name:"test.example.org",ProjectID:"p1",Scheme:"https"})
 var mu sync.Mutex;calls:=0;rejectRestore:=false
 caddy:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  mu.Lock();defer mu.Unlock();calls++
  if calls==2||(rejectRestore&&calls==3){http.Error(w,"route update failed",503)}
 }))
 defer caddy.Close()
 e:=New("test",t.TempDir(),st,runtime.NewMock(),proxy.NewWithURL(caddy.URL))
 ctx,cancel:=context.WithCancel(context.Background());defer cancel();e.StartWorkers(ctx)
 first,_:=e.Enqueue("p1",model.JobDeploy)
 if j:=wait(t,st,first.ID);j.Status!=model.JobSucceeded{t.Fatal(j.Error)}
 before,_:=st.Project("p1")
 second,_:=e.Enqueue("p1",model.JobDeploy,"nginx:1.25")
 fail:=wait(t,st,second.ID)
 if fail.Status!=model.JobFailed||!strings.Contains(fail.Error,"previous route restored"){t.Fatalf("missing recovery: %+v",fail)}
 after,_:=st.Project("p1")
 if after.CurrentRelease!=before.CurrentRelease||after.Image!=before.Image||after.Status!=model.StatusRunning{t.Fatalf("active release changed: %+v",after)}
 mu.Lock();n:=calls;mu.Unlock();if n!=3{t.Fatalf("expected rejected switch and restoration, got %d requests",n)}
}

func TestFailedCaddyRestoreMarksProjectDegraded(t *testing.T){
 st,err:=store.Open(t.TempDir());if err!=nil{t.Fatal(err)}
 st.AddProject(model.Project{ID:"p1",Name:"p1",Image:"nginx:alpine",Port:80})
 st.AddDomain(model.Domain{ID:"d1",Name:"test.example.org",ProjectID:"p1",Scheme:"https"})
 var mu sync.Mutex;calls:=0
 caddy:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  mu.Lock();defer mu.Unlock();calls++;if calls>1{http.Error(w,"route update failed",503)}
 }))
 defer caddy.Close()
 e:=New("test",t.TempDir(),st,runtime.NewMock(),proxy.NewWithURL(caddy.URL))
 ctx,cancel:=context.WithCancel(context.Background());defer cancel();e.StartWorkers(ctx)
 first,_:=e.Enqueue("p1",model.JobDeploy)
 if j:=wait(t,st,first.ID);j.Status!=model.JobSucceeded{t.Fatal(j.Error)}
 before,_:=st.Project("p1")
 second,_:=e.Enqueue("p1",model.JobDeploy,"nginx:1.25")
 fail:=wait(t,st,second.ID)
 if fail.Status!=model.JobFailed||!strings.Contains(fail.Error,"manual recovery"){t.Fatalf("no uncertainty recorded: %+v",fail)}
 after,_:=st.Project("p1")
 if after.CurrentRelease!=before.CurrentRelease||after.Status!=model.StatusFailed{t.Fatalf("ambiguous route incorrectly marked running: %+v",after)}
 mu.Lock();n:=calls;mu.Unlock();if n!=3{t.Fatalf("expected 3 Caddy requests, got %d",n)}
}
