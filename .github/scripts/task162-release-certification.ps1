$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$accepted161 = '87ed05c5bbc56f4fdf904dfbb239d9125b8136e0'
$installZip = 'codea-harness-1.6.2-windows-x64-install.zip'
$upgradeZip = 'codea-harness-1.6.2-windows-x64-upgrade.zip'
$checklistFile = 'codea-harness-1.6.2-release-checklist.json'
$whitelistFile = 'codea-dcep-tools-whitelist.txt'

function Invoke-Regression([string]$Script, [string]$Label) {
    Write-Host "TASK162 RELEASE: $Label"
    & $Script
    if ($LASTEXITCODE -ne 0) { throw "$Label failed with exit code $LASTEXITCODE" }
    $global:LASTEXITCODE = 0
}

function Assert-NoRuntimeSource([string]$Root, [string]$Label) {
    if (Test-Path (Join-Path $Root 'tools-runtime')) { throw "$Label contains forbidden tools-runtime/" }
    $forbidden = @(Get-ChildItem -Path $Root -Recurse -File | Where-Object {
        $_.Extension -eq '.go' -or $_.Name -eq 'go.mod' -or $_.Name -eq 'go.sum'
    })
    if ($forbidden.Count -gt 0) { throw "$Label contains Go Runtime source: $($forbidden.FullName -join ', ')" }
}

function Assert-ReleaseZip([string]$Zip, [string]$TopDir, [string]$Label, [string]$ExactHead, [string]$RuntimeHash) {
    if (-not (Test-Path $Zip -PathType Leaf)) { throw "$Label ZIP missing: $Zip" }
    $extract = Join-Path $env:RUNNER_TEMP ("task162-final-" + $Label.Replace(' ','-') + '-' + [guid]::NewGuid().ToString('N'))
    Expand-Archive -Path $Zip -DestinationPath $extract -Force
    $root = Join-Path $extract $TopDir
    if (-not (Test-Path $root -PathType Container)) { throw "$Label missing top-level $TopDir" }
    foreach ($required in @(
        'VERSION','RELEASE-MANIFEST.json','AGENTS.md','bootstrap.md','upgrade.md','harness.template.yaml','project.template.md',
        'agents','skills','contracts','tools','bin/codea-dcep-tools.exe','bin/ast-grep.exe'
    )) {
        if (-not (Test-Path (Join-Path $root $required))) { throw "$Label missing required $required" }
    }
    foreach ($state in @('harness.yaml','project.md','database.yaml','runs','chains')) {
        if (Test-Path (Join-Path $root $state)) { throw "$Label contains Project State $state" }
    }
    Assert-NoRuntimeSource $root $Label
    $manifest = Get-Content (Join-Path $root 'RELEASE-MANIFEST.json') -Raw | ConvertFrom-Json
    if ([string]$manifest.version -ne '1.6.2') { throw "$Label manifest version mismatch" }
    if ([string]$manifest.buildCommit -ne $ExactHead) { throw "$Label manifest buildCommit mismatch" }
    if ([string]$manifest.runtime -ne 'codea-dcep-tools.exe') { throw "$Label manifest runtime mismatch" }
    if ([string]$manifest.runtimeSha256 -ne $RuntimeHash) { throw "$Label manifest runtime hash mismatch" }
    if ([string]$manifest.astGrepVersion -ne '0.42.1') { throw "$Label ast-grep version mismatch" }
    $zipRuntimeHash = (Get-FileHash -Algorithm SHA256 (Join-Path $root 'bin/codea-dcep-tools.exe')).Hash.ToLowerInvariant()
    if ($zipRuntimeHash -ne $RuntimeHash) { throw "$Label Runtime hash does not match release Runtime" }
    return $root
}

