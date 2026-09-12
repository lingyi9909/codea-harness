package reviewcontext

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"codea-harness-tools/internal/nav"
)

type dubboAnnotation170 struct {
	Name  string
	Args  string
	Start int
	End   int
}

type dubboReference170 struct {
	InterfaceFQCN string
	Field         string
	GroupRaw      string
	Group         string
	VersionRaw    string
	Version       string
	Resolved      bool
}

type dubboMethod170 struct {
	Name           string
	ParameterTypes []string
	ParameterNames []string
	Start          int
	End            int
	BodyStart      int
	BodyEnd        int
}

type dubboProviderMethod170 struct {
	Workspace      string
	Path           string
	OwnerFQCN      string
	InterfaceFQCN  string
	Name           string
	ParameterTypes []string
	GroupRaw       string
	Group          string
	VersionRaw     string
	Version        string
	ConfigResolved bool
	Start          int
	End            int
	Data           []byte
}

type dubboRoot170 struct {
	Workspace string
	Root      string
}

var (
	dubboPackageRE170 = regexp.MustCompile(`(?m)^\s*package\s+([A-Za-z_$][A-Za-z0-9_$.]*)\s*;`)
	dubboImportRE170  = regexp.MustCompile(`(?m)^\s*import\s+([A-Za-z_$][A-Za-z0-9_$.]*)\s*;`)
	dubboFieldRE170   = regexp.MustCompile(`(?s)^\s*(?:(?:public|protected|private|static|final|volatile|transient)\s+)*([A-Za-z_$][A-Za-z0-9_$.<>?\[\]]*)\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*;`)
	dubboClassRE170   = regexp.MustCompile(`(?s)^\s*(?:(?:public|protected|private|abstract|final|static)\s+)*class\s+([A-Za-z_$][A-Za-z0-9_$]*)(?:\s+extends\s+[A-Za-z0-9_$.<>?]+)?(?:\s+implements\s+([^\{]+))?\s*\{`)
	dubboAttrStringRE170 = regexp.MustCompile(`([A-Za-z_$][A-Za-z0-9_$]*)\s*=\s*"([^"]*)"`)
	dubboAttrClassRE170  = regexp.MustCompile(`([A-Za-z_$][A-Za-z0-9_$]*)\s*=\s*([A-Za-z_$][A-Za-z0-9_$.]*)\s*\.class`)
	dubboIntRE170        = regexp.MustCompile(`^-?[0-9]+$`)
	dubboLongRE170       = regexp.MustCompile(`^-?[0-9]+[lL]$`)
)

var dubboReferenceAnnotations170 = map[string]bool{
	"org.apache.dubbo.config.annotation.DubboReference": true,
	"org.apache.dubbo.config.annotation.Reference":      true,
	"com.alibaba.dubbo.config.annotation.Reference":     true,
}

var dubboServiceAnnotations170 = map[string]bool{
	"org.apache.dubbo.config.annotation.DubboService": true,
	"org.apache.dubbo.config.annotation.Service":      true,
	"com.alibaba.dubbo.config.annotation.Service":     true,
}

