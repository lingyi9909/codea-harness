package report

import "testing"

func Test152FullReviewRejectsFindingOutsideVerifiedReviewedFiles(t *testing.T) {
	req := ReviewRequest{
		RunID:          "run-152-review-isolation",
		HarnessVersion: "1.5.2",
		BaseRef:        "develop",
		Head:           "HEAD",
		Result:         ResultFailed,
		Mode:           "FULL",
		Scope: ReviewScope{ChangedFiles: []string{
			"src/main/java/com/company/order/XxxServiceImpl.java",
		}},
		Coverage: ReviewCoverage{
			ReviewedFiles: []string{
				"src/main/java/com/company/order/XxxServiceImpl.java",
			},
			CallChains: []CallChain{{
				EntryPoint: "XxxController.submit",
				Chain: []string{
					"XxxController.submit",
					"XxxServiceImpl.submit",
					"AbstractTemplate.execute",
					"XxxServiceImpl.doExecute",
				},
			}},
			Status: "COMPLETE",
		},
		Findings: []Finding{{
			ID:                 "F-1",
			Category:           "PRODUCTION_CODE",
			Severity:           "HIGH",
			File:               "../company-framework/src/main/java/com/company/framework/AbstractTemplate.java",
			Problem:            "dependency workspace must not be reviewed",
			Evidence:           "workspace dependency is navigation context only",
			Impact:             "would expand review scope outside current repository",
			Recommendation:     "keep dependency node as chain context only",
			IntroducedByChange: true,
			Confidence:         1,
		}},
	}

	if err := Validate(req); err == nil {
		t.Fatal("FULL review must reject Finding.file outside verified reviewedFiles")
	}
}
