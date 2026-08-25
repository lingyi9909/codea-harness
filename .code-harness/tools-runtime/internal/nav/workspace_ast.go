package nav

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type workspaceMetaValue struct {
	Text string `json:"text"`
}

type workspaceSGLine struct {
	File string `json:"file"`
	Text string `json:"text"`
	Range struct {
		Start struct {
			Line   int `json:"line"`
			Column int `json:"column"`
		} `json:"start"`
		End struct {
			Line   int `json:"line"`
			Column int `json:"column"`
		} `json:"end"`
	} `json:"range"`
	MetaVariables struct {
		Single map[string]workspaceMetaValue `json:"single"`
	} `json:"metaVariables"`
}

type workspaceRawMatch struct {
	Path        string
	Text        string
	StartLine   int
	StartColumn int
	EndLine     int
	EndColumn   int
	Meta        map[string]string
}

type workspaceTypeMatch struct {
	Name        string
	Super       string
	Path        string
	StartLine   int
	StartColumn int
	EndLine     int
	EndColumn   int
}

type workspaceMethodMatch struct {
	Symbol      string
	Path        string
	StartLine   int
	StartColumn int
	EndLine     int
	EndColumn   int
}

func (n Navigator) runWorkspaceRaw(ctx context.Context, patterns ...string) ([]workspaceRawMatch, error) {
	scope := "src/main/java"
	if err := n.validate("X", scope); err != nil {
		return nil, err
	}
	root := strings.TrimSpace(n.RepoRoot)
	if root == "" {
		root = "."
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	rootAbs = filepath.Clean(rootAbs)
	sourceRoot := filepath.Join(rootAbs, "src", "main", "java")
	info, err := os.Stat(sourceRoot)
	if errors.Is(err, os.ErrNotExist) {
		return []workspaceRawMatch{}, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []workspaceRawMatch{}, nil
	}

	runner := n.Runner
	if runner == nil {
		runner = ExecRunner{}
	}
	seen := map[string]bool{}
	var out []workspaceRawMatch
	for _, pattern := range patterns {
		args := []string{"--lang", "java", "--json=stream", "--pattern", pattern, sourceRoot}
		data, runErr := runner.Run(ctx, n.AstGrepPath, args...)
		if runErr != nil {
			var exitErr *exec.ExitError
			if !errors.As(runErr, &exitErr) {
				return nil, runErr
			}
			if len(data) == 0 {
				continue
			}
		}
		scanner := bufio.NewScanner(bytes.NewReader(data))
		for scanner.Scan() {
			var line workspaceSGLine
			if json.Unmarshal(scanner.Bytes(), &line) != nil {
				continue
			}
			rel, err := workspaceRelativePath(rootAbs, line.File)
			if err != nil {
				return nil, err
			}
			match := workspaceRawMatch{
				Path:        rel,
				Text:        line.Text,
				StartLine:   line.Range.Start.Line + 1,
				StartColumn: line.Range.Start.Column + 1,
				EndLine:     line.Range.End.Line + 1,
				EndColumn:   line.Range.End.Column + 1,
				Meta:        map[string]string{},
			}
			if match.EndLine < match.StartLine {
				match.EndLine = match.StartLine
			}
			for key, value := range line.MetaVariables.Single {
				match.Meta[key] = strings.TrimSpace(value.Text)
			}
			key := fmt.Sprintf("%s:%d:%d:%d:%d:%s", match.Path, match.StartLine, match.StartColumn, match.EndLine, match.EndColumn, match.Text)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, match)
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			if out[i].StartLine == out[j].StartLine {
				if out[i].StartColumn == out[j].StartColumn {
					if out[i].EndLine == out[j].EndLine {
						return out[i].EndColumn < out[j].EndColumn
					}
					return out[i].EndLine < out[j].EndLine
				}
				return out[i].StartColumn < out[j].StartColumn
			}
			return out[i].StartLine < out[j].StartLine
		}
		return out[i].Path < out[j].Path
	})
	return out, nil
}

func workspaceRelativePath(rootAbs, file string) (string, error) {
	clean := filepath.Clean(file)
	if filepath.IsAbs(clean) {
		rel, err := filepath.Rel(rootAbs, clean)
		if err != nil {
			return "", err
		}
		slash := filepath.ToSlash(rel)
		if slash == ".." || strings.HasPrefix(slash, "../") {
			return "", fmt.Errorf("workspace ast-grep result escaped verified root: %s", file)
		}
		return slash, nil
	}
	slash := filepath.ToSlash(clean)
	if idx := strings.Index(slash, "src/main/java/"); idx >= 0 {
		return slash[idx:], nil
	}
	return slash, nil
}

