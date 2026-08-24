package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Result string

const (
	ResultPassed               Result = "PASSED"
	ResultFailed               Result = "FAILED"
	ResultManualActionRequired Result = "MANUAL_ACTION_REQUIRED"
)

type ReviewRequest struct {
	RunID          string         `json:"runId"`
	HarnessVersion string         `json:"harnessVersion"`
	BaseRef        string         `json:"baseRef"`
	Head           string         `json:"head"`
	Result         Result         `json:"result"`
	Mode           string         `json:"mode,omitempty"`
	Target         *ReviewTarget  `json:"target,omitempty"`
	ChainContext   *ChainContext  `json:"chainContext,omitempty"`
	Scope          ReviewScope    `json:"reviewScope"`
	Coverage       ReviewCoverage `json:"reviewCoverage"`
	Findings       []Finding      `json:"findings"`
}

type ReviewTarget struct {
	Symbol string `json:"symbol"`
	Kind   string `json:"kind"`
}

type ChainContext struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Source string `json:"source"` // ACCEPTED | DISCOVERED
	Status string `json:"status"` // VALID | TEMPORARY
}

type ReviewScope struct {
	ChangedFiles []string `json:"changedFiles"`
	ScopedFiles  []string `json:"scopedFiles,omitempty"`
}

type CallChain struct {
	EntryPoint string   `json:"entryPoint"`
	Chain      []string `json:"chain"`
}

type SymbolRoleEvidence struct {
	Symbol string `json:"symbol"`
	Role   string `json:"role"`
	Source string `json:"source"`
}

type ResourceRoleEvidence struct {
	Resource string `json:"resource"`
	Role     string `json:"role"`
	Source   string `json:"source"`
}

type ReviewCoverage struct {
	ReviewedFiles        []string               `json:"reviewedFiles"`
	CallChains           []CallChain            `json:"callChains"`
	SymbolRoleEvidence   []SymbolRoleEvidence   `json:"symbolRoleEvidence,omitempty"`
	ResourceRoleEvidence []ResourceRoleEvidence `json:"resourceRoleEvidence,omitempty"`
	ExternalDependencies []string               `json:"externalDependencies"`
	Unresolved           []string               `json:"unresolved"`
	MissingReviewedFiles []string               `json:"missingReviewedFiles"`
	RuntimeErrors        []string               `json:"runtimeErrors"`
	Status               string                 `json:"status"`
}

type Finding struct {
	ID                 string  `json:"id"`
	Category           string  `json:"category"`
	Severity           string  `json:"severity"`
	File               string  `json:"file"`
	Line               int     `json:"line,omitempty"`
	Problem            string  `json:"problem"`
	Evidence           string  `json:"evidence"`
	Impact             string  `json:"impact"`
	Recommendation     string  `json:"recommendation"`
	NeedsTest          bool    `json:"needsTest"`
	IntroducedByChange bool    `json:"introducedByChange"`
	Confidence         float64 `json:"confidence"`
}

func DecodeReviewRequest(data []byte) (ReviewRequest, error) {
	var req ReviewRequest
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return ReviewRequest{}, fmt.Errorf("decode review report request: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return ReviewRequest{}, errors.New("decode review report request: multiple JSON values are not allowed")
		}
		return ReviewRequest{}, fmt.Errorf("decode review report request: %w", err)
	}
	if err := Validate(req); err != nil {
		return ReviewRequest{}, err
	}
	return req, nil
}

