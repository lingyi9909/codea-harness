package reviewunit

import (
	"strings"
	"testing"

	analysisruntime "codea-harness-tools/internal/analysis"
)

func TestLoadRejectsStaleCertifiedAnalysis(t *testing.T) {
	facts := baseFacts160()
	manifest, err := buildFromFacts160(facts)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := CanonicalBytes(manifest)
	if err != nil {
		t.Fatal(err)
	}

	stale := facts
	stale.cert.AnalysisSHA256 = strings.Repeat("d", 64)
	if _, err := loadCanonicalWithFacts160(encoded, stale); err == nil || !strings.Contains(err.Error(), "REVIEW_UNIT_STALE") {
		t.Fatalf("manifest bound to old Certified ChangeAnalysis must fail closed, got %v", err)
	}
}

func TestBuildRejectsTargetedCertifiedIntentWithoutVerifiedScope(t *testing.T) {
	facts := baseFacts160()
	facts.cert.Intent = &analysisruntime.Intent{Mode: "TARGETED", Target: "OrderController.approve"}

	if _, err := buildFromFacts160(facts); err == nil || !strings.Contains(err.Error(), "REVIEW_UNIT_SCOPE_VIOLATION") {
		t.Fatalf("TARGETED Certified Analysis without TARGETED verified scope must fail closed, got %v", err)
	}
}
