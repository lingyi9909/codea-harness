package apply

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func hashText(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func writeRepoFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil { t.Fatal(err) }
}

func writePolicy(t *testing.T, root string) {
	t.Helper()
	writeRepoFile(t, root, ".code-harness/harness.yaml", `version: 2
project: {type: maven, root: ., module: ""}
review: {baseRef: origin/develop, includeWorkingTree: true}
integrationTest: {executable: ./mvnw, args: [test], reportDir: target/surefire-reports, timeoutSeconds: 600}
service:
  executable: ./mvnw
  args: [spring-boot:run]
  startupTimeoutSeconds: 120
  readiness: {type: log, pattern: Started}
  logFile: null
stopService: {mode: processTree}
initialization: {status: READY, unresolved: []}
scope:
  sourceIncludes: [src/main/java/**/*.java]
  testIncludes: [src/test/java/**/*.java]
  mapperIncludes: [src/main/resources/**/*Mapper.xml]
  configIncludes: [src/main/resources/**/*.yml]
write:
  allowedTestPaths: [src/test/java/**]
  allowedProductionPaths: [src/main/java/**]
  deniedPaths: [src/main/java/Denied.java, .code-harness/**]
runs: {directory: .code-harness/runs}
`)
	// Task 4 requires loading through the existing harness config schema.
	schema, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "harness-config.schema.json"))
	if err != nil { t.Fatal(err) }
	writeRepoFile(t, root, ".code-harness/contracts/harness-config.schema.json", string(schema))
}

func singleFileRequest(planType, runID, planID, path, before, after string) Request {
	diff := "--- a/" + path + "\n+++ b/" + path + "\n@@ -1 +1 @@\n-" + strings.TrimSuffix(before, "\n") + "\n+" + strings.TrimSuffix(after, "\n") + "\n"
	return Request{
		RunID: runID, PlanType: planType, PlanID: planID,
		DiffSha256: hashText(diff), UnifiedDiff: diff,
		Files: []FileRequest{{Path: path, BaseSha256: hashText(before)}},
	}
}

func TestRejectsDiffHashMismatchWithoutWrites(t *testing.T) {
	root := t.TempDir(); writePolicy(t, root)
	path := "src/main/java/A.java"; before := "old\n"; writeRepoFile(t, root, path, before)
	req := singleFileRequest("FIX", "run-1", "fix-1", path, before, "new\n")
	req.DiffSha256 = strings.Repeat("0", 64)
	res, err := Apply(root, req)
	if err == nil || !strings.Contains(err.Error(), "DIFF_HASH_MISMATCH") { t.Fatalf("err=%v result=%+v", err, res) }
	got, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if string(got) != before { t.Fatalf("file changed: %q", got) }
}

func TestRejectsBaseChangedWithoutWrites(t *testing.T) {
	root := t.TempDir(); writePolicy(t, root)
	path := "src/main/java/A.java"; before := "current\n"; writeRepoFile(t, root, path, before)
	req := singleFileRequest("FIX", "run-1", "fix-1", path, "approved-base\n", "new\n")
	res, err := Apply(root, req)
	if err == nil || !strings.Contains(err.Error(), "BASE_CHANGED") { t.Fatalf("err=%v result=%+v", err, res) }
	got, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if string(got) != before { t.Fatalf("file changed: %q", got) }
}

func TestWritePathPolicyEnforcesPlanTypeAndDeny(t *testing.T) {
	root := t.TempDir(); writePolicy(t, root)
	cases := []struct{name, planType, path string}{
		{"fix test", "FIX", "src/test/java/AIT.java"},
		{"test production", "TEST", "src/main/java/A.java"},
		{"denied overrides production", "FIX", "src/main/java/Denied.java"},
		{"framework managed", "FIX", ".code-harness/agents/orchestrator.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := "old\n"; writeRepoFile(t, root, tc.path, before)
			req := singleFileRequest(tc.planType, "run-"+strings.ReplaceAll(tc.name," ","-"), "plan-1", tc.path, before, "new\n")
			if _, err := Apply(root, req); err == nil { t.Fatal("expected path policy rejection") }
			got, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(tc.path)))
			if string(got) != before { t.Fatalf("file changed: %q", got) }
		})
	}
}

func TestRejectsUndeclaredTouchedFile(t *testing.T) {
	root := t.TempDir(); writePolicy(t, root)
	writeRepoFile(t, root, "src/main/java/A.java", "a\n")
	writeRepoFile(t, root, "src/main/java/B.java", "b\n")
	diff := "--- a/src/main/java/A.java\n+++ b/src/main/java/A.java\n@@ -1 +1 @@\n-a\n+A\n--- a/src/main/java/B.java\n+++ b/src/main/java/B.java\n@@ -1 +1 @@\n-b\n+B\n"
	req := Request{RunID:"run-1",PlanType:"FIX",PlanID:"fix-1",DiffSha256:hashText(diff),UnifiedDiff:diff,Files:[]FileRequest{{Path:"src/main/java/A.java",BaseSha256:hashText("a\n")}}}
	if _, err := Apply(root, req); err == nil || !strings.Contains(err.Error(), "DECLARED_FILES_MISMATCH") { t.Fatalf("err=%v", err) }
}

