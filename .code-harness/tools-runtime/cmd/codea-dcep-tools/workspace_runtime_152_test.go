package main

import (
    "io"
    "os"
    "path/filepath"
    "runtime"
    "strings"
    "testing"
)

func Test152ValidateHarnessConfigRunsWorkspaceSemanticGate(t *testing.T) {
    withWorkspaceRuntimeProject(t, func(root string) {
        mustWrite152(t, filepath.Join(root, ".code-harness", "contracts", "harness-config.schema.json"), `{"type":"object"}`)
        nested := filepath.Join(root, "nested-dependency")
        if err := os.MkdirAll(nested, 0o755); err != nil { t.Fatal(err) }
        mustWrite152(t, filepath.Join(root, ".code-harness", "harness.yaml"), `version: 2
workspaceDependencies:
  - id: nested
    root: ./nested-dependency
    maven:
      groupId: com.company
      artifactId: nested
    mode: READ_ONLY
`)
        err := run([]string{"validate", "--schema", ".code-harness/contracts/harness-config.schema.json", "--input", ".code-harness/harness.yaml", "--format", "yaml"})
        if err == nil || !strings.Contains(err.Error(), "WORKSPACE_DEPENDENCY_NOT_SIBLING") {
            t.Fatalf("Schema-valid non-sibling workspace must fail semantic gate, got %v", err)
        }
    })
}

func Test152ValidateHarnessConfigRejectsSymlinkEscapeAfterSchema(t *testing.T) {
    if runtime.GOOS == "windows" { t.Skip("symlink permission is environment-dependent on Windows; covered by workspace semantic unit gate") }
    parent := t.TempDir()
    root := filepath.Join(parent, "order-service")
    outsideParent := t.TempDir()
    outside := filepath.Join(outsideParent, "company-framework")
    if err := os.MkdirAll(filepath.Join(root, ".code-harness", "contracts"), 0o755); err != nil { t.Fatal(err) }
    if err := os.MkdirAll(outside, 0o755); err != nil { t.Fatal(err) }
    link := filepath.Join(parent, "company-framework")
    if err := os.Symlink(outside, link); err != nil { t.Skipf("symlink unavailable: %v", err) }
    mustWrite152(t, filepath.Join(root, ".code-harness", "contracts", "harness-config.schema.json"), `{"type":"object"}`)
    mustWrite152(t, filepath.Join(root, ".code-harness", "harness.yaml"), `version: 2
workspaceDependencies:
  - id: company-framework
    root: ../company-framework
    maven:
      groupId: com.company
      artifactId: company-framework
    mode: READ_ONLY
`)
    withChdir152(t, root, func() {
        err := run([]string{"validate", "--schema", ".code-harness/contracts/harness-config.schema.json", "--input", ".code-harness/harness.yaml", "--format", "yaml"})
        if err == nil || !strings.Contains(err.Error(), "WORKSPACE_DEPENDENCY_SYMLINK_ESCAPE") {
            t.Fatalf("Schema-valid symlink escape must fail semantic gate, got %v", err)
        }
    })
}

func Test152WorkspaceVerifyIsFormalRuntimeEntry(t *testing.T) {
    withWorkspaceRuntimeProject(t, func(root string) {
        dependency := filepath.Join(filepath.Dir(root), "company-framework")
        if err := os.MkdirAll(dependency, 0o755); err != nil { t.Fatal(err) }
        writeWorkspaceConfig152(t, root, "2.3.1")
        writeWorkspacePOMs152(t, root, dependency, "2.3.1", "2.3.1")
        output, err := captureStdout152(func() error { return run([]string{"workspace", "verify", "--id", "company-framework"}) })
        if err != nil { t.Fatalf("workspace verify failed: %v", err) }
        if !strings.Contains(output, `"status": "VERIFIED"`) || !strings.Contains(output, `"dependencyId": "company-framework"`) {
            t.Fatalf("workspace verify output=%s", output)
        }
    })
}

func Test152WorkspaceNavRejectsMismatchedSourceBeforeAstGrep(t *testing.T) {
    withWorkspaceRuntimeProject(t, func(root string) {
        dependency := filepath.Join(filepath.Dir(root), "company-framework")
        if err := os.MkdirAll(dependency, 0o755); err != nil { t.Fatal(err) }
        writeWorkspaceConfig152(t, root, "2.3.1")
        writeWorkspacePOMs152(t, root, dependency, "2.3.1", "2.4.0")
        err := run([]string{"nav", "workspace-inherited", "--workspace", "company-framework", "--from", "XxxServiceImpl.submit", "--method", "execute"})
        if err == nil || !strings.Contains(err.Error(), "WORKSPACE_DEPENDENCY_VERSION_MISMATCH") {
            t.Fatalf("workspace nav must stop at Maven mismatch before navigation, got %v", err)
        }
    })
}

func withWorkspaceRuntimeProject(t *testing.T, fn func(root string)) {
    t.Helper()
    parent := t.TempDir()
    root := filepath.Join(parent, "order-service")
    if err := os.MkdirAll(filepath.Join(root, ".code-harness", "contracts"), 0o755); err != nil { t.Fatal(err) }
    withChdir152(t, root, func() { fn(root) })
}

func withChdir152(t *testing.T, dir string, fn func()) {
    t.Helper()
    old, err := os.Getwd(); if err != nil { t.Fatal(err) }
    if err := os.Chdir(dir); err != nil { t.Fatal(err) }
    defer func() { _ = os.Chdir(old) }()
    fn()
}

func writeWorkspaceConfig152(t *testing.T, root, version string) {
    t.Helper()
    _ = version
    mustWrite152(t, filepath.Join(root, ".code-harness", "harness.yaml"), `version: 2
workspaceDependencies:
  - id: company-framework
    root: ../company-framework
    maven:
      groupId: com.company
      artifactId: company-framework
    mode: READ_ONLY
`)
    mustWrite152(t, filepath.Join(root, ".code-harness", "contracts", "harness-config.schema.json"), `{"type":"object"}`)
}

func writeWorkspacePOMs152(t *testing.T, root, dependency, currentVersion, sourceVersion string) {
    t.Helper()
    mustWrite152(t, filepath.Join(root, "pom.xml"), `<project><modelVersion>4.0.0</modelVersion><groupId>com.company</groupId><artifactId>order-service</artifactId><version>1.0.0</version><dependencies><dependency><groupId>com.company</groupId><artifactId>company-framework</artifactId><version>`+currentVersion+`</version></dependency></dependencies></project>`)
    mustWrite152(t, filepath.Join(dependency, "pom.xml"), `<project><modelVersion>4.0.0</modelVersion><groupId>com.company</groupId><artifactId>company-framework</artifactId><version>`+sourceVersion+`</version></project>`)
}

func mustWrite152(t *testing.T, path, content string) {
    t.Helper()
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { t.Fatal(err) }
    if err := os.WriteFile(path, []byte(content), 0o644); err != nil { t.Fatal(err) }
}

func captureStdout152(fn func() error) (string, error) {
    old := os.Stdout
    r, w, err := os.Pipe(); if err != nil { return "", err }
    os.Stdout = w
    callErr := fn()
    _ = w.Close()
    os.Stdout = old
    b, readErr := io.ReadAll(r)
    _ = r.Close()
    if readErr != nil { return "", readErr }
    return string(b), callErr
}
