package reviewrun

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test180ReportGoldens(t *testing.T) {
	for name, view := range reportGoldenScenarios180() {
		t.Run(name, func(t *testing.T) {
			got, err := renderReport(view)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join("testdata", "reports", name+".golden.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("report golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
			}
			for _, heading := range []string{
				"## 评审摘要",
				"## 调用链与覆盖范围",
				"## 风险清单",
				"## 问题明细",
				"## 后续处理与评审边界",
			} {
				if !bytes.Contains(got, []byte(heading)) {
					t.Fatalf("%s missing five-section heading %q", name, heading)
				}
			}
		})
	}
}

func Test180ConclusionMatrix(t *testing.T) {
	cases := []struct {
		name     string
		coverage string
		req      FinishRequest
		want     string
	}{
		{name: "partial high stays undetermined", coverage: "PARTIAL", req: FinishRequest{Findings: []Finding{{Severity: "HIGH"}}}, want: "UNDETERMINED"},
		{name: "complete critical blocks", coverage: "COMPLETE", req: FinishRequest{Findings: []Finding{{Severity: "CRITICAL"}}}, want: "BLOCKING"},
		{name: "complete high blocks", coverage: "COMPLETE", req: FinishRequest{Findings: []Finding{{Severity: "HIGH"}}}, want: "BLOCKING"},
		{name: "complete medium requires action", coverage: "COMPLETE", req: FinishRequest{Findings: []Finding{{Severity: "MEDIUM"}}}, want: "ACTION_REQUIRED"},
		{name: "complete low requires action", coverage: "COMPLETE", req: FinishRequest{Findings: []Finding{{Severity: "LOW"}}}, want: "ACTION_REQUIRED"},
		{name: "pending risk requires action", coverage: "COMPLETE", req: FinishRequest{PendingRisks: []string{"verify upstream guard"}}, want: "ACTION_REQUIRED"},
		{name: "complete empty is scoped no issues", coverage: "COMPLETE", req: FinishRequest{}, want: "NO_ISSUES_FOUND"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := conclusionFor(tc.coverage, tc.req); got != tc.want {
				t.Fatalf("conclusionFor(%s)=%s want %s", tc.coverage, got, tc.want)
			}
		})
	}
}

func Test180ReportSummaryCountsAreDerivedFromFindings(t *testing.T) {
	view := reportGoldenBase180()
	view.ReviewConclusion = "BLOCKING"
	view.Findings = []Finding{
		{ID: "F4", Severity: "LOW", Problem: "low", Impact: "i", Recommendation: "r", Verification: "v", Evidence: []Evidence{{Ref: ReadRef{Path: "A.java", StartLine: 1, EndLine: 1}, Quote: "a"}}},
		{ID: "F1", Severity: "CRITICAL", Problem: "critical", Impact: "i", Recommendation: "r", Verification: "v", Evidence: []Evidence{{Ref: ReadRef{Path: "A.java", StartLine: 1, EndLine: 1}, Quote: "a"}}},
		{ID: "F2", Severity: "HIGH", Problem: "high", Impact: "i", Recommendation: "r", Verification: "v", Evidence: []Evidence{{Ref: ReadRef{Path: "A.java", StartLine: 1, EndLine: 1}, Quote: "a"}}},
		{ID: "F3", Severity: "MEDIUM", Problem: "medium", Impact: "i", Recommendation: "r", Verification: "v", Evidence: []Evidence{{Ref: ReadRef{Path: "A.java", StartLine: 1, EndLine: 1}, Quote: "a"}}},
	}
	view.PendingRisks = []string{"pending one", "pending two"}
	got, err := renderReport(view)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"风险概览 | 严重 1 · 高 1 · 中 1 · 低 1",
		"待确认风险 | 2 项",
	} {
		if !bytes.Contains(got, []byte(want)) {
			t.Fatalf("derived summary missing %q:\n%s", want, got)
		}
	}
	if bytes.Contains(got, []byte("严重 3")) || bytes.Contains(got, []byte("高 3")) {
		t.Fatalf("pending risks were counted as confirmed severity: %s", got)
	}
}

