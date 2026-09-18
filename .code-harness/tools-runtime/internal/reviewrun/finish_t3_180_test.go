package reviewrun

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test180FinishRejectsReadOutsideSelectedScope(t *testing.T) {
	root, started, _ := setupT3Scope180(t, false)
	outside := filepath.Join(root, "B.java")
	outsideBytes := []byte("class B {}\n")
	if err := os.WriteFile(outside, outsideBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	outsideRef := ReadRef{Path: "B.java", SHA256: bytesSHA256(outsideBytes), StartLine: 1, EndLine: 1}
	_, err := Finish(context.Background(), root, FinishRequest{RunID: started.RunID, Reads: []ReadRef{outsideRef}})
	if err == nil || !strings.Contains(err.Error(), "OUTSIDE_SCOPE") {
		t.Fatalf("finish accepted a read outside selected scope: %v", err)
	}
	status, statusErr := Status(root, started.RunID)
	if statusErr != nil {
		t.Fatal(statusErr)
	}
	if status.Execution != "INCOMPLETE" {
		t.Fatalf("scope violation must not complete report: %+v", status)
	}
}

func Test180FinishCompleteScopeRequiresSelectedReads(t *testing.T) {
	root, started, _ := setupT3Scope180(t, false)
	got, err := Finish(context.Background(), root, FinishRequest{RunID: started.RunID, Reads: []ReadRef{}, Findings: []Finding{}})
	if err == nil || !strings.Contains(err.Error(), "SCOPE_NOT_READ") {
		t.Fatalf("finish accepted complete scope without reading it: got=%+v err=%v", got, err)
	}
	status, statusErr := Status(root, started.RunID)
	if statusErr != nil {
		t.Fatal(statusErr)
	}
	if status.Execution != "INCOMPLETE" {
		t.Fatalf("missing selected reads must keep report incomplete: %+v", status)
	}
}

func Test180FinishRejectsFindingWithoutEvidence(t *testing.T) {
	root, started, ref := setupT3Scope180(t, false)
	finding := Finding{
		ID:             "F1",
		Severity:       "HIGH",
		Problem:        "unproven problem",
		Impact:         "unknown impact",
		Recommendation: "do something",
		Verification:   "verify later",
		Evidence:       []Evidence{},
	}
	got, err := Finish(context.Background(), root, FinishRequest{RunID: started.RunID, Reads: []ReadRef{ref}, Findings: []Finding{finding}})
	if err == nil || !strings.Contains(err.Error(), "EVIDENCE_REQUIRED") {
		t.Fatalf("finish accepted a formal finding without source evidence: got=%+v err=%v", got, err)
	}
	status, statusErr := Status(root, started.RunID)
	if statusErr != nil {
		t.Fatal(statusErr)
	}
	if status.Execution != "INCOMPLETE" {
		t.Fatalf("evidence-less finding must keep report incomplete: %+v", status)
	}
}

func Test180FinishSelectedPartialScopeIsUndeterminedAndKeepsKnownGap(t *testing.T) {
	root, started, ref := setupT3Scope180(t, true)
	got, err := Finish(context.Background(), root, FinishRequest{RunID: started.RunID, Reads: []ReadRef{ref}, Findings: []Finding{}})
	if err != nil {
		t.Fatalf("selected partial scope should produce an honest partial report: %v", err)
	}
	if got.Execution != "COMPLETE" || got.Coverage != "PARTIAL" || got.ReviewConclusion != "UNDETERMINED" {
		t.Fatalf("partial scope produced dishonest outcome: %+v", got)
	}
	report, err := os.ReadFile(started.ReportPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(report, []byte("A.entry receiver unresolved")) {
		t.Fatalf("known scope gap disappeared from report:\n%s", report)
	}
	if bytes.Contains(report, []byte("NO_ISSUES_FOUND")) {
		t.Fatalf("partial scope must never claim NO_ISSUES_FOUND:\n%s", report)
	}
}


func Test180FinishAcceptsLFVisibleEvidenceForCRLFSource(t *testing.T) {
	root := t.TempDir()
	content := []byte("class A {\r\n    void entry() { throw new IllegalStateException(\"boom\"); }\r\n}\r\n")
	if err := os.WriteFile(filepath.Join(root, "A.java"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	started, err := Start(root)
	if err != nil {
		t.Fatal(err)
	}
	runDir, state, err := loadRun(root, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	chain := Chain{ID: "C1", Name: "A.entry", Nodes: []Node{{Path: "A.java", Symbol: "A.entry", Role: "CONTROLLER", Workspace: "current"}}}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeScope180(runDir, started.RunID, "crlf-options", []string{"C1"}, []Chain{chain}, rootAbs); err != nil {
		t.Fatal(err)
	}
	state.ScopeReady = true
	state.Coverage = "COMPLETE"
	state.SelectedIDs = []string{"C1"}
	if err := writeState(runDir, state); err != nil {
		t.Fatal(err)
	}
	scope, err := loadScope180(runDir, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if len(scope.FindingReads) != 1 {
		t.Fatalf("expected one formal finding range, got %+v", scope.FindingReads)
	}
	evidenceRef := scope.FindingReads[0]
	reads := append([]ReadRef{}, scope.Reads...)
	declared := false
	for _, ref := range reads {
		if readKey(ref) == readKey(evidenceRef) {
			declared = true
			break
		}
	}
	if !declared {
		reads = append(reads, evidenceRef)
	}
	finding := Finding{
		ID:             "F1",
		Severity:       "HIGH",
		Problem:        "entry always throws",
		Impact:         "request always fails",
		Recommendation: "remove the unconditional throw",
		Verification:   "read the selected method",
		Evidence: []Evidence{{
			Ref:   evidenceRef,
			Quote: "    void entry() { throw new IllegalStateException(\"boom\"); }",
		}},
	}
	got, err := Finish(context.Background(), root, FinishRequest{
		RunID:        started.RunID,
		Reads:        reads,
		Findings:     []Finding{finding},
		PendingRisks: []string{},
		Gaps:         []string{},
	})
	if err != nil {
		t.Fatalf("LF-visible evidence must match CRLF source bytes: %v", err)
	}
	if got.Execution != "COMPLETE" || got.ReviewConclusion != "BLOCKING" {
		t.Fatalf("CRLF evidence review did not complete: %+v", got)
	}
}

func setupT3Scope180(t *testing.T, partial bool) (string, Outcome, ReadRef) {
	t.Helper()
	root := t.TempDir()
	content := []byte("class A { void entry() {} }\n")
	if err := os.WriteFile(filepath.Join(root, "A.java"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	started, err := Start(root)
	if err != nil {
		t.Fatal(err)
	}
	runDir, state, err := loadRun(root, started.RunID)
	if err != nil {
		t.Fatal(err)
	}
	chain := Chain{ID: "C1", Name: "A.entry", Nodes: []Node{{Path: "A.java", Symbol: "A.entry", Role: "CONTROLLER", Workspace: "current"}}, Unresolved: []string{}}
	coverage := "COMPLETE"
	if partial {
		chain.Unresolved = []string{"A.entry receiver unresolved"}
		coverage = "PARTIAL"
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeScope180(runDir, started.RunID, "t3-options", []string{"C1"}, []Chain{chain}, rootAbs); err != nil {
		t.Fatal(err)
	}
	state.ScopeReady = true
	state.Coverage = coverage
	state.SelectedIDs = []string{"C1"}
	if err := writeState(runDir, state); err != nil {
		t.Fatal(err)
	}
	ref := ReadRef{Path: "A.java", SHA256: bytesSHA256(content), StartLine: 1, EndLine: 1}
	return root, started, ref
}
