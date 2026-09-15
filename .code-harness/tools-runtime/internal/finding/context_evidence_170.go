package finding

import (
	"fmt"
	"sort"
	"strings"

	"codea-harness-tools/internal/knowledge"
	"codea-harness-tools/internal/nav"
	"codea-harness-tools/internal/reviewunit"
)

func verifyContextRelationEvidence170(ctx VerifyContext, unit reviewunit.Unit, ref EvidenceRef) (EvidenceRef, error) {
	if ctx.reviewContext170 == nil || strings.TrimSpace(ref.RelationID) == "" {
		return EvidenceRef{}, findingError160("CONTEXT_RELATION_NOT_VERIFIED", "relation authority is unavailable")
	}
	var relation *nav.Relation170
	for i := range ctx.reviewContext170.Relations {
		candidate := &ctx.reviewContext170.Relations[i]
		if candidate.ID == strings.TrimSpace(ref.RelationID) {
			relation = candidate
			break
		}
	}
	if relation == nil {
		return EvidenceRef{}, findingError160("CONTEXT_RELATION_NOT_VERIFIED", "relation %s is not present", ref.RelationID)
	}
	allowed := false
	for _, check := range ctx.reviewContext170.Checks {
		if check.ReviewUnitID != unit.ID {
			continue
		}
		for _, id := range check.RelationIDs {
			if id == relation.ID {
				allowed = true
				break
			}
		}
	}
	if !allowed {
		return EvidenceRef{}, findingError160("CONTEXT_RELATION_NOT_VERIFIED", "relation %s is not bound to ReviewUnit %s", relation.ID, unit.ID)
	}
	workspace := strings.TrimSpace(ref.Workspace)
	if workspace != "" && workspace != relation.From.Workspace {
		return EvidenceRef{}, findingError160("CONTEXT_RELATION_NOT_VERIFIED", "relation workspace mismatch")
	}
	side := strings.ToUpper(strings.TrimSpace(ref.SourceSide))
	if side == "" {
		side = relation.From.Side
	}
	if side != relation.From.Side || (side != "BASE" && side != "CURRENT") {
		return EvidenceRef{}, findingError160("CONTEXT_RELATION_NOT_VERIFIED", "relation source side mismatch")
	}
	if strings.TrimSpace(ref.Value) != "" {
		matched := false
		for _, target := range relation.Targets {
			key, err := nav.ReviewRefKey170(target)
			if err == nil && key == strings.TrimSpace(ref.Value) {
				matched = true
				break
			}
		}
		if !matched {
			return EvidenceRef{}, findingError160("CONTEXT_RELATION_NOT_VERIFIED", "relation target mismatch")
		}
	}
	v := ref
	v.Kind = "CONTEXT_RELATION"
	v.RelationID = relation.ID
	v.Workspace = relation.From.Workspace
	v.SourceSide = side
	return v, nil
}

