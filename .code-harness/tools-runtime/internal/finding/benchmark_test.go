package finding

import (
    "crypto/sha256"
    "encoding/json"
    "fmt"
    "io/fs"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "testing"

    "codea-harness-tools/internal/analysis"
    "codea-harness-tools/internal/nav"
    "codea-harness-tools/internal/reviewrules"
    "codea-harness-tools/internal/reviewunit"
)

type benchmarkFile160 struct {
    Path string `json:"path"`
    Role string `json:"role"`
    Workspace string `json:"workspace"`
    Changed bool `json:"changed"`
    Content string `json:"content"`
}

type benchmarkSymbol160 struct {
    Symbol string `json:"symbol"`
    Path string `json:"path"`
    Role string `json:"role"`
    Workspace string `json:"workspace"`
    LineStart int `json:"lineStart"`
    LineEnd int `json:"lineEnd"`
}

type benchmarkExpected160 struct {
    Findings int `json:"findings"`
    Rejections int `json:"rejections"`
    RejectionCodes []string `json:"rejectionCodes,omitempty"`
    RuleDispatched bool `json:"ruleDispatched"`
}

type benchmarkCase160 struct {
    ID string `json:"id"`
    Class string `json:"class"`
    RuleID string `json:"ruleId"`
    Chain []string `json:"chain"`
    Files []benchmarkFile160 `json:"files"`
    Hunks []reviewunit.HunkRef `json:"hunks"`
    Symbols []benchmarkSymbol160 `json:"symbols"`
    Relations []analysis.ResourceRelation `json:"relations,omitempty"`
    Proposals []Proposal `json:"proposals"`
    Expected benchmarkExpected160 `json:"expected"`
}

type benchmarkResult160 struct {
    Case benchmarkCase160
    Set CertifiedSet
    Rejections []Rejection
    Dispatch reviewrules.Manifest
    Unit reviewunit.Manifest
}

func TestBenchmarkPositiveMustFindCases(t *testing.T) {
    cases := loadBenchmarkCases160(t)
    total := 0
    found := 0
    accepted := 0
    for _, c := range cases {
        if c.Class != "positive" { continue }
        total++
        result := runBenchmarkCase160(t, c)
        if len(result.Set.Findings) != c.Expected.Findings || len(result.Rejections) != c.Expected.Rejections {
            t.Fatalf("%s result mismatch: findings=%d rejections=%d expected=%+v", c.ID, len(result.Set.Findings), len(result.Rejections), c.Expected)
        }
        accepted += len(result.Set.Findings)
        if len(result.Set.Findings) > 0 { found++ }
    }
    if total != 12 { t.Fatalf("positive fixture count=%d want=12", total) }
    recall := float64(found) / float64(total)
    precision := 1.0
    if accepted > 0 { precision = float64(found) / float64(accepted) }
    t.Logf("Precision=%.2f MustFindRecall=%.2f", precision, recall)
    if precision < 0.90 { t.Fatalf("Precision %.2f < 0.90", precision) }
    if recall < 0.85 { t.Fatalf("MustFindRecall %.2f < 0.85", recall) }
}

func TestBenchmarkNegativeMustNotFindCases(t *testing.T) {
    cases := loadBenchmarkCases160(t)
    total := 0
    falsePositive := 0
    for _, c := range cases {
        if c.Class != "negative" { continue }
        total++
        result := runBenchmarkCase160(t, c)
        if len(result.Set.Findings) != c.Expected.Findings || len(result.Rejections) != c.Expected.Rejections {
            t.Fatalf("%s result mismatch: findings=%d rejections=%d expected=%+v", c.ID, len(result.Set.Findings), len(result.Rejections), c.Expected)
        }
        falsePositive += len(result.Set.Findings)
    }
    if total != 8 { t.Fatalf("negative fixture count=%d want=8", total) }
    precision := 1.0
    if falsePositive > 0 { precision = 0 }
    t.Logf("Precision=%.2f", precision)
    if falsePositive != 0 { t.Fatalf("negative benchmark produced %d false-positive findings", falsePositive) }
}

