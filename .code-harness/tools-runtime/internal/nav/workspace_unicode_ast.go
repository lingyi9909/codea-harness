package nav

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// workspaceHasJavaUnicodeEscape is deliberately only a conservative candidate
// signal. A matching escape never becomes semantic evidence by itself; the
// normalized source is always verified by ast-grep before Runtime reports an
// ambiguity.
func workspaceHasJavaUnicodeEscape(data []byte) bool {
	return bytes.Contains(data, []byte(`\u`))
}

// workspaceUnicodeEscapedTypeConflict verifies candidate files that may have
// hidden a type identifier behind a Java Unicode escape. ast-grep 0.42.1 does
// not apply javac's Unicode-escape preprocessing to raw source, so Runtime
// presents only these exceptional files through a normalized temporary view
// and still requires an AST declaration match. Ordinary files never take this
// path and the Workspace tree is not rescanned per pattern.
func (n Navigator) workspaceUnicodeEscapedTypeConflict(
	ctx context.Context,
	candidates []string,
	confirmed []workspaceTypeMatch,
	patterns ...string,
) (bool, error) {
	if len(candidates) == 0 || len(patterns) == 0 {
		return false, nil
	}
	rootAbs, sourceRoot, exists, err := n.workspaceRoots()
	if err != nil || !exists {
		return false, err
	}
	normalizedCandidates, err := workspaceNormalizeTargets(rootAbs, sourceRoot, candidates)
	if err != nil {
		return false, err
	}
	confirmedPaths := make(map[string]bool, len(confirmed))
	for _, typ := range confirmed {
		confirmedPaths[filepath.ToSlash(filepath.Clean(typ.Path))] = true
	}

	tempDir := ""
	var tempTargets []string
	tempToOriginal := map[string]string{}
	for _, candidate := range normalizedCandidates {
		rel, err := workspaceRelativePath(rootAbs, candidate)
		if err != nil {
			return false, err
		}
		if confirmedPaths[filepath.ToSlash(filepath.Clean(rel))] {
			continue
		}
		data, err := os.ReadFile(candidate)
		if err != nil {
			return false, err
		}
		if !workspaceHasJavaUnicodeEscape(data) {
			continue
		}
		normalized, changed := workspaceNormalizeJavaUnicodeEscapes(data)
		if !changed {
			continue
		}
		if tempDir == "" {
			tempDir, err = os.MkdirTemp("", "codea-workspace-unicode-ast-")
			if err != nil {
				return false, err
			}
			defer os.RemoveAll(tempDir)
		}
		tempPath := filepath.Join(tempDir, fmt.Sprintf("candidate-%06d.java", len(tempTargets)))
		if err := os.WriteFile(tempPath, normalized, 0o600); err != nil {
			return false, err
		}
		tempPath = filepath.Clean(tempPath)
		tempTargets = append(tempTargets, tempPath)
		tempToOriginal[tempPath] = filepath.ToSlash(rel)
	}
	if len(tempTargets) == 0 {
		return false, nil
	}

	runner := n.Runner
	if runner == nil {
		runner = ExecRunner{}
	}
	for _, pattern := range patterns {
		args := []string{"--lang", "java", "--json=stream", "--pattern", pattern}
		args = append(args, tempTargets...)
		data, runErr := runner.Run(ctx, n.AstGrepPath, args...)
		if runErr != nil {
			var exitErr *exec.ExitError
			if !errors.As(runErr, &exitErr) {
				return false, runErr
			}
			if len(data) == 0 {
				continue
			}
		}
		scanner := bufio.NewScanner(bytes.NewReader(data))
		scanner.Buffer(make([]byte, 64*1024), workspaceASTMaxJSONRecord)
		for scanner.Scan() {
			var line workspaceSGLine
			if json.Unmarshal(scanner.Bytes(), &line) != nil {
				continue
			}
			clean := filepath.Clean(line.File)
			if !filepath.IsAbs(clean) {
				if abs, absErr := filepath.Abs(clean); absErr == nil {
					clean = filepath.Clean(abs)
				}
			}
			if _, ok := tempToOriginal[clean]; ok {
				return true, nil
			}
		}
		if err := scanner.Err(); err != nil {
			return false, err
		}
	}
	return false, nil
}

func workspaceNormalizeJavaUnicodeEscapes(data []byte) ([]byte, bool) {
	if !workspaceHasJavaUnicodeEscape(data) {
		return data, false
	}
	var out strings.Builder
	out.Grow(len(data))
	changed := false
	for i := 0; i < len(data); {
		if data[i] == '\\' && i+2 < len(data) && data[i+1] == 'u' {
			j := i + 1
			for j < len(data) && data[j] == 'u' {
				j++
			}
			if j+4 <= len(data) {
				hex := string(data[j : j+4])
				if value, err := strconv.ParseUint(hex, 16, 16); err == nil {
					out.WriteRune(rune(value))
					i = j + 4
					changed = true
					continue
				}
			}
		}
		out.WriteByte(data[i])
		i++
	}
	if !changed {
		return data, false
	}
	return []byte(out.String()), true
}
