package nav

import (
	"context"
	"sort"
	"strings"
)

// ControllerEndpointMatch is a focused machine fact for Task 1 entrypoint inventory.
// It is produced only from pinned ast-grep declaration ranges and annotations.
type ControllerEndpointMatch struct {
	Controller          string
	Symbol              string
	Path                string
	ControllerStartLine int
	ControllerEndLine   int
	StartLine           int
	EndLine             int
}

var controllerAnnotations153 = map[string]bool{
	"Controller":     true,
	"RestController": true,
}

var mappingAnnotations153 = map[string]bool{
	"RequestMapping": true,
	"GetMapping":     true,
	"PostMapping":    true,
	"PutMapping":     true,
	"PatchMapping":   true,
	"DeleteMapping":  true,
}

// FindControllerEndpoints performs one declaration-pattern pass for types and one
// for methods. This intentionally avoids calling FindByAnnotation once per Spring
// annotation, which expands the same declaration pattern matrix repeatedly and is
// too expensive for a real Windows inventory scan.
func (n Navigator) FindControllerEndpoints(ctx context.Context, scope string) ([]ControllerEndpointMatch, error) {
	if err := n.validate("X", scope); err != nil {
		return nil, err
	}

	types, err := n.runRaw(ctx, scope, allTypePatterns()...)
	if err != nil {
		return nil, err
	}
	controllers := make([]rawMatch, 0)
	for _, typ := range types {
		kind, name := typeKindAndName(typ.Text)
		if kind != "CLASS" || name == "" || !hasAnyAnnotation153(typ.Text, controllerAnnotations153) {
			continue
		}
		controllers = append(controllers, typ)
	}
	controllers = dedupeRaw(controllers)
	if len(controllers) == 0 {
		return nil, nil
	}

	methods, err := n.runRaw(ctx, scope, allMethodPatterns()...)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	out := make([]ControllerEndpointMatch, 0)
	for _, method := range methods {
		if !hasAnyAnnotation153(method.Text, mappingAnnotations153) {
			continue
		}
		name := methodName(method.Text)
		if name == "" {
			continue
		}
		controller, ok := smallestContaining(controllers, method)
		if !ok {
			continue
		}
		_, controllerName := typeKindAndName(controller.Text)
		if controllerName == "" {
			continue
		}
		symbol := controllerName + "." + name
		key := method.Path + "\x00" + symbol
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, ControllerEndpointMatch{
			Controller:          controllerName,
			Symbol:              symbol,
			Path:                method.Path,
			ControllerStartLine: controller.StartLine,
			ControllerEndLine:   controller.EndLine,
			StartLine:           method.StartLine,
			EndLine:             method.EndLine,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		if out[i].Symbol != out[j].Symbol {
			return out[i].Symbol < out[j].Symbol
		}
		return out[i].StartLine < out[j].StartLine
	})
	return out, nil
}

func hasAnyAnnotation153(text string, accepted map[string]bool) bool {
	for _, annotation := range annotations(text) {
		name := strings.TrimPrefix(annotation, "@")
		if i := strings.IndexByte(name, '('); i >= 0 {
			name = name[:i]
		}
		if i := strings.LastIndexByte(name, '.'); i >= 0 {
			name = name[i+1:]
		}
		if accepted[name] {
			return true
		}
	}
	return false
}
