package reviewselection

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	analysisruntime "codea-harness-tools/internal/analysis"
	"codea-harness-tools/internal/chain"
	"codea-harness-tools/internal/reviewscope"
)

type Decision string

const (
	DecisionAutoFull   Decision = "AUTO_FULL"
	DecisionAutoSingle Decision = "AUTO_SINGLE"
	DecisionUser       Decision = "USER_SELECTION"
)

type ChainOption struct {
	SelectionID string                `json:"selectionId"`
	ChainID     string                `json:"chainId"`
	EntryPoints []string              `json:"entryPoints"`
	Source      string                `json:"source"`
	Status      string                `json:"status"`
	CallChain   reviewscope.CallChain `json:"callChain"`
}

type Options struct {
	RunID                  string                 `json:"runId"`
	ChangeSetSHA256        string                 `json:"changeSetSha256"`
	EntrypointCompleteness string                 `json:"entrypointCompleteness"`
	Intent                 analysisruntime.Intent `json:"intent"`
	Decision               Decision               `json:"decision"`
	AutoSelectionIDs       []string               `json:"autoSelectionIds,omitempty"`
	Chains                 []ChainOption          `json:"chains"`
	OptionsHash            string                 `json:"optionsHash"`
}

type OptionsOrigin struct {
	RunID           string                 `json:"runId"`
	ChangeSetSHA256 string                 `json:"changeSetSha256"`
	AnalysisSHA256  string                 `json:"analysisSha256"`
	Intent          analysisruntime.Intent `json:"intent"`
}

func BuildOptions(root string, certifiedAnalysisPath string, target string) (Options, error) {
	intent := analysisruntime.Intent{Mode: "FULL"}
	if target = strings.TrimSpace(target); target != "" {
		intent = analysisruntime.Intent{Mode: "TARGETED", Target: target}
	}
	return BuildOptionsForIntent(root, certifiedAnalysisPath, intent)
}

