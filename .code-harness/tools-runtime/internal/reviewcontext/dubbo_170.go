package reviewcontext

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

	"codea-harness-tools/internal/nav"
	"codea-harness-tools/internal/workspace"
)

// ProviderRoot170 is a source root that may participate in bounded Dubbo
// contract resolution. The private verified bit prevents arbitrary sibling
// directories from being promoted to provider authority by callers.
type ProviderRoot170 struct {
	Workspace string
	Root      string
	verified  bool
}

// ProviderRootsFromVerification170 is the product-facing constructor for
// dependency provider roots. Only the existing workspace Maven gate may grant
// provider authority.
func ProviderRootsFromVerification170(results []workspace.VerificationResult) []ProviderRoot170 {
	out := make([]ProviderRoot170, 0, len(results))
	seen := map[string]bool{}
	for _, result := range results {
		if result.Status != workspace.StatusVerified {
			continue
		}
		workspaceID := strings.TrimSpace(result.DependencyID)
		root := strings.TrimSpace(result.ConfirmedRoot)
		if workspaceID == "" || root == "" {
			continue
		}
		root = filepath.Clean(root)
		key := strings.ToLower(workspaceID) + "\x00" + strings.ToLower(root)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, ProviderRoot170{Workspace: workspaceID, Root: root, verified: true})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Workspace != out[j].Workspace {
			return out[i].Workspace < out[j].Workspace
		}
		return filepath.ToSlash(out[i].Root) < filepath.ToSlash(out[j].Root)
	})
	return out
}

// ResolveDubbo170 identifies source-level Dubbo consumer/provider contracts in
// the current repository and already-verified workspace dependencies. It never
// queries a registry, executes Maven, invokes RPC, or claims runtime routing.
func ResolveDubbo170(ctx context.Context, currentRoot string, consumer nav.ReviewRef170, providers []ProviderRoot170) ([]nav.Relation170, []nav.Issue170, error) {
	return resolveDubboContract170(ctx, currentRoot, consumer, providers)
}
