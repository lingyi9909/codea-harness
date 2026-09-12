package nav

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

type javaFileMeta170 struct {
	Package string
	Imports map[string]string
	Source  string
}

type javaType170 struct {
	Name       string
	FQCN       string
	Kind       string
	Package    string
	Path       string
	Super      string
	Interfaces []string
	Imports    map[string]string
	Raw        workspaceRawMatch
	Text       string
}

type javaParam170 struct {
	Type string
	Name string
}

type javaMethod170 struct {
	Owner      *javaType170
	Name       string
	ReturnType string
	Params     []javaParam170
	Raw        workspaceRawMatch
	Text       string
}

type javaIndex170 struct {
	Types      []*javaType170
	Methods    []*javaMethod170
	ByFQCN     map[string]*javaType170
	FileMeta   map[string]javaFileMeta170
	SimpleFQCN map[string][]string
}

type javaResolvedCall170 struct {
	Relation     Relation170
	Receiver     string
	ReceiverType string
	Method       string
}

var (
	javaPackageRE170    = regexp.MustCompile(`(?m)^\s*package\s+([A-Za-z_$][A-Za-z0-9_$.]*)\s*;`)
	javaImportRE170     = regexp.MustCompile(`(?m)^\s*import\s+(?:static\s+)?([A-Za-z_$][A-Za-z0-9_$.]*)\s*;`)
	javaExtendsRE170    = regexp.MustCompile(`\bextends\s+([A-Za-z_$][A-Za-z0-9_$.[\]<>?]*)`)
	javaImplementsRE170 = regexp.MustCompile(`\bimplements\s+([^\{]+)`)
	javaLocalRE170      = regexp.MustCompile(`(?m)(?:^|[;{}])\s*(?:final\s+)?([A-Za-z_$][A-Za-z0-9_$.[\]<>?]*)\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*(?:=|;)`)
	javaFieldRE170      = regexp.MustCompile(`(?m)(?:^|[;{}])\s*(?:public\s+|protected\s+|private\s+|static\s+|final\s+|volatile\s+|transient\s+)*([A-Za-z_$][A-Za-z0-9_$.[\]<>?]*)\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*(?:=|;)`)
	javaNewRE170        = regexp.MustCompile(`^new\s+([A-Za-z_$][A-Za-z0-9_$.[\]<>?]*)\s*\(`)
	javaStringRE170     = regexp.MustCompile(`^"(?:\\.|[^"\\])*"$`)
	javaCharRE170       = regexp.MustCompile(`^'(?:\\.|[^'\\])'$`)
	javaIntRE170        = regexp.MustCompile(`^[+-]?[0-9][0-9_]*$`)
	javaLongRE170       = regexp.MustCompile(`^[+-]?[0-9][0-9_]*[lL]$`)
)

func javaAllTypePatterns170() []string {
	base := []string{
		"class $C { $$$BODY }",
		"class $C extends $SUPER { $$$BODY }",
		"class $C implements $$$IFACES { $$$BODY }",
		"class $C extends $SUPER implements $$$IFACES { $$$BODY }",
		"interface $C { $$$BODY }",
		"interface $C extends $$$IFACES { $$$BODY }",
		"enum $C { $$$BODY }",
	}
	mods := []string{"", "public ", "protected ", "private ", "abstract ", "public abstract ", "final ", "public final "}
	out := make([]string, 0, len(base)*len(mods))
	for _, mod := range mods {
		for _, pattern := range base {
			out = append(out, mod+pattern)
		}
	}
	return withAnnotationVariants(out)
}

