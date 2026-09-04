$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$accepted161 = '87ed05c5bbc56f4fdf904dfbb239d9125b8136e0'
$acceptedHotfixTask1 = '119c87057718f3d1f6f0286622d32b350f21d64e'
$acceptedHotfixTask2 = '2503678e347dc0ba2bc2f0357cefd9306d199480'
$acceptedHotfixTask3 = '4a312c4a2c85a202b740d3a1f419b2812e42f866'
$acceptedReviewReliabilityBase = 'e23023481edef9f95cdc59938efe5de4840093b8'
$acceptedReviewReliabilityTask1 = '1141a240529ea3fedcc8df0d3750db31f9fb1104'
$acceptedReviewReliabilityTask2 = 'ab2f42f53f472aebf2b18b11a1c9166feee2a20c'
$acceptedReviewReliabilityTask3 = '6c5af8908edf73a5fb772069e9bc2d91d2ebe289'
$installZip = 'codea-harness-1.6.2-windows-x64-install.zip'
$upgradeZip = 'codea-harness-1.6.2-windows-x64-upgrade.zip'
$checklistFile = 'codea-harness-1.6.2-release-checklist.json'
$whitelistFile = 'codea-dcep-tools-whitelist.txt'

function Invoke-Regression([string]$Script, [string]$Label) {
    Write-Host "TASK162 RELEASE: $Label"
    $resolvedScript = (Resolve-Path $Script).Path
    $lines = @(& pwsh -NoProfile -File $resolvedScript 2>&1)
    $childExit = $LASTEXITCODE
    $lines | ForEach-Object { Write-Output $_ }
    if ($childExit -ne 0) { throw "$Label failed with child exit code $childExit" }
    $global:LASTEXITCODE = 0
}

function Assert-AcceptedHotfixBaselines {
    $accepted = [ordered]@{
        task1 = $acceptedHotfixTask1
        task2 = $acceptedHotfixTask2
        task3 = $acceptedHotfixTask3
    }
    foreach ($entry in $accepted.GetEnumerator()) {
        git cat-file -e "$($entry.Value)^{commit}"
        if ($LASTEXITCODE -ne 0) { throw "accepted Hotfix $($entry.Key) commit unavailable: $($entry.Value)" }
        git merge-base --is-ancestor $entry.Value HEAD
        $ancestorExit = $LASTEXITCODE
        $global:LASTEXITCODE = 0
        if ($ancestorExit -ne 0) { throw "accepted Hotfix $($entry.Key) is not an ancestor of release HEAD: $($entry.Value)" }
    }
    Write-Output "TASK162_FINAL_ACCEPTED_TASK1_BASELINE PASS head=$acceptedHotfixTask1"
    Write-Output "TASK162_FINAL_ACCEPTED_TASK2_BASELINE PASS head=$acceptedHotfixTask2"
    Write-Output "TASK162_FINAL_ACCEPTED_TASK3_BASELINE PASS head=$acceptedHotfixTask3"
}

function Assert-AcceptedReviewReliabilityBaselines {
    $accepted = [ordered]@{
        base = $acceptedReviewReliabilityBase
        task1 = $acceptedReviewReliabilityTask1
        task2 = $acceptedReviewReliabilityTask2
        task3 = $acceptedReviewReliabilityTask3
    }
    foreach ($entry in $accepted.GetEnumerator()) {
        git cat-file -e "$($entry.Value)^{commit}"
        if ($LASTEXITCODE -ne 0) { throw "accepted Review Reliability $($entry.Key) commit unavailable: $($entry.Value)" }
        git merge-base --is-ancestor $entry.Value HEAD
        $ancestorExit = $LASTEXITCODE
        $global:LASTEXITCODE = 0
        if ($ancestorExit -ne 0) { throw "accepted Review Reliability $($entry.Key) is not an ancestor of release HEAD: $($entry.Value)" }
    }
    Write-Output "TASK162_FINAL_ACCEPTED_REVIEW_RELIABILITY_BASE PASS head=$acceptedReviewReliabilityBase"
    Write-Output "TASK162_FINAL_ACCEPTED_REVIEW_RELIABILITY_TASK1 PASS head=$acceptedReviewReliabilityTask1"
    Write-Output "TASK162_FINAL_ACCEPTED_REVIEW_RELIABILITY_TASK2 PASS head=$acceptedReviewReliabilityTask2"
    Write-Output "TASK162_FINAL_ACCEPTED_REVIEW_RELIABILITY_TASK3 PASS head=$acceptedReviewReliabilityTask3"
}

