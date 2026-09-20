package symbolid

import (
	"strings"

	"codea-harness-tools/internal/projectpath"
)

const CurrentWorkspace = "current"

type Ref struct {
	Workspace string `json:"workspace,omitempty"`
	Path      string `json:"path"`
	Symbol    string `json:"symbol"`
}

func NormalizeWorkspace(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return CurrentWorkspace
	}
	return value
}

func Normalize(ref Ref) (Ref, bool) {
	symbol := strings.TrimSpace(ref.Symbol)
	p, ok := projectpath.Normalize(ref.Path)
	if symbol == "" || !ok {
		return Ref{}, false
	}
	return Ref{Workspace: NormalizeWorkspace(ref.Workspace), Path: p, Symbol: symbol}, true
}

func Key(ref Ref) (string, bool) {
	normalized, ok := Normalize(ref)
	if !ok {
		return "", false
	}
	return normalized.Workspace + "\x00" + normalized.Path + "\x00" + normalized.Symbol, true
}

func FromLocation(workspace, path, symbol string) (Ref, bool) {
	return Normalize(Ref{Workspace: workspace, Path: path, Symbol: symbol})
}

func Same(left, right Ref) bool {
	leftKey, leftOK := Key(left)
	rightKey, rightOK := Key(right)
	return leftOK && rightOK && leftKey == rightKey
}