func resolveDubboContract170(ctx context.Context, currentRoot string, consumer nav.ReviewRef170, providers []ProviderRoot170) ([]nav.Relation170, []nav.Issue170, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if consumer.Kind != "METHOD" {
		return nil, nil, fmt.Errorf("DUBBO_CONSUMER_INVALID: expected METHOD ref")
	}
	if _, err := nav.ReviewRefKey170(consumer); err != nil {
		return nil, nil, fmt.Errorf("DUBBO_CONSUMER_INVALID: %w", err)
	}
	root, err := filepath.Abs(strings.TrimSpace(currentRoot))
	if err != nil {
		return nil, nil, err
	}
	root = filepath.Clean(root)
	consumerPath, err := myBatisRepoFile170(root, consumer.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("DUBBO_CONSUMER_INVALID: %w", err)
	}
	data, err := os.ReadFile(consumerPath)
	if err != nil {
		return nil, nil, fmt.Errorf("DUBBO_CONSUMER_UNAVAILABLE: %w", err)
	}
	info, err := os.Lstat(consumerPath)
	if err != nil {
		return nil, nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("DUBBO_CONSUMER_INVALID: source is not a regular file")
	}

	pkg := dubboPackage170(data)
	imports := dubboImports170(data)
	classStart, classEnd, ok := dubboOwnerRange170(data, dubboSimpleName170(consumer.OwnerFQCN))
	if !ok {
		return []nav.Relation170{dubboUnresolvedRelation170(consumer, nil, "DUBBO_CONSUMER_UNRESOLVED: owner type was not found")}, []nav.Issue170{}, nil
	}
	caller, ok := dubboSelectMethod170(dubboMethods170(data, classStart, classEnd, consumer.Name, pkg, imports), consumer.ParameterTypes)
	if !ok {
		return []nav.Relation170{dubboUnresolvedRelation170(consumer, nil, "DUBBO_CONSUMER_UNRESOLVED: consumer method signature was not uniquely resolved")}, []nav.Issue170{}, nil
	}
	consumerEvidence := dubboRange170(data, consumer, caller.Start, caller.End)
	references := dubboReferences170(data, classStart, classEnd, pkg, imports, dubboLocalProperties170(root))
	callSpan, reference, remoteMethod, argTypes, ok := dubboRemoteCall170(data, caller, references)
	if !ok {
		relation := dubboUnresolvedRelation170(consumer, []nav.SourceRange170{consumerEvidence}, "DUBBO_REFERENCE_UNRESOLVED: no recognized Dubbo reference invocation")
		return []nav.Relation170{relation}, []nav.Issue170{}, nil
	}
	callEvidence := dubboRange170(data, consumer, callSpan[0], callSpan[1])
	evidence := []nav.SourceRange170{consumerEvidence, callEvidence}
	if !reference.Resolved {
		reason := "DUBBO_CONFIG_UNRESOLVED: consumer group/version is missing, wildcard, or unresolved"
		relation := dubboUnresolvedRelation170(consumer, evidence, reason)
		return []nav.Relation170{relation}, []nav.Issue170{{Code: "DUBBO_CONFIG_UNRESOLVED", At: callEvidence, Detail: reason}}, nil
	}

	roots := []dubboRoot170{{Workspace: consumer.Workspace, Root: root}}
	seenRoots := map[string]bool{strings.ToLower(filepath.Clean(root)): true}
	rejectedRoot := false
	for _, provider := range providers {
		if !provider.verified || strings.TrimSpace(provider.Workspace) == "" || strings.TrimSpace(provider.Root) == "" {
			rejectedRoot = true
			continue
		}
		abs, absErr := filepath.Abs(provider.Root)
		if absErr != nil {
			rejectedRoot = true
			continue
		}
		abs = filepath.Clean(abs)
		key := strings.ToLower(abs)
		if seenRoots[key] {
			continue
		}
		seenRoots[key] = true
		roots = append(roots, dubboRoot170{Workspace: strings.TrimSpace(provider.Workspace), Root: abs})
	}

	allCandidates := []dubboProviderMethod170{}
	matching := []dubboProviderMethod170{}
	providerConfigUnknown := false
	for _, providerRoot := range roots {
		candidates, scanErr := dubboProviderMethods170(ctx, providerRoot, reference.InterfaceFQCN, remoteMethod)
		if scanErr != nil {
			return nil, nil, scanErr
		}
		allCandidates = append(allCandidates, candidates...)
		for _, candidate := range candidates {
			if !candidate.ConfigResolved {
				providerConfigUnknown = true
				continue
			}
			if candidate.Group != reference.Group || candidate.Version != reference.Version {
				continue
			}
			if !dubboSignatureCompatible170(argTypes, candidate.ParameterTypes) {
				continue
			}
			matching = append(matching, candidate)
		}
	}

	assumptions := []string{
		"dubbo.interface=" + reference.InterfaceFQCN,
		"dubbo.method=" + remoteMethod,
		"dubbo.group=" + reference.Group,
		"dubbo.version=" + reference.Version,
		"dubbo.runtimeRouteVerified=false",
	}
	if reference.GroupRaw != reference.Group {
		assumptions = append(assumptions, "dubbo.group.raw="+reference.GroupRaw)
	}
	if reference.VersionRaw != reference.Version {
		assumptions = append(assumptions, "dubbo.version.raw="+reference.VersionRaw)
	}
	assumptions = uniqueSortedMyBatis170(assumptions)

	if len(matching) == 1 {
		candidate := matching[0]
		target := dubboProviderRef170(candidate, consumer.Side)
		targetEvidence := dubboRange170(candidate.Data, target, candidate.Start, candidate.End)
		relation := nav.Relation170{
			Kind: "DUBBO_CONTRACT", Resolution: "EXACT", From: consumer,
			Targets: []nav.ReviewRef170{target}, Evidence: append(evidence, targetEvidence), Assumptions: assumptions,
		}
		relation.ID = dubboRelationID170(relation)
		return []nav.Relation170{relation}, []nav.Issue170{}, nil
	}
	if len(matching) > 1 {
		targets := make([]nav.ReviewRef170, 0, len(matching))
		ambiguousEvidence := append([]nav.SourceRange170{}, evidence...)
		for _, candidate := range matching {
			target := dubboProviderRef170(candidate, consumer.Side)
			targets = append(targets, target)
			ambiguousEvidence = append(ambiguousEvidence, dubboRange170(candidate.Data, target, candidate.Start, candidate.End))
		}
		sort.Slice(targets, func(i, j int) bool {
			left, _ := nav.ReviewRefKey170(targets[i])
			right, _ := nav.ReviewRefKey170(targets[j])
			return left < right
		})
		relation := nav.Relation170{
			Kind: "DUBBO_CONTRACT", Resolution: "AMBIGUOUS", From: consumer, Targets: targets,
			Evidence: ambiguousEvidence, Reason: "DUBBO_PROVIDER_AMBIGUOUS: multiple verified providers satisfy the source contract", Assumptions: assumptions,
		}
		relation.ID = dubboRelationID170(relation)
		return []nav.Relation170{relation}, []nav.Issue170{}, nil
	}
	if providerConfigUnknown {
		reason := "DUBBO_CONFIG_UNRESOLVED: provider group/version is missing, wildcard, or unresolved"
		relation := dubboUnresolvedRelation170(consumer, evidence, reason)
		relation.Assumptions = assumptions
		relation.ID = dubboRelationID170(relation)
		return []nav.Relation170{relation}, []nav.Issue170{{Code: "DUBBO_CONFIG_UNRESOLVED", At: callEvidence, Detail: reason}}, nil
	}
	if len(allCandidates) > 0 {
		reason := "DUBBO_CONTRACT_UNRESOLVED: provider interface/method exists but group, version, or method signature does not match"
		relation := dubboUnresolvedRelation170(consumer, evidence, reason)
		relation.Assumptions = assumptions
		relation.ID = dubboRelationID170(relation)
		return []nav.Relation170{relation}, []nav.Issue170{}, nil
	}

	reason := "PROVIDER_SOURCE_UNAVAILABLE: no matching provider exists in current source or verified workspace dependencies"
	if rejectedRoot {
		reason = "PROVIDER_SOURCE_UNAVAILABLE: unverified provider roots were excluded"
	}
	relation := dubboUnresolvedRelation170(consumer, evidence, reason)
	relation.Assumptions = assumptions
	relation.ID = dubboRelationID170(relation)
	return []nav.Relation170{relation}, []nav.Issue170{{Code: "PROVIDER_SOURCE_UNAVAILABLE", At: callEvidence, Detail: reason}}, nil
}

