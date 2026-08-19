package coverage_test

import (
	"codea-harness-tools/internal/coverage"
	"testing"
)

type golden struct {
	name              string
	changed, reviewed []string
	unresolved        []string
	want              string
}

func TestReviewCoverageGoldenCases(t *testing.T) {
	cases := []golden{
		{"changed controller service repository", []string{"OrderController.java", "OrderServiceImpl.java", "OrderRepository.java"}, []string{"OrderController.java", "OrderServiceImpl.java", "OrderRepository.java"}, nil, "COMPLETE"},
		{"service only still expands upstream and downstream", []string{"OrderServiceImpl.java"}, []string{"OrderServiceImpl.java", "OrderController.java", "OrderRepository.java"}, nil, "COMPLETE"},
		{"interface resolves implementation", []string{"OrderController.java"}, []string{"OrderController.java", "OrderService.java", "OrderServiceImpl.java", "OrderRepository.java"}, nil, "COMPLETE"},
		{"multi service chain", []string{"OrderController.java"}, []string{"OrderController.java", "ServiceA.java", "ServiceB.java", "OrderRepository.java"}, nil, "COMPLETE"},
		{"unresolved implementation", []string{"OrderController.java"}, []string{"OrderController.java", "OrderService.java"}, []string{"OrderService.approve"}, "PARTIAL"},
		{"external boundary is resolved by classification", []string{"OrderController.java"}, []string{"OrderController.java", "OrderServiceImpl.java"}, nil, "COMPLETE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := coverage.Evaluate(tc.changed, tc.reviewed, tc.unresolved)
			if got.Status != tc.want {
				t.Fatalf("status=%s missing=%v unresolved=%v", got.Status, got.MissingChangedFiles, got.UnresolvedSymbols)
			}
		})
	}
}
