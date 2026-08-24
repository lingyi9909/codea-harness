package apply

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeRequestFile(t *testing.T, root string, req Request) string {
	t.Helper()
	p := filepath.Join(root, ".code-harness", "runs", req.RunID, "requests", "apply.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil { t.Fatal(err) }
	b, err := json.Marshal(req)
	if err != nil { t.Fatal(err) }
	if err := os.WriteFile(p, b, 0o600); err != nil { t.Fatal(err) }
	return p
}

func TestSealedApprovalRejectsSelfConsistentPatchReplacement(t *testing.T) {
	root := t.TempDir()
	writePolicy(t, root)
	path := "src/main/java/A.java"
	before := "old\n"
	writeRepoFile(t, root, path, before)

	approved := singleFileRequest("FIX", "run-sealed", "fix-sealed", path, before, "approved\n")
	input := writeRequestFile(t, root, approved)
	sealedPath, err := SealRequestFile(root, input)
	if err != nil { t.Fatalf("seal approved request: %v", err) }
	if _, err := os.Stat(sealedPath); err != nil { t.Fatalf("sealed snapshot missing: %v", err) }

	replaced := singleFileRequest("FIX", "run-sealed", "fix-sealed", path, before, "replacement\n")
	b, _ := json.Marshal(replaced)
	if err := os.WriteFile(input, b, 0o600); err != nil { t.Fatal(err) }
	if _, _, err := ApplyRequestFile(root, input); err == nil || !strings.Contains(err.Error(), "APPROVAL_IDENTITY_MISMATCH") {
		t.Fatalf("self-consistent Patch B must not replace approved Patch A: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if string(got) != before { t.Fatalf("file changed after approval mismatch: %q", got) }
}

func TestApplyRequestRequiresSealedApprovalBaseline(t *testing.T) {
	root := t.TempDir()
	writePolicy(t, root)
	path := "src/main/java/A.java"
	before := "old\n"
	writeRepoFile(t, root, path, before)
	req := singleFileRequest("FIX", "run-unsealed", "fix-unsealed", path, before, "new\n")
	input := writeRequestFile(t, root, req)
	if _, _, err := ApplyRequestFile(root, input); err == nil || !strings.Contains(err.Error(), "SEALED_PLAN_NOT_FOUND") {
		t.Fatalf("unsealed apply must be rejected: %v", err)
	}
}

func TestApplyInputPathRejectedBeforeReadAndRunIDMustMatchPath(t *testing.T) {
	root := t.TempDir()
	writePolicy(t, root)
	outside := filepath.Join(root, "outside", "missing.json")
	if _, _, err := ApplyRequestFile(root, outside); err == nil || !strings.Contains(err.Error(), "apply input must be under .code-harness/runs/<runId>/requests/*.json") {
		t.Fatalf("outside path must be rejected before read, got: %v", err)
	}

	path := "src/main/java/A.java"
	before := "old\n"
	writeRepoFile(t, root, path, before)
	req := singleFileRequest("FIX", "body-run", "fix-run", path, before, "new\n")
	b, _ := json.Marshal(req)
	mismatch := filepath.Join(root, ".code-harness", "runs", "path-run", "requests", "apply.json")
	if err := os.MkdirAll(filepath.Dir(mismatch), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(mismatch, b, 0o600); err != nil { t.Fatal(err) }
	if _, _, err := ApplyRequestFile(root, mismatch); err == nil || !strings.Contains(err.Error(), "RUN_ID_PATH_MISMATCH") {
		t.Fatalf("body runId mismatch must be rejected: %v", err)
	}

	nested := filepath.Join(root, ".code-harness", "runs", "path-run", "requests", "nested", "apply.json")
	if _, _, err := ApplyRequestFile(root, nested); err == nil || !strings.Contains(err.Error(), "apply input must be under .code-harness/runs/<runId>/requests/*.json") {
		t.Fatalf("nested request path must be rejected before read: %v", err)
	}
}

func TestRuntimeHardDenyCannotBeOverriddenByBroadHarnessPolicy(t *testing.T) {
	root := t.TempDir()
	writePolicy(t, root)
	broad := `version: 2
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
  allowedTestPaths: ["**"]
  allowedProductionPaths: ["**"]
  deniedPaths: []
runs: {directory: .code-harness/runs}
`
	writeRepoFile(t, root, ".code-harness/harness.yaml", broad)
	writeRepoFile(t, root, ".git/config", "old\n")
	writeRepoFile(t, root, ".code-harness/marker.txt", "old\n")

	cases := []struct{ name, path, before string }{
		{"git hard deny", ".git/config", "old\n"},
		{"framework hard deny", ".code-harness/marker.txt", "old\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := singleFileRequest("FIX", "run-hard-"+strings.ReplaceAll(tc.name, " ", "-"), "fix-hard", tc.path, tc.before, "new\n")
			if _, err := Apply(root, req); err == nil || !strings.Contains(err.Error(), "PATH_HARD_DENIED") {
				t.Fatalf("%s must be runtime hard-denied even with broad config: %v", tc.path, err)
			}
			got, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(tc.path)))
			if string(got) != tc.before { t.Fatalf("hard-denied file changed: %q", got) }
		})
	}
}
