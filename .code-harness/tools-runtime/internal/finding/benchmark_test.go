package finding

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/nav"
	"codea-harness-tools/internal/reviewrules"
	"codea-harness-tools/internal/reviewunit"
)

type benchmarkFile160 struct {
	Path          string `json:"path"`
	Role          string `json:"role"`
	Workspace     string `json:"workspace"`
	Changed       bool   `json:"changed"`
	BeforeContent string `json:"beforeContent"`
	Content       string `json:"content"`
}

type benchmarkSymbol160 struct {
	Symbol    string `json:"symbol"`
	Path      string `json:"path"`
	Role      string `json:"role"`
	Workspace string `json:"workspace"`
	LineStart int    `json:"lineStart"`
	LineEnd   int    `json:"lineEnd"`
}

type benchmarkRelation160 struct {
	Path       string `json:"path"`
	Role       string `json:"role"`
	Resource   string `json:"resource"`
	FromSymbol string `json:"fromSymbol"`
	FromKind   string `json:"fromKind"`
	Source     string `json:"source"`
	Evidence   string `json:"evidence"`
}

type benchmarkExpected160 struct {
	Findings       int      `json:"findings"`
	Rejections     int      `json:"rejections"`
	RejectionCodes []string `json:"rejectionCodes"`
	RuleDispatched bool     `json:"ruleDispatched"`
}

type benchmarkCase160 struct {
	ID             string                 `json:"id"`
	Class          string                 `json:"class"`
	RuleID         string                 `json:"ruleId"`
	Chain          []string               `json:"chain"`
	Files          []benchmarkFile160     `json:"files"`
	Symbols        []benchmarkSymbol160   `json:"symbols"`
	Relations      []benchmarkRelation160 `json:"relations"`
	Proposals      []Proposal             `json:"proposals"`
	Expected       benchmarkExpected160   `json:"expected"`
	ContextSymbols []string               `json:"contextSymbols"`
}

type benchmarkResult160 struct {
	Case       benchmarkCase160
	Set        CertifiedSet
	Rejections []Rejection
	Dispatch   reviewrules.Manifest
	Units      reviewunit.Manifest
}

var benchmarkAstGrepOnce160 sync.Once
var benchmarkAstGrepPath160 string
var benchmarkAstGrepErr160 error

func TestBenchmarkPositiveMustFindCases(t *testing.T) {
	cases := loadBenchmarkCases160(t)
	total, found := 0, 0
	for _, c := range cases {
		if c.Class != "positive" { continue }
		total++
		got := runBenchmarkCase160(t, c)
		assertBenchmarkExpected160(t, got)
		if len(got.Set.Findings) > 0 { found++ }
	}
	if total != 12 { t.Fatalf("positive case count=%d want 12", total) }
	recall := float64(found) / float64(total)
	t.Logf("MustFindRecall=%.2f", recall)
	if recall < 0.85 { t.Fatalf("MustFindRecall %.2f < 0.85", recall) }
}

func TestBenchmarkNegativeMustNotFindCases(t *testing.T) {
	cases := loadBenchmarkCases160(t)
	negative, falsePositive := 0, 0
	for _, c := range cases {
		if c.Class != "negative" { continue }
		negative++
		got := runBenchmarkCase160(t, c)
		assertBenchmarkExpected160(t, got)
		falsePositive += len(got.Set.Findings)
	}
	if negative != 8 { t.Fatalf("negative case count=%d want 8", negative) }
	precision := 1.0
	if falsePositive != 0 { precision = 0 }
	t.Logf("Precision=%.2f", precision)
	if precision < 0.90 { t.Fatalf("Precision %.2f < 0.90", precision) }
}

func TestBenchmarkAnchorRateIsOne(t *testing.T) {
	verified := 0
	for _, c := range loadBenchmarkCases160(t) {
		got := runBenchmarkCase160(t, c)
		for _, f := range got.Set.Findings {
			if f.Anchor.Kind == "" { t.Fatalf("%s emitted finding without anchor", c.ID) }
			verified++
		}
	}
	rate := 1.0
	if verified == 0 { rate = 0 }
	t.Logf("AnchorRate=%.2f", rate)
	if rate != 1.0 { t.Fatalf("AnchorRate %.2f want 1.00", rate) }
}

func TestBenchmarkDuplicateRateIsZero(t *testing.T) {
	for _, c := range loadBenchmarkCases160(t) {
		got := runBenchmarkCase160(t, c)
		seen := map[string]bool{}
		for _, f := range got.Set.Findings {
			key := f.RuleID + "\x00" + fmt.Sprintf("%v", f.Anchor)
			if seen[key] { t.Fatalf("%s retained semantic duplicate %s", c.ID, key) }
			seen[key] = true
		}
	}
	t.Log("DuplicateRate=0.00")
}

