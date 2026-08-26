package chain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	analysisruntime "codea-harness-tools/internal/analysis"
)

type EditOperation struct {
	Type   string   `json:"type"`
	From   string   `json:"from,omitempty"`
	To     string   `json:"to,omitempty"`
	Symbol string   `json:"symbol,omitempty"`
	After  string   `json:"after,omitempty"`
	Name   string   `json:"name,omitempty"`
	Notes  string   `json:"notes,omitempty"`
	Order  []string `json:"order,omitempty"`
}

type EditRequest struct {
	RunID              string          `json:"runId"`
	ChainID            string          `json:"chainId"`
	ChangeAnalysisPath string          `json:"changeAnalysisPath"`
	Operations         []EditOperation `json:"operations"`
}

type EditResult struct {
	Status        string   `json:"status"`
	CandidatePath string   `json:"candidatePath"`
	Added         []string `json:"added"`
	Removed       []string `json:"removed"`
	Changed       []string `json:"changed"`
}

func ApplyVerifiedEdit(root string, req EditRequest) (EditResult, error) {
	root = filepath.Clean(root)
	if !runIDPattern.MatchString(strings.TrimSpace(req.RunID)) {
		return EditResult{}, fmt.Errorf("CHAIN_EDIT_REQUEST_INVALID: invalid runId")
	}
	if err := ValidateID(strings.TrimSpace(req.ChainID)); err != nil {
		return EditResult{}, fmt.Errorf("CHAIN_EDIT_REQUEST_INVALID: %w", err)
	}
	if len(req.Operations) == 0 {
		return EditResult{}, fmt.Errorf("CHAIN_EDIT_REQUEST_INVALID: operations must not be empty")
	}
	analysis, cert, err := analysisruntime.LoadCertified(root, req.ChangeAnalysisPath)
	if err != nil {
		return EditResult{}, fmt.Errorf("CHAIN_EDIT_ANALYSIS_NOT_CERTIFIED: %w", err)
	}
	if cert.RunID != req.RunID {
		return EditResult{}, fmt.Errorf("CHAIN_EDIT_ANALYSIS_IDENTITY_MISMATCH")
	}
	analysisBytes, err := json.Marshal(analysis)
	if err != nil {
		return EditResult{}, fmt.Errorf("CHAIN_EDIT_ANALYSIS_ENCODE_FAILED: %w", err)
	}
	var evidence ChangeAnalysisEvidence
	if err := json.Unmarshal(analysisBytes, &evidence); err != nil {
		return EditResult{}, fmt.Errorf("CHAIN_EDIT_ANALYSIS_DECODE_FAILED: %w", err)
	}

	chainPath, err := ChainPath(root, req.ChainID)
	if err != nil {
		return EditResult{}, err
	}
	existing, err := Load(chainPath)
	if err != nil {
		if os.IsNotExist(err) {
			return EditResult{}, fmt.Errorf("CHAIN_NOT_FOUND: %s", req.ChainID)
		}
		return EditResult{}, fmt.Errorf("CHAIN_EDIT_LOAD_FAILED: %w", err)
	}
	if !chainMaintenanceIntentAuthorizes153(cert.Intent, existing) {
		return EditResult{}, fmt.Errorf("CHAIN_EDIT_ANALYSIS_INTENT_MISMATCH")
	}
	candidate, err := applyEditOperations153(existing, req.Operations, evidence)
	if err != nil {
		return EditResult{}, err
	}
	if candidate.Version != existing.Version || candidate.ID != existing.ID || candidate.Status != existing.Status || !reflect.DeepEqual(candidate.EntryPoints, existing.EntryPoints) {
		return EditResult{}, fmt.Errorf("CHAIN_EDIT_IDENTITY_CHANGED")
	}
	if err := verifyEditedChainFacts153(root, candidate, evidence); err != nil {
		return EditResult{}, err
	}

	added, removed, changed := editDiff153(existing, candidate)
	candidatePath := filepath.ToSlash(filepath.Join(".code-harness", "runs", req.RunID, "analysis", "chain-edit-candidates", req.ChainID+".yaml"))
	candidateBytes, err := MarshalYAML(candidate)
	if err != nil {
		return EditResult{}, fmt.Errorf("CHAIN_EDIT_CANDIDATE_ENCODE_FAILED: %w", err)
	}
	candidateAbs := filepath.Join(root, filepath.FromSlash(candidatePath))
	if err := os.MkdirAll(filepath.Dir(candidateAbs), 0o755); err != nil {
		return EditResult{}, fmt.Errorf("CHAIN_EDIT_CANDIDATE_DIR_FAILED: %w", err)
	}
	if err := atomicReplace(candidateAbs, candidateBytes); err != nil {
		return EditResult{}, fmt.Errorf("CHAIN_EDIT_CANDIDATE_WRITE_FAILED: %w", err)
	}
	if _, err := CertifyCandidate(root, candidate, candidatePath, "EDIT", cert); err != nil {
		_ = os.Remove(candidateAbs)
		_ = os.Remove(candidateCertPath153(root, candidatePath))
		return EditResult{}, fmt.Errorf("CHAIN_EDIT_CANDIDATE_CERT_FAILED: %w", err)
	}
	return EditResult{
		Status:        "EDIT_READY",
		CandidatePath: candidatePath,
		Added:         added,
		Removed:       removed,
		Changed:       changed,
	}, nil
}

