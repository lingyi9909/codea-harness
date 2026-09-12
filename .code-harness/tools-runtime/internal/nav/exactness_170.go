package nav

import (
	"strings"
	"unicode/utf8"
)

type javaOverloadSelection170 struct {
	Resolution string
	Method     *javaMethod170
	Candidates []*javaMethod170
	Reason     string
}

// finalizeTypeIdentities170 rebuilds type identity after the full AST type set
// is known. Nested types are identified by their nearest enclosing AST type,
// so sibling containers cannot collapse onto package + simple-name identity.
func (idx *javaIndex170) finalizeTypeIdentities170() {
	resolved := map[*javaType170]string{}
	var fqcn func(*javaType170) string
	fqcn = func(typ *javaType170) string {
		if typ == nil {
			return ""
		}
		if value := resolved[typ]; value != "" {
			return value
		}
		if parent := directJavaEnclosingType170(idx.Types, typ); parent != nil {
			if parentFQCN := fqcn(parent); parentFQCN != "" {
				resolved[typ] = parentFQCN + "." + typ.Name
				return resolved[typ]
			}
		}
		if typ.Package != "" {
			resolved[typ] = typ.Package + "." + typ.Name
		} else {
			resolved[typ] = typ.Name
		}
		return resolved[typ]
	}

	idx.ByFQCN = map[string]*javaType170{}
	idx.SimpleFQCN = map[string][]string{}
	for _, typ := range idx.Types {
		typ.FQCN = fqcn(typ)
		if existing := idx.ByFQCN[typ.FQCN]; existing == nil || javaTypeRangeSmaller170(typ, existing) {
			idx.ByFQCN[typ.FQCN] = typ
		}
		idx.SimpleFQCN[typ.Name] = appendUniqueString170(idx.SimpleFQCN[typ.Name], typ.FQCN)
	}
}

func directJavaEnclosingType170(types []*javaType170, child *javaType170) *javaType170 {
	var best *javaType170
	for _, candidate := range types {
		if candidate == nil || candidate == child || candidate.Path != child.Path {
			continue
		}
		if sameJavaRawRange170(candidate.Raw, child.Raw) {
			continue
		}
		if !workspaceTypeContainsRaw(workspaceTypeMatch{
			Path:        candidate.Path,
			StartLine:   candidate.Raw.StartLine,
			StartColumn: candidate.Raw.StartColumn,
			EndLine:     candidate.Raw.EndLine,
			EndColumn:   candidate.Raw.EndColumn,
		}, child.Raw) {
			continue
		}
		if best == nil || javaTypeRangeSmaller170(candidate, best) {
			best = candidate
		}
	}
	return best
}

func sameJavaRawRange170(left, right workspaceRawMatch) bool {
	return left.Path == right.Path &&
		left.StartLine == right.StartLine && left.StartColumn == right.StartColumn &&
		left.EndLine == right.EndLine && left.EndColumn == right.EndColumn
}

