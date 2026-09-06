package nav

import (
	"context"
	"errors"
	"os/exec"
	"strings"
)

// ProjectExecRunner keeps PROJECT discovery's broader Java declaration matching
// scoped to the public project-discovery path. Existing Review/AFFECTED
// navigation continues to use ExecRunner unchanged.
type ProjectExecRunner struct{}

func (ProjectExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	base := ExecRunner{}
	data, originalErr := base.Run(ctx, name, args...)
	if len(data) != 0 || originalErr == nil {
		return data, originalErr
	}
	var exitErr *exec.ExitError
	if !errors.As(originalErr, &exitErr) {
		return nil, originalErr
	}

	patternIndex := -1
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--pattern" {
			patternIndex = i + 1
			break
		}
	}
	if patternIndex < 0 {
		return data, originalErr
	}
	alternates := projectHeritagePatterns163(args[patternIndex])
	if len(alternates) == 0 {
		return data, originalErr
	}

	var combined []byte
	for _, pattern := range alternates {
		alternateArgs := append([]string(nil), args...)
		alternateArgs[patternIndex] = pattern
		out, err := base.Run(ctx, name, alternateArgs...)
		if err != nil {
			var alternateExit *exec.ExitError
			if !errors.As(err, &alternateExit) {
				return nil, err
			}
		}
		if len(out) == 0 {
			continue
		}
		combined = append(combined, out...)
		if combined[len(combined)-1] != '\n' {
			combined = append(combined, '\n')
		}
	}
	if len(combined) != 0 {
		return combined, nil
	}
	return data, originalErr
}

func projectHeritagePatterns163(pattern string) []string {
	const body = " { $$$BODY }"
	if !strings.Contains(pattern, "class ") || strings.Contains(pattern, " extends ") || strings.Contains(pattern, " implements ") || !strings.HasSuffix(pattern, body) {
		return nil
	}
	prefix := strings.TrimSuffix(pattern, body)
	return []string{
		prefix + " extends $SUPER" + body,
		prefix + " implements $$$IFACES" + body,
		prefix + " extends $SUPER implements $$$IFACES" + body,
	}
}
