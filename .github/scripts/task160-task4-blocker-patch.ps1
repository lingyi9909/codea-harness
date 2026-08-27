$ErrorActionPreference = 'Stop'

$path = '.code-harness/tools-runtime/internal/report/review.go'
$text = [System.IO.File]::ReadAllText($path)

if ($text -notmatch '"crypto/sha256"') {
  $text = [regex]::Replace($text, '"bytes"\r?\n', '"bytes"' + "`r`n`t" + '"crypto/sha256"' + "`r`n", 1)
}

$replacement = @'
func WriteCertifiedReport(repoRoot string, req ReviewRequest) (string, error) {
	if len(req.Findings) != 0 {
		return "", errors.New("RAW_AGENT_FINDINGS_FORBIDDEN: formal report accepts only same-run Certified Findings")
	}
	if req.Coverage.Status == "PARTIAL" {
		req.Findings = []Finding{}
		req.Result = ResultManualActionRequired
		return writeReviewTransport160(repoRoot, req)
	}
	set, cert, err := finding.LoadCertifiedWithCertificate(repoRoot, req.RunID)
	if err != nil {
		return "", err
	}
	if err := verifyCertifiedReportAuthority160(repoRoot, req, cert); err != nil {
		return "", err
	}
	return writeCertifiedSetReport160(repoRoot, req, set)
}

func writeCertifiedSetReport160(repoRoot string, req ReviewRequest, set finding.CertifiedSet) (string, error) {
	req.Findings = mapCertifiedFindings160(set.Findings)
	if len(req.Findings) > 0 {
		req.Result = ResultFailed
	} else {
		req.Result = ResultPassed
	}
	return writeReviewTransport160(repoRoot, req)
}

func verifyCertifiedReportAuthority160(repoRoot string, req ReviewRequest, cert finding.Certificate) error {
	mode := reviewMode(req)
	certMode := strings.ToUpper(strings.TrimSpace(cert.Mode))
	if certMode != mode {
		return fmt.Errorf("CERTIFIED_FINDINGS_REPORT_MODE_MISMATCH: certificate=%s report=%s", certMode, mode)
	}

	scopeSHA := strings.TrimSpace(cert.ScopeSHA256)
	if mode == "TARGETED" && scopeSHA == "" {
		return errors.New("CERTIFIED_FINDINGS_REPORT_SCOPE_MISMATCH: TARGETED certificate requires scopeSha256")
	}
	if scopeSHA == "" {
		return nil
	}

	scopePath := filepath.Join(repoRoot, ".code-harness", "runs", req.RunID, "analysis", "review-scope.json")
	scopeBytes, err := os.ReadFile(scopePath)
	if err != nil {
		return fmt.Errorf("CERTIFIED_FINDINGS_REPORT_SCOPE_MISMATCH: read current ReviewScope: %w", err)
	}
	currentSHA, err := canonicalReviewScopeSHA160(scopeBytes)
	if err != nil {
		return fmt.Errorf("CERTIFIED_FINDINGS_REPORT_SCOPE_MISMATCH: canonicalize current ReviewScope: %w", err)
	}
	if currentSHA != scopeSHA {
		return errors.New("CERTIFIED_FINDINGS_REPORT_SCOPE_MISMATCH: current ReviewScope hash differs from certificate")
	}

	var scope struct {
		Mode string `json:"mode"`
		Target *ReviewTarget `json:"target,omitempty"`
		ScopedFiles []string `json:"scopedFiles,omitempty"`
	}
	if err := json.Unmarshal(scopeBytes, &scope); err != nil {
		return fmt.Errorf("CERTIFIED_FINDINGS_REPORT_SCOPE_MISMATCH: decode current ReviewScope: %w", err)
	}
	if strings.ToUpper(strings.TrimSpace(scope.Mode)) != certMode {
		return errors.New("CERTIFIED_FINDINGS_REPORT_SCOPE_MISMATCH: current ReviewScope mode differs from certificate")
	}
	if certMode != "TARGETED" {
		return nil
	}
	if req.Target == nil || scope.Target == nil || req.Target.Symbol != scope.Target.Symbol || req.Target.Kind != scope.Target.Kind {
		return errors.New("CERTIFIED_FINDINGS_REPORT_SCOPE_MISMATCH: report target differs from certified ReviewScope")
	}
	if !equalNormalizedReportPaths160(req.Scope.ScopedFiles, scope.ScopedFiles) {
		return errors.New("CERTIFIED_FINDINGS_REPORT_SCOPE_MISMATCH: report scopedFiles differ from certified ReviewScope")
	}
	return nil
}

func canonicalReviewScopeSHA160(data []byte) (string, error) {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return "", err
	}
	canonical, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	canonical = append(canonical, '\n')
	sum := sha256.Sum256(canonical)
	return fmt.Sprintf("%x", sum), nil
}

func equalNormalizedReportPaths160(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	a := make([]string, len(left))
	b := make([]string, len(right))
	for i, item := range left { a[i] = normalizeReportPath(item) }
	for i, item := range right { b[i] = normalizeReportPath(item) }
	sort.Strings(a)
	sort.Strings(b)
	for i := range a {
		if a[i] != b[i] { return false }
	}
	return true
}

func mapCertifiedFindings160
'@

$pattern = '(?s)func WriteCertifiedReport\(repoRoot string, req ReviewRequest\) \(string, error\) \{.*?\r?\n\}\r?\n\r?\nfunc mapCertifiedFindings160'
$updated = [regex]::Replace($text, $pattern, $replacement, 1)
if ($updated -eq $text) {
  throw 'Task 4 report authority patch did not match WriteCertifiedReport block'
}
[System.IO.File]::WriteAllText($path, $updated, [System.Text.UTF8Encoding]::new($false))

git config user.name 'codea-task4-bot'
git config user.email 'codea-task4-bot@users.noreply.github.com'
git add -- $path
if (-not (git diff --cached --quiet)) {
  git commit -m 'fix: bind formal report to certified scope'
  git push origin "HEAD:$env:TASK4_HEAD_BRANCH"
}
