package nav

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
)

const (
	entrypointTypeRulePrefix164   = "entrypoint-type-"
	entrypointMethodRulePrefix164 = "entrypoint-method-"
)

type entrypointRuleMatch164 struct {
	RuleID string `json:"ruleId"`
	File   string `json:"file"`
	Text   string `json:"text"`
	Range  struct {
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

// FindControllerEndpointsBatch preserves the existing Controller/endpoint
// matching semantics while changing only the ast-grep transport: one Type scan
// over the exact files, followed by one Method scan over exact Controller files.
func (n Navigator) FindControllerEndpointsBatch(ctx context.Context, scopes []string) ([]ControllerEndpointMatch, error) {
	cleanScopes, err := exactEntrypointScopes164(scopes)
	if err != nil {
		return nil, err
	}
	if len(cleanScopes) == 0 {
		return nil, nil
	}

	types, err := n.runEntrypointRuleBatch164(ctx, cleanScopes, entrypointTypeRulePrefix164, allTypePatterns())
	if err != nil {
		return nil, err
	}
	controllers := make([]rawMatch, 0)
	controllerPathSet := map[string]bool{}
	for _, typ := range types {
		kind, name := typeKindAndName(typ.Text)
		if kind != "CLASS" || name == "" || !hasAnyAnnotation153(typ.Text, controllerAnnotations153) {
			continue
		}
		controllers = append(controllers, typ)
		controllerPathSet[typ.Path] = true
	}
	controllers = dedupeRaw(controllers)
	if len(controllers) == 0 {
		return nil, nil
	}

	controllerPaths := make([]string, 0, len(controllerPathSet))
	for p := range controllerPathSet {
		controllerPaths = append(controllerPaths, p)
	}
	sort.Strings(controllerPaths)
	methods, err := n.runEntrypointRuleBatch164(ctx, controllerPaths, entrypointMethodRulePrefix164, allMethodPatterns())
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
			StartColumn:         method.StartColumn,
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

func (n Navigator) runEntrypointRuleBatch164(ctx context.Context, scopes []string, rulePrefix string, patterns []string) ([]rawMatch, error) {
	cleanScopes, err := exactEntrypointScopes164(scopes)
	if err != nil {
		return nil, err
	}
	if len(cleanScopes) == 0 {
		return nil, nil
	}
	runner := n.Runner
	if runner == nil {
		runner = ExecRunner{}
	}
	args := []string{
		"scan",
		"--inline-rules", entrypointRulePack164(rulePrefix, patterns),
		"--json=stream",
		"--color", "never",
	}
	args = append(args, "--")
	args = append(args, cleanScopes...)
	output, runErr := runner.Run(ctx, n.AstGrepPath, args...)
	if runErr != nil {
		return nil, fmt.Errorf("ENTRYPOINT_AST_BATCH_EXECUTION_FAILED: %w", runErr)
	}

	allowed := make(map[string]bool, len(cleanScopes))
	for _, p := range cleanScopes {
		allowed[p] = true
	}
	seen := map[string]bool{}
	out := make([]rawMatch, 0)
	scanner := bufio.NewScanner(bytes.NewReader(output))
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 16*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var match entrypointRuleMatch164
		if err := json.Unmarshal(line, &match); err != nil {
			return nil, fmt.Errorf("ENTRYPOINT_AST_BATCH_OUTPUT_INVALID: %w", err)
		}
		if !strings.HasPrefix(match.RuleID, rulePrefix) {
			return nil, fmt.Errorf("ENTRYPOINT_AST_BATCH_RULE_ID_INVALID: phase=%s ruleId=%q", rulePrefix, match.RuleID)
		}
		matchPath := strings.ReplaceAll(match.File, "\\", "/")
		if !allowed[matchPath] {
			return nil, fmt.Errorf("ENTRYPOINT_SCAN_RESULT_OUT_OF_SCOPE: result=%s", matchPath)
		}
		raw := rawMatch{
			Path:        matchPath,
			Text:        match.Text,
			StartLine:   match.Range.Start.Line + 1,
			StartColumn: match.Range.Start.Column + 1,
			EndLine:     match.Range.End.Line + 1,
			EndColumn:   match.Range.End.Column + 1,
		}
		if raw.EndLine < raw.StartLine {
			raw.EndLine = raw.StartLine
		}
		key := fmt.Sprintf("%s:%d:%d:%d:%d:%s", raw.Path, raw.StartLine, raw.StartColumn, raw.EndLine, raw.EndColumn, raw.Text)
		if !seen[key] {
			seen[key] = true
			out = append(out, raw)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func entrypointRulePack164(prefix string, patterns []string) string {
	var b strings.Builder
	for i, pattern := range patterns {
		if i > 0 {
			b.WriteString("---\n")
		}
		fmt.Fprintf(&b, "id: %s%03d\n", prefix, i)
		b.WriteString("language: Java\n")
		b.WriteString("severity: hint\n")
		b.WriteString("message: codea entrypoint inventory\n")
		b.WriteString("rule:\n")
		encoded, _ := json.Marshal(pattern)
		fmt.Fprintf(&b, "  pattern: %s\n", encoded)
	}
	return b.String()
}

func exactEntrypointScopes164(scopes []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		value := strings.TrimSpace(strings.ReplaceAll(scope, "\\", "/"))
		if value == "" || path.IsAbs(value) || strings.ContainsAny(value, "\x00\r\n*?[") {
			return nil, fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: %q", scope)
		}
		clean := path.Clean(value)
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || clean != value || !strings.HasSuffix(strings.ToLower(clean), ".java") {
			return nil, fmt.Errorf("ENTRYPOINT_SCAN_SCOPE_WIDENED: %q", scope)
		}
		if seen[clean] {
			continue
		}
		seen[clean] = true
		out = append(out, clean)
	}
	sort.Strings(out)
	return out, nil
}
