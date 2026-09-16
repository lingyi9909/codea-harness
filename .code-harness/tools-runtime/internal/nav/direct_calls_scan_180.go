package nav

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
)

const directCallsRules180 = `id: codea-direct-calls-180-types
language: Java
rule:
  any:
    - kind: class_declaration
    - kind: interface_declaration
    - kind: enum_declaration
---
id: codea-direct-calls-180-methods
language: Java
rule:
  kind: method_declaration
---
id: codea-direct-calls-180-calls
language: Java
rule:
  kind: method_invocation
`

// runDirectCallsBatch180 keeps declaration and invocation matches independent
// while still using exactly one ast-grep process. Nested method invocations must
// not disappear merely because their enclosing method also matched.
func (n Navigator) runDirectCallsBatch180(ctx context.Context, scope string) ([]rawMatch, error) {
	if err := n.validate("X", scope); err != nil {
		return nil, err
	}
	r := n.Runner
	if r == nil {
		r = ExecRunner{}
	}
	cleanScope := strings.ReplaceAll(scope, "\\", "/")
	b, err := r.Run(ctx, n.AstGrepPath, "scan", "--inline-rules", directCallsRules180, "--json=stream", cleanScope)
	if err != nil {
		return nil, err
	}

	allowed := map[string]bool{
		"codea-direct-calls-180-types":   true,
		"codea-direct-calls-180-methods": true,
		"codea-direct-calls-180-calls":   true,
	}
	seen := map[string]bool{}
	out := make([]rawMatch, 0)
	s := bufio.NewScanner(bytes.NewReader(b))
	s.Buffer(make([]byte, 64*1024), workspaceASTMaxJSONRecord)
	for s.Scan() {
		if len(bytes.TrimSpace(s.Bytes())) == 0 {
			return nil, fmt.Errorf("malformed ast-grep direct-call output: empty JSON record")
		}
		var x batchRecord167
		if err := json.Unmarshal(s.Bytes(), &x); err != nil {
			return nil, fmt.Errorf("malformed ast-grep direct-call output: %w", err)
		}
		if !allowed[x.RuleID] {
			return nil, fmt.Errorf("malformed ast-grep direct-call output: unexpected ruleId %q", x.RuleID)
		}
		if strings.TrimSpace(x.File) == "" || strings.TrimSpace(x.Text) == "" {
			return nil, fmt.Errorf("malformed ast-grep direct-call output: missing file or text")
		}
		if x.Range == nil || x.Range.Start == nil || x.Range.End == nil || x.Range.Start.Line == nil || x.Range.Start.Column == nil || x.Range.End.Line == nil || x.Range.End.Column == nil {
			return nil, fmt.Errorf("malformed ast-grep direct-call output: missing range")
		}
		start, end := x.Range.Start, x.Range.End
		if *start.Line < 0 || *start.Column < 0 || *end.Line < *start.Line || *end.Column < 0 || (*end.Line == *start.Line && *end.Column < *start.Column) {
			return nil, fmt.Errorf("malformed ast-grep direct-call output: invalid range")
		}
		resultPath := path.Clean(strings.ReplaceAll(x.File, "\\", "/"))
		requestedScope := path.Clean(cleanScope)
		if resultPath != requestedScope && !strings.HasPrefix(resultPath, requestedScope+"/") {
			return nil, fmt.Errorf("ast-grep direct-call result outside requested scope: %s", x.File)
		}
		m := rawMatch{
			Path:        strings.ReplaceAll(x.File, "\\", "/"),
			Text:        x.Text,
			RuleID:      x.RuleID,
			StartLine:   *start.Line + 1,
			StartColumn: *start.Column + 1,
			EndLine:     *end.Line + 1,
			EndColumn:   *end.Column + 1,
		}
		key := fmt.Sprintf("%s:%s:%d:%d:%d:%d:%s", m.RuleID, m.Path, m.StartLine, m.StartColumn, m.EndLine, m.EndColumn, m.Text)
		if !seen[key] {
			seen[key] = true
			out = append(out, m)
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
