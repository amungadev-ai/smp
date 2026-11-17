// drevanta-maint backs up, verifies or restores a single version-1 state.json.
// Restore must only run while the Go engine is stopped.
package main

import(
 "flag"
 "fmt"
 "os"
 "drevanta/engine/internal/maintenance"
)

func main(){
 if len(os.Args)<2{fmt.Fprintln(os.Stderr,"Usage: drevanta-maint backup|verify|restore --archive FILE [--state STATE] [--confirm-offline]");os.Exit(2)}
 mode:=os.Args[1]
 fs:=flag.NewFlagSet(mode,flag.ExitOnError)
 archive:=fs.String("archive","","ZIP archive path")
 state:=fs.String("state","/var/lib/drevanta-engine/state.json","engine state.json file")
 confirm:=fs.Bool("confirm-offline",false,"confirm engine is stopped before restore")
 if err:=fs.Parse(os.Args[2:]);err!=nil{fmt.Fprintln(os.Stderr,err);os.Exit(2)}
 if *archive==""||fs.NArg()!=0{fmt.Fprintln(os.Stderr,"--archive FILE required and no positional arguments allowed");os.Exit(2)}
 switch mode{
 case "backup":
  m,err:=maintenance.Create(*state,*archive)
  if err!=nil{fatal(err)}
  fmt.Printf("Backup created: %s (%d bytes, SHA256 %s). No administrator password included.\n",*archive,m.StateSize,m.StateSHA256)
 case "verify":
  m,err:=maintenance.Verify(*archive)
  if err!=nil{fatal(err)}
  fmt.Printf("Backup verified: version %d, %d bytes, SHA256 %s\n",m.Format,m.StateSize,m.StateSHA256)
 case "restore":
  if !*confirm{fatal(fmt.Errorf("restore requires --confirm-offline; stop engine first"))}
  pre,err:=maintenance.Restore(*archive,*state)
  if err!=nil{fatal(err)}
  fmt.Printf("Restored engine JSON state from %s.\n",*archive)
  if pre!=""{fmt.Printf("Previous state retained at %s\n",pre)}
  fmt.Println("Administrator credentials were not changed; engine must be restarted manually.")
 default:
  fmt.Fprintln(os.Stderr,"Unknown operation:",mode)
  os.Exit(2)
 }
}
func fatal(err error){fmt.Fprintln(os.Stderr,"drevanta-maint:",err);os.Exit(1)}