func (n Navigator) buildJavaIndex170(ctx context.Context) (*javaIndex170, error) {
	rootAbs, sourceRoot, exists, err := n.workspaceRoots()
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrSymbolNotFound
	}
	files, err := workspaceJavaInventory(sourceRoot)
	if err != nil {
		return nil, err
	}
	idx := &javaIndex170{
		ByFQCN:     map[string]*javaType170{},
		FileMeta:   map[string]javaFileMeta170{},
		SimpleFQCN: map[string][]string{},
	}
	for _, file := range files {
		data, readErr := os.ReadFile(file)
		if readErr != nil {
			return nil, readErr
		}
		rel, relErr := workspaceRelativePath(rootAbs, file)
		if relErr != nil {
			return nil, relErr
		}
		idx.FileMeta[rel] = parseJavaFileMeta170(string(data))
	}
	typeRaw, err := n.runWorkspaceRawTargets(ctx, files, javaAllTypePatterns170()...)
	if err != nil {
		return nil, err
	}
	seenTypes := map[string]bool{}
	for _, raw := range typeRaw {
		kind, name := typeKindAndName(raw.Text)
		if name == "" {
			name = strings.TrimSpace(raw.Meta["C"])
		}
		if name == "" {
			continue
		}
		meta, ok := idx.FileMeta[raw.Path]
		if !ok {
			continue
		}
		fqcn := name
		if meta.Package != "" {
			fqcn = meta.Package + "." + name
		}
		key := fmt.Sprintf("%s:%d:%d:%d:%d:%s", raw.Path, raw.StartLine, raw.StartColumn, raw.EndLine, raw.EndColumn, fqcn)
		if seenTypes[key] {
			continue
		}
		seenTypes[key] = true
		typ := &javaType170{
			Name:       name,
			FQCN:       fqcn,
			Kind:       kind,
			Package:    meta.Package,
			Path:       raw.Path,
			Imports:    meta.Imports,
			Raw:        raw,
			Text:       raw.Text,
			Super:      parseJavaSuper170(raw.Text),
			Interfaces: parseJavaInterfaces170(raw.Text),
		}
		idx.Types = append(idx.Types, typ)
		if existing := idx.ByFQCN[fqcn]; existing == nil || javaTypeRangeSmaller170(typ, existing) {
			idx.ByFQCN[fqcn] = typ
		}
		idx.SimpleFQCN[name] = appendUniqueString170(idx.SimpleFQCN[name], fqcn)
	}

	methodRaw, err := n.runWorkspaceRawTargets(ctx, files, allMethodPatterns()...)
	if err != nil {
		return nil, err
	}
	for _, raw := range methodRaw {
		owner := smallestJavaOwner170(idx.Types, raw)
		if owner == nil {
			continue
		}
		name := methodName(raw.Text)
		if name == "" {
			continue
		}
		params, returnType := parseJavaMethodDeclaration170(raw.Text, name)
		idx.Methods = append(idx.Methods, &javaMethod170{Owner: owner, Name: name, ReturnType: returnType, Params: params, Raw: raw, Text: raw.Text})
	}
	sort.Slice(idx.Methods, func(i, j int) bool {
		a, b := idx.Methods[i], idx.Methods[j]
		if a.Owner.FQCN != b.Owner.FQCN {
			return a.Owner.FQCN < b.Owner.FQCN
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.Raw.Path != b.Raw.Path {
			return a.Raw.Path < b.Raw.Path
		}
		if a.Raw.StartLine != b.Raw.StartLine {
			return a.Raw.StartLine < b.Raw.StartLine
		}
		return a.Raw.StartColumn < b.Raw.StartColumn
	})
	return idx, nil
}

func parseJavaFileMeta170(source string) javaFileMeta170 {
	meta := javaFileMeta170{Imports: map[string]string{}, Source: source}
	if match := javaPackageRE170.FindStringSubmatch(source); len(match) == 2 {
		meta.Package = match[1]
	}
	for _, match := range javaImportRE170.FindAllStringSubmatch(source, -1) {
		if len(match) != 2 || strings.HasSuffix(match[1], ".*") {
			continue
		}
		fqcn := match[1]
		if i := strings.LastIndexByte(fqcn, '.'); i >= 0 && i+1 < len(fqcn) {
			meta.Imports[fqcn[i+1:]] = fqcn
		}
	}
	return meta
}

func parseJavaSuper170(text string) string {
	match := javaExtendsRE170.FindStringSubmatch(declarationLine(text))
	if len(match) != 2 {
		return ""
	}
	return normalizeJavaType170(match[1])
}

func parseJavaInterfaces170(text string) []string {
	line := declarationLine(text)
	match := javaImplementsRE170.FindStringSubmatch(line)
	if len(match) != 2 {
		if strings.Contains(line, "interface ") {
			if ext := javaExtendsRE170.FindStringSubmatch(line); len(ext) == 2 {
				return []string{normalizeJavaType170(ext[1])}
			}
		}
		return []string{}
	}
	parts := splitJavaTopLevel170(match[1], ',')
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if typ := normalizeJavaType170(part); typ != "" {
			out = append(out, typ)
		}
	}
	return out
}

