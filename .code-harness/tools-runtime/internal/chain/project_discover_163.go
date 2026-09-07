package chain

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codea-harness-tools/internal/nav"
)

const projectSourceAuthority163 = "PROJECT_SOURCE"

type ProjectSourceFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type ProjectSourceIdentity struct {
	Version       int                 `json:"version"`
	RunID         string              `json:"runId"`
	AuthorityKind string              `json:"authorityKind"`
	Files         []ProjectSourceFile `json:"files"`
	SourceHash    string              `json:"sourceHash"`
}

type ProjectDiscoverInput struct {
	RunID     string
	Target    string
	Navigator nav.Navigator
}

type projectPath163 struct {
	Nodes     []Node
	Resources []Resource
}

func DiscoverProject(ctx context.Context, root string, in ProjectDiscoverInput) (DiscoveryResult, ProjectSourceIdentity, error) {
	result := DiscoveryResult{Status: DiscoveryComplete, Chains: []Chain{}, Unresolved: []string{}}
	if !runIDPattern.MatchString(in.RunID) {
		return result, ProjectSourceIdentity{}, fmt.Errorf("invalid chain discovery runId %q", in.RunID)
	}
	if in.Target != "" && !targetPattern.MatchString(in.Target) {
		return result, ProjectSourceIdentity{}, fmt.Errorf("invalid chain discovery target %q", in.Target)
	}

	// Capture the source identity before any navigation. The exact same source
	// state must still exist after discovery before a candidate can be persisted
	// or certified as PROJECT_SOURCE authority.
	before, err := computeProjectSourceIdentity163(root, in.RunID)
	if err != nil {
		return result, ProjectSourceIdentity{}, err
	}

	navigator := in.Navigator
	if strings.TrimSpace(navigator.RepoRoot) == "" {
		navigator.RepoRoot = root
	}
	endpoints, err := navigator.FindControllerEndpoints(ctx, "src/main/java")
	if err != nil {
		return result, ProjectSourceIdentity{}, fmt.Errorf("PROJECT_ENTRYPOINT_DISCOVERY_FAILED: %w", err)
	}
	endpoints = projectFilterEndpoints163(endpoints, in.Target)
	if len(endpoints) == 0 {
		result.Status = DiscoveryPartial
		if strings.TrimSpace(in.Target) == "" {
			result.Unresolved = []string{"PROJECT_ENTRYPOINT_NOT_FOUND"}
		} else {
			result.Unresolved = []string{"PROJECT_TARGET_ENTRYPOINT_NOT_FOUND: " + in.Target}
		}
		identity, stableErr := finalizeProjectSourceIdentity163(root, before)
		if stableErr != nil {
			return result, ProjectSourceIdentity{}, stableErr
		}
		return result, identity, nil
	}

	var discovered []Chain
	for _, endpoint := range endpoints {
		entryPath, err := projectRepoPath163(root, endpoint.Path)
		if err != nil {
			return result, ProjectSourceIdentity{}, err
		}
		paths, unresolved, err := projectWalkMethod163(ctx, root, navigator, endpoint.Symbol, map[string]bool{})
		if err != nil {
			return result, ProjectSourceIdentity{}, err
		}
		if len(unresolved) != 0 {
			result.Status = DiscoveryPartial
			result.Unresolved = append(result.Unresolved, unresolved...)
		}
		for _, path := range paths {
			candidate := Chain{
				Version:     1,
				Name:        endpoint.Symbol,
				Status:      StatusDiscovered,
				EntryPoints: []EntryPoint{{Workspace: CurrentWorkspace, Symbol: endpoint.Symbol, Path: entryPath}},
				Nodes:       path.Nodes,
				Resources:   path.Resources,
				Boundaries:  []Boundary{},
			}
			candidate.ID = discoveredID(candidate)
			discovered = append(discovered, candidate)
		}
	}

	discovered = Canonicalize(discovered)
	result.Chains = discovered
	result.Unresolved = uniqueSorted(result.Unresolved)
	if len(discovered) == 0 && len(result.Unresolved) == 0 {
		result.Status = DiscoveryPartial
		result.Unresolved = []string{"PROJECT_CHAIN_NOT_RESOLVED"}
	}
	identity, err := finalizeProjectSourceIdentity163(root, before)
	if err != nil {
		return result, ProjectSourceIdentity{}, err
	}
	if err := persistDiscovered(root, in.RunID, discovered); err != nil {
		return result, ProjectSourceIdentity{}, err
	}
	return result, identity, nil
}

