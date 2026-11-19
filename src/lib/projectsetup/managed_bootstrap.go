package projectsetup

import (
 "context"
 "crypto/sha256"
 "fmt"
 "io"
 "os"
 "os/exec"
 "path/filepath"
 "runtime"
 "strings"
)

type BootstrapOptions struct {
 Names []string
 Check, Status, DryRun, Force, Verbose bool
}

type bootstrapRunner struct {ctx context.Context; out io.Writer; options BootstrapOptions; root string}

func (r bootstrapRunner) command(dir string, env []string, name string, args ...string) *exec.Cmd {
 if r.options.Verbose {fmt.Fprintf(r.out,"[EXEC] %s %q\n",name,args)}
 c:=exec.CommandContext(r.ctx,name,args...);c.Dir=dir;c.Env=append(os.Environ(),env...);return c
}
func (r bootstrapRunner) run(dir string, env []string, name string,args ...string) error {
 c:=r.command(dir,env,name,args...);c.Stdout=r.out;c.Stderr=r.out;return c.Run()
}
func (r bootstrapRunner) valid(dir string,env []string,name string,args ...string) bool {
 return r.command(dir,env,name,args...).Run()==nil
}
func (r bootstrapRunner) readonly() bool {return r.options.Check||r.options.Status||r.options.DryRun}
func absFrom(root,path string) string {if filepath.IsAbs(path){return filepath.Clean(path)};return filepath.Clean(filepath.Join(root,path))}
func fileExists(p string) bool {i,e:=os.Stat(p);return e==nil&&!i.IsDir()}
func dirExists(p string) bool {i,e:=os.Stat(p);return e==nil&&i.IsDir()}
func shellQuote(s string) string {return "'"+strings.ReplaceAll(s,"'","'\\''")+"'"}
func versionMatches(actual,wanted string) bool {return actual==wanted||strings.HasPrefix(actual,wanted+".")}

func RunManagedBootstrap(ctx context.Context, root string, config BootstrapConfig, options BootstrapOptions, out io.Writer) error {
 root,err:=filepath.Abs(root);if err!=nil{return err}
 names:=options.Names
 if len(names)==0 {for _,n:=range []string{"python","node"} {if e,ok:=config.Environments[n];ok&&e.Enabled {names=append(names,n)}}}
 r:=bootstrapRunner{ctx:ctx,out:out,options:options,root:root}
 bins:=map[string]string{};pending:=false
 for _,name:=range names {
  e,ok:=config.Environments[name];if !ok||!e.Enabled{return fmt.Errorf("environment %s not enabled",name)}
  path:=absFrom(root,e.Path);work:=absFrom(root,e.WorkingDirectory)
  if !dirExists(work){return fmt.Errorf("working directory missing: %s",work)}
  fmt.Fprintf(out,"[%s] %s (%s)\n",strings.ToUpper(name),path,e.Version)
  var bin string;var needs bool
  switch name {case "python":bin,needs,err=r.python(e,path,work);case "node":bin,needs,err=r.node(e,path,work);default:return fmt.Errorf("unsupported environment %s",name)}
  if err!=nil{return fmt.Errorf("%s bootstrap: %w",name,err)}
  bins[name]=bin;pending=pending||needs
 }
 if config.DirenvEnabled&&config.DirenvGenerate&&len(names)>0 {
  // Render all configured environments, even when only one is selected.
  for name,e:=range config.Environments {if !e.Enabled{continue};if _,ok:=bins[name];ok{continue};p:=absFrom(root,e.Path);if name=="python"{bins[name]=pythonBin(p)}else{bins[name]=filepath.Join(p,"versions","node","v"+e.Version,"bin")}}
  content:=direnvBlock(root,bins)
  changed,e:=managedEnvrc(root,content,r.readonly());if e!=nil{return e};pending=pending||changed&&r.readonly()
  if changed {fmt.Fprintln(out,"[DIRENV] managed .envrc block "+map[bool]string{true:"needs update",false:"updated"}[r.readonly()])}
 }
 if pending&&options.Check{return fmt.Errorf("bootstrap requirements not satisfied")}
 return nil
}

