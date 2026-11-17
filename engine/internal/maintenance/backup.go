// Package maintenance provides bounded, offline-recoverable snapshots of the
// engine JSON state. Backups DO NOT contain the admin password or runtime files.
package maintenance

import (
 "archive/zip"
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "os"
 "path/filepath"
 "time"

 "drevanta/engine/internal/model"
)

const MaxStateBytes int64=16<<20
const archiveMaxBytes int64=MaxStateBytes+1<<20
const manifestMaxBytes int64=4096

type Manifest struct{
 Format int `json:"format"`
 CreatedAt string `json:"created_at"`
 StateSHA256 string `json:"state_sha256"`
 StateSize int64 `json:"state_size"`
}

func checkState(raw []byte)error{
 if len(raw)==0||int64(len(raw))>MaxStateBytes{return errors.New("state is empty or exceeds 16 MiB")}
 var state model.State
 if err:=json.Unmarshal(raw,&state);err!=nil{return fmt.Errorf("invalid state JSON: %w",err)}
 if state.Version!=1{return fmt.Errorf("unsupported engine state version %d",state.Version)}
 return nil
}

func readRegular(path string,limit int64)([]byte,error){
 info,err:=os.Lstat(path)
 if err!=nil{return nil,err}
 if !info.Mode().IsRegular(){return nil,errors.New("regular file required (symlinks are forbidden)")}
 if info.Size()<0||info.Size()>limit{return nil,errors.New("file exceeds size limit")}
 raw,err:=os.ReadFile(path)
 if err!=nil{return nil,err}
 if int64(len(raw))>limit{return nil,errors.New("file exceeds size limit")}
 return raw,nil
}

func ReadState(path string)([]byte,error){
 if filepath.Base(path)!="state.json"{return nil,errors.New("expected state.json")}
 raw,err:=readRegular(path,MaxStateBytes)
 if err!=nil{return nil,err}
 if err:=checkState(raw);err!=nil{return nil,err}
 return raw,nil
}

func Create(statePath,archivePath string)(Manifest,error){
 var m Manifest
 raw,err:=ReadState(statePath)
 if err!=nil{return m,err}
 digest:=sha256.Sum256(raw)
 m=Manifest{Format:1,CreatedAt:time.Now().UTC().Format(time.RFC3339),StateSHA256:hex.EncodeToString(digest[:]),StateSize:int64(len(raw))}
 info,err:=json.Marshal(m)
 if err!=nil{return m,err}
 file,err:=os.OpenFile(archivePath,os.O_CREATE|os.O_EXCL|os.O_WRONLY,0600)
 if err!=nil{return m,err}
 success:=false
 defer func(){if !success{_ = os.Remove(archivePath)}}()
 zw:=zip.NewWriter(file)
 for _,entry:=range []struct{name string;payload []byte}{{"state.json",raw},{"manifest.json",info}}{
  header:=&zip.FileHeader{Name:entry.name,Method:zip.Store}
  header.SetMode(0600)
  w,writeErr:=zw.CreateHeader(header)
  if writeErr!=nil{_ = zw.Close();_ = file.Close();return m,writeErr}
  if _,writeErr=w.Write(entry.payload);writeErr!=nil{_ = zw.Close();_ = file.Close();return m,writeErr}
 }
 if err:=zw.Close();err!=nil{_ = file.Close();return m,err}
 if err:=file.Sync();err!=nil{_ = file.Close();return m,err}
 if err:=file.Close();err!=nil{return m,err}
 success=true
 return m,nil
}