func projectFilterEndpoints163(in []nav.ControllerEndpointMatch, target string) []nav.ControllerEndpointMatch {
	target = strings.TrimSpace(target)
	if target == "" {
		return in
	}
	out := make([]nav.ControllerEndpointMatch, 0)
	for _, endpoint := range in {
		if endpoint.Symbol == target || endpoint.Controller == target {
			out = append(out, endpoint)
		}
	}
	return out
}

func projectWalkMethod163(ctx context.Context, root string, navigator nav.Navigator, method string, visiting map[string]bool) ([]projectPath163, []string, error) {
	if visiting[method] {
		return []projectPath163{{}}, nil, nil
	}
	nextVisiting := make(map[string]bool, len(visiting)+1)
	for key, value := range visiting {
		nextVisiting[key] = value
	}
	nextVisiting[method] = true

	calls, err := navigator.FindDirectMethodCalls(ctx, method, "src/main/java")
	if err != nil {
		return nil, nil, fmt.Errorf("PROJECT_CALL_NAVIGATION_FAILED: %s: %w", method, err)
	}
	if len(calls) == 0 {
		return []projectPath163{{}}, nil, nil
	}

	var out []projectPath163
	var unresolved []string
	for _, call := range calls {
		if !call.Resolved || call.TargetSymbol == "" {
			unresolved = append(unresolved, fmt.Sprintf("PROJECT_CALL_TARGET_UNRESOLVED: %s line %d", method, call.Line))
			continue
		}
		branches, reasons, err := projectResolveCall163(ctx, root, navigator, call, nextVisiting)
		if err != nil {
			return nil, nil, err
		}
		unresolved = append(unresolved, reasons...)
		out = append(out, branches...)
	}
	if len(out) == 0 && len(unresolved) == 0 {
		out = []projectPath163{{}}
	}
	return out, unresolved, nil
}