func Validate(req ReviewRequest) error {
	if !validArtifactID(req.RunID) {
		return errors.New("invalid review report runId")
	}
	if strings.TrimSpace(req.HarnessVersion) == "" || strings.TrimSpace(req.BaseRef) == "" || strings.TrimSpace(req.Head) == "" {
		return errors.New("review report requires harnessVersion, baseRef, and head")
	}
	switch req.Result {
	case ResultPassed, ResultFailed, ResultManualActionRequired:
	default:
		return fmt.Errorf("invalid review result %q", req.Result)
	}
	if err := validateChainContext(req.ChainContext); err != nil {
		return err
	}
	mode := reviewMode(req)
	switch mode {
	case "FULL":
		if req.Target != nil {
			return errors.New("FULL review report must not contain target")
		}
	case "TARGETED":
		if req.Target == nil || strings.TrimSpace(req.Target.Symbol) == "" {
			return errors.New("TARGETED review report requires target")
		}
		if req.Target.Kind != "CLASS" && req.Target.Kind != "METHOD" {
			return fmt.Errorf("invalid review target kind %q", req.Target.Kind)
		}
		if len(req.Scope.ScopedFiles) == 0 {
			return errors.New("TARGETED review report requires scopedFiles")
		}
		if len(req.Coverage.CallChains) == 0 {
			return errors.New("TARGETED review report requires selected callChains")
		}
	default:
		return fmt.Errorf("invalid review mode %q", req.Mode)
	}
	switch req.Coverage.Status {
	case "COMPLETE", "PARTIAL":
	default:
		return fmt.Errorf("invalid review coverage status %q", req.Coverage.Status)
	}
	if req.Coverage.Status == "PARTIAL" && req.Result != ResultManualActionRequired {
		return errors.New("PARTIAL coverage requires MANUAL_ACTION_REQUIRED result")
	}
	for i, callChain := range req.Coverage.CallChains {
		if strings.TrimSpace(callChain.EntryPoint) == "" {
			return fmt.Errorf("call chain %d requires entryPoint", i)
		}
		for j, symbol := range callChain.Chain {
			if strings.TrimSpace(symbol) == "" {
				return fmt.Errorf("call chain %q has empty symbol at %d", callChain.EntryPoint, j)
			}
		}
	}
	if err := validateRoleEvidence(req.Coverage); err != nil {
		return err
	}
	var targetedFiles map[string]struct{}
	if mode == "TARGETED" {
		targetedFiles = make(map[string]struct{}, len(req.Scope.ScopedFiles))
		for _, file := range req.Scope.ScopedFiles {
			targetedFiles[normalizeReportPath(file)] = struct{}{}
		}
	}
	for i, f := range req.Findings {
		if strings.TrimSpace(f.ID) == "" || strings.TrimSpace(f.File) == "" || strings.TrimSpace(f.Problem) == "" || strings.TrimSpace(f.Evidence) == "" || strings.TrimSpace(f.Impact) == "" || strings.TrimSpace(f.Recommendation) == "" {
			return fmt.Errorf("finding %d has missing required fields", i)
		}
		if mode == "TARGETED" {
			if _, ok := targetedFiles[normalizeReportPath(f.File)]; !ok {
				return fmt.Errorf("finding %q file %q is outside verified scopedFiles", f.ID, f.File)
			}
		}
		switch f.Category {
		case "PRODUCTION_CODE", "TEST_VALIDITY":
		default:
			return fmt.Errorf("finding %q has invalid category %q", f.ID, f.Category)
		}
		switch strings.ToUpper(f.Severity) {
		case "CRITICAL", "HIGH", "MEDIUM", "LOW":
		default:
			return fmt.Errorf("finding %q has invalid severity %q", f.ID, f.Severity)
		}
		if f.Line < 0 {
			return fmt.Errorf("finding %q has invalid line", f.ID)
		}
		if f.Confidence < 0 || f.Confidence > 1 {
			return fmt.Errorf("finding %q has invalid confidence", f.ID)
		}
	}
	return nil
}

func validateChainContext(context *ChainContext) error {
	if context == nil {
		return nil
	}
	if strings.TrimSpace(context.ID) == "" || strings.TrimSpace(context.Name) == "" {
		return errors.New("review chainContext requires id and name")
	}
	switch context.Source {
	case "ACCEPTED":
		if context.Status != "VALID" {
			return errors.New("ACCEPTED review chainContext requires VALID status")
		}
	case "DISCOVERED":
		if context.Status != "TEMPORARY" {
			return errors.New("DISCOVERED review chainContext requires TEMPORARY status")
		}
	default:
		return fmt.Errorf("invalid review chainContext source %q", context.Source)
	}
	return nil
}