func openArchive(path string)([]byte,Manifest,error){
 var manifest Manifest
 if _,err:=readRegular(path,archiveMaxBytes);err!=nil{return nil,manifest,err}
 zr,err:=zip.OpenReader(path)
 if err!=nil{return nil,manifest,err}
 defer zr.Close()
 if len(zr.File)!=2{return nil,manifest,errors.New("expected exactly two backup entries")}
 entries:=make(map[string][]byte,2)
 for _,f:=range zr.File{
  var limit int64
  switch f.Name{
  case "state.json":limit=MaxStateBytes
  case "manifest.json":limit=manifestMaxBytes
  default:return nil,manifest,fmt.Errorf("unexpected archive entry %q",f.Name)
  }
  if _,exists:=entries[f.Name];exists{return nil,manifest,errors.New("duplicate archive entry")}
  if f.FileInfo().IsDir()||f.Mode()&os.ModeSymlink!=0||int64(f.UncompressedSize64)>limit{return nil,manifest,errors.New("unsafe or oversized archive entry")}
  rc,err:=f.Open()
  if err!=nil{return nil,manifest,err}
  raw,err:=io.ReadAll(io.LimitReader(rc,limit+1))
  closeErr:=rc.Close()
  if err!=nil{return nil,manifest,err}
  if closeErr!=nil{return nil,manifest,closeErr}
  if int64(len(raw))>limit{return nil,manifest,errors.New("archive entry exceeds limit")}
  entries[f.Name]=raw
 }
 raw,ok:=entries["state.json"]
 if !ok{return nil,manifest,errors.New("missing state.json")}
 meta,ok:=entries["manifest.json"]
 if !ok{return nil,manifest,errors.New("missing manifest.json")}
 if err:=json.Unmarshal(meta,&manifest);err!=nil{return nil,manifest,err}
 if manifest.Format!=1||manifest.StateSize!=int64(len(raw)){return nil,manifest,errors.New("backup manifest format or size mismatch")}
 digest:=sha256.Sum256(raw)
 if manifest.StateSHA256!=hex.EncodeToString(digest[:]){return nil,manifest,errors.New("state hash mismatch")}
 if err:=checkState(raw);err!=nil{return nil,manifest,err}
 return raw,manifest,nil
}

func Verify(archivePath string)(Manifest,error){
 _,m,err:=openArchive(archivePath)
 return m,err
}

// Restore atomically replaces state.json after validating the complete archive.
// The caller MUST ensure the engine process is stopped. An existing state file
// is copied to a private pre-restore recovery file in the same directory.
func Restore(archivePath,statePath string)(string,error){
 if filepath.Base(statePath)!="state.json"{return "",errors.New("restore target must be state.json")}
 raw,_,err:=openArchive(archivePath)
 if err!=nil{return "",err}
 dir:=filepath.Dir(statePath)
 dirInfo,err:=os.Lstat(dir)
 if err!=nil{return "",err}
 if !dirInfo.IsDir(){return "",errors.New("state directory must not be a symlink")}
 old,err:=readRegular(statePath,MaxStateBytes)
 if err!=nil&&!errors.Is(err,os.ErrNotExist){return "",err}
 if err!=nil{old=nil}
 tmp,err:=os.CreateTemp(dir,".drevanta-restoring-*")
 if err!=nil{return "",err}
 tmpPath:=tmp.Name()
 defer os.Remove(tmpPath)
 if _,err=tmp.Write(raw);err!=nil{tmp.Close();return "",err}
 if err=tmp.Sync();err!=nil{tmp.Close();return "",err}
 if err=tmp.Close();err!=nil{return "",err}
 recovery:=""
 if old!=nil{
  prev,createErr:=os.CreateTemp(dir,".drevanta-pre-restore-*")
  if createErr!=nil{return "",createErr}
  if _,createErr=prev.Write(old);createErr!=nil{prev.Close();os.Remove(prev.Name());return "",createErr}
  if createErr=prev.Sync();createErr!=nil{prev.Close();os.Remove(prev.Name());return "",createErr}
  if createErr=prev.Close();createErr!=nil{os.Remove(prev.Name());return "",createErr}
  recovery=prev.Name()
 }
 if err=os.Rename(tmpPath,statePath);err!=nil{return recovery,err}
 if d,openErr:=os.Open(dir);openErr==nil{_ = d.Sync();_ = d.Close()}
 return recovery,nil
}
