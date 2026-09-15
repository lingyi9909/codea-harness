$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

# Keep the accepted 1.6.4 builder/installer unchanged. Adapt only the release
# version and add one primary command to the existing ownership transaction.
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$source = Join-Path $PSScriptRoot 'task164-release-package.ps1'
$expectedBlob = '8d8539c8967afb3f8b5f2285d0afd7ed41f63bcc'
$actualBlob = (& git -C $repoRoot hash-object -- $source).Trim()
if ($LASTEXITCODE -ne 0 -or $actualBlob -ne $expectedBlob) { throw "Retained release builder changed: $actualBlob" }
$version = (Get-Content (Join-Path $repoRoot '.code-harness/VERSION') -Raw).Trim()
if ($version -ne '1.6.7') { throw "Expected 1.6.7, got $version" }
$adapted = (Get-Content $source -Raw).Replace('1.6.4', '1.6.7')
# Extend the retained builder at fixed insertion points; retain its installer
# safety checks and add the primary command to the same preflight/copy list.
$toolLine = '        New-ReviewerHostTool $toolDestination'
if (-not $adapted.Contains($toolLine)) { throw 'Host tool insertion point missing' }
$adapted = $adapted.Replace($toolLine, @'
        New-ReviewerHostTool $toolDestination
        $primaryDestination = Join-Path $hostRoot '.opencode/commands/harness-review.md'
        Copy-Item -LiteralPath (Join-Path $repoRoot '.code-harness/commands/harness-review.md') -Destination $primaryDestination
'@)
$manifestLine = "                submissionTool = '.opencode/tools/codea-reviewer-submit.ts'"
$adapted = $adapted.Replace($manifestLine, @'
                primaryCommand = '.opencode/commands/harness-review.md'
                primaryCommandUpgradeSource = 'host/.opencode/commands/harness-review.md'
                primaryCommandSha256 = (Get-FileHash -Algorithm SHA256 $primaryDestination).Hash.ToLowerInvariant()
                submissionTool = '.opencode/tools/codea-reviewer-submit.ts'
'@)
$installerLine = '            Copy-Item -LiteralPath $installerSource -Destination $installerPath -Force'
$adapted = $adapted.Replace($installerLine, @'
            $installerText = Get-Content $installerSource -Raw
            $installerText = $installerText.Replace("    '.opencode/tools/codea-reviewer-submit.ts'", "    '.opencode/tools/codea-reviewer-submit.ts',`n    '.opencode/commands/harness-review.md'")
            [IO.File]::WriteAllText($installerPath, $installerText, [Text.UTF8Encoding]::new($false))
'@)
$temp = Join-Path $PSScriptRoot ('.release167-package-' + [guid]::NewGuid().ToString('N') + '.tmp.ps1')
try {
    [IO.File]::WriteAllText($temp, $adapted, [Text.UTF8Encoding]::new($false))
    & pwsh -NoProfile -File $temp
    if ($LASTEXITCODE -ne 0) { throw "1.6.7 package build failed: $LASTEXITCODE" }
} finally {
    Remove-Item $temp -Force -ErrorAction SilentlyContinue
}
