package coverage

type Result struct {
	Status              string   `json:"status"`
	MissingChangedFiles []string `json:"missingChangedFiles,omitempty"`
	UnresolvedSymbols   []string `json:"unresolvedSymbols,omitempty"`
}

func Evaluate(changed, reviewed, unresolved []string) Result {
	seen := make(map[string]bool, len(reviewed))
	for _, p := range reviewed {
		seen[p] = true
	}
	var missing []string
	for _, p := range changed {
		if !seen[p] {
			missing = append(missing, p)
		}
	}
	status := "COMPLETE"
	if len(missing) > 0 || len(unresolved) > 0 {
		status = "PARTIAL"
	}
	return Result{Status: status, MissingChangedFiles: missing, UnresolvedSymbols: append([]string(nil), unresolved...)}
}
