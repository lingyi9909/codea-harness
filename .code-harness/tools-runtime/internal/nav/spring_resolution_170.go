package nav

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

type springCandidate170 struct {
	BeanName    string
	Target      ReviewRef170
	BeanType    string
	Primary     bool
	Conditional bool
	Evidence    SourceRange170
}

var springQuotedValueRE170 = regexp.MustCompile(`["']([^"']+)["']`)

var springAnnotationFQCN170 = map[string][]string{
	"Service":       {"org.springframework.stereotype.Service"},
	"Component":     {"org.springframework.stereotype.Component"},
	"Repository":    {"org.springframework.stereotype.Repository"},
	"Controller":    {"org.springframework.stereotype.Controller"},
	"Primary":       {"org.springframework.context.annotation.Primary"},
	"Profile":       {"org.springframework.context.annotation.Profile"},
	"Conditional":   {"org.springframework.context.annotation.Conditional"},
	"Bean":          {"org.springframework.context.annotation.Bean"},
	"Configuration": {"org.springframework.context.annotation.Configuration"},
	"Autowired":     {"org.springframework.beans.factory.annotation.Autowired"},
	"Qualifier":     {"org.springframework.beans.factory.annotation.Qualifier"},
}

func (n Navigator) ResolveInjection170(ctx context.Context, injection Injection170) (Relation170, error) {
	if injection.Owner.Kind != "TYPE" && injection.Owner.Kind != "FIELD" && injection.Owner.Kind != "METHOD" {
		return Relation170{}, fmt.Errorf("SPRING_INJECTION_INVALID: owner ref kind %q", injection.Owner.Kind)
	}
	if _, err := normalizeReviewRef170(injection.Owner); err != nil {
		return Relation170{}, err
	}
	switch injection.Kind {
	case "FIELD", "CONSTRUCTOR", "SETTER":
	default:
		return Relation170{}, fmt.Errorf("SPRING_INJECTION_INVALID: unsupported injection kind %q", injection.Kind)
	}

	idx, err := n.buildJavaIndex170(ctx)
	if err != nil {
		return Relation170{}, err
	}
	owner := idx.ByFQCN[injection.Owner.OwnerFQCN]
	if owner == nil {
		return Relation170{}, ErrSymbolNotFound
	}
	declared := idx.resolveTypeName170(owner, injection.DeclaredType)
	if declared == "" {
		return springUnresolved170(injection, nil, "JAVA_RECEIVER_UNRESOLVED: injection declared type unavailable"), nil
	}
	evidence, err := n.springInjectionEvidence170(injection, owner)
	if err != nil {
		return Relation170{}, err
	}

	ownerAnnotations := springTypeAnnotations170(idx, owner.FQCN)
	if injection.Kind == "CONSTRUCTOR" && !springHasExplicitConstructor170(idx.FileMeta[owner.Path].Source, owner.Name) && springHasAnnotation170(owner.Imports, ownerAnnotations, "RequiredArgsConstructor", "lombok.RequiredArgsConstructor") {
		rel := springUnresolved170(injection, []SourceRange170{evidence}, "GENERATED_CONSTRUCTOR_UNSUPPORTED: Lombok generated constructor is not source-verifiable")
		rel.ID = springRelationID170(rel)
		return rel, nil
	}
	if injection.Kind == "CONSTRUCTOR" && !springHasExplicitConstructor170(idx.FileMeta[owner.Path].Source, owner.Name) {
		rel := springUnresolved170(injection, []SourceRange170{evidence}, "JAVA_RECEIVER_UNRESOLVED: explicit constructor unavailable")
		rel.ID = springRelationID170(rel)
		return rel, nil
	}
	if injection.Kind == "SETTER" && !springHasSetter170(idx.FileMeta[owner.Path].Source, injection.Name) {
		rel := springUnresolved170(injection, []SourceRange170{evidence}, "JAVA_RECEIVER_UNRESOLVED: setter declaration unavailable")
		rel.ID = springRelationID170(rel)
		return rel, nil
	}

	candidates, err := n.springCandidates170(idx, declared, injection.Owner.Workspace, injection.Owner.Side)
	if err != nil {
		return Relation170{}, err
	}
	selected := candidates
	selector := strings.TrimSpace(injection.Qualifier)
	if selector == "" {
		selector = strings.TrimSpace(injection.ResourceName)
	}
	if selector != "" {
		filtered := make([]springCandidate170, 0)
		for _, candidate := range candidates {
			if candidate.BeanName == selector {
				filtered = append(filtered, candidate)
			}
		}
		selected = filtered
	}

	rel := Relation170{Kind: "SPRING_BINDING", From: injection.Owner, Targets: []ReviewRef170{}, Evidence: []SourceRange170{evidence}, Assumptions: []string{}}
	if len(selected) == 0 {
		rel.Resolution = "UNRESOLVED"
		rel.Reason = "SPRING_BEAN_AMBIGUOUS: no registered bean matches injection"
		rel.ID = springRelationID170(rel)
		return rel, nil
	}

	if selector == "" && len(selected) > 1 {
		primaries := make([]springCandidate170, 0)
		for _, candidate := range selected {
			if candidate.Primary {
				primaries = append(primaries, candidate)
			}
		}
		if len(primaries) == 1 {
			selected = primaries
		} else if len(primaries) > 1 {
			for _, candidate := range primaries {
				rel.Targets = append(rel.Targets, candidate.Target)
			}
			rel.Resolution = "AMBIGUOUS"
			rel.Reason = "SPRING_BEAN_AMBIGUOUS: multiple @Primary beans"
			rel.ID = springRelationID170(rel)
			return rel, nil
		}
	}
	if len(selected) > 1 {
		for _, candidate := range selected {
			rel.Targets = append(rel.Targets, candidate.Target)
		}
		rel.Resolution = "AMBIGUOUS"
		rel.Reason = "SPRING_BEAN_AMBIGUOUS: multiple registered beans match injection"
		rel.ID = springRelationID170(rel)
		return rel, nil
	}

	candidate := selected[0]
	rel.Targets = []ReviewRef170{candidate.Target}
	if candidate.Conditional {
		rel.Resolution = "CONDITIONAL"
		rel.Reason = "SPRING_CONDITION_UNRESOLVED: bean registration has unresolved profile/condition"
	} else {
		rel.Resolution = "EXACT"
	}
	rel.ID = springRelationID170(rel)
	return rel, nil
}

