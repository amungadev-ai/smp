package store
import ("fmt";"drevanta/engine/internal/model")
// ActivateRelease atomically saves the active release and its mapped host port.
// Failed persistence rolls the in-memory state back as well.
func (s *Store) ActivateRelease(projectID,releaseID,image string,port int,rollbackOf string)error{
 if port<1||port>65535{return fmt.Errorf("invalid upstream port %d",port)}
 s.mu.Lock();defer s.mu.Unlock()
 prior:=s.state;next:=prior
 next.Projects=append([]model.Project(nil),prior.Projects...)
 next.Releases=append([]model.Release(nil),prior.Releases...)
 next.Domains=append([]model.Domain(nil),prior.Domains...)
 pi,ri:=-1,-1
 for i,p:=range next.Projects{if p.ID==projectID{pi=i;break}}
 for i,r:=range next.Releases{if r.ID==releaseID&&r.ProjectID==projectID{ri=i;break}}
 if pi<0||ri<0{return fmt.Errorf("project or release missing")}
 p:=&next.Projects[pi];p.CurrentRelease=releaseID;p.UpstreamPort=port;p.Image=image;p.Status=model.StatusRunning
 p.ReleaseCount=0;for _,r:=range next.Releases{if r.ProjectID==projectID{p.ReleaseCount++}}
 r:=&next.Releases[ri];r.Status=model.ReleaseHealthy;r.RolledBackTo=rollbackOf;healthy:=true;r.HealthOK=&healthy
 for i:=range next.Domains{if next.Domains[i].ProjectID==projectID{next.Domains[i].Status=model.DomainActive}}
 s.state=next
 if err:=s.saveLocked();err!=nil{s.state=prior;return fmt.Errorf("persist activation: %w",err)}
 return nil
}