function Assert-RuntimeRenameRetained {
    if (-not (Test-Path '.code-harness/tools-runtime/cmd/codea-dcep-tools' -PathType Container)) { throw 'renamed Runtime command source missing' }
    if (Test-Path '.code-harness/tools-runtime/cmd/codea-harness-tools') { throw 'legacy Runtime command source still exists' }
    if (-not (Test-Path '.code-harness/bin/codea-dcep-tools.exe' -PathType Leaf)) { throw 'renamed Runtime binary missing' }
    if (Test-Path '.code-harness/bin/codea-harness-tools.exe') { throw 'legacy Runtime binary still exists' }

    $legacyRefs = @(& git grep -n 'codea-harness-tools\.exe' -- .code-harness .github/scripts .github/workflows README.md 2>$null)
    if ($LASTEXITCODE -gt 1) { throw 'git grep legacy Runtime references failed' }
    $allowed = @(
        '.code-harness/tools-runtime/internal/upgrade/task161_release_upgrade_test.go',
        '.github/scripts/task161-release.ps1',
        '.github/scripts/task161-real-160-upgrade.ps1',
        '.github/workflows/task161-runtime-rename-audit.yml',
        '.github/workflows/task162-task1-maven-multimodule.yml',
        '.github/scripts/task162-release-certification.ps1'
    )
    $unexpected = @($legacyRefs | Where-Object {
        $line = [string]$_
        -not ($allowed | Where-Object { $line.StartsWith($_ + ':') })
    })
    if ($unexpected.Count -gt 0) { throw "unexpected legacy Runtime refs:`n$($unexpected -join "`n")" }

    $legacyCmdRefs = @(& git grep -n 'cmd/codea-harness-tools' -- .code-harness .github/scripts .github/workflows README.md 2>$null)
    if ($LASTEXITCODE -gt 1) { throw 'git grep legacy Runtime command references failed' }
    $allowedCmd = @(
        '.github/scripts/task161-real-160-upgrade.ps1',
        '.github/workflows/task161-runtime-rename-audit.yml',
        '.github/workflows/task162-task1-maven-multimodule.yml',
        '.github/scripts/task162-release-certification.ps1'
    )
    $unexpectedCmd = @($legacyCmdRefs | Where-Object {
        $line = [string]$_
        -not ($allowedCmd | Where-Object { $line.StartsWith($_ + ':') })
    })
    if ($unexpectedCmd.Count -gt 0) { throw "unexpected legacy Runtime command refs:`n$($unexpectedCmd -join "`n")" }
    $global:LASTEXITCODE = 0
    Write-Output 'TASK161_RUNTIME_RENAME_AUDIT PASS'
}

Push-Location $repoRoot
try {
    $version = (Get-Content '.code-harness/VERSION' -Raw).Trim()
    if ($version -ne '1.6.2') { throw "unexpected release version: $version" }
    $exactHead = (git rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0 -or $exactHead -notmatch '^[0-9a-f]{40}$') { throw 'cannot resolve exact HEAD' }
    git cat-file -e "$accepted161^{commit}"
    if ($LASTEXITCODE -ne 0) { throw "accepted 1.6.1 baseline unavailable: $accepted161" }
    $goVersion = (go env GOVERSION).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($goVersion)) { throw 'cannot resolve Go version' }

    Write-Host 'TASK162 RELEASE: full Go regression'
    Push-Location '.code-harness/tools-runtime'
    try {
        go test -count=1 ./...
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        go vet ./...
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    } finally { Pop-Location }

    & './.github/scripts/task162-task2-package.ps1'
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    $global:LASTEXITCODE = 0

    Invoke-Regression './.github/scripts/task162-real-multimodule-regression.ps1' 'Task 1 Maven multi-module Review Authority E2E'
    Invoke-Regression './.github/scripts/task162-duplicate-symbol-authority-regression.ps1' 'Task 1 duplicate Symbol Authority E2E'
    Invoke-Regression './.github/scripts/task153-task1-real-entrypoint-inventory.ps1' 'retained single-module EntryPoint regression'
    Invoke-Regression './.github/scripts/task152-workspace-smoke.ps1' 'retained Workspace regression'
    Remove-Item '.code-harness/runs/.gitkeep' -ErrorAction SilentlyContinue
    Invoke-Regression './.github/scripts/task152-task5-real-business-regression.ps1' 'retained single-module business regression'
    Invoke-Regression './.github/scripts/task153-real-review-chain-regression.ps1' 'retained Chain regression'
    Invoke-Regression './.github/scripts/task160-real-review-precision-regression.ps1' 'retained 1.6 Review Precision regression'
    Assert-RuntimeRenameRetained

    Write-Host 'TASK162 RELEASE: Task 2 package/no-Go/upgrade regression'
    $task2Output = & './.github/scripts/task162-task2-release-package-cleanup-regression.ps1' 2>&1 | Tee-Object -Variable task2Lines
    $task2Text = ($task2Lines | Out-String)
    foreach ($marker in @(
        'TASK162_TASK2_ARTIFACT_CLEAN PASS',
        'TASK162_TASK2_NEW_INSTALL_NO_GO_ANALYSIS_REVIEW PASS',
        'TASK162_TASK2_REAL_161_TO_162_UPGRADE PASS',
        'TASK162_TASK2_PROJECT_STATE_PRESERVATION PASS',
        'TASK162_TASK2_RELEASE_PACKAGE_CLEANUP_E2E PASS'
    )) {
        if ($task2Text -notmatch [regex]::Escape($marker)) { throw "Task 2 final certification missing marker: $marker`n$task2Text" }
    }
    $global:LASTEXITCODE = 0

    Write-Host 'TASK162 RELEASE: rebuild final official artifacts'
    & './.github/scripts/task162-task2-package.ps1'
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    $global:LASTEXITCODE = 0

    $runtime = '.code-harness/bin/codea-dcep-tools.exe'
    if (-not (Test-Path $runtime -PathType Leaf)) { throw 'final Runtime missing' }
    $runtimeHash = (Get-FileHash -Algorithm SHA256 $runtime).Hash.ToLowerInvariant()
    $runtimeSize = (Get-Item $runtime).Length
    $signature = Get-AuthenticodeSignature $runtime
    $signatureStatus = if ($null -ne $signature.SignerCertificate) { [string]$signature.Status } else { 'Unsigned' }

    $whitelistLines = @(
        'Product:', 'Codea Harness',
        'Version:', '1.6.2',
        'Binary:', 'codea-dcep-tools.exe',
        'Runtime SHA256:', $runtimeHash,
        'File Size:', [string]$runtimeSize,
        'Build Commit:', $exactHead,
        'GOOS:', 'windows',
        'GOARCH:', 'amd64',
        'Go Version:', $goVersion,
        'Signature Status:', $signatureStatus
    )
    if ($null -ne $signature.SignerCertificate) { $whitelistLines += @('Publisher:', [string]$signature.SignerCertificate.Subject) }
    [IO.File]::WriteAllLines($whitelistFile, $whitelistLines, [Text.UTF8Encoding]::new($false))

    $installRoot = Assert-ReleaseZip $installZip '.code-harness' 'install package' $exactHead $runtimeHash
    $upgradeRoot = Assert-ReleaseZip $upgradeZip '.code-harness-upgrade' 'upgrade package' $exactHead $runtimeHash

    $installHash = (Get-FileHash -Algorithm SHA256 $installZip).Hash.ToLowerInvariant()
    $upgradeHash = (Get-FileHash -Algorithm SHA256 $upgradeZip).Hash.ToLowerInvariant()
    $checklist = [ordered]@{
        version = '1.6.2'
        exactHeadSha = $exactHead
        acceptedBaseline161 = $accepted161
        runtime = [ordered]@{
            binary = 'codea-dcep-tools.exe'
            sha256 = $runtimeHash
            size = $runtimeSize
            goVersion = $goVersion
            signatureStatus = $signatureStatus
        }
        whitelist = [ordered]@{
            file = $whitelistFile
            version = '1.6.2'
            buildCommit = $exactHead
            runtimeSha256 = $runtimeHash
        }
        artifacts = [ordered]@{
            install = [ordered]@{ file=$installZip; sha256=$installHash; size=(Get-Item $installZip).Length }
            upgrade = [ordered]@{ file=$upgradeZip; sha256=$upgradeHash; size=(Get-Item $upgradeZip).Length }
        }
        gates = [ordered]@{
            task1MavenMultiModule = 'PASS'
            task1DuplicateSymbolAuthority = 'PASS'
            task2PackageCleanup = 'PASS'
            newInstallNoGoRuntime = 'PASS'
            real161To162Upgrade = 'PASS'
            projectStatePreservation = 'PASS'
            singleModuleEntrypoint = 'PASS'
            workspaceRetained = 'PASS'
            singleModuleBusinessRetained = 'PASS'
            chainRetained = 'PASS'
            reviewPrecision160Retained = 'PASS'
            runtimeRename161Retained = 'PASS'
            fullGoRegression = 'PASS'
            goVet = 'PASS'
            installZipClean = 'PASS'
            upgradeZipClean = 'PASS'
            whitelistEvidence = 'PASS'
            exactHead = 'PASS'
        }
    } | ConvertTo-Json -Depth 10
    [IO.File]::WriteAllText($checklistFile, $checklist, [Text.UTF8Encoding]::new($false))

    Write-Output "TASK162_RELEASE_CERTIFICATION PASS exactHead=$exactHead runtimeSha256=$runtimeHash"
} finally {
    Pop-Location
}
exit 0
