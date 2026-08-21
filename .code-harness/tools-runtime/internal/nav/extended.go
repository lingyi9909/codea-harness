package nav

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

var ErrAmbiguousSymbol = errors.New("AMBIGUOUS_SYMBOL")
var ErrSymbolNotFound = errors.New("SYMBOL_NOT_FOUND")
var annotationNameRE = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

type SymbolInfo struct {
	Symbol        string   `json:"symbol"`
	Kind          string   `json:"kind"`
	DeclaringType string   `json:"declaringType"`
	Signature     string   `json:"signature"`
	ReturnType    string   `json:"returnType,omitempty"`
	Annotations   []string `json:"annotations"`
	Path          string   `json:"path"`
	LineStart     int      `json:"lineStart"`
	LineEnd       int      `json:"lineEnd"`
}

type AnnotationMatch struct {
	Symbol     string `json:"symbol"`
	Kind       string `json:"kind"`
	Annotation string `json:"annotation"`
	Path       string `json:"path"`
	LineStart  int    `json:"lineStart"`
	LineEnd    int    `json:"lineEnd"`
}

type AnnotationResult struct {
	Annotation string            `json:"annotation"`
	Scope      string            `json:"scope"`
	Matches    []AnnotationMatch `json:"matches"`
}

type CallerMatch struct {
	CallerSymbol string `json:"callerSymbol"`
	Path         string `json:"path"`
	Line         int    `json:"line"`
}

type CallerResult struct {
	Symbol  string        `json:"symbol"`
	Scope   string        `json:"scope"`
	Callers []CallerMatch `json:"callers"`
}

type rawMatch struct {
	Path, Text                                 string
	StartLine, StartColumn, EndLine, EndColumn int
}

type sgExtendedLine struct {
	File  string `json:"file"`
	Text  string `json:"text"`
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
}

func (n Navigator) runRaw(ctx context.Context, scope string, patterns ...string) ([]rawMatch, error) {
	if err := n.validate("X", scope); err != nil {
		return nil, err
	}
	runner := n.Runner
	if runner == nil {
		runner = ExecRunner{}
	}
	seen := map[string]bool{}
	var outMatches []rawMatch
	for _, p := range patterns {
		args := []string{"--lang", "java", "--json=stream", "--pattern", p, strings.ReplaceAll(scope, "\\", "/")}
		out, err := runner.Run(ctx, n.AstGrepPath, args...)
		if err != nil {
			var ee *exec.ExitError
			if !errors.As(err, &ee) {
				return nil, err
			}
			if len(out) == 0 {
				continue
			}
		}
		scanner := bufio.NewScanner(bytes.NewReader(out))
		for scanner.Scan() {
			var s sgExtendedLine
			if json.Unmarshal(scanner.Bytes(), &s) != nil {
				continue
			}
			m := rawMatch{Path: strings.ReplaceAll(s.File, "\\", "/"), Text: s.Text, StartLine: s.Range.Start.Line + 1, StartColumn: s.Range.Start.Column + 1, EndLine: s.Range.End.Line + 1, EndColumn: s.Range.End.Column + 1}
			if m.EndLine < m.StartLine {
				m.EndLine = m.StartLine
			}
			key := fmt.Sprintf("%s:%d:%d:%d:%d:%s", m.Path, m.StartLine, m.StartColumn, m.EndLine, m.EndColumn, m.Text)
			if seen[key] {
				continue
			}
			seen[key] = true
			outMatches = append(outMatches, m)
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}
	}
	return outMatches, nil
}

func typePatterns(name string) []string {
	return []string{
		"class " + name + " { $$$BODY }", "public class " + name + " { $$$BODY }", "final class " + name + " { $$$BODY }", "public final class " + name + " { $$$BODY }", "abstract class " + name + " { $$$BODY }", "public abstract class " + name + " { $$$BODY }",
		"interface " + name + " { $$$BODY }", "public interface " + name + " { $$$BODY }",
		"enum " + name + " { $$$BODY }", "public enum " + name + " { $$$BODY }",
	}
}

