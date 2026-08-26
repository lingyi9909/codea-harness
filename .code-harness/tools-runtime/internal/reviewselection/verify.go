package reviewselection

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/chain"
	"codea-harness-tools/internal/reviewscope"
)

type SelectionRequest struct {
	RunID        string   `json:"runId"`
	Mode         string   `json:"mode"`
	SelectionIDs []string `json:"selectionIds,omitempty"`
	OptionsHash  string   `json:"optionsHash"`
}

func validateSelectionAgainstOptions153(options Options, req SelectionRequest) ([]ChainOption, error) {
	if strings.TrimSpace(req.RunID) == "" || req.RunID != options.RunID || strings.TrimSpace(req.OptionsHash) == "" || req.OptionsHash != options.OptionsHash {
		return nil, fmt.Errorf("REVIEW_OPTIONS_STALE")
	}
	mode := strings.ToUpper(strings.TrimSpace(req.Mode))
	switch options.Decision {
	case DecisionAutoFull:
		if mode != "FULL" || len(req.SelectionIDs) != 0 {
			return nil, fmt.Errorf("REVIEW_SELECTION_SCOPE_INVALID: AUTO_FULL requires FULL with no selectionIds")
		}
		return []ChainOption{}, nil
	case DecisionAutoSingle:
		if mode != "TARGETED" || len(req.SelectionIDs) != 1 || len(options.AutoSelectionIDs) != 1 {
			return nil, fmt.Errorf("REVIEW_SELECTION_SCOPE_INVALID: AUTO_SINGLE requires exact Runtime autoSelectionId")
		}
		if req.SelectionIDs[0] != options.AutoSelectionIDs[0] {
			return nil, fmt.Errorf("REVIEW_SELECTION_UNKNOWN_CHAIN: %s", strings.TrimSpace(req.SelectionIDs[0]))
		}
	case DecisionUser:
		switch mode {
		case "FULL":
			if len(req.SelectionIDs) != 0 {
				return nil, fmt.Errorf("REVIEW_SELECTION_SCOPE_INVALID: FULL must not contain selectionIds")
			}
			return []ChainOption{}, nil
		case "LIST":
			if len(req.SelectionIDs) != 0 {
				return nil, fmt.Errorf("REVIEW_SELECTION_SCOPE_INVALID: LIST must not contain selectionIds")
			}
			return []ChainOption{}, nil
		case "TARGETED":
			if len(req.SelectionIDs) == 0 {
				return nil, fmt.Errorf("REVIEW_SELECTION_SCOPE_INVALID: USER_SELECTION TARGETED requires selectionIds")
			}
		default:
			return nil, fmt.Errorf("REVIEW_SELECTION_SCOPE_INVALID: USER_SELECTION requires FULL, TARGETED, or LIST")
		}
	default:
		return nil, fmt.Errorf("REVIEW_SELECTION_SCOPE_INVALID: unknown Runtime decision %q", options.Decision)
	}
	requested := map[string]bool{}
	for _, id := range req.SelectionIDs {
		id = strings.TrimSpace(id)
		if id == "" || requested[id] {
			return nil, fmt.Errorf("REVIEW_SELECTION_SCOPE_INVALID: duplicate/empty selectionId")
		}
		requested[id] = true
	}
	selected := make([]ChainOption, 0, len(requested))
	for _, option := range options.Chains {
		if requested[option.SelectionID] {
			selected = append(selected, option)
			delete(requested, option.SelectionID)
		}
	}
	if len(requested) != 0 {
		var unknown []string
		for id := range requested {
			unknown = append(unknown, id)
		}
		sort.Strings(unknown)
		return nil, fmt.Errorf("REVIEW_SELECTION_UNKNOWN_CHAIN: %s", strings.Join(unknown, ","))
	}
	return selected, nil
}

