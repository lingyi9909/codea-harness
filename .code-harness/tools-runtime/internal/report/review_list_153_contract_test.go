package report

import (
	"strings"
	"testing"
)

func Test153ExplicitHarnessReviewListRemainsListOnly(t *testing.T) {
	text := readHarnessContract(t, "agents", "orchestrator.md")
	for _, want := range []string{
		"harness review list → LIST",
		"不调用 `review-code`",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("explicit harness review list contract missing %q", want)
		}
	}
}
