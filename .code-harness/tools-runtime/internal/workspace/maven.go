package workspace

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type VerificationStatus string

const (
	StatusVerified           VerificationStatus = "VERIFIED"
	StatusVersionUnresolved  VerificationStatus = "VERSION_UNRESOLVED"
	StatusCoordinateMismatch VerificationStatus = "COORDINATE_MISMATCH"
	StatusVersionMismatch    VerificationStatus = "VERSION_MISMATCH"
	StatusSourceNotFound     VerificationStatus = "SOURCE_NOT_FOUND"
)

const (
	CodeSourceNotFound     = "WORKSPACE_DEPENDENCY_SOURCE_NOT_FOUND"
	CodeCoordinateMismatch = "WORKSPACE_DEPENDENCY_COORDINATE_MISMATCH"
	CodeVersionUnresolved  = "WORKSPACE_DEPENDENCY_VERSION_UNRESOLVED"
	CodeVersionMismatch    = "WORKSPACE_DEPENDENCY_VERSION_MISMATCH"
)

type VerificationResult struct {
	DependencyID   string             `json:"dependencyId"`
	Status         VerificationStatus `json:"status"`
	WorkspaceRoot  string             `json:"workspaceRoot,omitempty"`
	ConfirmedRoot  string             `json:"confirmedRoot,omitempty"`
	GroupID        string             `json:"groupId,omitempty"`
	ArtifactID     string             `json:"artifactId,omitempty"`
	CurrentVersion string             `json:"currentVersion,omitempty"`
	SourceVersion  string             `json:"sourceVersion,omitempty"`
	Code           string             `json:"code,omitempty"`
}

type pomParent struct {
	GroupID      string `xml:"groupId"`
	ArtifactID   string `xml:"artifactId"`
	Version      string `xml:"version"`
	RelativePath string `xml:"relativePath"`
}

type pomDependency struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
}

type pomProperties map[string]string

func (p *pomProperties) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	values := map[string]string{}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch v := tok.(type) {
		case xml.StartElement:
			var value string
			if err := d.DecodeElement(&value, &v); err != nil {
				return err
			}
			values[v.Name.Local] = strings.TrimSpace(value)
		case xml.EndElement:
			if v.Name == start.Name {
				*p = values
				return nil
			}
		}
	}
}

type rawPOM struct {
	XMLName              xml.Name        `xml:"project"`
	Parent               pomParent       `xml:"parent"`
	GroupID              string          `xml:"groupId"`
	ArtifactID           string          `xml:"artifactId"`
	Version              string          `xml:"version"`
	Properties           pomProperties   `xml:"properties"`
	Dependencies         []pomDependency `xml:"dependencies>dependency"`
	ManagedDependencies  []pomDependency `xml:"dependencyManagement>dependencies>dependency"`
}

type effectivePOM struct {
	GroupID             string
	ArtifactID          string
	Version             string
	Properties          map[string]string
	Dependencies        []pomDependency
	ManagedDependencies []pomDependency
}

