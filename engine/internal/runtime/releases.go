package runtime

// Release-scoped Docker operations. New image releases never reuse the current
// container, and all published ports bind to loopback only.
import (
 "bytes"
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "net"
 "net/http"
 "strconv"
 "strings"
 "time"
 "drevanta/engine/internal/model"
)

const labelProject="drevanta.project.id"
const labelRelease="drevanta.release.id"
func ReleaseContainerName(projectID,releaseID string)string{return "drevanta-release-"+projectID+"-"+releaseID}
func NewWithSocket(socket string)*Docker{
 d:=&Docker{mode:ModeDocker,sock:socket,logFollows:map[string]context.CancelFunc{},logSubs:map[string][]chan model.LogLine{}}
 d.client=&http.Client{Transport:&http.Transport{DialContext:func(ctx context.Context,_,_ string)(net.Conn,error){return (&net.Dialer{}).DialContext(ctx,"unix",socket)}}}
 return d
}
func dockerError(resp *http.Response,action string)error{msg,_:=io.ReadAll(io.LimitReader(resp.Body,1024));return fmt.Errorf("docker %s returned %d: %s",action,resp.StatusCode,strings_Sanitize(string(msg)))}
func (d *Docker) inspectRelease(ctx context.Context,projectID,releaseID string)(string,int,bool,error){
 resp,err:=d.callCtx(ctx,http.MethodGet,"/containers/"+ReleaseContainerName(projectID,releaseID)+"/json",nil,10*time.Second)
 if err!=nil{return "",0,false,err};defer resp.Body.Close()
 if resp.StatusCode==http.StatusNotFound{return "",0,false,errors.New("release container not found")}
 if resp.StatusCode!=http.StatusOK{return "",0,false,dockerError(resp,"inspect")}
 var v struct{
  ID string `json:"Id"`
  Config struct{Labels map[string]string `json:"Labels"`} `json:"Config"`
  State struct{Running bool `json:"Running"`} `json:"State"`
  NetworkSettings struct{Ports map[string][]struct{HostIP string `json:"HostIp"`;HostPort string `json:"HostPort"`} `json:"Ports"`} `json:"NetworkSettings"`
 }
 if err:=json.NewDecoder(io.LimitReader(resp.Body,1<<20)).Decode(&v);err!=nil{return "",0,false,err}
 if v.Config.Labels[labelProject]!=projectID||v.Config.Labels[labelRelease]!=releaseID{return "",0,false,errors.New("container ownership labels mismatch")}
 port:=0
 for _,bindings:=range v.NetworkSettings.Ports{for _,b:=range bindings{if b.HostIP!="127.0.0.1"{return "",0,false,errors.New("container port is not bound to loopback")};p,err:=strconv.Atoi(b.HostPort);if err==nil&&p>0&&p<65536{port=p}}}
 if v.State.Running&&port==0{return "",0,false,errors.New("no mapped loopback host port")}
 return v.ID,port,v.State.Running,nil
}
func (d *Docker) StartRelease(ctx context.Context,project model.Project,release model.Release,image string)(string,int,error){
 if d.mode==ModeMock{select{case <-ctx.Done():return "",0,ctx.Err();case <-time.After(100*time.Millisecond):};return "mock-"+release.ID,project.Port,nil}
 if project.Port<1||project.Port>65535{return "",0,errors.New("invalid container port")}
 d.mu.Lock();defer d.mu.Unlock()
 name:=ReleaseContainerName(project.ID,release.ID);key:=fmt.Sprintf("%d/tcp",project.Port)
 config:=map[string]any{"Image":image,"ExposedPorts":map[string]any{key:map[string]any{}},"HostConfig":map[string]any{"AutoRemove":false,"PortBindings":map[string]any{key:[]map[string]string{{"HostIp":"127.0.0.1","HostPort":""}}}},"Labels":map[string]string{labelProject:project.ID,labelRelease:release.ID}}
 raw,err:=json.Marshal(config);if err!=nil{return "",0,err}
 resp,err:=d.callCtx(ctx,http.MethodPost,"/containers/create?name="+name,bytes.NewReader(raw),30*time.Second)
 if err!=nil{return "",0,err}
 if resp.StatusCode!=http.StatusCreated{defer resp.Body.Close();return "",0,dockerError(resp,"create")}
 var created struct{ID string `json:"Id"`}
 err=json.NewDecoder(io.LimitReader(resp.Body,1024)).Decode(&created);resp.Body.Close()
 if err!=nil||created.ID==""{return "",0,fmt.Errorf("docker create returned invalid container ID: %v",err)}
 success:=false
 defer func(){if !success{cleanup,cancel:=context.WithTimeout(context.Background(),30*time.Second);defer cancel();_=d.removeCreated(cleanup,created.ID)}}()
 resp,err=d.callCtx(ctx,http.MethodPost,"/containers/"+created.ID+"/start",nil,45*time.Second)
 if err!=nil{return "",0,err}
 if resp.StatusCode!=http.StatusNoContent&&resp.StatusCode!=http.StatusOK{defer resp.Body.Close();return "",0,dockerError(resp,"start")};resp.Body.Close()
 id,port,running,err:=d.inspectRelease(ctx,project.ID,release.ID)
 if err!=nil{return "",0,err};if !running||port==0{return "",0,errors.New("release container not running on loopback")}
 success=true;return id,port,nil
}
func (d *Docker) removeCreated(ctx context.Context,id string)error{
 resp,err:=d.callCtx(ctx,http.MethodDelete,"/containers/"+id+"?force=1",nil,30*time.Second)
 if err!=nil{return err};defer resp.Body.Close()
 if resp.StatusCode!=http.StatusNoContent&&resp.StatusCode!=http.StatusNotFound{return dockerError(resp,"remove")};return nil
}
func (d *Docker) StopRelease(ctx context.Context,projectID,releaseID string)error{
 if d.mode==ModeMock{return nil}
 d.mu.Lock();defer d.mu.Unlock()
 id,_,_,err:=d.inspectRelease(ctx,projectID,releaseID)
 if err!=nil{if strings.Contains(err.Error(),"not found"){return nil};return err}
 return d.removeCreated(ctx,id)
}
func (d *Docker) RestartRelease(ctx context.Context,project model.Project)(int,error){
 if project.CurrentRelease==""{return 0,errors.New("no active release")}
 if d.mode==ModeMock{return project.UpstreamPort,nil}
 d.mu.Lock();defer d.mu.Unlock()
 id,_,running,err:=d.inspectRelease(ctx,project.ID,project.CurrentRelease);if err!=nil{return 0,err}
 route:="/containers/"+id+"/restart?t=10";if !running{route="/containers/"+id+"/start"}
 resp,err:=d.callCtx(ctx,http.MethodPost,route,nil,60*time.Second);if err!=nil{return 0,err};defer resp.Body.Close()
 if resp.StatusCode!=http.StatusNoContent&&resp.StatusCode!=http.StatusOK{return 0,dockerError(resp,"restart")}
 _,port,running,err:=d.inspectRelease(ctx,project.ID,project.CurrentRelease)
 if err!=nil{return 0,err};if !running{return 0,errors.New("release is not running")};return port,nil
}
