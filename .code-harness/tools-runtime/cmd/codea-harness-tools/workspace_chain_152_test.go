package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"codea-harness-tools/internal/chain"
	"codea-harness-tools/internal/coverage"
	"codea-harness-tools/internal/nav"
	"codea-harness-tools/internal/schema"
	"codea-harness-tools/internal/workspace"
)

type workspaceBusinessScenario152 struct {
	versionMismatch  bool
	notConfigured    bool
	ambiguousOverride bool
	sourceMissing    bool
}

type workspaceBusinessResult152 struct {
	Status         string
	Code           string
	MavenStatus    string
	ChangedSources map[string][]string
	ChainText      string
}

func Test152RealDualProjectWorkspaceBusinessRegression(t *testing.T) {
	result := runWorkspaceBusinessRegression152(t, workspaceBusinessScenario152{})
	if result.Status != "COMPLETE" {
		t.Fatalf("expected COMPLETE, got %+v", result)
	}
	if result.MavenStatus != "VERIFIED" {
		t.Fatalf("expected VERIFIED Maven source, got %+v", result)
	}
	wantSources := map[string][]string{
		"src/main/java/com/company/order/XxxController.java":         {"STAGED"},
		"src/main/java/com/company/order/XxxServiceImpl.java":        {"UNSTAGED"},
		"src/main/resources/mapper/XxxMapper.xml":                     {"UNTRACKED"},
	}
	if !equalSourceMap152(result.ChangedSources, wantSources) {
		t.Fatalf("unexpected real Change Set sources: got=%v want=%v", result.ChangedSources, wantSources)
	}
	for _, token := range []string{
		"status: DISCOVERED",
		"workspace: company-framework",
		"XxxService.submit",
		"AbstractTemplate.execute",
		"XxxServiceImpl.doExecute",
		"XxxMapper.updateStatus",
		"XxxMapper.xml",
	} {
		if !strings.Contains(result.ChainText, token) {
			t.Fatalf("DISCOVERED Chain missing %q:\n%s", token, result.ChainText)
		}
	}
}

func Test152RealDualProjectWorkspaceBusinessFailureRegressions(t *testing.T) {
	tests := []struct {
		name     string
		scenario workspaceBusinessScenario152
		code     string
	}{
		{name: "version mismatch", scenario: workspaceBusinessScenario152{versionMismatch: true}, code: "WORKSPACE_DEPENDENCY_VERSION_MISMATCH"},
		{name: "not configured", scenario: workspaceBusinessScenario152{notConfigured: true}, code: "WORKSPACE_DEPENDENCY_NOT_CONFIGURED"},
		{name: "ambiguous override", scenario: workspaceBusinessScenario152{ambiguousOverride: true}, code: "AMBIGUOUS_TEMPLATE_DISPATCH"},
		{name: "source missing", scenario: workspaceBusinessScenario152{sourceMissing: true}, code: "WORKSPACE_DEPENDENCY_SOURCE_NOT_FOUND"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := runWorkspaceBusinessRegression152(t, tc.scenario)
			if result.Status != "PARTIAL" || result.Code != tc.code {
				t.Fatalf("expected PARTIAL / %s, got %+v", tc.code, result)
			}
			if result.ChainText != "" {
				t.Fatalf("failure scenario must not persist guessed Chain: %s", result.ChainText)
			}
		})
	}
}

