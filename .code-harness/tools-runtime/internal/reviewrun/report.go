package reviewrun

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"sort"
	"strings"
	"text/template"
	"unicode/utf8"
)

//go:embed review.md.tmpl
var reviewTemplateText string

var reviewTemplate = template.Must(template.New("review.md").Funcs(template.FuncMap{
	"cell":              markdownText180,
	"code":              markdownCodeSpan180,
	"codeblock":         markdownCodeBlock180,
	"severityLabel":     severityLabel180,
	"executionLabel":    executionLabel180,
	"conclusionLabel":   conclusionLabel180,
	"coverageLabel":     coverageLabel180,
	"modeLabel":         modeLabel180,
	"targetLabel":       targetLabel180,
	"roleLabel":         roleLabel180,
	"isSelected":        isSelected180,
	"selectedCount":     selectedCount180,
	"unselectedCount":   unselectedCount180,
	"scopeSummary":      scopeSummary180,
	"riskCount":         riskCount180,
	"topFindings":       topFindings180,
	"firstAction":       firstAction180,
	"lineRange":         lineRange180,
	"unselectedSummary": unselectedSummary180,
	"add1":              addOne180,
}).Parse(reviewTemplateText))

type reportView struct {
	StatusText       string
	RunID            string
	Project          string
	ReportPath       string
	CreatedAt        string
	CompletedAt      string
	Execution        string
	ReviewConclusion string
	Coverage         string
	CancelReason     string
	ResultSHA256     string
	Findings         []Finding
	PendingRisks     []string
	Gaps             []string
	Intent           Intent
	Chains           []Chain
	SelectedIDs      []string
	ReadFileCount    int
	WaitingSelection bool
}

type reportMeta struct {
	SchemaVersion int    `json:"schemaVersion"`
	RunID         string `json:"runId"`
	Execution     string `json:"execution"`
	ResultSHA256  string `json:"resultSha256"`
}

const reportMetaPrefix = "<!-- codea-review-meta "

func renderReport(view reportView) ([]byte, error) {
	view.Findings = append([]Finding{}, view.Findings...)
	sort.SliceStable(view.Findings, func(i, j int) bool {
		ri, rj := severityRank180(view.Findings[i].Severity), severityRank180(view.Findings[j].Severity)
		if ri != rj {
			return ri < rj
		}
		pi, pj := firstEvidencePath180(view.Findings[i]), firstEvidencePath180(view.Findings[j])
		if pi != pj {
			return pi < pj
		}
		return view.Findings[i].ID < view.Findings[j].ID
	})
	if view.Findings == nil {
		view.Findings = []Finding{}
	}
	if view.PendingRisks == nil {
		view.PendingRisks = []string{}
	}
	if view.Gaps == nil {
		view.Gaps = []string{}
	}
	if view.Chains == nil {
		view.Chains = []Chain{}
	}
	if view.SelectedIDs == nil {
		view.SelectedIDs = []string{}
	}

	var buf bytes.Buffer
	if err := reviewTemplate.Execute(&buf, view); err != nil {
		return nil, fmt.Errorf("REVIEW_REPORT_RENDER_FAILED: %w", err)
	}
	out := buf.Bytes()
	if bytes.HasPrefix(out, []byte{0xef, 0xbb, 0xbf}) {
		return nil, fmt.Errorf("REVIEW_REPORT_BOM_FORBIDDEN")
	}
	if !utf8.Valid(out) {
		return nil, fmt.Errorf("REVIEW_REPORT_UTF8_INVALID")
	}
	return out, nil
}

