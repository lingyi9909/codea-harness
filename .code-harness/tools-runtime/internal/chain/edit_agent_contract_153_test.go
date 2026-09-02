package chain

// Task 5 contract assertions intentionally verify semantic authority, not Markdown formatting.
// Final acceptance also locks the normative chain-edit-candidates artifact directory.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func task153RepoRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Clean(filepath.Join("..", "..", "..", ".."))
	if _, err := os.Stat(filepath.Join(root, ".code-harness", "AGENTS.md")); err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

func task153ReadContractFile(t *testing.T, root string, parts ...string) string {
	t.Helper()
	path := filepath.Join(append([]string{root}, parts...)...)
	data, err := os.ReadFile(path)
	if err != nil { t.Fatalf("read %s: %v", filepath.ToSlash(path), err) }
	return string(data)
}

func requireTask153Markers(t *testing.T, body string, markers ...string) {
	t.Helper()
	for _, marker := range markers {
		if !strings.Contains(body, marker) {
			t.Fatalf("missing Task 5 contract marker %q", marker)
		}
	}
}

func Test153ChainEditAgentContract(t *testing.T) {
	root := task153RepoRoot(t)
	skill := task153ReadContractFile(t, root, ".code-harness", "skills", "edit-chain", "SKILL.md")
	requireTask153Markers(t, skill,
		"harness chain edit <id|Controller|Controller.method>",
		"REPLACE_NODE", "ADD_NODE", "REMOVE_NODE", "REORDER_NODE", "RENAME_CHAIN", "UPDATE_NOTES",
		"chain-edit-candidates", "CHAIN_MAINTENANCE", "chain seal-persist", "chain persist", "planId",
		"requests/**", "不得直接", ".code-harness/chains",
	)

	orchestrator := task153ReadContractFile(t, root, ".code-harness", "agents", "orchestrator.md")
	requireTask153Markers(t, orchestrator,
		"harness chain edit <id|Controller|Controller.method>",
		"edit-chain",
		"analysis/chain-edit-candidates/<id>.yaml",
		"chain seal-persist",
		"chain persist",
	)
	if strings.Contains(orchestrator, "不得新增 `chain accept/merge/split/edit/ignore`") {
		t.Fatal("Task 5 must remove the old Task 3 rule that forbids chain edit")
	}

	agents := task153ReadContractFile(t, root, ".code-harness", "AGENTS.md")
	requireTask153Markers(t, agents,
		"harness chain edit",
		"codea-dcep-tools.exe chain edit --input",
		"chain-edit-candidates",
		"chain seal-persist",
		"chain persist",
	)

	validateSkill := task153ReadContractFile(t, root, ".code-harness", "skills", "validate-chain", "SKILL.md")
	requireTask153Markers(t, validateSkill,
		"harness chain edit",
		"chain-edit-candidates",
		"EDIT",
		"chain seal-persist",
		"chain persist",
	)
}