func verifyBusinessRuleEvidence170(ctx VerifyContext, unit reviewunit.Unit, ref EvidenceRef) (EvidenceRef, error) {
	if ctx.knowledge170 == nil {
		return EvidenceRef{}, findingError160("BUSINESS_RULE_NOT_VERIFIED", "knowledge authority is unavailable")
	}
	if strings.TrimSpace(ref.SourceID) == "" || strings.TrimSpace(ref.RuleID) == "" || strings.TrimSpace(ref.SourceSHA256) == "" {
		return EvidenceRef{}, findingError160("BUSINESS_RULE_NOT_VERIFIED", "sourceId/ruleId/sourceSha256 are required")
	}
	if digest, err := knowledge.Digest170(*ctx.knowledge170); err != nil || digest != strings.TrimSpace(ctx.dispatch.KnowledgeSHA256) {
		return EvidenceRef{}, findingError160("BUSINESS_RULE_NOT_VERIFIED", "knowledge digest does not match dispatch")
	}
	ruleKey := "BUSINESS:" + strings.TrimSpace(ref.SourceID) + ":" + strings.TrimSpace(ref.RuleID)
	checkOK := false
	for _, check := range ctx.knowledge170.Checks {
		if check.ReviewUnitID == unit.ID && check.RuleKey == ruleKey && check.SourceID == ref.SourceID && check.RuleID == ref.RuleID && check.Status == "READY" {
			checkOK = true
			break
		}
	}
	if !checkOK {
		return EvidenceRef{}, findingError160("BUSINESS_RULE_NOT_VERIFIED", "business rule is not READY for ReviewUnit")
	}
	for _, source := range ctx.knowledge170.Sources {
		if source.SourceID == ref.SourceID && source.Kind == "RULES" && source.Status == "READY" && source.SHA256 == ref.SourceSHA256 && source.Rule != nil && source.Rule.RuleID == ref.RuleID && source.Rule.Status == "ACTIVE" {
			v := ref
			v.Kind = "BUSINESS_RULE"
			return v, nil
		}
	}
	return EvidenceRef{}, findingError160("BUSINESS_RULE_NOT_VERIFIED", "source metadata or digest mismatch")
}

func knowledgeAnchorForbidden170(ctx VerifyContext, anchor Anchor) bool {
	if ctx.knowledge170 == nil || strings.TrimSpace(anchor.Path) == "" {
		return false
	}
	candidate, ok := safeFindingPath160(anchor.Path)
	if !ok {
		return false
	}
	for _, source := range ctx.knowledge170.Sources {
		if p, ok := safeFindingPath160(source.Path); ok && p == candidate {
			return true
		}
	}
	return false
}

func validateRelationDispatch170(ctx VerifyContext, unit reviewunit.Unit, ruleID string, refs []EvidenceRef) error {
	if ctx.reviewContext170 == nil {
		return nil
	}
	for _, ref := range refs {
		if strings.ToUpper(strings.TrimSpace(ref.Kind)) != "CONTEXT_RELATION" {
			continue
		}
		matched := false
		for _, check := range ctx.reviewContext170.Checks {
			if check.ReviewUnitID != unit.ID || check.RuleID != ruleID || check.Status != "READY" {
				continue
			}
			for _, id := range check.RelationIDs {
				if id == ref.RelationID {
					matched = true
				}
			}
		}
		if !matched {
			return findingError160("CONTEXT_RELATION_NOT_VERIFIED", "relation %s is not READY for rule %s", ref.RelationID, ruleID)
		}
	}
	return nil
}

func hasCurrentContextRelation170(refs []EvidenceRef) bool {
	for _, ref := range refs {
		if strings.EqualFold(strings.TrimSpace(ref.Kind), "CONTEXT_RELATION") && strings.EqualFold(strings.TrimSpace(ref.SourceSide), "CURRENT") {
			return true
		}
	}
	return false
}

func hasBusinessCodeEvidence170(refs []EvidenceRef) bool {
	for _, ref := range refs {
		switch strings.ToUpper(strings.TrimSpace(ref.Kind)) {
		case "CHANGED_RANGE", "SOURCE_RANGE", "SYMBOL", "CHAIN", "RESOURCE_RELATION":
			return true
		case "CONTEXT_RELATION":
			if strings.EqualFold(strings.TrimSpace(ref.SourceSide), "CURRENT") {
				return true
			}
		}
	}
	return false
}

func normalizedReasons170(in []string) []string {
	out := append([]string(nil), in...)
	for i := range out {
		out[i] = strings.TrimSpace(out[i])
	}
	sort.Strings(out)
	return out
}

func blockedReason170(prefix string, reasons []string) []string {
	if len(reasons) == 0 {
		return []string{prefix}
	}
	out := make([]string, 0, len(reasons))
	for _, reason := range normalizedReasons170(reasons) {
		out = append(out, fmt.Sprintf("%s:%s", prefix, reason))
	}
	return out
}