func workspaceClassPatterns(name string, includeAbstract bool) []string {
	suffixes := []string{
		"class " + name + " { $$$BODY }",
		"class " + name + " extends $SUPER { $$$BODY }",
		"class " + name + " implements $$$IFACES { $$$BODY }",
		"class " + name + " extends $SUPER implements $$$IFACES { $$$BODY }",
	}
	mods := []string{"", "public ", "final ", "public final "}
	if includeAbstract {
		mods = append(mods, "abstract ", "public abstract ")
	}
	var out []string
	for _, mod := range mods {
		for _, suffix := range suffixes {
			out = append(out, mod+suffix)
		}
	}
	return withAnnotationVariants(out)
}

func workspaceSubclassPatterns(superName string) []string {
	bases := []string{
		"class $C extends " + superName + " { $$$BODY }",
		"public class $C extends " + superName + " { $$$BODY }",
		"final class $C extends " + superName + " { $$$BODY }",
		"public final class $C extends " + superName + " { $$$BODY }",
		"class $C extends " + superName + " implements $$$IFACES { $$$BODY }",
		"public class $C extends " + superName + " implements $$$IFACES { $$$BODY }",
		"final class $C extends " + superName + " implements $$$IFACES { $$$BODY }",
		"public final class $C extends " + superName + " implements $$$IFACES { $$$BODY }",
	}
	return withAnnotationVariants(bases)
}

func workspaceMethodPatterns(name string) []string {
	bodies := []string{"$RET " + name + "($$$ARGS) { $$$BODY }", "$RET " + name + "($$$ARGS);"}
	mods := []string{"", "public ", "protected ", "private ", "static ", "public static ", "protected static ", "private static ", "final ", "public final ", "protected final ", "abstract ", "public abstract ", "protected abstract ", "synchronized ", "public synchronized ", "protected synchronized ", "default "}
	var out []string
	for _, mod := range mods {
		for _, body := range bodies {
			out = append(out, mod+body)
		}
	}
	return withAnnotationVariants(out)
}

func (n Navigator) WorkspaceSuperclass(ctx context.Context, className string) (workspaceTypeMatch, error) {
	if !identRE.MatchString(className) {
		return workspaceTypeMatch{}, ErrInvalidSymbol
	}
	patterns := []string{
		"class " + className + " extends $SUPER { $$$BODY }",
		"public class " + className + " extends $SUPER { $$$BODY }",
		"final class " + className + " extends $SUPER { $$$BODY }",
		"public final class " + className + " extends $SUPER { $$$BODY }",
		"abstract class " + className + " extends $SUPER { $$$BODY }",
		"public abstract class " + className + " extends $SUPER { $$$BODY }",
		"class " + className + " extends $SUPER implements $$$IFACES { $$$BODY }",
		"public class " + className + " extends $SUPER implements $$$IFACES { $$$BODY }",
	}
	raw, err := n.runWorkspaceRaw(ctx, withAnnotationVariants(patterns)...)
	if err != nil {
		return workspaceTypeMatch{}, err
	}
	matches := dedupeWorkspaceTypes(raw, className, true)
	if len(matches) == 0 {
		return workspaceTypeMatch{}, ErrSymbolNotFound
	}
	if len(matches) > 1 {
		return workspaceTypeMatch{}, ErrAmbiguousSymbol
	}
	return matches[0], nil
}

func (n Navigator) WorkspaceMethod(ctx context.Context, owner, method string) (workspaceMethodMatch, error) {
	if !identRE.MatchString(owner) || !identRE.MatchString(method) {
		return workspaceMethodMatch{}, ErrInvalidSymbol
	}
	typesRaw, err := n.runWorkspaceRaw(ctx, workspaceClassPatterns(owner, true)...)
	if err != nil {
		return workspaceMethodMatch{}, err
	}
	types := dedupeWorkspaceTypes(typesRaw, owner, false)
	if len(types) == 0 {
		return workspaceMethodMatch{}, ErrSymbolNotFound
	}
	if len(types) > 1 {
		return workspaceMethodMatch{}, ErrAmbiguousSymbol
	}
	methodsRaw, err := n.runWorkspaceRaw(ctx, workspaceMethodPatterns(method)...)
	if err != nil {
		return workspaceMethodMatch{}, err
	}
	methods := workspaceMethodsInside(types[0], methodsRaw, owner, method)
	allTypes, err := n.workspaceAllClassTypes(ctx)
	if err != nil {
		return workspaceMethodMatch{}, err
	}
	methods = workspaceMethodsOwnedDirectlyBy(types[0], methods, allTypes)
	if len(methods) == 0 {
		return workspaceMethodMatch{}, ErrSymbolNotFound
	}
	if len(methods) > 1 {
		return workspaceMethodMatch{}, ErrAmbiguousSymbol
	}
	return methods[0], nil
}

