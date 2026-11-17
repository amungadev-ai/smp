package proxy
import ("strings";"testing";"drevanta/engine/internal/model")
func TestMappedHostPortAndSkipUncommittedProject(t *testing.T){
 ds:=[]model.Domain{{ID:"1",Name:"app.example.com",ProjectID:"a",Scheme:"https"},{ID:"2",Name:"idle.example.com",ProjectID:"b",Scheme:"https"}}
 ps:=[]model.Project{{ID:"a",Port:80,CurrentRelease:"release-1",UpstreamPort:32768},{ID:"b",Port:8080}}
 cfg:=BuildCaddyfile(ds,ps)
 if !strings.Contains(cfg,"reverse_proxy 127.0.0.1:32768")||strings.Contains(cfg,"127.0.0.1:80")||strings.Contains(cfg,"idle.example.com"){t.Fatalf("unsafe route: %s",cfg)}
}

func TestDedicatedAdminRemainsOnOwnPort(t *testing.T){
 c:=NewWithURL("http://127.0.0.1:2021")
 cfg:=c.BuildConfig(nil,nil)
 if !strings.Contains(cfg,"admin 127.0.0.1:2021")||strings.Contains(cfg,"admin 127.0.0.1:2019"){t.Fatalf("dedicated admin listener changed: %s",cfg)}
}