func TestBenchmarkAnchorRateIsOne(t *testing.T) {
    cases := loadBenchmarkCases160(t)
    accepted := 0
    anchored := 0
    for _, c := range cases {
        result := runBenchmarkCase160(t, c)
        for _, f := range result.Set.Findings {
            accepted++
            if f.Anchor.Kind == AnchorLine || f.Anchor.Kind == AnchorSymbol || f.Anchor.Kind == AnchorFile || f.Anchor.Kind == AnchorChangeSet { anchored++ }
        }
    }
    rate := 1.0
    if accepted > 0 { rate = float64(anchored) / float64(accepted) }
    t.Logf("AnchorRate=%.2f", rate)
    if rate != 1.0 { t.Fatalf("AnchorRate %.2f != 1.00", rate) }
}

func TestBenchmarkDuplicateRateIsZero(t *testing.T) {
    cases := loadBenchmarkCases160(t)
    duplicates := 0
    accepted := 0
    for _, c := range cases {
        result := runBenchmarkCase160(t, c)
        accepted += len(result.Set.Findings)
        seen := map[string]bool{}
        for _, f := range result.Set.Findings {
            key := f.RuleID + "|" + string(f.Anchor.Kind) + "|" + f.Anchor.Path + "|" + f.Anchor.Symbol + fmt.Sprintf("|%d", f.Anchor.Line)
            if seen[key] { duplicates++ }
            seen[key] = true
        }
    }
    rate := 0.0
    if accepted > 0 { rate = float64(duplicates) / float64(accepted) }
    t.Logf("DuplicateRate=%.2f", rate)
    if duplicates != 0 { t.Fatalf("DuplicateRate %.2f != 0", rate) }
}

func TestBenchmarkRuntimeArtifactsAreDeterministic(t *testing.T) {
    cases := loadBenchmarkCases160(t)
    stable := 0
    for _, c := range cases {
        first := runBenchmarkCase160(t, c)
        second := runBenchmarkCase160(t, c)
        u1, _ := reviewunit.CanonicalBytes(first.Unit)
        u2, _ := reviewunit.CanonicalBytes(second.Unit)
        d1, _ := reviewrules.CanonicalBytes(first.Dispatch)
        d2, _ := reviewrules.CanonicalBytes(second.Dispatch)
        f1, _ := canonicalCertifiedSet160(first.Set, true)
        f2, _ := canonicalCertifiedSet160(second.Set, true)
        if string(u1) != string(u2) || string(d1) != string(d2) || string(f1) != string(f2) {
            t.Fatalf("%s runtime artifacts are not deterministic", c.ID)
        }
        stable++
    }
    stability := float64(stable) / float64(len(cases))
    t.Logf("DeterministicArtifactStability=%.2f", stability)
    if stability != 1.0 { t.Fatalf("DeterministicArtifactStability %.2f != 1.00", stability) }
}

func loadBenchmarkCases160(t *testing.T) []benchmarkCase160 {
    t.Helper()
    root := filepath.Join("..", "..", "testdata", "review-benchmark")
    cases := []benchmarkCase160{}
    err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
        if err != nil { return err }
        if entry.IsDir() || entry.Name() != "case.json" { return nil }
        data, err := os.ReadFile(path)
        if err != nil { return err }
        var c benchmarkCase160
        if err := json.Unmarshal(data, &c); err != nil { return fmt.Errorf("decode %s: %w", path, err) }
        cases = append(cases, c)
        return nil
    })
    if err != nil { t.Fatalf("load benchmark fixtures: %v", err) }
    sort.Slice(cases, func(i, j int) bool { return cases[i].ID < cases[j].ID })
    if len(cases) != 24 { t.Fatalf("review benchmark must contain exactly 24 case.json fixtures, got %d", len(cases)) }
    seen := map[string]bool{}
    for _, c := range cases {
        if strings.TrimSpace(c.ID) == "" || seen[c.ID] { t.Fatalf("invalid or duplicate benchmark id %q", c.ID) }
        seen[c.ID] = true
    }
    return cases
}