func (idx *javaIndex170) resolveTypeNameExact170(owner *javaType170, declared string) string {
	declared = normalizeJavaType170(declared)
	base := strings.TrimSuffix(declared, "[]")
	if base == "" {
		return ""
	}
	if strings.Contains(base, ".") {
		if idx.ByFQCN[base] != nil {
			return base
		}
		if owner != nil && owner.Package != "" {
			candidate := owner.Package + "." + base
			if idx.ByFQCN[candidate] != nil {
				return candidate
			}
		}
		return base
	}
	if owner != nil {
		// A member/nested type in the lexical enclosing chain shadows imports.
		for scope := owner; scope != nil; scope = directJavaEnclosingType170(idx.Types, scope) {
			candidate := scope.FQCN + "." + base
			if idx.ByFQCN[candidate] != nil {
				return candidate
			}
		}
		if imported := owner.Imports[base]; imported != "" {
			return imported
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

func (idx *javaIndex170) resolveReceiverAt170(method *javaMethod170, receiver string, call workspaceRawMatch) (string, string) {
	owner := method.Owner
	switch receiver {
	case "":
		return owner.FQCN, ""
	case "this":
		return owner.FQCN, ""
	case "super":
		if owner.Super == "" {
			return "", "JAVA_RECEIVER_UNRESOLVED: superclass unavailable"
		}
		return idx.resolveTypeName170(owner, owner.Super), ""
	}
	for _, param := range method.Params {
		if param.Name == receiver {
			return idx.resolveTypeName170(owner, param.Type), ""
		}
	}
	if localType, found, uncertain := lexicalJavaLocalType170(method, receiver, call); uncertain {
		return "", "JAVA_RECEIVER_UNRESOLVED: lexical local scope unavailable"
	} else if found {
		return idx.resolveTypeName170(owner, localType), ""
	}
	if fieldType, found, uncertain := lexicalJavaFieldType170(owner, receiver); uncertain {
		return "", "JAVA_RECEIVER_UNRESOLVED: field scope ambiguous"
	} else if found {
		return idx.resolveTypeName170(owner, fieldType), ""
	}
	return "", "JAVA_RECEIVER_UNRESOLVED: receiver declaration unavailable"
}

func lexicalJavaLocalType170(method *javaMethod170, name string, call workspaceRawMatch) (string, bool, bool) {
	callOffset, ok := javaOffsetWithinRaw170(method.Raw, call, method.Text)
	if !ok {
		return "", false, true
	}
	callStack, ok := javaBraceStackAt170(method.Text, callOffset)
	if !ok {
		return "", false, true
	}
	matches := javaLocalRE170.FindAllStringSubmatchIndex(method.Text, -1)
	bestType := ""
	bestDepth, bestEnd := -1, -1
	for _, match := range matches {
		if len(match) < 6 || match[2] < 0 || match[3] < 0 || match[4] < 0 || match[5] < 0 {
			continue
		}
		if method.Text[match[4]:match[5]] != name {
			continue
		}
		declEnd := match[5]
		if declEnd > callOffset {
			continue
		}
		declStack, valid := javaBraceStackAt170(method.Text, declEnd)
		if !valid {
			return "", false, true
		}
		if !javaScopePrefix170(declStack, callStack) {
			continue
		}
		if len(declStack) > bestDepth || (len(declStack) == bestDepth && declEnd > bestEnd) {
			bestType = method.Text[match[2]:match[3]]
			bestDepth = len(declStack)
			bestEnd = declEnd
		}
	}
	if bestDepth < 0 {
		return "", false, false
	}
	return normalizeJavaType170(bestType), true, false
}

func lexicalJavaFieldType170(owner *javaType170, name string) (string, bool, bool) {
	matches := javaFieldRE170.FindAllStringSubmatchIndex(owner.Text, -1)
	found := ""
	for _, match := range matches {
		if len(match) < 6 || match[2] < 0 || match[3] < 0 || match[4] < 0 || match[5] < 0 {
			continue
		}
		if owner.Text[match[4]:match[5]] != name {
			continue
		}
		stack, ok := javaBraceStackAt170(owner.Text, match[5])
		if !ok {
			return "", false, true
		}
		// Type text has one unmatched opening brace at a direct field declaration.
		if len(stack) != 1 {
			continue
		}
		candidate := normalizeJavaType170(owner.Text[match[2]:match[3]])
		if found != "" && found != candidate {
			return "", false, true
		}
		found = candidate
	}
	return found, found != "", false
}

func javaOffsetWithinRaw170(outer, inner workspaceRawMatch, text string) (int, bool) {
	if outer.Path != inner.Path || inner.StartLine < outer.StartLine || inner.EndLine > outer.EndLine {
		return 0, false
	}
	lines := strings.Split(text, "\n")
	relLine := inner.StartLine - outer.StartLine
	if relLine < 0 || relLine >= len(lines) {
		return 0, false
	}
	column := inner.StartColumn
	if relLine == 0 {
		column = inner.StartColumn - outer.StartColumn + 1
	}
	if column < 1 {
		return 0, false
	}
	lineOffset, ok := javaByteOffsetForRuneColumn170(lines[relLine], column)
	if !ok {
		return 0, false
	}
	offset := lineOffset
	for i := 0; i < relLine; i++ {
		offset += len(lines[i]) + 1
	}
	if offset < 0 || offset > len(text) {
		return 0, false
	}
	return offset, true
}

func javaByteOffsetForRuneColumn170(line string, column int) (int, bool) {
	target := column - 1
	if target < 0 {
		return 0, false
	}
	offset := 0
	for i := 0; i < target; i++ {
		if offset >= len(line) {
			return 0, false
		}
		_, size := utf8.DecodeRuneInString(line[offset:])
		if size <= 0 {
			return 0, false
		}
		offset += size
	}
	return offset, true
}

func javaBraceStackAt170(text string, pos int) ([]int, bool) {
	if pos < 0 || pos > len(text) {
		return nil, false
	}
	stack := make([]int, 0)
	for i := 0; i < pos; {
		if i+2 < pos && text[i:i+3] == `"""` {
			// Text blocks require full Java lexical handling; fail closed instead.
			return nil, false
		}
		if i+1 < pos && text[i:i+2] == "//" {
			i += 2
			for i < pos && text[i] != '\n' {
				i++
			}
			continue
		}
		if i+1 < pos && text[i:i+2] == "/*" {
			end := strings.Index(text[i+2:pos], "*/")
			if end < 0 {
				return nil, false
			}
			i += end + 4
			continue
		}
		if text[i] == '"' || text[i] == '\'' {
			quote := text[i]
			i++
			closed := false
			for i < pos {
				if text[i] == '\\' {
					i += 2
					continue
				}
				if text[i] == quote {
					i++
					closed = true
					break
				}
				i++
			}
			if !closed {
				return nil, false
			}
			continue
		}
		switch text[i] {
		case '{':
			stack = append(stack, i)
		case '}':
			if len(stack) == 0 {
				return nil, false
			}
			stack = stack[:len(stack)-1]
		}
		i++
	}
	return stack, true
}

func javaScopePrefix170(scope, at []int) bool {
	if len(scope) > len(at) {
		return false
	}
	for i := range scope {
		if scope[i] != at[i] {
			return false
		}
	}
	return true
}

func (idx *javaIndex170) selectOverloadAt170(caller *javaMethod170, candidates []*javaMethod170, args []string, call workspaceRawMatch) javaOverloadSelection170 {
	arity := make([]*javaMethod170, 0)
	for _, candidate := range candidates {
		if len(candidate.Params) == len(args) {
			arity = append(arity, candidate)
		}
	}
	if len(arity) == 0 {
		return javaOverloadSelection170{Resolution: "UNRESOLVED", Reason: "JAVA_OVERLOAD_UNRESOLVED: no overload with matching arity"}
	}
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = idx.inferArgTypeAt170(caller, arg, call)
		if argTypes[i] == "" {
			return javaOverloadSelection170{Resolution: "UNRESOLVED", Candidates: arity, Reason: "JAVA_OVERLOAD_UNRESOLVED: unsupported argument type inference"}
		}
	}
	matches := make([]*javaMethod170, 0)
	for _, candidate := range arity {
		match := true
		for i, param := range candidate.Params {
			want := idx.resolveTypeName170(candidate.Owner, param.Type)
			if !javaArgumentTypeMatches170(want, argTypes[i]) {
				match = false
				break
			}
		}
		if match {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 1 {
		return javaOverloadSelection170{Resolution: "EXACT", Method: matches[0], Candidates: matches}
	}
	if len(matches) > 1 {
		return javaOverloadSelection170{Resolution: "AMBIGUOUS", Candidates: matches, Reason: "JAVA_OVERLOAD_UNRESOLVED: multiple compatible overloads"}
	}
	return javaOverloadSelection170{Resolution: "UNRESOLVED", Reason: "JAVA_OVERLOAD_UNRESOLVED: no compatible overload"}
}

func (idx *javaIndex170) inferArgTypeAt170(caller *javaMethod170, arg string, call workspaceRawMatch) string {
	arg = strings.TrimSpace(arg)
	if arg == "null" {
		return "<null>"
	}
	if arg == "true" || arg == "false" {
		return "boolean"
	}
	if javaStringRE170.MatchString(arg) {
		return "java.lang.String"
	}
	if javaCharRE170.MatchString(arg) {
		return "char"
	}
	if javaLongRE170.MatchString(arg) {
		return "long"
	}
	if javaIntRE170.MatchString(arg) {
		return "int"
	}
	lower := strings.ToLower(arg)
	if strings.ContainsAny(lower, ".e") {
		if strings.HasSuffix(lower, "f") {
			return "float"
		}
		if strings.HasSuffix(lower, "d") || isSimpleJavaDecimal170(lower) {
			return "double"
		}
	}
	if match := javaNewRE170.FindStringSubmatch(arg); len(match) == 2 {
		return idx.resolveTypeName170(caller.Owner, match[1])
	}
	if arg == "this" {
		return caller.Owner.FQCN
	}
	if identRE.MatchString(arg) {
		if typ, _ := idx.resolveReceiverAt170(caller, arg, call); typ != "" {
			return typ
		}
	}
	return ""
}

func isSimpleJavaDecimal170(value string) bool {
	value = strings.TrimSuffix(strings.TrimSuffix(value, "d"), "f")
	if value == "" {
		return false
	}
	digit := false
	for i, r := range value {
		switch {
		case r >= '0' && r <= '9':
			digit = true
		case r == '_' || r == '.' || r == 'e':
		case (r == '+' || r == '-') && (i == 0 || value[i-1] == 'e'):
		default:
			return false
		}
	}
	return digit
}

func javaArgumentTypeMatches170(want, got string) bool {
	if got == "<null>" {
		return !javaPrimitiveType170(want)
	}
	if javaTypeEqual170(want, got) {
		return true
	}
	boxing := map[string]string{
		"boolean": "java.lang.Boolean",
		"byte":    "java.lang.Byte",
		"short":   "java.lang.Short",
		"int":     "java.lang.Integer",
		"long":    "java.lang.Long",
		"char":    "java.lang.Character",
		"float":   "java.lang.Float",
		"double":  "java.lang.Double",
	}
	if boxing[want] == got || boxing[got] == want {
		return true
	}
	return false
}

func javaPrimitiveType170(value string) bool {
	switch normalizeJavaType170(value) {
	case "boolean", "byte", "short", "int", "long", "char", "float", "double":
		return true
	default:
		return false
	}
}

func springBeanOwnerRegistered170(owner *javaType170) (registered bool, conditional bool) {
	if owner == nil {
		return false, false
	}
	anns := annotations170(owner.Text)
	registered = springHasRecognizedAnnotation170(owner.Imports, anns, "Configuration") || springIsStereotype170(owner.Imports, anns)
	conditional = springHasCondition170(owner.Imports, anns)
	return registered, conditional
}
