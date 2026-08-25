package nav

import (
	"os"
	"path/filepath"
	"testing"

	"codea-harness-tools/internal/workspace"
)

func TestWorkspaceDoesNotTreatQualifiedHelperCallAsInheritedCall(t *testing.T) {
	current, dependency := workspaceInheritanceFixture(t)
	writeJava(t, current, "src/main/java/com/company/order/XxxServiceImpl.java", `package com.company.order;
import com.company.framework.AbstractTemplate;
public class XxxServiceImpl extends AbstractTemplate {
    private Helper helper;
    public void submit() { helper.execute(); }
    @Override protected void doExecute() { mapper.updateStatus(); }
}`)
	resolver := workspaceResolverForTest(t, current, verifiedWorkspace(dependency))
	result := resolver.ResolveInheritedCall("XxxServiceImpl.submit", "execute")
	if result.Status != NavigationPartial || result.Limitation == nil || result.Limitation.Code != CodeInheritedMethodNotFound {
		t.Fatalf("qualified helper.execute must not confirm inherited execute: %#v", result)
	}
}

func TestWorkspaceMethodOwnershipIsExactWithMultipleClassesInOneFile(t *testing.T) {
	current, dependency := workspaceInheritanceFixture(t)
	writeJava(t, current, "src/main/java/com/company/order/Multi.java", `package com.company.order;
import com.company.framework.AbstractTemplate;
class OtherServiceImpl extends AbstractTemplate {
    public void submit() { execute(); }
    @Override protected void doExecute() { }
}
class XxxServiceImpl extends AbstractTemplate {
    @Override protected void doExecute() { }
}`)
	_ = os.Remove(filepath.Join(current, "src/main/java/com/company/order/XxxServiceImpl.java"))
	resolver := workspaceResolverForTest(t, current, verifiedWorkspace(dependency))
	result := resolver.ResolveInheritedCall("XxxServiceImpl.submit", "execute")
	if result.Status != NavigationPartial || result.Limitation == nil || result.Limitation.Code != CodeInheritedMethodNotFound {
		t.Fatalf("method from sibling class in same file must not contaminate XxxServiceImpl ownership: %#v", result)
	}
}

func TestWorkspaceResolveInheritedMethodFromCurrentSubclass(t *testing.T) {
	current, dependency := workspaceInheritanceFixture(t)
	resolver := workspaceResolverForTest(t, current, verifiedWorkspace(dependency))
	result := resolver.ResolveInheritedCall("XxxServiceImpl.submit", "execute")
	assertWorkspaceFact(t, result, "company-framework", "AbstractTemplate.execute", "src/main/java/com/company/framework/AbstractTemplate.java", "XxxServiceImpl.submit")
}

func TestWorkspaceResolveSuperclassInternalMethod(t *testing.T) {
	current, dependency := workspaceInheritanceFixture(t)
	resolver := workspaceResolverForTest(t, current, verifiedWorkspace(dependency))
	result := resolver.ResolveSuperclassCall("AbstractTemplate.execute", "validate")
	assertWorkspaceFact(t, result, "company-framework", "AbstractTemplate.validate", "src/main/java/com/company/framework/AbstractTemplate.java", "AbstractTemplate.execute")
}

func TestWorkspaceTemplateMethodDispatchesBackToConcreteOverride(t *testing.T) {
	current, dependency := workspaceInheritanceFixture(t)
	resolver := workspaceResolverForTest(t, current, verifiedWorkspace(dependency))
	result := resolver.ResolveTemplateDispatch("AbstractTemplate.execute", "doExecute", "XxxServiceImpl")
	assertWorkspaceFact(t, result, "current", "XxxServiceImpl.doExecute", "src/main/java/com/company/order/XxxServiceImpl.java", "AbstractTemplate.execute")
}

func TestWorkspaceTemplateMethodAmbiguousDispatchIsPartial(t *testing.T) {
	current, dependency := workspaceInheritanceFixture(t)
	writeJava(t, current, "src/main/java/com/company/order/AnotherServiceImpl.java", `package com.company.order;
import com.company.framework.AbstractTemplate;
public class AnotherServiceImpl extends AbstractTemplate {
    @Override protected void doExecute() { }
}`)
	resolver := workspaceResolverForTest(t, current, verifiedWorkspace(dependency))
	result := resolver.ResolveTemplateDispatch("AbstractTemplate.execute", "doExecute", "")
	if result.Status != NavigationPartial || result.Limitation == nil || result.Limitation.Code != "AMBIGUOUS_TEMPLATE_DISPATCH" {
		t.Fatalf("expected ambiguous template dispatch PARTIAL, got %#v", result)
	}
	if result.Fact != nil {
		t.Fatalf("ambiguous dispatch must not guess a fact: %#v", result.Fact)
	}
}

