package reviewcontext

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func VerifyRelations170(ctx context.Context, claimed Context170, input BuildInput170, resolver Resolver170) error {
	if strings.TrimSpace(claimed.RunID) != strings.TrimSpace(input.RunID) || strings.ToUpper(strings.TrimSpace(claimed.Phase)) != strings.ToUpper(strings.TrimSpace(input.Phase)) {
		return fmt.Errorf("CONTEXT_RELATION_NOT_VERIFIED: context identity mismatch")
	}
	fresh, err := Build170(ctx, input, resolver)
	if err != nil {
		return fmt.Errorf("CONTEXT_RELATION_NOT_VERIFIED: rebuild failed: %w", err)
	}
	freshByID := map[string][]byte{}
	for _, relation := range fresh.Relations {
		encoded, err := json.Marshal(relation)
		if err != nil {
			return fmt.Errorf("CONTEXT_RELATION_NOT_VERIFIED: encode fresh relation: %w", err)
		}
		freshByID[relation.ID] = encoded
	}
	for _, relation := range claimed.Relations {
		encoded, err := json.Marshal(relation)
		if err != nil {
			return fmt.Errorf("CONTEXT_RELATION_NOT_VERIFIED: encode claimed relation: %w", err)
		}
		verified, ok := freshByID[relation.ID]
		if !ok || string(verified) != string(encoded) {
			return fmt.Errorf("CONTEXT_RELATION_NOT_VERIFIED: relation %s differs from runtime resolution", relation.ID)
		}
	}
	for _, check := range claimed.Checks {
		for _, relationID := range check.RelationIDs {
			if _, ok := freshByID[relationID]; !ok {
				return fmt.Errorf("CONTEXT_RELATION_NOT_VERIFIED: check %s/%s references unverified relation %s", check.ReviewUnitID, check.RuleID, relationID)
			}
		}
	}
	return nil
}
