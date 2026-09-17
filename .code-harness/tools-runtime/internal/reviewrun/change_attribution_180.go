package reviewrun

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var diffHunk180 = regexp.MustCompile(`^@@ -[0-9]+(?:,[0-9]+)? \+([0-9]+)(?:,([0-9]+))? @@`)

func validateChangeAttribution180(ctx context.Context, root, runDir string, req FinishRequest) error {
	hasIntroduced := false
	for _, finding := range req.Findings {
		if finding.IntroducedByChange != nil && *finding.IntroducedByChange {
			hasIntroduced = true
			break
		}
	}
	if !hasIntroduced {
		return nil
	}
	prepared, err := loadPreparedOptions180(runDir)
	if err != nil {
		return fmt.Errorf("REVIEW_FINISH_CHANGE_ATTRIBUTION_CONTEXT_MISSING: %w", err)
	}
	if prepared.Intent.Mode == "CURRENT_IMPLEMENTATION" {
		return fmt.Errorf("REVIEW_FINISH_CHANGE_ATTRIBUTION_INVALID: CURRENT_IMPLEMENTATION cannot mark findings introducedByChange")
	}
	if prepared.Intent.Mode != "CHANGES" {
		return fmt.Errorf("REVIEW_FINISH_CHANGE_ATTRIBUTION_INVALID: unsupported mode %q", prepared.Intent.Mode)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	for _, finding := range req.Findings {
		if finding.IntroducedByChange == nil || !*finding.IntroducedByChange {
			continue
		}
		proven := false
		for _, ev := range finding.Evidence {
			quoteRanges, err := evidenceQuoteRanges180(rootAbs, ev)
			if err != nil {
				return err
			}
			changed, err := changedLineRangesForPath180(ctx, rootAbs, ev.Ref.Path)
			if err != nil {
				return fmt.Errorf("REVIEW_FINISH_CHANGE_ATTRIBUTION_DIFF_FAILED: %w", err)
			}
			for _, qr := range quoteRanges {
				if overlapsChangedRange180(qr, changed) {
					proven = true
					break
				}
			}
			if proven {
				break
			}
		}
		if !proven {
			return fmt.Errorf("REVIEW_FINISH_CHANGE_ATTRIBUTION_UNPROVEN: %s", finding.ID)
		}
	}
	return nil
}

func changedLineRangesForPath180(ctx context.Context, root, rel string) ([]ReadRef, error) {
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(rel)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return nil, fmt.Errorf("invalid change path %q", rel)
	}
	untracked := exec.CommandContext(ctx, "git", "ls-files", "--others", "--exclude-standard", "--", clean)
	untracked.Dir = root
	untrackedOut, err := untracked.Output()
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(strings.TrimSpace(string(untrackedOut)), "\n") {
		if filepath.ToSlash(strings.TrimSpace(line)) == clean {
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(clean)))
			if err != nil {
				return nil, err
			}
			lines := bytes.Count(data, []byte("\n"))
			if len(data) == 0 || data[len(data)-1] != '\n' {
				lines++
			}
			if lines < 1 {
				lines = 1
			}
			return []ReadRef{{Path: clean, StartLine: 1, EndLine: lines}}, nil
		}
	}

	cmd := exec.CommandContext(ctx, "git", "diff", "--unified=0", "--no-ext-diff", "--no-color", "HEAD", "--", clean)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	refs := []ReadRef{}
	for _, line := range strings.Split(string(out), "\n") {
		m := diffHunk180.FindStringSubmatch(line)
		if len(m) == 0 {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		count := 1
		if m[2] != "" {
			count, _ = strconv.Atoi(m[2])
		}
		if count <= 0 {
			continue
		}
		refs = append(refs, ReadRef{Path: clean, StartLine: start, EndLine: start + count - 1})
	}
	return refs, nil
}

func overlapsChangedRange180(ref ReadRef, changed []ReadRef) bool {
	path := filepath.ToSlash(filepath.Clean(ref.Path))
	for _, ch := range changed {
		if filepath.ToSlash(filepath.Clean(ch.Path)) != path {
			continue
		}
		if ref.EndLine >= ch.StartLine && ref.StartLine <= ch.EndLine {
			return true
		}
	}
	return false
}
