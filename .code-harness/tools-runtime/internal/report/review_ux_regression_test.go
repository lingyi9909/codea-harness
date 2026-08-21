package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewCodeSkillContractMatches132FindingRules(t *testing.T) {
	path := filepath.Join("..", "..", "..", "skills", "review-code", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"version: 3",
		"PRODUCTION_CODE",
		"TEST_VALIDITY",
		"category",
		"problem",
		"测试代码默认不得产生普通 Finding",
		"problem / evidence / impact / recommendation",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("review-code skill missing %q", want)
		}
	}
}

func TestCallChainEntryPointAlreadyFirstIsNotDuplicated(t *testing.T) {
	req := sampleRequest()
	req.Coverage.CallChains = []CallChain{{
		EntryPoint: "OrderController.approve",
		Chain: []string{
			"OrderController.approve",
			"OrderService.approve",
		},
	}}
	md, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(md, "`OrderController.approve`") != 1 {
		t.Fatalf("entryPoint duplicated in markdown:\n%s", md)
	}
	if !strings.Contains(md, "`OrderController.approve`\n↓\n`OrderService.approve`") {
		t.Fatalf("normalized call chain missing:\n%s", md)
	}
}

func TestCallChainEntryPointMissingFromChainIsPrepended(t *testing.T) {
	req := sampleRequest()
	req.Coverage.CallChains = []CallChain{{
		EntryPoint: "OrderController.approve",
		Chain: []string{
			"OrderService.approve",
			"OrderServiceImpl.approve",
		},
	}}
	md, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	want := "`OrderController.approve`\n↓\n`OrderService.approve`\n↓\n`OrderServiceImpl.approve`"
	if !strings.Contains(md, want) {
		t.Fatalf("entryPoint was not prepended:\n%s", md)
	}
}

func TestCallChainEmptyChainStillRendersEntryPoint(t *testing.T) {
	req := sampleRequest()
	req.Coverage.CallChains = []CallChain{{EntryPoint: "OrderController.approve"}}
	md, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "### 调用链 1\n\n`OrderController.approve`") {
		t.Fatalf("entryPoint-only call chain missing:\n%s", md)
	}
}

func TestReviewReportFixedUITextIsChinese(t *testing.T) {
	md, err := Render(sampleRequest())
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"普通代码质量 Review",
		"| HEAD |",
	} {
		if strings.Contains(md, forbidden) {
			t.Fatalf("fixed UI still contains %q:\n%s", forbidden, md)
		}
	}
	for _, want := range []string{
		"普通代码质量评审",
		"| 当前提交 | abc123 |",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("fixed Chinese UI missing %q:\n%s", want, md)
		}
	}
}
