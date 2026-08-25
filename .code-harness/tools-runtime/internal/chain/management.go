package chain

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ListItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      Status `json:"status"`
	StatusLabel string `json:"statusLabel"`
}

type RefreshResult struct {
	Candidate    Chain    `json:"candidate"`
	ExistingHash string   `json:"existingHash"`
	Changed      bool     `json:"changed"`
	Added        []string `json:"added"`
	Removed      []string `json:"removed"`
	Errors       []string `json:"errors"`
}

func FileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:]), nil
}

func SaveAccepted(root string, c Chain, expectedExistingHash string) error {
	if c.Status != StatusAccepted {
		return fmt.Errorf("only ACCEPTED chains may be persisted to Project State")
	}
	if err := validateModel(c); err != nil {
		return err
	}
	if errs := validateProjectIdentity(root, c.ID); len(errs) != 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	path, err := ChainPath(root, c.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create chain Project State directory: %w", err)
	}
	_, statErr := os.Stat(path)
	exists := statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		return fmt.Errorf("check existing chain: %w", statErr)
	}
	if exists {
		if strings.TrimSpace(expectedExistingHash) == "" {
			return fmt.Errorf("CHAIN_OVERWRITE_REQUIRES_EXPECTED_HASH: %s", c.ID)
		}
		actual, err := FileHash(path)
		if err != nil {
			return fmt.Errorf("hash existing chain: %w", err)
		}
		if actual != strings.TrimSpace(expectedExistingHash) {
			return fmt.Errorf("CHAIN_EXPECTED_HASH_MISMATCH: %s", c.ID)
		}
	} else if strings.TrimSpace(expectedExistingHash) != "" {
		return fmt.Errorf("CHAIN_EXPECTED_HASH_WITHOUT_EXISTING_FILE: %s", c.ID)
	}

	data, err := MarshalYAML(c)
	if err != nil {
		return err
	}
	return atomicReplace(path, data)
}

func List(root string) ([]ListItem, error) {
	dir := filepath.Join(filepath.Clean(root), ".code-harness", "chains")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []ListItem{}, nil
	}
	if err != nil {
		return nil, err
	}
	var out []ListItem
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".yaml") {
			continue
		}
		c, err := Load(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("load project chain %s: %w", entry.Name(), err)
		}
		base := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		if base != c.ID {
			return nil, fmt.Errorf("CHAIN_ID_FILENAME_MISMATCH: file %q contains id %q", entry.Name(), c.ID)
		}
		if seen[c.ID] {
			return nil, fmt.Errorf("DUPLICATE_PROJECT_CHAIN_ID: %s", c.ID)
		}
		seen[c.ID] = true
		out = append(out, ListItem{ID: c.ID, Name: c.Name, Status: c.Status, StatusLabel: chineseStatusLabel(c.Status)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func RenderChinese(c Chain) string {
	var b strings.Builder
	b.WriteString(c.Name)
	b.WriteString("\n\n状态：")
	b.WriteString(chineseStatusLabel(c.Status))
	b.WriteString("\n\n入口：\n")
	for i, entry := range c.EntryPoints {
		fmt.Fprintf(&b, "%d. %s\n", i+1, entry.Symbol)
	}
	b.WriteString("\n核心链路：\n")
	if len(c.EntryPoints) != 0 {
		b.WriteString("🌐 ")
		b.WriteString(c.EntryPoints[0].Symbol)
		b.WriteByte('\n')
	}
	for _, node := range c.Nodes {
		b.WriteString("   ↓\n")
		b.WriteString(nodeRoleIcon(node.Role))
		b.WriteByte(' ')
		b.WriteString(node.Symbol)
		b.WriteByte('\n')
	}
	for _, resource := range c.Resources {
		b.WriteString("   ↓\n📄 ")
		b.WriteString(normalizeRepoPath(resource.Path))
		if strings.TrimSpace(resource.Symbol) != "" {
			b.WriteString("#")
			b.WriteString(resource.Symbol)
		}
		b.WriteByte('\n')
	}
	for _, boundary := range c.Boundaries {
		b.WriteString("   ↓\n🔗 ")
		b.WriteString(boundary.Symbol)
		b.WriteByte('\n')
	}
	if strings.TrimSpace(c.Notes) != "" {
		b.WriteString("\n说明：\n")
		b.WriteString(strings.TrimSpace(c.Notes))
		b.WriteByte('\n')
	}
	return b.String()
}

func Refresh(root string, existing Chain, discovered Chain) RefreshResult {
	result := RefreshResult{Added: []string{}, Removed: []string{}, Errors: []string{}}
	path, err := ChainPath(root, existing.ID)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result
	}
	hash, err := FileHash(path)
	if err != nil {
		result.Errors = append(result.Errors, "read existing Project State: "+err.Error())
		return result
	}
	result.ExistingHash = hash
	candidate := discovered
	candidate.ID = existing.ID
	candidate.Name = existing.Name
	candidate.Notes = existing.Notes
	candidate.Status = StatusAccepted
	candidate.Version = existing.Version
	result.Candidate = candidate

	oldFacts := chainFactSet(existing)
	newFacts := chainFactSet(candidate)
	for fact := range newFacts {
		if !oldFacts[fact] {
			result.Added = append(result.Added, fact)
		}
	}
	for fact := range oldFacts {
		if !newFacts[fact] {
			result.Removed = append(result.Removed, fact)
		}
	}
	sort.Strings(result.Added)
	sort.Strings(result.Removed)
	result.Changed = len(result.Added) != 0 || len(result.Removed) != 0
	return result
}

func chainFactSet(c Chain) map[string]bool {
	out := map[string]bool{}
	for _, entry := range c.EntryPoints {
		out["entry:"+workspaceFactPrefix(entry.Workspace)+entry.Symbol+"@"+normalizeRepoPath(entry.Path)] = true
	}
	for _, node := range c.Nodes {
		out["node:"+workspaceFactPrefix(node.Workspace)+node.Role+":"+node.Symbol+"@"+normalizeRepoPath(node.Path)] = true
	}
	for _, resource := range c.Resources {
		out["resource:"+resource.Role+":"+resource.Symbol+"@"+normalizeRepoPath(resource.Path)] = true
	}
	for _, boundary := range c.Boundaries {
		out["boundary:"+boundary.Role+":"+boundary.Symbol+"@"+normalizeRepoPath(boundary.Path)] = true
	}
	return out
}

func workspaceFactPrefix(workspace string) string {
	workspace = effectiveWorkspace(strings.TrimSpace(workspace))
	if workspace == CurrentWorkspace {
		return ""
	}
	return "[" + workspace + "]:"
}

func chineseStatusLabel(status Status) string {
	switch status {
	case StatusAccepted:
		return "✅ 已确认"
	case StatusStale:
		return "⚠️ 已过期"
	case StatusDiscovered:
		return "🔎 临时发现"
	default:
		return "未知状态"
	}
}

func nodeRoleIcon(role string) string {
	switch role {
	case "SERVICE":
		return "⚙️"
	case "REPOSITORY", "MAPPER":
		return "🗄"
	default:
		return "🔹"
	}
}
