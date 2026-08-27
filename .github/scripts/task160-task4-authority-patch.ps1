$ErrorActionPreference = 'Stop'

$reviewPath = '.code-harness/tools-runtime/internal/report/review.go'
$text = Get-Content -Raw -LiteralPath $reviewPath
if ($text -notmatch 'func WriteCertifiedReport\(') {
    $text = $text.Replace("`t`"strings`"`n)", "`t`"strings`"`n`n`t`"codea-harness-tools/internal/finding`"`n)")
    $text = $text.Replace("`tLine               int     ``json:`"line,omitempty`"```n`tProblem", "`tLine               int     ``json:`"line,omitempty`"```n`tAnchorKind         string  ``json:`"anchorKind,omitempty`"```n`tSymbol             string  ``json:`"symbol,omitempty`"```n`tProblem")

    $oldLoop = @'
	for i, f := range req.Findings {
		if strings.TrimSpace(f.ID) == "" || strings.TrimSpace(f.File) == "" || strings.TrimSpace(f.Problem) == "" || strings.TrimSpace(f.Evidence) == "" || strings.TrimSpace(f.Impact) == "" || strings.TrimSpace(f.Recommendation) == "" {
			return fmt.Errorf("finding %d has missing required fields", i)
		}
		if _, ok := findingFiles[normalizeReportPath(f.File)]; !ok {
			if mode == "TARGETED" {
				return fmt.Errorf("finding %q file %q is outside verified scopedFiles", f.ID, f.File)
			}
			return fmt.Errorf("finding %q file %q is outside verified reviewedFiles", f.ID, f.File)
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
'@
    $newLoop = @'
	for i, f := range req.Findings {
		anchorKind := strings.ToUpper(strings.TrimSpace(f.AnchorKind))
		if strings.TrimSpace(f.ID) == "" || strings.TrimSpace(f.Problem) == "" || strings.TrimSpace(f.Evidence) == "" || strings.TrimSpace(f.Impact) == "" || strings.TrimSpace(f.Recommendation) == "" {
			return fmt.Errorf("finding %d has missing required fields", i)
		}
		if anchorKind != "CHANGESET" {
			if strings.TrimSpace(f.File) == "" {
				return fmt.Errorf("finding %d has missing required fields", i)
			}
			if _, ok := findingFiles[normalizeReportPath(f.File)]; !ok {
				if mode == "TARGETED" {
					return fmt.Errorf("finding %q file %q is outside verified scopedFiles", f.ID, f.File)
				}
				return fmt.Errorf("finding %q file %q is outside verified reviewedFiles", f.ID, f.File)
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
		switch anchorKind {
		case "":
			if f.Line < 0 { return fmt.Errorf("finding %q has invalid line", f.ID) }
		case "LINE":
			if f.Line < 1 || strings.TrimSpace(f.Symbol) != "" { return fmt.Errorf("finding %q has invalid LINE anchor", f.ID) }
		case "SYMBOL":
			if f.Line != 0 || strings.TrimSpace(f.Symbol) == "" { return fmt.Errorf("finding %q has invalid SYMBOL anchor", f.ID) }
		case "FILE":
			if f.Line != 0 || strings.TrimSpace(f.Symbol) != "" { return fmt.Errorf("finding %q has invalid FILE anchor", f.ID) }
		case "CHANGESET":
			if f.Line != 0 || strings.TrimSpace(f.Symbol) != "" { return fmt.Errorf("finding %q has invalid CHANGESET anchor", f.ID) }
		default:
			return fmt.Errorf("finding %q has invalid anchorKind %q", f.ID, f.AnchorKind)
		}
		if f.Confidence < 0 || f.Confidence > 1 {
			return fmt.Errorf("finding %q has invalid confidence", f.ID)
		}
	}
'@
    if (-not $text.Contains($oldLoop)) { throw 'review finding validation block not found' }
    $text = $text.Replace($oldLoop, $newLoop)

    $oldLocation = @'
		location := singleLine(f.File)
		if f.Line > 0 {
			location += ":" + strconv.Itoa(f.Line)
		}
'@
    $newLocation = @'
		location := findingLocation160(f)
'@
    if (-not $text.Contains($oldLocation)) { throw 'review finding location block not found' }
    $text = $text.Replace($oldLocation, $newLocation)

    $helper = @'
func findingLocation160(f Finding) string {
	switch strings.ToUpper(strings.TrimSpace(f.AnchorKind)) {
	case "SYMBOL":
		return singleLine(f.File) + " · " + singleLine(f.Symbol)
	case "FILE":
		return singleLine(f.File)
	case "CHANGESET":
		return "跨文件变更集"
	case "LINE":
		return singleLine(f.File) + ":" + strconv.Itoa(f.Line)
	default:
		location := singleLine(f.File)
		if f.Line > 0 { location += ":" + strconv.Itoa(f.Line) }
		return location
	}
}

'@
    $text = $text.Replace('func sortedFindings(findings []Finding) []Finding {', $helper + 'func sortedFindings(findings []Finding) []Finding {')

    $text = $text.Replace('func Write(repoRoot string, req ReviewRequest) (string, error) {', 'func writeReviewTransport160(repoRoot string, req ReviewRequest) (string, error) {')
    $authority = @'
func Write(repoRoot string, req ReviewRequest) (string, error) {
	contract := filepath.Join(repoRoot, ".code-harness", "contracts", "certified-findings.schema.json")
	if _, err := os.Stat(contract); err == nil {
		return WriteCertifiedReport(repoRoot, req)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect Certified Findings contract: %w", err)
	}
	return writeReviewTransport160(repoRoot, req)
}

func WriteCertifiedReport(repoRoot string, req ReviewRequest) (string, error) {
	if len(req.Findings) != 0 {
		return "", errors.New("RAW_AGENT_FINDINGS_FORBIDDEN: formal report accepts only same-run Certified Findings")
	}
	set, err := finding.LoadCertified(repoRoot, req.RunID)
	if err != nil {
		return "", err
	}
	req.Findings = mapCertifiedFindings160(set.Findings)
	if req.Coverage.Status == "PARTIAL" {
		req.Result = ResultManualActionRequired
	} else if len(req.Findings) > 0 {
		req.Result = ResultFailed
	} else {
		req.Result = ResultPassed
	}
	return writeReviewTransport160(repoRoot, req)
}

func mapCertifiedFindings160(in []finding.CertifiedFinding) []Finding {
	out := make([]Finding, 0, len(in))
	for _, certified := range in {
		item := Finding{
			ID: certified.ID, Category: certified.Category, Severity: certified.Severity,
			Problem: certified.Problem, Evidence: certifiedEvidenceSummary160(certified.EvidenceRefs),
			Impact: certified.Impact, Recommendation: certified.Recommendation,
			NeedsTest: certified.NeedsTest, IntroducedByChange: certified.IntroducedByChange,
			Confidence: certified.Confidence, AnchorKind: string(certified.Anchor.Kind),
		}
		switch certified.Anchor.Kind {
		case finding.AnchorLine:
			item.File, item.Line = certified.Anchor.Path, certified.Anchor.Line
		case finding.AnchorSymbol:
			item.File, item.Symbol = certified.Anchor.Path, certified.Anchor.Symbol
		case finding.AnchorFile:
			item.File = certified.Anchor.Path
		case finding.AnchorChangeSet:
			// CHANGESET has no invented file/line authority; evidence summary carries the cross-file proof.
		}
		out = append(out, item)
	}
	return out
}

func certifiedEvidenceSummary160(refs []finding.EvidenceRef) string {
	parts := make([]string, 0, len(refs))
	for _, ref := range refs {
		part := strings.ToUpper(strings.TrimSpace(ref.Kind))
		if strings.TrimSpace(ref.Path) != "" { part += " " + strings.TrimSpace(ref.Path) }
		if ref.StartLine > 0 { part += fmt.Sprintf(":%d-%d", ref.StartLine, ref.EndLine) }
		if strings.TrimSpace(ref.Value) != "" { part += " " + strings.TrimSpace(ref.Value) }
		parts = append(parts, part)
	}
	return strings.Join(parts, "；")
}

'@
    $text = $text.Replace('func writeReviewTransport160(repoRoot string, req ReviewRequest) (string, error) {', $authority + 'func writeReviewTransport160(repoRoot string, req ReviewRequest) (string, error) {')
    Set-Content -LiteralPath $reviewPath -Value $text -Encoding utf8NoBOM
}

$schemaPath = '.code-harness/contracts/review-output.schema.json'
$schema = Get-Content -Raw -LiteralPath $schemaPath
if ($schema -notmatch 'Runtime-owned renderer transport') {
    $schema = $schema.Replace('"description": "代码评审输出，由 Reviewer 在分析变更后产出。固定用户文案默认中文；Java 类名、方法名、路径和技术名词保持源码原文。"', '"description": "Runtime-owned renderer transport。1.6 起 Reviewer 不得把 findings[] 作为正式 Finding authority；正式问题只能由 same-run Certified Findings 映射。固定用户文案默认中文；Java 类名、方法名、路径和技术名词保持源码原文。"')
    $schema = $schema.Replace('"description": "评审发现列表。生产代码问题为 PRODUCTION_CODE；测试代码仅允许 TEST_VALIDITY。"', '"description": "仅供 Runtime renderer transport 使用，由 same-run Certified Findings 映射；Agent 不得直接提交为正式 Finding。"')
    $schema = $schema.Replace('"line": {', '"anchorKind": {"enum": ["LINE", "SYMBOL", "FILE", "CHANGESET"]},`n          "symbol": {"type": "string", "minLength": 1},`n          "line": {')
    Set-Content -LiteralPath $schemaPath -Value $schema -Encoding utf8NoBOM
}

$orchestratorPath = '.code-harness/agents/orchestrator.md'
$orchestrator = Get-Content -Raw -LiteralPath $orchestratorPath
if ($orchestrator -notmatch 'Certified Findings Authority（1.6 Task 4）') {
    $section = @'

## Certified Findings Authority（1.6 Task 4）

正式 Review 的 Finding authority 固定为 Runtime same-run Certified Findings。Agent 只能在 `requests/**` 提出 Finding Proposal，不能直接写正式 Finding、`analysis/**` 或 `review.md`。

固定顺序不可跳过：

```text
analysis certify
→ review scope/selection verify
→ review units
→ review dispatch
→ reviewer review-code produces requests/finding-proposals.json
→ review certify-findings
→ report render from same-run Certified Findings
```

`review certify-findings` 必须逐条重新验证 proposal 的 rule dispatch、ReviewUnit scope、anchor、evidence 与 introducedByChange；无效 proposal 只进入 machine rejection，不得进入 Certified Findings。semantic duplicate 由 Runtime canonical identity 去重。

`certified-findings.cert.json` 必须绑定当前 `ChangeSet / Certified ChangeAnalysis / ReviewUnit / RuleDispatch / finding-proposals` exact identity；任一上游 authoritative artifact byte 变化都 fail closed，禁止生成正式报告。

formal `review.md` 不接受 Agent raw `findings[]`。LINE 只显示真实 `path:line`；SYMBOL 显示 `path + symbol`；FILE 只显示 path；CHANGESET 显示跨文件 evidence summary，任何非 LINE anchor 都不得伪造行号。

'@
    $orchestrator = $orchestrator.Replace('## Review Change Set（review/test/api-doc changed 共用）', $section + '## Review Change Set（review/test/api-doc changed 共用）')
    Set-Content -LiteralPath $orchestratorPath -Value $orchestrator -Encoding utf8NoBOM
}

$changed = git status --porcelain -- $reviewPath $schemaPath $orchestratorPath
if ($changed) {
    git config user.name 'github-actions[bot]'
    git config user.email '41898282+github-actions[bot]@users.noreply.github.com'
    git add -- $reviewPath $schemaPath $orchestratorPath
    git commit -m 'feat: wire certified findings report authority'
    git push origin HEAD:$env:TASK4_HEAD_BRANCH
}