func (n Navigator) springCandidates170(idx *javaIndex170, declared, workspace, side string) ([]springCandidate170, error) {
	byKey := map[string]springCandidate170{}
	for _, typ := range idx.Types {
		anns := annotations170(typ.Text)
		if !springIsStereotype170(typ.Imports, anns) {
			continue
		}
		if !idx.springAssignable170(typ.FQCN, declared, map[string]bool{}) {
			continue
		}
		candidate := springCandidate170{
			BeanName:    springBeanName170(anns, typ.Name),
			BeanType:    typ.FQCN,
			Primary:     springHasRecognizedAnnotation170(typ.Imports, anns, "Primary"),
			Conditional: springHasCondition170(typ.Imports, anns),
			Target:      ReviewRef170{Workspace: workspace, Path: typ.Path, Side: side, Kind: "TYPE", OwnerFQCN: typ.FQCN, Name: typ.Name, ParameterTypes: []string{}},
		}
		rangeRef, err := n.springTextEvidence170(candidate.Target, typ.Path, typ.Text, typ.Raw.StartLine, typ.Raw.StartColumn)
		if err != nil {
			return nil, err
		}
		candidate.Evidence = rangeRef
		key, keyErr := ReviewRefKey170(candidate.Target)
		if keyErr != nil {
			return nil, keyErr
		}
		key += "\x00" + candidate.BeanName
		if prior, exists := byKey[key]; exists {
			candidate.Primary = candidate.Primary || prior.Primary
			candidate.Conditional = candidate.Conditional || prior.Conditional
		}
		byKey[key] = candidate
	}

	for _, method := range idx.Methods {
		anns := annotations170(method.Text)
		if !springHasRecognizedAnnotation170(method.Owner.Imports, anns, "Bean") {
			continue
		}
		returnType := idx.resolveTypeName170(method.Owner, method.ReturnType)
		if returnType == "" || !idx.springTypeCompatible170(returnType, declared) {
			continue
		}
		candidate := springCandidate170{
			BeanName:    springBeanName170(anns, method.Name),
			BeanType:    returnType,
			Primary:     springHasRecognizedAnnotation170(method.Owner.Imports, anns, "Primary"),
			Conditional: springHasCondition170(method.Owner.Imports, anns) || springHasCondition170(method.Owner.Imports, springTypeAnnotations170(idx, method.Owner.FQCN)),
			Target:      idx.methodRef170(method, workspace, side),
		}
		rangeRef, err := n.springTextEvidence170(candidate.Target, method.Raw.Path, method.Text, method.Raw.StartLine, method.Raw.StartColumn)
		if err != nil {
			return nil, err
		}
		candidate.Evidence = rangeRef
		key, keyErr := ReviewRefKey170(candidate.Target)
		if keyErr != nil {
			return nil, keyErr
		}
		key += "\x00" + candidate.BeanName
		if prior, exists := byKey[key]; exists {
			candidate.Primary = candidate.Primary || prior.Primary
			candidate.Conditional = candidate.Conditional || prior.Conditional
		}
		byKey[key] = candidate
	}

	out := make([]springCandidate170, 0, len(byKey))
	for _, candidate := range byKey {
		out = append(out, candidate)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].BeanName != out[j].BeanName {
			return out[i].BeanName < out[j].BeanName
		}
		if out[i].Target.OwnerFQCN != out[j].Target.OwnerFQCN {
			return out[i].Target.OwnerFQCN < out[j].Target.OwnerFQCN
		}
		return out[i].Target.Name < out[j].Target.Name
	})
	return out, nil
}