func TestWorkspaceNavigationRejectsUnverifiedSourcesBeforeReadingJava(t *testing.T) {
	current, dependency := workspaceInheritanceFixture(t)
	cases := []struct {
		name string
		verification workspace.VerificationResult
		code string
	}{
		{"not configured", workspace.VerificationResult{}, "WORKSPACE_DEPENDENCY_NOT_CONFIGURED"},
		{"source missing", workspace.VerificationResult{DependencyID: "company-framework", Status: workspace.StatusSourceNotFound, Code: workspace.CodeSourceNotFound}, workspace.CodeSourceNotFound},
		{"coordinate mismatch", workspace.VerificationResult{DependencyID: "company-framework", Status: workspace.StatusCoordinateMismatch, Code: workspace.CodeCoordinateMismatch}, workspace.CodeCoordinateMismatch},
		{"version unresolved", workspace.VerificationResult{DependencyID: "company-framework", Status: workspace.StatusVersionUnresolved, Code: workspace.CodeVersionUnresolved}, workspace.CodeVersionUnresolved},
		{"version mismatch", workspace.VerificationResult{DependencyID: "company-framework", Status: workspace.StatusVersionMismatch, Code: workspace.CodeVersionMismatch}, workspace.CodeVersionMismatch},
	}
	_ = dependency
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resolver := WorkspaceInheritanceResolver{CurrentRoot: current, Dependency: tc.verification}
			result := resolver.ResolveInheritedCall("XxxServiceImpl.submit", "execute")
			if result.Status != NavigationPartial || result.Limitation == nil || result.Limitation.Code != tc.code {
				t.Fatalf("expected %s PARTIAL, got %#v", tc.code, result)
			}
		})
	}
}

func TestWorkspaceInheritedMethodMissingAndAmbiguousAreMachineFacts(t *testing.T) {
	current, dependency := workspaceInheritanceFixture(t)
	resolver := workspaceResolverForTest(t, current, verifiedWorkspace(dependency))
	missing := resolver.ResolveInheritedCall("XxxServiceImpl.submit", "missingMethod")
	if missing.Status != NavigationPartial || missing.Limitation == nil || missing.Limitation.Code != "INHERITED_METHOD_NOT_FOUND" {
		t.Fatalf("expected inherited method missing, got %#v", missing)
	}

	writeJava(t, dependency, "src/main/java/com/company/framework/AbstractTemplate.java", `package com.company.framework;
public abstract class AbstractTemplate {
    public void execute() { validate(); doExecute(); }
    public void execute(String value) { doExecute(); }
    protected void validate() {}
    protected abstract void doExecute();
}`)
	ambiguous := resolver.ResolveInheritedCall("XxxServiceImpl.submit", "execute")
	if ambiguous.Status != NavigationPartial || ambiguous.Limitation == nil || ambiguous.Limitation.Code != "AMBIGUOUS_INHERITED_METHOD" {
		t.Fatalf("expected ambiguous inherited method, got %#v", ambiguous)
	}
}

func workspaceResolverForTest(t *testing.T, current string, verification workspace.VerificationResult) WorkspaceInheritanceResolver {
	t.Helper()
	astPath := os.Getenv("CODEA_AST_GREP_TEST_PATH")
	if astPath == "" {
		t.Skip("real ast-grep path not configured; dedicated workspace navigation gate runs these regressions")
	}
	abs, err := filepath.Abs(astPath)
	if err != nil { t.Fatal(err) }
	return WorkspaceInheritanceResolver{CurrentRoot: current, Dependency: verification, AstGrepPath: abs}
}

func workspaceInheritanceFixture(t *testing.T) (string, string) {
	t.Helper()
	parent := t.TempDir()
	current := filepath.Join(parent, "order-service")
	dependency := filepath.Join(parent, "company-framework")
	if err := os.MkdirAll(current, 0o755); err != nil { t.Fatal(err) }
	if err := os.MkdirAll(dependency, 0o755); err != nil { t.Fatal(err) }
	writeJava(t, dependency, "src/main/java/com/company/framework/AbstractTemplate.java", `package com.company.framework;
public abstract class AbstractTemplate {
    public void execute() {
        validate();
        doExecute();
    }
    protected void validate() {}
    protected abstract void doExecute();
}`)
	writeJava(t, current, "src/main/java/com/company/order/XxxServiceImpl.java", `package com.company.order;
import com.company.framework.AbstractTemplate;
public class XxxServiceImpl extends AbstractTemplate {
    public void submit() { execute(); }
    @Override protected void doExecute() { mapper.updateStatus(); }
}`)
	return current, dependency
}

func verifiedWorkspace(root string) workspace.VerificationResult {
	return workspace.VerificationResult{
		DependencyID: "company-framework",
		Status: workspace.StatusVerified,
		WorkspaceRoot: root,
		ConfirmedRoot: root,
		GroupID: "com.company",
		ArtifactID: "company-framework",
		CurrentVersion: "2.3.1",
		SourceVersion: "2.3.1",
	}
}

func writeJava(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { t.Fatal(err) }
}

func assertWorkspaceFact(t *testing.T, result WorkspaceNavigationResult, workspaceID, symbol, path, from string) {
	t.Helper()
	if result.Status != NavigationComplete || result.Fact == nil || result.Limitation != nil {
		t.Fatalf("expected COMPLETE fact, got %#v", result)
	}
	if result.Fact.Workspace != workspaceID || result.Fact.Symbol != symbol || result.Fact.Path != path || result.Fact.From != from || result.Fact.Source != "WORKSPACE_INHERITANCE" {
		t.Fatalf("unexpected fact: %#v", result.Fact)
	}
}