func TestBenchmarkRuntimeArtifactsAreDeterministic(t *testing.T) {
	cases := loadBenchmarkCases160(t)
	for _, c := range cases {
		left := runBenchmarkCase160(t, c)
		right := runBenchmarkCase160(t, c)
		if len(left.Set.Findings) != len(right.Set.Findings) || len(left.Rejections) != len(right.Rejections) {
			t.Fatalf("%s deterministic result count changed", c.ID)
		}
		for i := range left.Set.Findings {
			if left.Set.Findings[i].RuleID != right.Set.Findings[i].RuleID || left.Set.Findings[i].Anchor != right.Set.Findings[i].Anchor {
				t.Fatalf("%s deterministic finding changed", c.ID)
			}
		}
		if dispatchShape160(left.Dispatch) != dispatchShape160(right.Dispatch) {
			t.Fatalf("%s deterministic dispatch shape changed", c.ID)
		}
	}
	t.Log("DeterministicArtifactStability=1.00")
}

func loadBenchmarkCases160(t *testing.T) []benchmarkCase160 {
	t.Helper()
	root := filepath.Join("..", "..", "testdata", "review-benchmark")
	var cases []benchmarkCase160
	for _, class := range []string{"positive", "negative", "contract"} {
		entries, err := os.ReadDir(filepath.Join(root, class))
		if err != nil { t.Fatal(err) }
		for _, entry := range entries {
			if !entry.IsDir() { continue }
			data, err := os.ReadFile(filepath.Join(root, class, entry.Name(), "case.json"))
			if err != nil { t.Fatal(err) }
			var c benchmarkCase160
			if err := json.Unmarshal(data, &c); err != nil { t.Fatalf("decode %s/%s: %v", class, entry.Name(), err) }
			cases = append(cases, c)
		}
	}
	sort.Slice(cases, func(i, j int) bool { return cases[i].ID < cases[j].ID })
	if len(cases) != 24 { t.Fatalf("benchmark case count=%d want 24", len(cases)) }
	return cases
}

