package nav

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// DirectMethodCall is a Runtime navigation fact for one direct Java method call.
// The call expression and enclosing method are AST-confirmed. ReceiverType is
// emitted only when the receiver can be resolved from the AST-confirmed owner
// declaration; unresolved receivers are kept separate instead of guessed.
type DirectMethodCall struct {
	FromSymbol   string `json:"fromSymbol"`
	TargetSymbol string `json:"targetSymbol,omitempty"`
	Receiver     string `json:"receiver,omitempty"`
	ReceiverType string `json:"receiverType,omitempty"`
	Method       string `json:"method"`
	Path         string `json:"path"`
	Line         int    `json:"line"`
	Resolved     bool   `json:"resolved"`
}

// ImplementationType is an AST-confirmed concrete implementation declaration.
type ImplementationType struct {
	Symbol      string   `json:"symbol"`
	Path        string   `json:"path"`
	Annotations []string `json:"annotations"`
}

// FindDirectMethodCalls returns direct calls contained by exactly one requested
// owner method. Text parsing is used only to read identifiers from AST match
// text; target type authority comes from the AST-confirmed enclosing type and
// its field declarations.
func (n Navigator) FindDirectMethodCalls(ctx context.Context, symbol, scope string) ([]DirectMethodCall, error) {
	if err := n.validate(symbol, scope); err != nil {
		return nil, err
	}
	owner, member := splitSymbol(symbol)
	if member == "" {
		return nil, ErrInvalidSymbol
	}

	types, err := n.runRaw(ctx, scope, typePatterns(owner)...)
	if err != nil {
		return nil, err
	}
	types = filterTypeName(types, owner)
	if len(types) == 0 {
		return nil, ErrSymbolNotFound
	}
	if len(types) > 1 {
		return nil, ErrAmbiguousSymbol
	}

	methods, err := n.runRaw(ctx, scope, methodPatterns(member)...)
	if err != nil {
		return nil, err
	}
	var owners []rawMatch
	for _, method := range methods {
		if methodName(method.Text) == member && contains(types[0], method) {
			owners = append(owners, method)
		}
	}
	owners = dedupeRaw(owners)
	if len(owners) == 0 {
		return nil, ErrSymbolNotFound
	}
	if len(owners) > 1 {
		return nil, ErrAmbiguousSymbol
	}
	methodRange := owners[0]

	calls, err := n.runRaw(ctx, scope, "$OBJ.$M($$$ARGS)", "$M($$$ARGS)")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := make([]DirectMethodCall, 0)
	for _, call := range calls {
		if !contains(methodRange, call) {
			continue
		}
		receiver, called, ok := directCallParts163(call.Text)
		if !ok {
			continue
		}
		fact := DirectMethodCall{
			FromSymbol: symbol,
			Receiver:   receiver,
			Method:     called,
			Path:       call.Path,
			Line:       call.StartLine,
		}
		switch receiver {
		case "", "this", "super":
			fact.ReceiverType = owner
			fact.TargetSymbol = owner + "." + called
			fact.Resolved = true
		default:
			if typ, ok := receiverFieldType(types, call, receiver); ok {
				fact.ReceiverType = simpleType(typ)
				if identRE.MatchString(fact.ReceiverType) {
					fact.TargetSymbol = fact.ReceiverType + "." + called
					fact.Resolved = true
				}
			}
		}
		key := fmt.Sprintf("%s:%d:%s:%s", fact.Path, fact.Line, fact.TargetSymbol, fact.Receiver)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, fact)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].TargetSymbol < out[j].TargetSymbol
	})
	return out, nil
}

// FindImplementationTypes upgrades FindImplementations' AST class matches into
// exact type facts. No implementation is inferred from naming conventions.
func (n Navigator) FindImplementationTypes(ctx context.Context, symbol, scope string) ([]ImplementationType, error) {
	result, err := n.FindImplementations(ctx, symbol, scope)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := make([]ImplementationType, 0)
	for _, match := range result.Matches {
		kind, name := typeKindAndName(match.Text)
		if kind != "CLASS" || name == "" {
			continue
		}
		info, err := n.GetSymbolInfo(ctx, name, scope)
		if err != nil {
			return nil, err
		}
		key := info.Path + "\x00" + info.Symbol
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, ImplementationType{Symbol: info.Symbol, Path: info.Path, Annotations: append([]string{}, info.Annotations...)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Symbol < out[j].Symbol
	})
	return out, nil
}

func directCallParts163(text string) (receiver, method string, ok bool) {
	s := strings.TrimSpace(text)
	open := strings.IndexByte(s, '(')
	if open <= 0 {
		return "", "", false
	}
	left := strings.TrimSpace(s[:open])
	if left == "" {
		return "", "", false
	}
	if dot := strings.LastIndexByte(left, '.'); dot >= 0 {
		receiver = strings.TrimSpace(left[:dot])
		method = strings.TrimSpace(left[dot+1:])
		if receiver == "" || strings.ContainsAny(receiver, "()[]{} +-*/!?=:,<>\"") {
			return "", "", false
		}
		if i := strings.LastIndexByte(receiver, '.'); i >= 0 {
			receiver = receiver[i+1:]
		}
	} else {
		method = left
	}
	if !identRE.MatchString(method) {
		return "", "", false
	}
	if receiver != "" && receiver != "this" && receiver != "super" && !identRE.MatchString(receiver) {
		return "", "", false
	}
	return receiver, method, true
}
