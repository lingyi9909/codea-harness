package reviewrun

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
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