func VerifyAndBuildScope(root string, req SelectionRequest) (reviewscope.Selection, error) {
	root = filepath.Clean(root)
	if strings.TrimSpace(req.RunID) == "" {
		return reviewscope.Selection{}, fmt.Errorf("REVIEW_SELECTION_SCOPE_INVALID: missing runId")
	}
	analysisPath := filepath.ToSlash(filepath.Join(".code-harness", "runs", req.RunID, "analysis", "change-analysis.json"))
	analysis, cert, err := analysisruntime.LoadCertified(root, analysisPath)
	if err != nil {
		return reviewscope.Selection{}, fmt.Errorf("REVIEW_OPTIONS_STALE: %w", err)
	}
	if cert.RunID != req.RunID {
		return reviewscope.Selection{}, fmt.Errorf("REVIEW_OPTIONS_STALE")
	}

	optionsPath := filepath.Join(root, ".code-harness", "runs", req.RunID, "analysis", "review-options.json")
	optionsBytes, err := os.ReadFile(optionsPath)
	if err != nil {
		return reviewscope.Selection{}, fmt.Errorf("REVIEW_OPTIONS_STALE: %w", err)
	}
	var stored Options
	if err := decodeStrictReviewArtifact153(optionsBytes, &stored); err != nil {
		return reviewscope.Selection{}, fmt.Errorf("REVIEW_OPTIONS_STALE: %w", err)
	}
	if stored.RunID != cert.RunID || stored.ChangeSetSHA256 != cert.ChangeSetSHA256 || stored.EntrypointCompleteness != "COMPLETE" {
		return reviewscope.Selection{}, fmt.Errorf("REVIEW_OPTIONS_STALE")
	}

	origin, err := loadOptionsOrigin153(root, req.RunID)
	if err != nil {
		return reviewscope.Selection{}, fmt.Errorf("REVIEW_OPTIONS_STALE: %w", err)
	}
	if origin.RunID != cert.RunID || origin.ChangeSetSHA256 != cert.ChangeSetSHA256 || origin.AnalysisSHA256 != cert.AnalysisSHA256 {
		return reviewscope.Selection{}, fmt.Errorf("REVIEW_OPTIONS_STALE: origin identity changed")
	}
	normalizedIntent, err := normalizeReviewIntent153(origin.Intent)
	if err != nil || !reflect.DeepEqual(origin.Intent, normalizedIntent) {
		return reviewscope.Selection{}, fmt.Errorf("REVIEW_OPTIONS_STALE: origin intent changed")
	}

	// Stored review-options are only a Runtime artifact to consume, never the
	// source of truth for their own completeness. Rebuild the complete option
	// set from the original Runtime-owned intent plus the current certified
	// analysis, entrypoint inventory, and current Chain facts, then compare the
	// entire artifact exactly before interpreting the selection request.
	authoritative, err := BuildOptionsForIntent(root, analysisPath, origin.Intent)
	if err != nil {
		return reviewscope.Selection{}, fmt.Errorf("REVIEW_OPTIONS_STALE: authoritative rebuild failed: %w", err)
	}
	if !reflect.DeepEqual(stored, authoritative) {
		return reviewscope.Selection{}, fmt.Errorf("REVIEW_OPTIONS_STALE: stored options differ from authoritative Runtime rebuild")
	}

	selected, err := validateSelectionAgainstOptions153(authoritative, req)
	if err != nil {
		return reviewscope.Selection{}, err
	}
	analysisBytes, err := json.Marshal(analysis)
	if err != nil {
		return reviewscope.Selection{}, fmt.Errorf("REVIEW_SELECTION_SCOPE_INVALID: %w", err)
	}
	switch strings.ToUpper(strings.TrimSpace(req.Mode)) {
	case "FULL":
		scope, err := reviewscope.BuildFullSelection(analysisBytes)
		if err != nil {
			return reviewscope.Selection{}, fmt.Errorf("REVIEW_SELECTION_SCOPE_INVALID: %w", err)
		}
		return scope, nil
	case "LIST":
		return reviewscope.Selection{Mode: "LIST", SelectedCallChains: []reviewscope.CallChain{}, ScopedFiles: []string{}}, nil
	case "TARGETED":
		callChains, err := selectedCallChains153(root, selected, analysis, cert)
		if err != nil {
			return reviewscope.Selection{}, fmt.Errorf("REVIEW_SELECTION_SCOPE_INVALID: %w", err)
		}
		var scope reviewscope.Selection
		if authoritative.Intent.Mode == "TARGETED" {
			target := reviewscope.Target{Symbol: authoritative.Intent.Target, Kind: reviewTargetKind153(authoritative.Intent.Target)}
			scope, err = reviewscope.BuildTargetedSelectionForTarget(callChains, analysisBytes, target)
		} else {
			scope, err = reviewscope.BuildTargetedSelection(callChains, analysisBytes)
		}
		if err != nil {
			return reviewscope.Selection{}, fmt.Errorf("REVIEW_SELECTION_SCOPE_INVALID: %w", err)
		}
		return scope, nil
	default:
		return reviewscope.Selection{}, fmt.Errorf("REVIEW_SELECTION_SCOPE_INVALID: unsupported mode")
	}
}