func (n Navigator) WorkspaceMethodCalls(ctx context.Context, fromSymbol, called string) (bool, error) {
	owner, method, ok := splitQualifiedMethod(fromSymbol)
	if !ok || !identRE.MatchString(called) {
		return false, ErrInvalidSymbol
	}
	from, err := n.WorkspaceMethod(ctx, owner, method)
	if err != nil {
		return false, err
	}
	calls, err := n.runWorkspaceRaw(ctx, called+"($$$ARGS)", "super."+called+"($$$ARGS)")
	if err != nil {
		return false, err
	}
	for _, call := range calls {
		if call.Path == from.Path && workspaceMethodContainsRaw(from, call) {
			return true, nil
		}
	}
	return false, nil
}

func (n Navigator) WorkspaceDirectSubclassesWithMethod(ctx context.Context, superName, method, concrete string) ([]workspaceTypeMatch, error) {
	if !identRE.MatchString(superName) || !identRE.MatchString(method) {
		return nil, ErrInvalidSymbol
	}
	if concrete != "" && !identRE.MatchString(concrete) {
		return nil, ErrInvalidSymbol
	}
	classesRaw, err := n.runWorkspaceRaw(ctx, workspaceSubclassPatterns(superName)...)
	if err != nil {
		return nil, err
	}
	methodRaw, err := n.runWorkspaceRaw(ctx, workspaceMethodPatterns(method)...)
	if err != nil {
		return nil, err
	}
	allTypes, err := n.workspaceAllClassTypes(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []workspaceTypeMatch
	for _, raw := range classesRaw {
		name := strings.TrimSpace(raw.Meta["C"])
		if name == "" {
			continue
		}
		if concrete != "" && name != concrete {
			continue
		}
		typ := workspaceTypeMatch{
			Name:        name,
			Super:       superName,
			Path:        raw.Path,
			StartLine:   raw.StartLine,
			StartColumn: raw.StartColumn,
			EndLine:     raw.EndLine,
			EndColumn:   raw.EndColumn,
		}
		methods := workspaceMethodsInside(typ, methodRaw, name, method)
		methods = workspaceMethodsOwnedDirectlyBy(typ, methods, allTypes)
		if len(methods) > 1 {
			return nil, ErrAmbiguousSymbol
		}
		if len(methods) != 1 {
			continue
		}
		key := fmt.Sprintf("%s:%d:%d:%d:%d:%s", typ.Path, typ.StartLine, typ.StartColumn, typ.EndLine, typ.EndColumn, typ.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, typ)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			if out[i].StartLine == out[j].StartLine {
				return out[i].StartColumn < out[j].StartColumn
			}
			return out[i].StartLine < out[j].StartLine
		}
		return out[i].Path < out[j].Path
	})
	return out, nil
}

func (n Navigator) workspaceAllClassTypes(ctx context.Context) ([]workspaceTypeMatch, error) {
	raw, err := n.runWorkspaceRaw(ctx, workspaceClassPatterns("$C", true)...)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []workspaceTypeMatch
	for _, match := range raw {
		name := strings.TrimSpace(match.Meta["C"])
		if name == "" {
			continue
		}
		typ := workspaceTypeMatch{
			Name:        name,
			Super:       normalizeWorkspaceType(match.Meta["SUPER"]),
			Path:        match.Path,
			StartLine:   match.StartLine,
			StartColumn: match.StartColumn,
			EndLine:     match.EndLine,
			EndColumn:   match.EndColumn,
		}
		key := fmt.Sprintf("%s:%d:%d:%d:%d:%s", typ.Path, typ.StartLine, typ.StartColumn, typ.EndLine, typ.EndColumn, typ.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, typ)
	}
	return out, nil
}