func VerifyDirectMavenDependencies(repoRoot string, deps []Dependency) []VerificationResult {
	results := make([]VerificationResult, 0, len(deps))
	current, currentErr := loadEffectivePOM(filepath.Join(repoRoot, "pom.xml"), map[string]bool{})
	for _, dep := range deps {
		result := VerificationResult{
			DependencyID:  dep.ID,
			WorkspaceRoot: dep.ResolvedRoot,
			GroupID:       dep.Maven.GroupID,
			ArtifactID:    dep.Maven.ArtifactID,
		}
		if currentErr != nil {
			result.Status = StatusCoordinateMismatch
			result.Code = CodeCoordinateMismatch
			results = append(results, result)
			continue
		}

		currentVersion, directFound, versionResolved := directDependencyVersion(current, dep.Maven.GroupID, dep.Maven.ArtifactID)
		if !directFound {
			result.Status = StatusCoordinateMismatch
			result.Code = CodeCoordinateMismatch
			results = append(results, result)
			continue
		}
		if !versionResolved {
			result.Status = StatusVersionUnresolved
			result.Code = CodeVersionUnresolved
			results = append(results, result)
			continue
		}
		result.CurrentVersion = currentVersion

		sourceRoot := strings.TrimSpace(dep.ResolvedRoot)
		if sourceRoot == "" {
			candidate := dep.Root
			if !filepath.IsAbs(candidate) {
				candidate = filepath.Join(repoRoot, candidate)
			}
			sourceRoot = filepath.Clean(candidate)
		}
		sourcePomPath := filepath.Join(sourceRoot, "pom.xml")
		if info, err := os.Stat(sourcePomPath); err != nil || info.IsDir() {
			result.Status = StatusSourceNotFound
			result.Code = CodeSourceNotFound
			results = append(results, result)
			continue
		}
		source, err := loadEffectivePOM(sourcePomPath, map[string]bool{})
		if err != nil {
			result.Status = StatusSourceNotFound
			result.Code = CodeSourceNotFound
			results = append(results, result)
			continue
		}
		sourceGroup, okGroup := resolveValue(source.GroupID, source.Properties)
		sourceArtifact, okArtifact := resolveValue(source.ArtifactID, source.Properties)
		if !okGroup || !okArtifact || sourceGroup != dep.Maven.GroupID || sourceArtifact != dep.Maven.ArtifactID {
			result.Status = StatusCoordinateMismatch
			result.Code = CodeCoordinateMismatch
			results = append(results, result)
			continue
		}
		sourceVersion, okVersion := resolveValue(source.Version, source.Properties)
		if !okVersion || strings.TrimSpace(sourceVersion) == "" {
			result.Status = StatusVersionUnresolved
			result.Code = CodeVersionUnresolved
			results = append(results, result)
			continue
		}
		result.SourceVersion = sourceVersion
		if currentVersion != sourceVersion {
			result.Status = StatusVersionMismatch
			result.Code = CodeVersionMismatch
			results = append(results, result)
			continue
		}
		result.Status = StatusVerified
		result.ConfirmedRoot = filepath.Clean(sourceRoot)
		results = append(results, result)
	}
	return results
}

func directDependencyVersion(model effectivePOM, groupID, artifactID string) (string, bool, bool) {
	for _, dep := range model.Dependencies {
		g, gok := resolveValue(dep.GroupID, model.Properties)
		a, aok := resolveValue(dep.ArtifactID, model.Properties)
		if !gok || !aok || g != groupID || a != artifactID {
			continue
		}
		version := strings.TrimSpace(dep.Version)
		if version == "" {
			for _, managed := range model.ManagedDependencies {
				mg, mgok := resolveValue(managed.GroupID, model.Properties)
				ma, maok := resolveValue(managed.ArtifactID, model.Properties)
				if mgok && maok && mg == groupID && ma == artifactID {
					version = managed.Version
					break
				}
			}
		}
		resolved, ok := resolveValue(version, model.Properties)
		if !ok || strings.TrimSpace(resolved) == "" {
			return "", true, false
		}
		return resolved, true, true
	}
	return "", false, false
}

