// Package jobs stages release-specific containers and switches Caddy only
// after health checks, without disrupting the previous running release.
package jobs

import (
 "context"
 "crypto/rand"
 "encoding/hex"
 "fmt"
 "path/filepath"
 "sync"
 "time"
 "drevanta/engine/internal/healthcheck"
 "drevanta/engine/internal/model"
 "drevanta/engine/internal/proxy"
 "drevanta/engine/internal/runtime"
 "drevanta/engine/internal/store"
)

const (
 pullTimeout=10*time.Minute
 startTimeout=2*time.Minute
 healthAttempts=20
 healthPause=500*time.Millisecond
 // Whole-config Caddy updates and deployments are serialized for safety.
 workerCount=1
 retainedContainers=2
)
type Engine struct{
 Version string
 DataDir string
 Store *store.Store
 Docker *runtime.Docker
 Caddy *proxy.Caddy
 queue chan string
 mu sync.Mutex
}
func New(version,dataDir string,st *store.Store,d *runtime.Docker,c *proxy.Caddy)*Engine{
 return &Engine{Version:version,DataDir:dataDir,Store:st,Docker:d,Caddy:c,queue:make(chan string,128)}
}
func (e *Engine) StartWorkers(ctx context.Context){
 for i:=0;i<workerCount;i++{go e.worker(ctx)}
 // Queued jobs resume; an interrupted running job is failed, never replayed
 // against live infrastructure without an explicit user retry.
 for _,j:=range e.Store.Jobs(""){
  if j.Status==model.JobRunning{e.fail(j.ID,"engine restarted while job was running; retry explicitly")}
  if j.Status==model.JobQueued{select{case e.queue<-j.ID:default:e.fail(j.ID,"recovery queue full")}}
 }
}
func newID()string{var b [8]byte;if _,err:=rand.Read(b[:]);err!=nil{return fmt.Sprintf("%d",time.Now().UnixNano())};return hex.EncodeToString(b[:])}
func (e *Engine) Enqueue(projectID,kind string,image ...string)(*model.Job,error){
 project,ok:=e.Store.Project(projectID);if !ok{return nil,fmt.Errorf("project %s not found",projectID)}
 if kind!=model.JobDeploy&&kind!=model.JobRollback&&kind!=model.JobRestart{return nil,fmt.Errorf("unknown job kind")}
 requested:=project.Image;if len(image)>0{requested=image[0]}
 e.mu.Lock();defer e.mu.Unlock()
 if len(e.queue)>=cap(e.queue){return nil,fmt.Errorf("deployment queue full")}
 job:=model.Job{ID:newID(),ProjectID:projectID,Kind:kind,Image:requested,Status:model.JobQueued,Created:model.Now()}
 if err:=e.Store.AddJob(job);err!=nil{return nil,err}
 e.queue<-job.ID;return &job,nil
}
func (e *Engine) worker(ctx context.Context){for{select{case <-ctx.Done():return;case id:=<-e.queue:e.run(ctx,id)}}}
func (e *Engine) jobLog(id,msg string){_=e.Store.UpdateJob(id,func(j *model.Job){j.Log=append(j.Log,model.JobLogLine{TS:model.Now(),Message:msg})})}
func (e *Engine) fail(id,msg string){_=e.Store.UpdateJob(id,func(j *model.Job){j.Status=model.JobFailed;j.Error=msg;j.Finished=model.Now();j.Log=append(j.Log,model.JobLogLine{TS:model.Now(),Message:"FAILED: "+msg})})}
func (e *Engine) succeed(id string){_=e.Store.UpdateJob(id,func(j *model.Job){j.Status=model.JobSucceeded;j.Finished=model.Now()})}
func (e *Engine) run(ctx context.Context,id string){
 if ctx.Err()!=nil{return}
 job,ok:=e.Store.Job(id);if !ok||job.Status!=model.JobQueued{return}
 if err:=e.Store.UpdateJob(id,func(j *model.Job){j.Status=model.JobRunning;j.Started=model.Now()});err!=nil{return}
 project,ok:=e.Store.Project(job.ProjectID);if !ok{e.fail(id,"project vanished");return}
 switch job.Kind{
 case model.JobDeploy:img:=job.Image;if img==""{img=project.Image};e.runDeploy(ctx,id,project,img,"")
 case model.JobRollback:e.runRollback(ctx,id,project)
 case model.JobRestart:e.runRestart(ctx,id,project)
 }
}
func splitImage(image string)(string,string){
 tag:="latest"
 for i:=len(image)-1;i>=0;i--{if image[i]=='/'{break};if image[i]==':'{return image[:i],image[i+1:]}}
 return image,tag
}
func fullImage(r model.Release)string{return r.Image+":"+r.Tag}
func (e *Engine) restorePreviousStatus(project model.Project){
 status:=model.StatusFailed;if project.CurrentRelease!=""{status=model.StatusRunning}
 _=e.Store.UpdateProject(project.ID,func(p *model.Project){p.Status=status})
}
// routingUncertain marks the project degraded if neither route switch nor
// restoration can be confirmed. Keep the candidate for manual recovery.
func (e *Engine) routingUncertain(jobID string,project model.Project,releaseID,msg string){
 _=e.Store.UpdateRelease(releaseID,func(r *model.Release){r.Status=model.ReleaseFailed})
 _=e.Store.UpdateProject(project.ID,func(p *model.Project){p.Status=model.StatusFailed})
 e.fail(jobID,msg)
}
func (e *Engine) failedRelease(jobID string,project model.Project,releaseID,msg string){
 _=e.Store.UpdateRelease(releaseID,func(r *model.Release){r.Status=model.ReleaseFailed})
 e.restorePreviousStatus(project);e.fail(jobID,msg)
}
func (e *Engine) runDeploy(ctx context.Context,jobID string,project model.Project,image,rollbackOf string){
 name,tag:=splitImage(image)
 release:=model.Release{ID:newID(),ProjectID:project.ID,Image:name,Tag:tag,Status:model.ReleaseDeploying,Created:model.Now(),RolledBackTo:rollbackOf}
 if err:=e.Store.AddRelease(release);err!=nil{e.fail(jobID,"persist release: "+err.Error());return}
 _=e.Store.UpdateJob(jobID,func(j *model.Job){j.ReleaseID=release.ID})
 _=e.Store.UpdateProject(project.ID,func(p *model.Project){p.Status=model.StatusDeploying})
 e.jobLog(jobID,"pulling "+image)
 pullCtx,cancel:=context.WithTimeout(ctx,pullTimeout)
 err:=e.Docker.Pull(pullCtx,image);cancel()
 if err!=nil{e.failedRelease(jobID,project,release.ID,"pull: "+err.Error());return}
 e.jobLog(jobID,"starting isolated release "+release.ID)
 runCtx,cancel:=context.WithTimeout(ctx,startTimeout)
 containerID,hostPort,err:=e.Docker.StartRelease(runCtx,project,release,image);cancel()
 if err!=nil{e.failedRelease(jobID,project,release.ID,"start: "+err.Error());return}
 // On failure, destroy only the new candidate; never delete the active one.
 cleanup:=true
 defer func(){if cleanup{
  cc,done:=context.WithTimeout(context.Background(),30*time.Second);defer done()
  if err:=e.Docker.StopRelease(cc,project.ID,release.ID);err!=nil{e.jobLog(jobID,"candidate cleanup warning: "+err.Error())}
 }}()
 if err:=e.Store.UpdateRelease(release.ID,func(r *model.Release){r.HostPort=hostPort;r.ContainerID=containerID});err!=nil{
  e.failedRelease(jobID,project,release.ID,"persist candidate: "+err.Error());return
 }
 e.jobLog(jobID,fmt.Sprintf("candidate on 127.0.0.1:%d; waiting for HTTP health",hostPort))
 healthy,latency,herr:=e.checkHealth(ctx,hostPort)
 if !healthy{
  _=e.Store.UpdateRelease(release.ID,func(r *model.Release){r.Status=model.ReleaseUnhealthy;r.HealthOK=&healthy})
  e.restorePreviousStatus(project);e.fail(jobID,"health check: "+safe(herr));return
 }
 _=e.Store.UpdateRelease(release.ID,func(r *model.Release){r.HealthOK=&healthy;r.HealthLatencyMS=latency})
 e.jobLog(jobID,fmt.Sprintf("candidate healthy in %d ms",latency))
 // Caddy sees the desired release target before the state marks it current.
 // On Caddy failure, the prior route remains in effect.
 previousProjects:=e.Store.Projects()
 desired:=append([]model.Project(nil),previousProjects...)
 for i:=range desired{if desired[i].ID==project.ID{desired[i].UpstreamPort=hostPort;desired[i].CurrentRelease=release.ID}}
 domains:=e.Store.Domains()
 if err:=e.Caddy.ApplyConfig(domains,desired);err!=nil{
  // An HTTP failure does not prove Caddy left its route unchanged.
  // Restore the known-good configuration before removing the candidate.
  if restoreErr:=e.Caddy.ApplyConfig(domains,previousProjects);restoreErr!=nil{
   cleanup=false
   e.routingUncertain(jobID,project,release.ID,"Caddy switch rejected AND restoration failed; candidate retained for manual recovery: "+err.Error()+"; "+restoreErr.Error());return
  }
  e.failedRelease(jobID,project,release.ID,"Caddy routing rejected; previous route restored: "+err.Error());return
 }
 e.jobLog(jobID,"Caddy accepted candidate routing; committing release")
 if err:=e.Store.ActivateRelease(project.ID,release.ID,image,hostPort,rollbackOf);err!=nil{
  // If route recovery fails, the candidate may still serve traffic. Keep it!
  if restoreErr:=e.Caddy.ApplyConfig(domains,previousProjects);restoreErr!=nil{
   cleanup=false
   e.routingUncertain(jobID,project,release.ID,"persistence failed AND route recovery failed; candidate retained for manual recovery: "+err.Error()+"; "+restoreErr.Error());return
  }
  e.failedRelease(jobID,project,release.ID,err.Error());return
 }
 cleanup=false
 e.jobLog(jobID,"active release changed; prior healthy release remained running during switch")
 e.succeed(jobID)
 e.reapOldReleases(project.ID,release.ID,jobID)
 if e.Caddy.Mode()==proxy.ModeCaddy&&e.DataDir!=""{
  cfg:=proxy.BuildCaddyfile(domains,e.Store.Projects())
  if err:=writeFile(filepath.Join(e.DataDir,"Caddyfile"),[]byte(cfg));err!=nil{
   e.jobLog(jobID,"Caddyfile mirror warning: "+err.Error())
  }
 }
}
func (e *Engine) reapOldReleases(projectID,activeID,jobID string){
 releases:=e.Store.Releases(projectID);kept:=0
 for i:=len(releases)-1;i>=0;i--{
  r:=releases[i]
  if r.Status!=model.ReleaseHealthy&&r.Status!=model.ReleaseRolledBack{continue}
  kept++
  if kept<=retainedContainers||r.ID==activeID{continue}
  cc,done:=context.WithTimeout(context.Background(),30*time.Second)
  err:=e.Docker.StopRelease(cc,projectID,r.ID);done()
  if err!=nil{e.jobLog(jobID,"old release cleanup warning: "+err.Error())}
 }
}
func (e *Engine) runRollback(ctx context.Context,jobID string,project model.Project){
 if project.CurrentRelease==""{e.fail(jobID,"no active release to roll back to");return}
 releases:=e.Store.Releases(project.ID)
 for i:=len(releases)-1;i>=0;i--{
  r:=releases[i]
  if r.ID==project.CurrentRelease||r.Status!=model.ReleaseHealthy&&r.Status!=model.ReleaseRolledBack{continue}
  e.jobLog(jobID,"rolling back to "+fullImage(r)+" from release "+r.ID)
  e.runDeploy(ctx,jobID,project,fullImage(r),r.ID);return
 }
 e.fail(jobID,"no previous healthy release to roll back to")
}
func (e *Engine) runRestart(ctx context.Context,jobID string,project model.Project){
 e.jobLog(jobID,"restarting active release "+project.CurrentRelease)
 port,err:=e.Docker.RestartRelease(ctx,project)
 if err!=nil{e.fail(jobID,"restart: "+err.Error());return}
 if port!=project.UpstreamPort&&e.Docker.Mode()!=runtime.ModeMock{
  e.fail(jobID,"restarted container changed host port; routing requires repair");return
 }
 healthy,latency,err:=e.checkHealth(ctx,port)
 if !healthy{e.fail(jobID,"restart health check: "+safe(err));return}
 e.jobLog(jobID,fmt.Sprintf("restart healthy in %d ms",latency));e.succeed(jobID)
}
func (e *Engine) checkHealth(ctx context.Context,hostPort int)(bool,int64,error){
 if e.Docker.Mode()==runtime.ModeMock{
  select{case <-ctx.Done():return false,0,ctx.Err();case <-time.After(100*time.Millisecond):return true,12,nil}
 }
 hCtx,cancel:=context.WithTimeout(ctx,healthAttempts*healthPause+5*time.Second);defer cancel()
 return healthcheck.WaitUntilHealthy(hCtx,hostPort,healthAttempts,healthPause)
}
func safe(err error)string{if err==nil{return "unknown error"};return err.Error()}
