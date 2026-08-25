package analysis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"codea-harness-tools/internal/changeset"
	"codea-harness-tools/internal/nav"
)

const inventoryComplete153 = "COMPLETE"

type ControllerEndpoint struct {
	Controller          string
	Symbol              string
	Path                string
	ControllerStartLine int
	ControllerEndLine   int
	StartLine           int
	EndLine             int
}

type entrypointScanner153 interface {
	Current(context.Context, string) ([]ControllerEndpoint, error)
	Base(context.Context, changeset.Snapshot, string) ([]ControllerEndpoint, error)
}

type navigationEntrypointScanner153 struct {
	repoRoot    string
	astGrepPath string
}

type rootedRunner153 struct{ dir string }

func (r rootedRunner153) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = r.dir
	return cmd.Output()
}

func BuildEntrypointInventory(repoRoot, runID string, snapshot changeset.Snapshot, intent Intent) (EntrypointInventory, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil { return EntrypointInventory{}, fmt.Errorf("ENTRYPOINT_REPO_ROOT_INVALID: %w", err) }
	scanner := navigationEntrypointScanner153{
		repoRoot: absRoot,
		astGrepPath: filepath.Join(absRoot, ".code-harness", "bin", "ast-grep.exe"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return buildEntrypointInventoryWithScanner(ctx, runID, snapshot, intent, scanner)
}

func buildEntrypointInventoryWithScanner(ctx context.Context, runID string, snapshot changeset.Snapshot, intent Intent, scanner entrypointScanner153) (EntrypointInventory, error) {
	if strings.TrimSpace(runID) == "" { return EntrypointInventory{}, errors.New("ENTRYPOINT_INVENTORY_RUN_ID_REQUIRED") }
	mode := strings.ToUpper(strings.TrimSpace(intent.Mode))
	switch mode {
	case "FULL", "LIST", "TARGETED", "CHAIN_MAINTENANCE":
	default:
		return EntrypointInventory{}, fmt.Errorf("ENTRYPOINT_INVENTORY_INTENT_INVALID: %s", intent.Mode)
	}
	if (mode == "TARGETED" || mode == "CHAIN_MAINTENANCE") && strings.TrimSpace(intent.Target) == "" {
		return EntrypointInventory{}, errors.New("ENTRYPOINT_INVENTORY_TARGET_REQUIRED")
	}

	byKey := map[string]ExpectedEntrypoint{}
	for _, changed := range snapshot.Files {
		if !isProductionJava153(changed.Path) { continue }
		current, err := scanner.Current(ctx, changed.Path)
		if err != nil { return EntrypointInventory{}, fmt.Errorf("ENTRYPOINT_CURRENT_SCAN_FAILED: %s: %w", changed.Path, err) }

		status := strings.ToUpper(strings.TrimSpace(changed.Status))
		switch status {
		case "A":
			for _, ep := range current { addExpected153(byKey, ep, "") }
		case "D":
			base, err := scanner.Base(ctx, snapshot, changed.Path)
			if err != nil { return EntrypointInventory{}, fmt.Errorf("ENTRYPOINT_BASE_SCAN_FAILED: %s: %w", changed.Path, err) }
			for _, ep := range base { addExpected153(byKey, ep, DispositionRemoved) }
		default:
			base, err := scanner.Base(ctx, snapshot, changed.Path)
			if err != nil { return EntrypointInventory{}, fmt.Errorf("ENTRYPOINT_BASE_SCAN_FAILED: %s: %w", changed.Path, err) }
			collectModifiedEntrypoints153(byKey, changed, current, base)
		}
	}

	items := make([]ExpectedEntrypoint, 0, len(byKey))
	for _, ep := range byKey {
		if targetAllowsEntrypoint153(intent, ep.Symbol) { items = append(items, ep) }
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Symbol != items[j].Symbol { return items[i].Symbol < items[j].Symbol }
		if items[i].Path != items[j].Path { return items[i].Path < items[j].Path }
		return items[i].Disposition < items[j].Disposition
	})
	return EntrypointInventory{
		RunID: runID, Status: inventoryComplete153,
		ExpectedEntrypoints: items, ChangeSetSHA256: snapshot.SHA256,
	}, nil
}

func collectModifiedEntrypoints153(out map[string]ExpectedEntrypoint, changed changeset.File, current, base []ControllerEndpoint) {
	currentBySymbol := map[string]ControllerEndpoint{}
	for _, ep := range current { currentBySymbol[ep.Symbol] = ep }

	// New-side evidence covers additions/replacements inside endpoints that exist now.
	for _, ep := range current {
		if anyNewHunkIntersects153(changed.Hunks, ep.StartLine, ep.EndLine) {
			addExpected153(out, ep, "")
		}
	}

	// Old-side evidence is equally authoritative for pure deletions. If the symbol
	// still exists, the current endpoint remains the obligation; only a vanished
	// symbol is represented as REMOVED.
	for _, old := range base {
		if !anyOldHunkIntersects153(changed.Hunks, old.StartLine, old.EndLine) { continue }
		if now, stillPresent := currentBySymbol[old.Symbol]; stillPresent {
			addExpected153(out, now, "")
		} else {
			addExpected153(out, old, DispositionRemoved)
		}
	}

	// Class-level changes must be evaluated on both snapshots. A class-level
	// hunk is one that intersects the controller declaration/body but no endpoint
	// on that same side; either side makes all current endpoints obligations.
	for _, controller := range uniqueControllers153(current) {
		classLevelChanged := newControllerClassLevelChanged153(changed.Hunks, controller, current)
		if !classLevelChanged {
			for _, oldController := range uniqueControllers153(base) {
				if oldController.Controller != controller.Controller || filepath.ToSlash(oldController.Path) != filepath.ToSlash(controller.Path) { continue }
				if oldControllerClassLevelChanged153(changed.Hunks, oldController, base) {
					classLevelChanged = true
					break
				}
			}
		}
		if classLevelChanged {
			for _, ep := range current {
				if ep.Controller == controller.Controller { addExpected153(out, ep, "") }
			}
		}
	}

	// Preserve the prior removed-endpoint behavior for a missing symbol when an
	// old-side class-level change makes the old controller itself an obligation.
	for _, old := range base {
		if _, stillPresent := currentBySymbol[old.Symbol]; stillPresent { continue }
		if oldControllerClassLevelChanged153(changed.Hunks, old, base) {
			addExpected153(out, old, DispositionRemoved)
		}
	}
}

func uniqueControllers153(in []ControllerEndpoint) []ControllerEndpoint {
	seen := map[string]bool{}
	out := make([]ControllerEndpoint, 0, len(in))
	for _, ep := range in {
		key := ep.Path + "\x00" + ep.Controller
		if !seen[key] { seen[key] = true; out = append(out, ep) }
	}
	return out
}

func newControllerClassLevelChanged153(hunks []changeset.Hunk, controller ControllerEndpoint, current []ControllerEndpoint) bool {
	for _, h := range hunks {
		if h.NewLines <= 0 || !lineRangeIntersects153(h.NewStart, h.NewLines, controller.ControllerStartLine, controller.ControllerEndLine) { continue }
		if !hunkIntersectsAnyCurrentEndpoint153(h, current, controller.Controller) { return true }
	}
	return false
}

func hunkIntersectsAnyCurrentEndpoint153(h changeset.Hunk, endpoints []ControllerEndpoint, controller string) bool {
	for _, ep := range endpoints {
		if ep.Controller == controller && lineRangeIntersects153(h.NewStart, h.NewLines, ep.StartLine, ep.EndLine) { return true }
	}
	return false
}

func oldControllerClassLevelChanged153(hunks []changeset.Hunk, old ControllerEndpoint, base []ControllerEndpoint) bool {
	for _, h := range hunks {
		if h.OldLines <= 0 || !lineRangeIntersects153(h.OldStart, h.OldLines, old.ControllerStartLine, old.ControllerEndLine) { continue }
		insideEndpoint := false
		for _, ep := range base {
			if ep.Controller == old.Controller && lineRangeIntersects153(h.OldStart, h.OldLines, ep.StartLine, ep.EndLine) { insideEndpoint = true; break }
		}
		if !insideEndpoint { return true }
	}
	return false
}

func anyNewHunkIntersects153(hunks []changeset.Hunk, start, end int) bool {
	for _, h := range hunks { if lineRangeIntersects153(h.NewStart, h.NewLines, start, end) { return true } }
	return false
}
func anyOldHunkIntersects153(hunks []changeset.Hunk, start, end int) bool {
	for _, h := range hunks { if lineRangeIntersects153(h.OldStart, h.OldLines, start, end) { return true } }
	return false
}
func lineRangeIntersects153(start, lines, itemStart, itemEnd int) bool {
	if lines <= 0 || start <= 0 || itemStart <= 0 || itemEnd < itemStart { return false }
	end := start + lines - 1
	return start <= itemEnd && end >= itemStart
}

func addExpected153(out map[string]ExpectedEntrypoint, ep ControllerEndpoint, disposition EntrypointDisposition) {
	if strings.TrimSpace(ep.Symbol) == "" || strings.TrimSpace(ep.Path) == "" { return }
	key := ep.Path + "\x00" + ep.Symbol
	candidate := ExpectedEntrypoint{Symbol: ep.Symbol, Path: filepath.ToSlash(ep.Path), Disposition: disposition}
	if disposition == DispositionRemoved { candidate.Limitation = "SOURCE_REMOVED" }
	if existing, ok := out[key]; !ok || (existing.Disposition == DispositionRemoved && disposition == "") { out[key] = candidate }
}

func targetAllowsEntrypoint153(intent Intent, symbol string) bool {
	mode := strings.ToUpper(strings.TrimSpace(intent.Mode))
	if mode != "TARGETED" && mode != "CHAIN_MAINTENANCE" { return true }
	target := strings.TrimSpace(intent.Target)
	if target == "" { return false }
	if target == symbol { return true }
	if !strings.Contains(target, ".") {
		if i := strings.LastIndex(symbol, "."); i > 0 && symbol[:i] == target { return true }
	}
	// A downstream Service target has no changed-Controller obligation by name alone.
	// Later certified call-chain evidence establishes its relevant upstream scope.
	return false
}

func isProductionJava153(p string) bool {
	p = filepath.ToSlash(filepath.Clean(p))
	return strings.HasPrefix(p, "src/main/java/") && strings.HasSuffix(p, ".java")
}

func VerifyEntrypointDispositions(inventory EntrypointInventory, proposal ChangeAnalysis) error {
	confirmed := map[string]bool{}
	for _, chain := range proposal.CallChains {
		if strings.TrimSpace(chain.EntryPoint) != "" { confirmed[chain.EntryPoint] = true }
	}
	partial := map[string]string{}
	for _, unresolved := range proposal.ReviewCoverage.UnresolvedSymbols {
		if strings.TrimSpace(unresolved.Reason) == "" { continue }
		if strings.TrimSpace(unresolved.Symbol) != "" { partial[unresolved.Symbol] = unresolved.Reason }
		if strings.TrimSpace(unresolved.From) != "" { partial[unresolved.From] = unresolved.Reason }
	}
	var missing []string
	for _, expected := range inventory.ExpectedEntrypoints {
		if expected.Disposition == DispositionRemoved { continue }
		if confirmed[expected.Symbol] { continue }
		if partial[expected.Symbol] != "" { continue }
		missing = append(missing, expected.Symbol)
	}
	if len(missing) == 0 { return nil }
	sort.Strings(missing)
	return fmt.Errorf("ENTRYPOINT_COMPLETENESS_INCOMPLETE: %s", strings.Join(missing, ", "))
}

func (s navigationEntrypointScanner153) Current(ctx context.Context, p string) ([]ControllerEndpoint, error) {
	clean := filepath.FromSlash(p)
	if _, err := os.Stat(filepath.Join(s.repoRoot, clean)); err != nil {
		if os.IsNotExist(err) { return nil, nil }
		return nil, err
	}
	return s.scanAtRoot(ctx, s.repoRoot, filepath.ToSlash(p))
}

func (s navigationEntrypointScanner153) Base(ctx context.Context, snapshot changeset.Snapshot, p string) ([]ControllerEndpoint, error) {
	mergeBaseCmd := exec.CommandContext(ctx, "git", "merge-base", snapshot.BaseRef, "HEAD")
	mergeBaseCmd.Dir = s.repoRoot
	mergeBaseBytes, err := mergeBaseCmd.CombinedOutput()
	if err != nil { return nil, fmt.Errorf("git merge-base %s HEAD: %w: %s", snapshot.BaseRef, err, strings.TrimSpace(string(mergeBaseBytes))) }
	mergeBase := strings.TrimSpace(string(mergeBaseBytes))
	object := mergeBase + ":" + filepath.ToSlash(p)
	show := exec.CommandContext(ctx, "git", "show", object)
	show.Dir = s.repoRoot
	content, err := show.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) { return nil, nil }
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "codea-harness-entrypoint-base-")
	if err != nil { return nil, err }
	defer os.RemoveAll(tmp)
	tmpFile := filepath.Join(tmp, filepath.FromSlash(p))
	if err := os.MkdirAll(filepath.Dir(tmpFile), 0o755); err != nil { return nil, err }
	if err := os.WriteFile(tmpFile, content, 0o600); err != nil { return nil, err }
	return s.scanAtRoot(ctx, tmp, filepath.ToSlash(p))
}

