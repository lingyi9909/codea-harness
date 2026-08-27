package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/finding"
	"codea-harness-tools/internal/report"
	"codea-harness-tools/internal/reviewunit"
)

func TestTask4LoadCertifiedAndReportRejectSourceMutationAfterCertification(t *testing.T) {
	runID, req := prepareTask4CertifiedTargetedAuthority160(t)
	analysisPath := filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json")
	analysisBytes, err := os.ReadFile(analysisPath)
	if err != nil { t.Fatal(err) }
	var doc struct {
		ChangedFiles []struct { Path string `json:"path"` } `json:"changedFiles"`
	}
	if err := json.Unmarshal(analysisBytes, &doc); err != nil { t.Fatal(err) }
	if len(doc.ChangedFiles) == 0 || strings.TrimSpace(doc.ChangedFiles[0].Path) == "" {
		t.Fatal("fixture requires at least one changed source file")
	}
	sourcePath := filepath.FromSlash(doc.ChangedFiles[0].Path)
	before, err := os.ReadFile(sourcePath)
	if err != nil { t.Fatal(err) }
	if err := os.WriteFile(sourcePath, append(before, []byte("\n// task4 stale source\n")...), 0o644); err != nil { t.Fatal(err) }

	if _, err := finding.LoadCertified(".", runID); err == nil {
		t.Fatal("LoadCertified must reject when source/Working Tree changes after certification even if runs/** bytes are untouched")
	}
	if _, err := report.WriteCertifiedReport(".", req); err == nil {
		t.Fatal("formal report must reject when source/Working Tree changes after certification")
	}
}

func TestTask4TargetedCertificateRejectsFullReport(t *testing.T) {
	_, req := prepareTask4CertifiedTargetedAuthority160(t)
	req.Mode = "FULL"
	req.Target = nil
	req.Scope.ScopedFiles = nil
	if _, err := report.WriteCertifiedReport(".", req); err == nil {
		t.Fatal("TARGETED Certified Findings certificate must reject a FULL report transport")
	}
}

func TestTask4TargetedCertificateRejectsWrongTargetOrScopedFiles(t *testing.T) {
	_, req := prepareTask4CertifiedTargetedAuthority160(t)

	wrongTarget := req
	wrongTarget.Target = &report.ReviewTarget{Symbol: req.Target.Symbol + ".wrong", Kind: req.Target.Kind}
	if _, err := report.WriteCertifiedReport(".", wrongTarget); err == nil {
		t.Fatal("TARGETED certificate must reject report transport with the wrong target")
	}

	wrongScope := req
	wrongScope.Scope.ScopedFiles = append([]string(nil), req.Scope.ScopedFiles...)
	wrongScope.Scope.ScopedFiles = append(wrongScope.Scope.ScopedFiles, "src/main/java/com/acme/ScopeOut.java")
	if _, err := report.WriteCertifiedReport(".", wrongScope); err == nil {
		t.Fatal("TARGETED certificate must reject report transport with wrong scopedFiles")
	}
}

