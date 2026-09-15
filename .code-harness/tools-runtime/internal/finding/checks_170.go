package finding

import (
	"fmt"
	"sort"
	"strings"
)

type CheckResult170 struct {
	ReviewUnitID string   `json:"reviewUnitId"`
	RuleID       string   `json:"ruleId"`
	Status       string   `json:"status"` // COMPLETED | INCOMPLETE
	Reason       string   `json:"reason"`
	SourceIDs    []string `json:"sourceIds"`
}

type BlockedCheck170 struct {
	ReviewUnitID string   `json:"reviewUnitId"`
	RuleID       string   `json:"ruleId"`
	Reasons      []string `json:"reasons"`
}

type ReviewContextSummary170 struct {
	Status        string            `json:"status"` // COMPLETE | PARTIAL
	BlockedChecks []BlockedCheck170 `json:"blockedChecks"`
}

func ValidateCheckResults170(ctx VerifyContext, checks []CheckResult170) (ReviewContextSummary170, error) {
	dispatched := map[string]bool{}
	required := map[string]bool{}
	for _, d := range ctx.dispatch.Dispatches {
		key := d.ReviewUnitID + "\x00" + d.RuleID
		dispatched[key] = true
		required[key] = true
	}

	blocked := map[string]BlockedCheck170{}
	if ctx.reviewContext170 != nil {
		for _, check := range ctx.reviewContext170.Checks {
			key := check.ReviewUnitID + "\x00" + check.RuleID
			if check.Status == "BLOCKED" {
				delete(required, key)
				blocked[key] = BlockedCheck170{ReviewUnitID: check.ReviewUnitID, RuleID: check.RuleID, Reasons: blockedReason170("RUNTIME_BLOCKED", check.Reasons)}
			}
		}
	}
	if ctx.knowledge170 != nil {
		for _, check := range ctx.knowledge170.Checks {
			key := check.ReviewUnitID + "\x00" + check.RuleKey
			if check.Status == "BLOCKED" {
				delete(required, key)
				blocked[key] = BlockedCheck170{ReviewUnitID: check.ReviewUnitID, RuleID: check.RuleKey, Reasons: blockedReason170("KNOWLEDGE_BLOCKED", check.Reasons)}
			}
		}
	}

	seen := map[string]bool{}
	for _, result := range checks {
		result.ReviewUnitID = strings.TrimSpace(result.ReviewUnitID)
		result.RuleID = strings.TrimSpace(result.RuleID)
		result.Status = strings.ToUpper(strings.TrimSpace(result.Status))
		result.Reason = strings.TrimSpace(result.Reason)
		key := result.ReviewUnitID + "\x00" + result.RuleID
		if seen[key] {
			return ReviewContextSummary170{}, fmt.Errorf("REVIEW_CHECK_DUPLICATE: %s/%s", result.ReviewUnitID, result.RuleID)
		}
		seen[key] = true
		if !dispatched[key] {
			return ReviewContextSummary170{}, fmt.Errorf("REVIEW_CHECK_NOT_DISPATCHED: %s/%s", result.ReviewUnitID, result.RuleID)
		}
		if _, runtimeBlocked := blocked[key]; runtimeBlocked {
			continue
		}
		if result.Status != "COMPLETED" && result.Status != "INCOMPLETE" {
			return ReviewContextSummary170{}, fmt.Errorf("REVIEW_CHECK_INVALID_STATUS: %s", result.Status)
		}
		if strings.HasPrefix(result.RuleID, "BUSINESS:") {
			parts := strings.Split(result.RuleID, ":")
			if len(parts) != 3 {
				return ReviewContextSummary170{}, fmt.Errorf("REVIEW_CHECK_NOT_DISPATCHED: invalid business rule key")
			}
			sourceOK := false
			for _, id := range result.SourceIDs {
				if strings.TrimSpace(id) == parts[1] {
					sourceOK = true
					break
				}
			}
			if !sourceOK {
				return ReviewContextSummary170{}, fmt.Errorf("REVIEW_CHECK_SOURCE_NOT_READ: %s", parts[1])
			}
		}
		if result.Status == "INCOMPLETE" {
			if result.Reason == "" {
				return ReviewContextSummary170{}, fmt.Errorf("REVIEW_CHECK_INCOMPLETE_REASON_REQUIRED: %s/%s", result.ReviewUnitID, result.RuleID)
			}
			blocked[key] = BlockedCheck170{ReviewUnitID: result.ReviewUnitID, RuleID: result.RuleID, Reasons: []string{"REVIEWER_INCOMPLETE:" + result.Reason}}
		}
	}
	for key := range required {
		if !seen[key] {
			return ReviewContextSummary170{}, fmt.Errorf("REVIEW_CHECK_MISSING: %s", strings.ReplaceAll(key, "\x00", "/"))
		}
	}

	blockedChecks := make([]BlockedCheck170, 0, len(blocked))
	for _, check := range blocked {
		blockedChecks = append(blockedChecks, check)
	}
	sort.Slice(blockedChecks, func(i, j int) bool {
		if blockedChecks[i].ReviewUnitID != blockedChecks[j].ReviewUnitID {
			return blockedChecks[i].ReviewUnitID < blockedChecks[j].ReviewUnitID
		}
		return blockedChecks[i].RuleID < blockedChecks[j].RuleID
	})
	status := "COMPLETE"
	if len(blockedChecks) > 0 {
		status = "PARTIAL"
	}
	return ReviewContextSummary170{Status: status, BlockedChecks: blockedChecks}, nil
}