func chainMaintenanceIntentAuthorizes153(intent *analysisruntime.Intent, existing Chain) bool {
	if intent == nil || strings.ToUpper(strings.TrimSpace(intent.Mode)) != "CHAIN_MAINTENANCE" {
		return false
	}
	target := strings.TrimSpace(intent.Target)
	if target == "" { return false }
	for _, ep := range existing.EntryPoints {
		symbol := strings.TrimSpace(ep.Symbol)
		if target == symbol { return true }
		if i := strings.LastIndex(symbol, "."); i > 0 && target == symbol[:i] { return true }
	}
	return false
}

func applyEditOperations153(existing Chain, operations []EditOperation, evidence ChangeAnalysisEvidence) (Chain, error) {
	candidate := cloneChain153(existing)
	for i, raw := range operations {
		op := raw
		op.Type = strings.ToUpper(strings.TrimSpace(op.Type))
		switch op.Type {
		case "REPLACE_NODE":
			from, to := strings.TrimSpace(op.From), strings.TrimSpace(op.To)
			if from == "" || to == "" {
				return Chain{}, fmt.Errorf("CHAIN_EDIT_REQUEST_INVALID: operations[%d] REPLACE_NODE requires from/to", i)
			}
			index, err := exactNodeIndex153(candidate.Nodes, from)
			if err != nil {
				return Chain{}, fmt.Errorf("CHAIN_EDIT_FACT_NOT_VERIFIED: %w", err)
			}
			node, err := resolveCurrentEditNode153(to, evidence)
			if err != nil {
				return Chain{}, err
			}
			candidate.Nodes[index] = node
		case "ADD_NODE":
			symbol, after := strings.TrimSpace(op.Symbol), strings.TrimSpace(op.After)
			if symbol == "" || after == "" {
				return Chain{}, fmt.Errorf("CHAIN_EDIT_REQUEST_INVALID: operations[%d] ADD_NODE requires symbol/after", i)
			}
			if containsNodeSymbol153(candidate.Nodes, symbol) {
				return Chain{}, fmt.Errorf("CHAIN_EDIT_REQUEST_INVALID: operations[%d] ADD_NODE duplicates %s", i, symbol)
			}
			afterIndex, err := exactNodeIndex153(candidate.Nodes, after)
			if err != nil {
				return Chain{}, fmt.Errorf("CHAIN_EDIT_REQUEST_INVALID: operations[%d] ADD_NODE after must name one existing node", i)
			}
			node, err := resolveCurrentEditNode153(symbol, evidence)
			if err != nil {
				return Chain{}, err
			}
			candidate.Nodes = append(candidate.Nodes, Node{})
			copy(candidate.Nodes[afterIndex+2:], candidate.Nodes[afterIndex+1:])
			candidate.Nodes[afterIndex+1] = node
		case "REMOVE_NODE":
			symbol := strings.TrimSpace(op.Symbol)
			if symbol == "" {
				return Chain{}, fmt.Errorf("CHAIN_EDIT_REQUEST_INVALID: operations[%d] REMOVE_NODE requires symbol", i)
			}
			index, err := exactNodeIndex153(candidate.Nodes, symbol)
			if err != nil {
				return Chain{}, fmt.Errorf("CHAIN_EDIT_FACT_NOT_VERIFIED: %w", err)
			}
			candidate.Nodes = append(candidate.Nodes[:index:index], candidate.Nodes[index+1:]...)
		case "REORDER_NODE":
			if len(op.Order) != len(candidate.Nodes) {
				return Chain{}, fmt.Errorf("CHAIN_EDIT_REQUEST_INVALID: operations[%d] REORDER_NODE must contain every current node exactly once", i)
			}
			bySymbol := map[string]Node{}
			for _, node := range candidate.Nodes {
				symbol := strings.TrimSpace(node.Symbol)
				if symbol == "" || bySymbol[symbol].Symbol != "" {
					return Chain{}, fmt.Errorf("CHAIN_EDIT_REQUEST_INVALID: operations[%d] REORDER_NODE is ambiguous", i)
				}
				bySymbol[symbol] = node
			}
			reordered := make([]Node, 0, len(op.Order))
			seen := map[string]bool{}
			for _, rawSymbol := range op.Order {
				symbol := strings.TrimSpace(rawSymbol)
				node, ok := bySymbol[symbol]
				if !ok || seen[symbol] {
					return Chain{}, fmt.Errorf("CHAIN_EDIT_REQUEST_INVALID: operations[%d] REORDER_NODE must contain every current node exactly once", i)
				}
				seen[symbol] = true
				reordered = append(reordered, node)
			}
			candidate.Nodes = reordered
		case "RENAME_CHAIN":
			name := strings.TrimSpace(op.Name)
			if name == "" {
				return Chain{}, fmt.Errorf("CHAIN_EDIT_REQUEST_INVALID: operations[%d] RENAME_CHAIN requires non-empty name", i)
			}
			candidate.Name = name
		case "UPDATE_NOTES":
			candidate.Notes = op.Notes
		default:
			return Chain{}, fmt.Errorf("CHAIN_EDIT_REQUEST_INVALID: operations[%d] unsupported type %q", i, op.Type)
		}
	}
	return candidate, nil
}

