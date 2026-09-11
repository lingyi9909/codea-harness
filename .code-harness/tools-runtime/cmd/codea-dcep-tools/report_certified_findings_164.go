package main

import (
	"encoding/json"
	"fmt"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/finding"
	"codea-harness-tools/internal/report"
)

func applyCertifiedFindingAuthority164(runID string, analysisCert analysisruntime.Certificate, authoritative *report.ReviewRequest) error {
	set, cert, err := finding.LoadCertifiedWithCertificate(".", runID)
	if err != nil {
		return fmt.Errorf("REVIEWER_UNAVAILABLE: MANUAL_ACTION_REQUIRED: HARD STOP: Runtime-certified Reviewer findings unavailable: %w", err)
	}
	if set.RunID != runID || cert.RunID != runID || set.HarnessVersion != analysisCert.RuntimeVersion || set.ChangeSetSHA256 != analysisCert.ChangeSetSHA256 || set.ChangeAnalysisSHA256 != analysisCert.AnalysisSHA256 {
		return fmt.Errorf("REVIEWER_UNAVAILABLE: MANUAL_ACTION_REQUIRED: HARD STOP: Certified Findings authority does not match Certified ChangeAnalysis")
	}
	authoritative.Findings = make([]report.Finding, 0, len(set.Findings))
	for _, source := range set.Findings {
		evidenceBytes, marshalErr := json.Marshal(source.EvidenceRefs)
		if marshalErr != nil {
			return fmt.Errorf("encode Runtime-certified finding evidence: %w", marshalErr)
		}
		authoritative.Findings = append(authoritative.Findings, report.Finding{
			ID:                 source.ID,
			Category:           source.Category,
			Severity:           source.Severity,
			File:               source.Anchor.Path,
			Line:               source.Anchor.Line,
			AnchorKind:         string(source.Anchor.Kind),
			Symbol:             source.Anchor.Symbol,
			Problem:            source.Problem,
			Evidence:           string(evidenceBytes),
			Impact:             source.Impact,
			Recommendation:     source.Recommendation,
			NeedsTest:          source.NeedsTest,
			IntroducedByChange: source.IntroducedByChange,
			Confidence:         source.Confidence,
		})
	}
	if authoritative.Coverage.Status != "COMPLETE" {
		authoritative.Result = report.ResultManualActionRequired
	} else if len(authoritative.Findings) == 0 {
		authoritative.Result = report.ResultPassed
	} else {
		authoritative.Result = report.ResultFailed
	}
	return nil
}