func validateRoleEvidence(coverage ReviewCoverage) error {
	seen := make(map[string]string, len(coverage.SymbolRoleEvidence)+len(coverage.ResourceRoleEvidence))
	for i, evidence := range coverage.SymbolRoleEvidence {
		symbol := strings.TrimSpace(evidence.Symbol)
		if symbol == "" {
			return fmt.Errorf("symbol role evidence %d requires symbol", i)
		}
		switch evidence.Role {
		case "Controller", "Service", "Repository", "Mapper", "Entity", "DTO", "VO", "Validator", "ExceptionHandler", "Config", "Utility", "Other":
		default:
			return fmt.Errorf("symbol role evidence %q has invalid role %q", symbol, evidence.Role)
		}
		switch evidence.Source {
		case "FIND_SYMBOL", "FIND_REFERENCES", "FIND_IMPLEMENTATIONS":
		default:
			return fmt.Errorf("symbol role evidence %q has invalid source %q", symbol, evidence.Source)
		}
		if previous, ok := seen[symbol]; ok {
			return fmt.Errorf("duplicate role evidence for %q (%s and symbol)", symbol, previous)
		}
		seen[symbol] = "symbol"
	}
	for i, evidence := range coverage.ResourceRoleEvidence {
		resource := strings.TrimSpace(evidence.Resource)
		if resource == "" {
			return fmt.Errorf("resource role evidence %d requires resource", i)
		}
		switch evidence.Role {
		case "MapperXml":
			if evidence.Source != "MAPPER_STATEMENT" {
				return fmt.Errorf("resource role evidence %q requires MAPPER_STATEMENT source", resource)
			}
		case "YamlConfig":
			if evidence.Source != "CONFIG_REFERENCE" {
				return fmt.Errorf("resource role evidence %q requires CONFIG_REFERENCE source", resource)
			}
		default:
			return fmt.Errorf("resource role evidence %q has invalid role %q", resource, evidence.Role)
		}
		if previous, ok := seen[resource]; ok {
			return fmt.Errorf("duplicate role evidence for %q (%s and resource)", resource, previous)
		}
		seen[resource] = "resource"
	}
	return nil
}

func reviewMode(req ReviewRequest) string {
	mode := strings.ToUpper(strings.TrimSpace(req.Mode))
	if mode == "" {
		return "FULL"
	}
	return mode
}

func scopeFiles(req ReviewRequest) []string {
	if reviewMode(req) == "TARGETED" {
		return req.Scope.ScopedFiles
	}
	return req.Scope.ChangedFiles
}

func normalizeReportPath(value string) string {
	return filepath.ToSlash(filepath.Clean(strings.TrimSpace(value)))
}

func validArtifactID(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for i := 0; i < len(value); i++ {
		b := value[i]
		if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '.' || b == '_' || b == '-' {
			continue
		}
		return false
	}
	return true
}

func Render(req ReviewRequest) (string, error) {
	if err := Validate(req); err != nil {
		return "", err
	}
	var b strings.Builder
	writeHeader(&b, req)
	if req.Coverage.Status == "PARTIAL" {
		writePartial(&b, req)
		writeNextStep(&b, req)
		return b.String(), nil
	}
	writeSeverityOverview(&b, req.Findings)
	writeScope(&b, req)
	writeCallChains(&b, req.Coverage)
	writeCoverage(&b, req.Coverage)
	writeFindings(&b, req.Findings)
	writeSummary(&b, req)
	writeNextStep(&b, req)
	return b.String(), nil
}