func dubboReferences170(data []byte, start, end int, pkg string, imports, props map[string]string) []dubboReference170 {
	annotations := dubboAnnotations170(data, start, end, imports, dubboReferenceAnnotations170)
	out := []dubboReference170{}
	for _, annotation := range annotations {
		match := dubboFieldRE170.FindSubmatch(data[annotation.End:end])
		if len(match) != 3 {
			continue
		}
		attrs := dubboAnnotationAttrs170(annotation.Args)
		groupRaw, versionRaw := attrs["group"], attrs["version"]
		group, groupOK := dubboResolveConfigValue170(groupRaw, props)
		version, versionOK := dubboResolveConfigValue170(versionRaw, props)
		out = append(out, dubboReference170{
			InterfaceFQCN: dubboResolveType170(string(match[1]), pkg, imports), Field: string(match[2]),
			GroupRaw: groupRaw, Group: group, VersionRaw: versionRaw, Version: version, Resolved: groupOK && versionOK,
		})
	}
	return out
}

func dubboRemoteCall170(data []byte, caller dubboMethod170, refs []dubboReference170) ([2]int, dubboReference170, string, []string, bool) {
	var none [2]int
	if caller.BodyStart < 0 || caller.BodyEnd <= caller.BodyStart || caller.BodyEnd > len(data) {
		return none, dubboReference170{}, "", nil, false
	}
	body := string(data[caller.BodyStart:caller.BodyEnd])
	paramTypes := map[string]string{}
	for i, name := range caller.ParameterNames {
		if i < len(caller.ParameterTypes) {
			paramTypes[name] = caller.ParameterTypes[i]
		}
	}
	for _, ref := range refs {
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(ref.Field) + `\s*\.\s*([A-Za-z_$][A-Za-z0-9_$]*)\s*\(([^)]*)\)`)
		loc := re.FindStringSubmatchIndex(body)
		if len(loc) < 6 {
			continue
		}
		method := body[loc[2]:loc[3]]
		argsText := body[loc[4]:loc[5]]
		argTypes := []string{}
		for _, arg := range dubboSplitComma170(argsText) {
			argTypes = append(argTypes, dubboInferArgumentType170(strings.TrimSpace(arg), paramTypes))
		}
		return [2]int{caller.BodyStart + loc[0], caller.BodyStart + loc[1]}, ref, method, argTypes, true
	}
	return none, dubboReference170{}, "", nil, false
}

