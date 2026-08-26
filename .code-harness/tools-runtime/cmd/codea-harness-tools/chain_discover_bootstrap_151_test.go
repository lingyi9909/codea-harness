package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func Test151ChainDiscoverBootstrapContractIsSelfContained(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", "..", "..", ".."))
	paths := []string{
		filepath.Join(repoRoot, ".code-harness", "agents", "orchestrator.md"),
		filepath.Join(repoRoot, ".code-harness", "agents", "reviewer.md"),
		filepath.Join(repoRoot, ".code-harness", "skills", "analyze-change", "SKILL.md"),
		filepath.Join(repoRoot, ".code-harness", "skills", "discover-chain", "SKILL.md"),
	}
	texts := map[string]string{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		texts[path] = string(data)
	}

	orchestrator := texts[paths[0]]
	if !strings.Contains(orchestrator, "| `harness chain discover [target]` | Reviewer → discover-chain |") {
		t.Fatal("orchestrator must continue routing direct chain discover to Reviewer → discover-chain")
	}
	reviewer := texts[paths[1]]
	if !strings.Contains(reviewer, "analyze-change → discover-chain → Controlled Runtime") {
		t.Fatal("Reviewer must keep coordinating analyze-change → discover-chain → Controlled Runtime")
	}
	analyzeChange := texts[paths[2]]
	for _, want := range []string{"committed", "staged", "unstaged", "untracked", "生产 Controller Method"} {
		if !strings.Contains(analyzeChange, want) {
			t.Fatalf("analyze-change must preserve Change Set/new entrypoint semantics %q", want)
		}
	}

	discoverSkill := texts[paths[3]]
	for _, want := range []string{
		"Chain Discover Bootstrap（1.5.1）",
		"harness chain discover [target] 是自包含流程",
		"current Change Set",
		"analyze-change",
		"ChangeAnalysis Schema validate",
		"Runtime machine coverage verify",
		"chain discover",
		"DISCOVERED Chain",
		"不得要求用户先执行 harness review",
		"source revision / Change Set",
		"不存在或已过期时自动重新 analyze-change",
		"COMMITTED / STAGED / UNSTAGED / UNTRACKED",
		"新增 production Controller Method",
		"PARTIAL",
		"未解析",
		"原因",
		"IMPLEMENTATION_NOT_FOUND",
		"reviewCoverage.unresolvedSymbols",
	} {
		if !strings.Contains(discoverSkill, want) {
			t.Fatalf("discover-chain missing 1.5.1 bootstrap contract %q", want)
		}
	}

	for path, text := range texts {
		for _, forbidden := range []string{
			"Chain 通常依赖历史沉淀",
			"新增 Controller 首次可能无法自动形成 Chain",
			"需要先经过 harness review 正式 Coverage",
			"可能需要额外 Code Navigation",
		} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s contains forbidden discover guidance %q", path, forbidden)
			}
		}
	}
}