func projectResolveCall163(ctx context.Context, root string, navigator nav.Navigator, call nav.DirectMethodCall, visiting map[string]bool) ([]projectPath163, []string, error) {
	typeInfo, err := navigator.GetSymbolInfo(ctx, call.ReceiverType, "src/main/java")
	if errors.Is(err, nav.ErrSymbolNotFound) {
		return []projectPath163{{}}, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("PROJECT_TARGET_TYPE_RESOLUTION_FAILED: %s: %w", call.ReceiverType, err)
	}
	typePath, err := projectRepoPath163(root, typeInfo.Path)
	if err != nil {
		return nil, nil, err
	}

	if projectHasAnnotation163(typeInfo.Annotations, "Mapper") {
		if _, err := navigator.GetSymbolInfo(ctx, call.TargetSymbol, "src/main/java"); err != nil {
			return nil, []string{"PROJECT_MAPPER_METHOD_NOT_RESOLVED: " + call.TargetSymbol}, nil
		}
		resource, found, err := projectFindMapperXML163(root, call.ReceiverType, typePath, call.Method)
		if err != nil {
			return nil, nil, err
		}
		if !found {
			return nil, []string{"PROJECT_MAPPER_RESOURCE_NOT_RESOLVED: " + call.TargetSymbol}, nil
		}
		return []projectPath163{{
			Nodes:     []Node{{Workspace: CurrentWorkspace, Symbol: call.TargetSymbol, Path: typePath, Role: "MAPPER"}},
			Resources: []Resource{resource},
		}}, nil, nil
	}

	if strings.EqualFold(typeInfo.Kind, "INTERFACE") {
		if _, err := navigator.GetSymbolInfo(ctx, call.TargetSymbol, "src/main/java"); err != nil {
			return nil, []string{"PROJECT_INTERFACE_METHOD_NOT_RESOLVED: " + call.TargetSymbol}, nil
		}
		impls, err := navigator.FindImplementationTypes(ctx, call.ReceiverType, "src/main/java")
		if err != nil {
			return nil, nil, fmt.Errorf("PROJECT_IMPLEMENTATION_DISCOVERY_FAILED: %s: %w", call.ReceiverType, err)
		}
		if len(impls) == 0 {
			return nil, []string{"IMPLEMENTATION_NOT_FOUND: " + call.ReceiverType}, nil
		}
		if len(impls) > 1 {
			return nil, []string{"AMBIGUOUS_IMPLEMENTATION: " + call.ReceiverType}, nil
		}
		role := projectRoleFromAnnotations163(impls[0].Annotations)
		if role == "" {
			return nil, []string{"PROJECT_IMPLEMENTATION_ROLE_UNVERIFIED: " + impls[0].Symbol}, nil
		}
		implPath, err := projectRepoPath163(root, impls[0].Path)
		if err != nil {
			return nil, nil, err
		}
		implMethod := impls[0].Symbol + "." + call.Method
		if _, err := navigator.GetSymbolInfo(ctx, implMethod, "src/main/java"); err != nil {
			return nil, []string{"PROJECT_IMPLEMENTATION_METHOD_NOT_RESOLVED: " + implMethod}, nil
		}
		tails, unresolved, err := projectWalkMethod163(ctx, root, navigator, implMethod, visiting)
		if err != nil {
			return nil, nil, err
		}
		if len(tails) == 0 {
			return nil, unresolved, nil
		}
		prefix := []Node{
			{Workspace: CurrentWorkspace, Symbol: call.TargetSymbol, Path: typePath, Role: role},
			{Workspace: CurrentWorkspace, Symbol: implMethod, Path: implPath, Role: role},
		}
		return projectPrefixPaths163(prefix, tails), unresolved, nil
	}

	role := projectRoleFromAnnotations163(typeInfo.Annotations)
	if role == "" {
		role = "OTHER"
	}
	if _, err := navigator.GetSymbolInfo(ctx, call.TargetSymbol, "src/main/java"); err != nil {
		return nil, []string{"PROJECT_METHOD_NOT_RESOLVED: " + call.TargetSymbol}, nil
	}
	tails, unresolved, err := projectWalkMethod163(ctx, root, navigator, call.TargetSymbol, visiting)
	if err != nil {
		return nil, nil, err
	}
	prefix := []Node{{Workspace: CurrentWorkspace, Symbol: call.TargetSymbol, Path: typePath, Role: role}}
	return projectPrefixPaths163(prefix, tails), unresolved, nil
}

func projectPrefixPaths163(prefix []Node, tails []projectPath163) []projectPath163 {
	out := make([]projectPath163, 0, len(tails))
	for _, tail := range tails {
		nodes := append([]Node{}, prefix...)
		nodes = append(nodes, tail.Nodes...)
		out = append(out, projectPath163{Nodes: nodes, Resources: append([]Resource{}, tail.Resources...)})
	}
	return out
}

func projectRoleFromAnnotations163(annotations []string) string {
	for _, annotation := range annotations {
		name := strings.TrimPrefix(strings.TrimSpace(annotation), "@")
		if i := strings.IndexByte(name, '('); i >= 0 {
			name = name[:i]
		}
		if i := strings.LastIndexByte(name, '.'); i >= 0 {
			name = name[i+1:]
		}
		switch name {
		case "Service":
			return "SERVICE"
		case "Repository":
			return "REPOSITORY"
		case "Mapper":
			return "MAPPER"
		}
	}
	return ""
}