func springTypeAnnotations170(idx *javaIndex170, fqcn string) []string {
	seen := map[string]bool{}
	var out []string
	for _, typ := range idx.Types {
		if typ.FQCN != fqcn {
			continue
		}
		for _, ann := range annotations170(typ.Text) {
			if !seen[ann] {
				seen[ann] = true
				out = append(out, ann)
			}
		}
	}
	sort.Strings(out)
	return out
}

func (idx *javaIndex170) springAssignable170(candidateFQCN, declared string, visiting map[string]bool) bool {
	if idx.springTypeCompatible170(candidateFQCN, declared) {
		return true
	}
	if visiting[candidateFQCN] {
		return false
	}
	visiting[candidateFQCN] = true
	typ := idx.ByFQCN[candidateFQCN]
	if typ == nil {
		return false
	}
	for _, iface := range typ.Interfaces {
		fqcn := idx.resolveTypeName170(typ, iface)
		if idx.springTypeCompatible170(fqcn, declared) || idx.springAssignable170(fqcn, declared, visiting) {
			return true
		}
	}
	if typ.Super != "" {
		fqcn := idx.resolveTypeName170(typ, typ.Super)
		if idx.springTypeCompatible170(fqcn, declared) || idx.springAssignable170(fqcn, declared, visiting) {
			return true
		}
	}
	return false
}

func (idx *javaIndex170) springTypeCompatible170(candidate, declared string) bool {
	return javaTypeEqual170(candidate, declared)
}

func springIsStereotype170(imports map[string]string, anns []string) bool {
	for _, name := range []string{"Service", "Component", "Repository", "Controller"} {
		if springHasRecognizedAnnotation170(imports, anns, name) {
			return true
		}
	}
	return false
}

func springHasRecognizedAnnotation170(imports map[string]string, anns []string, simple string) bool {
	for _, fqcn := range springAnnotationFQCN170[simple] {
		if springHasAnnotation170(imports, anns, simple, fqcn) {
			return true
		}
	}
	return false
}

func springHasAnnotation170(imports map[string]string, anns []string, simple, fqcn string) bool {
	for _, ann := range anns {
		name := strings.TrimPrefix(strings.TrimSpace(ann), "@")
		if i := strings.IndexByte(name, '('); i >= 0 {
			name = strings.TrimSpace(name[:i])
		}
		if name == fqcn {
			return true
		}
		if name == simple && imports[simple] == fqcn {
			return true
		}
	}
	return false
}

func springHasCondition170(imports map[string]string, anns []string) bool {
	if springHasRecognizedAnnotation170(imports, anns, "Profile") || springHasRecognizedAnnotation170(imports, anns, "Conditional") {
		return true
	}
	for _, ann := range anns {
		name := strings.TrimPrefix(strings.TrimSpace(ann), "@")
		if i := strings.IndexByte(name, '('); i >= 0 {
			name = strings.TrimSpace(name[:i])
		}
		simple := name
		if i := strings.LastIndexByte(simple, '.'); i >= 0 {
			simple = simple[i+1:]
		}
		if strings.HasPrefix(simple, "ConditionalOn") {
			fqcn := imports[simple]
			if strings.HasPrefix(fqcn, "org.springframework.boot.autoconfigure.condition.") || strings.HasPrefix(name, "org.springframework.boot.autoconfigure.condition.") {
				return true
			}
		}
	}
	return false
}