func writeHeader(b *strings.Builder, req ReviewRequest) {
	fmt.Fprintln(b, "# 🔍 代码评审报告")
	fmt.Fprintln(b)
	fmt.Fprintln(b, "| 项目 | 内容 |")
	fmt.Fprintln(b, "|---|---|")
	fmt.Fprintf(b, "| 评审结果 | %s |\n", resultLabel(req.Result))
	if reviewMode(req) == "TARGETED" {
		fmt.Fprintln(b, "| 评审模式 | 🎯 定向评审 |")
		fmt.Fprintf(b, "| 评审目标 | `%s` |\n", singleLine(req.Target.Symbol))
	} else {
		fmt.Fprintln(b, "| 评审模式 | 📦 完整评审 |")
	}
	if req.ChainContext != nil {
		fmt.Fprintf(b, "| 业务链 | %s |\n", singleLine(req.ChainContext.Name))
		fmt.Fprintf(b, "| Chain ID | `%s` |\n", singleLine(req.ChainContext.ID))
		fmt.Fprintf(b, "| Chain 来源 | %s |\n", chainSourceLabel(req.ChainContext.Source))
		fmt.Fprintf(b, "| Chain 状态 | %s |\n", chainStatusLabel(req.ChainContext.Status))
	}
	fmt.Fprintf(b, "| Harness 版本 | %s |\n", singleLine(req.HarnessVersion))
	fmt.Fprintf(b, "| 评审基线 | %s |\n", singleLine(req.BaseRef))
	fmt.Fprintf(b, "| 当前提交 | %s |\n", singleLine(req.Head))
	fmt.Fprintf(b, "| Change Set 文件 | %d |\n", len(req.Scope.ChangedFiles))
	fmt.Fprintf(b, "| 本次 Scope 文件 | %d |\n", len(scopeFiles(req)))
	fmt.Fprintf(b, "| 已评审文件 | %d |\n", len(req.Coverage.ReviewedFiles))
	fmt.Fprintf(b, "| 问题数量 | %d |\n", len(req.Findings))
	fmt.Fprintf(b, "| 下一步 | %s |\n", firstScreenNextAction(req))
	fmt.Fprintln(b)
	if req.ChainContext != nil && req.ChainContext.Source == "DISCOVERED" {
		fmt.Fprintln(b, "⚠️ 本次评审使用临时发现的业务链，尚未沉淀到项目 Chain。")
		fmt.Fprintln(b)
	}
}

func chainSourceLabel(source string) string {
	if source == "ACCEPTED" {
		return "项目已确认"
	}
	return "本次临时发现"
}

func chainStatusLabel(status string) string {
	if status == "VALID" {
		return "已确认"
	}
	return "临时"
}

func firstScreenNextAction(req ReviewRequest) string {
	switch req.Result {
	case ResultPassed:
		return "无需处理阻断问题"
	case ResultFailed:
		if id := primaryFindingID(req.Findings); id != "" {
			return fmt.Sprintf("优先处理阻断问题；可使用 `harness fix finding:%s`", singleLine(id))
		}
		return "优先处理阻断问题"
	default:
		if len(req.Coverage.RuntimeErrors) > 0 {
			return "处理运行时契约校验错误后重新评审"
		}
		return "处理未解析项/缺失评审文件后重新评审"
	}
}

func writeSeverityOverview(b *strings.Builder, findings []Finding) {
	counts := severityCounts(findings)
	fmt.Fprintln(b, "## 📊 问题概览")
	fmt.Fprintln(b)
	fmt.Fprintln(b, "| 级别 | 数量 |")
	fmt.Fprintln(b, "|---|---:|")
	fmt.Fprintf(b, "| 🔴 严重 | %d |\n", counts["CRITICAL"])
	fmt.Fprintf(b, "| 🟠 高 | %d |\n", counts["HIGH"])
	fmt.Fprintf(b, "| 🟡 中 | %d |\n", counts["MEDIUM"])
	fmt.Fprintf(b, "| 🟢 低 | %d |\n", counts["LOW"])
	fmt.Fprintln(b)
	fmt.Fprintln(b, "---")
	fmt.Fprintln(b)
}

func writeScope(b *strings.Builder, req ReviewRequest) {
	fmt.Fprintln(b, "## 📁 评审范围")
	fmt.Fprintln(b)
	var production, tests []string
	for _, file := range scopeFiles(req) {
		if isTestFile(file) {
			tests = append(tests, file)
		} else {
			production = append(production, file)
		}
	}
	sort.Strings(production)
	sort.Strings(tests)
	if len(production) > 0 {
		fmt.Fprintln(b, "### 生产代码")
		fmt.Fprintln(b)
		writeCodeList(b, production, "无")
		fmt.Fprintln(b)
	}
	if len(tests) > 0 {
		fmt.Fprintln(b, "### 测试代码")
		fmt.Fprintln(b)
		writeCodeList(b, tests, "无")
		fmt.Fprintln(b)
		fmt.Fprintln(b, "> 测试代码用于覆盖分析，不执行普通代码质量评审。")
		fmt.Fprintln(b)
	}
	if len(production) == 0 && len(tests) == 0 {
		fmt.Fprintln(b, "无变更文件。")
		fmt.Fprintln(b)
	}
	fmt.Fprintln(b, "---")
	fmt.Fprintln(b)
}