func TestRejectsTraversalAbsoluteRenameAndBinaryPatch(t *testing.T) {
	root := t.TempDir(); writePolicy(t, root)
	cases := []Request{
		{RunID:"run-1",PlanType:"FIX",PlanID:"fix-1",DiffSha256:hashText("--- a/../A.java\n+++ b/../A.java\n"),UnifiedDiff:"--- a/../A.java\n+++ b/../A.java\n",Files:[]FileRequest{{Path:"../A.java",BaseSha256:strings.Repeat("0",64)}}},
		{RunID:"run-2",PlanType:"FIX",PlanID:"fix-2",DiffSha256:hashText("Binary files a/src/main/java/A.java and b/src/main/java/A.java differ\n"),UnifiedDiff:"Binary files a/src/main/java/A.java and b/src/main/java/A.java differ\n",Files:[]FileRequest{{Path:"src/main/java/A.java",BaseSha256:strings.Repeat("0",64)}}},
		{RunID:"run-3",PlanType:"FIX",PlanID:"fix-3",DiffSha256:hashText("--- a/src/main/java/A.java\n+++ b/src/main/java/B.java\n"),UnifiedDiff:"--- a/src/main/java/A.java\n+++ b/src/main/java/B.java\n",Files:[]FileRequest{{Path:"src/main/java/A.java",BaseSha256:strings.Repeat("0",64)}}},
	}
	for i, req := range cases {
		if _, err := Apply(root, req); err == nil { t.Fatalf("case %d should reject", i) }
	}
}

func TestAtomicMultiFileApplyRollsBackWhenSecondReplaceFails(t *testing.T) {
	root := t.TempDir(); writePolicy(t, root)
	for _, p := range []string{"src/main/java/A.java","src/main/java/B.java"} { writeRepoFile(t, root, p, strings.TrimSuffix(filepath.Base(p), ".java")+"\n") }
	diff := "--- a/src/main/java/A.java\n+++ b/src/main/java/A.java\n@@ -1 +1 @@\n-A\n+A2\n--- a/src/main/java/B.java\n+++ b/src/main/java/B.java\n@@ -1 +1 @@\n-B\n+B2\n"
	req := Request{RunID:"run-atomic",PlanType:"FIX",PlanID:"fix-atomic",DiffSha256:hashText(diff),UnifiedDiff:diff,Files:[]FileRequest{{Path:"src/main/java/A.java",BaseSha256:hashText("A\n")},{Path:"src/main/java/B.java",BaseSha256:hashText("B\n")}}}
	calls := 0
	ops := defaultFileOps()
	origReplace := ops.replace
	ops.replace = func(src, dst string) error {
		calls++
		if calls == 2 { return errors.New("injected second replacement failure") }
		return origReplace(src, dst)
	}
	res, err := applyWithOps(root, req, ops)
	if err == nil { t.Fatal("expected injected failure") }
	if !res.RollbackPerformed { t.Fatalf("rollbackPerformed=false: %+v", res) }
	for p, want := range map[string]string{"src/main/java/A.java":"A\n","src/main/java/B.java":"B\n"} {
		got, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		if string(got) != want { t.Fatalf("%s=%q want %q", p, got, want) }
	}
}

func TestSuccessWritesValidatedEvidenceAndBlocksDuplicatePlan(t *testing.T) {
	root := t.TempDir(); writePolicy(t, root)
	path := "src/main/java/A.java"; before := "old\n"; after := "new\n"; writeRepoFile(t, root, path, before)
	req := singleFileRequest("FIX", "run-evidence", "fix-evidence", path, before, after)
	res, err := Apply(root, req)
	if err != nil { t.Fatal(err) }
	if res.Status != StatusApplied || res.RollbackPerformed || len(res.Files)!=1 || res.Files[0].BeforeSha256!=hashText(before) || res.Files[0].AfterSha256!=hashText(after) { t.Fatalf("result=%+v", res) }
	evidence := filepath.Join(root, ".code-harness", "runs", req.RunID, "evidence", "apply", req.PlanID+".json")
	if _, err := os.Stat(evidence); err != nil { t.Fatalf("missing evidence: %v", err) }
	if _, err := Apply(root, req); err == nil || !strings.Contains(err.Error(), "PLAN_ALREADY_APPLIED") { t.Fatalf("duplicate apply err=%v", err) }
}
