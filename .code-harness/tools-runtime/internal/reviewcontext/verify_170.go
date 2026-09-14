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
	if len(claimed.Relations) != len(fresh.Relations) {
		return fmt.Errorf("CONTEXT_RELATION_NOT_VERIFIED: relation set size changed")
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
	claimedChecks, err := json.Marshal(claimed.Checks)
	if err != nil {
		return err
	}
	freshChecks, err := json.Marshal(fresh.Checks)
	if err != nil {
		return err
	}
	if string(claimedChecks) != string(freshChecks) {
		return fmt.Errorf("CONTEXT_RELATION_NOT_VERIFIED: rule ownership/checks changed")
	}
	return nil
}
