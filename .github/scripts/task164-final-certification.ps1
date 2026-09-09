$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$root = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$base = '48158a74a5cbec61ac8936e1c65901013e757101'
$expected = $env:GITHUB_SHA
$version = '1.6.4'
$releaseRef = 'refs/heads/release/1.6.4-final-certification'
$evidence = Join-Path $root 'task164-final-evidence'
$checklistPath = Join-Path $root 'codea-harness-1.6.4-release-checklist.json'
$utf8 = [Text.UTF8Encoding]::new($false)
$results = [ordered]@{}
$artifacts = [ordered]@{}
$head = ''
New-Item -ItemType Directory -Force $evidence | Out-Null

function Write-Checklist([string]$Status) {
    $record = [ordered]@{
        version = $version
        status = $Status
        exactHeadSha = $head
        acceptedTask3Baseline = $base
        workflowRunId = $env:GITHUB_RUN_ID
        generatedAtUtc = [DateTime]::UtcNow.ToString('o')
        gates = $results
        artifacts = $artifacts
    }
    [IO.File]::WriteAllText($checklistPath, ($record | ConvertTo-Json -Depth 20), $utf8)
}

function Invoke-Checked([string]$Executable, [string[]]$Arguments) {
    & $Executable @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$Executable $($Arguments -join ' ') exited $LASTEXITCODE"
    }
}

function Invoke-Go([string[]]$Arguments) {
    Push-Location (Join-Path $root '.code-harness/tools-runtime')
    try {
        Invoke-Checked 'go' $Arguments
    } finally {
        Pop-Location
    }
}

function Invoke-Script([string]$Path) {
    Invoke-Checked 'pwsh' @('-NoProfile','-File',(Join-Path $root $Path))
}

function Invoke-Gate([string]$Name, [scriptblock]$Action, [string[]]$Markers = @()) {
    $log = Join-Path $evidence "$Name.log"
    $status = 'FAIL'
    $errorText = $null
    try {
        [IO.File]::WriteAllText($log, '', $utf8)
        $global:LASTEXITCODE = 0
        & $Action *>&1 | Tee-Object -FilePath $log
        if ($LASTEXITCODE -ne 0) { throw "Gate process exited $LASTEXITCODE" }
        $text = [IO.File]::ReadAllText($log)
        foreach ($marker in $Markers) {
            if (-not $text.Contains($marker)) { throw "Required evidence missing: $marker" }
        }
        $status = 'PASS'
        Write-Output "TASK164_FINAL_GATE PASS name=$Name"
    } catch {
        $errorText = ($_ | Out-String).Trim()
        Add-Content -Path $log -Value $errorText
        Write-Output "TASK164_FINAL_GATE FAIL name=$Name error=$errorText"
    } finally {
        $global:LASTEXITCODE = 0
        $results[$Name] = [ordered]@{
            status = $status
            log = "task164-final-evidence/$Name.log"
            error = $errorText
        }
        Write-Checklist 'PENDING'
    }
}

function Assert-ReleaseScope {
    $script:head = (git -C $root rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0 -or $head -notmatch '^[0-9a-f]{40}$') { throw 'Cannot resolve exact HEAD' }
    if ($head -ne $expected) { throw "Exact HEAD mismatch: $head != $expected" }
    if ($env:GITHUB_REF -cne $releaseRef) { throw "Unexpected release ref: $($env:GITHUB_REF)" }
    Invoke-Checked 'git' @('-C',$root,'merge-base','--is-ancestor',$base,$head)
    if ((Get-Content (Join-Path $root '.code-harness/VERSION') -Raw).Trim() -ne $version) {
        throw 'Release version mismatch'
    }
    $changed = @(& git -C $root diff --name-only "$base..$head")
    if ($LASTEXITCODE -ne 0) { throw 'Cannot inspect release scope' }
    $allowed = @(
        '.code-harness/VERSION',
        'CHANGELOG.md',
        '.code-harness/tools-runtime/cmd/codea-dcep-tools/task160_release_test.go',
        '.github/scripts/task164-final-certification.ps1',
        '.github/scripts/task164-release-package.ps1',
        '.github/workflows/task164-final-certification.yml',
        'docs/superpowers/plans/2026-09-09-codea-harness-1.6.4-final-certification-plan.md'
    )
    foreach ($path in $changed) {
        if ($path -cnotin $allowed) { throw "Unapproved release scope: $path" }
    }
    $productionChanges = @($changed | Where-Object {
        $_ -like '.code-harness/tools-runtime/*' -and $_ -ne '.code-harness/tools-runtime/cmd/codea-dcep-tools/task160_release_test.go'
    })
    if ($productionChanges.Count -ne 0) {
        throw "Task 1-3 production scope changed during release: $($productionChanges -join ',')"
    }
    Write-Output "TASK164_FINAL_SCOPE PASS head=$head files=$($changed.Count)"
}

