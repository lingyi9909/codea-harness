package main

import (
	"fmt"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/finding"
)

// verifyCertifiedFindingAuthority164 binds findings to the certified analysis.
// The certified report writer loads and maps findings and determines the result.
func verifyCertifiedFindingAuthority164(runID string, analysisCert analysisruntime.Certificate) error {
	set, cert, err := finding.LoadCertifiedWithCertificate(".", runID)
	if err != nil {
		return fmt.Errorf("REVIEWER_UNAVAILABLE: MANUAL_ACTION_REQUIRED: HARD STOP: Runtime-certified Reviewer findings unavailable: %w", err)
	}
	if set.RunID != runID || cert.RunID != runID || set.HarnessVersion != analysisCert.RuntimeVersion || set.ChangeSetSHA256 != analysisCert.ChangeSetSHA256 || set.ChangeAnalysisSHA256 != analysisCert.AnalysisSHA256 {
		return fmt.Errorf("REVIEWER_UNAVAILABLE: MANUAL_ACTION_REQUIRED: HARD STOP: Certified Findings authority does not match Certified ChangeAnalysis")
	}
	return nil
}