func Test180ReportEscapesDisplayContentAndKeepsEvidenceLiteral(t *testing.T) {
	dangerous := "pipe | `tick` <script>alert(1)</script> ![remote](https://example.test/a.png)"
	escaped := markdownText180(dangerous)
	for _, forbidden := range []string{"<script>", "![remote](", "| `tick`"} {
		if strings.Contains(escaped, forbidden) {
			t.Fatalf("display content remained active: %q", escaped)
		}
	}
	if !strings.Contains(escaped, "\\|") || !strings.Contains(escaped, "&lt;script&gt;") || !strings.Contains(escaped, "\\!") {
		t.Fatalf("display escaping incomplete: %q", escaped)
	}

	quote := "value := ```danger```\n<script>alert(1)</script>"
	block := markdownCodeBlock180(quote)
	if !strings.HasPrefix(block, "````\n") || !strings.HasSuffix(block, "\n````") {
		t.Fatalf("evidence fence did not grow around embedded backticks: %q", block)
	}

	path := "C:\\工作区\\模块|A\\Order`Mapper.xml"
	span := markdownCodeSpan180(path)
	if !strings.Contains(span, path) || !strings.HasPrefix(span, "``") {
		t.Fatalf("Windows/Chinese/backtick path was not represented as a safe code span: %q", span)
	}
}

func Test180ReportLongChainIsCompleteAndUnselectedNodesStayOutOfCheckedList(t *testing.T) {
	view := reportGoldenBase180()
	longNodes := make([]Node, 0, 24)
	for i := 0; i < 24; i++ {
		longNodes = append(longNodes, Node{
			Path:   fmt.Sprintf("src/main/java/com/example/Layer%02d.java", i),
			Symbol: fmt.Sprintf("Layer%02d.call", i),
			Role:   "SERVICE",
		})
	}
	view.Chains = []Chain{
		{ID: "C1", Name: "long-selected", Nodes: longNodes},
		{ID: "C2", Name: "not-selected", Nodes: []Node{{Path: "secret/Unselected.java", Symbol: "Unselected.mustNotAppear", Role: "SERVICE"}}},
	}
	view.SelectedIDs = []string{"C1"}
	view.ReadFileCount = 24
	view.ReviewConclusion = "NO_ISSUES_FOUND"
	got, err := renderReport(view)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 24; i++ {
		want := fmt.Sprintf("Layer%02d.call", i)
		if !bytes.Contains(got, []byte(want)) {
			t.Fatalf("long chain truncated at %s", want)
		}
	}
	if bytes.Contains(got, []byte("Unselected.mustNotAppear")) || bytes.Contains(got, []byte("secret/Unselected.java")) {
		t.Fatalf("unselected node leaked into checked-chain detail:\n%s", got)
	}
	if !bytes.Contains(got, []byte("C2 not-selected")) {
		t.Fatalf("unselected chain boundary disappeared:\n%s", got)
	}
}

func Test180ReportWaitingSelectionShowsCandidatesWithoutCheckedNodes(t *testing.T) {
	view := reportGoldenBase180()
	view.StatusText = "评审未完成"
	view.Execution = "INCOMPLETE"
	view.Coverage = "PARTIAL"
	view.ReviewConclusion = "UNDETERMINED"
	view.SelectedIDs = []string{}
	view.WaitingSelection = true
	view.ReadFileCount = 0
	view.ResultSHA256 = ""
	got, err := renderReport(view)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("等待真实用户选择调用链")) {
		t.Fatalf("waiting-selection state missing:\n%s", got)
	}
	if bytes.Contains(got, []byte("OrderServiceImpl.create")) || bytes.Contains(got, []byte("OrderMapper.insertOrder")) {
		t.Fatalf("waiting candidate nodes were rendered as checked detail:\n%s", got)
	}
}

func Test180ReportZeroOptionsAndCurrentImplementationMode(t *testing.T) {
	noChanges := reportGoldenBase180()
	noChanges.StatusText = "评审未完成"
	noChanges.Execution = "INCOMPLETE"
	noChanges.ReviewConclusion = "UNDETERMINED"
	noChanges.Coverage = "PARTIAL"
	noChanges.Intent = Intent{Mode: "CHANGES", Target: "OrderController.create"}
	noChanges.Chains = []Chain{}
	noChanges.SelectedIDs = []string{}
	noChanges.ReadFileCount = 0
	noChanges.Gaps = []string{"NO_RELEVANT_CHANGES"}
	noChanges.ResultSHA256 = ""
	got, err := renderReport(noChanges)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("变更评审")) || !bytes.Contains(got, []byte("NO\_RELEVANT\_CHANGES")) {
		t.Fatalf("zero-option changes report lost mode/gap:\n%s", got)
	}
	if bytes.Contains(got, []byte("NO_ISSUES_FOUND")) {
		t.Fatalf("zero-option/incomplete changes report claimed no issues:\n%s", got)
	}

	current := reportGoldenBase180()
	current.Intent = Intent{Mode: "CURRENT_IMPLEMENTATION", Target: "OrderController.create"}
	current.ReviewConclusion = "NO_ISSUES_FOUND"
	got, err = renderReport(current)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("当前实现评审")) {
		t.Fatalf("current implementation mode missing:\n%s", got)
	}
}