func runWorkspaceBusinessRegression152(t *testing.T, scenario workspaceBusinessScenario152) workspaceBusinessResult152 {
	t.Helper()
	fixture := newWorkspaceBusinessFixture152(t, scenario)
	changedSources := collectWorkspaceBusinessChanges152(t, fixture.current)
	if len(changedSources) != 3 {
		t.Fatalf("expected exactly staged+unstaged+untracked business changes in current project, got %v", changedSources)
	}
	for p := range changedSources {
		if strings.Contains(filepath.ToSlash(p), "company-framework") || strings.HasPrefix(filepath.ToSlash(p), "../") {
			t.Fatalf("workspace dependency leaked into current Change Set: %s", p)
		}
	}

	configBytes, err := os.ReadFile(filepath.Join(fixture.current, ".code-harness", "harness.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	deps, err := workspace.ValidateConfigYAML(fixture.current, configBytes)
	if err != nil {
		t.Fatalf("workspace config semantic validation failed: %v", err)
	}
	if len(deps) == 0 {
		assertNoWorkspaceBusinessDiscovery152(t, fixture.current)
		return workspaceBusinessResult152{Status: "PARTIAL", Code: "WORKSPACE_DEPENDENCY_NOT_CONFIGURED", ChangedSources: changedSources}
	}
	verified := workspace.VerifyDirectMavenDependencies(fixture.current, []workspace.Dependency{deps[0]})
	if len(verified) != 1 {
		t.Fatalf("expected one Maven verification result, got %+v", verified)
	}
	mavenStatus := string(verified[0].Status)
	if verified[0].Status != workspace.StatusVerified {
		code := strings.TrimSpace(verified[0].Code)
		if code == "" {
			code = "WORKSPACE_DEPENDENCY_COORDINATE_MISMATCH"
		}
		assertNoWorkspaceBusinessDiscovery152(t, fixture.current)
		return workspaceBusinessResult152{Status: "PARTIAL", Code: code, MavenStatus: mavenStatus, ChangedSources: changedSources}
	}

	resolver := nav.WorkspaceInheritanceResolver{
		CurrentRoot: fixture.current,
		Dependency:  verified[0],
		AstGrepPath: "fixture-ast-grep",
		Runner:      businessAstRunner152{ambiguous: scenario.ambiguousOverride},
	}
	inherited := resolver.ResolveInheritedCall("XxxServiceImpl.submit", "execute")
	if inherited.Status != nav.NavigationComplete || inherited.Fact == nil {
		code := limitationCode152(inherited)
		assertNoWorkspaceBusinessDiscovery152(t, fixture.current)
		return workspaceBusinessResult152{Status: "PARTIAL", Code: code, MavenStatus: mavenStatus, ChangedSources: changedSources}
	}
	superCall := resolver.ResolveSuperclassCall("AbstractTemplate.execute", "validate")
	if superCall.Status != nav.NavigationComplete || superCall.Fact == nil {
		code := limitationCode152(superCall)
		assertNoWorkspaceBusinessDiscovery152(t, fixture.current)
		return workspaceBusinessResult152{Status: "PARTIAL", Code: code, MavenStatus: mavenStatus, ChangedSources: changedSources}
	}
	concrete := "XxxServiceImpl"
	if scenario.ambiguousOverride {
		concrete = ""
	}
	dispatch := resolver.ResolveTemplateDispatch("AbstractTemplate.execute", "doExecute", concrete)
	if dispatch.Status != nav.NavigationComplete || dispatch.Fact == nil {
		code := limitationCode152(dispatch)
		assertNoWorkspaceBusinessDiscovery152(t, fixture.current)
		return workspaceBusinessResult152{Status: "PARTIAL", Code: code, MavenStatus: mavenStatus, ChangedSources: changedSources}
	}

	if inherited.Fact.Workspace != "company-framework" || inherited.Fact.Source != "WORKSPACE_INHERITANCE" || inherited.Fact.Symbol != "AbstractTemplate.execute" {
		t.Fatalf("workspace inherited fact not machine-qualified: %+v", inherited.Fact)
	}
	if dispatch.Fact.Workspace != "current" || dispatch.Fact.Source != "WORKSPACE_INHERITANCE" || dispatch.Fact.Symbol != "XxxServiceImpl.doExecute" {
		t.Fatalf("template dispatch did not return to current override: %+v", dispatch.Fact)
	}

	locations := []chain.SymbolLocationEvidence{
		{Workspace: "current", Symbol: "XxxController.submit", Path: "src/main/java/com/company/order/XxxController.java", Role: "Controller", Source: "FIND_SYMBOL"},
		{Workspace: "current", Symbol: "XxxService.submit", Path: "src/main/java/com/company/order/XxxService.java", Role: "Service", Source: "FIND_SYMBOL", From: "XxxController.submit"},
		{Workspace: "current", Symbol: "XxxServiceImpl.submit", Path: "src/main/java/com/company/order/XxxServiceImpl.java", Role: "Service", Source: "FIND_IMPLEMENTATIONS", From: "XxxService.submit"},
		factLocation152(*inherited.Fact),
		factLocation152(*superCall.Fact),
		factLocation152(*dispatch.Fact),
		{Workspace: "current", Symbol: "XxxMapper.updateStatus", Path: "src/main/java/com/company/order/XxxMapper.java", Role: "Mapper", Source: "FIND_SYMBOL", From: "XxxServiceImpl.doExecute"},
	}
	for _, location := range locations {
		if location.Workspace != "current" {
			continue
		}
		mustContainWorkspaceBusinessSource152(t, fixture.current, location.Path, strings.Split(location.Symbol, ".")[0])
	}

	changed := changedEvidence152(changedSources)
	resource := chain.ResourceRelationEvidence{
		Path:       "src/main/resources/mapper/XxxMapper.xml",
		Role:       "MapperXml",
		Resource:   "XxxMapper.xml#updateStatus",
		FromSymbol: "XxxMapper.updateStatus",
		FromKind:   "METHOD",
		Source:     "MAPPER_STATEMENT",
		Evidence:   "statement id updateStatus matches XxxMapper.updateStatus",
	}
	analysisEvidence := chain.ChangeAnalysisEvidence{
		ChangedFiles: changed,
		AffectedControllers: []chain.AffectedControllerEvidence{{
			Controller: "XxxController", Endpoints: []string{"XxxController.submit"}, ImpactType: "DIRECT_CHANGE", SourceSymbols: []string{"XxxController.submit"},
		}},
		CallChains: []chain.CallChainEvidence{{
			EntryPoint: "XxxController.submit",
			Chain: []string{"XxxController.submit", "XxxService.submit", "XxxServiceImpl.submit", "AbstractTemplate.execute", "XxxServiceImpl.doExecute", "XxxMapper.updateStatus"},
		}},
		SymbolLocations:      locations,
		ResourceRelations:    []chain.ResourceRelationEvidence{resource},
		ExternalDependencies: []string{},
		ReviewCoverage:       chain.ReviewCoverageEvidence{UnresolvedSymbols: []chain.UnresolvedSymbolEvidence{}},
	}

	analysisJSON := buildWorkspaceBusinessAnalysisJSON152(t, fixture, changedSources, analysisEvidence)
	schemaBytes := readTask152Contract(t, "change-analysis.schema.json")
	if err := schema.ValidateJSON(schemaBytes, analysisJSON); err != nil {
		t.Fatalf("generated ChangeAnalysis schema validation failed: %v\n%s", err, analysisJSON)
	}
	machineCoverage, err := coverage.VerifyAnalysisJSON(analysisJSON)
	if err != nil || machineCoverage.Status != "COMPLETE" {
		t.Fatalf("generated ChangeAnalysis FULL machine coverage failed: result=%+v err=%v\n%s", machineCoverage, err, analysisJSON)
	}
	if strings.Contains(string(analysisJSON), `"reviewedFiles":[{"path":"src/main/java/com/company/framework/AbstractTemplate.java"`) {
		t.Fatal("dependency workspace source leaked into reviewCoverage.reviewedFiles")
	}

	runID := "run-task5-workspace-business"
	analysisPath := filepath.Join(fixture.current, ".code-harness", "runs", runID, "analysis", "change-analysis.json")
	mustWrite152(t, analysisPath, string(analysisJSON))
	if matches, _ := filepath.Glob(filepath.Join(fixture.current, ".code-harness", "runs", runID, "analysis", "discovered-chains", "*.yaml")); len(matches) != 0 {
		t.Fatalf("discovered Chain was prewritten before chain.Discover: %v", matches)
	}
	if _, err := os.Stat(filepath.Join(fixture.current, ".code-harness", "chains")); !os.IsNotExist(err) {
		t.Fatalf("historical chains must not be required/prewritten, err=%v", err)
	}

	discovery, err := chain.Discover(fixture.current, chain.DiscoverInput{RunID: runID, Target: "XxxController", ChangeAnalysis: analysisEvidence})
	if err != nil {
		t.Fatalf("direct workspace chain discover failed: %v", err)
	}
	if discovery.Status != chain.DiscoveryComplete || len(discovery.Chains) != 1 || len(discovery.Unresolved) != 0 {
		t.Fatalf("direct workspace chain discovery incomplete: %+v", discovery)
	}
	files, err := filepath.Glob(filepath.Join(fixture.current, ".code-harness", "runs", runID, "analysis", "discovered-chains", "*.yaml"))
	if err != nil || len(files) != 1 {
		t.Fatalf("expected exactly one persisted DISCOVERED Chain, files=%v err=%v", files, err)
	}
	chainBytes, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	chainText := string(chainBytes)

	if _, err := os.Stat(filepath.Join(fixture.dependency, ".code-harness")); !os.IsNotExist(err) {
		t.Fatalf("dependency workspace received run/state/write data: %v", err)
	}
	if got := mustReadWorkspaceBusiness152(t, filepath.Join(fixture.dependency, "src/main/java/com/company/framework/AbstractTemplate.java")); got != fixture.dependencySource {
		t.Fatal("dependency source was modified by navigation/discovery")
	}
	if _, err := os.Stat(filepath.Join(fixture.current, ".code-harness", "runs", runID, "review.md")); !os.IsNotExist(err) {
		t.Fatalf("Task 5 discovery must not create review Finding artifact, err=%v", err)
	}

	return workspaceBusinessResult152{Status: "COMPLETE", MavenStatus: mavenStatus, ChangedSources: changedSources, ChainText: chainText}
}

type workspaceBusinessFixture152 struct {
	current          string
	dependency       string
	baselineCommit   string
	dependencySource string
}

func newWorkspaceBusinessFixture152(t *testing.T, scenario workspaceBusinessScenario152) workspaceBusinessFixture152 {
	t.Helper()
	parent := t.TempDir()
	current := filepath.Join(parent, "order-service")
	dependency := filepath.Join(parent, "company-framework")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dependency, 0o755); err != nil {
		t.Fatal(err)
	}

	currentPOM := `<project><modelVersion>4.0.0</modelVersion><groupId>com.company</groupId><artifactId>order-service</artifactId><version>1.0.0</version><dependencies><dependency><groupId>com.company</groupId><artifactId>company-framework</artifactId><version>2.3.1</version></dependency></dependencies></project>`
	mustWrite152(t, filepath.Join(current, "pom.xml"), currentPOM)
	if !scenario.sourceMissing {
		depVersion := "2.3.1"
		if scenario.versionMismatch {
			depVersion = "2.4.0"
		}
		mustWrite152(t, filepath.Join(dependency, "pom.xml"), `<project><modelVersion>4.0.0</modelVersion><groupId>com.company</groupId><artifactId>company-framework</artifactId><version>`+depVersion+`</version></project>`)
	}
	dependencySource := `package com.company.framework;
public abstract class AbstractTemplate {
    public void execute() {
        validate();
        doExecute();
    }
    protected void validate() {}
    protected abstract void doExecute();
}
`
	mustWrite152(t, filepath.Join(dependency, "src/main/java/com/company/framework/AbstractTemplate.java"), dependencySource)

	harness := "version: 2\n"
	if !scenario.notConfigured {
		harness += `workspaceDependencies:
  - id: company-framework
    root: ../company-framework
    maven:
      groupId: com.company
      artifactId: company-framework
    mode: READ_ONLY
`
	}
	mustWrite152(t, filepath.Join(current, ".code-harness", "harness.yaml"), harness)
	mustWrite152(t, filepath.Join(current, ".gitignore"), ".code-harness/\n")

	controller := `package com.company.order;
public class XxxController {
    private XxxService service;
    public void submit() { service.submit(); }
}
`
	service := `package com.company.order;
public interface XxxService {
    void submit();
}
`
	serviceImpl := `package com.company.order;
import com.company.framework.AbstractTemplate;
public class XxxServiceImpl extends AbstractTemplate implements XxxService {
    private XxxMapper mapper;
    public void submit() { execute(); }
    @Override protected void doExecute() { mapper.updateStatus(); }
}
`
	mapper := `package com.company.order;
public interface XxxMapper {
    void updateStatus();
}
`
	mustWrite152(t, filepath.Join(current, "src/main/java/com/company/order/XxxController.java"), controller)
	mustWrite152(t, filepath.Join(current, "src/main/java/com/company/order/XxxService.java"), service)
	mustWrite152(t, filepath.Join(current, "src/main/java/com/company/order/XxxServiceImpl.java"), serviceImpl)
	mustWrite152(t, filepath.Join(current, "src/main/java/com/company/order/XxxMapper.java"), mapper)
	if scenario.ambiguousOverride {
		mustWrite152(t, filepath.Join(current, "src/main/java/com/company/order/AnotherServiceImpl.java"), `package com.company.order;
import com.company.framework.AbstractTemplate;
public class AnotherServiceImpl extends AbstractTemplate {
    @Override protected void doExecute() {}
}
`)
	}

	gitWorkspaceBusiness152(t, current, "init", "-b", "develop")
	gitWorkspaceBusiness152(t, current, "config", "user.email", "codea@example.invalid")
	gitWorkspaceBusiness152(t, current, "config", "user.name", "Codea Test")
	gitWorkspaceBusiness152(t, current, "add", ".gitignore", "pom.xml", "src/main/java")
	gitWorkspaceBusiness152(t, current, "commit", "-m", "business baseline")
	baseline := strings.TrimSpace(gitWorkspaceBusiness152(t, current, "rev-parse", "HEAD"))

	mustWrite152(t, filepath.Join(current, "src/main/java/com/company/order/XxxController.java"), strings.Replace(controller, "public void submit() { service.submit(); }", "public void submit() { /* staged business change */ service.submit(); }", 1))
	gitWorkspaceBusiness152(t, current, "add", "src/main/java/com/company/order/XxxController.java")
	mustWrite152(t, filepath.Join(current, "src/main/java/com/company/order/XxxServiceImpl.java"), strings.Replace(serviceImpl, "public void submit() { execute(); }", "public void submit() { /* unstaged business change */ execute(); }", 1))
	mustWrite152(t, filepath.Join(current, "src/main/resources/mapper/XxxMapper.xml"), `<mapper namespace="com.company.order.XxxMapper"><update id="updateStatus">update orders set status = #{status} where id = #{id}</update></mapper>`)

	return workspaceBusinessFixture152{current: current, dependency: dependency, baselineCommit: baseline, dependencySource: dependencySource}
}

func collectWorkspaceBusinessChanges152(t *testing.T, root string) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	collect := func(source, raw string) {
		for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
			p := filepath.ToSlash(strings.TrimSpace(line))
			if p == "" {
				continue
			}
			out[p] = append(out[p], source)
		}
	}
	collect("STAGED", gitWorkspaceBusiness152(t, root, "diff", "--cached", "--name-only", "--diff-filter=ACMR"))
	collect("UNSTAGED", gitWorkspaceBusiness152(t, root, "diff", "--name-only", "--diff-filter=ACMR"))
	collect("UNTRACKED", gitWorkspaceBusiness152(t, root, "ls-files", "--others", "--exclude-standard"))
	for p := range out {
		sort.Strings(out[p])
	}
	return out
}

