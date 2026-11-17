package maintenance

import(
 "archive/zip"
 "bytes"
 "encoding/json"
 "os"
 "path/filepath"
 "strings"
 "testing"

 "drevanta/engine/internal/model"
)

func stateBytes(t *testing.T,name string)[]byte{
 t.Helper()
 raw,err:=json.Marshal(model.State{Version:1,Projects:[]model.Project{{ID:name,Name:name}}})
 if err!=nil{t.Fatal(err)}
 return raw
}

func TestCreateVerifyRestore(t *testing.T){
 dir:=t.TempDir();path:=filepath.Join(dir,"state.json");arc:=filepath.Join(dir,"backup.zip")
 initial:=stateBytes(t,"original")
 if err:=os.WriteFile(path,initial,0600);err!=nil{t.Fatal(err)}
 manifest,err:=Create(path,arc)
 if err!=nil{t.Fatal(err)}
 if manifest.StateSize!=int64(len(initial)){t.Fatal("size mismatch")}
 if _,err:=Create(path,arc);err==nil{t.Fatal("archive overwritten")}
 verified,err:=Verify(arc)
 if err!=nil||verified.StateSHA256!=manifest.StateSHA256{t.Fatalf("archive failed verification: %v",err)}
 updated:=stateBytes(t,"newer")
 if err:=os.WriteFile(path,updated,0600);err!=nil{t.Fatal(err)}
 recovery,err:=Restore(arc,path)
 if err!=nil{t.Fatal(err)}
 if recovery==""{t.Fatal("no previous state copy retained")}
 actual,err:=os.ReadFile(path)
 if err!=nil||!bytes.Equal(actual,initial){t.Fatalf("restore incorrect: %v",err)}
 backupBefore,err:=os.ReadFile(recovery)
 if err!=nil||!bytes.Equal(backupBefore,updated){t.Fatal("pre-restore state not preserved")}
 info,err:=os.Stat(path)
 if err!=nil||info.Mode().Perm()!=0600{t.Fatal("restored state not private")}
 archiveInfo,err:=os.Stat(arc)
 if err!=nil||archiveInfo.Mode().Perm()!=0600{t.Fatal("backup archive not private")}
}

func TestRejectTamperingAndUnsafeInputs(t *testing.T){
 dir:=t.TempDir();state:=filepath.Join(dir,"state.json")
 if err:=os.WriteFile(state,stateBytes(t,"first"),0600);err!=nil{t.Fatal(err)}
 arc:=filepath.Join(dir,"a.zip")
 if _,err:=Create(state,arc);err!=nil{t.Fatal(err)}
 symlink:=filepath.Join(dir,"shortcut.zip")
 if err:=os.Symlink(arc,symlink);err!=nil{t.Skip(err)}
 if _,err:=Verify(symlink);err==nil{t.Fatal("archive symlink accepted")}
 if err:=os.Symlink(state,filepath.Join(dir,"linked-state.json"));err!=nil{t.Skip(err)}
 if _,err:=ReadState(filepath.Join(dir,"linked-state.json"));err==nil{t.Fatal("non-state target accepted")}
 raw,err:=os.ReadFile(arc);if err!=nil{t.Fatal(err)}
 // Stored ZIP payload corruption must fail CRC, SHA256 or JSON validation.
 broken:=bytes.Replace(raw,[]byte("first"),[]byte("later"),1)
 if bytes.Equal(broken,raw){t.Fatal("test setup failed")}
 brokenPath:=filepath.Join(dir,"tampered.zip")
 if err:=os.WriteFile(brokenPath,broken,0600);err!=nil{t.Fatal(err)}
 if _,err:=Verify(brokenPath);err==nil{t.Fatal("tampered archive accepted")}
 // Invalid versions cannot be archived or restored.
 bad:=strings.Replace(string(stateBytes(t,"first")),`"version":1`,`"version":99`,1)
 if err:=os.WriteFile(state,[]byte(bad),0600);err!=nil{t.Fatal(err)}
 if _,err:=Create(state,filepath.Join(dir,"bad.zip"));err==nil{t.Fatal("future state version accepted")}
}

func TestRejectArchiveWithExtraEntry(t *testing.T){
 dir:=t.TempDir();path:=filepath.Join(dir,"extra.zip")
 f,err:=os.Create(path);if err!=nil{t.Fatal(err)}
 w:=zip.NewWriter(f)
 for _,name:=range []string{"state.json","manifest.json","../escape"}{
  entry,err:=w.Create(name);if err!=nil{t.Fatal(err)}
  if _,err:=entry.Write([]byte("{}"));err!=nil{t.Fatal(err)}
 }
 if err:=w.Close();err!=nil{t.Fatal(err)}
 if err:=f.Close();err!=nil{t.Fatal(err)}
 if _,err:=Verify(path);err==nil{t.Fatal("unsafe extra entry accepted")}
}

func TestStateLimitAndRestoreTarget(t *testing.T){
 dir:=t.TempDir();state:=filepath.Join(dir,"state.json")
 if err:=os.WriteFile(state,bytes.Repeat([]byte("z"),int(MaxStateBytes)+1),0600);err!=nil{t.Fatal(err)}
 if _,err:=Create(state,filepath.Join(dir,"oversized.zip"));err==nil{t.Fatal("oversized state accepted")}
 if _,err:=Restore(filepath.Join(dir,"missing.zip"),filepath.Join(dir,"wrong.json"));err==nil{t.Fatal("invalid restore target accepted")}
}