func javaTypeRangeSmaller170(left, right *javaType170) bool {
	leftSpan := (left.Raw.EndLine-left.Raw.StartLine)*100000 + (left.Raw.EndColumn-left.Raw.StartColumn)
	rightSpan := (right.Raw.EndLine-right.Raw.StartLine)*100000 + (right.Raw.EndColumn-right.Raw.StartColumn)
	return leftSpan < rightSpan
}

func smallestJavaOwner170(types []*javaType170, raw workspaceRawMatch) *javaType170 {
	var best *javaType170
	for _, typ := range types {
		if !workspaceTypeContainsRaw(workspaceTypeMatch{Path: typ.Path, StartLine: typ.Raw.StartLine, StartColumn: typ.Raw.StartColumn, EndLine: typ.Raw.EndLine, EndColumn: typ.Raw.EndColumn}, raw) {
			continue
		}
		if best == nil || javaTypeRangeSmaller170(typ, best) {
			best = typ
		}
	}
	return best
}

func parseJavaMethodDeclaration170(text, name string) ([]javaParam170, string) {
	line := declarationLine(text)
	needle := name + "("
	namePos := strings.Index(line, needle)
	if namePos < 0 {
		return []javaParam170{}, ""
	}
	open := namePos + len(name)
	close := balancedClose(line, open)
	if close < 0 {
		return []javaParam170{}, ""
	}
	paramsText := strings.TrimSpace(line[open+1 : close])
	params := make([]javaParam170, 0)
	if paramsText != "" {
		for _, rawParam := range splitJavaTopLevel170(paramsText, ',') {
			param := parseJavaParam170(rawParam)
			if param.Name != "" && param.Type != "" {
				params = append(params, param)
			}
		}
	}
	before := strings.TrimSpace(line[:namePos])
	parts := strings.Fields(before)
	returnType := ""
	if len(parts) > 0 {
		returnType = normalizeJavaType170(parts[len(parts)-1])
	}
	return params, returnType
}

func parseJavaParam170(text string) javaParam170 {
	text = stripJavaAnnotations170(text)
	text = strings.ReplaceAll(text, "...", "[]")
	parts := strings.Fields(text)
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "final", "volatile", "transient":
			continue
		}
		filtered = append(filtered, part)
	}
	if len(filtered) < 2 {
		return javaParam170{}
	}
	return javaParam170{Type: normalizeJavaType170(strings.Join(filtered[:len(filtered)-1], " ")), Name: strings.TrimSpace(filtered[len(filtered)-1])}
}

func stripJavaAnnotations170(text string) string {
	s := strings.TrimSpace(text)
	for strings.HasPrefix(s, "@") {
		end := annotationEnd(s)
		if end <= 0 || end > len(s) {
			break
		}
		s = strings.TrimSpace(s[end:])
	}
	return s
}

func splitJavaTopLevel170(text string, separator byte) []string {
	var out []string
	start := 0
	paren, angle, bracket, brace := 0, 0, 0, 0
	quote := byte(0)
	escaped := false
	for i := 0; i < len(text); i++ {
		c := text[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
		case '(':
			paren++
		case ')':
			if paren > 0 { paren-- }
		case '<':
			angle++
		case '>':
			if angle > 0 { angle-- }
		case '[':
			bracket++
		case ']':
			if bracket > 0 { bracket-- }
		case '{':
			brace++
		case '}':
			if brace > 0 { brace-- }
		default:
			if c == separator && paren == 0 && angle == 0 && bracket == 0 && brace == 0 {
				out = append(out, strings.TrimSpace(text[start:i]))
				start = i + 1
			}
		}
	}
	out = append(out, strings.TrimSpace(text[start:]))
	return out
}

func normalizeJavaType170(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "? extends ", "")
	value = strings.ReplaceAll(value, "? super ", "")
	for {
		start := strings.IndexByte(value, '<')
		if start < 0 {
			break
		}
		depth := 0
		end := -1
		for i := start; i < len(value); i++ {
			switch value[i] {
			case '<': depth++
			case '>':
				depth--
				if depth == 0 { end = i; break }
			}
			if end >= 0 { break }
		}
		if end < 0 {
			value = value[:start]
			break
		}
		value = value[:start] + value[end+1:]
	}
	return strings.TrimSpace(value)
}