func BuildOptionsForIntent(root string, certifiedAnalysisPath string, intent analysisruntime.Intent) (Options, error) {
	root = filepath.Clean(root)
	intent, err := normalizeReviewIntent153(intent)
	if err != nil {
		return Options{}, err
	}
	analysis, cert, err := analysisruntime.LoadCertified(root, certifiedAnalysisPath)
	if err != nil {
		return Options{}, fmt.Errorf("REVIEW_OPTIONS_ANALYSIS_NOT_CERTIFIED: %w", err)
	}
	intent, err = authoritativeReviewIntent153(root, cert, intent)
	if err != nil {
		return Options{}, err
	}
	inventoryPath := filepath.Join(root, ".code-harness", "runs", cert.RunID, "analysis", "entrypoint-inventory.json")
	inventoryBytes, err := os.ReadFile(inventoryPath)
	if err != nil {
		return Options{}, fmt.Errorf("REVIEW_OPTIONS_INVENTORY_READ_FAILED: %w", err)
	}
	var inventory analysisruntime.EntrypointInventory
	if err := json.Unmarshal(inventoryBytes, &inventory); err != nil {
		return Options{}, fmt.Errorf("REVIEW_OPTIONS_INVENTORY_DECODE_FAILED: %w", err)
	}
	if inventory.RunID != cert.RunID || inventory.ChangeSetSHA256 != cert.ChangeSetSHA256 || strings.ToUpper(strings.TrimSpace(inventory.Status)) != "COMPLETE" {
		return Options{}, fmt.Errorf("ENTRYPOINT_COMPLETENESS_INCOMPLETE")
	}

	analysisBytes, err := json.Marshal(analysis)
	if err != nil {
		return Options{}, fmt.Errorf("REVIEW_OPTIONS_ANALYSIS_ENCODE_FAILED: %w", err)
	}
	selection := reviewscope.Selection{Mode: "FULL"}
	if intent.Mode == "TARGETED" {
		selected := matchingCallChains153(intent.Target, analysis.CallChains)
		if len(selected) == 0 {
			return Options{}, fmt.Errorf("REVIEW_TARGET_NOT_FOUND: %s", intent.Target)
		}
		selection = reviewscope.Selection{Mode: "TARGETED", SelectedCallChains: selected}
	}
	resolved, err := reviewscope.ResolveChainContexts(root, selection, analysisBytes, reviewscope.ChainResolveOptions{
		RunID: cert.RunID,
		CertifyDiscovered: func(candidate chain.Chain) error {
			candidatePath := filepath.ToSlash(filepath.Join(".code-harness", "runs", cert.RunID, "analysis", "discovered-chains", candidate.ID+".yaml"))
			_, err := chain.CertifyCandidate(root, candidate, candidatePath, "DISCOVERED", cert)
			return err
		},
	})
	if err != nil {
		return Options{}, fmt.Errorf("REVIEW_OPTIONS_CHAIN_RESOLUTION_FAILED: %w", err)
	}
	if resolved.Status == reviewscope.ChainResolutionStaleDecision {
		return Options{}, fmt.Errorf("REVIEW_CHAIN_STALE_REQUIRES_DECISION")
	}
	if resolved.Status == reviewscope.ChainResolutionPartial {
		return Options{}, fmt.Errorf("REVIEW_OPTIONS_CHAIN_RESOLUTION_PARTIAL: %s", strings.Join(resolved.Unresolved, "; "))
	}

	eligible := make([]reviewscope.CallChain, 0, len(analysis.CallChains))
	for _, current := range analysis.CallChains {
		eligible = append(eligible, reviewscope.CallChain(current))
	}
	if intent.Mode == "TARGETED" {
		eligible = selection.SelectedCallChains
	}
	mapped := map[string]bool{}

	chains := make([]ChainOption, 0, len(resolved.Contexts))
	for _, ctx := range resolved.Contexts {
		var candidate chain.Chain
		source := "ACCEPTED"
		status := "VALID"
		switch ctx.Source {
		case "ACCEPTED":
			path, err := chain.ChainPath(root, ctx.ID)
			if err != nil {
				return Options{}, err
			}
			candidate, err = chain.Load(path)
			if err != nil {
				return Options{}, fmt.Errorf("REVIEW_OPTIONS_CHAIN_LOAD_FAILED: %w", err)
			}
		case "DISCOVERED":
			candidatePath := filepath.ToSlash(filepath.Join(".code-harness", "runs", cert.RunID, "analysis", "discovered-chains", ctx.ID+".yaml"))
			var err error
			candidate, _, err = chain.LoadRuntimeCandidate(root, candidatePath, cert)
			if err != nil {
				return Options{}, fmt.Errorf("REVIEW_OPTIONS_CHAIN_LOAD_FAILED: %w", err)
			}
			source = "TEMPORARY"
			status = "TEMPORARY"
		default:
			return Options{}, fmt.Errorf("REVIEW_OPTIONS_CHAIN_SOURCE_INVALID: %s", ctx.Source)
		}
		for _, current := range eligible {
			payload := reviewscope.CallChain(current)
			if candidateContainsCallChain153(candidate, payload) {
				chains = append(chains, ChainOption{ChainID: candidate.ID, EntryPoints: []string{current.EntryPoint}, Source: source, Status: status, CallChain: payload})
				mapped[callChainKey153(payload)] = true
			}
		}
	}
	// Projection must never turn an unmapped certified chain into an automatic
	// zero/single decision. Chain resolution and menu projection must agree.
	for _, payload := range eligible {
		if !mapped[callChainKey153(payload)] {
			return Options{}, fmt.Errorf("REVIEW_OPTIONS_CHAIN_RESOLUTION_PARTIAL: certified callChain %q has no matching Business Chain context", payload.EntryPoint)
		}
	}

	return finalizeOptions153(Options{
		RunID:                  cert.RunID,
		ChangeSetSHA256:        cert.ChangeSetSHA256,
		EntrypointCompleteness: "COMPLETE",
		Intent:                 intent,
		Chains:                 chains,
	}, cert.AnalysisSHA256)
}