func writeCallChains(b *strings.Builder, coverage ReviewCoverage) {
	fmt.Fprintln(b, "## 🔗 代码调用链")
	fmt.Fprintln(b)
	if len(coverage.CallChains) == 0 {
		fmt.Fprintln(b, "未发现需要展开的项目内部调用链。")
		fmt.Fprintln(b)
		return
	}
	labels := callChainRoleLabels(coverage)
	for i, callChain := range coverage.CallChains {
		fmt.Fprintf(b, "### 调用链 %d\n\n", i+1)
		nodes := normalizeCallChain(callChain)
		for j, symbol := range nodes {
			label := labels[strings.TrimSpace(symbol)]
			if label == "" {
				label = "🔹 代码节点"
			}
			fmt.Fprintf(b, "%s｜`%s`\n", label, singleLine(symbol))
			if j < len(nodes)-1 {
				fmt.Fprintln(b, "↓")
			}
		}
		fmt.Fprintln(b)
	}
	fmt.Fprintln(b, "---")
	fmt.Fprintln(b)
}

func normalizeCallChain(callChain CallChain) []string {
	entry := strings.TrimSpace(callChain.EntryPoint)
	nodes := make([]string, 0, len(callChain.Chain)+1)
	if entry != "" {
		nodes = append(nodes, entry)
	}
	for i, symbol := range callChain.Chain {
		symbol = strings.TrimSpace(symbol)
		if i == 0 && symbol == entry {
			continue
		}
		nodes = append(nodes, symbol)
	}
	return nodes
}

func callChainRoleLabels(coverage ReviewCoverage) map[string]string {
	labels := make(map[string]string, len(coverage.SymbolRoleEvidence)+len(coverage.ResourceRoleEvidence))
	for _, evidence := range coverage.SymbolRoleEvidence {
		symbol := strings.TrimSpace(evidence.Symbol)
		switch evidence.Role {
		case "Controller":
			labels[symbol] = "🌐 接口入口"
		case "Service":
			if evidence.Source == "FIND_IMPLEMENTATIONS" {
				labels[symbol] = "🧠 业务实现"
			} else {
				labels[symbol] = "⚙️ 业务服务"
			}
		case "Repository", "Mapper":
			labels[symbol] = "🗄 数据访问"
		default:
			labels[symbol] = "🔹 代码节点"
		}
	}
	for _, evidence := range coverage.ResourceRoleEvidence {
		resource := strings.TrimSpace(evidence.Resource)
		if evidence.Role == "MapperXml" {
			labels[resource] = "📄 Mapper XML"
		} else {
			labels[resource] = "🔹 代码节点"
		}
	}
	return labels
}

func writeCoverage(b *strings.Builder, coverage ReviewCoverage) {
	fmt.Fprintln(b, "## ✅ 评审覆盖")
	fmt.Fprintln(b)
	fmt.Fprintln(b, "### 已评审文件")
	fmt.Fprintln(b)
	files := append([]string(nil), coverage.ReviewedFiles...)
	sort.Strings(files)
	writeCodeList(b, files, "无")
	fmt.Fprintln(b)

	fmt.Fprintln(b, "### 外部依赖")
	fmt.Fprintln(b)
	deps := append([]string(nil), coverage.ExternalDependencies...)
	sort.Strings(deps)
	writeCodeList(b, deps, "无")
	fmt.Fprintln(b)

	fmt.Fprintln(b, "### 未解析项")
	fmt.Fprintln(b)
	unresolved := unresolvedItems(coverage)
	writeCodeList(b, unresolved, "无")
	fmt.Fprintln(b)

	fmt.Fprintln(b, "### 覆盖状态")
	fmt.Fprintln(b)
	if coverage.Status == "COMPLETE" {
		fmt.Fprintln(b, "✅ 完整")
	} else {
		fmt.Fprintln(b, "⚠️ 不完整")
	}
	fmt.Fprintln(b)
	fmt.Fprintln(b, "---")
	fmt.Fprintln(b)
}