func Test151DirectDiscoverSupportsFreshAddedControllerServiceMapperStack(t *testing.T) {
	withTempProject(t)
	installChangeAnalysisSchema(t)
	setup151FreshGitFixture(t)

	runID := "run-151-bootstrap"
	analysisPath := filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json")
	if _, err := os.Stat(filepath.Join(".code-harness", "runs")); !os.IsNotExist(err) {
		t.Fatalf("precondition: no historical run allowed, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(".code-harness", "chains")); !os.IsNotExist(err) {
		t.Fatalf("precondition: no historical chains/** allowed, err=%v", err)
	}
	if _, err := os.Stat(analysisPath); !os.IsNotExist(err) {
		t.Fatalf("precondition: test must not prewrite change-analysis.json, err=%v", err)
	}
	beforeHarnessFiles := fileSet151(t, ".code-harness")

	trace, err := executeHarnessDiscoverIntent151(t, "harness chain discover NewController", runID)
	if err != nil {
		t.Fatal(err)
	}
	wantTrace := []string{
		"current Change Set",
		"analyze-change",
		"ChangeAnalysis Schema validate",
		"Runtime machine coverage verify",
		"chain discover",
		"DISCOVERED Chain",
	}
	if strings.Join(trace, " -> ") != strings.Join(wantTrace, " -> ") {
		t.Fatalf("bootstrap trace=%v want=%v", trace, wantTrace)
	}

	analysisBytes, err := os.ReadFile(analysisPath)
	if err != nil {
		t.Fatalf("bootstrap must generate change-analysis.json: %v", err)
	}
	var generated struct {
		ChangedFiles []struct {
			Path    string   `json:"path"`
			Sources []string `json:"sources"`
		} `json:"changedFiles"`
	}
	if err := json.Unmarshal(analysisBytes, &generated); err != nil {
		t.Fatalf("generated change-analysis.json invalid JSON: %v", err)
	}
	if len(generated.ChangedFiles) != 5 {
		t.Fatalf("generated Change Set files=%d want 5: %s", len(generated.ChangedFiles), analysisBytes)
	}
	sourceKinds := map[string]bool{}
	for _, f := range generated.ChangedFiles {
		for _, source := range f.Sources {
			sourceKinds[source] = true
		}
	}
	for _, want := range []string{"STAGED", "UNSTAGED", "UNTRACKED"} {
		if !sourceKinds[want] {
			t.Fatalf("generated Change Set missing %s source: %s", want, analysisBytes)
		}
	}

	discoveredDir := filepath.Join(".code-harness", "runs", runID, "analysis", "discovered-chains")
	entries, err := os.ReadDir(discoveredDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected exactly one DISCOVERED chain, entries=%v err=%v", entries, err)
	}
	yamlBytes, err := os.ReadFile(filepath.Join(discoveredDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	yamlText := string(yamlBytes)
	for _, want := range []string{"status: DISCOVERED", "NewController.approve", "ApprovalService.approve", "ApprovalServiceImpl.approve", "ApprovalMapper.updateStatus", "MAPPER_XML"} {
		if !strings.Contains(yamlText, want) {
			t.Fatalf("discovered YAML missing %q:\n%s", want, yamlText)
		}
	}

	if _, err := os.Stat(filepath.Join(".code-harness", "chains")); !os.IsNotExist(err) {
		t.Fatalf("direct discovery must not create chains/**, err=%v", err)
	}
	if hasFileNamed151(t, filepath.Join(".code-harness", "runs"), "review.md") {
		t.Fatal("direct discovery must not generate review.md")
	}
	afterHarnessFiles := fileSet151(t, ".code-harness")
	for path := range afterHarnessFiles {
		if beforeHarnessFiles[path] {
			continue
		}
		if !strings.HasPrefix(path, "runs/") {
			t.Fatalf("direct discovery wrote outside runs/**: %s", path)
		}
	}
}

func setup151FreshGitFixture(t *testing.T) {
	t.Helper()
	git151(t, "init", "-b", "develop")
	git151(t, "config", "user.email", "codea@example.invalid")
	git151(t, "config", "user.name", "Codea Test")
	git151(t, "add", ".code-harness/contracts/change-analysis.schema.json")
	git151(t, "commit", "-m", "baseline")
	git151(t, "checkout", "-b", "feature/new-approval")

	writeFile(t, "src/main/java/com/example/approval/NewController.java", `package com.example.approval;
@RestController
public class NewController {
    private final ApprovalService approvalService;
    public NewController(ApprovalService approvalService) { this.approvalService = approvalService; }
    @PostMapping("/approve")
    public void approve() { approvalService.approve(); }
}
`)
	writeFile(t, "src/main/java/com/example/approval/ApprovalService.java", `package com.example.approval;
public interface ApprovalService { void approve(); }
`)
	writeFile(t, "src/main/java/com/example/approval/ApprovalServiceImpl.java", `package com.example.approval;
@Service
public class ApprovalServiceImpl implements ApprovalService {
    private final ApprovalMapper mapper;
    public ApprovalServiceImpl(ApprovalMapper mapper) { this.mapper = mapper; }
    public void approve() { mapper.updateStatus(); }
}
`)
	writeFile(t, "src/main/java/com/example/approval/ApprovalMapper.java", `package com.example.approval;
@Mapper
public interface ApprovalMapper { void updateStatus(); }
`)
	writeFile(t, "src/main/resources/mapper/ApprovalMapper.xml", `<mapper namespace="com.example.approval.ApprovalMapper"><update id="updateStatus">update approval set status='DONE'</update></mapper>`)

	git151(t, "add",
		"src/main/java/com/example/approval/NewController.java",
		"src/main/java/com/example/approval/ApprovalService.java",
		"src/main/java/com/example/approval/ApprovalServiceImpl.java",
		"src/main/java/com/example/approval/ApprovalMapper.java",
	)
	f, err := os.OpenFile("src/main/java/com/example/approval/ApprovalServiceImpl.java", os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("// unstaged follow-up\n"); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func executeHarnessDiscoverIntent151(t *testing.T, intent, runID string) ([]string, error) {
	t.Helper()
	parts := strings.Fields(intent)
	if len(parts) != 4 || parts[0] != "harness" || parts[1] != "chain" || parts[2] != "discover" || parts[3] == "" {
		return nil, fmt.Errorf("unsupported harness intent %q", intent)
	}
	target := parts[3]
	trace := []string{"current Change Set"}

	analysis, err := analyzeCurrentChangeSet151(t, target)
	if err != nil {
		return trace, err
	}
	trace = append(trace, "analyze-change")

	analysisPath := filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json")
	if _, err := os.Stat(analysisPath); !os.IsNotExist(err) {
		return trace, fmt.Errorf("bootstrap must start without existing ChangeAnalysis: %v", err)
	}
	analysisJSON, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return trace, err
	}
	writeFile(t, analysisPath, string(analysisJSON))

	if err := run([]string{
		"validate",
		"--schema", filepath.Join(".code-harness", "contracts", "change-analysis.schema.json"),
		"--input", analysisPath,
		"--format", "json",
	}); err != nil {
		return trace, fmt.Errorf("ChangeAnalysis schema/coverage verification failed: %w", err)
	}
	trace = append(trace, "ChangeAnalysis Schema validate", "Runtime machine coverage verify")

	request := map[string]any{
		"runId":              runID,
		"target":             target,
		"changeAnalysisPath": filepath.ToSlash(analysisPath),
	}
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return trace, err
	}
	requestPath := writeQueryRequest(t, runID, "chain-discover.json", string(requestJSON))
	if err := run([]string{"chain", "discover", "--input", requestPath}); err != nil {
		return trace, err
	}
	trace = append(trace, "chain discover")

	discoveredDir := filepath.Join(".code-harness", "runs", runID, "analysis", "discovered-chains")
	entries, err := os.ReadDir(discoveredDir)
	if err != nil || len(entries) == 0 {
		return trace, fmt.Errorf("chain discover produced no DISCOVERED Chain: entries=%v err=%v", entries, err)
	}
	trace = append(trace, "DISCOVERED Chain")
	return trace, nil
}

func analyzeCurrentChangeSet151(t *testing.T, target string) (map[string]any, error) {
	t.Helper()
	sources := collectChangeSet151(t)
	if len(sources) == 0 {
		return nil, fmt.Errorf("current Change Set is empty")
	}

	contents := map[string]string{}
	types := map[string]string{}
	roles := map[string]string{}
	implementedServiceInterfaces := map[string]bool{}

	for path := range sources {
		data, err := os.ReadFile(filepath.FromSlash(path))
		if err != nil {
			return nil, err
		}
		contents[path] = string(data)
		if strings.HasSuffix(path, ".java") {
			name := javaTypeName151(string(data))
			if name == "" {
				return nil, fmt.Errorf("TYPE_NOT_FOUND: %s", path)
			}
			types[name] = path
			if strings.Contains(string(data), "@Service") {
				roles[path] = "Service"
				if iface := implementedInterface151(string(data)); iface != "" {
					implementedServiceInterfaces[iface] = true
				}
			}
		}
	}

	for path, content := range contents {
		switch {
		case strings.HasSuffix(path, ".xml") && strings.Contains(content, "<mapper"):
			roles[path] = "MapperXml"
		case strings.Contains(content, "@RestController"):
			roles[path] = "Controller"
		case strings.Contains(content, "@Mapper"):
			roles[path] = "Mapper"
		case implementedServiceInterfaces[javaTypeName151(content)]:
			roles[path] = "Service"
		}
		if roles[path] == "" {
			return nil, fmt.Errorf("ROLE_NOT_VERIFIED: %s", path)
		}
	}

	controllerPath := types[target]
	if controllerPath == "" || roles[controllerPath] != "Controller" {
		return nil, fmt.Errorf("CONTROLLER_ENTRYPOINT_NOT_FOUND: %s", target)
	}
	controllerMethod := controllerEndpointMethod151(contents[controllerPath])
	if controllerMethod == "" {
		return nil, fmt.Errorf("CONTROLLER_METHOD_NOT_FOUND: %s", target)
	}
	controllerFields := javaFields151(contents[controllerPath])
	serviceVar, serviceMethod := firstFieldCall151(contents[controllerPath], controllerFields)
	if serviceVar == "" {
		return nil, fmt.Errorf("SERVICE_CALL_NOT_FOUND: %s.%s", target, controllerMethod)
	}
	serviceType := controllerFields[serviceVar]
	servicePath := types[serviceType]
	if servicePath == "" || roles[servicePath] != "Service" {
		return nil, fmt.Errorf("SERVICE_NOT_FOUND: %s", serviceType)
	}

	implType, implPath := implementationFor151(contents, roles, serviceType)
	if implPath == "" {
		return nil, fmt.Errorf("IMPLEMENTATION_NOT_FOUND: %s.%s", serviceType, serviceMethod)
	}
	implFields := javaFields151(contents[implPath])
	mapperVar, mapperMethod := firstFieldCall151(contents[implPath], implFields)
	if mapperVar == "" {
		return nil, fmt.Errorf("MAPPER_CALL_NOT_FOUND: %s.%s", implType, serviceMethod)
	}
	mapperType := implFields[mapperVar]
	mapperPath := types[mapperType]
	if mapperPath == "" || roles[mapperPath] != "Mapper" {
		return nil, fmt.Errorf("MAPPER_NOT_FOUND: %s", mapperType)
	}

	xmlPath := ""
	for path, content := range contents {
		if roles[path] != "MapperXml" {
			continue
		}
		if strings.Contains(content, "namespace=\"com.example.approval."+mapperType+"\"") && strings.Contains(content, "id=\""+mapperMethod+"\"") {
			xmlPath = path
			break
		}
	}
	if xmlPath == "" {
		return nil, fmt.Errorf("MAPPER_XML_STATEMENT_NOT_FOUND: %s.%s", mapperType, mapperMethod)
	}

	paths := make([]string, 0, len(sources))
	for path := range sources {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	changedFiles := make([]map[string]any, 0, len(paths))
	reviewedFiles := make([]map[string]any, 0, len(paths))
	for _, path := range paths {
		changedFiles = append(changedFiles, map[string]any{
			"path":    path,
			"role":    roles[path],
			"sources": sources[path],
		})
		reviewedFiles = append(reviewedFiles, map[string]any{
			"path":   path,
			"role":   roles[path],
			"reason": "CHANGED",
		})
	}

	mergeBase := git151(t, "merge-base", "develop", "HEAD")
	head := git151(t, "rev-parse", "HEAD")
	branch := git151(t, "rev-parse", "--abbrev-ref", "HEAD")
	controllerSymbol := target + "." + controllerMethod
	serviceSymbol := serviceType + "." + serviceMethod
	implSymbol := implType + "." + serviceMethod
	mapperSymbol := mapperType + "." + mapperMethod

	return map[string]any{
		"reviewScope": map[string]any{
			"currentBranch":      branch,
			"baseRef":            "develop",
			"baseCommit":         mergeBase,
			"mergeBase":          mergeBase,
			"headCommit":         head,
			"includeWorkingTree": true,
		},
		"changedFiles": changedFiles,
		"affectedControllers": []map[string]any{{
			"controller":    target,
			"endpoints":     []string{controllerSymbol},
			"impactType":    "DIRECT_CHANGE",
			"sourceSymbols": []string{controllerSymbol},
		}},
		"callChains": []map[string]any{{
			"entryPoint": controllerSymbol,
			"chain":      []string{controllerSymbol, serviceSymbol, implSymbol, mapperSymbol},
		}},
		"symbolLocations": []map[string]any{
			{"symbol": controllerSymbol, "path": controllerPath, "role": "Controller", "source": "FIND_SYMBOL"},
			{"symbol": serviceSymbol, "path": servicePath, "role": "Service", "source": "FIND_SYMBOL"},
			{"symbol": implSymbol, "path": implPath, "role": "Service", "source": "FIND_IMPLEMENTATIONS", "from": serviceSymbol},
			{"symbol": mapperSymbol, "path": mapperPath, "role": "Mapper", "source": "FIND_SYMBOL"},
		},
		"resourceRelations": []map[string]any{{
			"path":       xmlPath,
			"role":       "MapperXml",
			"resource":   filepath.Base(xmlPath) + "#" + mapperMethod,
			"fromSymbol": mapperSymbol,
			"fromKind":   "METHOD",
			"source":     "MAPPER_STATEMENT",
			"evidence":   "statement id " + mapperMethod + " matches " + mapperSymbol,
		}},
		"externalDependencies": []any{},
		"riskAreas":             []any{},
		"reviewCoverage": map[string]any{
			"status":            "COMPLETE",
			"reviewedFiles":     reviewedFiles,
			"unresolvedSymbols": []any{},
		},
	}, nil
}

func collectChangeSet151(t *testing.T) map[string][]string {
	t.Helper()
	sets := map[string]map[string]bool{}
	add := func(source, output string) {
		for _, path := range lines151(output) {
			if path == "" {
				continue
			}
			if sets[path] == nil {
				sets[path] = map[string]bool{}
			}
			sets[path][source] = true
		}
	}
	mergeBase := git151(t, "merge-base", "develop", "HEAD")
	add("COMMITTED", git151(t, "diff", "--name-only", mergeBase+"..HEAD"))
	add("STAGED", git151(t, "diff", "--cached", "--name-only", "--diff-filter=ACMR"))
	add("UNSTAGED", git151(t, "diff", "--name-only", "--diff-filter=ACMR"))
	add("UNTRACKED", git151(t, "ls-files", "--others", "--exclude-standard"))

	order := []string{"COMMITTED", "STAGED", "UNSTAGED", "UNTRACKED"}
	result := map[string][]string{}
	for path, sourceSet := range sets {
		if !strings.HasPrefix(path, "src/main/") {
			continue
		}
		for _, source := range order {
			if sourceSet[source] {
				result[path] = append(result[path], source)
			}
		}
	}
	return result
}

func javaTypeName151(content string) string {
	m := regexp.MustCompile(`(?m)\b(?:class|interface)\s+([A-Za-z_][A-Za-z0-9_]*)`).FindStringSubmatch(content)
	if len(m) == 2 {
		return m[1]
	}
	return ""
}

func implementedInterface151(content string) string {
	m := regexp.MustCompile(`(?m)\bimplements\s+([A-Za-z_][A-Za-z0-9_]*)`).FindStringSubmatch(content)
	if len(m) == 2 {
		return m[1]
	}
	return ""
}

func controllerEndpointMethod151(content string) string {
	m := regexp.MustCompile(`(?s)@PostMapping(?:\([^)]*\))?\s*public\s+[A-Za-z0-9_<>,?\[\] ]+\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`).FindStringSubmatch(content)
	if len(m) == 2 {
		return m[1]
	}
	return ""
}

func javaFields151(content string) map[string]string {
	fields := map[string]string{}
	re := regexp.MustCompile(`(?m)(?:private|protected|public)\s+final\s+([A-Za-z_][A-Za-z0-9_]*)\s+([A-Za-z_][A-Za-z0-9_]*)\s*;`)
	for _, m := range re.FindAllStringSubmatch(content, -1) {
		fields[m[2]] = m[1]
	}
	return fields
}

func firstFieldCall151(content string, fields map[string]string) (string, string) {
	re := regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	for _, m := range re.FindAllStringSubmatch(content, -1) {
		if fields[m[1]] != "" {
			return m[1], m[2]
		}
	}
	return "", ""
}

func implementationFor151(contents, roles map[string]string, serviceType string) (string, string) {
	needle := regexp.MustCompile(`\bimplements\s+` + regexp.QuoteMeta(serviceType) + `\b`)
	for path, content := range contents {
		if roles[path] == "Service" && needle.MatchString(content) {
			return javaTypeName151(content), path
		}
	}
	return "", ""
}

func lines151(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(strings.TrimSpace(s), "\n")
	for i := range parts {
		parts[i] = filepath.ToSlash(strings.TrimSpace(parts[i]))
	}
	return parts
}

func git151(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func fileSet151(t *testing.T, root string) map[string]bool {
	t.Helper()
	result := map[string]bool{}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return result
	}
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(rel)] = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return result
}

func hasFileNamed151(t *testing.T, root, name string) bool {
	t.Helper()
	found := false
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return false
	}
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == name {
			found = true
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return found
}
