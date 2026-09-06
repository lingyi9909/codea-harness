package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/report"
	"codea-harness-tools/internal/requestcontract"
	"codea-harness-tools/internal/reviewscope"
)

func runReport(args []string) error {
	if len(args) == 0 {
		return errors.New("report requires review or api-doc")
	}
	switch args[0] {
	case "review":
		return runReviewReport(args[1:])
	case "api-doc":
		return runApiDocReport(args[1:])
	default:
		return fmt.Errorf("unknown report action %q", args[0])
	}
}

func runReviewReport(args []string) error {
	fs := flag.NewFlagSet("report review", flag.ContinueOnError)
	input := fs.String("input", "", "structured review report request under .code-harness/runs/<runId>/requests")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *input == "" {
		return errors.New("report review requires --input")
	}

	runID, cleanInput, err := validateAnalysisRequestPath153(*input)
	if err != nil {
		return errors.New("review report input must be under .code-harness/runs/<runId>/requests")
	}
	if err := verifyReviewTransportPath153(runID, cleanInput); err != nil {
		return err
	}
	data, err := os.ReadFile(cleanInput)
	if err != nil {
		return fmt.Errorf("read review report request: %w", err)
	}
	if err := requestcontract.Validate("report-review-request.schema.json", data); err != nil {
		return fmt.Errorf("REPORT_REVIEW_REQUEST_SCHEMA_INVALID: %w", err)
	}
	proposal, err := decodeReviewTransport153(data)
	if err != nil {
		return err
	}
	if proposal.RunID != runID {
		return fmt.Errorf("REVIEW_REPORT_RUN_ID_MISMATCH: body runId %q path runId %q", proposal.RunID, runID)
	}

	analysisPath := filepath.ToSlash(filepath.Join(".code-harness", "runs", runID, "analysis", "change-analysis.json"))
	certified, cert, err := analysisruntime.LoadCertified(".", analysisPath)
	if err != nil {
		return err
	}
	certifiedJSON, err := json.Marshal(certified)
	if err != nil {
		return fmt.Errorf("encode Certified ChangeAnalysis for report: %w", err)
	}
	selectionJSON, err := reviewSelectionProposal153(proposal)
	if err != nil {
		return err
	}
	verifiedScope, err := reviewscope.Verify(selectionJSON, certifiedJSON)
	if err != nil {
		return err
	}
	machine, err := reviewscope.ComputeCoverageFromAnalysis(verifiedScope, certifiedJSON)
	if err != nil {
		return err
	}

	authoritative := buildCertifiedReviewRequest153(proposal, certified, cert, verifiedScope, machine)
	path, err := report.Write(".", authoritative)
	if err != nil {
		return err
	}
	if err := os.Remove(cleanInput); err != nil {
		return fmt.Errorf("remove review report transport after success: %w", err)
	}
	return reportPathJSON(path)
}

func decodeReviewTransport153(data []byte) (report.ReviewRequest, error) {
	var req report.ReviewRequest
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return report.ReviewRequest{}, fmt.Errorf("decode review report request: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return report.ReviewRequest{}, errors.New("decode review report request: multiple JSON values are not allowed")
		}
		return report.ReviewRequest{}, fmt.Errorf("decode review report request: %w", err)
	}
	return req, nil
}

func verifyReviewTransportPath153(runID, input string) error {
	root, err := filepath.Abs(".")
	if err != nil {
		return err
	}
	inputAbs, err := filepath.Abs(input)
	if err != nil {
		return err
	}
	requestRoot := filepath.Join(root, ".code-harness", "runs", runID, "requests")
	rel, err := filepath.Rel(requestRoot, inputAbs)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("review report input must be under .code-harness/runs/<runId>/requests")
	}
	realRoot, err := filepath.EvalSymlinks(requestRoot)
	if err != nil {
		return fmt.Errorf("resolve review report request directory: %w", err)
	}
	realInput, err := filepath.EvalSymlinks(inputAbs)
	if err != nil {
		return fmt.Errorf("resolve review report input: %w", err)
	}
	realRel, err := filepath.Rel(realRoot, realInput)
	if err != nil || realRel == "." || realRel == ".." || strings.HasPrefix(realRel, ".."+string(filepath.Separator)) {
		return errors.New("review report input must be under .code-harness/runs/<runId>/requests")
	}
	return nil
}

func reviewSelectionProposal153(req report.ReviewRequest) ([]byte, error) {
	mode := strings.ToUpper(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = "FULL"
	}
	selection := reviewscope.Selection{Mode: mode, ScopedFiles: append([]string(nil), req.Scope.ScopedFiles...)}
	for _, c := range req.Coverage.CallChains {
		selection.SelectedCallChains = append(selection.SelectedCallChains, reviewscope.CallChain{EntryPoint: c.EntryPoint, Chain: append([]string(nil), c.Chain...)})
	}
	if req.Target != nil {
		selection.Target = &reviewscope.Target{Symbol: req.Target.Symbol, Kind: req.Target.Kind}
	}
	b, err := json.Marshal(selection)
	if err != nil {
		return nil, fmt.Errorf("encode review scope proposal: %w", err)
	}
	return b, nil
}