func pythonBin(path string) string {if runtime.GOOS=="windows"{return filepath.Join(path,"Scripts")};return filepath.Join(path,"bin")}
func (r bootstrapRunner) python(e BootstrapEnvironment,path,work string)(string,bool,error){
 bin:=pythonBin(path);py:=filepath.Join(bin,"python");if runtime.GOOS=="windows"{py+=".exe"}
 versionCode:="import sys; print('.'.join(map(str,sys.version_info[:3]))); assert sys.prefix != sys.base_prefix"
 if fileExists(py){
  b,err:=r.command(work,nil,py,"-c",versionCode).Output()
  if err!=nil||!versionMatches(strings.TrimSpace(string(b)),e.Version){return bin,true,fmt.Errorf("existing venv invalid or wrong version; preserve it and select another path: %s",path)}
 } else {
  if _,err:=os.Lstat(path);!os.IsNotExist(err){return bin,true,fmt.Errorf("existing environment has no interpreter: %s",path)}
  fmt.Fprintln(r.out,"[CREATE] Python venv")
  if r.readonly(){return bin,true,nil}
  parts:=strings.Split(e.Version,".");base:="python"+strings.Join(parts[:2],".")
  b,err:=r.command(work,nil,base,"-c","import sys; print('.'.join(map(str,sys.version_info[:3])))").Output()
  if err!=nil||!versionMatches(strings.TrimSpace(string(b)),e.Version){return bin,true,fmt.Errorf("install Python %s first",e.Version)}
  if err=r.run(work,nil,base,"-m","venv",path);err!=nil{return bin,true,err}
 }
 reqs:=e.Requirements
 if len(reqs)==0&&fileExists(filepath.Join(work,"requirements.txt")){reqs=[]string{"requirements.txt"}}
 args:=[]string{"-m","pip","install"};files:=[]string{}
 for _,p:=range reqs{p=absFrom(work,p);files=append(files,p);args=append(args,"-r",p)}
 digest,err:=dependencyDigest(files,e.Version);if err!=nil{return bin,true,err}
 stamp:=filepath.Join(path,".update-cli-requirements")
 old,_:=os.ReadFile(stamp)
 // pip check validates installed dependency consistency; direct requirements
 // are checked through pip's own parser, including nested requirement files.
 checkArgs:=append([]string{"-c",pythonRequirementsCheck},files...)
 healthy:=strings.TrimSpace(string(old))==digest&&r.valid(work,nil,py,"-m","pip","check")&&r.valid(work,nil,py,checkArgs...)
 if healthy&&!r.options.Force{fmt.Fprintln(r.out,"[OK] Python dependencies; installation skipped");return bin,false,nil}
 if len(files)==0&&r.valid(work,nil,py,"-m","pip","check"){fmt.Fprintln(r.out,"[OK] no Python requirements");return bin,false,nil}
 fmt.Fprintln(r.out,"[INSTALL] Python dependencies")
 if r.readonly(){return bin,true,nil}
 if !r.valid(work,nil,py,"-m","pip","--version"){if err=r.run(work,nil,py,"-m","ensurepip");err!=nil{return bin,true,err}}
 if len(files)>0{if err=r.run(work,nil,py,args...);err!=nil{return bin,true,err}}
 if err=r.run(work,nil,py,"-m","pip","check");err!=nil{return bin,true,err}
 return bin,false,os.WriteFile(stamp,[]byte(digest+"\n"),0600)
}
const pythonRequirementsCheck = "import sys, importlib.metadata as m\nfrom pip._internal.req.req_file import parse_requirements\nfrom pip._internal.network.session import PipSession\nfrom pip._vendor.packaging.requirements import Requirement\nfor f in sys.argv[1:]:\n for p in parse_requirements(f, session=PipSession()):\n  if p.is_editable: sys.exit(1)\n  r=Requirement(p.requirement)\n  if r.marker and not r.marker.evaluate(): continue\n  if r.url or not r.specifier.contains(m.version(r.name), prereleases=True): sys.exit(1)\n"