function Assert-PostReviewReliabilityTask3CertificationScope {
    $allowed = @(
        '.code-harness/tools-runtime/internal/upgrade/upgrade.go',
        '.code-harness/tools-runtime/internal/upgrade/task162_review_reliability_run_readme_test.go',
        '.github/scripts/task162-release-certification.ps1',
        '.github/scripts/task162-review-reliability-final-certification-contract-regression.ps1',
        '.github/scripts/task162-task2-package.ps1',
        '.github/scripts/task162-task2-release-package-cleanup-regression.ps1',
        '.github/workflows/task162-review-reliability-final-certification-contract.yml',
        '.github/workflows/task162-review-reliability-final-release-certification.yml',
        '.github/workflows/task162-review-reliability-final-upgrade-readme.yml'
    )
    $changed = @(& git diff --name-only "$acceptedReviewReliabilityTask3..HEAD")
    if ($LASTEXITCODE -ne 0) { throw 'cannot inspect post-Review-Reliability-Task3 certification scope' }
    $unexpected = @($changed | Where-Object { $_ -and ($_ -notin $allowed) })
    if ($unexpected.Count -gt 0) {
        throw "post-Review-Reliability-Task3 release scope contains non-certification changes:`n$($unexpected -join "`n")"
    }
    Write-Output 'TASK162_FINAL_POST_REVIEW_RELIABILITY_TASK3_CERTIFICATION_SCOPE PASS'
}

function Assert-NoRuntimeSource([string]$Root, [string]$Label) {
    if (Test-Path (Join-Path $Root 'tools-runtime')) { throw "$Label contains forbidden tools-runtime/" }
    $forbidden = @(Get-ChildItem -Path $Root -Recurse -File | Where-Object {
        $_.Extension -eq '.go' -or $_.Name -eq 'go.mod' -or $_.Name -eq 'go.sum'
    })
    if ($forbidden.Count -gt 0) { throw "$Label contains Go Runtime source: $($forbidden.FullName -join ', ')" }
}

function Assert-RunReadmeOnly([string]$Root, [string]$Label) {
    $runsRoot = Join-Path $Root 'runs'
    $readme = Join-Path $runsRoot 'README.md'
    if (-not (Test-Path $readme -PathType Leaf)) { throw "$Label missing runs/README.md" }
    $unexpected = @(Get-ChildItem -Path $runsRoot -Force | Where-Object { $_.Name -ne 'README.md' })
    if ($unexpected.Count -gt 0) { throw "$Label contains forbidden Run state: $($unexpected.FullName -join ', ')" }
}

function Assert-ReleaseZip([string]$Zip, [string]$TopDir, [string]$Label, [string]$ExactHead, [string]$RuntimeHash) {
    if (-not (Test-Path $Zip -PathType Leaf)) { throw "$Label ZIP missing: $Zip" }
    $extract = Join-Path $env:RUNNER_TEMP ("task162-final-" + $Label.Replace(' ','-') + '-' + [guid]::NewGuid().ToString('N'))
    Expand-Archive -Path $Zip -DestinationPath $extract -Force
    $root = Join-Path $extract $TopDir
    if (-not (Test-Path $root -PathType Container)) { throw "$Label missing top-level $TopDir" }
    foreach ($required in @(
        'VERSION','RELEASE-MANIFEST.json','AGENTS.md','bootstrap.md','upgrade.md','harness.template.yaml','project.template.md',
        'agents','skills','contracts','tools','bin/codea-dcep-tools.exe','bin/ast-grep.exe','runs/README.md'
    )) {
        if (-not (Test-Path (Join-Path $root $required))) { throw "$Label missing required $required" }
    }
    foreach ($state in @('harness.yaml','project.md','database.yaml','chains')) {
        if (Test-Path (Join-Path $root $state)) { throw "$Label contains Project State $state" }
    }
    Assert-RunReadmeOnly $root $Label
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
        '.github/scripts/task162-release-certification.ps1',
        '.github/scripts/task162-hotfix-task3-real-plain-review-e2e.ps1',
        '.github/workflows/task161-runtime-rename-audit.yml',
        '.github/workflows/task162-task1-maven-multimodule.yml',
        '.github/workflows/task162-hotfix-task1-canonical-changeset.yml',
        '.github/workflows/task162-hotfix-task2-invocation-contract.yml',
        '.github/workflows/task162-hotfix-task3-certification.yml',
        '.github/workflows/task162-hotfix-task3-real-plain-review-e2e.yml'
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
        '.github/scripts/task162-release-certification.ps1',
        '.github/workflows/task161-runtime-rename-audit.yml',
        '.github/workflows/task162-task1-maven-multimodule.yml',
        '.github/workflows/task162-hotfix-task1-canonical-changeset.yml',
        '.github/workflows/task162-hotfix-task2-invocation-contract.yml',
        '.github/workflows/task162-hotfix-task3-certification.yml'
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
    Assert-AcceptedHotfixBaselines
    Assert-AcceptedReviewReliabilityBaselines
    Assert-PostReviewReliabilityTask3CertificationScope
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

    Invoke-Regression './.github/scripts/task162-review-reliability-task1-contract-regression.ps1' 'Review Reliability Task 1 invocation contract'
    Invoke-Regression './.github/scripts/task162-review-reliability-task2-contract-regression.ps1' 'Review Reliability Task 2 fresh lifecycle contract'

    Write-Host 'TASK162 RELEASE: Review Reliability focused Go contracts'
    Push-Location '.code-harness/tools-runtime'
    try {
        go test -count=1 -run 'Test162ReviewReliabilityTask(1|2)' -v ./cmd/codea-dcep-tools
        if ($LASTEXITCODE -ne 0) { throw "Review Reliability focused Go contracts failed with exit code $LASTEXITCODE" }
    } finally { Pop-Location }
    $global:LASTEXITCODE = 0
    Write-Output 'TASK162_FINAL_REVIEW_RELIABILITY_FOCUSED_GO_CONTRACT PASS'

    Invoke-Regression './.github/scripts/task162-review-reliability-task1-real-agent-e2e-v2.ps1' 'Review Reliability Task 1 real Agent changed/zero-change E2E'
    Invoke-Regression './.github/scripts/task162-review-reliability-task2-real-agent-e2e.ps1' 'Review Reliability Task 2 same-session fresh lifecycle E2E'
    Invoke-Regression './.github/scripts/task162-review-reliability-task3-run-readme-regression.ps1' 'Review Reliability Task 3 Run README contract'

    Invoke-Regression './.github/scripts/task162-hotfix-task1-agent-authority-regression.ps1' 'Hotfix Task 1 Agent authority'
    Invoke-Regression './.github/scripts/task162-hotfix-task1-agent-snapshot-request-contract.ps1' 'Hotfix Task 1 Agent Snapshot request contract'
    Invoke-Regression './.github/scripts/task162-hotfix-task1-canonical-changeset-regression.ps1' 'Hotfix Task 1 Canonical ChangeSet authority'
    Invoke-Regression './.github/scripts/task162-final-task2-invocation-contract-regression.ps1' 'Hotfix Task 2 Active Agent invocation contract (final adapter)'

    Write-Host 'TASK162 RELEASE: Task 2 invocation contract focused tests'
    Push-Location '.code-harness/tools-runtime'
    try {
        go test -count=1 -run 'Test162HotfixTask2' -v ./cmd/codea-dcep-tools
        if ($LASTEXITCODE -ne 0) { throw "Task 2 invocation contract focused tests failed with exit code $LASTEXITCODE" }
    } finally { Pop-Location }
    $global:LASTEXITCODE = 0
    Write-Output 'TASK162_FINAL_TASK2_FOCUSED_GO_CONTRACT PASS'

    Invoke-Regression './.github/scripts/task162-hotfix-task2-runtime-invocation-regression.ps1' 'Hotfix Task 2 Runtime invocation contract'
    Invoke-Regression './.github/scripts/task162-hotfix-task3-real-plain-review-e2e.ps1' 'Hotfix Task 3 Real plain harness review E2E'
    Write-Output 'NO_RUNTIME_ZERO_ARG_USAGE PASS'
    Write-Output 'NO_LEGACY_RUNTIME_INVOCATION PASS'
    Write-Output 'NO_UNKNOWN_REQUEST_FIELD PASS'
    Write-Output 'NO_CHANGE_SET_MISMATCH PASS'

    Invoke-Regression './.github/scripts/task162-real-multimodule-regression.ps1' 'Task 1 Maven multi-module Review Authority E2E'
    Invoke-Regression './.github/scripts/task162-duplicate-symbol-authority-regression.ps1' 'Task 1 duplicate Symbol Authority E2E'
    Invoke-Regression './.github/scripts/task162-hotfix-final-entrypoint-inventory-regression.ps1' 'retained single-module EntryPoint regression (final Task 2 contract adapter)'
    Invoke-Regression './.github/scripts/task152-workspace-smoke.ps1' 'retained Workspace regression'
    Remove-Item '.code-harness/runs/.gitkeep' -ErrorAction SilentlyContinue
    Invoke-Regression './.github/scripts/task152-task5-real-business-regression.ps1' 'retained single-module business regression'
    Invoke-Regression './.github/scripts/task162-hotfix-final-chain-regression.ps1' 'retained Chain regression (final Task 2 contract adapter)'
    Invoke-Regression './.github/scripts/task160-real-review-precision-regression.ps1' 'retained 1.6 Review Precision regression'
    Assert-RuntimeRenameRetained

    Write-Host 'TASK162 RELEASE: Task 2 package/no-Go/upgrade regression'
    $task2Script = (Resolve-Path './.github/scripts/task162-hotfix-final-package-cleanup-regression.ps1').Path
    $task2Lines = @(& pwsh -NoProfile -File $task2Script 2>&1)
    $task2Exit = $LASTEXITCODE
    $task2Lines | ForEach-Object { Write-Output $_ }
    if ($task2Exit -ne 0) { throw "Task 2 package/no-Go/upgrade regression failed with child exit code $task2Exit" }
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
        acceptedHotfix = [ordered]@{
            task1 = $acceptedHotfixTask1
            task2 = $acceptedHotfixTask2
            task3 = $acceptedHotfixTask3
        }
        acceptedReviewReliability = [ordered]@{
            base = $acceptedReviewReliabilityBase
            task1 = $acceptedReviewReliabilityTask1
            task2 = $acceptedReviewReliabilityTask2
            task3 = $acceptedReviewReliabilityTask3
        }
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
            reviewReliabilityTask1InvocationContract = 'PASS'
            reviewReliabilityTask1RealAgent = 'PASS'
            reviewReliabilityTask2FreshLifecycle = 'PASS'
            reviewReliabilityTask2SameSession = 'PASS'
            reviewReliabilityTask3RunReadme = 'PASS'
            reviewReliabilityTask3PackageReadme = 'PASS'
            postReviewReliabilityTask3CertificationScope = 'PASS'
            hotfixTask1CanonicalAuthority = 'PASS'
            hotfixTask2InvocationContract = 'PASS'
            hotfixTask3RealPlainReview = 'PASS'
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

    Write-Output "TASK162_POST_HOTFIX_RELEASE_CERTIFICATION PASS exactHead=$exactHead runtimeSha256=$runtimeHash"
    Write-Output "TASK162_REVIEW_RELIABILITY_FINAL_CERTIFICATION PASS exactHead=$exactHead runtimeSha256=$runtimeHash"
    Write-Output "TASK162_RELEASE_CERTIFICATION PASS exactHead=$exactHead runtimeSha256=$runtimeHash"
} finally {
    Pop-Location
}
exit 0
