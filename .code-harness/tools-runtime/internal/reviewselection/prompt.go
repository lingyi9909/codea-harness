package reviewselection

import (
	"fmt"
	"strings"
)

// SelectionPrompt is both the displayed menu and the Host-attested text. Exact
// paths distinguish branches with identical symbols in different modules.
func SelectionPrompt(options Options) string {
	if options.Decision != DecisionUser {
		return ""
	}
	lines := []string{"Run: " + options.RunID, "Options: " + options.OptionsHash, "Review: " + options.Intent.Mode + " " + options.Intent.Target}
	for _, c := range options.Chains {
		symbols := append([]string(nil), c.CallChain.Chain...)
		for i, ref := range c.CallChain.ChainRefs {
			if i < len(symbols) {
				symbols[i] += fmt.Sprintf(" [%s:%s]", ref.Workspace, ref.Path)
			}
		}
		lines = append(lines, c.SelectionID+": "+strings.Join(symbols, " -> "))
	}
	return strings.Join(lines, "\n")
}