func changedEvidence152(sources map[string][]string) []chain.ChangedFileEvidence {
	paths := make([]string, 0, len(sources))
	for p := range sources {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	out := make([]chain.ChangedFileEvidence, 0, len(paths))
	for _, p := range paths {
		role := "Other"
		switch {
		case strings.HasSuffix(p, "XxxController.java"):
			role = "Controller"
		case strings.HasSuffix(p, "XxxServiceImpl.java"):
			role = "Service"
		case strings.HasSuffix(p, "XxxMapper.xml"):
			role = "MapperXml"
		}
		out = append(out, chain.ChangedFileEvidence{Path: p, Role: role})
	}
	return out
}

func buildWorkspaceBusinessAnalysisJSON152(t *testing.T, fixture workspaceBusinessFixture152, sources map[string][]string, evidence chain.ChangeAnalysisEvidence) []byte {
	t.Helper()
	changed := make([]map[string]any, 0, len(sources))
	paths := make([]string, 0, len(sources))
	for p := range sources {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		role := "Other"
		switch {
		case strings.HasSuffix(p, "XxxController.java"):
			role = "Controller"
		case strings.HasSuffix(p, "XxxServiceImpl.java"):
			role = "Service"
		case strings.HasSuffix(p, "XxxMapper.xml"):
			role = "MapperXml"
		}
		changed = append(changed, map[string]any{"path": p, "role": role, "sources": sources[p]})
	}
	roleByPath := map[string]string{
		"src/main/java/com/company/order/XxxController.java":  "Controller",
		"src/main/java/com/company/order/XxxService.java":     "Service",
		"src/main/java/com/company/order/XxxServiceImpl.java": "Service",
		"src/main/java/com/company/order/XxxMapper.java":      "Mapper",
		"src/main/resources/mapper/XxxMapper.xml":              "MapperXml",
	}
	reviewedPaths := make([]string, 0, len(roleByPath))
	for p := range roleByPath {
		reviewedPaths = append(reviewedPaths, p)
	}
	sort.Strings(reviewedPaths)
	reviewed := make([]map[string]any, 0, len(reviewedPaths))
	for _, p := range reviewedPaths {
		reason := "CALL_CHAIN"
		if _, changedFile := sources[p]; changedFile {
			reason = "CHANGED"
		}
		reviewed = append(reviewed, map[string]any{"path": p, "role": roleByPath[p], "reason": reason})
	}
	doc := map[string]any{
		"reviewScope": map[string]any{
			"currentBranch": "develop", "baseRef": "HEAD", "baseCommit": fixture.baselineCommit, "mergeBase": fixture.baselineCommit, "headCommit": fixture.baselineCommit, "includeWorkingTree": true,
		},
		"changedFiles":         changed,
		"affectedControllers": evidence.AffectedControllers,
		"callChains":          evidence.CallChains,
		"symbolLocations":     evidence.SymbolLocations,
		"resourceRelations":   evidence.ResourceRelations,
		"externalDependencies": []string{},
		"riskAreas":            []any{},
		"reviewCoverage": map[string]any{
			"status": "COMPLETE", "reviewedFiles": reviewed, "unresolvedSymbols": []any{},
		},
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func limitationCode152(result nav.WorkspaceNavigationResult) string {
	if result.Limitation == nil || strings.TrimSpace(result.Limitation.Code) == "" {
		return "WORKSPACE_NAVIGATION_PARTIAL"
	}
	return result.Limitation.Code
}

func factLocation152(fact nav.WorkspaceNavigationFact) chain.SymbolLocationEvidence {
	return chain.SymbolLocationEvidence{Workspace: fact.Workspace, Symbol: fact.Symbol, Path: fact.Path, Role: fact.Role, Source: fact.Source, From: fact.From}
}

func assertNoWorkspaceBusinessDiscovery152(t *testing.T, root string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(root, ".code-harness", "runs", "*", "analysis", "discovered-chains", "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("PARTIAL scenario persisted guessed discovered Chain: %v", matches)
	}
}

func readTask152Contract(t *testing.T, name string) []byte {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../contracts", name))
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read contract %s: %v", path, err)
	}
	return b
}

func mustContainWorkspaceBusinessSource152(t *testing.T, root, rel, token string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read current navigation evidence %s: %v", rel, err)
	}
	if !strings.Contains(string(b), token) {
		t.Fatalf("current navigation evidence %s does not contain %q", rel, token)
	}
}

func mustReadWorkspaceBusiness152(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func gitWorkspaceBusiness152(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmdArgs := append([]string{"-C", root}, args...)
	cmd := exec.Command("git", cmdArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(cmdArgs, " "), err, out)
	}
	return string(out)
}

func equalSourceMap152(left, right map[string][]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, want := range right {
		got, ok := left[key]
		if !ok || strings.Join(got, "\x00") != strings.Join(want, "\x00") {
			return false
		}
	}
	return true
}

type businessAstRunner152 struct {
	ambiguous bool
}

func (r businessAstRunner152) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	if len(args) < 6 {
		return nil, fmt.Errorf("unexpected ast-grep args: %v", args)
	}
	pattern := args[4]
	sourceRoot := args[len(args)-1]
	if strings.Contains(filepath.ToSlash(sourceRoot), "/company-framework/") {
		return r.frameworkOutput152(pattern, sourceRoot), nil
	}
	return r.currentOutput152(pattern, sourceRoot), nil
}

