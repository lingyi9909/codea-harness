package reviewscope

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
)

type Target struct {
	Symbol string `json:"symbol"`
	Kind   string `json:"kind"`
}

type CallChain struct {
	EntryPoint string   `json:"entryPoint"`
	Chain      []string `json:"chain"`
}

type Selection struct {
	Mode               string      `json:"mode"`
	Target             *Target     `json:"target,omitempty"`
	SelectedCallChains []CallChain `json:"selectedCallChains"`
	ScopedFiles        []string    `json:"scopedFiles"`
	allChangedFiles    []string
}

type CoverageResult struct {
	Status       string   `json:"status"`
	MissingFiles []string `json:"missingFiles,omitempty"`
}

type changeAnalysis struct {
	ChangedFiles []struct {
		Path string `json:"path"`
	} `json:"changedFiles"`
	CallChains []CallChain `json:"callChains"`
	ReviewCoverage struct {
		ReviewedFiles []struct {
			Path string `json:"path"`
		} `json:"reviewedFiles"`
	} `json:"reviewCoverage"`
}

func Verify(selectionJSON, changeAnalysisJSON []byte) (Selection, error) {
	selection, err := decodeSelection(selectionJSON)
	if err != nil {
		return Selection{}, err
	}
	var analysis changeAnalysis
	if err := json.Unmarshal(changeAnalysisJSON, &analysis); err != nil {
		return Selection{}, fmt.Errorf("parse change analysis: %w", err)
	}

	if selection.Mode != "FULL" && selection.Mode != "TARGETED" {
		return Selection{}, fmt.Errorf("invalid review scope mode %q", selection.Mode)
	}
	if selection.Mode == "FULL" {
		if selection.Target != nil {
			return Selection{}, errors.New("FULL review scope must not contain target")
		}
	} else {
		if selection.Target == nil || strings.TrimSpace(selection.Target.Symbol) == "" {
			return Selection{}, errors.New("TARGETED review scope requires target")
		}
		if selection.Target.Kind != "CLASS" && selection.Target.Kind != "METHOD" {
			return Selection{}, fmt.Errorf("invalid target kind %q", selection.Target.Kind)
		}
		if len(selection.SelectedCallChains) == 0 || len(selection.ScopedFiles) == 0 {
			return Selection{}, errors.New("TARGETED review scope requires selectedCallChains and scopedFiles")
		}
	}

	for _, selected := range selection.SelectedCallChains {
		if !containsChain(analysis.CallChains, selected) {
			return Selection{}, fmt.Errorf("selected call chain %q is not present in validated ChangeAnalysis", selected.EntryPoint)
		}
	}
	if selection.Mode == "TARGETED" && !targetInChains(*selection.Target, selection.SelectedCallChains) {
		return Selection{}, fmt.Errorf("target %q is not represented by selected call chains", selection.Target.Symbol)
	}

	universe := make(map[string]struct{})
	selection.allChangedFiles = make([]string, 0, len(analysis.ChangedFiles))
	for _, f := range analysis.ChangedFiles {
		p := normalizePath(f.Path)
		if p == "" {
			continue
		}
		universe[p] = struct{}{}
		selection.allChangedFiles = append(selection.allChangedFiles, p)
	}
	for _, f := range analysis.ReviewCoverage.ReviewedFiles {
		p := normalizePath(f.Path)
		if p != "" {
			universe[p] = struct{}{}
		}
	}
	selection.allChangedFiles = uniqueSorted(selection.allChangedFiles)

	if selection.Mode == "TARGETED" {
		classes := relatedClasses(*selection.Target, selection.SelectedCallChains)
		for i, f := range selection.ScopedFiles {
			p := normalizePath(f)
			selection.ScopedFiles[i] = p
			if _, ok := universe[p]; !ok {
				return Selection{}, fmt.Errorf("scoped file %q is not present in ChangeAnalysis evidence", f)
			}
			if !fileMatchesClasses(p, classes) {
				return Selection{}, fmt.Errorf("scoped file %q is not justified by selected call chain or target relation", f)
			}
		}
		selection.ScopedFiles = uniqueSorted(selection.ScopedFiles)
	}

	return selection, nil
}

func ComputeCoverage(selection Selection, reviewedFiles []string) CoverageResult {
	required := selection.ScopedFiles
	if selection.Mode == "FULL" {
		required = selection.allChangedFiles
	}
	seen := make(map[string]struct{}, len(reviewedFiles))
	for _, f := range reviewedFiles {
		seen[normalizePath(f)] = struct{}{}
	}
	missing := make([]string, 0)
	for _, f := range required {
		p := normalizePath(f)
		if _, ok := seen[p]; !ok {
			missing = append(missing, p)
		}
	}
	missing = uniqueSorted(missing)
	status := "COMPLETE"
	if len(missing) > 0 {
		status = "PARTIAL"
	}
	return CoverageResult{Status: status, MissingFiles: missing}
}

func decodeSelection(data []byte) (Selection, error) {
	var selection Selection
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&selection); err != nil {
		return Selection{}, fmt.Errorf("parse review scope selection: %w", err)
	}
	return selection, nil
}

func containsChain(chains []CallChain, selected CallChain) bool {
	for _, candidate := range chains {
		if strings.TrimSpace(candidate.EntryPoint) != strings.TrimSpace(selected.EntryPoint) || len(candidate.Chain) != len(selected.Chain) {
			continue
		}
		equal := true
		for i := range candidate.Chain {
			if strings.TrimSpace(candidate.Chain[i]) != strings.TrimSpace(selected.Chain[i]) {
				equal = false
				break
			}
		}
		if equal {
			return true
		}
	}
	return false
}

func targetInChains(target Target, chains []CallChain) bool {
	for _, chain := range chains {
		nodes := append([]string{chain.EntryPoint}, chain.Chain...)
		for _, node := range nodes {
			if target.Kind == "METHOD" && strings.TrimSpace(node) == strings.TrimSpace(target.Symbol) {
				return true
			}
			if target.Kind == "CLASS" && className(node, "METHOD") == className(target.Symbol, "CLASS") {
				return true
			}
		}
	}
	return false
}

func relatedClasses(target Target, chains []CallChain) map[string]struct{} {
	classes := map[string]struct{}{}
	if c := className(target.Symbol, target.Kind); c != "" {
		classes[c] = struct{}{}
	}
	for _, chain := range chains {
		for _, node := range append([]string{chain.EntryPoint}, chain.Chain...) {
			if c := className(node, "METHOD"); c != "" {
				classes[c] = struct{}{}
			}
	}
	return classes
}

func className(symbol, kind string) string {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return ""
	}
	parts := strings.Split(symbol, ".")
	if kind == "METHOD" && len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	return parts[len(parts)-1]
}

func fileMatchesClasses(file string, classes map[string]struct{}) bool {
	base := path.Base(normalizePath(file))
	for _, suffix := range []string{".java", ".kt", ".xml", ".yml", ".yaml"} {
		if strings.HasSuffix(base, suffix) {
			base = strings.TrimSuffix(base, suffix)
			break
		}
	}
	_, ok := classes[base]
	return ok
}

func normalizePath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	return path.Clean(value)
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || value == "." {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
