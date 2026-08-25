package workspace

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"codea-harness-tools/internal/schema"
)

func TestWorkspaceDependenciesOptionalAndValidSibling(t *testing.T) {
	parent := t.TempDir()
	repo := filepath.Join(parent, "order-service")
	dep := filepath.Join(parent, "company-framework")
	mustMkdir(t, repo)
	mustMkdir(t, dep)

	deps, err := ValidateConfigYAML(repo, []byte("version: 2\n"))
	if err != nil {
		t.Fatalf("optional workspaceDependencies must preserve 1.5.1 behavior: %v", err)
	}
	if len(deps) != 0 {
		t.Fatalf("expected no dependencies, got %#v", deps)
	}

	deps, err = ValidateConfigYAML(repo, []byte(`workspaceDependencies:
  - id: company-framework
    root: ../company-framework
    maven:
      groupId: com.company
      artifactId: company-framework
    mode: READ_ONLY
`))
	if err != nil {
		t.Fatalf("valid direct sibling dependency rejected: %v", err)
	}
	if len(deps) != 1 || deps[0].ID != "company-framework" || deps[0].Maven.GroupID != "com.company" || deps[0].Maven.ArtifactID != "company-framework" {
		t.Fatalf("unexpected parsed dependency: %#v", deps)
	}
}

func TestWorkspaceDependencyRejectMatrix(t *testing.T) {
	parent := t.TempDir()
	repo := filepath.Join(parent, "order-service")
	depA := filepath.Join(parent, "company-framework")
	depB := filepath.Join(parent, "company-common")
	mustMkdir(t, repo)
	mustMkdir(t, depA)
	mustMkdir(t, depB)

	cases := []struct {
		name string
		yaml string
		code string
	}{
		{
			name: "duplicate id",
			yaml: depsYAML(
				depYAML("company-framework", "../company-framework", "READ_ONLY"),
				depYAML("company-framework", "../company-common", "READ_ONLY"),
			),
			code: "WORKSPACE_DEPENDENCY_DUPLICATE_ID",
		},
		{
			name: "duplicate root",
			yaml: depsYAML(
				depYAML("company-framework", "../company-framework", "READ_ONLY"),
				depYAML("company-framework-copy", "../company-framework", "READ_ONLY"),
			),
			code: "WORKSPACE_DEPENDENCY_DUPLICATE_ROOT",
		},
		{
			name: "missing root",
			yaml: depsYAML(depYAML("missing", "../missing", "READ_ONLY")),
			code: "WORKSPACE_DEPENDENCY_SOURCE_NOT_FOUND",
		},
		{
			name: "current repo",
			yaml: depsYAML(depYAML("self", ".", "READ_ONLY")),
			code: "WORKSPACE_DEPENDENCY_CURRENT_PROJECT",
		},
		{
			name: "non sibling child",
			yaml: depsYAML(depYAML("child", "./src", "READ_ONLY")),
			code: "WORKSPACE_DEPENDENCY_NOT_SIBLING",
		},
		{
			name: "unsupported mode",
			yaml: depsYAML(depYAML("company-framework", "../company-framework", "WRITE")),
			code: "WORKSPACE_DEPENDENCY_MODE_UNSUPPORTED",
		},
		{
			name: "traversal outside workspace parent",
			yaml: depsYAML(depYAML("escape", "../../escape", "READ_ONLY")),
			code: "WORKSPACE_DEPENDENCY_PATH_REJECTED",
		},
		{
			name: "network share",
			yaml: depsYAML(depYAML("network", `\\\\server\\share`, "READ_ONLY")),
			code: "WORKSPACE_DEPENDENCY_PATH_REJECTED",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ValidateConfigYAML(repo, []byte(tc.yaml))
			if err == nil || !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("expected %s, got %v", tc.code, err)
			}
		})
	}
}

func TestWorkspaceDependencyRejectsSiblingSymlinkEscape(t *testing.T) {
	parent := t.TempDir()
	repo := filepath.Join(parent, "order-service")
	outsideParent := t.TempDir()
	outside := filepath.Join(outsideParent, "real-framework")
	link := filepath.Join(parent, "company-framework")
	mustMkdir(t, repo)
	mustMkdir(t, outside)
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	_, err := ValidateConfigYAML(repo, []byte(depsYAML(depYAML("company-framework", "../company-framework", "READ_ONLY"))))
	if err == nil || !strings.Contains(err.Error(), "WORKSPACE_DEPENDENCY_SYMLINK_ESCAPE") {
		t.Fatalf("expected symlink escape rejection, got %v", err)
	}
}

func TestHarnessConfigSchemaKeepsVersion2AndAllowsOptionalWorkspaceDependencies(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	harnessRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", "..", ".."))
	schemaBytes, err := os.ReadFile(filepath.Join(harnessRoot, "contracts", "harness-config.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	templateBytes, err := os.ReadFile(filepath.Join(harnessRoot, "harness.template.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.ValidateYAML(schemaBytes, templateBytes); err != nil {
		t.Fatalf("existing 1.5.1 config template must remain valid: %v", err)
	}
	withWorkspace := append(append([]byte(nil), templateBytes...), []byte(`
workspaceDependencies:
  - id: company-framework
    root: ../company-framework
    maven:
      groupId: com.company
      artifactId: company-framework
    mode: READ_ONLY
`)...)
	if err := schema.ValidateYAML(schemaBytes, withWorkspace); err != nil {
		t.Fatalf("version 2 schema must allow optional workspaceDependencies: %v", err)
	}
}

func TestProjectAdapterDoesNotAutoScanWorkspaceDependencies(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	harnessRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", "..", ".."))
	data, err := os.ReadFile(filepath.Join(harnessRoot, "agents", "project-adapter.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"workspaceDependencies", "显式", "不得自动扫描"} {
		if !strings.Contains(text, want) {
			t.Fatalf("project adapter missing workspace dependency boundary %q", want)
		}
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func depsYAML(items ...string) string {
	return "workspaceDependencies:\n" + strings.Join(items, "")
}

func depYAML(id, root, mode string) string {
	return "  - id: " + id + "\n" +
		"    root: " + root + "\n" +
		"    maven:\n" +
		"      groupId: com.company\n" +
		"      artifactId: " + id + "\n" +
		"    mode: " + mode + "\n"
}
