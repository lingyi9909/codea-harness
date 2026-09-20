$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$root = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$base = '6605916b4929434ea3362ab5b4fc6325ca117a2f'
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
$env:TASK164_FINAL_EVIDENCE_DIR = $evidence

function Write-Checklist([string]$Status) {
    $record = [ordered]@{
        version = $version
        status = $Status
        exactHeadSha = $head
        closureHotfixBase = $base
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
    if ($LASTEXITCODE -ne 0) { throw 'Cannot inspect Task 4 scope' }
    $allowed = @(
        '.code-harness/AGENTS.md',
        '.code-harness/bootstrap.md',
        '.code-harness/agents/orchestrator.md',
        '.code-harness/contracts/reviewer-host-contract.md',
        '.github/scripts/task164-closure-opencode-resolved-contract.ps1',
        '.github/scripts/task164-closure-install-e2e.ps1',
        '.github/scripts/task164-closure-agent-contract.ps1',
        '.github/scripts/task164-install.ps1',
        '.github/scripts/task164-release-package.ps1',
        '.github/scripts/task164-release-blocker-task2-e2e.ps1',
        '.github/scripts/task164-task4-packaged-plain-review-e2e.ps1',
        '.github/scripts/task164-task4-plain-review-server.py',
        '.github/scripts/task164-task4-progress-interruption-e2e.ps1',
        '.github/workflows/task164-closure-product-e2e.yml',
        '.github/workflows/task164-final-certification.yml',
        '.github/scripts/task164-final-certification-contract.py',
        '.github/scripts/task164-final-certification.ps1',
        'README.md',
        'docs/superpowers/plans/2026-09-11-codea-harness-1.6.4-final-certification-closure-hotfix-plan.md'
    )
    foreach ($path in $changed) {
        if ($path -cnotin $allowed) { throw "Unapproved Task 4 scope: $path" }
    }
    $runtimeGoChanges = @($changed | Where-Object {
        $_ -like '.code-harness/tools-runtime/*' -and ([string]$_).EndsWith('.go',[StringComparison]::OrdinalIgnoreCase)
    })
    if ($runtimeGoChanges.Count -ne 0) {
        throw "Runtime Go implementation changed during Closure: $($runtimeGoChanges -join ',')"
    }
    Write-Output "TASK164_FINAL_SCOPE PASS head=$head files=$($changed.Count) closureHotfixBase=$base"
}

function Build-RevokedRCUpgrade {
    $rc = Join-Path $env:RUNNER_TEMP 'task164-final-revoked-rc'
    $out = Join-Path $env:RUNNER_TEMP 'task164-revoked-rc-upgrade.zip'
    Remove-Item $rc -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item $out -Force -ErrorAction SilentlyContinue
    Invoke-Checked 'git' @('-C',$root,'worktree','add','--detach',$rc,'6aa5d9dad0623cd60a845360c9b20ab153921e87')
    try {
        Invoke-Checked 'pwsh' @('-NoProfile','-File',(Join-Path $rc '.github/scripts/task164-release-package.ps1'))
        $built = Join-Path $rc 'codea-harness-1.6.4-windows-x64-upgrade.zip'
        if (-not (Test-Path $built -PathType Leaf)) { throw 'Revoked RC did not produce upgrade package' }
        Copy-Item $built $out -Force
        Write-Output 'TASK164_FINAL_REVOKED_RC_PACKAGE PASS head=6aa5d9dad0623cd60a845360c9b20ab153921e87'
    } finally {
        & git -C $root worktree remove --force $rc 2>&1 | Out-Null
        $global:LASTEXITCODE = 0
    }
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

            # Framework ownership and Reviewer Host ownership are deliberately
            # separate. Upgrade Host resources are staged transactionally under
            # host/ and governed by hostAgents rather than managedFiles.
            $allReleaseFiles = @(Get-ChildItem $releaseRoot -Recurse -Force -File | Where-Object {
                $_.FullName -ne $manifestPath
            })
            $managedReleaseFiles = @(
                foreach ($file in $allReleaseFiles) {
                    $rel = [IO.Path]::GetRelativePath($releaseRoot,$file.FullName).Replace('\','/')
                    if ($kind -eq 'upgrade' -and $rel.StartsWith('host/', [StringComparison]::Ordinal)) { continue }
                    $file
                }
            )
            $managedProperties = @($manifest.managedFiles.PSObject.Properties)
            if ($managedProperties.Count -ne $managedReleaseFiles.Count) {
                throw "$kind managed framework inventory count mismatch"
            }
            foreach ($property in $managedProperties) {
                if ([string]$property.Name -like 'host/*') {
                    throw "$kind host payload leaked into managedFiles: $($property.Name)"
                }
            }
            foreach ($file in $managedReleaseFiles) {
                $rel = [IO.Path]::GetRelativePath($releaseRoot,$file.FullName).Replace('\','/')
                $hash = (Get-FileHash $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
                if ([string]$manifest.managedFiles.$rel -ne $hash) {
                    throw "$kind managed inventory mismatch: $rel"
                }
            }

            $reviewerHost = $manifest.hostAgents.reviewer
            if ($null -eq $reviewerHost) { throw "$kind Reviewer Host metadata missing" }
            if ([string]$reviewerHost.path -cne '.opencode/agents/reviewer.md' -or
                [string]$reviewerHost.upgradeSource -cne 'host/.opencode/agents/reviewer.md' -or
                [string]$reviewerHost.command -cne '.opencode/commands/harness-review-reviewer.md' -or
                [string]$reviewerHost.commandUpgradeSource -cne 'host/.opencode/commands/harness-review-reviewer.md' -or
                [string]$reviewerHost.submissionTool -cne '.opencode/tools/codea-reviewer-submit.ts' -or
                [string]$reviewerHost.submissionToolUpgradeSource -cne 'host/.opencode/tools/codea-reviewer-submit.ts') {
                throw "$kind Reviewer Host path metadata mismatch"
            }

            if ($kind -eq 'install') {
                $hostRoot = $dest
                $hostPaths = @([string]$reviewerHost.path,[string]$reviewerHost.command,[string]$reviewerHost.submissionTool)
                $actualHostFiles = @(Get-ChildItem (Join-Path $dest '.opencode') -Recurse -Force -File)
                $actualHostRel = @($actualHostFiles | ForEach-Object {
                    [IO.Path]::GetRelativePath($dest,$_.FullName).Replace('\','/')
                } | Sort-Object)
            } else {
                $hostRoot = $releaseRoot
                $hostPaths = @([string]$reviewerHost.upgradeSource,[string]$reviewerHost.commandUpgradeSource,[string]$reviewerHost.submissionToolUpgradeSource)
                $actualHostFiles = @(
                    foreach ($file in $allReleaseFiles) {
                        $rel = [IO.Path]::GetRelativePath($releaseRoot,$file.FullName).Replace('\','/')
                        if ($rel.StartsWith('host/', [StringComparison]::Ordinal)) { $file }
                    }
                )
                $actualHostRel = @($actualHostFiles | ForEach-Object {
                    [IO.Path]::GetRelativePath($releaseRoot,$_.FullName).Replace('\','/')
                } | Sort-Object)
            }
            $expectedHostRel = @($hostPaths | Sort-Object)
            if ($actualHostFiles.Count -ne 3 -or ($actualHostRel -join '|') -cne ($expectedHostRel -join '|')) {
                throw "$kind Reviewer Host inventory mismatch actual=$($actualHostRel -join ',')"
            }
            $hostChecks = @(
                [pscustomobject]@{ Path=(Join-Path $hostRoot $hostPaths[0]); Hash=[string]$reviewerHost.sha256; Name='reviewer' },
                [pscustomobject]@{ Path=(Join-Path $hostRoot $hostPaths[1]); Hash=[string]$reviewerHost.commandSha256; Name='command' },
                [pscustomobject]@{ Path=(Join-Path $hostRoot $hostPaths[2]); Hash=[string]$reviewerHost.submissionToolSha256; Name='submission-tool' }
            )
            foreach ($check in $hostChecks) {
                if (-not (Test-Path $check.Path -PathType Leaf)) { throw "$kind Reviewer Host $($check.Name) missing" }
                $actualHash = (Get-FileHash $check.Path -Algorithm SHA256).Hash.ToLowerInvariant()
                if ($actualHash -cne $check.Hash) { throw "$kind Reviewer Host $($check.Name) SHA mismatch" }
            }

            if ((Get-FileHash (Join-Path $releaseRoot 'bin/codea-dcep-tools.exe') -Algorithm SHA256).Hash.ToLowerInvariant() -ne $runtimeHash) {
                throw "$kind packaged Runtime mismatch"
            }
            $artifacts[$kind] = [ordered]@{
                file = (Split-Path $zip -Leaf)
                sha256 = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLowerInvariant()
                size = (Get-Item $zip).Length
                reviewerHost = [ordered]@{
                    agentSha256 = [string]$reviewerHost.sha256
                    commandSha256 = [string]$reviewerHost.commandSha256
                    submissionToolSha256 = [string]$reviewerHost.submissionToolSha256
                }
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
    } @(
        'TASK164_RELEASE_PACKAGE_BUILD PASS version=1.6.4',
        'INSTALL_SAFE_ENTRYPOINT_PACKAGED PASS file=install.ps1'
    )

    Invoke-Gate 'closureResolvedReviewerHost' {
        Invoke-Script '.github/scripts/task164-closure-opencode-resolved-contract.ps1'
    } @(
        'OPENCODE_11825_REVIEWER_PERMISSION_RESOLVED PASS',
        'OPENCODE_11825_REVIEWER_COMMAND_SUBTASK_RESOLVED PASS',
        'REVIEWER_BASH_DENIED PASS',
        'REVIEWER_TASK_DENIED PASS',
        'REVIEWER_RUNTIME_ARTIFACT_WRITE_DENIED PASS',
        'REVIEWER_SUBMIT_TOOL_ALLOWED PASS'
    )

    Invoke-Gate 'closureInstallSafety' {
        Invoke-Script '.github/scripts/task164-closure-install-e2e.ps1'
    } @(
        'INSTALL_REVIEWER_HOST_RESOURCES PASS',
        'INSTALL_EXISTING_OPENCODE_CONFLICT_FAIL_CLOSED PASS'
    )

    Invoke-Gate 'closureAgentContract' {
        Invoke-Script '.github/scripts/task164-closure-agent-contract.ps1'
    } @('HARNESS_164_AGENT_CONTRACT_CONSISTENT PASS')

    if (Test-Path '.code-harness/bin/ast-grep.exe') {
        $env:CODEA_AST_GREP_TEST_PATH = (Resolve-Path '.code-harness/bin/ast-grep.exe').Path
    }

    Invoke-Gate 'task164MigrationE2E' {
        Build-RevokedRCUpgrade
        Invoke-Script '.github/scripts/task164-release-blocker-task1-e2e.ps1'
    } @(
        'CONFIG_MIGRATION_163_TO_164_REGISTERED PASS',
        'CONFIG_MIGRATION_BEFORE_TARGET_SCHEMA_VALIDATION PASS',
        'CONFIG_MIGRATION_TARGET_SCHEMA_VALID PASS',
        'CONFIG_MIGRATION_IDEMPOTENT PASS',
        'CONFIG_USER_VALUES_PRESERVED PASS',
        'CONFIG_UNSUPPORTED_MIGRATION_FAIL_CLOSED PASS',
        'TASK164_CONFIG_PACKAGED_163_TO_164_E2E PASS'
    )

    Invoke-Gate 'task164ReviewerEntryE2E' {
        Invoke-Script '.github/scripts/task164-release-blocker-task2-plain-review-e2e.ps1'
    } @(
        'TASK164_TASK2_TOP_LEVEL_REVIEW_CHAIN PASS',
        'TASK164_TASK2_TOP_LEVEL_DISABLED_HARD_STOP PASS',
        'gate_task2_entry_e2e PASS'
    )

    Invoke-Gate 'task164ReviewerAuthorityE2E' {
        Invoke-Script '.github/scripts/task164-release-blocker-task2-e2e.ps1'
    } @(
        'REVIEWER_INDEPENDENT_INVOCATION PASS',
        'REVIEWER_FINDING_PROPOSAL_HOST_RECEIPT PASS',
        'REVIEWER_RUNTIME_AUTHORITY_SEPARATION PASS',
        'REVIEWER_UNAVAILABLE_FAIL_CLOSED PASS',
        'MAIN_AGENT_REVIEWER_FALLBACK_FORBIDDEN PASS',
        'MAIN_AGENT_FORGED_REVIEWER_RECEIPT_RUNTIME_REJECTED PASS',
        'TASK164_RELEASE_BLOCKER_TASK2_E2E PASS'
    )

    Invoke-Gate 'task164ReviewerCancelE2E' {
        Invoke-Script '.github/scripts/task164-release-blocker-task2-session-cancel-e2e.ps1'
    } @(
        'REVIEWER_SESSION_STARTED_BEFORE_CANCEL PASS',
        'REVIEWER_SESSION_CANCELLED PASS',
        'REVIEWER_SESSION_CANCEL_FAIL_CLOSED PASS',
        'REVIEWER_CHILD_CANCEL_CRASH_FAIL_CLOSED PASS'
    )

    Invoke-Gate 'task164PackagedFullReviewE2E' {
        Invoke-Script '.github/scripts/task164-task4-packaged-plain-review-e2e.ps1'
    } @(
        'OPENCODE_RUNTIME_PROGRESS_RENDERED PASS',
        'OPENCODE_RUNTIME_PROGRESS_1_TO_8 PASS',
        'PROMPT_ONLY_PROGRESS_NOT_AUTHORITY PASS',
        'TASK164_TASK4_PACKAGED_PLAIN_REVIEW_8_OF_8 PASS',
        'TASK164_TASK4_INDEPENDENT_REVIEWER_BOTH_PHASES PASS',
        'TASK164_TASK4_RUNTIME_PROGRESS_TERMINAL PASS',
        'TASK164_TASK4_REVIEW_MD PASS',
        'TASK164_TASK4_GATE_B PASS'
    )

    Invoke-Gate 'task164ProgressInterruptionE2E' {
        Invoke-Script '.github/scripts/task164-task4-progress-interruption-e2e.ps1'
    } @(
        'OPENCODE_INTERRUPTION_STAGE_VISIBLE PASS',
        'OPENCODE_INTERRUPTION_LATER_STAGES_BLOCKED PASS',
        'TASK164_TASK4_INTERRUPTION_CHANGE_ANALYSIS PASS',
        'TASK164_TASK4_DOWNSTREAM_BLOCKED PASS',
        'TASK164_TASK4_GATE_D PASS'
    )

    Invoke-Gate 'fullGoRegression' {
        Invoke-Go @('test','-count=1','./...')
        Write-Output 'TASK164_FINAL_GO_TEST PASS'
    } @('TASK164_FINAL_GO_TEST PASS')

    Invoke-Gate 'goVet' {
        Invoke-Go @('vet','./...')
        Write-Output 'TASK164_FINAL_GO_VET PASS'
    } @('TASK164_FINAL_GO_VET PASS')

    Invoke-Gate 'task164Task3ProgressState' {
        Invoke-Go @('test','-count=1','-v','./cmd/codea-dcep-tools','-run','^Test164Task3')
        Invoke-Go @('test','-count=1','-v','./internal/reviewprogress','-run','^Test164Task3')
        Write-Output 'REVIEW_STAGE_ORDER_RUNTIME_OWNED PASS'
        Write-Output 'REVIEW_STAGE_TRANSITION_FAIL_CLOSED PASS'
        Write-Output 'REVIEW_FRESH_RUN_STATE PASS'
        Write-Output 'REVIEW_STAGE_FAILURE_ATTRIBUTION PASS'
        Write-Output 'OPENCODE_PROGRESS_FROM_RUNTIME_EVENTS PASS'
        Write-Output 'PROMPT_ONLY_PROGRESS_NOT_AUTHORITY PASS'
    } @(
        'REVIEW_STAGE_ORDER_RUNTIME_OWNED PASS',
        'REVIEW_STAGE_TRANSITION_FAIL_CLOSED PASS',
        'REVIEW_FRESH_RUN_STATE PASS',
        'REVIEW_STAGE_FAILURE_ATTRIBUTION PASS',
        'OPENCODE_PROGRESS_FROM_RUNTIME_EVENTS PASS',
        'PROMPT_ONLY_PROGRESS_NOT_AUTHORITY PASS'
    )

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
        Write-Output 'TASK164_FINAL_AUTHORITY_SUCCESS_USER_SELECTION PASS'
    } @('TASK163_TASK3_ACTIVE_CONTRACT_HARD_STOP PASS','TASK164_FINAL_AUTHORITY_SUCCESS_USER_SELECTION PASS')

    Invoke-Gate 'retained163UpgradeV2' {
        Invoke-Go @('test','-count=1','-v','./internal/upgrade')
        Write-Output 'TASK164_FINAL_RETAINED_163_UPGRADE PASS'
    } @('TASK164_FINAL_RETAINED_163_UPGRADE PASS')

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