func dubboProviderMethods170(ctx context.Context, root dubboRoot170, interfaceFQCN, methodName string) ([]dubboProviderMethod170, error) {
	sourceRoot := filepath.Join(root.Root, "src", "main", "java")
	info, err := os.Stat(sourceRoot)
	if err != nil || !info.IsDir() {
		return []dubboProviderMethod170{}, nil
	}
	paths := []string{}
	err = filepath.WalkDir(sourceRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".java") {
			return nil
		}
		entryInfo, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		if entryInfo.Mode().IsRegular() {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	props := dubboLocalProperties170(root.Root)
	out := []dubboProviderMethod170{}
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, readErr
		}
		pkg := dubboPackage170(data)
		imports := dubboImports170(data)
		for _, annotation := range dubboAnnotations170(data, 0, len(data), imports, dubboServiceAnnotations170) {
			classMatch := dubboClassRE170.FindSubmatchIndex(data[annotation.End:])
			if len(classMatch) == 0 || classMatch[0] != 0 {
				continue
			}
			owner := string(data[annotation.End+classMatch[2] : annotation.End+classMatch[3]])
			implementsText := ""
			if classMatch[4] >= 0 && classMatch[5] >= 0 {
				implementsText = string(data[annotation.End+classMatch[4] : annotation.End+classMatch[5]])
			}
			attrs := dubboAnnotationAttrs170(annotation.Args)
			providerInterface := dubboProviderInterface170(attrs, implementsText, pkg, imports, interfaceFQCN)
			if providerInterface != interfaceFQCN {
				continue
			}
			groupRaw, versionRaw := attrs["group"], attrs["version"]
			group, groupOK := dubboResolveConfigValue170(groupRaw, props)
			version, versionOK := dubboResolveConfigValue170(versionRaw, props)
			open := annotation.End + classMatch[1] - 1
			if open < 0 || open >= len(data) || data[open] != '{' {
				continue
			}
			close, braceOK := dubboMatchingBrace170(data, open)
			if !braceOK {
				continue
			}
			methods := dubboMethods170(data, open+1, close, methodName, pkg, imports)
			rel, relErr := filepath.Rel(root.Root, path)
			if relErr != nil {
				return nil, relErr
			}
			for _, method := range methods {
				out = append(out, dubboProviderMethod170{
					Workspace: root.Workspace, Path: filepath.ToSlash(rel), OwnerFQCN: dubboJoinPackage170(pkg, owner),
					InterfaceFQCN: providerInterface, Name: methodName, ParameterTypes: method.ParameterTypes,
					GroupRaw: groupRaw, Group: group, VersionRaw: versionRaw, Version: version,
					ConfigResolved: groupOK && versionOK, Start: method.Start, End: method.End, Data: data,
				})
			}
		}
	}
	return out, nil
}

