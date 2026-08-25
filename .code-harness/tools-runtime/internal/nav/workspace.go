package nav

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"codea-harness-tools/internal/workspace"
)

const (
	NavigationComplete = "COMPLETE"
	NavigationPartial  = "PARTIAL"
)

const (
	CodeWorkspaceNotConfigured     = "WORKSPACE_DEPENDENCY_NOT_CONFIGURED"
	CodeSuperclassNotFound         = "SUPERCLASS_NOT_FOUND"
	CodeInheritedMethodNotFound    = "INHERITED_METHOD_NOT_FOUND"
	CodeAmbiguousInheritedMethod   = "AMBIGUOUS_INHERITED_METHOD"
	CodeTemplateOverrideNotFound   = "TEMPLATE_OVERRIDE_NOT_FOUND"
	CodeAmbiguousTemplateDispatch  = "AMBIGUOUS_TEMPLATE_DISPATCH"
)

type WorkspaceNavigationFact struct {
	Workspace string `json:"workspace"`
	Symbol    string `json:"symbol"`
	Path      string `json:"path"`
	Role      string `json:"role,omitempty"`
	Source    string `json:"source"`
	From      string `json:"from,omitempty"`
}

type WorkspaceNavigationLimitation struct {
	Code   string `json:"code"`
	Symbol string `json:"symbol,omitempty"`
	From   string `json:"from,omitempty"`
}

type WorkspaceNavigationResult struct {
	Status     string                         `json:"status"`
	Fact       *WorkspaceNavigationFact       `json:"fact,omitempty"`
	Limitation *WorkspaceNavigationLimitation `json:"limitation,omitempty"`
}

type WorkspaceInheritanceResolver struct {
	CurrentRoot string
	Dependency  workspace.VerificationResult
}

type javaMethod struct {
	Name     string
	Body     string
	Abstract bool
}

type javaClass struct {
	Name    string
	Super   string
	Path    string
	Methods []javaMethod
}

var classDeclarationRE = regexp.MustCompile(`(?m)\b(?:public\s+)?(?:final\s+|abstract\s+)?class\s+([A-Za-z_$][A-Za-z0-9_$]*)(?:\s+extends\s+([A-Za-z_$][A-Za-z0-9_$.]*))?`)
var methodDeclarationRE = regexp.MustCompile(`(?m)(?:^|\n)\s*(?:@[A-Za-z_$][A-Za-z0-9_$.]*(?:\([^\n]*\))?\s*)*(?:(?:public|protected|private)\s+)?(?:(?:abstract|static|final|synchronized|native)\s+)*(?:[A-Za-z_$][A-Za-z0-9_$.<>?,\[\]]*\s+)+([A-Za-z_$][A-Za-z0-9_$]*)\s*\([^;{}]*\)\s*(\{|;)`)

func (r WorkspaceInheritanceResolver) ResolveInheritedCall(fromSymbol, method string) WorkspaceNavigationResult {
	if rejected := r.rejectUnverified(method, fromSymbol); rejected != nil {
		return *rejected
	}
	owner, fromMethod, ok := splitQualifiedMethod(fromSymbol)
	if !ok {
		return partial(CodeInheritedMethodNotFound, fromSymbol, fromSymbol)
	}
	currentClasses, err := findClasses(r.CurrentRoot, owner)
	if err != nil || len(currentClasses) != 1 || strings.TrimSpace(currentClasses[0].Super) == "" {
		return partial(CodeSuperclassNotFound, owner, fromSymbol)
	}
	fromMethods := methodsNamed(currentClasses[0], fromMethod)
	if len(fromMethods) != 1 || !methodCalls(fromMethods[0], method) {
		return partial(CodeInheritedMethodNotFound, owner+"."+method, fromSymbol)
	}

	superName := simpleJavaName(currentClasses[0].Super)
	superClasses, err := findClasses(r.Dependency.ConfirmedRoot, superName)
	if err != nil || len(superClasses) == 0 {
		return partial(CodeSuperclassNotFound, superName, fromSymbol)
	}
	if len(superClasses) > 1 {
		return partial(CodeAmbiguousInheritedMethod, superName+"."+method, fromSymbol)
	}
	matches := methodsNamed(superClasses[0], method)
	if len(matches) == 0 {
		return partial(CodeInheritedMethodNotFound, superName+"."+method, fromSymbol)
	}
	if len(matches) > 1 {
		return partial(CodeAmbiguousInheritedMethod, superName+"."+method, fromSymbol)
	}
	return completeFact(r.Dependency.DependencyID, superName+"."+method, superClasses[0].Path, fromSymbol)
}