func loadOptionsOrigin153(root, runID string) (OptionsOrigin, error) {
	path := filepath.Join(root, ".code-harness", "runs", runID, "analysis", "review-options-origin.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return OptionsOrigin{}, fmt.Errorf("read review options origin: %w", err)
	}
	var origin OptionsOrigin
	if err := decodeStrictReviewArtifact153(data, &origin); err != nil {
		return OptionsOrigin{}, fmt.Errorf("decode review options origin: %w", err)
	}
	return origin, nil
}

func decodeStrictReviewArtifact153(data []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}

func reviewTargetKind153(target string) string {
	if strings.Contains(strings.TrimSpace(target), ".") {
		return "METHOD"
	}
	return "CLASS"
}

func loadOptionChain153(root string, option ChainOption, cert analysisruntime.Certificate) (chain.Chain, error) {
	switch option.Source {
	case "ACCEPTED":
		path, err := chain.ChainPath(root, option.ChainID)
		if err != nil {
			return chain.Chain{}, err
		}
		return chain.Load(path)
	case "TEMPORARY":
		candidatePath := filepath.ToSlash(filepath.Join(".code-harness", "runs", cert.RunID, "analysis", "discovered-chains", option.ChainID+".yaml"))
		candidate, _, err := chain.LoadRuntimeCandidate(root, candidatePath, cert)
		return candidate, err
	default:
		return chain.Chain{}, fmt.Errorf("invalid ReviewOption source %q", option.Source)
	}
}

func optionEntryPoints153(candidate chain.Chain) []string {
	values := make([]string, 0, len(candidate.EntryPoints))
	for _, entry := range candidate.EntryPoints {
		values = append(values, strings.TrimSpace(entry.Symbol))
	}
	return uniqueSorted153(values)
}

func selectedCallChains153(root string, selected []ChainOption, analysis analysisruntime.ChangeAnalysis, cert analysisruntime.Certificate) ([]reviewscope.CallChain, error) {
	seen := map[string]bool{}
	out := make([]reviewscope.CallChain, 0)
	for _, option := range selected {
		candidate, err := loadOptionChain153(root, option, cert)
		if err != nil {
			return nil, err
		}
		nodes := make([]string, 0, len(candidate.Nodes))
		for _, node := range candidate.Nodes {
			nodes = append(nodes, strings.TrimSpace(node.Symbol))
		}
		for _, entry := range candidate.EntryPoints {
			sequence := append([]string{strings.TrimSpace(entry.Symbol)}, nodes...)
			matched := false
			for _, current := range analysis.CallChains {
				if current.EntryPoint != entry.Symbol || !reflect.DeepEqual(current.Chain, sequence) {
					continue
				}
				keyBytes, _ := json.Marshal(current)
				key := string(keyBytes)
				if !seen[key] {
					out = append(out, reviewscope.CallChain{EntryPoint: current.EntryPoint, Chain: append([]string(nil), current.Chain...)})
					seen[key] = true
				}
				matched = true
			}
			if !matched {
				return nil, fmt.Errorf("selected Chain %s has no exact certified callChain for %s", candidate.ID, entry.Symbol)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		left, _ := json.Marshal(out[i])
		right, _ := json.Marshal(out[j])
		return string(left) < string(right)
	})
	return out, nil
}