func BuildOptionsOrigin(root string, certifiedAnalysisPath string, options Options) (OptionsOrigin, error) {
	root = filepath.Clean(root)
	_, cert, err := analysisruntime.LoadCertified(root, certifiedAnalysisPath)
	if err != nil {
		return OptionsOrigin{}, fmt.Errorf("REVIEW_OPTIONS_ANALYSIS_NOT_CERTIFIED: %w", err)
	}
	intent, err := normalizeReviewIntent153(options.Intent)
	if err != nil {
		return OptionsOrigin{}, err
	}
	if options.RunID != cert.RunID || options.ChangeSetSHA256 != cert.ChangeSetSHA256 {
		return OptionsOrigin{}, fmt.Errorf("REVIEW_OPTIONS_IDENTITY_INVALID")
	}
	origin := OptionsOrigin{
		RunID:           cert.RunID,
		ChangeSetSHA256: cert.ChangeSetSHA256,
		AnalysisSHA256:  cert.AnalysisSHA256,
		Intent:          intent,
	}
	if err := sealReviewIntentAuthority153(root, origin); err != nil {
		return OptionsOrigin{}, err
	}
	return origin, nil
}

func matchingCallChains153(target string, all []analysisruntime.CallChain) []reviewscope.CallChain {
	out := make([]reviewscope.CallChain, 0)
	for _, candidate := range all {
		matched := false
		for _, symbol := range candidate.Chain {
			if exactTargetMatch153(symbol, target) {
				matched = true
				break
			}
		}
		if !matched && exactTargetMatch153(candidate.EntryPoint, target) {
			matched = true
		}
		if matched {
			out = append(out, reviewscope.CallChain(candidate))
		}
	}
	return out
}

func exactTargetMatch153(symbol, target string) bool {
	symbol = strings.TrimSpace(symbol)
	target = strings.TrimSpace(target)
	if symbol == target {
		return true
	}
	if strings.Contains(target, ".") {
		return false
	}
	if i := strings.LastIndex(symbol, "."); i > 0 {
		return symbol[:i] == target
	}
	return false
}

func normalizeReviewIntent153(in analysisruntime.Intent) (analysisruntime.Intent, error) {
	mode := strings.ToUpper(strings.TrimSpace(in.Mode))
	target := strings.TrimSpace(in.Target)
	if mode == "" {
		if target == "" {
			mode = "FULL"
		} else {
			mode = "TARGETED"
		}
	}
	switch mode {
	case "FULL":
		if target != "" {
			return analysisruntime.Intent{}, fmt.Errorf("REVIEW_OPTIONS_INTENT_INVALID: FULL must not contain target")
		}
	case "TARGETED":
		if target == "" {
			return analysisruntime.Intent{}, fmt.Errorf("REVIEW_OPTIONS_INTENT_INVALID: TARGETED requires target")
		}
	default:
		return analysisruntime.Intent{}, fmt.Errorf("REVIEW_OPTIONS_INTENT_INVALID: unsupported mode %q", mode)
	}
	return analysisruntime.Intent{Mode: mode, Target: target}, nil
}