func (idx *javaIndex170) resolveTypeName170(owner *javaType170, declared string) string {
	declared = normalizeJavaType170(declared)
	base := strings.TrimSuffix(declared, "[]")
	if base == "" {
		return ""
	}
	if strings.Contains(base, ".") {
		return base
	}
	if owner != nil {
		if fqcn := owner.Imports[base]; fqcn != "" {
			return fqcn
		}
		if owner.Package != "" {
			candidate := owner.Package + "." + base
			if idx.ByFQCN[candidate] != nil {
				return candidate
			}
		}
	}
	if matches := idx.SimpleFQCN[base]; len(matches) == 1 {
		return matches[0]
	}
	switch base {
	case "String", "Long", "Integer", "Boolean", "Double", "Float", "Short", "Byte", "Character", "Object":
		return "java.lang." + base
	}
	return base
}

func (idx *javaIndex170) findMethod170(ref ReviewRef170) (*javaMethod170, error) {
	owner := idx.ByFQCN[ref.OwnerFQCN]
	if owner == nil {
		return nil, ErrSymbolNotFound
	}
	var candidates []*javaMethod170
	for _, method := range idx.Methods {
		if method.Owner.FQCN == ref.OwnerFQCN && method.Name == ref.Name {
			candidates = append(candidates, method)
		}
	}
	if len(ref.ParameterTypes) > 0 {
		filtered := candidates[:0]
		for _, candidate := range candidates {
			if len(candidate.Params) != len(ref.ParameterTypes) {
				continue
			}
			match := true
			for i, param := range candidate.Params {
				left := idx.resolveTypeName170(owner, param.Type)
				right := idx.resolveTypeName170(owner, ref.ParameterTypes[i])
				if !javaTypeEqual170(left, right) {
					match = false
					break
				}
			}
			if match { filtered = append(filtered, candidate) }
		}
		candidates = filtered
	}
	if len(candidates) == 0 {
		return nil, ErrSymbolNotFound
	}
	if len(candidates) > 1 {
		return nil, ErrAmbiguousSymbol
	}
	return candidates[0], nil
}

func javaTypeEqual170(left, right string) bool {
	left, right = normalizeJavaType170(left), normalizeJavaType170(right)
	if left == right {
		return true
	}
	return simpleType(left) == simpleType(right) && (left == simpleType(left) || right == simpleType(right))
}

func (n Navigator) InspectMethod170(ctx context.Context, ref ReviewRef170) (MethodFacts170, error) {
	if ref.Kind != "METHOD" {
		return MethodFacts170{}, fmt.Errorf("JAVA_METHOD_INVALID: expected METHOD ref")
	}
	if _, err := normalizeReviewRef170(ref); err != nil {
		return MethodFacts170{}, err
	}
	idx, err := n.buildJavaIndex170(ctx)
	if err != nil {
		return MethodFacts170{}, err
	}
	method, err := idx.findMethod170(ref)
	if err != nil {
		return MethodFacts170{}, err
	}
	calls, err := n.resolveMethodCalls170(ctx, idx, method, ref)
	if err != nil {
		return MethodFacts170{}, err
	}
	facts := MethodFacts170{Method: ref, Calls: make([]Relation170, 0, len(calls)), Issues: []Issue170{}}
	for _, call := range calls {
		facts.Calls = append(facts.Calls, call.Relation)
		if call.Relation.Resolution != "EXACT" {
			code := call.Relation.Reason
			if i := strings.IndexAny(code, ": ;"); i > 0 { code = code[:i] }
			at := SourceRange170{Ref: ref, StartLine: 1, EndLine: 1, StartColumn: 1, EndColumn: 2}
			if len(call.Relation.Evidence) > 0 { at = call.Relation.Evidence[0] }
			facts.Issues = append(facts.Issues, Issue170{Code: code, At: at, Detail: call.Relation.Reason})
		}
	}
	return facts, nil
}