func runBenchmarkCase160(t *testing.T, c benchmarkCase160) benchmarkResult160 {
	t.Helper()
	root, runID := prepareBenchmarkRepo160(t, c)
	analysisValue, _, err := analysisruntime.LoadCertified(root, filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json")))
	if err != nil { t.Fatalf("%s load Certified Analysis: %v", c.ID, err) }

	units, err := reviewunit.Build(reviewunit.BuildInput{RunID: runID, CertifiedRunID: runID, RepoRoot: root})
	if err != nil { t.Fatalf("%s real reviewunit.Build: %v", c.ID, err) }
	rules, catalogSHA, err := reviewrules.LoadCatalog(filepath.Join(root, ".code-harness", "review-rules", "spring-v1.yaml"))
	if err != nil { t.Fatal(err) }
	dispatch, err := reviewrules.BuildDispatch(units, rules, catalogSHA)
	if err != nil { t.Fatalf("%s real RuleDispatch: %v", c.ID, err) }

	ranges := map[string]nav.SymbolInfo{}
	for _, s := range c.Symbols {
		workspace := strings.TrimSpace(s.Workspace)
		if workspace != "" && workspace != "current" { continue }
		ranges[s.Symbol] = nav.SymbolInfo{Symbol: s.Symbol, Path: s.Path, LineStart: s.LineStart, LineEnd: s.LineEnd}
	}
	verify := VerifyContext{trusted: true, repoRoot: root, analysis: analysisValue, units: units, dispatch: dispatch, symbolRanges: ranges}
	proposals := append([]Proposal(nil), c.Proposals...)
	for i := range proposals {
		proposals[i].ReviewUnitID = benchmarkUnitForProposal160(t, units, dispatch, analysisValue, proposals[i])
	}
	proposalBytes, err := json.Marshal(proposals)
	if err != nil { t.Fatal(err) }
	proposalSHA := fmt.Sprintf("%x", sha256.Sum256(proposalBytes))
	set, _, rejections, err := Certify(CertifyContext{
		Verify: verify, RunID: runID, HarnessVersion: units.HarnessVersion,
		ChangeSetSHA256: units.ChangeSetSHA256, ChangeAnalysisSHA256: units.ChangeAnalysisSHA256,
		ReviewUnitsSHA256: units.SHA256, RuleDispatchSHA256: dispatch.SHA256,
		FindingProposalsSHA256: proposalSHA, Mode: string(units.Mode), ScopeSHA256: units.ReviewScopeSHA256,
	}, proposals)
	if err != nil { t.Fatalf("%s certify: %v", c.ID, err) }
	got := benchmarkResult160{Case: c, Set: set, Rejections: rejections, Dispatch: dispatch, Units: units}
	assertBenchmarkDispatch160(t, got)
	return got
}

func prepareBenchmarkRepo160(t *testing.T, c benchmarkCase160) (string, string) {
	t.Helper()
	parent := t.TempDir()
	root := filepath.Join(parent, "current")
	if err := os.MkdirAll(root, 0o755); err != nil { t.Fatal(err) }
	writeBenchmarkFile160(t, root, ".gitignore", ".code-harness/\n")
	writeBenchmarkFile160(t, root, "src/main/java/com/acme/BenchmarkController.java", "package com.acme;\npublic class BenchmarkController { public void entry() {} }\n")
	for _, f := range c.Files {
		workspace := normalizeBenchmarkWorkspace160(f.Workspace)
		if workspace != "current" { continue }
		before := f.Content
		if f.Changed && f.BeforeContent != "" { before = f.BeforeContent }
		writeBenchmarkFile160(t, root, f.Path, before)
	}
	setupBenchmarkDependencies160(t, parent, root, c)
	gitBenchmark160(t, root, "init")
	gitBenchmark160(t, root, "config", "user.email", "benchmark@codea.local")
	gitBenchmark160(t, root, "config", "user.name", "Codea Benchmark")
	gitBenchmark160(t, root, "add", ".")
	gitBenchmark160(t, root, "commit", "-m", "benchmark baseline")
	baseHead := strings.TrimSpace(gitBenchmark160(t, root, "rev-parse", "HEAD"))
	branch := strings.TrimSpace(gitBenchmark160(t, root, "branch", "--show-current"))
	if branch == "" { branch = "master" }
	for _, f := range c.Files {
		if normalizeBenchmarkWorkspace160(f.Workspace) == "current" && f.Changed {
			writeBenchmarkFile160(t, root, f.Path, f.Content)
		}
	}

	copyBenchmarkHarness160(t, root)
	installBenchmarkAstGrep160(t, root)
	runID := "bench-" + strings.ReplaceAll(c.ID, "_", "-")
	requestDir := filepath.Join(root, ".code-harness", "runs", runID, "requests")
	if err := os.MkdirAll(requestDir, 0o755); err != nil { t.Fatal(err) }
	changed := make([]map[string]any, 0)
	reviewed := make([]map[string]any, 0)
	for _, f := range c.Files {
		if normalizeBenchmarkWorkspace160(f.Workspace) != "current" || !f.Changed { continue }
		changed = append(changed, map[string]any{"path": f.Path, "role": f.Role, "sources": []string{"UNSTAGED"}})
		reviewed = append(reviewed, map[string]any{"path": f.Path, "role": f.Role, "reason": "CHANGED"})
	}
	locations := make([]map[string]any, 0, len(c.Symbols)+1)
	locations = append(locations, map[string]any{"workspace": "current", "symbol": "BenchmarkController.entry", "path": "src/main/java/com/acme/BenchmarkController.java", "role": "Controller", "source": "BENCHMARK_RUNTIME"})
	for _, s := range c.Symbols {
		locations = append(locations, map[string]any{"workspace": normalizeBenchmarkWorkspace160(s.Workspace), "symbol": s.Symbol, "path": s.Path, "role": s.Role, "source": "BENCHMARK_RUNTIME"})
	}
	chains := []map[string]any{}
	if len(c.Chain) > 0 {
		chain := append([]string(nil), c.Chain...)
		entry := chain[0]
		if benchmarkRoleForSymbol160(c, entry) != "Controller" {
			chain = append([]string{"BenchmarkController.entry"}, chain...)
			entry = "BenchmarkController.entry"
		}
		chains = append(chains, map[string]any{"entryPoint": entry, "chain": chain})
	}
	relations := make([]map[string]any, 0, len(c.Relations))
	for _, r := range c.Relations {
		relations = append(relations, map[string]any{"path": r.Path, "role": r.Role, "resource": r.Resource, "fromSymbol": r.FromSymbol, "fromKind": r.FromKind, "source": r.Source, "evidence": r.Evidence})
	}
	external := []string{}
	for _, s := range c.Symbols {
		ws := normalizeBenchmarkWorkspace160(s.Workspace)
		if ws != "current" { external = append(external, ws) }
	}
	sort.Strings(external)
	external = uniqueBenchmarkStrings160(external)
	draft := map[string]any{
		"reviewScope": map[string]any{"currentBranch": branch, "baseRef": "HEAD", "baseCommit": baseHead, "mergeBase": baseHead, "headCommit": baseHead, "includeWorkingTree": true},
		"changedFiles": changed, "affectedControllers": []any{}, "callChains": chains,
		"symbolLocations": locations, "resourceRelations": relations, "externalDependencies": external,
		"riskAreas": []any{}, "reviewCoverage": map[string]any{"status": "COMPLETE", "reviewedFiles": reviewed, "unresolvedSymbols": []any{}},
	}
	draftBytes, err := json.MarshalIndent(draft, "", "  ")
	if err != nil { t.Fatal(err) }
	draftBytes = append(draftBytes, '\n')
	draftRel := filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "requests", "change-analysis-draft.json"))
	writeBenchmarkFile160(t, root, draftRel, string(draftBytes))
	if _, err := analysisruntime.Certify(root, analysisruntime.CertifyRequest{RunID: runID, DraftPath: draftRel, BaseRef: "HEAD", IncludeWorkingTree: true, Intent: analysisruntime.Intent{Mode: "FULL"}}); err != nil {
		t.Fatalf("%s real analysis certify: %v", c.ID, err)
	}
	return root, runID
}

