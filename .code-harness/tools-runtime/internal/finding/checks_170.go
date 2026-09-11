package finding

// CheckResult170 freezes the Reviewer -> Runtime check-completion wire type.
// T8 validates and activates it together with Host-v2 evidence. T1 does not
// change production stage progression or enable the new Host branch.
type CheckResult170 struct {
	ReviewUnitID string   `json:"reviewUnitId"`
	RuleID       string   `json:"ruleId"`
	Status       string   `json:"status"` // COMPLETED | INCOMPLETE
	Reason       string   `json:"reason"`
	SourceIDs    []string `json:"sourceIds"`
}

// 1.7 stage ownership remains the existing eight-stage Runtime authority:
// DISCOVERY is performed within CHANGE_ANALYSIS; knowledge loading and RULES
// context are REVIEW_PLANNING work. T5 moves the existing dispatch-driven
// planning advance to final planning completion. No Agent-controlled stage
// field is introduced here. CHANGE_ANALYSIS keeps Host-v1 semantics; FINDINGS
// gains Host-v2 checks only when T8 selects it from trusted run version data.
