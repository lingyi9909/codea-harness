package report

import (
	"fmt"
	"sort"
	"strings"

	"codea-harness-tools/internal/finding"
)

func finalizeCertifiedRequest170(req ReviewRequest, set finding.CertifiedSet) (ReviewRequest, error) {
	req.Findings = mapCertifiedFindings160(set.Findings)
	if set.ReviewContext == nil {
		if len(req.Coverage.RuntimeErrors) > 0 {
			req.Result = ResultManualActionRequired
			req.Coverage.Status = "PARTIAL"
			return req, nil
		}
		if len(req.Findings) > 0 {
			req.Result = ResultFailed
		} else {
			req.Result = ResultPassed
		}
		return req, nil
	}

	summary := set.ReviewContext
	switch strings.ToUpper(strings.TrimSpace(summary.Status)) {
	case "COMPLETE":
		if len(summary.BlockedChecks) != 0 {
			return ReviewRequest{}, fmt.Errorf("CERTIFIED_FINDINGS_REVIEW_CONTEXT_INVALID: COMPLETE contains blocked checks")
		}
		if len(req.Coverage.RuntimeErrors) > 0 {
			req.Result = ResultManualActionRequired
			req.Coverage.Status = "PARTIAL"
			return req, nil
		}
		req.Coverage.Status = "COMPLETE"
		if len(req.Findings) > 0 {
			req.Result = ResultFailed
		} else {
			req.Result = ResultPassed
		}
		return req, nil
	case "PARTIAL":
		req.Result = ResultManualActionRequired
		req.Coverage.Status = "PARTIAL"
		for _, blocked := range summary.BlockedChecks {
			reasons := append([]string(nil), blocked.Reasons...)
			sort.Strings(reasons)
			if len(reasons) == 0 {
				reasons = []string{"INCOMPLETE"}
			}
			req.Coverage.Unresolved = append(req.Coverage.Unresolved,
				fmt.Sprintf("未完成检查 %s/%s: %s", blocked.ReviewUnitID, blocked.RuleID, strings.Join(reasons, ", ")))
		}
		sort.Strings(req.Coverage.Unresolved)
		return req, nil
	default:
		return ReviewRequest{}, fmt.Errorf("CERTIFIED_FINDINGS_REVIEW_CONTEXT_INVALID: status=%q", summary.Status)
	}
}