func finalizeOptions153(in Options, analysisHash string) (Options, error) {
	if strings.ToUpper(strings.TrimSpace(in.EntrypointCompleteness)) != "COMPLETE" {
		return Options{}, fmt.Errorf("ENTRYPOINT_COMPLETENESS_INCOMPLETE")
	}
	if strings.TrimSpace(in.RunID) == "" || len(strings.TrimSpace(in.ChangeSetSHA256)) != 64 || len(strings.TrimSpace(analysisHash)) != 64 {
		return Options{}, fmt.Errorf("REVIEW_OPTIONS_IDENTITY_INVALID")
	}
	intent, err := normalizeReviewIntent153(in.Intent)
	if err != nil {
		return Options{}, err
	}
	out := in
	out.EntrypointCompleteness = "COMPLETE"
	out.Intent = intent
	out.AutoSelectionIDs = nil
	out.OptionsHash = ""
	out.Chains = make([]ChainOption, len(in.Chains))
	copy(out.Chains, in.Chains)
	for i := range out.Chains {
		out.Chains[i].SelectionID = ""
		out.Chains[i].EntryPoints = uniqueSorted153(out.Chains[i].EntryPoints)
	}
	sort.Slice(out.Chains, func(i, j int) bool {
		left, right := callChainKey153(out.Chains[i].CallChain), callChainKey153(out.Chains[j].CallChain)
		if left != right {
			return left < right
		}
		if out.Chains[i].ChainID != out.Chains[j].ChainID {
			return out.Chains[i].ChainID < out.Chains[j].ChainID
		}
		return out.Chains[i].Source < out.Chains[j].Source
	})
	// Multiple Business Chains may certify the same semantic call chain. Keep
	// one deterministic provenance representative, never inflate the menu.
	unique := make([]ChainOption, 0, len(out.Chains))
	seen := map[string]bool{}
	for _, option := range out.Chains {
		key := callChainKey153(option.CallChain)
		if !seen[key] {
			unique = append(unique, option)
			seen[key] = true
		}
	}
	out.Chains = unique
	for i := range out.Chains {
		out.Chains[i].SelectionID = fmt.Sprintf("C%d", i+1)
	}
	switch len(out.Chains) {
	case 0:
		out.Decision = DecisionAutoFull
	case 1:
		out.Decision = DecisionAutoSingle
		out.AutoSelectionIDs = []string{"C1"}
	default:
		out.Decision = DecisionUser
	}
	identity := struct {
		AnalysisHash           string                 `json:"analysisHash"`
		RunID                  string                 `json:"runId"`
		ChangeSetSHA256        string                 `json:"changeSetSha256"`
		EntrypointCompleteness string                 `json:"entrypointCompleteness"`
		Intent                 analysisruntime.Intent `json:"intent"`
		Decision               Decision               `json:"decision"`
		AutoSelectionIDs       []string               `json:"autoSelectionIds,omitempty"`
		Chains                 []ChainOption          `json:"chains"`
	}{analysisHash, out.RunID, out.ChangeSetSHA256, out.EntrypointCompleteness, out.Intent, out.Decision, out.AutoSelectionIDs, out.Chains}
	b, err := json.Marshal(identity)
	if err != nil {
		return Options{}, err
	}
	sum := sha256.Sum256(b)
	out.OptionsHash = fmt.Sprintf("%x", sum[:])
	return out, nil
}

func uniqueSorted153(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

// Business Chain files group entries over a shared node sequence; selection
// authority is the exact certified call-chain payload, including optional refs.
func candidateContainsCallChain153(candidate chain.Chain, payload reviewscope.CallChain) bool {
	nodes := make([]string, 0, len(candidate.Nodes))
	for _, node := range candidate.Nodes {
		nodes = append(nodes, strings.TrimSpace(node.Symbol))
	}
	normalized := make([]string, len(payload.Chain))
	for i, symbol := range payload.Chain {
		normalized[i] = strings.TrimSpace(symbol)
	}
	entryPoint := strings.TrimSpace(payload.EntryPoint)
	for _, entry := range candidate.EntryPoints {
		if strings.TrimSpace(entry.Symbol) == entryPoint && reflect.DeepEqual(append([]string{entryPoint}, nodes...), normalized) {
			return true
		}
	}
	return false
}

func callChainKey153(payload reviewscope.CallChain) string {
	data, _ := json.Marshal(payload)
	return string(data)
}