func dependencyDigest(files []string,version string)(string,error){
 h:=sha256.New();fmt.Fprintln(h,version)
 for _,p:=range files{b,e:=os.ReadFile(p);if e!=nil{return "",e};fmt.Fprintf(h,"%s\x00%d\x00",p,len(b));h.Write(b)}
 return fmt.Sprintf("%x",h.Sum(nil)),nil
}
func (r bootstrapRunner) node(e BootstrapEnvironment,path,work string)(string,bool,error){
 if runtime.GOOS=="windows"{return "",false,fmt.Errorf("nvm bootstrap requires POSIX shell")}
 bin:=filepath.Join(path,"versions","node","v"+e.Version,"bin");node:=filepath.Join(bin,"node")
 env:=[]string{"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH")}
 if !fileExists(filepath.Join(work,"package.json")){return bin,true,fmt.Errorf("package.json missing in %s",work)}
 if !r.valid(work,env,node,"--version"){
  fmt.Fprintln(r.out,"[INSTALL] Node.js via nvm")
  if r.readonly(){return bin,true,nil}
  home,_:=os.UserHomeDir();nvmDir:=os.Getenv("NVM_DIR");if nvmDir==""{nvmDir=filepath.Join(home,".nvm")}
  script:=filepath.Join(nvmDir,"nvm.sh");if !fileExists(script){return bin,true,fmt.Errorf("install nvm first (nvm.sh not found at %s)",script)}
  if err:=os.MkdirAll(path,0755);err!=nil{return bin,true,err}
  if err:=r.run(work,nil,"bash","-c","export NVM_DIR=\"$1\"; . \"$2\" --no-use; nvm install \"$3\"","bootstrap",path,script,e.Version);err!=nil{return bin,true,err}
 }
 actual,err:=r.command(work,env,node,"--version").Output();if err!=nil||strings.TrimSpace(string(actual))!="v"+e.Version{return bin,true,fmt.Errorf("Node version mismatch")}
 npm:=filepath.Join(bin,"npm");files:=[]string{filepath.Join(work,"package.json")};lock:=filepath.Join(work,"package-lock.json");action:="install";if fileExists(lock){files=append(files,lock);action="ci"}
 digest,err:=dependencyDigest(files,e.Version);if err!=nil{return bin,true,err}
 // Per working directory, not shared across checkout changes.
 stamp:=filepath.Join(work,"node_modules",".update-cli-dependencies");old,_:=os.ReadFile(stamp)
 if !r.options.Force&&strings.TrimSpace(string(old))==digest&&r.valid(work,env,npm,"ls","--depth=0"){fmt.Fprintln(r.out,"[OK] npm dependencies; installation skipped");return bin,false,nil}
 fmt.Fprintf(r.out,"[INSTALL] npm %s\n",action)
 if r.readonly(){return bin,true,nil}
 if err=r.run(work,env,npm,action);err!=nil{return bin,true,err}
 if fileExists(lock)&&len(files)==1{files=append(files,lock)}
 digest,err=dependencyDigest(files,e.Version);if err!=nil{return bin,true,err}
 if err=os.MkdirAll(filepath.Dir(stamp),0755);err!=nil{return bin,true,err}
 return bin,false,os.WriteFile(stamp,[]byte(digest+"\n"),0600)
}

const beginEnvrc="# BEGIN update-cli managed bootstrap"
const endEnvrc="# END update-cli managed bootstrap"
func direnvBlock(root string,bins map[string]string)string{
 s:=beginEnvrc+"\nexport PROJECT_ROOT=\"$(pwd -P)\"\n"
 if b:=bins["python"];b!=""{p:=filepath.Dir(b);exe:="python";if runtime.GOOS=="windows"{exe+=".exe"};s+="export VENV_DIR="+shellQuote(p)+"\nexport VIRTUAL_ENV=\"$VENV_DIR\"\nexport PROJECT_PYTHON="+shellQuote(filepath.Join(b,exe))+"\nPATH_add "+shellQuote(b)+"\n"}
 if b:=bins["node"];b!=""{s+="export PROJECT_NODE="+shellQuote(filepath.Join(b,"node"))+"\nPATH_add "+shellQuote(b)+"\n"}
 return s+endEnvrc
}
func managedEnvrc(root,block string,readonly bool)(bool,error){
 p:=filepath.Join(root,".envrc");old,err:=os.ReadFile(p);if err!=nil&&!os.IsNotExist(err){return false,err}
 s:=string(old);a,b:=strings.Index(s,beginEnvrc),strings.Index(s,endEnvrc)
 if (a<0)!=(b<0)||a>=0&&b<a{return false,fmt.Errorf("malformed managed .envrc block")}
 var next string
 if a>=0{next=s[:a]+block+s[b+len(endEnvrc):]}else{next=strings.TrimRight(s,"\n");if next!=""{next+="\n\n"};next+=block+"\n"}
 if s==next{return false,nil};if readonly{return true,nil}
 mode:=os.FileMode(0644);if info,e:=os.Stat(p);e==nil{mode=info.Mode().Perm()}
 return true,os.WriteFile(p,[]byte(next),mode)
}