func benchmarkUnitForProposal160(t *testing.T, units reviewunit.Manifest, dispatch reviewrules.Manifest, analysis analysisruntime.ChangeAnalysis, p Proposal) string {
	t.Helper()
	candidateIDs := map[string]bool{}
	for _, d := range dispatch.Dispatches {
		if d.RuleID == p.RuleID { candidateIDs[d.ReviewUnitID] = true }
	}
	if len(candidateIDs) == 0 { return "NO-DISPATCHED-UNIT" }
	path := strings.TrimSpace(p.Anchor.Path)
	if path == "" && strings.TrimSpace(p.Anchor.Symbol) != "" {
		for _, loc := range analysis.SymbolLocations {
			if loc.Symbol == p.Anchor.Symbol && normalizeBenchmarkWorkspace160(loc.Workspace) == "current" { path = loc.Path; break }
		}
	}
	if path != "" {
		for _, u := range units.Units {
			if !candidateIDs[u.ID] { continue }
			for _, f := range u.Files { if f.Path == path { return u.ID } }
		}
	}
	for _, u := range units.Units { if candidateIDs[u.ID] { return u.ID } }
	return "NO-DISPATCHED-UNIT"
}

func assertBenchmarkExpected160(t *testing.T, got benchmarkResult160) {
	t.Helper()
	if len(got.Set.Findings) != got.Case.Expected.Findings { t.Fatalf("%s findings=%d want %d", got.Case.ID, len(got.Set.Findings), got.Case.Expected.Findings) }
	if len(got.Rejections) != got.Case.Expected.Rejections { t.Fatalf("%s rejections=%v want %d", got.Case.ID, got.Rejections, got.Case.Expected.Rejections) }
	codes := make([]string, 0, len(got.Rejections))
	for _, r := range got.Rejections { codes = append(codes, r.Code) }
	sort.Strings(codes)
	want := append([]string(nil), got.Case.Expected.RejectionCodes...); sort.Strings(want)
	if strings.Join(codes, "\x00") != strings.Join(want, "\x00") { t.Fatalf("%s rejection codes=%v want %v", got.Case.ID, codes, want) }
}

func assertBenchmarkDispatch160(t *testing.T, got benchmarkResult160) {
	t.Helper()
	found := false
	for _, d := range got.Dispatch.Dispatches { if d.RuleID == got.Case.RuleID { found = true; break } }
	if found != got.Case.Expected.RuleDispatched { t.Fatalf("%s rule %s dispatched=%v want %v", got.Case.ID, got.Case.RuleID, found, got.Case.Expected.RuleDispatched) }
}

func dispatchShape160(m reviewrules.Manifest) string {
	parts := make([]string, 0, len(m.Dispatches))
	for _, d := range m.Dispatches { parts = append(parts, d.RuleID+":"+strings.Join(d.DispatchReason, ",")) }
	sort.Strings(parts)
	return strings.Join(parts, "|")
}