func projectHasAnnotation163(annotations []string, wanted string) bool {
	return projectRoleFromAnnotations163(annotations) == strings.ToUpper(wanted)
}

func projectFindMapperXML163(root, mapperType, mapperPath, method string) (Resource, bool, error) {
	expectedNamespace := projectJavaFQN163(mapperType, mapperPath)
	if expectedNamespace == "" {
		return Resource{}, false, nil
	}
	resourceRoot := filepath.Join(filepath.Clean(root), "src", "main", "resources")
	var matches []string
	err := filepath.WalkDir(resourceRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.EqualFold(filepath.Ext(entry.Name()), ".xml") {
			return nil
		}
		ok, err := projectMapperStatement163(path, expectedNamespace, method)
		if err != nil {
			return err
		}
		if ok {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return Resource{}, false, fmt.Errorf("PROJECT_MAPPER_RESOURCE_SCAN_FAILED: %w", err)
	}
	if len(matches) == 0 {
		return Resource{}, false, nil
	}
	if len(matches) > 1 {
		return Resource{}, false, fmt.Errorf("PROJECT_MAPPER_RESOURCE_AMBIGUOUS: %s.%s", mapperType, method)
	}
	rel, err := projectRepoPath163(root, matches[0])
	if err != nil {
		return Resource{}, false, err
	}
	return Resource{Path: rel, Symbol: mapperType + "." + method, Role: "MAPPER_XML"}, true, nil
}

func projectMapperStatement163(path, namespace, method string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	decoder := xml.NewDecoder(f)
	mapperMatch := false
	statementMatch := false
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return false, fmt.Errorf("PROJECT_MAPPER_XML_INVALID: %s: %w", path, err)
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local == "mapper" {
			for _, attr := range start.Attr {
				if attr.Name.Local == "namespace" && strings.TrimSpace(attr.Value) == namespace {
					mapperMatch = true
				}
			}
			continue
		}
		for _, attr := range start.Attr {
			if attr.Name.Local == "id" && strings.TrimSpace(attr.Value) == method {
				statementMatch = true
			}
		}
	}
	return mapperMatch && statementMatch, nil
}

func projectJavaFQN163(typeName, path string) string {
	path = filepath.ToSlash(filepath.Clean(path))
	const prefix = "src/main/java/"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, ".java") {
		return ""
	}
	pkg := strings.TrimSuffix(strings.TrimPrefix(path, prefix), ".java")
	parts := strings.Split(pkg, "/")
	if len(parts) == 0 || parts[len(parts)-1] != typeName {
		return ""
	}
	return strings.Join(parts, ".")
}

func projectRepoPath163(root, value string) (string, error) {
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", err
	}
	value = filepath.Clean(value)
	if !filepath.IsAbs(value) {
		value = filepath.Join(rootAbs, value)
	}
	abs, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootAbs, abs)
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("PROJECT_SOURCE_PATH_ESCAPED_ROOT: %s", value)
	}
	return rel, nil
}

func SnapshotProjectSource(root, runID string) (ProjectSourceIdentity, error) {
	identity, err := computeProjectSourceIdentity163(root, runID)
	if err != nil {
		return ProjectSourceIdentity{}, err
	}
	if err := persistProjectSourceIdentity163(root, identity); err != nil {
		return ProjectSourceIdentity{}, err
	}
	return identity, nil
}

func finalizeProjectSourceIdentity163(root string, before ProjectSourceIdentity) (ProjectSourceIdentity, error) {
	after, err := computeProjectSourceIdentity163(root, before.RunID)
	if err != nil {
		return ProjectSourceIdentity{}, err
	}
	if before.SourceHash == "" || before.SourceHash != after.SourceHash {
		return ProjectSourceIdentity{}, fmt.Errorf("PROJECT_SOURCE_CHANGED_DURING_DISCOVERY")
	}
	if err := persistProjectSourceIdentity163(root, before); err != nil {
		return ProjectSourceIdentity{}, err
	}
	return before, nil
}