func (r WorkspaceInheritanceResolver) ResolveSuperclassCall(fromSymbol, method string) WorkspaceNavigationResult {
	if rejected := r.rejectUnverified(method, fromSymbol); rejected != nil {
		return *rejected
	}
	owner, fromMethod, ok := splitQualifiedMethod(fromSymbol)
	if !ok {
		return partial(CodeInheritedMethodNotFound, fromSymbol, fromSymbol)
	}
	classes, err := findClasses(r.Dependency.ConfirmedRoot, owner)
	if err != nil || len(classes) == 0 {
		return partial(CodeSuperclassNotFound, owner, fromSymbol)
	}
	if len(classes) > 1 {
		return partial(CodeAmbiguousInheritedMethod, owner+"."+method, fromSymbol)
	}
	fromMethods := methodsNamed(classes[0], fromMethod)
	if len(fromMethods) != 1 || !methodCalls(fromMethods[0], method) {
		return partial(CodeInheritedMethodNotFound, owner+"."+method, fromSymbol)
	}
	matches := methodsNamed(classes[0], method)
	if len(matches) == 0 {
		return partial(CodeInheritedMethodNotFound, owner+"."+method, fromSymbol)
	}
	if len(matches) > 1 {
		return partial(CodeAmbiguousInheritedMethod, owner+"."+method, fromSymbol)
	}
	return completeFact(r.Dependency.DependencyID, owner+"."+method, classes[0].Path, fromSymbol)
}

func (r WorkspaceInheritanceResolver) ResolveTemplateDispatch(fromSymbol, hook, concreteClass string) WorkspaceNavigationResult {
	if rejected := r.rejectUnverified(hook, fromSymbol); rejected != nil {
		return *rejected
	}
	owner, fromMethod, ok := splitQualifiedMethod(fromSymbol)
	if !ok {
		return partial(CodeTemplateOverrideNotFound, hook, fromSymbol)
	}
	classes, err := findClasses(r.Dependency.ConfirmedRoot, owner)
	if err != nil || len(classes) == 0 {
		return partial(CodeSuperclassNotFound, owner, fromSymbol)
	}
	if len(classes) > 1 {
		return partial(CodeAmbiguousTemplateDispatch, owner+"."+hook, fromSymbol)
	}
	fromMethods := methodsNamed(classes[0], fromMethod)
	if len(fromMethods) != 1 || !methodCalls(fromMethods[0], hook) {
		return partial(CodeTemplateOverrideNotFound, owner+"."+hook, fromSymbol)
	}
	hookDeclarations := methodsNamed(classes[0], hook)
	if len(hookDeclarations) == 0 {
		return partial(CodeTemplateOverrideNotFound, owner+"."+hook, fromSymbol)
	}
	if len(hookDeclarations) > 1 {
		return partial(CodeAmbiguousTemplateDispatch, owner+"."+hook, fromSymbol)
	}

	candidates, err := findDirectSubclassesWithMethod(r.CurrentRoot, owner, hook, concreteClass)
	if err != nil || len(candidates) == 0 {
		return partial(CodeTemplateOverrideNotFound, owner+"."+hook, fromSymbol)
	}
	if len(candidates) > 1 {
		return partial(CodeAmbiguousTemplateDispatch, owner+"."+hook, fromSymbol)
	}
	fact := WorkspaceNavigationFact{
		Workspace: "current",
		Symbol: candidates[0].Name+"."+hook,
		Path: candidates[0].Path,
		Role: "Service",
		Source: "WORKSPACE_INHERITANCE",
		From: fromSymbol,
	}
	return WorkspaceNavigationResult{Status: NavigationComplete, Fact: &fact}
}

func (r WorkspaceInheritanceResolver) rejectUnverified(symbol, from string) *WorkspaceNavigationResult {
	if strings.TrimSpace(r.Dependency.DependencyID) == "" {
		result := partial(CodeWorkspaceNotConfigured, symbol, from)
		return &result
	}
	if r.Dependency.Status != workspace.StatusVerified {
		code := strings.TrimSpace(r.Dependency.Code)
		if code == "" {
			switch r.Dependency.Status {
			case workspace.StatusSourceNotFound:
				code = workspace.CodeSourceNotFound
			case workspace.StatusCoordinateMismatch:
				code = workspace.CodeCoordinateMismatch
			case workspace.StatusVersionUnresolved:
				code = workspace.CodeVersionUnresolved
			case workspace.StatusVersionMismatch:
				code = workspace.CodeVersionMismatch
			default:
				code = CodeWorkspaceNotConfigured
			}
		}
		result := partial(code, symbol, from)
		return &result
	}
	if strings.TrimSpace(r.Dependency.ConfirmedRoot) == "" {
		result := partial(workspace.CodeSourceNotFound, symbol, from)
		return &result
	}
	info, err := os.Stat(r.Dependency.ConfirmedRoot)
	if err != nil || !info.IsDir() {
		result := partial(workspace.CodeSourceNotFound, symbol, from)
		return &result
	}
	return nil
}

