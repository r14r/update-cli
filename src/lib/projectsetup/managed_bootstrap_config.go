package projectsetup

import (
 "fmt"
 "regexp"
)

type BootstrapEnvironment struct {
 Enabled bool
 Version, Path, VersionManager, PackageManager, WorkingDirectory string
 Requirements []string
}

type BootstrapConfig struct {
 Configured bool
 Environments map[string]BootstrapEnvironment
 DirenvEnabled, DirenvGenerate bool
}

func defaultBootstrapEnv(name string) BootstrapEnvironment {
 e := BootstrapEnvironment{Enabled:true, Path:"../.venv/"+name, WorkingDirectory:"."}
 if name=="python" {e.Version="3.12"} else {e.Version="24.15.0"; e.VersionManager="nvm"; e.PackageManager="npm"}
 return e
}

func DefaultBootstrapConfig(names []string) BootstrapConfig {
 c:=BootstrapConfig{Configured:true, Environments:map[string]BootstrapEnvironment{}, DirenvEnabled:true, DirenvGenerate:true}
 for _,name:=range names {c.Environments[name]=defaultBootstrapEnv(name)}
 return c
}

func parseBootstrap(n *simpleYAMLNode) (BootstrapConfig,error) {
 c:=DefaultBootstrapConfig(nil)
 if n.kind!=yamlMap {return c,lineError(n,"bootstrap must be a mapping")}
 for k,v:=range n.m {
  if v.kind!=yamlMap {return c,lineError(v,"bootstrap."+k+" must be a mapping")}
  switch k {
  case "environments":
   for name,s:=range v.m {
    if name!="python" && name!="node" {return c,lineError(s,"unsupported environment "+name)}
    if s.kind!=yamlMap {return c,lineError(s,"environment must be a mapping")}
    e:=defaultBootstrapEnv(name)
    for key,val:=range s.m {
     var err error
     switch key {
     case "enabled": e.Enabled,err=nodeBool(val)
     case "version": e.Version,err=nodeString(val)
     case "path": e.Path,err=nodeString(val)
     case "requirements": e.Requirements,err=nodeStringList(val)
     case "version_manager": e.VersionManager,err=nodeString(val)
     case "package_manager": e.PackageManager,err=nodeString(val)
     case "working_directory": e.WorkingDirectory,err=nodeString(val)
     default: return c,lineError(val,"unknown bootstrap field "+key)
     }
     if err!=nil {return c,lineError(val,err.Error())}
    }
    if !regexp.MustCompile("^[0-9]+[.][0-9]+([.][0-9]+)?$").MatchString(e.Version) || e.Path=="" || e.WorkingDirectory=="" {return c,lineError(s,"invalid version or path")}
    if name=="node" && (e.VersionManager!="nvm" || e.PackageManager!="npm" || len(e.Requirements)>0 || !regexp.MustCompile("^[0-9]+[.][0-9]+[.][0-9]+$").MatchString(e.Version)) {return c,lineError(s,"node requires exact version, nvm and npm")}
    if name=="python" && (e.VersionManager!="" || e.PackageManager!="") {return c,lineError(s,"python uses venv and pip")}
    c.Environments[name]=e
   }
  case "direnv":
   for key,val:=range v.m {
    b,err:=nodeBool(val);if err!=nil{return c,err}
    switch key {case "enabled": c.DirenvEnabled=b;case "generate":c.DirenvGenerate=b;default:return c,lineError(val,"unknown direnv field "+key)}
   }
  default:return c,lineError(v,"unknown bootstrap field "+k)
  }
 }
 return c,nil
}

func ReadBootstrapConfig(path string) (BootstrapConfig,error) {
 m,err:=ParseManifest(path);return m.Bootstrap,err
}

func bootstrapSchema() map[string]any {
 str:=map[string]any{"type":"string","minLength":1}; boolean:=map[string]any{"type":"boolean"}
 env:=func(node bool) map[string]any {
  props:=map[string]any{"enabled":boolean,"version":str,"path":str,"working_directory":str}
  if node {props["version_manager"]=map[string]any{"const":"nvm"};props["package_manager"]=map[string]any{"const":"npm"}} else {props["requirements"]=map[string]any{"$ref":"#/$defs/stringList"}}
  return map[string]any{"type":"object","additionalProperties":false,"properties":props}
 }
 return map[string]any{"type":"object","additionalProperties":false,"properties":map[string]any{
 "environments":map[string]any{"type":"object","additionalProperties":false,"properties":map[string]any{"python":env(false),"node":env(true)}},
 "direnv":map[string]any{"type":"object","additionalProperties":false,"properties":map[string]any{"enabled":boolean,"generate":boolean}}}}
}

func BootstrapYAML(names []string) string {
 s:="bootstrap:\n  environments:\n"
 for _,name:=range names {
  e:=defaultBootstrapEnv(name)
  s+=fmt.Sprintf("    %s:\n      enabled: true\n      version: %q\n      path: %s\n",name,e.Version,e.Path)
  if name=="node" {s+="      version_manager: nvm\n      package_manager: npm\n"}
 }
 return s+"  direnv:\n    enabled: true\n    generate: true\n"
}