func dubboProviderInterface170(attrs map[string]string, implementsText, pkg string, imports map[string]string, wanted string) string {
	if value := attrs["interfaceName"]; value != "" {
		return dubboResolveType170(value, pkg, imports)
	}
	if value := attrs["interfaceClass"]; value != "" {
		return dubboResolveType170(value, pkg, imports)
	}
	for _, raw := range dubboSplitComma170(implementsText) {
		candidate := dubboResolveType170(strings.TrimSpace(raw), pkg, imports)
		if candidate == wanted {
			return candidate
		}
	}
	return ""
}

func dubboMethods170(data []byte, start, end int, name, pkg string, imports map[string]string) []dubboMethod170 {
	if start < 0 {
		start = 0
	}
	if end > len(data) {
		end = len(data)
	}
	if end <= start {
		return nil
	}
	pattern := regexp.MustCompile(`(?s)(?:(?:public|protected|private|static|final|abstract|synchronized|native|default)\s+)*([A-Za-z_$][A-Za-z0-9_$.<>?\[\]]*)\s+` + regexp.QuoteMeta(name) + `\s*\(([^)]*)\)\s*(?:throws\s+[^\{;]+)?([\{;])`)
	matches := pattern.FindAllSubmatchIndex(data[start:end], -1)
	out := []dubboMethod170{}
	for _, match := range matches {
		if len(match) < 8 {
			continue
		}
		paramsText := string(data[start+match[4] : start+match[5]])
		paramTypes, paramNames := dubboParseParameters170(paramsText, pkg, imports)
		method := dubboMethod170{Name: name, ParameterTypes: paramTypes, ParameterNames: paramNames, Start: start + match[0]}
		terminal := start + match[6]
		if terminal < 0 || terminal >= len(data) {
			continue
		}
		if data[terminal] == ';' {
			method.End, method.BodyStart, method.BodyEnd = terminal+1, -1, -1
		} else {
			close, ok := dubboMatchingBrace170(data, terminal)
			if !ok || close > end {
				continue
			}
			method.BodyStart, method.BodyEnd, method.End = terminal+1, close, close+1
		}
		out = append(out, method)
	}
	return out
}

func dubboSelectMethod170(methods []dubboMethod170, wanted []string) (dubboMethod170, bool) {
	if len(methods) == 1 && len(wanted) == 0 {
		return methods[0], true
	}
	matches := []dubboMethod170{}
	for _, method := range methods {
		if len(wanted) != len(method.ParameterTypes) {
			continue
		}
		match := true
		for i := range wanted {
			if !dubboTypeEqual170(wanted[i], method.ParameterTypes[i]) {
				match = false
				break
			}
		}
		if match {
			matches = append(matches, method)
		}
	}
	if len(matches) == 1 {
		return matches[0], true
	}
	return dubboMethod170{}, false
}

func dubboSignatureCompatible170(args, params []string) bool {
	if len(args) != len(params) {
		return false
	}
	for i := range args {
		if args[i] == "" || !dubboTypeEqual170(args[i], params[i]) {
			return false
		}
	}
	return true
}

