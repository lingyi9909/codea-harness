package main

import (
	"os"
	"path/filepath"
	"runtime"
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

	runID := "run-151-bootstrap"
	analysisPath := filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json")
	analysis := `{
  "reviewScope":{"currentBranch":"feature/new-approval","baseRef":"origin/develop","baseCommit":"base","mergeBase":"base","headCommit":"head-new","includeWorkingTree":true},
  "changedFiles":[
    {"path":"src/main/java/com/example/approval/NewController.java","role":"Controller","sources":["UNTRACKED"]},
    {"path":"src/main/java/com/example/approval/ApprovalService.java","role":"Service","sources":["STAGED"]},
    {"path":"src/main/java/com/example/approval/ApprovalServiceImpl.java","role":"Service","sources":["UNSTAGED"]},
    {"path":"src/main/java/com/example/approval/ApprovalMapper.java","role":"Mapper","sources":["COMMITTED"]},
    {"path":"src/main/resources/mapper/ApprovalMapper.xml","role":"MapperXml","sources":["UNTRACKED"]}
  ],
  "affectedControllers":[
    {"controller":"NewController","endpoints":["NewController.approve"],"impactType":"DIRECT_CHANGE","sourceSymbols":["NewController.approve"]}
  ],
  "callChains":[
    {"entryPoint":"NewController.approve","chain":["NewController.approve","ApprovalService.approve","ApprovalServiceImpl.approve","ApprovalMapper.updateStatus"]}
  ],
  "symbolLocations":[
    {"symbol":"NewController.approve","path":"src/main/java/com/example/approval/NewController.java","role":"Controller","source":"FIND_SYMBOL"},
    {"symbol":"ApprovalService.approve","path":"src/main/java/com/example/approval/ApprovalService.java","role":"Service","source":"FIND_SYMBOL"},
    {"symbol":"ApprovalServiceImpl.approve","path":"src/main/java/com/example/approval/ApprovalServiceImpl.java","role":"Service","source":"FIND_IMPLEMENTATIONS","from":"ApprovalService.approve"},
    {"symbol":"ApprovalMapper.updateStatus","path":"src/main/java/com/example/approval/ApprovalMapper.java","role":"Mapper","source":"FIND_SYMBOL"}
  ],
  "resourceRelations":[
    {"path":"src/main/resources/mapper/ApprovalMapper.xml","role":"MapperXml","resource":"ApprovalMapper.xml#updateStatus","fromSymbol":"ApprovalMapper.updateStatus","fromKind":"METHOD","source":"MAPPER_STATEMENT","evidence":"statement id updateStatus matches ApprovalMapper.updateStatus"}
  ],
  "externalDependencies":[],
  "riskAreas":[],
  "reviewCoverage":{"status":"COMPLETE","reviewedFiles":[
    {"path":"src/main/java/com/example/approval/NewController.java","role":"Controller","reason":"CHANGED"},
    {"path":"src/main/java/com/example/approval/ApprovalService.java","role":"Service","reason":"CHANGED"},
    {"path":"src/main/java/com/example/approval/ApprovalServiceImpl.java","role":"Service","reason":"CHANGED"},
    {"path":"src/main/java/com/example/approval/ApprovalMapper.java","role":"Mapper","reason":"CHANGED"},
    {"path":"src/main/resources/mapper/ApprovalMapper.xml","role":"MapperXml","reason":"CHANGED"}
  ],"unresolvedSymbols":[]}
}`
	writeFile(t, analysisPath, analysis)
	requestPath := writeQueryRequest(t, runID, "chain-discover.json", `{"runId":"run-151-bootstrap","target":"NewController","changeAnalysisPath":".code-harness/runs/run-151-bootstrap/analysis/change-analysis.json"}`)

	if err := run([]string{"chain", "discover", "--input", requestPath}); err != nil {
		t.Fatal(err)
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
	for _, want := range []string{"status: DISCOVERED", "NewController.approve", "ApprovalServiceImpl.approve", "ApprovalMapper.updateStatus", "MAPPER_XML"} {
		if !strings.Contains(yamlText, want) {
			t.Fatalf("discovered YAML missing %q:\n%s", want, yamlText)
		}
	}
	if _, err := os.Stat(filepath.Join(".code-harness", "chains")); !os.IsNotExist(err) {
		t.Fatalf("direct discovery must not create Project State chains/**, err=%v", err)
	}
}
