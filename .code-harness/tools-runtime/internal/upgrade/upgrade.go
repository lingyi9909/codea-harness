package upgrade

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"codea-harness-tools/internal/schema"
)

const (
	StatusUpgraded = "UPGRADED"
	StatusAlreadyUpToDate = "ALREADY_UP_TO_DATE"
	StatusManualActionRequired = "MANUAL_ACTION_REQUIRED"
	StatusUpgradeFailed = "UPGRADE_FAILED"
)

type Result struct {
	Status string `json:"status"`
	FromVersion string `json:"fromVersion"`
	ToVersion string `json:"toVersion"`
	UpdatedFiles []string `json:"updatedFiles,omitempty"`
	RemovedFiles []string `json:"removedFiles,omitempty"`
	PreservedFiles []string `json:"preservedFiles,omitempty"`
	RollbackPerformed bool `json:"rollbackPerformed"`
	Errors []string `json:"errors"`
	Migrations []string `json:"migrations,omitempty"`
}
type RefProvider interface{ DetectBaseRef() (string, bool) }
type StaticRefs struct { OriginHead string; RemoteBranches, LocalBranches []string }
func (s StaticRefs) DetectBaseRef()(string,bool){ if s.OriginHead!=""{return s.OriginHead,true}; has:=func(xs []string,v string)bool{for _,x:=range xs{if x==v{return true}};return false}; for _,v:=range []string{"origin/master","origin/main","origin/develop"}{if has(s.RemoteBranches,v){return v,true}}; for _,v:=range []string{"master","main","develop"}{if has(s.LocalBranches,v){return v,true}}; return "",false }

type Options struct { SourceDir, TargetDir string; Refs RefProvider }
var semverRE=regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`)
func parseVer(s string)([3]int,error){m:=semverRE.FindStringSubmatch(strings.TrimSpace(s)); if m==nil{return [3]int{},fmt.Errorf("invalid semver %q",strings.TrimSpace(s))}; var v [3]int; for i:=0;i<3;i++{v[i],_=strconv.Atoi(m[i+1])}; return v,nil}
func cmp(a,b [3]int)int{for i:=0;i<3;i++{if a[i]<b[i]{return -1};if a[i]>b[i]{return 1}};return 0}

var requiredSource=[]string{"VERSION","AGENTS.md","bootstrap.md","upgrade.md","harness.template.yaml","project.template.md","agents","skills","contracts","tools","contracts/harness-config.schema.json","bin/codea-harness-tools.exe","bin/ast-grep.exe"}

func Run(o Options) Result {
	r:=Result{PreservedFiles:[]string{"project.md","runs/**"},Errors:[]string{}}
	oldB,err:=os.ReadFile(filepath.Join(o.TargetDir,"VERSION")); if err!=nil{return failManual(r,err)}
	newB,err:=os.ReadFile(filepath.Join(o.SourceDir,"VERSION")); if err!=nil{return failManual(r,err)}
	r.FromVersion=strings.TrimSpace(string(oldB)); r.ToVersion=strings.TrimSpace(string(newB))
	oldV,err:=parseVer(r.FromVersion); if err!=nil{return failManual(r,err)}
	newV,err:=parseVer(r.ToVersion); if err!=nil{return failManual(r,err)}
	if cmp(newV,oldV)<0{return failManual(r,fmt.Errorf("downgrade is not allowed"))}
	if cmp(newV,oldV)==0{r.Status=StatusAlreadyUpToDate;return r}
	for _,rel:=range requiredSource{if _,err:=os.Stat(filepath.Join(o.SourceDir,rel));err!=nil{return failManual(r,fmt.Errorf("incomplete upgrade package: %s",rel))}}
	cfgPath:=filepath.Join(o.TargetDir,"harness.yaml"); cfg,err:=os.ReadFile(cfgPath); if err!=nil{return failManual(r,err)}
	migrated:=cfg
	if !hasTopLevelReview(cfg){
		if o.Refs==nil{return failManual(r,fmt.Errorf("cannot detect Review baseline; configure review.baseRef"))}
		base,ok:=o.Refs.DetectBaseRef(); if !ok{return failManual(r,fmt.Errorf("无法确定 Review 基线，请配置 review.baseRef"))}
		migrated=append(append([]byte{},cfg...),[]byte(fmt.Sprintf("\nreview:\n  baseRef: %s\n  includeWorkingTree: true\n",base))...)
		r.Migrations=append(r.Migrations,"add-review-config-v1")
	}
	backup:=o.TargetDir+".backup-"+strconv.FormatInt(time.Now().UnixNano(),10)
	if err:=copyTree(o.TargetDir,backup,nil);err!=nil{return failManual(r,fmt.Errorf("backup: %w",err))}
	rollback:=func(e error)Result{_ = os.RemoveAll(o.TargetDir); _ = os.Rename(backup,o.TargetDir); r.Status=StatusUpgradeFailed; r.RollbackPerformed=true; r.Errors=[]string{e.Error()}; return r}
	if err:=copyTree(o.SourceDir,o.TargetDir,func(rel string,entry fs.DirEntry)bool{if rel=="VERSION"||rel=="harness.yaml"||rel=="project.md"||rel=="runs"||strings.HasPrefix(rel,"runs/"){return false};return true});err!=nil{return rollback(err)}
	if err:=os.WriteFile(cfgPath,migrated,0o644);err!=nil{return rollback(err)}
	schemaB,err:=os.ReadFile(filepath.Join(o.SourceDir,"contracts","harness-config.schema.json")); if err!=nil{return rollback(err)}
	if err:=schema.ValidateYAML(schemaB,migrated);err!=nil{return rollback(fmt.Errorf("harness.yaml incompatible with new schema: %w",err))}
	if err:=os.WriteFile(filepath.Join(o.TargetDir,"VERSION"),newB,0o644);err!=nil{return rollback(err)}
	_ = os.RemoveAll(backup); r.Status=StatusUpgraded; r.UpdatedFiles=listFrameworkFiles(o.SourceDir); return r
}
func failManual(r Result,e error)Result{r.Status=StatusManualActionRequired;r.Errors=[]string{e.Error()};return r}
func hasTopLevelReview(b []byte)bool{for _,l:=range strings.Split(strings.ReplaceAll(string(b),"\r\n","\n"),"\n"){if strings.HasPrefix(l,"review:"){return true}};return false}
func copyTree(src,dst string,allow func(string,fs.DirEntry)bool)error{return filepath.WalkDir(src,func(p string,d fs.DirEntry,err error)error{if err!=nil{return err};rel,_:=filepath.Rel(src,p);if rel=="."{return os.MkdirAll(dst,0o755)};rel=filepath.ToSlash(rel);if allow!=nil&&!allow(rel,d){if d.IsDir(){return filepath.SkipDir};return nil};to:=filepath.Join(dst,filepath.FromSlash(rel));if d.IsDir(){return os.MkdirAll(to,0o755)};b,err:=os.ReadFile(p);if err!=nil{return err};if err:=os.MkdirAll(filepath.Dir(to),0o755);err!=nil{return err};return os.WriteFile(to,b,0o644)})}
func listFrameworkFiles(root string)[]string{var xs []string;_ = filepath.WalkDir(root,func(p string,d fs.DirEntry,e error)error{if e!=nil||d.IsDir(){return nil};rel,_:=filepath.Rel(root,p);rel=filepath.ToSlash(rel);if rel!="harness.yaml"&&rel!="project.md"&&!strings.HasPrefix(rel,"runs/"){xs=append(xs,rel)};return nil});sort.Strings(xs);return xs}