func allTypePatterns() []string {
	return []string{
		"class $C { $$$BODY }", "public class $C { $$$BODY }", "final class $C { $$$BODY }", "public final class $C { $$$BODY }", "abstract class $C { $$$BODY }", "public abstract class $C { $$$BODY }",
		"interface $C { $$$BODY }", "public interface $C { $$$BODY }", "enum $C { $$$BODY }", "public enum $C { $$$BODY }",
	}
}

func methodPatterns(name string) []string {
	bases := []string{"$RET " + name + "($$$ARGS) { $$$BODY }", "$RET " + name + "($$$ARGS);"}
	mods := []string{"public ", "protected ", "private ", "static ", "public static ", "protected static ", "private static ", "final ", "public final ", "abstract ", "public abstract ", "default "}
	out := append([]string{}, bases...)
	for _, m := range mods {
		out = append(out, m+bases[0], m+bases[1])
	}
	return out
}

func allMethodPatterns() []string {
	bases := []string{"$RET $M($$$ARGS) { $$$BODY }", "$RET $M($$$ARGS);"}
	mods := []string{"public ", "protected ", "private ", "static ", "public static ", "protected static ", "private static ", "final ", "public final ", "abstract ", "public abstract ", "default "}
	out := append([]string{}, bases...)
	for _, m := range mods {
		out = append(out, m+bases[0], m+bases[1])
	}
	return out
}

func fieldPatterns(name string) []string {
	bases := []string{"$T " + name + ";", "$T " + name + " = $INIT;"}
	mods := []string{"public ", "protected ", "private ", "static ", "public static ", "private static ", "final ", "private final ", "public final ", "private static final ", "public static final "}
	out := append([]string{}, bases...)
	for _, m := range mods {
		out = append(out, m+bases[0], m+bases[1])
	}
	return out
}

func annotationPatterns(name string) []string {
	prefixes := []string{"@" + name + " ", "@" + name + "($$$ANNARGS) "}
	decls := append(allTypePatterns(), allMethodPatterns()...)
	var out []string
	for _, pre := range prefixes {
		for _, d := range decls {
			out = append(out, pre+d)
		}
	}
	return out
}

func (n Navigator) GetSymbolInfo(ctx context.Context, symbol, scope string) (SymbolInfo, error) {
	if err := n.validate(symbol, scope); err != nil {
		return SymbolInfo{}, err
	}
	owner, member := splitSymbol(symbol)
	types, err := n.runRaw(ctx, scope, typePatterns(owner)...)
	if err != nil {
		return SymbolInfo{}, err
	}
	types = filterTypeName(types, owner)
	if member == "" {
		if len(types) == 0 {
			return SymbolInfo{}, ErrSymbolNotFound
		}
		if len(types) > 1 {
			return SymbolInfo{}, ErrAmbiguousSymbol
		}
		return makeTypeInfo(symbol, types[0], owner), nil
	}
	methods, err := n.runRaw(ctx, scope, methodPatterns(member)...)
	if err != nil {
		return SymbolInfo{}, err
	}
	fields, err := n.runRaw(ctx, scope, fieldPatterns(member)...)
	if err != nil {
		return SymbolInfo{}, err
	}
	var infos []SymbolInfo
	for _, typ := range types {
		for _, m := range methods {
			if contains(typ, m) && methodName(m.Text) == member {
				infos = append(infos, makeMethodInfo(symbol, owner, member, m))
			}
		}
		for _, f := range fields {
			if contains(typ, f) && fieldName(f.Text) == member {
				infos = append(infos, makeFieldInfo(symbol, owner, member, f))
			}
		}
	}
	infos = dedupeInfos(infos)
	if len(infos) == 0 {
		return SymbolInfo{}, ErrSymbolNotFound
	}
	if len(infos) > 1 {
		return SymbolInfo{}, ErrAmbiguousSymbol
	}
	return infos[0], nil
}

