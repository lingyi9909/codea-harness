package nav

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// FindDirectMethodCallsBatch180 snapshots Java declarations and call expressions
// with one ast-grep process, then projects direct calls for every method. It is
// intentionally narrow for the 1.8 review prepare path.
func (n Navigator) FindDirectMethodCallsBatch180(ctx context.Context, scope string) (map[string][]DirectMethodCall, error) {
	records, err := n.runDirectCallsBatch180(ctx, scope)
	if err != nil {
		return nil, err
	}
	var types, methods, calls []rawMatch
	for _, r := range records {
		switch r.RuleID {
		case "codea-direct-calls-180-types":
			types = append(types, r)
		case "codea-direct-calls-180-methods":
			methods = append(methods, r)
		case "codea-direct-calls-180-calls":
			calls = append(calls, r)
		default:
			return nil, fmt.Errorf("unexpected direct-call ruleId %q", r.RuleID)
		}
	}
	out := map[string][]DirectMethodCall{}
	for _, method := range methods {
		ownerType, ok := smallestContaining(types, method)
		if !ok {
			continue
		}
		_, owner := typeKindAndName(ownerType.Text)
		member := methodName(method.Text)
		if owner == "" || member == "" {
			continue
		}
		from := owner + "." + member
		seen := map[string]bool{}
		for _, call := range calls {
			if !contains(method, call) {
				continue
			}
			receiver, called, ok := directCallParts163(call.Text)
			if !ok {
				continue
			}
			fact := DirectMethodCall{FromSymbol: from, Receiver: receiver, Method: called, Path: call.Path, Line: call.StartLine}
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
			if !seen[key] {
				seen[key] = true
				out[from] = append(out[from], fact)
			}
		}
		sort.Slice(out[from], func(i, j int) bool {
			if out[from][i].Line != out[from][j].Line {
				return out[from][i].Line < out[from][j].Line
			}
			return out[from][i].TargetSymbol < out[from][j].TargetSymbol
		})
	}
	return out, nil
}

// FindImplementationTypesBatch180 resolves concrete implementations for a set
// of receiver types in one ast-grep process. It never guesses from class names.
func (n Navigator) FindImplementationTypesBatch180(ctx context.Context, owners []string, scope string) (map[string][]ImplementationType, error) {
	uniq := map[string]bool{}
	clean := make([]string, 0, len(owners))
	for _, owner := range owners {
		owner = strings.TrimSpace(owner)
		if owner == "" || uniq[owner] {
			continue
		}
		if err := n.validate(owner, scope); err != nil {
			return nil, err
		}
		uniq[owner] = true
		clean = append(clean, owner)
	}
	out := map[string][]ImplementationType{}
	if len(clean) == 0 {
		return out, nil
	}

	// Reuse the kind-based declaration snapshot so classes with multiple
	// interfaces (for example "implements Serializable, OrderService") are
	// visible. This is still one bounded ast-grep process.
	records, err := n.runRawBatch167(ctx, scope, "codea-symbol-info-167", []string{"class $C { $$$BODY }"})
	if err != nil {
		return nil, err
	}
	for _, r := range records {
		if r.RuleID != "codea-symbol-info-167-types" {
			continue
		}
		kind, name := typeKindAndName(r.Text)
		if kind != "CLASS" || name == "" {
			continue
		}
		for _, owner := range clean {
			if !declarationRelates180(r.Text, owner) {
				continue
			}
			key := r.Path + "\x00" + name
			duplicate := false
			for _, v := range out[owner] {
				if v.Path+"\x00"+v.Symbol == key {
					duplicate = true
					break
				}
			}
			if !duplicate {
				out[owner] = append(out[owner], ImplementationType{Symbol: name, Path: r.Path})
			}
		}
	}
	for owner := range out {
		sort.Slice(out[owner], func(i, j int) bool {
			if out[owner][i].Path != out[owner][j].Path {
				return out[owner][i].Path < out[owner][j].Path
			}
			return out[owner][i].Symbol < out[owner][j].Symbol
		})
	}
	return out, nil
}

func declarationRelates180(text, owner string) bool {
	header := text
	if i := strings.IndexByte(header, '{'); i >= 0 {
		header = header[:i]
	}
	normalized := strings.NewReplacer(",", " ", "\n", " ", "\r", " ", "\t", " ").Replace(header)
	fields := strings.Fields(normalized)
	for i, f := range fields {
		switch f {
		case "extends":
			if i+1 < len(fields) && simpleType(fields[i+1]) == owner {
				return true
			}
		case "implements":
			for j := i + 1; j < len(fields); j++ {
				if simpleType(fields[j]) == owner {
					return true
				}
			}
		}
	}
	return false
}