func parseReportMeta(data []byte) (reportMeta, error) {
	text := strings.TrimSpace(string(data))
	idx := strings.LastIndex(text, reportMetaPrefix)
	if idx < 0 {
		return reportMeta{}, fmt.Errorf("REVIEW_REPORT_META_MISSING")
	}
	tail := text[idx+len(reportMetaPrefix):]
	end := strings.Index(tail, " -->")
	if end < 0 {
		return reportMeta{}, fmt.Errorf("REVIEW_REPORT_META_INVALID")
	}
	raw := tail[:end]
	var meta reportMeta
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&meta); err != nil {
		return reportMeta{}, fmt.Errorf("REVIEW_REPORT_META_INVALID: %w", err)
	}
	if meta.SchemaVersion != SchemaVersion || meta.RunID == "" {
		return reportMeta{}, fmt.Errorf("REVIEW_REPORT_META_INVALID")
	}
	return meta, nil
}

func markdownText180(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = strings.ReplaceAll(value, "\n", " ")
	value = html.EscapeString(value)
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"`", "\\`",
		"*", "\\*",
		"_", "\\_",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"|", "\\|",
		"!", "\\!",
		"#", "\\#",
		"~", "\\~",
	)
	return replacer.Replace(value)
}

func markdownCodeSpan180(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = strings.ReplaceAll(value, "\n", " ")
	// A pipe remains a Markdown table delimiter even inside many inline-code
	// renderers. Escape it before fencing so source paths/symbols cannot add
	// columns while their visible text remains literal.
	value = strings.ReplaceAll(value, "|", "\\|")
	maxRun := maxBacktickRun180(value)
	fence := strings.Repeat("`", maxRun+1)
	if maxRun == 0 {
		fence = "`"
	}
	if strings.HasPrefix(value, "`") || strings.HasSuffix(value, "`") || strings.HasPrefix(value, " ") || strings.HasSuffix(value, " ") {
		return fence + " " + value + " " + fence
	}
	return fence + value + fence
}

func markdownCodeBlock180(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	fenceLen := maxBacktickRun180(value) + 1
	if fenceLen < 3 {
		fenceLen = 3
	}
	fence := strings.Repeat("`", fenceLen)
	return fence + "\n" + value + "\n" + fence
}

func maxBacktickRun180(value string) int {
	maxRun := 0
	current := 0
	for _, r := range value {
		if r == '`' {
			current++
			if current > maxRun {
				maxRun = current
			}
		} else {
			current = 0
		}
	}
	return maxRun
}

func severityRank180(severity string) int {
	switch strings.ToUpper(strings.TrimSpace(severity)) {
	case "CRITICAL":
		return 0
	case "HIGH":
		return 1
	case "MEDIUM":
		return 2
	case "LOW":
		return 3
	default:
		return 4
	}
}

func severityLabel180(severity string) string {
	switch strings.ToUpper(strings.TrimSpace(severity)) {
	case "CRITICAL":
		return "严重"
	case "HIGH":
		return "高"
	case "MEDIUM":
		return "中"
	case "LOW":
		return "低"
	default:
		return "未知"
	}
}

func executionLabel180(execution string) string {
	switch execution {
	case "COMPLETE":
		return "已完成"
	case "CANCELLED":
		return "已取消"
	default:
		return "未完成"
	}
}

func conclusionLabel180(conclusion string) string {
	switch conclusion {
	case "BLOCKING":
		return "🔴 存在阻断问题"
	case "ACTION_REQUIRED":
		return "🟠 需要处理"
	case "NO_ISSUES_FOUND":
		return "🟢 声明范围内未发现需处理问题"
	default:
		return "⚪ 未形成正式结论"
	}
}

func coverageLabel180(coverage string) string {
	if coverage == "COMPLETE" {
		return "完整"
	}
	return "部分 / 未完成"
}

func modeLabel180(mode string) string {
	switch strings.ToUpper(strings.TrimSpace(mode)) {
	case "CHANGES":
		return "变更评审"
	case "CURRENT_IMPLEMENTATION":
		return "当前实现评审"
	default:
		return "尚未确定"
	}
}

func targetLabel180(target string) string {
	if strings.TrimSpace(target) == "" {
		return "尚未确定"
	}
	return markdownCodeSpan180(target)
}