func dubboParseParameters170(raw, pkg string, imports map[string]string) ([]string, []string) {
	parts := dubboSplitComma170(raw)
	types, names := []string{}, []string{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields := strings.Fields(part)
		clean := []string{}
		for _, field := range fields {
			if strings.HasPrefix(field, "@") || field == "final" {
				continue
			}
			clean = append(clean, field)
		}
		if len(clean) < 2 {
			return nil, nil
		}
		name := clean[len(clean)-1]
		typeName := strings.Join(clean[:len(clean)-1], " ")
		types = append(types, dubboResolveType170(typeName, pkg, imports))
		names = append(names, strings.TrimSuffix(name, "[]"))
	}
	return types, names
}

func dubboInferArgumentType170(arg string, params map[string]string) string {
	if value, ok := params[arg]; ok {
		return value
	}
	if len(arg) >= 2 && arg[0] == '"' && arg[len(arg)-1] == '"' {
		return "java.lang.String"
	}
	if arg == "true" || arg == "false" {
		return "boolean"
	}
	if dubboLongRE170.MatchString(arg) {
		return "long"
	}
	if dubboIntRE170.MatchString(arg) {
		return "int"
	}
	if strings.HasSuffix(arg, ".class") {
		return "java.lang.Class"
	}
	return ""
}

func dubboAnnotations170(data []byte, start, end int, imports map[string]string, allowed map[string]bool) []dubboAnnotation170 {
	if start < 0 {
		start = 0
	}
	if end > len(data) {
		end = len(data)
	}
	out := []dubboAnnotation170{}
	for i := start; i < end; {
		if data[i] == '/' && i+1 < end && data[i+1] == '/' {
			i += 2
			for i < end && data[i] != '\n' {
				i++
			}
			continue
		}
		if data[i] == '/' && i+1 < end && data[i+1] == '*' {
			i += 2
			for i+1 < end && !(data[i] == '*' && data[i+1] == '/') {
				i++
			}
			if i+1 < end {
				i += 2
			}
			continue
		}
		if data[i] == '"' || data[i] == '\'' {
			quote := data[i]
			i++
			for i < end {
				if data[i] == '\\' {
					i += 2
					continue
				}
				if i < end && data[i] == quote {
					i++
					break
				}
				i++
			}
			continue
		}
		if data[i] != '@' {
			i++
			continue
		}
		annStart := i
		i++
		nameStart := i
		for i < end && (dubboIdentByte170(data[i]) || data[i] == '.') {
			i++
		}
		if nameStart == i {
			continue
		}
		name := string(data[nameStart:i])
		fq := name
		if !strings.Contains(name, ".") {
			fq = imports[name]
		}
		if !allowed[fq] {
			continue
		}
		for i < end && dubboSpaceByte170(data[i]) {
			i++
		}
		args := ""
		if i < end && data[i] == '(' {
			close, ok := dubboMatchingParen170(data, i, end)
			if !ok {
				continue
			}
			args = string(data[i+1 : close])
			i = close + 1
		}
		out = append(out, dubboAnnotation170{Name: fq, Args: args, Start: annStart, End: i})
	}
	return out
}

func dubboAnnotationAttrs170(args string) map[string]string {
	out := map[string]string{}
	for _, match := range dubboAttrStringRE170.FindAllStringSubmatch(args, -1) {
		if len(match) == 3 {
			out[match[1]] = strings.TrimSpace(match[2])
		}
	}
	for _, match := range dubboAttrClassRE170.FindAllStringSubmatch(args, -1) {
		if len(match) == 3 {
			out[match[1]] = strings.TrimSpace(match[2])
		}
	}
	return out
}

func dubboResolveConfigValue170(raw string, props map[string]string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "*" {
		return "", false
	}
	value := raw
	for i := 0; i < 16; i++ {
		start := strings.Index(value, "${")
		if start < 0 {
			value = strings.TrimSpace(value)
			return value, value != "" && value != "*"
		}
		endRel := strings.IndexByte(value[start+2:], '}')
		if endRel < 0 {
			return "", false
		}
		end := start + 2 + endRel
		key := strings.TrimSpace(value[start+2 : end])
		replacement, ok := props[key]
		if !ok {
			return "", false
		}
		value = value[:start] + replacement + value[end+1:]
	}
	return "", false
}

