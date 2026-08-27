package finding

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

func DecodeProposals(data []byte) ([]Proposal, error) {
	var proposals []Proposal
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&proposals); err != nil {
		return nil, fmt.Errorf("FINDING_PROPOSAL_INVALID: decode: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("FINDING_PROPOSAL_INVALID: multiple JSON values are not allowed")
		}
		return nil, fmt.Errorf("FINDING_PROPOSAL_INVALID: trailing JSON: %w", err)
	}
	seen := map[string]bool{}
	for i := range proposals {
		if err := validateProposalShape160(proposals[i]); err != nil {
			return nil, fmt.Errorf("FINDING_PROPOSAL_INVALID: proposal %d: %w", i, err)
		}
		id := strings.TrimSpace(proposals[i].ProposalID)
		if seen[id] {
			return nil, fmt.Errorf("FINDING_PROPOSAL_INVALID: duplicate proposalId %q", id)
		}
		seen[id] = true
	}
	if proposals == nil {
		proposals = []Proposal{}
	}
	return proposals, nil
}

func validateProposalShape160(p Proposal) error {
	if strings.TrimSpace(p.ProposalID) == "" || strings.TrimSpace(p.ReviewUnitID) == "" || strings.TrimSpace(p.RuleID) == "" {
		return fmt.Errorf("proposalId, reviewUnitId and ruleId are required")
	}
	switch p.Category {
	case "PRODUCTION_CODE", "TEST_VALIDITY":
	default:
		return fmt.Errorf("invalid category %q", p.Category)
	}
	switch strings.ToLower(strings.TrimSpace(p.Severity)) {
	case "critical", "high", "medium", "low":
	default:
		return fmt.Errorf("invalid severity %q", p.Severity)
	}
	if strings.TrimSpace(p.Problem) == "" || strings.TrimSpace(p.Impact) == "" || strings.TrimSpace(p.Recommendation) == "" {
		return fmt.Errorf("problem, impact and recommendation are required")
	}
	if p.Confidence < 0 || p.Confidence > 1 {
		return fmt.Errorf("confidence must be within [0,1]")
	}
	switch p.Anchor.Kind {
	case AnchorLine:
		if strings.TrimSpace(p.Anchor.Path) == "" || p.Anchor.Line < 1 {
			return fmt.Errorf("LINE anchor requires path and positive line")
		}
	case AnchorSymbol:
		if strings.TrimSpace(p.Anchor.Symbol) == "" || p.Anchor.Line != 0 {
			return fmt.Errorf("SYMBOL anchor requires symbol and no line")
		}
	case AnchorFile:
		if strings.TrimSpace(p.Anchor.Path) == "" || p.Anchor.Line != 0 || strings.TrimSpace(p.Anchor.Symbol) != "" {
			return fmt.Errorf("FILE anchor requires only path")
		}
	case AnchorChangeSet:
		if strings.TrimSpace(p.Anchor.Path) != "" || p.Anchor.Line != 0 || strings.TrimSpace(p.Anchor.Symbol) != "" {
			return fmt.Errorf("CHANGESET anchor must not claim path, line or symbol")
		}
	default:
		return fmt.Errorf("invalid anchor kind %q", p.Anchor.Kind)
	}
	if len(p.EvidenceRefs) == 0 {
		return fmt.Errorf("evidenceRefs must not be empty")
	}
	for i, e := range p.EvidenceRefs {
		if err := validateEvidenceShape160(e); err != nil {
			return fmt.Errorf("evidenceRefs[%d]: %w", i, err)
		}
	}
	return nil
}

func validateEvidenceShape160(e EvidenceRef) error {
	kind := strings.ToUpper(strings.TrimSpace(e.Kind))
	switch kind {
	case "SYMBOL", "CHAIN":
		if strings.TrimSpace(e.Value) == "" {
			return fmt.Errorf("%s evidence requires value", kind)
		}
	case "SOURCE_RANGE", "CHANGED_RANGE":
		if strings.TrimSpace(e.Path) == "" || e.StartLine < 1 || e.EndLine < e.StartLine {
			return fmt.Errorf("%s evidence requires path and valid range", kind)
		}
	case "RESOURCE_RELATION":
		if strings.TrimSpace(e.Path) == "" {
			return fmt.Errorf("RESOURCE_RELATION evidence requires path")
		}
	default:
		return fmt.Errorf("unsupported evidence kind %q", e.Kind)
	}
	return nil
}