func (n Navigator) resolveMethodCalls170(ctx context.Context, idx *javaIndex170, method *javaMethod170, fromRef ReviewRef170) ([]javaResolvedCall170, error) {
	rawCalls, err := n.runWorkspaceRawTargets(ctx, []string{method.Raw.Path}, "$OBJ.$M($$$ARGS)", "$M($$$ARGS)")
	if err != nil {
		return nil, err
	}
	out := make([]javaResolvedCall170, 0)
	for _, raw := range rawCalls {
		if !workspaceMethodContainsRaw(workspaceMethodMatch{Path: method.Raw.Path, StartLine: method.Raw.StartLine, StartColumn: method.Raw.StartColumn, EndLine: method.Raw.EndLine, EndColumn: method.Raw.EndColumn}, raw) {
			continue
		}
		receiver, called, args, ok := javaCallParts170(raw.Text)
		if !ok {
			continue
		}
		ownerFQCN, receiverReason := idx.resolveReceiver170(method, receiver)
		evidence, rangeErr := n.javaSourceRange170(fromRef, raw)
		if rangeErr != nil {
			return nil, rangeErr
		}
		relation := Relation170{
			Kind:        "JAVA_CALL",
			From:        fromRef,
			Targets:     []ReviewRef170{},
			Evidence:    []SourceRange170{evidence},
			Assumptions: []string{},
		}
		resolved := javaResolvedCall170{Relation: relation, Receiver: receiver, ReceiverType: ownerFQCN, Method: called}
		if ownerFQCN == "" {
			resolved.Relation.Resolution = "UNRESOLVED"
			if receiverReason == "" { receiverReason = "JAVA_RECEIVER_UNRESOLVED" }
			resolved.Relation.Reason = receiverReason
			resolved.Relation.ID = relationID170(resolved.Relation, raw)
			out = append(out, resolved)
			continue
		}
		candidates := idx.methodsForCall170(ownerFQCN, called)
		if len(candidates) == 0 {
			resolved.Relation.Resolution = "UNRESOLVED"
			resolved.Relation.Reason = "JAVA_RECEIVER_UNRESOLVED: target method declaration unavailable"
			resolved.Relation.ID = relationID170(resolved.Relation, raw)
			out = append(out, resolved)
			continue
		}
		selected, ambiguous := idx.selectOverload170(method, candidates, args)
		if selected != nil && !ambiguous {
			resolved.Relation.Resolution = "EXACT"
			resolved.Relation.Targets = []ReviewRef170{idx.methodRef170(selected, fromRef.Workspace, fromRef.Side)}
			resolved.Relation.Reason = ""
		} else {
			resolved.Relation.Resolution = "AMBIGUOUS"
			resolved.Relation.Reason = "JAVA_OVERLOAD_UNRESOLVED: multiple compatible overloads"
			for _, candidate := range candidates {
				if len(candidate.Params) == len(args) {
					resolved.Relation.Targets = append(resolved.Relation.Targets, idx.methodRef170(candidate, fromRef.Workspace, fromRef.Side))
				}
			}
			if len(resolved.Relation.Targets) == 0 {
				for _, candidate := range candidates {
					resolved.Relation.Targets = append(resolved.Relation.Targets, idx.methodRef170(candidate, fromRef.Workspace, fromRef.Side))
				}
			}
		}
		resolved.Relation.ID = relationID170(resolved.Relation, raw)
		out = append(out, resolved)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Relation.Evidence[0], out[j].Relation.Evidence[0]
		if a.Ref.Path != b.Ref.Path { return a.Ref.Path < b.Ref.Path }
		if a.StartLine != b.StartLine { return a.StartLine < b.StartLine }
		if a.StartColumn != b.StartColumn { return a.StartColumn < b.StartColumn }
		return out[i].Relation.ID < out[j].Relation.ID
	})
	return out, nil
}

func (idx *javaIndex170) resolveReceiver170(method *javaMethod170, receiver string) (string, string) {
	owner := method.Owner
	switch receiver {
	case "":
		return owner.FQCN, ""
	case "this":
		return owner.FQCN, ""
	case "super":
		if owner.Super == "" { return "", "JAVA_RECEIVER_UNRESOLVED: superclass unavailable" }
		return idx.resolveTypeName170(owner, owner.Super), ""
	}
	for _, param := range method.Params {
		if param.Name == receiver {
			return idx.resolveTypeName170(owner, param.Type), ""
		}
	}
	for _, match := range javaLocalRE170.FindAllStringSubmatch(method.Text, -1) {
		if len(match) == 3 && match[2] == receiver {
			return idx.resolveTypeName170(owner, match[1]), ""
		}
	}
	for _, match := range javaFieldRE170.FindAllStringSubmatch(owner.Text, -1) {
		if len(match) == 3 && match[2] == receiver {
			return idx.resolveTypeName170(owner, match[1]), ""
		}
	}
	return "", "JAVA_RECEIVER_UNRESOLVED: receiver declaration unavailable"
}