func buildCertifiedReviewRequest153(proposal report.ReviewRequest, certified analysisruntime.ChangeAnalysis, cert analysisruntime.Certificate, scope reviewscope.Selection, machine reviewscope.CoverageResult) report.ReviewRequest {
	out := proposal
	out.HarnessVersion = cert.RuntimeVersion
	out.BaseRef = cert.BaseRef
	out.Head = cert.Head
	out.Mode = scope.Mode
	if scope.Target == nil {
		out.Target = nil
	} else {
		out.Target = &report.ReviewTarget{Symbol: scope.Target.Symbol, Kind: scope.Target.Kind}
	}
	out.Scope = report.ReviewScope{ChangedFiles: certifiedChangedPaths153(certified)}
	if scope.Mode == "TARGETED" {
		out.Scope.ScopedFiles = append([]string(nil), scope.ScopedFiles...)
	}
	out.Coverage = report.ReviewCoverage{
		ReviewedFiles:        reviewedPathsForScope153(certified, scope),
		CallChains:           reportCallChains153(certified, scope),
		SymbolRoleEvidence:   reportSymbolEvidence153(certified),
		ResourceRoleEvidence: reportResourceEvidence153(certified),
		ExternalDependencies: append([]string(nil), certified.ExternalDependencies...),
		Unresolved:           reportUnresolved153(certified),
		MissingReviewedFiles: append([]string(nil), machine.MissingFiles...),
		RuntimeErrors:        []string{},
		Status:               machine.Status,
	}
	if machine.Status != "COMPLETE" {
		out.Result = report.ResultManualActionRequired
	}
	return out
}

func certifiedChangedPaths153(a analysisruntime.ChangeAnalysis) []string {
	out := make([]string, 0, len(a.ChangedFiles))
	for _, f := range a.ChangedFiles {
		out = append(out, filepath.ToSlash(filepath.Clean(f.Path)))
	}
	return uniqueStrings153(out)
}

func reviewedPathsForScope153(a analysisruntime.ChangeAnalysis, scope reviewscope.Selection) []string {
	allowed := map[string]bool{}
	if scope.Mode == "TARGETED" {
		for _, p := range scope.ScopedFiles {
			allowed[filepath.ToSlash(filepath.Clean(p))] = true
		}
	}
	out := []string{}
	for _, f := range a.ReviewCoverage.ReviewedFiles {
		p := filepath.ToSlash(filepath.Clean(f.Path))
		if scope.Mode == "TARGETED" && !allowed[p] {
			continue
		}
		out = append(out, p)
	}
	return uniqueStrings153(out)
}

func reportCallChains153(a analysisruntime.ChangeAnalysis, scope reviewscope.Selection) []report.CallChain {
	out := []report.CallChain{}
	if scope.Mode == "TARGETED" {
		for _, c := range scope.SelectedCallChains {
			out = append(out, report.CallChain{EntryPoint: c.EntryPoint, Chain: append([]string(nil), c.Chain...)})
		}
		return out
	}
	for _, c := range a.CallChains {
		out = append(out, report.CallChain{EntryPoint: c.EntryPoint, Chain: append([]string(nil), c.Chain...)})
	}
	return out
}

func reportSymbolEvidence153(a analysisruntime.ChangeAnalysis) []report.SymbolRoleEvidence {
	out := []report.SymbolRoleEvidence{}
	seen := map[string]bool{}
	for _, loc := range a.SymbolLocations {
		workspaceID := strings.TrimSpace(loc.Workspace)
		if workspaceID != "" && workspaceID != "current" {
			continue
		}
		switch loc.Source {
		case "FIND_SYMBOL", "FIND_REFERENCES", "FIND_IMPLEMENTATIONS":
		default:
			continue
		}
		if seen[loc.Symbol] {
			continue
		}
		seen[loc.Symbol] = true
		out = append(out, report.SymbolRoleEvidence{Symbol: loc.Symbol, Role: loc.Role, Source: loc.Source})
	}
	return out
}

func reportResourceEvidence153(a analysisruntime.ChangeAnalysis) []report.ResourceRoleEvidence {
	out := make([]report.ResourceRoleEvidence, 0, len(a.ResourceRelations))
	for _, relation := range a.ResourceRelations {
		out = append(out, report.ResourceRoleEvidence{Resource: relation.Resource, Role: relation.Role, Source: relation.Source})
	}
	return out
}

func reportUnresolved153(a analysisruntime.ChangeAnalysis) []string {
	out := []string{}
	for _, u := range a.ReviewCoverage.UnresolvedSymbols {
		item := strings.TrimSpace(u.Symbol)
		if strings.TrimSpace(u.From) != "" {
			item += " <- " + strings.TrimSpace(u.From)
		}
		if strings.TrimSpace(u.Reason) != "" {
			item += ": " + strings.TrimSpace(u.Reason)
		}
		if item != "" {
			out = append(out, item)
		}
	}
	return uniqueStrings153(out)
}

func uniqueStrings153(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range in {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func runApiDocReport(args []string) error {
	fs := flag.NewFlagSet("report api-doc", flag.ContinueOnError)
	input := fs.String("input", "", "structured api-doc report request under .code-harness/runs/<runId>/requests")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *input == "" {
		return errors.New("report api-doc requires --input")
	}
	path, err := report.WriteApiDocRequestFile(".", *input)
	if err != nil {
		return err
	}
	return reportPathJSON(path)
}

func reportPathJSON(path string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(cwd, path)
	if err != nil {
		return err
	}
	return writeJSONAndStatus(map[string]any{"status": "OK", "reportPath": filepath.ToSlash(rel)}, true)
}
