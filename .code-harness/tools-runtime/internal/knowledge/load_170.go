package knowledge

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	maxKnowledgeDocuments170  = 20
	maxKnowledgeBytes170      = 256 * 1024
	maxExperienceDocuments170 = 5
)

type cachedDocument170 struct {
	content string
	sha256  string
	size    int
}

func Load170(in LoadInput170) (LoadResult170, error) {
	result := LoadResult170{Manifest: Manifest170{
		RunID: in.RunID, ProjectID: in.Binding.ProjectID, Status: "READY", BindingSHA256: in.BindingSHA256,
		Sources: []SourceRecord170{}, Checks: []BusinessCheck170{}, Issues: []string{},
	}, Documents: []Document170{}}
	if !idPattern170.MatchString(in.RunID) {
		return result, fmt.Errorf("KNOWLEDGE_RUN_ID_INVALID: %q", in.RunID)
	}
	if err := validateBinding170(in.Binding); err != nil {
		return result, err
	}
	if len(in.BindingSHA256) != 64 {
		return result, fmt.Errorf("KNOWLEDGE_BINDING_INVALID: binding digest must be SHA-256")
	}
	if _, err := hex.DecodeString(in.BindingSHA256); err != nil {
		return result, fmt.Errorf("KNOWLEDGE_BINDING_INVALID: binding digest must be SHA-256")
	}
	repoRoot, err := filepath.Abs(in.RepoRoot)
	if err != nil {
		return result, fmt.Errorf("KNOWLEDGE_REPO_ROOT_INVALID: %w", err)
	}

	type indexedSource struct {
		index  int
		source Source170
	}
	ordered := make([]indexedSource, len(in.Binding.Sources))
	for i, source := range in.Binding.Sources {
		ordered[i] = indexedSource{index: i, source: source}
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		pi, pj := sourcePriority170(ordered[i].source), sourcePriority170(ordered[j].source)
		if pi != pj {
			return pi < pj
		}
		if pi < 2 {
			return ordered[i].source.ID < ordered[j].source.ID
		}
		return false
	})

	records := make([]SourceRecord170, len(in.Binding.Sources))
	cache := map[string]cachedDocument170{}
	uniqueDocs, uniqueBytes := 0, 0
	experiencePaths := map[string]bool{}
	ruleOwners := map[string][]int{}

	for _, item := range ordered {
		source := item.source
		record := SourceRecord170{SourceID: source.ID, Root: source.Root, Path: source.Path, Kind: source.Kind, Required: source.Required, Status: "READY", Reasons: []string{}}
		root, rootErr := sourceRoot170(repoRoot, in.Binding.TeamRoot, source.Root)
		if rootErr != nil {
			blockSource170(&result.Manifest, &record, source.Required, "KNOWLEDGE_SOURCE_ROOT_INVALID: "+rootErr.Error())
			records[item.index] = record
			if source.Kind == "RULES" {
				appendUnknownChecks170(&result.Manifest, in.Units, source.ID, record.Reasons)
			}
			continue
		}
		resolved, resolveErr := resolveSourcePath170(root, source.Path)
		if resolveErr != nil {
			blockSource170(&result.Manifest, &record, source.Required, resolveErr.Error())
			records[item.index] = record
			if source.Kind == "RULES" {
				appendUnknownChecks170(&result.Manifest, in.Units, source.ID, record.Reasons)
			}
			continue
		}

		if source.Kind == "EXPERIENCE" && !experiencePaths[resolved] && len(experiencePaths) >= maxExperienceDocuments170 {
			blockSource170(&result.Manifest, &record, false, "KNOWLEDGE_BUDGET_EXPERIENCE: maximum 5 experience documents")
			records[item.index] = record
			continue
		}

		doc, cached := cache[resolved]
		if !cached {
			if uniqueDocs >= maxKnowledgeDocuments170 {
				blockSource170(&result.Manifest, &record, source.Required, "KNOWLEDGE_BUDGET_DOCUMENTS: maximum 20 unique documents")
				records[item.index] = record
				if source.Kind == "RULES" {
					appendUnknownChecks170(&result.Manifest, in.Units, source.ID, record.Reasons)
				}
				continue
			}
			content, readErr := readStableUTF8File170(resolved)
			if readErr != nil {
				blockSource170(&result.Manifest, &record, source.Required, readErr.Error())
				records[item.index] = record
				if source.Kind == "RULES" {
					appendUnknownChecks170(&result.Manifest, in.Units, source.ID, record.Reasons)
				}
				continue
			}
			if uniqueBytes+len(content) > maxKnowledgeBytes170 {
				blockSource170(&result.Manifest, &record, source.Required, "KNOWLEDGE_BUDGET_BYTES: maximum 256 KiB unique document bytes")
				records[item.index] = record
				if source.Kind == "RULES" {
					appendUnknownChecks170(&result.Manifest, in.Units, source.ID, record.Reasons)
				}
				continue
			}
			sum := sha256.Sum256(content)
			doc = cachedDocument170{content: string(content), sha256: fmt.Sprintf("%x", sum[:]), size: len(content)}
			cache[resolved] = doc
			uniqueDocs++
			uniqueBytes += len(content)
		}
		if source.Kind == "EXPERIENCE" {
			experiencePaths[resolved] = true
		}
		record.SHA256 = doc.sha256

		if source.Kind == "RULES" {
			rule, _, parseErr := ParseRule170([]byte(doc.content))
			if parseErr != nil {
				blockSource170(&result.Manifest, &record, true, parseErr.Error())
				appendUnknownChecks170(&result.Manifest, in.Units, source.ID, record.Reasons)
				records[item.index] = record
				continue
			}
			record.Rule = &rule
			ruleOwners[rule.RuleID] = append(ruleOwners[rule.RuleID], item.index)
			if rule.Status != "ACTIVE" {
				record.Status = "NOT_APPLICABLE"
			} else if rule.ProjectID != "*" && rule.ProjectID != in.Binding.ProjectID {
				blockSource170(&result.Manifest, &record, true, "KNOWLEDGE_PROJECT_MISMATCH: rule projectId does not match binding projectId")
				appendRuleChecks170(&result.Manifest, in.Units, source.ID, rule, "BLOCKED", record.Reasons)
			} else {
				applicable := appendRuleChecks170(&result.Manifest, in.Units, source.ID, rule, "READY", nil)
				if !applicable {
					record.Status = "NOT_APPLICABLE"
				}
			}
		}
		records[item.index] = record
		result.Documents = append(result.Documents, Document170{SourceID: source.ID, Content: doc.content})
	}

	for ruleID, indexes := range ruleOwners {
		if len(indexes) < 2 {
			continue
		}
		result.Manifest.Status = "PARTIAL"
		result.Manifest.Issues = append(result.Manifest.Issues, "DUPLICATE_RULE_ID: "+ruleID)
		for _, idx := range indexes {
			records[idx].Status = "BLOCKED"
			records[idx].Reasons = uniqueStrings170(append(records[idx].Reasons, "DUPLICATE_RULE_ID: "+ruleID))
		}
		for i := range result.Manifest.Checks {
			if result.Manifest.Checks[i].RuleID == ruleID {
				result.Manifest.Checks[i].Status = "BLOCKED"
				result.Manifest.Checks[i].Reasons = uniqueStrings170(append(result.Manifest.Checks[i].Reasons, "DUPLICATE_RULE_ID: "+ruleID))
			}
		}
	}

	result.Manifest.Sources = records
	if result.Manifest.Status == "READY" {
		for _, record := range records {
			if record.Required && record.Status == "BLOCKED" {
				result.Manifest.Status = "PARTIAL"
				break
			}
		}
	}
	for _, check := range result.Manifest.Checks {
		if check.Status == "BLOCKED" {
			result.Manifest.Status = "PARTIAL"
			break
		}
	}
	sortManifest170(&result.Manifest)
	return result, nil
}