func (idx *javaIndex170) methodsForCall170(ownerFQCN, name string) []*javaMethod170 {
	visited := map[string]bool{}
	var walk func(string) []*javaMethod170
	walk = func(fqcn string) []*javaMethod170 {
		if fqcn == "" || visited[fqcn] { return nil }
		visited[fqcn] = true
		var methods []*javaMethod170
		for _, method := range idx.Methods {
			if method.Owner.FQCN == fqcn && method.Name == name { methods = append(methods, method) }
		}
		if len(methods) > 0 { return methods }
		typ := idx.ByFQCN[fqcn]
		if typ == nil || typ.Super == "" { return methods }
		return walk(idx.resolveTypeName170(typ, typ.Super))
	}
	return walk(ownerFQCN)
}

func (idx *javaIndex170) selectOverload170(caller *javaMethod170, candidates []*javaMethod170, args []string) (*javaMethod170, bool) {
	var arity []*javaMethod170
	for _, candidate := range candidates {
		if len(candidate.Params) == len(args) { arity = append(arity, candidate) }
	}
	if len(arity) == 0 { return nil, true }
	if len(arity) == 1 { return arity[0], false }
	argTypes := make([]string, len(args))
	complete := true
	for i, arg := range args {
		argTypes[i] = idx.inferArgType170(caller, arg)
		if argTypes[i] == "" || argTypes[i] == "<null>" { complete = false }
	}
	if !complete { return nil, true }
	var matches []*javaMethod170
	for _, candidate := range arity {
		match := true
		for i, param := range candidate.Params {
			want := idx.resolveTypeName170(candidate.Owner, param.Type)
			if !javaTypeEqual170(want, argTypes[i]) { match = false; break }
		}
		if match { matches = append(matches, candidate) }
	}
	if len(matches) == 1 { return matches[0], false }
	return nil, true
}

func (idx *javaIndex170) inferArgType170(caller *javaMethod170, arg string) string {
	arg = strings.TrimSpace(arg)
	if arg == "null" { return "<null>" }
	if javaStringRE170.MatchString(arg) { return "java.lang.String" }
	if javaCharRE170.MatchString(arg) { return "char" }
	if javaLongRE170.MatchString(arg) { return "long" }
	if javaIntRE170.MatchString(arg) { return "int" }
	if match := javaNewRE170.FindStringSubmatch(arg); len(match) == 2 {
		return idx.resolveTypeName170(caller.Owner, match[1])
	}
	if identRE.MatchString(arg) {
		if typ, _ := idx.resolveReceiver170(caller, arg); typ != "" { return typ }
	}
	return ""
}

func (idx *javaIndex170) methodRef170(method *javaMethod170, workspace, side string) ReviewRef170 {
	params := make([]string, 0, len(method.Params))
	for _, param := range method.Params {
		params = append(params, idx.resolveTypeName170(method.Owner, param.Type))
	}
	return ReviewRef170{Workspace: workspace, Path: method.Owner.Path, Side: side, Kind: "METHOD", OwnerFQCN: method.Owner.FQCN, Name: method.Name, ParameterTypes: params}
}

func javaCallParts170(text string) (receiver, method string, args []string, ok bool) {
	s := strings.TrimSpace(text)
	open := strings.IndexByte(s, '(')
	if open <= 0 { return "", "", nil, false }
	close := balancedClose(s, open)
	if close < 0 { return "", "", nil, false }
	left := strings.TrimSpace(s[:open])
	if dot := strings.LastIndexByte(left, '.'); dot >= 0 {
		receiver = strings.TrimSpace(left[:dot])
		method = strings.TrimSpace(left[dot+1:])
	} else {
		method = left
	}
	if !identRE.MatchString(method) { return "", "", nil, false }
	if receiver != "" && receiver != "this" && receiver != "super" && !identRE.MatchString(receiver) {
		return "", "", nil, false
	}
	argText := strings.TrimSpace(s[open+1 : close])
	if argText == "" { return receiver, method, []string{}, true }
	return receiver, method, splitJavaTopLevel170(argText, ','), true
}