func writeFindings(b *strings.Builder, findings []Finding) {
	if len(findings) == 0 {
		return
	}
	fmt.Fprintln(b, "# 🚨 问题清单")
	fmt.Fprintln(b)
	for _, f := range sortedFindings(findings) {
		emoji, name := severityDisplayParts(f.Severity)
		fmt.Fprintf(b, "### %s %s｜%s\n\n", emoji, singleLine(f.ID), name)
		fmt.Fprintln(b, "📍 **位置**")
		fmt.Fprintln(b)
		location := singleLine(f.File)
		if f.Line > 0 {
			location += ":" + strconv.Itoa(f.Line)
		}
		fmt.Fprintf(b, "`%s`\n\n", location)
		writeIconSection(b, "❗", "问题", f.Problem)
		writeIconSection(b, "🔎", "证据", f.Evidence)
		writeIconSection(b, "💥", "影响", f.Impact)
		writeIconSection(b, "🛠", "修复建议", f.Recommendation)
		fmt.Fprintln(b, "🧪 **是否需要测试**")
		fmt.Fprintln(b)
		if f.NeedsTest {
			fmt.Fprintln(b, "是")
		} else {
			fmt.Fprintln(b, "否")
		}
		fmt.Fprintln(b)
		fmt.Fprintln(b, "**置信度**")
		fmt.Fprintln(b)
		fmt.Fprintf(b, "%d%%\n\n", int(f.Confidence*100+0.5))
		fmt.Fprintln(b, "---")
		fmt.Fprintln(b)
	}
}

func sortedFindings(findings []Finding) []Finding {
	sorted := append([]Finding(nil), findings...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, c := sorted[i], sorted[j]
		if severityRank(a.Severity) != severityRank(c.Severity) {
			return severityRank(a.Severity) < severityRank(c.Severity)
		}
		if a.File != c.File {
			return a.File < c.File
		}
		if a.Line != c.Line {
			return a.Line < c.Line
		}
		return a.ID < c.ID
	})
	return sorted
}

func primaryFindingID(findings []Finding) string {
	sorted := sortedFindings(findings)
	if len(sorted) == 0 {
		return ""
	}
	return strings.TrimSpace(sorted[0].ID)
}

func writeSummary(b *strings.Builder, req ReviewRequest) {
	counts := severityCounts(req.Findings)
	fmt.Fprintln(b, "## 📌 评审结论")
	fmt.Fprintln(b)
	switch req.Result {
	case ResultPassed:
		fmt.Fprintln(b, "### ✅ 本次评审通过")
		fmt.Fprintln(b)
		fmt.Fprintln(b, "未发现需要处理的生产代码问题。")
	case ResultFailed:
		fmt.Fprintln(b, "### ❌ 本次评审未通过")
		fmt.Fprintln(b)
		fmt.Fprintf(b, "共发现 **%d** 个问题：\n\n", len(req.Findings))
		fmt.Fprintf(b, "- 🔴 严重：%d\n", counts["CRITICAL"])
		fmt.Fprintf(b, "- 🟠 高：%d\n", counts["HIGH"])
		fmt.Fprintf(b, "- 🟡 中：%d\n", counts["MEDIUM"])
		fmt.Fprintf(b, "- 🟢 低：%d\n", counts["LOW"])
	default:
		fmt.Fprintln(b, "### ⚠️ 需要人工处理")
	}
	writeTargetedDisclaimer(b, req)
}

func writePartial(b *strings.Builder, req ReviewRequest) {
	fmt.Fprintln(b, "## ⚠️ 评审未完整完成")
	fmt.Fprintln(b)
	fmt.Fprintln(b, "当前评审需要人工处理，不能判定为通过。")
	fmt.Fprintln(b)
	fmt.Fprintln(b, "### 未解析项")
	fmt.Fprintln(b)
	items := append([]string(nil), req.Coverage.Unresolved...)
	for _, e := range req.Coverage.RuntimeErrors {
		items = append(items, "运行时契约校验错误: "+e)
	}
	sort.Strings(items)
	writeCodeList(b, items, "无")
	fmt.Fprintln(b)
	fmt.Fprintln(b, "### 尚未评审文件")
	fmt.Fprintln(b)
	missing := append([]string(nil), req.Coverage.MissingReviewedFiles...)
	sort.Strings(missing)
	writeCodeList(b, missing, "无")
	fmt.Fprintln(b)
	fmt.Fprintln(b, "### 评审结论")
	fmt.Fprintln(b)
	fmt.Fprintln(b, "⚠️ 需要人工处理")
	writeTargetedDisclaimer(b, req)
}