func dubboLocalProperties170(root string) map[string]string {
	out := map[string]string{}
	for _, name := range []string{"application.properties", "bootstrap.properties"} {
		path := filepath.Join(root, "src", "main", "resources", name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
				continue
			}
			idx := strings.IndexAny(line, "=:")
			if idx <= 0 {
				continue
			}
			key, value := strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:])
			if key != "" {
				out[key] = value
			}
		}
	}
	return out
}

func dubboPackage170(data []byte) string {
	match := dubboPackageRE170.FindSubmatch(data)
	if len(match) == 2 {
		return string(match[1])
	}
	return ""
}

func dubboImports170(data []byte) map[string]string {
	out := map[string]string{}
	for _, match := range dubboImportRE170.FindAllSubmatch(data, -1) {
		if len(match) != 2 {
			continue
		}
		fq := string(match[1])
		if dot := strings.LastIndexByte(fq, '.'); dot >= 0 && dot < len(fq)-1 {
			out[fq[dot+1:]] = fq
		}
	}
	return out
}

func dubboResolveType170(raw, pkg string, imports map[string]string) string {
	raw = strings.TrimSpace(strings.TrimSuffix(raw, "..."))
	arraySuffix := ""
	for strings.HasSuffix(raw, "[]") {
		arraySuffix += "[]"
		raw = strings.TrimSuffix(raw, "[]")
	}
	if idx := strings.IndexByte(raw, '<'); idx >= 0 {
		raw = raw[:idx]
	}
	raw = strings.TrimSpace(raw)
	primitive := map[string]bool{"boolean": true, "byte": true, "short": true, "int": true, "long": true, "float": true, "double": true, "char": true, "void": true}
	if primitive[raw] {
		return raw + arraySuffix
	}
	javaLang := map[string]bool{"String": true, "Long": true, "Integer": true, "Boolean": true, "Byte": true, "Short": true, "Double": true, "Float": true, "Character": true, "Object": true, "Class": true}
	if javaLang[raw] {
		return "java.lang." + raw + arraySuffix
	}
	if strings.Contains(raw, ".") {
		return raw + arraySuffix
	}
	if fq := imports[raw]; fq != "" {
		return fq + arraySuffix
	}
	return dubboJoinPackage170(pkg, raw) + arraySuffix
}

func dubboTypeEqual170(left, right string) bool {
	left, right = strings.TrimSpace(left), strings.TrimSpace(right)
	return left == right || dubboSimpleName170(left) == dubboSimpleName170(right)
}

func dubboOwnerRange170(data []byte, owner string) (int, int, bool) {
	re := regexp.MustCompile(`\b(?:class|interface|enum)\s+` + regexp.QuoteMeta(owner) + `\b[^\{]*\{`)
	loc := re.FindIndex(data)
	if len(loc) != 2 {
		return 0, 0, false
	}
	open := loc[1] - 1
	close, ok := dubboMatchingBrace170(data, open)
	if !ok {
		return 0, 0, false
	}
	return loc[0], close + 1, true
}

func dubboMatchingBrace170(data []byte, open int) (int, bool) {
	return dubboMatchingDelimiter170(data, open, len(data), '{', '}')
}

func dubboMatchingParen170(data []byte, open, end int) (int, bool) {
	return dubboMatchingDelimiter170(data, open, end, '(', ')')
}

