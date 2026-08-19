package upgrade_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codea-harness-tools/internal/upgrade"
)

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil { t.Fatal(err) }
}

func validConfig(review string) string {
	return "version: 1\nproject:\n  type: maven\n  root: .\n  module: \"\"\n" + review + `integrationTest:
  executable: mvn
  args:
    - test
  reportDir: target/surefire-reports
  timeoutSeconds: 600
service:
  executable: mvn
  args:
    - spring-boot:run
  startupTimeoutSeconds: 120
  readiness:
    type: log
    pattern: Started
  logFile: null
stopService:
  mode: processTree
initialization:
  status: READY
  unresolved: []
scope:
  sourceIncludes:
    - src/main/java/**
  testIncludes:
    - src/test/java/**
write:
  allowedTestPaths:
    - src/test/**
  allowedProductionPaths:
    - src/main/**
  deniedPaths: []
runs:
  directory: .code-harness/runs
`
}

const schema = `{"type":"object","required":["version","project","review","integrationTest","service","stopService","initialization","scope","write","runs"],"properties":{"review":{"type":"object","required":["baseRef","includeWorkingTree"],"properties":{"baseRef":{"type":"string","minLength":1},"includeWorkingTree":{"type":"boolean"}}}}}`

func makePair(t *testing.T, config string) (string, string) {
	t.Helper(); root:=t.TempDir(); target:=filepath.Join(root,".code-harness"); source:=filepath.Join(root,".code-harness-upgrade")
	write(t,target,"VERSION","1.1.0\n"); write(t,target,"harness.yaml",config); write(t,target,"project.md","keep-project\n"); write(t,target,"runs/keep.txt","keep-run\n"); write(t,target,"AGENTS.md","old\n")
	write(t,source,"VERSION","1.1.1\n")
	for _,rel:=range []string{"AGENTS.md","bootstrap.md","upgrade.md","harness.template.yaml","project.template.md","agents/x.md","skills/x/SKILL.md","contracts/upgrade-result.schema.json","tools/README.md"}{write(t,source,rel,"new "+rel+"\n")}
	write(t,source,"contracts/harness-config.schema.json",schema); write(t,source,"bin/codea-harness-tools.exe","runtime"); write(t,source,"bin/ast-grep.exe","ast-grep"); return source,target
}

func TestUpgradeAddsReviewUsingDetectedOriginDevelop(t *testing.T){source,target:=makePair(t,validConfig("")); result:=upgrade.Run(upgrade.Options{SourceDir:source,TargetDir:target,Refs:upgrade.StaticRefs{RemoteBranches:[]string{"origin/develop"}}}); if result.Status!=upgrade.StatusUpgraded{t.Fatalf("status=%s errors=%v",result.Status,result.Errors)}; b,_:=os.ReadFile(filepath.Join(target,"harness.yaml")); if !strings.Contains(string(b),"review:\n  baseRef: origin/develop\n  includeWorkingTree: true\n"){t.Fatalf("migration missing:\n%s",b)}}
func TestUpgradePreservesExistingReviewExactly(t *testing.T){review:="review:\n  baseRef: origin/release\n  includeWorkingTree: false\n";source,target:=makePair(t,validConfig(review));result:=upgrade.Run(upgrade.Options{SourceDir:source,TargetDir:target,Refs:upgrade.StaticRefs{RemoteBranches:[]string{"origin/develop"}}});if result.Status!=upgrade.StatusUpgraded{t.Fatalf("status=%s errors=%v",result.Status,result.Errors)};b,_:=os.ReadFile(filepath.Join(target,"harness.yaml"));if strings.Count(string(b),review)!=1{t.Fatalf("existing review changed:\n%s",b)}}
func TestUpgradeNeedsManualActionWhenBaseRefCannotBeDetected(t *testing.T){source,target:=makePair(t,validConfig(""));before,_:=os.ReadFile(filepath.Join(target,"harness.yaml"));result:=upgrade.Run(upgrade.Options{SourceDir:source,TargetDir:target,Refs:upgrade.StaticRefs{}});if result.Status!=upgrade.StatusManualActionRequired{t.Fatalf("status=%s errors=%v",result.Status,result.Errors)};after,_:=os.ReadFile(filepath.Join(target,"harness.yaml"));if string(after)!=string(before){t.Fatal("target changed on manual action")}}
func TestUpgradeRollsBackWhenNewSchemaValidationFails(t *testing.T){source,target:=makePair(t,validConfig(""));write(t,source,"contracts/harness-config.schema.json",`{"type":"object","required":["mustNotExist"]}`);beforeVersion,_:=os.ReadFile(filepath.Join(target,"VERSION"));beforeAgents,_:=os.ReadFile(filepath.Join(target,"AGENTS.md"));result:=upgrade.Run(upgrade.Options{SourceDir:source,TargetDir:target,Refs:upgrade.StaticRefs{RemoteBranches:[]string{"origin/develop"}}});if result.Status!=upgrade.StatusUpgradeFailed||!result.RollbackPerformed{t.Fatalf("result=%+v",result)};afterVersion,_:=os.ReadFile(filepath.Join(target,"VERSION"));afterAgents,_:=os.ReadFile(filepath.Join(target,"AGENTS.md"));if string(afterVersion)!=string(beforeVersion)||string(afterAgents)!=string(beforeAgents){t.Fatal("rollback did not restore old harness")}}
func TestUpgradeRejectsPackageMissingToolRuntimeBinary(t *testing.T){source,target:=makePair(t,validConfig("review:\n  baseRef: origin/develop\n  includeWorkingTree: true\n"));if err:=os.Remove(filepath.Join(source,"bin","codea-harness-tools.exe"));err!=nil{t.Fatal(err)};result:=upgrade.Run(upgrade.Options{SourceDir:source,TargetDir:target,Refs:upgrade.StaticRefs{RemoteBranches:[]string{"origin/develop"}}});if result.Status!=upgrade.StatusManualActionRequired{t.Fatalf("result=%+v",result)}}