func dedupeWorkspaceTypes(raw []workspaceRawMatch, expected string, requireSuper bool) []workspaceTypeMatch {
	seen := map[string]bool{}
	var out []workspaceTypeMatch
	for _, match := range raw {
		super := normalizeWorkspaceType(match.Meta["SUPER"])
		if requireSuper && super == "" {
			continue
		}
		typ := workspaceTypeMatch{
			Name:        expected,
			Super:       super,
			Path:        match.Path,
			StartLine:   match.StartLine,
			StartColumn: match.StartColumn,
			EndLine:     match.EndLine,
			EndColumn:   match.EndColumn,
		}
		key := fmt.Sprintf("%s:%d:%d:%d:%d:%s:%s", typ.Path, typ.StartLine, typ.StartColumn, typ.EndLine, typ.EndColumn, typ.Name, typ.Super)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, typ)
	}
	return out
}

func workspaceMethodsInside(typ workspaceTypeMatch, raw []workspaceRawMatch, owner, method string) []workspaceMethodMatch {
	seen := map[string]bool{}
	var out []workspaceMethodMatch
	for _, match := range raw {
		if match.Path != typ.Path || !workspaceTypeContainsRaw(typ, match) {
			continue
		}
		item := workspaceMethodMatch{
			Symbol:      owner + "." + method,
			Path:        match.Path,
			StartLine:   match.StartLine,
			StartColumn: match.StartColumn,
			EndLine:     match.EndLine,
			EndColumn:   match.EndColumn,
		}
		key := fmt.Sprintf("%s:%d:%d:%d:%d", item.Path, item.StartLine, item.StartColumn, item.EndLine, item.EndColumn)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StartLine == out[j].StartLine {
			return out[i].StartColumn < out[j].StartColumn
		}
		return out[i].StartLine < out[j].StartLine
	})
	return out
}

func workspaceMethodsOwnedDirectlyBy(owner workspaceTypeMatch, methods []workspaceMethodMatch, allTypes []workspaceTypeMatch) []workspaceMethodMatch {
	out := make([]workspaceMethodMatch, 0, len(methods))
	for _, method := range methods {
		nestedOwner := false
		for _, candidate := range allTypes {
			if candidate.Path != owner.Path || sameWorkspaceTypeRange(candidate, owner) {
				continue
			}
			if !workspaceTypeContainsType(owner, candidate) {
				continue
			}
			if workspaceTypeContainsMethod(candidate, method) {
				nestedOwner = true
				break
			}
		}
		if !nestedOwner {
			out = append(out, method)
		}
	}
	return out
}

func workspaceTypeContainsRaw(typ workspaceTypeMatch, raw workspaceRawMatch) bool {
	return positionLE(typ.StartLine, typ.StartColumn, raw.StartLine, raw.StartColumn) &&
		positionLE(raw.EndLine, raw.EndColumn, typ.EndLine, typ.EndColumn)
}

func workspaceTypeContainsMethod(typ workspaceTypeMatch, method workspaceMethodMatch) bool {
	return typ.Path == method.Path &&
		positionLE(typ.StartLine, typ.StartColumn, method.StartLine, method.StartColumn) &&
		positionLE(method.EndLine, method.EndColumn, typ.EndLine, typ.EndColumn)
}

func workspaceTypeContainsType(outer, inner workspaceTypeMatch) bool {
	return outer.Path == inner.Path &&
		positionLE(outer.StartLine, outer.StartColumn, inner.StartLine, inner.StartColumn) &&
		positionLE(inner.EndLine, inner.EndColumn, outer.EndLine, outer.EndColumn)
}

func workspaceMethodContainsRaw(method workspaceMethodMatch, raw workspaceRawMatch) bool {
	return method.Path == raw.Path &&
		positionLE(method.StartLine, method.StartColumn, raw.StartLine, raw.StartColumn) &&
		positionLE(raw.EndLine, raw.EndColumn, method.EndLine, method.EndColumn)
}

func sameWorkspaceTypeRange(left, right workspaceTypeMatch) bool {
	return left.Path == right.Path && left.StartLine == right.StartLine && left.StartColumn == right.StartColumn && left.EndLine == right.EndLine && left.EndColumn == right.EndColumn
}

func positionLE(leftLine, leftColumn, rightLine, rightColumn int) bool {
	if leftLine != rightLine {
		return leftLine < rightLine
	}
	return leftColumn <= rightColumn
}

func normalizeWorkspaceType(value string) string {
	value = strings.TrimSpace(value)
	if i := strings.Index(value, "<"); i >= 0 {
		value = value[:i]
	}
	if i := strings.LastIndex(value, "."); i >= 0 {
		value = value[i+1:]
	}
	return strings.TrimSpace(value)
}