func copyBenchmarkHarness160(t *testing.T, root string) {
	t.Helper()
	wd, err := os.Getwd(); if err != nil { t.Fatal(err) }
	harnessRoot := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
	for _, rel := range []string{
		"VERSION", "contracts/change-analysis.schema.json", "contracts/entrypoint-inventory.schema.json", "contracts/change-analysis-cert.schema.json", "review-rules/spring-v1.yaml",
	} {
		data, err := os.ReadFile(filepath.Join(harnessRoot, filepath.FromSlash(rel))); if err != nil { t.Fatal(err) }
		writeBenchmarkFile160(t, root, filepath.ToSlash(filepath.Join(".code-harness", rel)), string(data))
	}
}

func installBenchmarkAstGrep160(t *testing.T, root string) {
	t.Helper()
	benchmarkAstGrepOnce160.Do(func() {
		dir, err := os.MkdirTemp("", "codea-benchmark-ast-grep-"); if err != nil { benchmarkAstGrepErr160 = err; return }
		src := filepath.Join(dir, "main.go")
		if err := os.WriteFile(src, []byte("package main\nfunc main() {}\n"), 0o644); err != nil { benchmarkAstGrepErr160 = err; return }
		benchmarkAstGrepPath160 = filepath.Join(dir, "ast-grep.exe")
		cmd := exec.Command("go", "build", "-o", benchmarkAstGrepPath160, src)
		if out, err := cmd.CombinedOutput(); err != nil { benchmarkAstGrepErr160 = fmt.Errorf("build ast-grep stub: %v: %s", err, out) }
	})
	if benchmarkAstGrepErr160 != nil { t.Fatal(benchmarkAstGrepErr160) }
	data, err := os.ReadFile(benchmarkAstGrepPath160); if err != nil { t.Fatal(err) }
	path := filepath.Join(root, ".code-harness", "bin", "ast-grep.exe")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(path, data, 0o755); err != nil { t.Fatal(err) }
}

func setupBenchmarkDependencies160(t *testing.T, parent, root string, c benchmarkCase160) {
	t.Helper()
	seen := map[string]bool{}
	for _, s := range c.Symbols {
		ws := normalizeBenchmarkWorkspace160(s.Workspace)
		if ws == "current" || seen[ws] { continue }
		seen[ws] = true
		dep := filepath.Join(parent, ws)
		if err := os.MkdirAll(dep, 0o755); err != nil { t.Fatal(err) }
		writeBenchmarkFile160(t, dep, "pom.xml", `<?xml version="1.0"?><project><modelVersion>4.0.0</modelVersion><groupId>com.company</groupId><artifactId>`+ws+`</artifactId><version>2.3.1</version></project>`)
		writeBenchmarkFile160(t, dep, s.Path, "package com.shared; public class SharedPolicy { public void check() {} }\n")
		writeBenchmarkFile160(t, root, "pom.xml", `<?xml version="1.0"?><project><modelVersion>4.0.0</modelVersion><groupId>com.company</groupId><artifactId>benchmark</artifactId><version>1.0.0</version><dependencies><dependency><groupId>com.company</groupId><artifactId>`+ws+`</artifactId><version>2.3.1</version></dependency></dependencies></project>`)
		harness := "workspaceDependencies:\n  - id: "+ws+"\n    root: ../"+ws+"\n    maven:\n      groupId: com.company\n      artifactId: "+ws+"\n    mode: READ_ONLY\n"
		writeBenchmarkFile160(t, root, ".code-harness/harness.yaml", harness)
	}
}

func benchmarkRoleForSymbol160(c benchmarkCase160, symbol string) string {
	for _, s := range c.Symbols { if s.Symbol == symbol { return s.Role } }
	if symbol == "BenchmarkController.entry" { return "Controller" }
	return ""
}

func normalizeBenchmarkWorkspace160(value string) string { if strings.TrimSpace(value) == "" { return "current" }; return strings.TrimSpace(value) }
func uniqueBenchmarkStrings160(in []string) []string { out:=[]string{}; last:=""; for _,v:=range in { if v!="" && v!=last { out=append(out,v); last=v } }; return out }

func writeBenchmarkFile160(t *testing.T, root, rel, content string) {
	t.Helper(); path := filepath.Join(root, filepath.FromSlash(rel)); if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { t.Fatal(err) }; if err := os.WriteFile(path, []byte(content), 0o644); err != nil { t.Fatal(err) }
}

func gitBenchmark160(t *testing.T, root string, args ...string) string {
	t.Helper(); cmd := exec.Command("git", args...); cmd.Dir = root; out, err := cmd.CombinedOutput(); if err != nil { t.Fatalf("git %v: %v: %s", args, err, out) }; return string(out)
}