func writeNextStep(b *strings.Builder, req ReviewRequest) {
	fmt.Fprintln(b)
	fmt.Fprintln(b, "## ➡️ 下一步")
	fmt.Fprintln(b)
	switch req.Result {
	case ResultPassed:
		fmt.Fprintln(b, "下一步：无需处理阻断问题。")
	case ResultFailed:
		if id := primaryFindingID(req.Findings); id != "" {
			fmt.Fprintf(b, "下一步：优先处理阻断问题；可使用 `harness fix finding:%s`。\n", singleLine(id))
		} else {
			fmt.Fprintln(b, "下一步：优先处理阻断问题。")
		}
	default:
		fmt.Fprintln(b, manualNextAction(req.Coverage))
	}
}

func manualNextAction(coverage ReviewCoverage) string {
	unresolved := append([]string(nil), coverage.Unresolved...)
	missing := append([]string(nil), coverage.MissingReviewedFiles...)
	runtimeErrors := append([]string(nil), coverage.RuntimeErrors...)
	sort.Strings(unresolved)
	sort.Strings(missing)
	sort.Strings(runtimeErrors)

	if len(runtimeErrors) == 0 && len(unresolved) == 1 && len(missing) == 1 {
		return fmt.Sprintf("请先处理未解析项 `%s`，并补充评审文件 `%s`。", singleLine(unresolved[0]), singleLine(missing[0]))
	}
	var lines []string
	if len(runtimeErrors) > 0 {
		lines = append(lines, "请先处理以下运行时契约校验错误：")
		for _, item := range runtimeErrors {
			lines = append(lines, fmt.Sprintf("- `%s`", singleLine(item)))
		}
	}
	if len(unresolved) > 0 {
		lines = append(lines, "请先处理以下未解析项：")
		for _, item := range unresolved {
			lines = append(lines, fmt.Sprintf("- `%s`", singleLine(item)))
		}
	}
	if len(missing) > 0 {
		lines = append(lines, "请补充以下评审文件：")
		for _, item := range missing {
			lines = append(lines, fmt.Sprintf("- `%s`", singleLine(item)))
		}
	}
	if len(lines) == 0 {
		return "请根据上方人工处理项完成补充后重新评审。"
	}
	return strings.Join(lines, "\n")
}

func writeTargetedDisclaimer(b *strings.Builder, req ReviewRequest) {
	if reviewMode(req) != "TARGETED" {
		return
	}
	fmt.Fprintln(b)
	fmt.Fprintln(b, "> 本结论只覆盖本次定向评审范围，不代表整个 Change Set 已完成评审。")
}

func resultLabel(result Result) string {
	switch result {
	case ResultPassed:
		return "✅ 通过"
	case ResultFailed:
		return "❌ 未通过"
	default:
		return "⚠️ 需要人工处理"
	}
}

func severityLabel(severity string) string {
	emoji, name := severityDisplayParts(severity)
	return emoji + " " + name
}

func severityDisplayParts(severity string) (string, string) {
	switch strings.ToUpper(severity) {
	case "CRITICAL":
		return "🔴", "严重"
	case "HIGH":
		return "🟠", "高"
	case "MEDIUM":
		return "🟡", "中"
	default:
		return "🟢", "低"
	}
}

func severityRank(severity string) int {
	switch strings.ToUpper(severity) {
	case "CRITICAL":
		return 0
	case "HIGH":
		return 1
	case "MEDIUM":
		return 2
	default:
		return 3
	}
}

func severityCounts(findings []Finding) map[string]int {
	counts := map[string]int{"CRITICAL": 0, "HIGH": 0, "MEDIUM": 0, "LOW": 0}
	for _, f := range findings {
		counts[strings.ToUpper(f.Severity)]++
	}
	return counts
}

func unresolvedItems(coverage ReviewCoverage) []string {
	items := append([]string(nil), coverage.Unresolved...)
	for _, f := range coverage.MissingReviewedFiles {
		items = append(items, "尚未评审文件: "+f)
	}
	for _, e := range coverage.RuntimeErrors {
		items = append(items, "运行时契约校验错误: "+e)
	}
	sort.Strings(items)
	return items
}