func (n Navigator) FindByAnnotation(ctx context.Context, annotation, scope string) (AnnotationResult, error) {
	if !annotationNameRE.MatchString(annotation) {
		return AnnotationResult{}, ErrInvalidSymbol
	}
	if err := n.validate("X", scope); err != nil {
		return AnnotationResult{}, err
	}
	matches, err := n.runRaw(ctx, scope, annotationPatterns(annotation)...)
	if err != nil {
		return AnnotationResult{}, err
	}
	types, err := n.runRaw(ctx, scope, allTypePatterns()...)
	if err != nil {
		return AnnotationResult{}, err
	}
	var out []AnnotationMatch
	seen := map[string]bool{}
	for _, m := range matches {
		ann := findAnnotation(m.Text, annotation)
		if ann == "" {
			continue
		}
		if kind, name := typeKindAndName(m.Text); kind != "" {
			k := m.Path + ":" + fmt.Sprint(m.StartLine) + ":" + name
			if !seen[k] {
				seen[k] = true
				out = append(out, AnnotationMatch{Symbol: name, Kind: kind, Annotation: ann, Path: m.Path, LineStart: m.StartLine, LineEnd: m.EndLine})
			}
			continue
		}
		name := methodName(m.Text)
		if name == "" {
			continue
		}
		owner := enclosingTypeName(types, m)
		if owner == "" {
			continue
		}
		k := m.Path + ":" + fmt.Sprint(m.StartLine) + ":" + owner + "." + name
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, AnnotationMatch{Symbol: owner + "." + name, Kind: "METHOD", Annotation: ann, Path: m.Path, LineStart: m.StartLine, LineEnd: m.EndLine})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return out[i].LineStart < out[j].LineStart
		}
		return out[i].Path < out[j].Path
	})
	return AnnotationResult{Annotation: annotation, Scope: scope, Matches: out}, nil
}

func (n Navigator) FindCallers(ctx context.Context, symbol, scope string) (CallerResult, error) {
	if err := n.validate(symbol, scope); err != nil {
		return CallerResult{}, err
	}
	_, member := splitSymbol(symbol)
	if member == "" {
		return CallerResult{}, ErrInvalidSymbol
	}
	calls, err := n.runRaw(ctx, scope, "$OBJ."+member+"($$$ARGS)", member+"($$$ARGS)")
	if err != nil {
		return CallerResult{}, err
	}
	methods, err := n.runRaw(ctx, scope, allMethodPatterns()...)
	if err != nil {
		return CallerResult{}, err
	}
	types, err := n.runRaw(ctx, scope, allTypePatterns()...)
	if err != nil {
		return CallerResult{}, err
	}
	var callers []CallerMatch
	seen := map[string]bool{}
	for _, call := range calls {
		method, ok := smallestContaining(methods, call)
		if !ok {
			continue
		}
		name := methodName(method.Text)
		if name == "" {
			continue
		}
		owner := enclosingTypeName(types, method)
		caller := name
		if owner != "" {
			caller = owner + "." + name
		}
		k := fmt.Sprintf("%s:%d:%s", call.Path, call.StartLine, caller)
		if seen[k] {
			continue
		}
		seen[k] = true
		callers = append(callers, CallerMatch{CallerSymbol: caller, Path: call.Path, Line: call.StartLine})
	}
	sort.Slice(callers, func(i, j int) bool {
		if callers[i].Path == callers[j].Path {
			return callers[i].Line < callers[j].Line
		}
		return callers[i].Path < callers[j].Path
	})
	return CallerResult{Symbol: symbol, Scope: scope, Callers: callers}, nil
}

func filterTypeName(ms []rawMatch, name string) []rawMatch {
	var out []rawMatch
	for _, m := range ms {
		_, n := typeKindAndName(m.Text)
		if n == name {
			out = append(out, m)
		}
	}
	return dedupeRaw(out)
}

func dedupeRaw(ms []rawMatch) []rawMatch {
	seen := map[string]bool{}
	out := ms[:0]
	for _, m := range ms {
		k := fmt.Sprintf("%s:%d:%d", m.Path, m.StartLine, m.EndLine)
		if !seen[k] {
			seen[k] = true
			out = append(out, m)
		}
	}
	return out
}

func dedupeInfos(in []SymbolInfo) []SymbolInfo {
	seen := map[string]bool{}
	var out []SymbolInfo
	for _, v := range in {
		k := fmt.Sprintf("%s:%d:%d:%s", v.Path, v.LineStart, v.LineEnd, v.Kind)
		if !seen[k] {
			seen[k] = true
			out = append(out, v)
		}
	}
	return out
}

