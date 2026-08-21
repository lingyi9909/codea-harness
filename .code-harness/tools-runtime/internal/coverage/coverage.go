package coverage

import (
	"encoding/json"
	"fmt"
	"sort"
)

type Result struct {
	Status              string   `json:"status"`
	MissingChangedFiles []string `json:"missingChangedFiles,omitempty"`
	UnresolvedSymbols   []string `json:"unresolvedSymbols,omitempty"`
}

func Evaluate(changed, reviewed, unresolved []string) Result {
	return EvaluateRequired(changed, reviewed, unresolved)
}

// EvaluateRequired machine-checks exactly the files required by the declared review scope.
// FULL passes the complete changed-file set; TARGETED passes only verified scopedFiles.
func EvaluateRequired(required, reviewed, unresolved []string) Result {
	seen := make(map[string]bool, len(reviewed))
	for _, p := range reviewed {
		seen[p] = true
	}
	var missing []string
	for _, p := range required {
		if !seen[p] {
			missing = append(missing, p)
		}
	}
	sort.Strings(missing)
	status := "COMPLETE"
	if len(missing) > 0 || len(unresolved) > 0 {
		status = "PARTIAL"
	}
	return Result{Status: status, MissingChangedFiles: missing, UnresolvedSymbols: append([]string(nil), unresolved...)}
}

type analysis struct {
	ChangedFiles []struct {
		Path string `json:"path"`
	} `json:"changedFiles"`
	ReviewCoverage struct {
		Status        string `json:"status"`
		ReviewedFiles []struct {
			Path string `json:"path"`
		} `json:"reviewedFiles"`
		UnresolvedSymbols []struct {
			Symbol string `json:"symbol"`
		} `json:"unresolvedSymbols"`
	} `json:"reviewCoverage"`
}

func VerifyAnalysisJSON(data []byte) (Result, error) {
	var a analysis
	if err := json.Unmarshal(data, &a); err != nil {
		return Result{}, fmt.Errorf("parse change analysis: %w", err)
	}
	changed := make([]string, 0, len(a.ChangedFiles))
	for _, f := range a.ChangedFiles {
		changed = append(changed, f.Path)
	}
	reviewed := make([]string, 0, len(a.ReviewCoverage.ReviewedFiles))
	for _, f := range a.ReviewCoverage.ReviewedFiles {
		reviewed = append(reviewed, f.Path)
	}
	unresolved := make([]string, 0, len(a.ReviewCoverage.UnresolvedSymbols))
	for _, s := range a.ReviewCoverage.UnresolvedSymbols {
		unresolved = append(unresolved, s.Symbol)
	}
	r := EvaluateRequired(changed, reviewed, unresolved)
	if a.ReviewCoverage.Status != r.Status {
		return r, fmt.Errorf("reviewCoverage.status=%s but machine status=%s", a.ReviewCoverage.Status, r.Status)
	}
	if r.Status != "COMPLETE" {
		return r, fmt.Errorf("review coverage incomplete: missingChangedFiles=%v unresolvedSymbols=%v", r.MissingChangedFiles, r.UnresolvedSymbols)
	}
	return r, nil
}