func sourcePriority170(source Source170) int {
	if source.Kind == "RULES" {
		return 0
	}
	if source.Required {
		return 1
	}
	return 2
}

func sourceRoot170(repoRoot, teamRoot, root string) (string, error) {
	if root == "PROJECT" {
		return filepath.EvalSymlinks(repoRoot)
	}
	if root != "TEAM" {
		return "", fmt.Errorf("unknown root %q", root)
	}
	value := strings.TrimSpace(teamRoot)
	if value == "" {
		return "", fmt.Errorf("teamRoot is empty")
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(repoRoot, value)
	}
	abs, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

func resolveSourcePath170(root, sourcePath string) (string, error) {
	candidate := filepath.Join(root, filepath.FromSlash(sourcePath))
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("KNOWLEDGE_SOURCE_MISSING: %s", sourcePath)
		}
		return "", fmt.Errorf("KNOWLEDGE_SOURCE_RESOLVE_FAILED: %s: %w", sourcePath, err)
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return "", fmt.Errorf("KNOWLEDGE_SOURCE_OUTSIDE_ROOT: %s", sourcePath)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("KNOWLEDGE_SOURCE_OUTSIDE_ROOT: %s", sourcePath)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("KNOWLEDGE_SOURCE_STAT_FAILED: %s: %w", sourcePath, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("KNOWLEDGE_SOURCE_NOT_REGULAR: %s", sourcePath)
	}
	return resolved, nil
}