func isTestFile(path string) bool {
	p := strings.ReplaceAll(strings.ToLower(filepath.ToSlash(path)), "\\", "/")
	return strings.Contains(p, "/src/test/") || strings.HasPrefix(p, "src/test/")
}

func Write(repoRoot string, req ReviewRequest) (string, error) {
	markdown, err := Render(req)
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}
	runsRoot := filepath.Join(root, ".code-harness", "runs")
	runDir := filepath.Join(runsRoot, req.RunID)
	rel, err := filepath.Rel(runsRoot, runDir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("review report path escapes runs directory")
	}
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return "", fmt.Errorf("create review report directory: %w", err)
	}
	realRunsRoot, err := filepath.EvalSymlinks(runsRoot)
	if err != nil {
		return "", fmt.Errorf("resolve runs directory: %w", err)
	}
	realRunDir, err := filepath.EvalSymlinks(runDir)
	if err != nil {
		return "", fmt.Errorf("resolve review report directory: %w", err)
	}
	realRel, err := filepath.Rel(realRunsRoot, realRunDir)
	if err != nil || realRel == ".." || strings.HasPrefix(realRel, ".."+string(filepath.Separator)) {
		return "", errors.New("review report path escapes runs directory")
	}
	path := filepath.Join(runDir, "review.md")
	if err := os.WriteFile(path, []byte(markdown), 0o600); err != nil {
		return "", fmt.Errorf("write review report: %w", err)
	}
	return path, nil
}

func WriteRequestFile(repoRoot, inputPath string) (string, error) {
	if filepath.IsAbs(inputPath) || !strings.EqualFold(filepath.Ext(inputPath), ".json") {
		return "", errors.New("review report input must be a JSON request under .code-harness/runs/<runId>/requests")
	}
	cleanInput := filepath.Clean(inputPath)
	runsRootRelative := filepath.Clean(filepath.Join(".code-harness", "runs"))
	runsRel, err := filepath.Rel(runsRootRelative, cleanInput)
	if err != nil || runsRel == "." || runsRel == ".." || strings.HasPrefix(runsRel, ".."+string(filepath.Separator)) {
		return "", errors.New("review report input must be under .code-harness/runs/<runId>/requests")
	}
	data, err := os.ReadFile(cleanInput)
	if err != nil {
		return "", fmt.Errorf("read review report request: %w", err)
	}
	req, err := DecodeReviewRequest(data)
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}
	inputAbs, err := filepath.Abs(cleanInput)
	if err != nil {
		return "", err
	}
	requestRoot := filepath.Join(root, ".code-harness", "runs", req.RunID, "requests")
	rel, err := filepath.Rel(requestRoot, inputAbs)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("review report input must be under .code-harness/runs/<runId>/requests")
	}
	realRequestRoot, err := filepath.EvalSymlinks(requestRoot)
	if err != nil {
		return "", fmt.Errorf("resolve review report request directory: %w", err)
	}
	realInput, err := filepath.EvalSymlinks(inputAbs)
	if err != nil {
		return "", fmt.Errorf("resolve review report input: %w", err)
	}
	realRel, err := filepath.Rel(realRequestRoot, realInput)
	if err != nil || realRel == "." || realRel == ".." || strings.HasPrefix(realRel, ".."+string(filepath.Separator)) {
		return "", errors.New("review report input must be under .code-harness/runs/<runId>/requests")
	}
	path, err := Write(root, req)
	if err != nil {
		return "", err
	}
	if err := os.Remove(inputAbs); err != nil {
		return "", fmt.Errorf("remove review report transport after success: %w", err)
	}
	return path, nil
}

func writeCodeList(b *strings.Builder, values []string, empty string) {
	if len(values) == 0 {
		fmt.Fprintln(b, empty)
		return
	}
	for _, v := range values {
		fmt.Fprintf(b, "- `%s`\n", singleLine(v))
	}
}

func writeBoldSection(b *strings.Builder, title, content string) {
	fmt.Fprintf(b, "**%s**\n\n", title)
	fmt.Fprintln(b, strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n")))
	fmt.Fprintln(b)
}

func writeIconSection(b *strings.Builder, icon, title, content string) {
	fmt.Fprintf(b, "%s **%s**\n\n", icon, title)
	fmt.Fprintln(b, strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n")))
	fmt.Fprintln(b)
}

func singleLine(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}
