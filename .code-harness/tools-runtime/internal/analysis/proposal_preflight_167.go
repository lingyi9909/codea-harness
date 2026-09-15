package analysis

import (
	"fmt"
	"strings"

	"codea-harness-tools/internal/symbolid"
)

// ValidateSemanticProposalRefs167 checks only the submitted structure. It does
// not confer authority; certification still verifies live facts and coverage.
func ValidateSemanticProposalRefs167(a ChangeAnalysis) error {
	locations := map[string]bool{}
	bySymbol := map[string]map[string]bool{}
	for _, loc := range a.SymbolLocations {
		ref, ok := symbolid.FromLocation(loc.Workspace, loc.Path, loc.Symbol)
		if !ok {
			return fmt.Errorf("invalid symbol location %q", loc.Symbol)
		}
		key, _ := symbolid.Key(ref)
		locations[key] = true
		if bySymbol[ref.Symbol] == nil {
			bySymbol[ref.Symbol] = map[string]bool{}
		}
		bySymbol[ref.Symbol][key] = true
	}
	resolve := func(symbol string, ref *SymbolRef) error {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			return fmt.Errorf("empty symbol")
		}
		if ref != nil {
			normalized, ok := symbolid.Normalize(*ref)
			if !ok || normalized.Symbol != symbol {
				return fmt.Errorf("exact ref does not match symbol %q", symbol)
			}
			key, _ := symbolid.Key(normalized)
			if !locations[key] {
				return fmt.Errorf("exact location missing for %q at %q", symbol, ref.Path)
			}
			return nil
		}
		if len(bySymbol[symbol]) != 1 {
			return fmt.Errorf("missing or ambiguous location for %q", symbol)
		}
		return nil
	}
	for i, chain := range a.CallChains {
		if len(chain.ChainRefs) != 0 && len(chain.ChainRefs) != len(chain.Chain) {
			return fmt.Errorf("callChains[%d] chainRefs length=%d chain length=%d; supply one exact ref per node in order", i, len(chain.ChainRefs), len(chain.Chain))
		}
		if err := resolve(chain.EntryPoint, chain.EntryPointRef); err != nil {
			return fmt.Errorf("callChains[%d] entryPointRef: %w", i, err)
		}
		for j, node := range chain.Chain {
			var ref *SymbolRef
			if len(chain.ChainRefs) > 0 {
				ref = &chain.ChainRefs[j]
			}
			if err := resolve(node, ref); err != nil {
				return fmt.Errorf("callChains[%d] chainRefs[%d]: %w", i, j, err)
			}
		}
	}
	return nil
}