func runBenchmarkCase160(t *testing.T, c benchmarkCase160) benchmarkResult160 {
    t.Helper()
    root := t.TempDir()
    unit := reviewunit.Unit{ID: "RU-BENCH", Files: []reviewunit.FileRef{}, ChangedHunks: append([]reviewunit.HunkRef(nil), c.Hunks...)}
    if len(c.Chain) > 0 { unit.EntryPoint = c.Chain[0]; unit.Chain = append([]string(nil), c.Chain...) }
    analysisValue := analysis.ChangeAnalysis{ResourceRelations: append([]analysis.ResourceRelation(nil), c.Relations...)}
    ranges := map[string]nav.SymbolInfo{}
    for _, f := range c.Files {
        workspace := f.Workspace
        if workspace == "" { workspace = "current" }
        unit.Files = append(unit.Files, reviewunit.FileRef{Path: f.Path, Role: f.Role, Changed: f.Changed, Workspace: workspace})
        if workspace == "current" {
            abs := filepath.Join(root, filepath.FromSlash(f.Path))
            if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil { t.Fatal(err) }
            if err := os.WriteFile(abs, []byte(f.Content), 0o644); err != nil { t.Fatal(err) }
        }
    }
    for _, s := range c.Symbols {
        workspace := s.Workspace
        if workspace == "" { workspace = "current" }
        analysisValue.SymbolLocations = append(analysisValue.SymbolLocations, analysis.SymbolLocation{Workspace: workspace, Symbol: s.Symbol, Path: s.Path, Role: s.Role, Source: "BENCHMARK"})
        if workspace == "current" {
            ranges[s.Symbol] = nav.SymbolInfo{Symbol: s.Symbol, Path: s.Path, LineStart: s.LineStart, LineEnd: s.LineEnd}
        }
    }
    manifest := reviewunit.Manifest{RunID: "run-benchmark", HarnessVersion: "1.6.0", Mode: reviewunit.ModeFull, ChangeSetSHA256: strings.Repeat("a", 64), ChangeAnalysisSHA256: strings.Repeat("b", 64), Units: []reviewunit.Unit{unit}}
    canonical, err := reviewunit.CanonicalBytes(manifest)
    if err != nil { t.Fatalf("%s canonical review unit: %v", c.ID, err) }
    manifest.SHA256 = fmt.Sprintf("%x", sha256.Sum256(canonical))
    rules, catalogSHA, err := reviewrules.LoadCatalog(filepath.Join("..", "..", "..", "review-rules", "spring-v1.yaml"))
    if err != nil { t.Fatalf("%s load rule catalog: %v", c.ID, err) }
    dispatch, err := reviewrules.BuildDispatch(manifest, rules, catalogSHA)
    if err != nil { t.Fatalf("%s build dispatch: %v", c.ID, err) }
    dispatched := false
    for _, d := range dispatch.Dispatches { if d.RuleID == c.RuleID { dispatched = true } }
    if dispatched != c.Expected.RuleDispatched { t.Fatalf("%s rule dispatched=%v want=%v", c.ID, dispatched, c.Expected.RuleDispatched) }
    verify := VerifyContext{trusted: true, repoRoot: root, analysis: analysisValue, units: manifest, dispatch: dispatch, symbolRanges: ranges}
    proposalsBytes, _ := json.Marshal(c.Proposals)
    ctx := CertifyContext{Verify: verify, RunID: manifest.RunID, HarnessVersion: manifest.HarnessVersion, ChangeSetSHA256: manifest.ChangeSetSHA256, ChangeAnalysisSHA256: manifest.ChangeAnalysisSHA256, ReviewUnitsSHA256: manifest.SHA256, RuleDispatchSHA256: dispatch.SHA256, FindingProposalsSHA256: fmt.Sprintf("%x", sha256.Sum256(proposalsBytes)), Mode: string(manifest.Mode)}
    set, _, rejections, err := Certify(ctx, c.Proposals)
    if err != nil { t.Fatalf("%s certify: %v", c.ID, err) }
    gotCodes := make([]string, 0, len(rejections))
    for _, r := range rejections { gotCodes = append(gotCodes, r.Code) }
    sort.Strings(gotCodes)
    wantCodes := append([]string(nil), c.Expected.RejectionCodes...)
    sort.Strings(wantCodes)
    if strings.Join(gotCodes, "|") != strings.Join(wantCodes, "|") { t.Fatalf("%s rejection codes=%v want=%v", c.ID, gotCodes, wantCodes) }
    return benchmarkResult160{Case: c, Set: set, Rejections: rejections, Dispatch: dispatch, Unit: manifest}
}