func reportGoldenBase180() reportView {
	return reportView{
		StatusText:       "评审完成",
		RunID:            "review-00000000000000000000000000000001",
		Project:          "demo",
		ReportPath:       `C:\工作区\demo\.code-harness\runs\review-00000000000000000000000000000001\review.md`,
		Execution:        "COMPLETE",
		Coverage:         "COMPLETE",
		Intent:           Intent{Mode: "CHANGES", Target: "OrderController.create"},
		Chains:           reportGoldenChains180(),
		SelectedIDs:      []string{"C1"},
		ReadFileCount:    4,
		ResultSHA256:     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Findings:         []Finding{},
		PendingRisks:     []string{},
		Gaps:             []string{},
	}
}

func reportGoldenChains180() []Chain {
	return []Chain{
		{ID: "C1", Name: "OrderController.create", Nodes: []Node{
			{Path: "src/main/java/com/example/OrderController.java", Symbol: "OrderController.create", Role: "CONTROLLER"},
			{Path: "src/main/java/com/example/OrderServiceImpl.java", Symbol: "OrderServiceImpl.create", Role: "SERVICE"},
			{Path: "src/main/java/com/example/OrderMapper.java", Symbol: "OrderMapper.insertOrder", Role: "MAPPER"},
			{Path: "src/main/resources/mapper/OrderMapper.xml", Symbol: "OrderMapper.insertOrder", Role: "SQL"},
		}},
		{ID: "C2", Name: "OrderController.cancel", Nodes: []Node{
			{Path: "src/main/java/com/example/OrderController.java", Symbol: "OrderController.cancel", Role: "CONTROLLER"},
		}},
	}
}

func reportGoldenScenarios180() map[string]reportView {
	blocking := reportGoldenBase180()
	blocking.ReviewConclusion = "BLOCKING"
	blocking.Findings = []Finding{{
		ID:             "F-001",
		Severity:       "HIGH",
		Problem:        "更新缺少租户隔离",
		Impact:         "可能修改其他租户数据",
		Recommendation: "绑定可信租户条件",
		Verification:   "跨租户请求必须失败",
		Evidence: []Evidence{{Ref: ReadRef{
			Path: "src/main/resources/mapper/OrderMapper.xml", StartLine: 18, EndLine: 21,
		}, Quote: "UPDATE orders\nSET status = #{status}\nWHERE id = #{orderId}"}},
	}}

	action := reportGoldenBase180()
	action.ReviewConclusion = "ACTION_REQUIRED"
	action.Findings = []Finding{{
		ID:             "F-002",
		Severity:       "MEDIUM",
		Problem:        "异常路径缺少重试边界",
		Impact:         "瞬时失败可能直接返回",
		Recommendation: "明确重试或失败语义",
		Verification:   "注入瞬时失败验证",
		Evidence: []Evidence{{Ref: ReadRef{
			Path: "src/main/java/com/example/OrderServiceImpl.java", StartLine: 12, EndLine: 12,
		}, Quote: "orderMapper.insertOrder();"}},
	}}
	action.PendingRisks = []string{"需要确认上游是否已有统一重试"}

	clean := reportGoldenBase180()
	clean.ReviewConclusion = "NO_ISSUES_FOUND"

	partial := reportGoldenBase180()
	partial.Coverage = "PARTIAL"
	partial.ReviewConclusion = "UNDETERMINED"
	partial.Gaps = []string{"OrderMapper dynamic SQL unresolved"}

	waiting := reportGoldenBase180()
	waiting.StatusText = "评审未完成"
	waiting.Execution = "INCOMPLETE"
	waiting.Coverage = "PARTIAL"
	waiting.ReviewConclusion = "UNDETERMINED"
	waiting.SelectedIDs = []string{}
	waiting.ReadFileCount = 0
	waiting.WaitingSelection = true
	waiting.ResultSHA256 = ""

	cancelled := reportGoldenBase180()
	cancelled.StatusText = "评审已取消"
	cancelled.Execution = "CANCELLED"
	cancelled.Coverage = "PARTIAL"
	cancelled.ReviewConclusion = "UNDETERMINED"
	cancelled.SelectedIDs = []string{}
	cancelled.ReadFileCount = 0
	cancelled.CancelReason = "用户取消"
	cancelled.ResultSHA256 = ""

	return map[string]reportView{
		"blocking":          blocking,
		"action-required":   action,
		"no-issues":         clean,
		"partial":           partial,
		"waiting-selection": waiting,
		"cancelled":         cancelled,
	}
}