func (s navigationEntrypointScanner153) scanAtRoot(ctx context.Context, scanRoot, scope string) ([]ControllerEndpoint, error) {
	n := nav.Navigator{RepoRoot: scanRoot, AstGrepPath: s.astGrepPath, Runner: rootedRunner153{dir: scanRoot}}
	matches, err := n.FindControllerEndpoints(ctx, scope)
	if err != nil { return nil, err }
	out := make([]ControllerEndpoint, 0, len(matches))
	for _, match := range matches {
		if filepath.ToSlash(match.Path) != filepath.ToSlash(scope) { continue }
		out = append(out, ControllerEndpoint{
			Controller: match.Controller,
			Symbol: match.Symbol,
			Path: filepath.ToSlash(match.Path),
			ControllerStartLine: match.ControllerStartLine,
			ControllerEndLine: match.ControllerEndLine,
			StartLine: match.StartLine,
			EndLine: match.EndLine,
		})
	}
	sort.Slice(out, func(i, j int) bool { if out[i].Symbol != out[j].Symbol { return out[i].Symbol < out[j].Symbol }; return out[i].StartLine < out[j].StartLine })
	return out, nil
}

func owner153(symbol string) string {
	if i := strings.LastIndex(symbol, "."); i > 0 { return symbol[:i] }
	return ""
}
