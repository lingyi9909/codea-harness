package report

import (
	"strings"
	"testing"
)

func TestTargetedHeaderShowsChangedFilesOutsideScope(t *testing.T) {
	req := sampleRequest()
	req.Mode = "TARGETED"
	req.Target = &ReviewTarget{Symbol: "OrderController.approve", Kind: "METHOD"}
	req.Scope.ScopedFiles = []string{"src/main/java/OrderController.java", "src/main/java/OrderServiceImpl.java"}
	req.Coverage.CallChains = []CallChain{{EntryPoint: "OrderController.approve", Chain: []string{"OrderController.approve", "OrderService.approve"}}}
	md, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "| 未纳入本次定向评审 | 1 |") {
		t.Fatalf("targeted exclusion count missing:\n%s", md)
	}
}
