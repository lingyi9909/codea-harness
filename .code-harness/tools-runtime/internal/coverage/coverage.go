package coverage

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	"codea-harness-tools/internal/projectpath"
)

type Result struct {
	Status              string   `json:"status"`
	MissingChangedFiles []string `json:"missingChangedFiles,omitempty"`
	UnresolvedSymbols   []string `json:"unresolvedSymbols,omitempty"`
}

func Evaluate(changed, reviewed, unresolved []string) Result {
	return EvaluateRequired(changed, reviewed, unresolved)
}

// EvaluateRequired machine-checks exactly the files required by the declared review scope.
// FULL passes the complete changed-file set; TARGETED passes only verified scopedFiles.
func EvaluateRequired(required, reviewed, unresolved []string) Result {
	seen := make(map[string]bool, len(reviewed))
	for _, p := range reviewed {
		seen[p] = true
	}
	var missing []string
	for _, p := range required {
		if !seen[p] {
			missing = append(missing, p)
		}
	}
	sort.Strings(missing)
	status := "COMPLETE"
	if len(missing) > 0 || len(unresolved) > 0 {
		status = "PARTIAL"
	}
	return Result{Status: status, MissingChangedFiles: missing, UnresolvedSymbols: append([]string(nil), unresolved...)}
}

type analysisFile struct {
	Path string `json:"path"`
	Role string `json:"role"`
}

type navigationLocation struct {
	Workspace string `json:"workspace"`
	Path      string `json:"path"`
}

type resourceRelation struct {
	Path string `json:"path"`
}

type analysis struct {
	ChangedFiles      []analysisFile       `json:"changedFiles"`
	SymbolLocations   []navigationLocation `json:"symbolLocations"`
	ResourceRelations []resourceRelation   `json:"resourceRelations"`
	ReviewCoverage    struct {
		Status            string         `json:"status"`
		ReviewedFiles     []analysisFile `json:"reviewedFiles"`
		UnresolvedSymbols []struct {
			Symbol string `json:"symbol"`
		} `json:"unresolvedSymbols"`
	} `json:"reviewCoverage"`
}

func VerifyAnalysisJSON(data []byte) (Result, error) {
	var a analysis
	if err := json.Unmarshal(data, &a); err != nil {
		return Result{}, fmt.Errorf("parse change analysis: %w", err)
	}
	if err := validateResourceRoles(a.ChangedFiles, a.ReviewCoverage.ReviewedFiles); err != nil {
		return Result{}, err
	}
	if err := validateReviewedFileSources(a); err != nil {
		return Result{Status: "PARTIAL"}, err
	}
	changed := make([]string, 0, len(a.ChangedFiles))
	for _, f := range a.ChangedFiles {
		changed = append(changed, f.Path)
	}
	reviewed := make([]string, 0, len(a.ReviewCoverage.ReviewedFiles))
	for _, f := range a.ReviewCoverage.ReviewedFiles {
		reviewed = append(reviewed, f.Path)
	}
	unresolved := make([]string, 0, len(a.ReviewCoverage.UnresolvedSymbols))
	for _, s := range a.ReviewCoverage.UnresolvedSymbols {
		unresolved = append(unresolved, s.Symbol)
	}
	r := EvaluateRequired(changed, reviewed, unresolved)
	if a.ReviewCoverage.Status != r.Status {
		return r, fmt.Errorf("reviewCoverage.status=%s but machine status=%s", a.ReviewCoverage.Status, r.Status)
	}
	if r.Status != "COMPLETE" {
		return r, fmt.Errorf("review coverage incomplete: missingChangedFiles=%v unresolvedSymbols=%v", r.MissingChangedFiles, r.UnresolvedSymbols)
	}
	return r, nil
}

// validateReviewedFileSources is the FULL-review isolation gate. A dependency workspace
// may contribute navigation/call-chain evidence, but it is never a Review Scope source.
// Legal reviewedFiles origins are exactly: current changedFiles, current-workspace
// symbolLocations, and current-project resourceRelations.
func validateReviewedFileSources(a analysis) error {
	allowed := make(map[string]bool, len(a.ChangedFiles)+len(a.SymbolLocations)+len(a.ResourceRelations))
	dependencyWorkspace := make(map[string]string)

	for _, file := range a.ChangedFiles {
		allowed[normalizePath(file.Path)] = true
	}
	for _, location := range a.SymbolLocations {
		p := normalizePath(location.Path)
		workspace := strings.TrimSpace(location.Workspace)
		if workspace == "" || workspace == "current" {
			allowed[p] = true
			continue
		}
		if _, alreadyCurrent := allowed[p]; !alreadyCurrent {
			dependencyWorkspace[p] = workspace
		}
	}
	for _, relation := range a.ResourceRelations {
		allowed[normalizePath(relation.Path)] = true
	}

	for _, reviewed := range a.ReviewCoverage.ReviewedFiles {
		p := normalizePath(reviewed.Path)
		if allowed[p] {
			continue
		}
		if workspace := dependencyWorkspace[p]; workspace != "" {
			return fmt.Errorf("reviewed file %q belongs to dependency workspace %q and is outside current project FULL review scope", p, workspace)
		}
		return fmt.Errorf("reviewed file %q is not justified by current changedFiles, current-workspace symbolLocations, or current-project resourceRelations", p)
	}
	return nil
}

func validateResourceRoles(changed, reviewed []analysisFile) error {
	changedRoles := make(map[string]string, len(changed))
	for _, file := range changed {
		p := normalizePath(file.Path)
		role := strings.TrimSpace(file.Role)
		if err := validateResourcePathRole(p, role); err != nil {
			return fmt.Errorf("changed file: %w", err)
		}
		if previous, exists := changedRoles[p]; exists && previous != role {
			return fmt.Errorf("changed file %q has conflicting roles %q and %q", p, previous, role)
		}
		changedRoles[p] = role
	}
	for _, file := range reviewed {
		p := normalizePath(file.Path)
		role := strings.TrimSpace(file.Role)
		changedRole, changedFile := changedRoles[p]
		if changedFile && (isResourceRole(changedRole) || isResourceRole(role) || expectedResourceRole(p) != "") && role != changedRole {
			return fmt.Errorf("reviewed file role %q for %q does not match changed file role %q", role, p, changedRole)
		}
		if err := validateResourcePathRole(p, role); err != nil {
			return fmt.Errorf("reviewed file: %w", err)
		}
	}
	return nil
}

func validateResourcePathRole(value, role string) error {
	expected := expectedResourceRole(value)
	if expected != "" && role != expected {
		return fmt.Errorf("resource path %q must use role %s, got %q", value, expected, role)
	}
	switch role {
	case "MapperXml":
		if expected != "MapperXml" {
			return fmt.Errorf("MapperXml path %q must match src/main/resources/**/*Mapper.xml", value)
		}
	case "YamlConfig":
		if expected != "YamlConfig" {
			return fmt.Errorf("YamlConfig path %q must match src/main/resources/**/*.yml", value)
		}
	}
	return nil
}

func expectedResourceRole(value string) string {
	switch projectpath.Classify(value) {
	case projectpath.MapperXML:
		return "MapperXml"
	case projectpath.YAMLConfig:
		return "YamlConfig"
	default:
		return ""
	}
}

func isResourceRole(role string) bool {
	return role == "MapperXml" || role == "YamlConfig"
}

func normalizePath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	return path.Clean(value)
}