func dubboMatchingDelimiter170(data []byte, open, end int, left, right byte) (int, bool) {
	if open < 0 || open >= end || open >= len(data) || data[open] != left {
		return 0, false
	}
	depth, quote := 0, byte(0)
	for i := open; i < end && i < len(data); i++ {
		b := data[i]
		if quote != 0 {
			if b == '\\' {
				i++
				continue
			}
			if b == quote {
				quote = 0
			}
			continue
		}
		if b == '"' || b == '\'' {
			quote = b
			continue
		}
		if b == left {
			depth++
		} else if b == right {
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

func dubboSplitComma170(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	out := []string{}
	runes := []rune(raw)
	start, angle, paren, bracket := 0, 0, 0, 0
	quote := rune(0)
	for i, r := range runes {
		if quote != 0 {
			if r == quote && (i == 0 || runes[i-1] != '\\') {
				quote = 0
			}
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
		case '<':
			angle++
		case '>':
			if angle > 0 {
				angle--
			}
		case '(':
			paren++
		case ')':
			if paren > 0 {
				paren--
			}
		case '[':
			bracket++
		case ']':
			if bracket > 0 {
				bracket--
			}
		case ',':
			if angle == 0 && paren == 0 && bracket == 0 {
				out = append(out, strings.TrimSpace(string(runes[start:i])))
				start = i + 1
			}
		}
	}
	out = append(out, strings.TrimSpace(string(runes[start:])))
	return out
}

func dubboRange170(data []byte, ref nav.ReviewRef170, start, end int) nav.SourceRange170 {
	if len(data) == 0 {
		return nav.SourceRange170{Ref: ref, StartLine: 1, EndLine: 1, StartColumn: 1, EndColumn: 2}
	}
	if start < 0 {
		start = 0
	}
	if end > len(data) {
		end = len(data)
	}
	if end <= start {
		if start >= len(data) {
			start = len(data) - 1
		}
		end = start + 1
	}
	startLine, startCol := myBatisLineColumn170(data, start)
	endLine, endCol := myBatisLineColumn170(data, end)
	if startLine == endLine && endCol <= startCol {
		endCol = startCol + 1
	}
	return nav.SourceRange170{Ref: ref, StartLine: startLine, EndLine: endLine, StartColumn: startCol, EndColumn: endCol}
}

func dubboProviderRef170(candidate dubboProviderMethod170, side string) nav.ReviewRef170 {
	return nav.ReviewRef170{
		Workspace: candidate.Workspace, Path: candidate.Path, Side: side, Kind: "METHOD",
		OwnerFQCN: candidate.OwnerFQCN, Name: candidate.Name, ParameterTypes: append([]string{}, candidate.ParameterTypes...),
	}
}

func dubboUnresolvedRelation170(from nav.ReviewRef170, evidence []nav.SourceRange170, reason string) nav.Relation170 {
	if evidence == nil {
		evidence = []nav.SourceRange170{}
	}
	relation := nav.Relation170{Kind: "DUBBO_CONTRACT", Resolution: "UNRESOLVED", From: from, Targets: []nav.ReviewRef170{}, Evidence: evidence, Reason: reason, Assumptions: []string{}}
	relation.ID = dubboRelationID170(relation)
	return relation
}

func dubboRelationID170(relation nav.Relation170) string {
	parts := []string{relation.Kind, relation.Resolution, relation.Reason}
	if key, err := nav.ReviewRefKey170(relation.From); err == nil {
		parts = append(parts, key)
	}
	targets := []string{}
	for _, target := range relation.Targets {
		if key, err := nav.ReviewRefKey170(target); err == nil {
			targets = append(targets, key)
		}
	}
	sort.Strings(targets)
	parts = append(parts, targets...)
	assumptions := append([]string{}, relation.Assumptions...)
	sort.Strings(assumptions)
	parts = append(parts, assumptions...)
	for _, source := range relation.Evidence {
		key, _ := nav.ReviewRefKey170(source.Ref)
		parts = append(parts, fmt.Sprintf("%s:%d:%d:%d:%d", key, source.StartLine, source.StartColumn, source.EndLine, source.EndColumn))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "dubbo_contract:" + hex.EncodeToString(sum[:])
}

func dubboJoinPackage170(pkg, name string) string {
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, ".") || pkg == "" {
		return name
	}
	return pkg + "." + name
}

func dubboSimpleName170(value string) string {
	value = strings.TrimSpace(value)
	for strings.HasSuffix(value, "[]") {
		value = strings.TrimSuffix(value, "[]")
	}
	if dot := strings.LastIndexByte(value, '.'); dot >= 0 {
		return value[dot+1:]
	}
	return value
}

func dubboIdentByte170(b byte) bool {
	return b == '_' || b == '$' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

func dubboSpaceByte170(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n'
}
