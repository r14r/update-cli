package projectsetup

import (
 "bytes"
 "context"
 "os"
 "path/filepath"
 "strings"
 "testing"
)

func TestManagedBootstrapConfig(t *testing.T) {
 root:=t.TempDir()
 p:=filepath.Join(root,"update-cli.yaml")
 if err:=os.WriteFile(p,[]byte("schemaVersion: 2\n"+BootstrapYAML([]string{"python","node"})),0600);err!=nil{t.Fatal(err)}
 c,err:=ReadBootstrapConfig(p)
 if err!=nil{t.Fatal(err)}
 if !c.Configured||c.Environments["python"].Version!="3.12"||c.Environments["node"].VersionManager!="nvm"{t.Fatalf("unexpected config: %+v",c)}
}

func TestManagedBootstrapDryRun(t *testing.T){
 root:=t.TempDir()
 c:=DefaultBootstrapConfig([]string{"python"})
 e:=c.Environments["python"];e.Path=".venv/python";c.Environments["python"]=e
 var out bytes.Buffer
 if err:=RunManagedBootstrap(context.Background(),root,c,BootstrapOptions{DryRun:true},&out);err!=nil{t.Fatal(err)}
 if _,err:=os.Stat(filepath.Join(root,".venv"));!os.IsNotExist(err){t.Fatal("dry-run created environment")}
 if _,err:=os.Stat(filepath.Join(root,".envrc"));!os.IsNotExist(err){t.Fatal("dry-run changed envrc")}
 if !strings.Contains(out.String(),"[CREATE]"){t.Fatalf("missing plan: %s",out.String())}
 if err:=RunManagedBootstrap(context.Background(),root,c,BootstrapOptions{Check:true},&out);err==nil{t.Fatal("missing environment passed check")}
}

func TestManagedEnvrcPreservesCustomContent(t *testing.T){
 root:=t.TempDir();p:=filepath.Join(root,".envrc")
 if err:=os.WriteFile(p,[]byte("export CUSTOM=1\n"),0600);err!=nil{t.Fatal(err)}
 block:=direnvBlock(root,map[string]string{"python":filepath.Join(root,"env","bin")})
 if changed,err:=managedEnvrc(root,block,false);err!=nil||!changed{t.Fatalf("changed=%v err=%v",changed,err)}
 if changed,err:=managedEnvrc(root,block,false);err!=nil||changed{t.Fatalf("not idempotent: changed=%v err=%v",changed,err)}
 b,err:=os.ReadFile(p);if err!=nil||!strings.Contains(string(b),"export CUSTOM=1")||strings.Count(string(b),beginEnvrc)!=1{t.Fatalf("unexpected envrc: %s %v",b,err)}
}

func TestManagedBootstrapRejectsInvalidVersion(t *testing.T){
 n,err:=parseSimpleYAML([]byte("environments:\n  node:\n    version: latest\n"))
 if err!=nil{t.Fatal(err)}
 if _,err=parseBootstrap(n);err==nil{t.Fatal("accepted invalid version")}
}