func relationID170(relation Relation170, raw workspaceRawMatch) string {
	parts := []string{relation.Kind, relation.Resolution, relation.From.Workspace, relation.From.Path, relation.From.OwnerFQCN, relation.From.Name, raw.Path, fmt.Sprint(raw.StartLine), fmt.Sprint(raw.StartColumn), fmt.Sprint(raw.EndLine), fmt.Sprint(raw.EndColumn)}
	for _, target := range relation.Targets {
		key, err := ReviewRefKey170(target)
		if err == nil { parts = append(parts, key) }
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return strings.ToLower(relation.Kind) + ":" + hex.EncodeToString(sum[:])
}

func (n Navigator) javaSourceRange170(ref ReviewRef170, raw workspaceRawMatch) (SourceRange170, error) {
	root := strings.TrimSpace(n.RepoRoot)
	if root == "" { root = "." }
	path := filepath.Join(root, filepath.FromSlash(raw.Path))
	data, err := os.ReadFile(path)
	if err != nil { return SourceRange170{}, err }
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if raw.StartLine < 1 || raw.StartLine > len(lines) || raw.EndLine < 1 || raw.EndLine > len(lines) {
		return SourceRange170{}, fmt.Errorf("JAVA_SOURCE_RANGE_INVALID: AST range outside source")
	}
	startColumn := javaRuneColumn170(lines[raw.StartLine-1], raw.StartColumn-1)
	endColumn := javaRuneColumn170(lines[raw.EndLine-1], raw.EndColumn-1)
	if raw.StartLine == raw.EndLine && endColumn <= startColumn {
		endColumn = startColumn + max170(1, utf8.RuneCountInString(raw.Text))
	}
	return SourceRange170{Ref: ref, StartLine: raw.StartLine, EndLine: raw.EndLine, StartColumn: startColumn, EndColumn: endColumn}, nil
}

func javaRuneColumn170(line string, zeroBasedByte int) int {
	if zeroBasedByte < 0 { zeroBasedByte = 0 }
	if zeroBasedByte > len(line) { zeroBasedByte = len(line) }
	for zeroBasedByte > 0 && zeroBasedByte < len(line) && !utf8.RuneStart(line[zeroBasedByte]) { zeroBasedByte-- }
	return utf8.RuneCountInString(line[:zeroBasedByte]) + 1
}

func max170(a, b int) int { if a > b { return a }; return b }

func appendUniqueString170(in []string, value string) []string {
	for _, current := range in { if current == value { return in } }
	return append(in, value)
}

func (n Navigator) legacyMethodRef170(ctx context.Context, symbol, scope string) (ReviewRef170, *javaIndex170, error) {
	if err := n.validate(symbol, scope); err != nil { return ReviewRef170{}, nil, err }
	owner, name := splitSymbol(symbol)
	if name == "" { return ReviewRef170{}, nil, ErrInvalidSymbol }
	idx, err := n.buildJavaIndex170(ctx)
	if err != nil { return ReviewRef170{}, nil, err }
	var ownerFQCN string
	if strings.Contains(owner, ".") { ownerFQCN = owner } else if matches := idx.SimpleFQCN[owner]; len(matches) == 1 { ownerFQCN = matches[0] }
	if ownerFQCN == "" { return ReviewRef170{}, nil, ErrAmbiguousSymbol }
	var candidates []*javaMethod170
	for _, method := range idx.Methods {
		if method.Owner.FQCN == ownerFQCN && method.Name == name { candidates = append(candidates, method) }
	}
	if len(candidates) == 0 { return ReviewRef170{}, nil, ErrSymbolNotFound }
	if len(candidates) > 1 { return ReviewRef170{}, nil, ErrAmbiguousSymbol }
	ref := idx.methodRef170(candidates[0], "current", "CURRENT")
	return ref, idx, nil
}

func isWorkspaceAvailable170(n Navigator) bool {
	_, _, exists, err := n.workspaceRoots()
	return err == nil && exists
}

var _ = errors.Is