func (r businessAstRunner152) frameworkOutput152(pattern, sourceRoot string) []byte {
	file := filepath.Join(sourceRoot, "com", "company", "framework", "AbstractTemplate.java")
	var lines [][]byte
	if strings.Contains(pattern, "class AbstractTemplate") && !strings.Contains(pattern, "extends") {
		lines = append(lines, astLine152(file, "public abstract class AbstractTemplate", 1, 20, nil))
	}
	if strings.Contains(pattern, "class $C") && !strings.Contains(pattern, "extends") {
		lines = append(lines, astLine152(file, "public abstract class AbstractTemplate", 1, 20, map[string]string{"C": "AbstractTemplate"}))
	}
	if strings.Contains(pattern, "$RET execute(") {
		lines = append(lines, astLine152(file, "public void execute()", 3, 7, nil))
	}
	if strings.Contains(pattern, "$RET validate(") {
		lines = append(lines, astLine152(file, "protected void validate()", 9, 10, nil))
	}
	if strings.Contains(pattern, "$RET doExecute(") {
		lines = append(lines, astLine152(file, "protected abstract void doExecute()", 11, 12, nil))
	}
	if pattern == "validate($$$ARGS)" {
		lines = append(lines, astLine152(file, "validate()", 4, 4, nil))
	}
	if pattern == "doExecute($$$ARGS)" {
		lines = append(lines, astLine152(file, "doExecute()", 5, 5, nil))
	}
	return joinAstLines152(lines)
}