func readStableUTF8File170(path string) ([]byte, error) {
	before, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("KNOWLEDGE_SOURCE_READ_FAILED: %w", err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("KNOWLEDGE_SOURCE_READ_FAILED: %w", err)
	}
	if !utf8.Valid(first) {
		return nil, fmt.Errorf("KNOWLEDGE_SOURCE_UTF8_INVALID: %s", filepath.Base(path))
	}
	after, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("KNOWLEDGE_SOURCE_CHANGED: %w", err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("KNOWLEDGE_SOURCE_CHANGED: %w", err)
	}
	firstSum, secondSum := sha256.Sum256(first), sha256.Sum256(second)
	if before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) || firstSum != secondSum {
		return nil, fmt.Errorf("KNOWLEDGE_SOURCE_CHANGED: %s changed while reading", filepath.Base(path))
	}
	return first, nil
}

func blockSource170(manifest *Manifest170, record *SourceRecord170, required bool, reason string) {
	record.Status = "BLOCKED"
	record.Reasons = uniqueStrings170(append(record.Reasons, reason))
	manifest.Issues = uniqueStrings170(append(manifest.Issues, reason))
	if required {
		manifest.Status = "PARTIAL"
	}
}

func appendUnknownChecks170(manifest *Manifest170, units []Unit170, sourceID string, reasons []string) {
	if len(reasons) == 0 {
		reasons = []string{"KNOWLEDGE_SOURCE_UNAVAILABLE"}
	}
	for _, unit := range units {
		manifest.Checks = append(manifest.Checks, BusinessCheck170{ReviewUnitID: unit.ID, RuleKey: "BUSINESS:" + sourceID + ":UNKNOWN", SourceID: sourceID, RuleID: "UNKNOWN", Status: "BLOCKED", Reasons: append([]string(nil), reasons...)})
	}
}

func appendRuleChecks170(manifest *Manifest170, units []Unit170, sourceID string, rule Rule170, status string, reasons []string) bool {
	matched := false
	for _, unit := range units {
		if !Applies170(rule, unit) {
			continue
		}
		matched = true
		manifest.Checks = append(manifest.Checks, BusinessCheck170{ReviewUnitID: unit.ID, RuleKey: "BUSINESS:" + sourceID + ":" + rule.RuleID, SourceID: sourceID, RuleID: rule.RuleID, Status: status, Reasons: append([]string(nil), reasons...)})
	}
	return matched
}

func sortManifest170(manifest *Manifest170) {
	sort.Slice(manifest.Sources, func(i, j int) bool { return manifest.Sources[i].SourceID < manifest.Sources[j].SourceID })
	sort.Slice(manifest.Checks, func(i, j int) bool {
		if manifest.Checks[i].ReviewUnitID != manifest.Checks[j].ReviewUnitID {
			return manifest.Checks[i].ReviewUnitID < manifest.Checks[j].ReviewUnitID
		}
		return manifest.Checks[i].RuleKey < manifest.Checks[j].RuleKey
	})
	manifest.Issues = uniqueStrings170(manifest.Issues)
	for i := range manifest.Sources {
		manifest.Sources[i].Reasons = uniqueStrings170(manifest.Sources[i].Reasons)
	}
	for i := range manifest.Checks {
		manifest.Checks[i].Reasons = uniqueStrings170(manifest.Checks[i].Reasons)
	}
}

func uniqueStrings170(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, value := range in {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}