func springBeanName170(anns []string, fallback string) string {
	for _, ann := range anns {
		name := strings.TrimPrefix(strings.TrimSpace(ann), "@")
		simple := name
		if i := strings.IndexByte(simple, '('); i >= 0 {
			simple = simple[:i]
		}
		if i := strings.LastIndexByte(simple, '.'); i >= 0 {
			simple = simple[i+1:]
		}
		switch simple {
		case "Service", "Component", "Repository", "Controller", "Bean":
			if match := springQuotedValueRE170.FindStringSubmatch(ann); len(match) == 2 && strings.TrimSpace(match[1]) != "" {
				return strings.TrimSpace(match[1])
			}
		}
	}
	if fallback == "" {
		return ""
	}
	runes := []rune(fallback)
	if len(runes) == 0 {
		return ""
	}
	runes[0] = []rune(strings.ToLower(string(runes[0])))[0]
	return string(runes)
}

func springHasExplicitConstructor170(source, typeName string) bool {
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(typeName) + `\s*\(`)
	return pattern.FindStringIndex(source) != nil
}

func springHasSetter170(source, name string) bool {
	if name == "" {
		return false
	}
	runes := []rune(name)
	runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
	setter := "set" + string(runes)
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(setter) + `\s*\(`)
	return pattern.FindStringIndex(source) != nil
}

func (n Navigator) springInjectionEvidence170(injection Injection170, owner *javaType170) (SourceRange170, error) {
	if len(injection.Evidence) > 0 {
		if err := validateSourceRange170(injection.Evidence[0]); err != nil {
			return SourceRange170{}, err
		}
		return injection.Evidence[0], nil
	}
	needle := injection.Name
	if injection.Kind == "CONSTRUCTOR" {
		needle = owner.Name
	}
	if injection.Kind == "SETTER" {
		runes := []rune(injection.Name)
		if len(runes) > 0 {
			runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
			needle = "set" + string(runes)
		}
	}
	return n.springTextEvidence170(injection.Owner, owner.Path, owner.Text, owner.Raw.StartLine, owner.Raw.StartColumn, needle)
}

func (n Navigator) springTextEvidence170(ref ReviewRef170, path, text string, startLine, startColumn int, needles ...string) (SourceRange170, error) {
	root := strings.TrimSpace(n.RepoRoot)
	if root == "" {
		root = "."
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return SourceRange170{}, err
	}
	source := strings.ReplaceAll(string(data), "\r\n", "\n")
	needle := ""
	for _, candidate := range needles {
		if strings.TrimSpace(candidate) != "" {
			needle = candidate
			break
		}
	}
	if needle == "" {
		needle = strings.TrimSpace(text)
		if len(needle) > 32 {
			needle = needle[:32]
		}
	}
	byteIndex := strings.Index(source, needle)
	if byteIndex < 0 {
		if startLine < 1 {
			startLine = 1
		}
		if startColumn < 1 {
			startColumn = 1
		}
		return SourceRange170{Ref: ref, StartLine: startLine, EndLine: startLine, StartColumn: startColumn, EndColumn: startColumn + 1}, nil
	}
	prefix := source[:byteIndex]
	line := strings.Count(prefix, "\n") + 1
	lineStart := strings.LastIndex(prefix, "\n") + 1
	column := utf8.RuneCountInString(source[lineStart:byteIndex]) + 1
	end := column + max170(1, utf8.RuneCountInString(needle))
	return SourceRange170{Ref: ref, StartLine: line, EndLine: line, StartColumn: column, EndColumn: end}, nil
}

func springUnresolved170(injection Injection170, evidence []SourceRange170, reason string) Relation170 {
	if evidence == nil {
		evidence = []SourceRange170{}
	}
	return Relation170{Kind: "SPRING_BINDING", Resolution: "UNRESOLVED", From: injection.Owner, Targets: []ReviewRef170{}, Evidence: evidence, Reason: reason, Assumptions: []string{}}
}

func springRelationID170(rel Relation170) string {
	raw := workspaceRawMatch{Path: rel.From.Path}
	if len(rel.Evidence) > 0 {
		raw.StartLine, raw.StartColumn = rel.Evidence[0].StartLine, rel.Evidence[0].StartColumn
		raw.EndLine, raw.EndColumn = rel.Evidence[0].EndLine, rel.Evidence[0].EndColumn
	}
	return relationID170(rel, raw)
}