func verifyEditedChainFacts153(root string, candidate Chain, evidence ChangeAnalysisEvidence) error {
	result := Validate(root, candidate, EvidenceSnapshot(evidence))
	if result.Status != ValidationValid {
		details := append([]string(nil), result.Errors...)
		if len(details) == 0 {
			details = append(details, result.Warnings...)
		}
		if len(details) == 0 {
			details = []string{result.Status}
		}
		return fmt.Errorf("CHAIN_EDIT_FACT_NOT_VERIFIED: %s", strings.Join(details, "; "))
	}
	return nil
}

func resolveCurrentEditNode153(symbol string, evidence ChangeAnalysisEvidence) (Node, error) {
	symbol = strings.TrimSpace(symbol)
	var matches []SymbolLocationEvidence
	for _, location := range evidence.SymbolLocations {
		if locationWorkspace(location) != CurrentWorkspace || strings.TrimSpace(location.Symbol) != symbol || !isProductionJavaPath(location.Path) {
			continue
		}
		matches = append(matches, location)
	}
	matches = uniqueLocations(matches)
	if len(matches) != 1 {
		return Node{}, fmt.Errorf("CHAIN_EDIT_FACT_NOT_VERIFIED: current-workspace symbol %s has %d exact verified locations", symbol, len(matches))
	}
	return Node{
		Workspace: CurrentWorkspace,
		Symbol:    symbol,
		Path:      normalizeRepoPath(matches[0].Path),
		Role:      nodeRole(matches[0].Role),
	}, nil
}

func exactNodeIndex153(nodes []Node, symbol string) (int, error) {
	found := -1
	for i := range nodes {
		if strings.TrimSpace(nodes[i].Symbol) != strings.TrimSpace(symbol) {
			continue
		}
		if found >= 0 {
			return -1, fmt.Errorf("node %s is ambiguous", symbol)
		}
		found = i
	}
	if found < 0 {
		return -1, fmt.Errorf("node %s not found", symbol)
	}
	return found, nil
}

func containsNodeSymbol153(nodes []Node, symbol string) bool {
	_, err := exactNodeIndex153(nodes, symbol)
	return err == nil
}

func cloneChain153(in Chain) Chain {
	out := in
	out.EntryPoints = append([]EntryPoint(nil), in.EntryPoints...)
	out.Nodes = append([]Node(nil), in.Nodes...)
	out.Resources = append([]Resource(nil), in.Resources...)
	out.Boundaries = append([]Boundary(nil), in.Boundaries...)
	return out
}

func editDiff153(before, after Chain) (added, removed, changed []string) {
	beforeNodes := map[string]bool{}
	afterNodes := map[string]bool{}
	for _, node := range before.Nodes {
		beforeNodes[node.Symbol] = true
	}
	for _, node := range after.Nodes {
		afterNodes[node.Symbol] = true
	}
	for symbol := range afterNodes {
		if !beforeNodes[symbol] {
			added = append(added, "node:"+symbol)
		}
	}
	for symbol := range beforeNodes {
		if !afterNodes[symbol] {
			removed = append(removed, "node:"+symbol)
		}
	}
	if before.Name != after.Name {
		changed = append(changed, fmt.Sprintf("name:%q->%q", before.Name, after.Name))
	}
	if before.Notes != after.Notes {
		changed = append(changed, "notes")
	}
	beforeOrder, afterOrder := make([]string, len(before.Nodes)), make([]string, len(after.Nodes))
	for i := range before.Nodes {
		beforeOrder[i] = before.Nodes[i].Symbol
	}
	for i := range after.Nodes {
		afterOrder[i] = after.Nodes[i].Symbol
	}
	if !reflect.DeepEqual(beforeOrder, afterOrder) {
		changed = append(changed, "node-order:"+strings.Join(beforeOrder, " -> ")+"=>"+strings.Join(afterOrder, " -> "))
	}
	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(changed)
	return added, removed, changed
}