func roleLabel180(role string) string {
	switch strings.ToUpper(strings.TrimSpace(role)) {
	case "CONTROLLER":
		return "接口入口"
	case "SERVICE":
		return "业务实现"
	case "MAPPER":
		return "数据访问"
	case "SQL":
		return "SQL 执行"
	default:
		return strings.TrimSpace(role)
	}
}

func isSelected180(ids []string, id string) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

func selectedCount180(chains []Chain, ids []string) int {
	count := 0
	for _, chain := range chains {
		if isSelected180(ids, chain.ID) {
			count++
		}
	}
	return count
}

func unselectedCount180(chains []Chain, ids []string) int {
	count := 0
	for _, chain := range chains {
		if !isSelected180(ids, chain.ID) {
			count++
		}
	}
	return count
}

func scopeSummary180(chains []Chain, ids []string, readFileCount int, waiting bool) string {
	if waiting {
		return fmt.Sprintf("等待人工选择，共 %d 条候选调用链", len(chains))
	}
	selected := selectedCount180(chains, ids)
	if selected == 0 && len(chains) == 0 {
		if readFileCount > 0 {
			return fmt.Sprintf("无调用链，已检查 %d 个文件", readFileCount)
		}
		return "尚未形成可检查范围"
	}
	if readFileCount > 0 {
		return fmt.Sprintf("已选择 %d/%d 条调用链，检查 %d 个文件", selected, len(chains), readFileCount)
	}
	return fmt.Sprintf("已选择 %d/%d 条调用链", selected, len(chains))
}

func riskCount180(findings []Finding, severity string) int {
	count := 0
	for _, finding := range findings {
		if strings.EqualFold(finding.Severity, severity) {
			count++
		}
	}
	return count
}

func topFindings180(findings []Finding) []Finding {
	if len(findings) <= 3 {
		return findings
	}
	return findings[:3]
}

func firstAction180(findings []Finding, pending []string, gaps []string, execution, coverage string) string {
	if len(findings) > 0 {
		return findings[0].Recommendation
	}
	if len(pending) > 0 {
		return "优先确认待确认风险，再决定是否需要修改代码。"
	}
	if execution != "COMPLETE" || coverage != "COMPLETE" {
		return "先补齐选择与覆盖缺口，再形成最终风险结论。"
	}
	return "保持当前保护条件，并在后续变更后重新评审声明范围。"
}

func lineRange180(ref ReadRef) string {
	if ref.StartLine <= 0 {
		return "行号未提供"
	}
	if ref.EndLine <= ref.StartLine {
		return fmt.Sprintf("第 %d 行", ref.StartLine)
	}
	return fmt.Sprintf("第 %d–%d 行", ref.StartLine, ref.EndLine)
}

func unselectedSummary180(chains []Chain, ids []string) string {
	values := []string{}
	for _, chain := range chains {
		if isSelected180(ids, chain.ID) {
			continue
		}
		name := strings.TrimSpace(chain.Name)
		if name == "" {
			name = "未命名调用链"
		}
		values = append(values, chain.ID+" "+name)
	}
	if len(values) == 0 {
		return "无"
	}
	return markdownText180(strings.Join(values, "；"))
}

func firstEvidencePath180(finding Finding) string {
	if len(finding.Evidence) == 0 {
		return ""
	}
	return filepathSlash180(finding.Evidence[0].Ref.Path)
}

func filepathSlash180(value string) string {
	return strings.ReplaceAll(value, "\\", "/")
}

func countReadFiles180(reads []ReadRef) int {
	seen := map[string]bool{}
	for _, ref := range reads {
		path := filepathSlash180(strings.TrimSpace(ref.Path))
		if path != "" {
			seen[path] = true
		}
	}
	return len(seen)
}

func addOne180(value int) int {
	return value + 1
}

func bytesSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
