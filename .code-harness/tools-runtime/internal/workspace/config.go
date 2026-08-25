package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const ModeReadOnly = "READ_ONLY"

var dependencyIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

type MavenCoordinate struct {
	GroupID    string `json:"groupId" yaml:"groupId"`
	ArtifactID string `json:"artifactId" yaml:"artifactId"`
}

type Dependency struct {
	ID           string          `json:"id" yaml:"id"`
	Root         string          `json:"root" yaml:"root"`
	Maven        MavenCoordinate `json:"maven" yaml:"maven"`
	Mode         string          `json:"mode" yaml:"mode"`
	ResolvedRoot string          `json:"-" yaml:"-"`
}

type Config struct {
	WorkspaceDependencies []Dependency `json:"workspaceDependencies,omitempty" yaml:"workspaceDependencies,omitempty"`
}

func ValidateConfigYAML(repoRoot string, data []byte) ([]Dependency, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("WORKSPACE_DEPENDENCY_CONFIG_INVALID: %w", err)
	}
	if len(cfg.WorkspaceDependencies) == 0 {
		return []Dependency{}, nil
	}

	repoAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("WORKSPACE_DEPENDENCY_CURRENT_PROJECT_INVALID: %w", err)
	}
	repoAbs = filepath.Clean(repoAbs)
	repoInfo, err := os.Stat(repoAbs)
	if err != nil || !repoInfo.IsDir() {
		return nil, fmt.Errorf("WORKSPACE_DEPENDENCY_CURRENT_PROJECT_INVALID: %s", repoAbs)
	}
	repoResolved, err := filepath.EvalSymlinks(repoAbs)
	if err != nil {
		return nil, fmt.Errorf("WORKSPACE_DEPENDENCY_CURRENT_PROJECT_INVALID: %w", err)
	}
	repoResolved = filepath.Clean(repoResolved)
	workspaceParent := filepath.Dir(repoResolved)
	lexicalParent := filepath.Dir(repoAbs)

	seenIDs := map[string]struct{}{}
	seenRoots := map[string]struct{}{}
	out := make([]Dependency, 0, len(cfg.WorkspaceDependencies))
	for _, dep := range cfg.WorkspaceDependencies {
		dep.ID = strings.TrimSpace(dep.ID)
		dep.Root = strings.TrimSpace(dep.Root)
		dep.Mode = strings.TrimSpace(dep.Mode)
		dep.Maven.GroupID = strings.TrimSpace(dep.Maven.GroupID)
		dep.Maven.ArtifactID = strings.TrimSpace(dep.Maven.ArtifactID)

		if !dependencyIDPattern.MatchString(dep.ID) {
			return nil, fmt.Errorf("WORKSPACE_DEPENDENCY_ID_INVALID: %q", dep.ID)
		}
		if _, ok := seenIDs[dep.ID]; ok {
			return nil, fmt.Errorf("WORKSPACE_DEPENDENCY_DUPLICATE_ID: %s", dep.ID)
		}
		seenIDs[dep.ID] = struct{}{}
		if dep.Mode != ModeReadOnly {
			return nil, fmt.Errorf("WORKSPACE_DEPENDENCY_MODE_UNSUPPORTED: %s mode=%q", dep.ID, dep.Mode)
		}
		if dep.Root == "" || isNetworkPath(dep.Root) {
			return nil, fmt.Errorf("WORKSPACE_DEPENDENCY_PATH_REJECTED: %s root=%q", dep.ID, dep.Root)
		}

		candidate := dep.Root
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(repoAbs, candidate)
		}
		candidate = filepath.Clean(candidate)

		if samePath(candidate, repoAbs) {
			return nil, fmt.Errorf("WORKSPACE_DEPENDENCY_CURRENT_PROJECT: %s", dep.ID)
		}
		relFromParent, err := filepath.Rel(lexicalParent, candidate)
		if err != nil {
			return nil, fmt.Errorf("WORKSPACE_DEPENDENCY_PATH_REJECTED: %s: %w", dep.ID, err)
		}
		relSlash := filepath.ToSlash(relFromParent)
		if relSlash == ".." || strings.HasPrefix(relSlash, "../") {
			return nil, fmt.Errorf("WORKSPACE_DEPENDENCY_PATH_REJECTED: %s root=%q", dep.ID, dep.Root)
		}
		if relSlash == "." || strings.Contains(relSlash, "/") {
			return nil, fmt.Errorf("WORKSPACE_DEPENDENCY_NOT_SIBLING: %s root=%q", dep.ID, dep.Root)
		}

		info, err := os.Stat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("WORKSPACE_DEPENDENCY_SOURCE_NOT_FOUND: %s root=%q", dep.ID, dep.Root)
		}
		if err != nil {
			return nil, fmt.Errorf("WORKSPACE_DEPENDENCY_SOURCE_NOT_FOUND: %s: %w", dep.ID, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("WORKSPACE_DEPENDENCY_SOURCE_NOT_FOUND: %s root is not a directory", dep.ID)
		}
		resolved, err := filepath.EvalSymlinks(candidate)
		if err != nil {
			return nil, fmt.Errorf("WORKSPACE_DEPENDENCY_SOURCE_NOT_FOUND: %s: %w", dep.ID, err)
		}
		resolved = filepath.Clean(resolved)
		if samePath(resolved, repoResolved) {
			return nil, fmt.Errorf("WORKSPACE_DEPENDENCY_CURRENT_PROJECT: %s", dep.ID)
		}
		if !samePath(filepath.Dir(resolved), workspaceParent) {
			return nil, fmt.Errorf("WORKSPACE_DEPENDENCY_SYMLINK_ESCAPE: %s root=%q", dep.ID, dep.Root)
		}

		rootKey := pathKey(resolved)
		if _, ok := seenRoots[rootKey]; ok {
			return nil, fmt.Errorf("WORKSPACE_DEPENDENCY_DUPLICATE_ROOT: %s root=%q", dep.ID, dep.Root)
		}
		seenRoots[rootKey] = struct{}{}
		dep.ResolvedRoot = resolved
		out = append(out, dep)
	}
	return out, nil
}

func isNetworkPath(value string) bool {
	normalized := strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	return strings.HasPrefix(normalized, "//")
}

func samePath(left, right string) bool {
	return pathKey(left) == pathKey(right)
}

func pathKey(value string) string {
	return strings.ToLower(filepath.ToSlash(filepath.Clean(value)))
}