func prepareTask4CertifiedTargetedAuthority160(t *testing.T) (string, report.ReviewRequest) {
	t.Helper()
	sourceRoot, err := os.Getwd()
	if err != nil { t.Fatal(err) }
	options := task153BuildReviewOptions(t)
	if len(options.AutoSelectionIDs) != 1 {
		t.Fatalf("fixture must produce one AUTO_SINGLE selection, got %+v", options)
	}
	const runID = "run-task4-review"
	selection := writeQueryRequest(t, runID, "task4-blocker-select.json", `{"runId":"`+runID+`","mode":"TARGETED","selectionIds":["`+options.AutoSelectionIDs[0]+`"],"optionsHash":"`+options.OptionsHash+`"}`)
	if err := run([]string{"review", "select", "--input", selection}); err != nil {
		t.Fatalf("review select fixture: %v", err)
	}
	for _, name := range []string{
		"review-unit.schema.json",
		"finding-proposals.schema.json",
		"certified-findings.schema.json",
		"certified-findings-cert.schema.json",
	} {
		copyTask153CommandContract(t, ".", name)
	}
	installTask160DispatchFramework(t, sourceRoot)
	if err := run([]string{"review", "units", "--run-id", runID}); err != nil {
		t.Fatalf("review units fixture: %v", err)
	}
	if err := run([]string{"review", "dispatch", "--run-id", runID}); err != nil {
		t.Fatalf("review dispatch fixture: %v", err)
	}

	proposalPath := filepath.Join(".code-harness", "runs", runID, "requests", "finding-proposals.json")
	if err := os.MkdirAll(filepath.Dir(proposalPath), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(proposalPath, []byte("[]\n"), 0o644); err != nil { t.Fatal(err) }
	proposalBytes, err := os.ReadFile(proposalPath)
	if err != nil { t.Fatal(err) }

	verifyCtx, err := finding.LoadVerifyContext(".", runID, "")
	if err != nil { t.Fatalf("load verify authority: %v", err) }
	analysisPath := filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json"))
	analysisValue, analysisCert, err := analysisruntime.LoadCertified(".", analysisPath)
	if err != nil { t.Fatalf("load certified analysis: %v", err) }
	units, err := reviewunit.Load(reviewunit.BuildInput{RunID: runID, CertifiedRunID: runID, RepoRoot: "."})
	if err != nil { t.Fatalf("load review units: %v", err) }
	unitBytes, err := os.ReadFile(filepath.Join(".code-harness", "runs", runID, "analysis", "review-units.json"))
	if err != nil { t.Fatal(err) }
	dispatchBytes, err := os.ReadFile(filepath.Join(".code-harness", "runs", runID, "analysis", "rule-dispatch.json"))
	if err != nil { t.Fatal(err) }
	ctx := finding.CertifyContext{
		Verify: verifyCtx,
		RunID: runID,
		HarnessVersion: analysisCert.RuntimeVersion,
		ChangeSetSHA256: analysisCert.ChangeSetSHA256,
		ChangeAnalysisSHA256: analysisCert.AnalysisSHA256,
		ReviewUnitsSHA256: task4BlockerSHA160(unitBytes),
		RuleDispatchSHA256: task4BlockerSHA160(dispatchBytes),
		FindingProposalsSHA256: task4BlockerSHA160(proposalBytes),
		Mode: string(units.Mode),
		ScopeSHA256: units.ReviewScopeSHA256,
	}
	set, cert, rejections, err := finding.Certify(ctx, nil)
	if err != nil { t.Fatalf("certify empty findings: %v", err) }
	if len(rejections) != 0 { t.Fatalf("empty proposal set must have no rejections: %+v", rejections) }
	if err := finding.WriteCertified(".", set, cert); err != nil { t.Fatalf("write Certified Findings: %v", err) }

	scopeBytes, err := os.ReadFile(filepath.Join(".code-harness", "runs", runID, "analysis", "review-scope.json"))
	if err != nil { t.Fatal(err) }
	var scope struct {
		Mode string `json:"mode"`
		Target *struct {
			Symbol string `json:"symbol"`
			Kind string `json:"kind"`
		} `json:"target"`
		SelectedCallChains []struct {
			EntryPoint string `json:"entryPoint"`
			Chain []string `json:"chain"`
		} `json:"selectedCallChains"`
		ScopedFiles []string `json:"scopedFiles"`
	}
	if err := json.Unmarshal(scopeBytes, &scope); err != nil { t.Fatal(err) }
	if scope.Mode != "TARGETED" || scope.Target == nil { t.Fatalf("expected TARGETED fixture, got %s", scopeBytes) }
	callChains := make([]report.CallChain, 0, len(scope.SelectedCallChains))
	for _, chain := range scope.SelectedCallChains {
		callChains = append(callChains, report.CallChain{EntryPoint: chain.EntryPoint, Chain: append([]string(nil), chain.Chain...)})
	}
	changedFiles := make([]string, 0, len(analysisValue.ChangedFiles))
	for _, file := range analysisValue.ChangedFiles { changedFiles = append(changedFiles, file.Path) }
	versionBytes, err := os.ReadFile(filepath.Join(".code-harness", "VERSION"))
	if err != nil { t.Fatal(err) }
	req := report.ReviewRequest{
		RunID: runID,
		HarnessVersion: strings.TrimSpace(string(versionBytes)),
		BaseRef: analysisCert.BaseRef,
		Head: analysisCert.Head,
		Result: report.ResultPassed,
		Mode: "TARGETED",
		Target: &report.ReviewTarget{Symbol: scope.Target.Symbol, Kind: scope.Target.Kind},
		Scope: report.ReviewScope{ChangedFiles: changedFiles, ScopedFiles: append([]string(nil), scope.ScopedFiles...)},
		Coverage: report.ReviewCoverage{
			ReviewedFiles: append([]string(nil), scope.ScopedFiles...),
			CallChains: callChains,
			ExternalDependencies: []string{}, Unresolved: []string{}, MissingReviewedFiles: []string{}, RuntimeErrors: []string{},
			Status: "COMPLETE",
		},
		Findings: []report.Finding{},
	}
	return runID, req
}

func task4BlockerSHA160(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }
