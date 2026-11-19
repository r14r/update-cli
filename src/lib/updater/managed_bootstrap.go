package updater

import (
 "context"
 "flag"
 "fmt"
 "io"
 "os"
 "path/filepath"
 "strings"
 "github.com/r14r/update-cli/src/lib/projectsetup"
)

func managedBootstrapCommand(ctx context.Context,args []string)(bool,error){
 special:=false
 for _,a:=range args{key,_,_:=strings.Cut(a,"=");switch key{case "--env","--envs","--check","--status","--dry-run","--force","--verbose","--list","--init":special=true}}
 // Leave old command line behavior unchanged unless opt-in configuration or a new flag is present.
 fs:=flag.NewFlagSet("bootstrap",flag.ContinueOnError);fs.SetOutput(io.Discard)
 env:=fs.String("env","","");envs:=fs.String("envs","","");rootArg:=fs.String("root","","");fs.StringVar(rootArg,"r","","")
 check:=fs.Bool("check",false,"");status:=fs.Bool("status",false,"");dry:=fs.Bool("dry-run",false,"");force:=fs.Bool("force",false,"");verbose:=fs.Bool("verbose",false,"");list:=fs.Bool("list",false,"");init:=fs.Bool("init",false,"")
 for _,name:=range []string{"no-ui","noui","no-wait","no-ask","no-color"}{fs.Bool(name,false,"")}
 if err:=fs.Parse(args);err!=nil{if special{return true,err};return false,nil}
 if len(fs.Args())>0{return special,fmt.Errorf("unexpected arguments")}
 if *list{fmt.Println("python (venv/pip)\nnode (nvm/npm)");return true,nil}
 if *env!=""&&*envs!=""{return true,fmt.Errorf("--env and --envs are mutually exclusive")}
 names:=[]string{};selection:=*env+*envs
 if selection!=""{seen:=map[string]bool{};for _,n:=range strings.Split(selection,","){n=strings.TrimSpace(n);if n!="python"&&n!="node"{return true,fmt.Errorf("unsupported environment %q",n)};if !seen[n]{names=append(names,n);seen[n]=true}}}
 root:=*rootArg;if root==""{var err error;root,err=os.Getwd();if err!=nil{return true,err}}
 root,err:=filepath.Abs(root);if err!=nil{return true,err}
 manifest:=filepath.Join(root,"update-cli.yaml")
 if _,err=os.Stat(manifest);os.IsNotExist(err){candidate:=filepath.Join(root,"current","update-cli.yaml");if _,e:=os.Stat(candidate);e==nil{manifest=candidate;root=filepath.Dir(candidate)}}
 var c projectsetup.BootstrapConfig
 if _,err=os.Stat(manifest);err==nil{c,err=projectsetup.ReadBootstrapConfig(manifest);if err!=nil{if special{return true,err};return false,nil}}else if !os.IsNotExist(err){return true,err}
 if !special&&!c.Configured{return false,nil}
 if *init{
  if c.Configured{return true,fmt.Errorf("bootstrap already configured; edit %s",manifest)}
  if len(names)==0{names=[]string{"python","node"}}
  block:=projectsetup.BootstrapYAML(names)
  if *dry||*check||*status{fmt.Print(block);return true,nil}
  old,e:=os.ReadFile(manifest);if os.IsNotExist(e){old=[]byte("schemaVersion: 2\n")}else if e!=nil{return true,e}
  return true,os.WriteFile(manifest,[]byte(strings.TrimRight(string(old),"\n")+"\n\n"+block),0644)
 }
 if !c.Configured{
  if len(names)==0{return true,fmt.Errorf("no managed bootstrap configuration; use --init or --env")}
  c=projectsetup.DefaultBootstrapConfig(names)
 }
 return true,projectsetup.RunManagedBootstrap(ctx,root,c,projectsetup.BootstrapOptions{Names:names,Check:*check,Status:*status,DryRun:*dry,Force:*force,Verbose:*verbose},os.Stdout)
}