func contains(outer, inner rawMatch) bool {
	return outer.Path == inner.Path && outer.StartLine <= inner.StartLine && outer.EndLine >= inner.EndLine
}

func smallestContaining(candidates []rawMatch, target rawMatch) (rawMatch, bool) {
	var best rawMatch
	found := false
	for _, c := range candidates {
		if !contains(c, target) {
			continue
		}
		if !found || (c.EndLine-c.StartLine) < (best.EndLine-best.StartLine) {
			best = c
			found = true
		}
	}
	return best, found
}

func enclosingTypeName(types []rawMatch, m rawMatch) string {
	t, ok := smallestContaining(types, m)
	if !ok {
		return ""
	}
	_, name := typeKindAndName(t.Text)
	return name
}

var typeDeclRE = regexp.MustCompile(`\b(class|interface|enum)\s+([A-Za-z_$][A-Za-z0-9_$]*)\b`)

func typeKindAndName(text string) (string, string) {
	m := typeDeclRE.FindStringSubmatch(text)
	if len(m) != 3 {
		return "", ""
	}
	return strings.ToUpper(m[1]), m[2]
}

func annotations(text string) []string {
	var out []string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "@") {
			out = append(out, line)
			continue
		}
		if line != "" {
			break
		}
	}
	return out
}

func findAnnotation(text, name string) string {
	prefix := "@" + name
	for _, a := range annotations(text) {
		if a == prefix || strings.HasPrefix(a, prefix+"(") {
			return a
		}
	}
	return ""
}

func declarationLine(text string) string {
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "@") {
			continue
		}
		return strings.Join(strings.Fields(line), " ")
	}
	return ""
}

func methodName(text string) string {
	line := declarationLine(text)
	open := strings.Index(line, "(")
	if open < 0 {
		return ""
	}
	before := strings.TrimSpace(line[:open])
	parts := strings.Fields(before)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func methodParts(text, name string) (sig, ret string) {
	line := declarationLine(text)
	needle := name + "("
	i := strings.Index(line, needle)
	if i < 0 {
		return "", ""
	}
	rest := line[i+len(name):]
	close := strings.Index(rest, ")")
	if close < 0 {
		return "", ""
	}
	sig = name + rest[:close+1]
	before := strings.TrimSpace(line[:i])
	parts := strings.Fields(before)
	if len(parts) > 0 {
		ret = parts[len(parts)-1]
	}
	return sig, ret
}

func fieldName(text string) string {
	line := declarationLine(text)
	line = strings.TrimSuffix(strings.TrimSpace(line), ";")
	if i := strings.Index(line, "="); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func fieldType(text, name string) string {
	line := declarationLine(text)
	i := strings.Index(line, name)
	if i < 0 {
		return ""
	}
	before := strings.TrimSpace(line[:i])
	parts := strings.Fields(before)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func makeTypeInfo(symbol string, m rawMatch, name string) SymbolInfo {
	kind, _ := typeKindAndName(m.Text)
	return SymbolInfo{Symbol: symbol, Kind: kind, DeclaringType: name, Signature: typeSignature(m.Text, name), Annotations: annotations(m.Text), Path: m.Path, LineStart: m.StartLine, LineEnd: m.EndLine}
}

func typeSignature(text, name string) string {
	line := declarationLine(text)
	if i := strings.Index(line, "{"); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	m := typeDeclRE.FindStringSubmatch(line)
	if len(m) == 3 && m[2] == name {
		return strings.TrimSpace(m[1] + " " + name)
	}
	return line
}

func makeMethodInfo(symbol, owner, name string, m rawMatch) SymbolInfo {
	sig, ret := methodParts(m.Text, name)
	return SymbolInfo{Symbol: symbol, Kind: "METHOD", DeclaringType: owner, Signature: sig, ReturnType: ret, Annotations: annotations(m.Text), Path: m.Path, LineStart: m.StartLine, LineEnd: m.EndLine}
}

func makeFieldInfo(symbol, owner, name string, m rawMatch) SymbolInfo {
	return SymbolInfo{Symbol: symbol, Kind: "FIELD", DeclaringType: owner, Signature: name, ReturnType: fieldType(m.Text, name), Annotations: annotations(m.Text), Path: m.Path, LineStart: m.StartLine, LineEnd: m.EndLine}
}
