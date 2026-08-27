$ErrorActionPreference = 'Stop'
$path = '.code-harness/tools-runtime/internal/report/review.go'
$text = Get-Content -Raw -LiteralPath $path
$changed = $false

if ($text -notmatch 'codea-harness-tools/internal/finding') {
    $lf = "`t`"strings`"`n)"
    $crlf = "`t`"strings`"`r`n)"
    if ($text.Contains($crlf)) {
        $text = $text.Replace($crlf, "`t`"strings`"`r`n`r`n`t`"codea-harness-tools/internal/finding`"`r`n)")
    } elseif ($text.Contains($lf)) {
        $text = $text.Replace($lf, "`t`"strings`"`n`n`t`"codea-harness-tools/internal/finding`"`n)")
    } else { throw 'strings import terminator not found' }
    $changed = $true
}

if ($text -notmatch 'AnchorKind\s+string') {
    $oldLF = "`tLine               int     ``json:`"line,omitempty`"```n`tProblem"
    $newLF = "`tLine               int     ``json:`"line,omitempty`"```n`tAnchorKind         string  ``json:`"anchorKind,omitempty`"```n`tSymbol             string  ``json:`"symbol,omitempty`"```n`tProblem"
    $oldCRLF = $oldLF.Replace("`n", "`r`n")
    $newCRLF = $newLF.Replace("`n", "`r`n")
    if ($text.Contains($oldCRLF)) { $text = $text.Replace($oldCRLF, $newCRLF) }
    elseif ($text.Contains($oldLF)) { $text = $text.Replace($oldLF, $newLF) }
    else { throw 'Finding line/problem fields not found' }
    $changed = $true
}

if ($changed) {
    Set-Content -LiteralPath $path -Value $text -Encoding utf8NoBOM
    git config user.name 'github-actions[bot]'
    git config user.email '41898282+github-actions[bot]@users.noreply.github.com'
    git add -- $path
    git commit -m 'fix: complete certified finding report transport'
    git push origin HEAD:$env:TASK4_HEAD_BRANCH
}