func (r businessAstRunner152) currentOutput152(pattern, sourceRoot string) []byte {
	serviceFile := filepath.Join(sourceRoot, "com", "company", "order", "XxxServiceImpl.java")
	anotherFile := filepath.Join(sourceRoot, "com", "company", "order", "AnotherServiceImpl.java")
	var lines [][]byte
	if strings.Contains(pattern, "class XxxServiceImpl") && strings.Contains(pattern, "extends $SUPER") {
		lines = append(lines, astLine152(serviceFile, "public class XxxServiceImpl extends AbstractTemplate implements XxxService", 1, 20, map[string]string{"SUPER": "AbstractTemplate"}))
	}
	if strings.Contains(pattern, "class $C extends AbstractTemplate") {
		lines = append(lines, astLine152(serviceFile, "public class XxxServiceImpl extends AbstractTemplate implements XxxService", 1, 20, map[string]string{"C": "XxxServiceImpl"}))
		if r.ambiguous {
			lines = append(lines, astLine152(anotherFile, "public class AnotherServiceImpl extends AbstractTemplate", 1, 10, map[string]string{"C": "AnotherServiceImpl"}))
		}
	}
	if strings.Contains(pattern, "class $C") && strings.Contains(pattern, "extends $SUPER") {
		lines = append(lines, astLine152(serviceFile, "public class XxxServiceImpl extends AbstractTemplate implements XxxService", 1, 20, map[string]string{"C": "XxxServiceImpl", "SUPER": "AbstractTemplate"}))
		if r.ambiguous {
			lines = append(lines, astLine152(anotherFile, "public class AnotherServiceImpl extends AbstractTemplate", 1, 10, map[string]string{"C": "AnotherServiceImpl", "SUPER": "AbstractTemplate"}))
		}
	}
	if strings.Contains(pattern, "$RET submit(") {
		lines = append(lines, astLine152(serviceFile, "public void submit()", 5, 7, nil))
	}
	if strings.Contains(pattern, "$RET doExecute(") {
		lines = append(lines, astLine152(serviceFile, "protected void doExecute()", 9, 11, nil))
		if r.ambiguous {
			lines = append(lines, astLine152(anotherFile, "protected void doExecute()", 4, 6, nil))
		}
	}
	if pattern == "execute($$$ARGS)" {
		lines = append(lines, astLine152(serviceFile, "execute()", 6, 6, nil))
	}
	return joinAstLines152(lines)
}

func astLine152(file, text string, startLine, endLine int, meta map[string]string) []byte {
	single := map[string]map[string]string{}
	for key, value := range meta {
		single[key] = map[string]string{"text": value}
	}
	line := map[string]any{
		"file": file,
		"text": text,
		"range": map[string]any{
			"start": map[string]int{"line": startLine - 1, "column": 0},
			"end":   map[string]int{"line": endLine - 1, "column": 80},
		},
		"metaVariables": map[string]any{"single": single},
	}
	b, _ := json.Marshal(line)
	return append(b, '\n')
}

func joinAstLines152(lines [][]byte) []byte {
	var out []byte
	for _, line := range lines {
		out = append(out, line...)
	}
	return out
}