func loadEffectivePOM(path string, visiting map[string]bool) (effectivePOM, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return effectivePOM{}, err
	}
	abs = filepath.Clean(abs)
	key := strings.ToLower(filepath.ToSlash(abs))
	if visiting[key] {
		return effectivePOM{}, fmt.Errorf("local Maven parent cycle: %s", abs)
	}
	visiting[key] = true
	defer delete(visiting, key)

	data, err := os.ReadFile(abs)
	if err != nil {
		return effectivePOM{}, err
	}
	var raw rawPOM
	if err := xml.Unmarshal(data, &raw); err != nil {
		return effectivePOM{}, fmt.Errorf("parse local pom %s: %w", abs, err)
	}

	model := effectivePOM{Properties: map[string]string{}}
	if hasParent(raw.Parent) {
		parentPath := localParentPath(filepath.Dir(abs), raw.Parent.RelativePath)
		if parentPath != "" {
			if parent, err := loadEffectivePOM(parentPath, visiting); err == nil {
				model.GroupID = parent.GroupID
				model.Version = parent.Version
				for k, v := range parent.Properties {
					model.Properties[k] = v
				}
				model.ManagedDependencies = append(model.ManagedDependencies, parent.ManagedDependencies...)
			}
		}
	}
	if strings.TrimSpace(raw.GroupID) != "" {
		model.GroupID = strings.TrimSpace(raw.GroupID)
	}
	model.ArtifactID = strings.TrimSpace(raw.ArtifactID)
	if strings.TrimSpace(raw.Version) != "" {
		model.Version = strings.TrimSpace(raw.Version)
	}
	for k, v := range raw.Properties {
		model.Properties[k] = strings.TrimSpace(v)
	}
	model.Dependencies = append([]pomDependency(nil), raw.Dependencies...)
	model.ManagedDependencies = mergeManaged(model.ManagedDependencies, raw.ManagedDependencies)

	installBuiltins(&model)
	return model, nil
}

func hasParent(parent pomParent) bool {
	return strings.TrimSpace(parent.GroupID) != "" || strings.TrimSpace(parent.ArtifactID) != "" || strings.TrimSpace(parent.Version) != "" || strings.TrimSpace(parent.RelativePath) != ""
}

func localParentPath(projectDir, relative string) string {
	relative = strings.TrimSpace(relative)
	if relative == "" {
		relative = "../pom.xml"
	}
	if filepath.IsAbs(relative) {
		return ""
	}
	clean := filepath.Clean(filepath.Join(projectDir, relative))
	if info, err := os.Stat(clean); err != nil || info.IsDir() {
		return ""
	}
	return clean
}

func mergeManaged(parent, child []pomDependency) []pomDependency {
	out := append([]pomDependency(nil), parent...)
	for _, candidate := range child {
		replaced := false
		for i := range out {
			if strings.TrimSpace(out[i].GroupID) == strings.TrimSpace(candidate.GroupID) && strings.TrimSpace(out[i].ArtifactID) == strings.TrimSpace(candidate.ArtifactID) {
				out[i] = candidate
				replaced = true
				break
			}
		}
		if !replaced {
			out = append(out, candidate)
		}
	}
	return out
}

func installBuiltins(model *effectivePOM) {
	// Resolve local inherited/property values to a fixed point, then expose the
	// standard Maven project/pom aliases. No Maven process or network is used.
	for i := 0; i < 16; i++ {
		changed := false
		for k, v := range model.Properties {
			if resolved, ok := resolveValue(v, model.Properties); ok && resolved != v {
				model.Properties[k] = resolved
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	if resolved, ok := resolveValue(model.GroupID, model.Properties); ok {
		model.GroupID = resolved
	}
	if resolved, ok := resolveValue(model.Version, model.Properties); ok {
		model.Version = resolved
	}
	model.Properties["project.groupId"] = model.GroupID
	model.Properties["pom.groupId"] = model.GroupID
	model.Properties["project.version"] = model.Version
	model.Properties["pom.version"] = model.Version
	model.Properties["project.artifactId"] = model.ArtifactID
	model.Properties["pom.artifactId"] = model.ArtifactID
}

func resolveValue(value string, props map[string]string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", true
	}
	resolved := value
	for i := 0; i < 32; i++ {
		start := strings.Index(resolved, "${")
		if start < 0 {
			return strings.TrimSpace(resolved), true
		}
		endRel := strings.Index(resolved[start+2:], "}")
		if endRel < 0 {
			return "", false
		}
		end := start + 2 + endRel
		key := resolved[start+2 : end]
		replacement, ok := props[key]
		if !ok {
			return "", false
		}
		resolved = resolved[:start] + replacement + resolved[end+1:]
	}
	return "", false
}

var _ = errors.Is