function Assert-ReleaseArtifacts {
    $runtime = Join-Path $root '.code-harness/bin/codea-dcep-tools.exe'
    $ast = Join-Path $root '.code-harness/bin/ast-grep.exe'
    if (-not (Test-Path $runtime -PathType Leaf)) { throw 'Missing built Runtime' }
    if (-not (Test-Path $ast -PathType Leaf)) { throw 'Missing pinned ast-grep' }
    $runtimeHash = (Get-FileHash $runtime -Algorithm SHA256).Hash.ToLowerInvariant()
    $astHash = (Get-FileHash $ast -Algorithm SHA256).Hash.ToLowerInvariant()

    foreach ($kind in @('install','upgrade')) {
        $zip = Join-Path $root "codea-harness-1.6.4-windows-x64-$kind.zip"
        if (-not (Test-Path $zip -PathType Leaf)) { throw "Missing $kind ZIP" }
        $dest = Join-Path $env:RUNNER_TEMP ('task164-final-archive-' + $kind + '-' + [guid]::NewGuid().ToString('N'))
        try {
            Expand-Archive $zip $dest -Force
            $top = if ($kind -eq 'install') { '.code-harness' } else { '.code-harness-upgrade' }
            $releaseRoot = Join-Path $dest $top
            $manifestPath = Join-Path $releaseRoot 'RELEASE-MANIFEST.json'
            $manifest = Get-Content $manifestPath -Raw | ConvertFrom-Json
            if ([string]$manifest.version -ne $version) { throw "$kind manifest version mismatch" }
            if ([string]$manifest.buildCommit -ne $head) { throw "$kind manifest exact HEAD mismatch" }
            if ([string]$manifest.runtimeSha256 -ne $runtimeHash) { throw "$kind Runtime SHA mismatch" }
            if ([string]$manifest.astGrepSha256 -ne $astHash) { throw "$kind ast-grep SHA mismatch" }
            if ([string]$manifest.astGrepVersion -ne '0.42.1') { throw "$kind ast-grep version mismatch" }
            if ((Get-Content (Join-Path $releaseRoot 'VERSION') -Raw).Trim() -ne $version) {
                throw "$kind VERSION mismatch"
            }
            $files = @(Get-ChildItem $releaseRoot -Recurse -Force -File | Where-Object {
                $_.FullName -ne $manifestPath
            })
            if (@($manifest.managedFiles.PSObject.Properties).Count -ne $files.Count) {
                throw "$kind managed inventory count mismatch"
            }
            foreach ($file in $files) {
                $rel = [IO.Path]::GetRelativePath($releaseRoot,$file.FullName).Replace('\','/')
                $hash = (Get-FileHash $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
                if ([string]$manifest.managedFiles.$rel -ne $hash) {
                    throw "$kind managed inventory mismatch: $rel"
                }
            }
            if ((Get-FileHash (Join-Path $releaseRoot 'bin/codea-dcep-tools.exe') -Algorithm SHA256).Hash.ToLowerInvariant() -ne $runtimeHash) {
                throw "$kind packaged Runtime mismatch"
            }
            $artifacts[$kind] = [ordered]@{
                file = (Split-Path $zip -Leaf)
                sha256 = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLowerInvariant()
                size = (Get-Item $zip).Length
            }
        } finally {
            Remove-Item $dest -Recurse -Force -ErrorAction SilentlyContinue
        }
    }

    $signature = Get-AuthenticodeSignature $runtime
    $signatureStatus = if ($null -ne $signature.SignerCertificate) { [string]$signature.Status } else { 'Unsigned' }
    $goVersion = (go env GOVERSION).Trim()
    if ($LASTEXITCODE -ne 0) { throw 'Cannot determine Go version' }
    $whitelist = Join-Path $root 'codea-dcep-tools-whitelist.txt'
    $lines = @(
        'Product:','Codea Harness',
        'Version:',$version,
        'Binary:','codea-dcep-tools.exe',
        'Runtime SHA256:',$runtimeHash,
        'File Size:',[string](Get-Item $runtime).Length,
        'Build Commit:',$head,
        'GOOS:','windows',
        'GOARCH:','amd64',
        'Go Version:',$goVersion,
        'Signature Status:',$signatureStatus
    )
    if ($null -ne $signature.SignerCertificate) {
        $lines += @('Publisher:',[string]$signature.SignerCertificate.Subject)
    }
    [IO.File]::WriteAllLines($whitelist,$lines,$utf8)
    $artifacts['runtime'] = [ordered]@{
        binary = 'codea-dcep-tools.exe'
        sha256 = $runtimeHash
        size = (Get-Item $runtime).Length
        signatureStatus = $signatureStatus
    }
    $artifacts['astGrep'] = [ordered]@{
        binary = 'ast-grep.exe'
        version = '0.42.1'
        sha256 = $astHash
        size = (Get-Item $ast).Length
    }
    $artifacts['whitelist'] = [ordered]@{
        file = (Split-Path $whitelist -Leaf)
        sha256 = (Get-FileHash $whitelist -Algorithm SHA256).Hash.ToLowerInvariant()
    }
    Write-Output "TASK164_FINAL_ARTIFACTS PASS head=$head runtimeSha256=$runtimeHash"
}

Push-Location $root
try {
    Invoke-Gate 'exactHeadAndScope' { Assert-ReleaseScope } @('TASK164_FINAL_SCOPE PASS')
    if ($results.exactHeadAndScope.status -ne 'PASS') {
        Write-Checklist 'BLOCKED'
        exit 1
    }

    Invoke-Gate 'pinnedOpenCode' {
        Invoke-Checked 'npm' @('install','-g','opencode-ai@1.18.25')
        $out = (& opencode --version 2>&1 | Out-String).Trim()
        if ($LASTEXITCODE -ne 0 -or $out -notmatch '1\.18\.25') {
            throw "OpenCode version mismatch: $out"
        }
        Write-Output "TASK164_FINAL_OPENCODE PASS version=$out"
    } @('TASK164_FINAL_OPENCODE PASS')

    Invoke-Gate 'packageBuild' {
        Invoke-Script '.github/scripts/task164-release-package.ps1'
    } @('TASK164_RELEASE_PACKAGE_BUILD PASS version=1.6.4')

    if (Test-Path '.code-harness/bin/ast-grep.exe') {
        $env:CODEA_AST_GREP_TEST_PATH = (Resolve-Path '.code-harness/bin/ast-grep.exe').Path
    }

    Invoke-Gate 'fullGoRegression' {
        Invoke-Go @('test','-count=1','./...')
        Write-Output 'TASK164_FINAL_GO_TEST PASS'
    } @('TASK164_FINAL_GO_TEST PASS')

    Invoke-Gate 'goVet' {
        Invoke-Go @('vet','./...')
        Write-Output 'TASK164_FINAL_GO_VET PASS'
    } @('TASK164_FINAL_GO_VET PASS')

    Invoke-Gate 'task164Task1ScopeParity' {
        Invoke-Go @('test','-count=1','-v','./internal/analysis','-run','Test164Entrypoint|Test164CertifyEntrypointScopeWidening|Test164CertifyBatchProtocol')
        Invoke-Go @('test','-count=1','-v','./internal/nav','-run','Test164EntrypointBatch')
        Write-Output 'TASK164_FINAL_TASK1_PARITY PASS'
    } @('TASK164_FINAL_TASK1_PARITY PASS')

    Invoke-Gate 'task164Task1LargeWorkspace' {
        Invoke-Go @('test','-count=1','-v','./internal/analysis','-run','^Test164EntrypointPerformanceWindowsGate$')
        Write-Output 'TASK164_FINAL_TASK1_LARGE_WORKSPACE PASS'
    } @('TASK164_FINAL_TASK1_LARGE_WORKSPACE PASS')

    Invoke-Gate 'task164TelemetryPerformance' {
        Invoke-Go @('test','-count=1','-v','./internal/analysis','-run','^Test164CertifyPerformance')
        Write-Output 'TASK164_FINAL_PERFORMANCE PASS'
    } @('TASK164_TASK2_FIXTURE_21 PASS','TASK164_TASK2_FIXTURE_50 PASS','TASK164_TASK2_FIXTURE_100 PASS','TASK164_FINAL_PERFORMANCE PASS')

    Invoke-Gate 'task164FreshnessAuthority' {
        Invoke-Go @('test','-count=1','-v','./internal/changeset','-run','^Test164VerifyFreshness')
        Invoke-Go @('test','-count=1','-v','./internal/analysis','-run','^Test164CanonicalCertify.*FreshnessFastPath')
        Write-Output 'TASK164_FINAL_FRESHNESS_AUTHORITY PASS'
    } @('TASK164_FINAL_FRESHNESS_AUTHORITY PASS')

    Invoke-Gate 'retained163WorkspaceAST' {
        Invoke-Go @('test','-count=1','-v','./internal/nav','-run','Test163WorkspaceAST|TestWorkspace|Test152Nested')
        Write-Output 'TASK164_FINAL_RETAINED_163_WORKSPACE_AST PASS'
    } @('TASK164_FINAL_RETAINED_163_WORKSPACE_AST PASS')

    Invoke-Gate 'retained163CleanDiscovery' {
        Invoke-Go @('test','-count=1','-v','./internal/chain','-run','^Test163Task2')
        Invoke-Go @('test','-count=1','-v','./cmd/codea-dcep-tools','-run','^Test163Task2|TestChainDiscover')
        Write-Output 'TASK164_FINAL_RETAINED_163_DISCOVERY PASS'
    } @('TASK164_FINAL_RETAINED_163_DISCOVERY PASS')

    Invoke-Gate 'authorityFailureOrder' {
        Invoke-Go @('test','-count=1','-v','./internal/analysis','-run','^Test164CanonicalCertify.*FreshnessFastPath')
        Invoke-Go @('test','-count=1','-v','./cmd/codea-dcep-tools','-run','Test153ChainLoaderRejectsUncertifiedAnalysis|Test153ChainDiscoverRejectsUncertifiedAnalysis|Test153ChainRefreshRejectsUncertifiedAnalysis|Test153ChainReviewContextRejectsUncertifiedAnalysis')
        Invoke-Go @('test','-count=1','./internal/reviewselection','./internal/reviewscope','./internal/reviewunit','./internal/report')
        Write-Output 'TASK164_FINAL_AUTHORITY_FAILURE_ORDER PASS'
    } @('TASK164_FINAL_AUTHORITY_FAILURE_ORDER PASS')

    Invoke-Gate 'authoritySuccessUserSelection' {
        Invoke-Go @('test','-count=1','-v','./cmd/codea-dcep-tools','-run','Test153ExplicitTargetUserSelectionPreservesTargetForEveryUpstreamChoice|Test153ReviewSelectRejectsRehashedOptionSetDeletion')
        Invoke-Script '.github/scripts/task163-task3-active-contract-regression.ps1'
        Invoke-Script '.github/scripts/task163-task3-negative-control.ps1'
        Invoke-Script '.github/scripts/task163-task3-real-multi-chain-same-session-e2e.ps1'
        Write-Output 'TASK164_FINAL_AUTHORITY_SUCCESS_USER_SELECTION PASS'
    } @('TASK163_TASK3_ACTIVE_CONTRACT_HARD_STOP PASS','TASK163_TASK3_NEGATIVE_CONTROL_RED PASS','TASK163_TASK3_REAL_OPENCODE_SAME_SESSION_E2E PASS','TASK164_FINAL_AUTHORITY_SUCCESS_USER_SELECTION PASS')

    Invoke-Gate 'retained163UpgradeV2' {
        Invoke-Go @('test','-count=1','-v','./internal/upgrade')
        Invoke-Script '.github/scripts/task163-task4-package-regression.ps1'
        Write-Output 'TASK164_FINAL_RETAINED_163_UPGRADE PASS'
    } @('TASK163_TASK4_PACKAGE PASS kind=install','TASK163_TASK4_PACKAGE PASS kind=upgrade','TASK163_TASK4_INSTALLED_MANIFEST PASS','TASK164_FINAL_RETAINED_163_UPGRADE PASS')

    Invoke-Gate 'retained162Task1RealAgent' {
        Invoke-Script '.github/scripts/task162-review-reliability-task1-real-agent-e2e-v2.ps1'
    }
    Invoke-Gate 'retained162Task2SameSession' {
        Invoke-Script '.github/scripts/task162-review-reliability-task2-real-agent-e2e.ps1'
    }
    Invoke-Gate 'retained162RealPlainReview' {
        Invoke-Script '.github/scripts/task162-hotfix-task3-real-plain-review-e2e.ps1'
    }
    Invoke-Gate 'retained162AuthorityContracts' {
        Invoke-Script '.github/scripts/task162-review-reliability-task1-contract-regression.ps1'
        Invoke-Script '.github/scripts/task162-review-reliability-task2-contract-regression.ps1'
        Invoke-Script '.github/scripts/task162-hotfix-task1-agent-authority-regression.ps1'
        Invoke-Script '.github/scripts/task162-final-task2-invocation-contract-regression.ps1'
        Invoke-Go @('test','-count=1','./internal/reviewselection','./internal/reviewscope','./internal/reviewunit','./internal/reviewrules')
        Write-Output 'TASK164_FINAL_RETAINED_162_AUTHORITY PASS'
    } @('TASK164_FINAL_RETAINED_162_AUTHORITY PASS')

    Invoke-Gate 'releaseArtifacts' {
        Assert-ReleaseArtifacts
    } @('TASK164_FINAL_ARTIFACTS PASS')

    Invoke-Gate 'finalExactHead' {
        $now = (git -C $root rev-parse HEAD).Trim()
        if ($now -ne $head) { throw "HEAD changed during certification: $now != $head" }
        $dirty = @(git -C $root diff --name-only HEAD)
        if ($LASTEXITCODE -ne 0 -or $dirty.Count -ne 0) {
            throw "Tracked source changed during certification: $($dirty -join ',')"
        }
        Write-Output "TASK164_FINAL_EXACT_HEAD PASS head=$head"
    } @('TASK164_FINAL_EXACT_HEAD PASS')

    $failed = @($results.GetEnumerator() | Where-Object { $_.Value.status -ne 'PASS' } | ForEach-Object { $_.Key })
    if ($failed.Count -gt 0) {
        Write-Checklist 'BLOCKED'
        Write-Output "TASK164_FINAL_CERTIFICATION BLOCKED gates=$($failed -join ',')"
        exit 1
    }

    Write-Checklist 'PASS'
    Write-Output ([IO.File]::ReadAllText($checklistPath))
    Write-Output "TASK164_FINAL_CERTIFICATION PASS exactHead=$head"
    exit 0
} finally {
    Pop-Location
}