func findDirectSubclassesWithMethod(root, superName, method, concrete string) ([]javaClass, error) {
	classes, err := allJavaClasses(root)
	if err != nil {
		return nil, err
	}
	var matches []javaClass
	for _, class := range classes {
		if simpleJavaName(class.Super) != simpleJavaName(superName) {
			continue
		}
		if concrete != "" && class.Name != concrete {
			continue
		}
		if len(methodsNamed(class, method)) != 1 {
			continue
		}
		matches = append(matches, class)
	}
	return matches, nil
}

func findClasses(root, name string) ([]javaClass, error) {
	classes, err := allJavaClasses(root)
	if err != nil {
		return nil, err
	}
	var matches []javaClass
	for _, class := range classes {
		if class.Name == simpleJavaName(name) {
			matches = append(matches, class)
		}
	}
	return matches, nil
}

func allJavaClasses(root string) ([]javaClass, error) {
	sourceRoot := filepath.Join(root, "src", "main", "java")
	var classes []javaClass
	err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
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
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		parsed := parseJavaClasses(string(data))
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		for i := range parsed {
			parsed[i].Path = filepath.ToSlash(rel)
			classes = append(classes, parsed[i])
		}
		return nil
	})
	if os.IsNotExist(err) {
		return []javaClass{}, nil
	}
	return classes, err
}

func parseJavaClasses(source string) []javaClass {
	classMatches := classDeclarationRE.FindAllStringSubmatchIndex(source, -1)
	classes := make([]javaClass, 0, len(classMatches))
	for _, match := range classMatches {
		if len(match) < 6 {
			continue
		}
		name := source[match[2]:match[3]]
		super := ""
		if match[4] >= 0 && match[5] >= 0 {
			super = source[match[4]:match[5]]
		}
		classes = append(classes, javaClass{Name: name, Super: super, Methods: parseJavaMethods(source)})
	}
	return classes
}

func parseJavaMethods(source string) []javaMethod {
	matches := methodDeclarationRE.FindAllStringSubmatchIndex(source, -1)
	methods := make([]javaMethod, 0, len(matches))
	for _, match := range matches {
		if len(match) < 6 || match[2] < 0 || match[3] < 0 || match[4] < 0 || match[5] < 0 {
			continue
		}
		name := source[match[2]:match[3]]
		terminator := source[match[4]:match[5]]
		method := javaMethod{Name: name, Abstract: terminator == ";"}
		if terminator == "{" {
			open := match[5] - 1
			close := matchingBrace(source, open)
			if close > open {
				method.Body = source[open+1 : close]
			}
		}
		methods = append(methods, method)
	}
	return methods
}

func matchingBrace(source string, open int) int {
	depth := 0
	inString := byte(0)
	escaped := false
	for i := open; i < len(source); i++ {
		c := source[i]
		if inString != 0 {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == inString {
				inString = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			inString = c
			continue
		}
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func methodsNamed(class javaClass, name string) []javaMethod {
	var matches []javaMethod
	for _, method := range class.Methods {
		if method.Name == name {
			matches = append(matches, method)
		}
	}
	return matches
}

func methodCalls(method javaMethod, called string) bool {
	if strings.TrimSpace(method.Body) == "" {
		return false
	}
	pattern := regexp.MustCompile(`(?:\bsuper\s*\.\s*)?\b` + regexp.QuoteMeta(called) + `\s*\(`)
	return pattern.FindStringIndex(method.Body) != nil
}

func splitQualifiedMethod(symbol string) (string, string, bool) {
	idx := strings.LastIndex(strings.TrimSpace(symbol), ".")
	if idx <= 0 || idx == len(symbol)-1 {
		return "", "", false
	}
	return symbol[:idx], symbol[idx+1:], true
}

func simpleJavaName(name string) string {
	name = strings.TrimSpace(name)
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

func completeFact(workspaceID, symbol, path, from string) WorkspaceNavigationResult {
	fact := WorkspaceNavigationFact{
		Workspace: workspaceID,
		Symbol: symbol,
		Path: filepath.ToSlash(path),
		Role: "Service",
		Source: "WORKSPACE_INHERITANCE",
		From: from,
	}
	return WorkspaceNavigationResult{Status: NavigationComplete, Fact: &fact}
}

func partial(code, symbol, from string) WorkspaceNavigationResult {
	return WorkspaceNavigationResult{
		Status: NavigationPartial,
		Limitation: &WorkspaceNavigationLimitation{Code: code, Symbol: symbol, From: from},
	}
}

var _ = fmt.Sprintf
