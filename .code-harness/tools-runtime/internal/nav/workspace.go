package nav

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"codea-harness-tools/internal/workspace"
)

const (
	NavigationComplete = "COMPLETE"
	NavigationPartial  = "PARTIAL"
)

const (
	CodeWorkspaceNotConfigured    = "WORKSPACE_DEPENDENCY_NOT_CONFIGURED"
	CodeSuperclassNotFound        = "SUPERCLASS_NOT_FOUND"
	CodeInheritedMethodNotFound   = "INHERITED_METHOD_NOT_FOUND"
	CodeAmbiguousInheritedMethod  = "AMBIGUOUS_INHERITED_METHOD"
	CodeTemplateOverrideNotFound  = "TEMPLATE_OVERRIDE_NOT_FOUND"
	CodeAmbiguousTemplateDispatch = "AMBIGUOUS_TEMPLATE_DISPATCH"
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
	AstGrepPath string
	Runner      Runner
}

func (r WorkspaceInheritanceResolver) ResolveInheritedCall(fromSymbol, method string) WorkspaceNavigationResult {
	if rejected := r.rejectUnverified(method, fromSymbol); rejected != nil {
		return *rejected
	}
	owner, _, ok := splitQualifiedMethod(fromSymbol)
	if !ok {
		return partial(CodeInheritedMethodNotFound, fromSymbol, fromSymbol)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	current := r.navigator(r.CurrentRoot)
	super, err := current.WorkspaceSuperclass(ctx, owner)
	if err != nil {
		if errors.Is(err, ErrAmbiguousSymbol) {
			return partial(CodeAmbiguousInheritedMethod, owner+"."+method, fromSymbol)
		}
		return partial(CodeSuperclassNotFound, owner, fromSymbol)
	}
	calls, err := current.WorkspaceMethodCalls(ctx, fromSymbol, method)
	if err != nil || !calls {
		return partial(CodeInheritedMethodNotFound, owner+"."+method, fromSymbol)
	}

	superName := normalizeWorkspaceType(super.Super)
	if superName == "" {
		return partial(CodeSuperclassNotFound, owner, fromSymbol)
	}
	dependency := r.navigator(r.Dependency.ConfirmedRoot)
	target, err := dependency.WorkspaceMethod(ctx, superName, method)
	if err != nil {
		if errors.Is(err, ErrAmbiguousSymbol) {
			return partial(CodeAmbiguousInheritedMethod, superName+"."+method, fromSymbol)
		}
		return partial(CodeInheritedMethodNotFound, superName+"."+method, fromSymbol)
	}
	return completeFact(r.Dependency.DependencyID, superName+"."+method, target.Path, fromSymbol)
}

func (r WorkspaceInheritanceResolver) ResolveSuperclassCall(fromSymbol, method string) WorkspaceNavigationResult {
	if rejected := r.rejectUnverified(method, fromSymbol); rejected != nil {
		return *rejected
	}
	owner, _, ok := splitQualifiedMethod(fromSymbol)
	if !ok {
		return partial(CodeInheritedMethodNotFound, fromSymbol, fromSymbol)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dependency := r.navigator(r.Dependency.ConfirmedRoot)
	calls, err := dependency.WorkspaceMethodCalls(ctx, fromSymbol, method)
	if err != nil || !calls {
		return partial(CodeInheritedMethodNotFound, owner+"."+method, fromSymbol)
	}
	target, err := dependency.WorkspaceMethod(ctx, owner, method)
	if err != nil {
		if errors.Is(err, ErrAmbiguousSymbol) {
			return partial(CodeAmbiguousInheritedMethod, owner+"."+method, fromSymbol)
		}
		return partial(CodeInheritedMethodNotFound, owner+"."+method, fromSymbol)
	}
	return completeFact(r.Dependency.DependencyID, owner+"."+method, target.Path, fromSymbol)
}

func (r WorkspaceInheritanceResolver) ResolveTemplateDispatch(fromSymbol, hook, concreteClass string) WorkspaceNavigationResult {
	if rejected := r.rejectUnverified(hook, fromSymbol); rejected != nil {
		return *rejected
	}
	owner, _, ok := splitQualifiedMethod(fromSymbol)
	if !ok {
		return partial(CodeTemplateOverrideNotFound, hook, fromSymbol)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dependency := r.navigator(r.Dependency.ConfirmedRoot)
	calls, err := dependency.WorkspaceMethodCalls(ctx, fromSymbol, hook)
	if err != nil || !calls {
		return partial(CodeTemplateOverrideNotFound, owner+"."+hook, fromSymbol)
	}
	if _, err := dependency.WorkspaceMethod(ctx, owner, hook); err != nil {
		if errors.Is(err, ErrAmbiguousSymbol) {
			return partial(CodeAmbiguousTemplateDispatch, owner+"."+hook, fromSymbol)
		}
		return partial(CodeTemplateOverrideNotFound, owner+"."+hook, fromSymbol)
	}

	current := r.navigator(r.CurrentRoot)
	candidates, err := current.WorkspaceDirectSubclassesWithMethod(ctx, owner, hook, concreteClass)
	if err != nil {
		if errors.Is(err, ErrAmbiguousSymbol) {
			return partial(CodeAmbiguousTemplateDispatch, owner+"."+hook, fromSymbol)
		}
		return partial(CodeTemplateOverrideNotFound, owner+"."+hook, fromSymbol)
	}
	if len(candidates) == 0 {
		return partial(CodeTemplateOverrideNotFound, owner+"."+hook, fromSymbol)
	}
	if len(candidates) > 1 {
		return partial(CodeAmbiguousTemplateDispatch, owner+"."+hook, fromSymbol)
	}
	method, err := current.WorkspaceMethod(ctx, candidates[0].Name, hook)
	if err != nil {
		if errors.Is(err, ErrAmbiguousSymbol) {
			return partial(CodeAmbiguousTemplateDispatch, owner+"."+hook, fromSymbol)
		}
		return partial(CodeTemplateOverrideNotFound, owner+"."+hook, fromSymbol)
	}
	return completeFact("current", candidates[0].Name+"."+hook, method.Path, fromSymbol)
}

func (r WorkspaceInheritanceResolver) navigator(root string) Navigator {
	astPath := strings.TrimSpace(r.AstGrepPath)
	if astPath == "" {
		astPath = "ast-grep"
	}
	return Navigator{RepoRoot: root, AstGrepPath: astPath, Runner: r.Runner}
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

func splitQualifiedMethod(symbol string) (string, string, bool) {
	idx := strings.LastIndex(strings.TrimSpace(symbol), ".")
	if idx <= 0 || idx == len(symbol)-1 {
		return "", "", false
	}
	return symbol[:idx], symbol[idx+1:], true
}

func completeFact(workspaceID, symbol, path, from string) WorkspaceNavigationResult {
	fact := WorkspaceNavigationFact{
		Workspace: workspaceID,
		Symbol:    symbol,
		Path:      strings.ReplaceAll(path, "\\", "/"),
		Role:      "Service",
		Source:    "WORKSPACE_INHERITANCE",
		From:      from,
	}
	return WorkspaceNavigationResult{Status: NavigationComplete, Fact: &fact}
}

func partial(code, symbol, from string) WorkspaceNavigationResult {
	return WorkspaceNavigationResult{
		Status: NavigationPartial,
		Limitation: &WorkspaceNavigationLimitation{
			Code:   code,
			Symbol: symbol,
			From:   from,
		},
	}
}
