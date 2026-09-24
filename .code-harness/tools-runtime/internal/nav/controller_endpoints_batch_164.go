package nav

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

// FindControllerEndpointsBatch is the Entrypoint Inventory-specific AST path.
// It preserves the existing declaration pattern union while executing exact
// file sets in deterministic command-line-safe batches. Windows CreateProcess
// has a much smaller command-line ceiling than large repositories can require,
// so file paths must never be appended to one unbounded ast-grep invocation.
func (n Navigator) FindControllerEndpointsBatch(ctx context.Context, targets []string) ([]ControllerEndpointMatch, error) {
	cleanTargets, err := n.validateEntrypointBatchTargets164(targets)
	if err != nil {
		return nil, err
	}
	if len(cleanTargets) == 0 {
		return []ControllerEndpointMatch{}, nil
	}

	types, err := n.runRawBatch164(ctx, cleanTargets, "codea-entrypoint-types-164", allTypePatterns())
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
		return []ControllerEndpointMatch{}, nil
	}

	controllerTargets := make([]string, 0, len(controllerPathSet))
	for _, target := range cleanTargets {
		if controllerPathSet[target] {
			controllerTargets = append(controllerTargets, target)
		}
	}
	if len(controllerTargets) == 0 {
		return nil, fmt.Errorf("ENTRYPOINT_SCAN_RESULT_OUT_OF_SCOPE: controller matches did not resolve to requested files")
	}

	methods, err := n.runRawBatch164(ctx, controllerTargets, "codea-entrypoint-methods-164", allMethodPatterns())
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

func (n Navigator) validateEntrypointBatchTargets164(targets []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(targets))
	for _, raw := range targets {
		if strings.ContainsAny(raw, "\x00\r\n") {
			return nil, ErrInvalidScope
		}
		clean := strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/")
		if err := n.validate("X", clean); err != nil {
			return nil, ErrInvalidScope
		}
		clean = path.Clean(clean)
		if strings.ContainsAny(clean, "*?[") || !strings.EqualFold(path.Ext(clean), ".java") {
			return nil, ErrInvalidScope
		}
		if seen[clean] {
			return nil, ErrInvalidScope
		}
		seen[clean] = true
		out = append(out, clean)
	}
	return out, nil
}

func (n Navigator) runRawBatch164(ctx context.Context, targets []string, ruleID string, patterns []string) ([]rawMatch, error) {
	cleanTargets, err := n.validateEntrypointBatchTargets164(targets)
	if err != nil {
		return nil, err
	}
	if len(cleanTargets) == 0 || len(patterns) == 0 {
		return []rawMatch{}, nil
	}

	runner := n.Runner
	if runner == nil {
		runner = ExecRunner{}
	}
	baseArgs := []string{"scan", "--inline-rules", entrypointInlineRule164(ruleID, patterns), "--json=stream"}
	batches, err := entrypointTargetBatches164(n.AstGrepPath, baseArgs, cleanTargets)
	if err != nil {
		return nil, err
	}

	allowed := make(map[string]bool, len(cleanTargets))
	for _, target := range cleanTargets {
		allowed[target] = true
	}
	seen := map[string]bool{}
	out := make([]rawMatch, 0)
	for _, batch := range batches {
		args := append(append([]string(nil), baseArgs...), batch...)
		data, runErr := runner.Run(ctx, n.AstGrepPath, args...)
		if runErr != nil {
			var exitErr *exec.ExitError
			if !errors.As(runErr, &exitErr) {
				return nil, runErr
			}
			if len(data) == 0 {
				continue
			}
		}

		scanner := bufio.NewScanner(bytes.NewReader(data))
		scanner.Buffer(make([]byte, 64*1024), workspaceASTMaxJSONRecord)
		for scanner.Scan() {
			var line sgExtendedLine
			if json.Unmarshal(scanner.Bytes(), &line) != nil {
				continue
			}
			match := rawMatch{
				Path:        strings.ReplaceAll(line.File, "\\", "/"),
				Text:        line.Text,
				StartLine:   line.Range.Start.Line + 1,
				StartColumn: line.Range.Start.Column + 1,
				EndLine:     line.Range.End.Line + 1,
				EndColumn:   line.Range.End.Column + 1,
			}
			if match.EndLine < match.StartLine {
				match.EndLine = match.StartLine
			}
			if strings.ContainsAny(match.Path, "\x00\r\n") || !allowed[match.Path] {
				return nil, fmt.Errorf("ENTRYPOINT_SCAN_RESULT_OUT_OF_SCOPE: %s", match.Path)
			}
			key := fmt.Sprintf("%s:%d:%d:%d:%d:%s", match.Path, match.StartLine, match.StartColumn, match.EndLine, match.EndColumn, match.Text)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, match)
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

const entrypointBatchCommandBudgetUTF16 = 20 * 1024

func entrypointTargetBatches164(executable string, baseArgs, targets []string) ([][]string, error) {
	fixed := entrypointCommandUnits164(executable)
	for _, arg := range baseArgs {
		fixed += entrypointCommandUnits164(arg)
	}
	if fixed >= entrypointBatchCommandBudgetUTF16 {
		return nil, fmt.Errorf("ENTRYPOINT_SCAN_ARGUMENT_BUDGET_EXCEEDED: fixed=%d budget=%d", fixed, entrypointBatchCommandBudgetUTF16)
	}

	batches := make([][]string, 0, 1)
	current := make([]string, 0)
	currentUnits := fixed
	for _, target := range targets {
		units := entrypointCommandUnits164(target)
		if fixed+units > entrypointBatchCommandBudgetUTF16 {
			return nil, fmt.Errorf("ENTRYPOINT_SCAN_ARGUMENT_BUDGET_EXCEEDED: target=%s units=%d budget=%d", target, fixed+units, entrypointBatchCommandBudgetUTF16)
		}
		if len(current) > 0 && currentUnits+units > entrypointBatchCommandBudgetUTF16 {
			batches = append(batches, current)
			current = make([]string, 0)
			currentUnits = fixed
		}
		current = append(current, target)
		currentUnits += units
	}
	if len(current) > 0 {
		batches = append(batches, current)
	}
	return batches, nil
}

// Count UTF-16 code units because Windows CreateProcess applies its command-line
// limit after UTF-16 conversion. The small per-argument allowance covers quotes
// and separators; the 20 Ki-unit budget leaves substantial headroom below the
// Windows process command-line ceiling for escaping overhead.
func entrypointCommandUnits164(value string) int {
	return len(utf16.Encode([]rune(value))) + 3
}

func entrypointInlineRule164(ruleID string, patterns []string) string {
	var b strings.Builder
	b.WriteString("id: ")
	b.WriteString(ruleID)
	b.WriteString("\nlanguage: Java\nseverity: warning\nmessage: Codea Entrypoint Inventory batch\nrule:\n  any:\n")
	for _, pattern := range patterns {
		b.WriteString("    - pattern: ")
		b.WriteString(strconv.Quote(pattern))
		b.WriteByte('\n')
	}
	return b.String()
}