func persistProjectSourceIdentity163(root string, identity ProjectSourceIdentity) error {
	if identity.RunID == "" || identity.AuthorityKind != projectSourceAuthority163 || identity.SourceHash == "" {
		return fmt.Errorf("PROJECT_SOURCE_IDENTITY_INVALID")
	}
	data, err := json.MarshalIndent(identity, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	path := filepath.Join(filepath.Clean(root), ".code-harness", "runs", identity.RunID, "analysis", "project-source.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("PROJECT_SOURCE_DIR_FAILED: %w", err)
	}
	if err := atomicReplace(path, data); err != nil {
		return fmt.Errorf("PROJECT_SOURCE_WRITE_FAILED: %w", err)
	}
	return nil
}

func computeProjectSourceIdentity163(root, runID string) (ProjectSourceIdentity, error) {
	if !runIDPattern.MatchString(runID) {
		return ProjectSourceIdentity{}, fmt.Errorf("invalid chain discovery runId %q", runID)
	}
	root = filepath.Clean(root)
	sourceRoot := filepath.Join(root, "src", "main")
	var files []ProjectSourceFile
	err := filepath.WalkDir(sourceRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := projectRepoPath163(root, path)
		if err != nil {
			return err
		}
		files = append(files, ProjectSourceFile{Path: rel, SHA256: fmt.Sprintf("%x", sha256.Sum256(data))})
		return nil
	})
	if err != nil {
		return ProjectSourceIdentity{}, fmt.Errorf("PROJECT_SOURCE_INVENTORY_FAILED: %w", err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	payload := struct {
		Version       int                 `json:"version"`
		RunID         string              `json:"runId"`
		AuthorityKind string              `json:"authorityKind"`
		Files         []ProjectSourceFile `json:"files"`
	}{Version: 1, RunID: runID, AuthorityKind: projectSourceAuthority163, Files: files}
	canonical, err := json.Marshal(payload)
	if err != nil {
		return ProjectSourceIdentity{}, err
	}
	return ProjectSourceIdentity{
		Version:       1,
		RunID:         runID,
		AuthorityKind: projectSourceAuthority163,
		Files:         files,
		SourceHash:    fmt.Sprintf("%x", sha256.Sum256(canonical)),
	}, nil
}

func loadProjectSourceIdentity163(root, runID string) (ProjectSourceIdentity, error) {
	path := filepath.Join(filepath.Clean(root), ".code-harness", "runs", runID, "analysis", "project-source.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return ProjectSourceIdentity{}, fmt.Errorf("PROJECT_SOURCE_READ_FAILED: %w", err)
	}
	var identity ProjectSourceIdentity
	if err := json.Unmarshal(data, &identity); err != nil {
		return ProjectSourceIdentity{}, fmt.Errorf("PROJECT_SOURCE_DECODE_FAILED: %w", err)
	}
	canonical, err := json.MarshalIndent(identity, "", "  ")
	if err != nil {
		return ProjectSourceIdentity{}, err
	}
	canonical = append(canonical, '\n')
	if !bytes.Equal(data, canonical) {
		return ProjectSourceIdentity{}, fmt.Errorf("PROJECT_SOURCE_BYTES_NOT_CANONICAL")
	}
	computed, err := computeProjectSourceIdentity163(root, runID)
	if err != nil {
		return ProjectSourceIdentity{}, err
	}
	if identity.RunID != runID || identity.AuthorityKind != projectSourceAuthority163 || identity.SourceHash == "" || identity.SourceHash != computed.SourceHash {
		return ProjectSourceIdentity{}, fmt.Errorf("PROJECT_SOURCE_STALE")
	}
	return identity, nil
}
