$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$root = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$base = '7b339fa61745a227202a4a81e6c26a8ffc2ca11f'
$expected = $env:GITHUB_SHA
$evidence = Join-Path $root 'task163-final-evidence'
$checklistPath = Join-Path $root 'codea-harness-1.6.3-release-checklist.json'
$utf8 = [Text.UTF8Encoding]::new($false)
New-Item -ItemType Directory -Force $evidence | Out-Null
$results = [ordered]@{}
$artifacts = [ordered]@{}
$head = ''
$version = '1.6.3'

function Write-Checklist([string]$Status) {
    $record = [ordered]@{
        version = $version
        status = $Status
        exactHeadSha = $head
        acceptedBaseline = $base
        workflowRunId = $env:GITHUB_RUN_ID
        generatedAtUtc = [DateTime]::UtcNow.ToString('o')
        gates = $results
        artifacts = $artifacts
    }
    [IO.File]::WriteAllText($checklistPath, ($record | ConvertTo-Json -Depth 15), $utf8)
}
function Invoke-Checked([string]$Executable, [string[]]$Arguments) {
    & $Executable @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$Executable $($Arguments -join ' ') exited $LASTEXITCODE" }
}
function Invoke-Go([string[]]$Arguments) {
    Push-Location (Join-Path $root '.code-harness/tools-runtime')
    try { Invoke-Checked 'go' $Arguments } finally { Pop-Location }
}
function Invoke-Gate([string]$Name, [scriptblock]$Action, [string[]]$Markers = @()) {
    $log = Join-Path $evidence "$Name.log"
    $status = 'FAIL'
    $errorText = $null
    try {
        # Tee-Object does not create a file for a successful silent command.
        # Start a fresh log for every gate, including go vet and failed probes.
        [IO.File]::WriteAllText($log, '', $utf8)
        $global:LASTEXITCODE = 0
        & $Action *>&1 | Tee-Object -FilePath $log
        if ($LASTEXITCODE -ne 0) { throw "Gate process exited $LASTEXITCODE" }
        $text = [IO.File]::ReadAllText($log)
        foreach ($marker in $Markers) {
            if (-not $text.Contains($marker)) { throw "Required evidence missing: $marker" }
        }
        $status = 'PASS'
        Write-Output "TASK163_FINAL_GATE PASS name=$Name"
    } catch {
        $errorText = ($_ | Out-String).Trim()
        Add-Content -Path $log -Value $errorText
        Write-Output "TASK163_FINAL_GATE FAIL name=$Name error=$errorText"
    } finally {
        $global:LASTEXITCODE = 0
        $results[$Name] = [ordered]@{ status=$status; log="task163-final-evidence/$Name.log"; error=$errorText }
        Write-Checklist 'PENDING'
    }
}
function Invoke-Script([string]$Path) {
    Invoke-Checked 'pwsh' @('-NoProfile','-File',(Join-Path $root $Path))
}
function Assert-ReleaseScope {
    $script:head = (git -C $root rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0 -or $head -notmatch '^[0-9a-f]{40}$') { throw 'Cannot resolve exact HEAD' }
    if ($head -ne $expected) { throw "Exact HEAD mismatch: $head != $expected" }
    # Actions checks out an exact SHA in detached HEAD. The trusted workflow
    # event ref identifies the release branch; never infer it from local HEAD.
    $releaseRef = 'refs/heads/release/1.6.3-final-certification'
    if ($env:GITHUB_REF -cne $releaseRef) { throw "Unexpected release ref: $($env:GITHUB_REF)" }
    Invoke-Checked 'git' @('-C',$root,'merge-base','--is-ancestor',$base,$head)
    if ((Get-Content (Join-Path $root '.code-harness/VERSION') -Raw).Trim() -ne $version) { throw 'Release version mismatch' }
    $changed = @(& git -C $root diff --name-only "$base..$head")
    if ($LASTEXITCODE -ne 0) { throw 'Cannot inspect release scope' }
    # Exact legacy test migrations diagnosed by final CI 34072313345.
    # No directory-wide test allowance and no Task 1-4 production changes.
    $retainedTestMigrations = @(
        '.code-harness/tools-runtime/cmd/codea-dcep-tools/chain_discover_bootstrap_151_test.go',
        '.code-harness/tools-runtime/cmd/codea-dcep-tools/task160_release_test.go',
        '.code-harness/tools-runtime/cmd/codea-dcep-tools/workspace_chain_152_test.go'
    )
    foreach ($path in $changed) {
        if ($path -eq '.code-harness/VERSION' -or
            $path -cin $retainedTestMigrations -or
            $path -match '^\.github/scripts/task163-(final-certification|release-package|final-contract-regression)\.ps1$' -or
            $path -eq '.github/workflows/task163-final-certification.yml' -or
            $path -match '^docs/superpowers/(plans|evidence)/2026-09-07-codea-harness-1\.6\.3-final-certification.*\.md$') { continue }
        throw "Unapproved release scope: $path"
    }
    Write-Output "TASK163_FINAL_SCOPE PASS head=$head files=$($changed.Count)"
}
function Assert-ReleaseArtifacts {
    $runtime = Join-Path $root '.code-harness/bin/codea-dcep-tools.exe'
    $runtimeHash = (Get-FileHash $runtime -Algorithm SHA256).Hash.ToLowerInvariant()
    $astHash = (Get-FileHash (Join-Path $root '.code-harness/bin/ast-grep.exe') -Algorithm SHA256).Hash.ToLowerInvariant()
    foreach ($kind in @('install','upgrade')) {
        $zip = Join-Path $root "codea-harness-1.6.3-windows-x64-$kind.zip"
        if (-not (Test-Path $zip -PathType Leaf)) { throw "Missing $kind ZIP" }
        $dest = Join-Path $env:RUNNER_TEMP ('task163-final-archive-' + $kind + '-' + [guid]::NewGuid().ToString('N'))
        try {
            Expand-Archive $zip $dest -Force
            $top = if ($kind -eq 'install') { '.code-harness' } else { '.code-harness-upgrade' }
            $releaseRoot = Join-Path $dest $top
            $manifest = Get-Content (Join-Path $releaseRoot 'RELEASE-MANIFEST.json') -Raw | ConvertFrom-Json
            if ($manifest.version -ne $version -or $manifest.buildCommit -ne $head -or
                $manifest.runtimeSha256 -ne $runtimeHash -or $manifest.astGrepSha256 -ne $astHash -or
                $manifest.astGrepVersion -ne '0.42.1') { throw "$kind manifest identity mismatch" }
            if ((Get-Content (Join-Path $releaseRoot 'VERSION') -Raw).Trim() -ne $version) { throw "$kind VERSION mismatch" }
            $files = @(Get-ChildItem $releaseRoot -Recurse -Force -File | Where-Object { $_.FullName -ne (Join-Path $releaseRoot 'RELEASE-MANIFEST.json') })
            if (@($manifest.managedFiles.PSObject.Properties).Count -ne $files.Count) { throw "$kind inventory count mismatch" }
            foreach ($file in $files) {
                $rel = [IO.Path]::GetRelativePath($releaseRoot,$file.FullName).Replace('\','/')
                $hash = (Get-FileHash $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
                if ($manifest.managedFiles.$rel -ne $hash) { throw "$kind inventory mismatch: $rel" }
            }
            if ((Get-FileHash (Join-Path $releaseRoot 'bin/codea-dcep-tools.exe') -Algorithm SHA256).Hash.ToLowerInvariant() -ne $runtimeHash) { throw "$kind Runtime mismatch" }
            $artifacts[$kind] = [ordered]@{ file=(Split-Path $zip -Leaf); sha256=(Get-FileHash $zip -Algorithm SHA256).Hash.ToLowerInvariant(); size=(Get-Item $zip).Length }
        } finally { Remove-Item $dest -Recurse -Force -ErrorAction SilentlyContinue }
    }
    $signature = Get-AuthenticodeSignature $runtime
    $signatureStatus = if ($null -ne $signature.SignerCertificate) { [string]$signature.Status } else { 'Unsigned' }
    $goVersion = (go env GOVERSION).Trim()
    if ($LASTEXITCODE -ne 0) { throw 'Cannot determine Go version' }
    $whitelist = Join-Path $root 'codea-dcep-tools-whitelist.txt'
    $lines = @('Product:','Codea Harness','Version:',$version,'Binary:','codea-dcep-tools.exe','Runtime SHA256:',$runtimeHash,'File Size:',[string](Get-Item $runtime).Length,'Build Commit:',$head,'GOOS:','windows','GOARCH:','amd64','Go Version:',$goVersion,'Signature Status:',$signatureStatus)
    if ($null -ne $signature.SignerCertificate) { $lines += @('Publisher:',[string]$signature.SignerCertificate.Subject) }
    [IO.File]::WriteAllLines($whitelist,$lines,$utf8)
    $artifacts['runtime'] = [ordered]@{ binary='codea-dcep-tools.exe'; sha256=$runtimeHash; size=(Get-Item $runtime).Length; signatureStatus=$signatureStatus }
    $artifacts['whitelist'] = [ordered]@{ file=(Split-Path $whitelist -Leaf); sha256=(Get-FileHash $whitelist -Algorithm SHA256).Hash.ToLowerInvariant() }
    Write-Output "TASK163_FINAL_ARTIFACTS PASS head=$head runtimeSha256=$runtimeHash"
}

Push-Location $root
try {
    Invoke-Gate 'exactHeadAndScope' { Assert-ReleaseScope } @('TASK163_FINAL_SCOPE PASS')
    if ($results.exactHeadAndScope.status -ne 'PASS') { Write-Checklist 'BLOCKED'; exit 1 }

    Invoke-Gate 'pinnedOpenCode' {
        Invoke-Checked 'npm' @('install','-g','opencode-ai@1.18.25')
        $out = (& opencode --version 2>&1 | Out-String).Trim()
        if ($LASTEXITCODE -ne 0 -or $out -notmatch '1\.18\.25') { throw "OpenCode version mismatch: $out" }
        Write-Output "TASK163_FINAL_OPENCODE PASS version=$out"
    } @('TASK163_FINAL_OPENCODE PASS')
    Invoke-Gate 'packageBuild' { Invoke-Script '.github/scripts/task163-release-package.ps1' } @('TASK163_RELEASE_PACKAGE_BUILD PASS version=1.6.3')
    if (Test-Path '.code-harness/bin/ast-grep.exe') { $env:CODEA_AST_GREP_TEST_PATH = (Resolve-Path '.code-harness/bin/ast-grep.exe').Path }

    Invoke-Gate 'fullGoRegression' { Invoke-Go @('test','-count=1','./...') }
    Invoke-Gate 'goVet' { Invoke-Go @('vet','./...') }
    Invoke-Gate 'task1LargeAST' {
        Invoke-Go @('test','-count=1','-v','./internal/nav','-run','Test163WorkspaceAST|TestWorkspace|Test152Nested')
        Write-Output 'TASK163_FINAL_TASK1_LARGE_AST PASS'
    } @('TASK163_FINAL_TASK1_LARGE_AST PASS')
    Invoke-Gate 'task2CleanDiscovery' {
        Invoke-Go @('test','-count=1','-v','./internal/chain','-run','^Test163Task2')
        Invoke-Go @('test','-count=1','-v','./cmd/codea-dcep-tools','-run','^Test163Task2|TestChainDiscover')
        Write-Output 'TASK163_FINAL_TASK2_DISCOVERY PASS'
    } @('TASK163_FINAL_TASK2_DISCOVERY PASS')
    Invoke-Gate 'task3ActiveContract' { Invoke-Script '.github/scripts/task163-task3-active-contract-regression.ps1' } @('TASK163_TASK3_ACTIVE_CONTRACT_HARD_STOP PASS')
    Invoke-Gate 'task3NegativeControl' { Invoke-Script '.github/scripts/task163-task3-negative-control.ps1' } @('TASK163_TASK3_NEGATIVE_CONTROL_RED PASS')
    Invoke-Gate 'task3RealSameSession' { Invoke-Script '.github/scripts/task163-task3-real-multi-chain-same-session-e2e.ps1' } @('TASK163_TASK3_REAL_OPENCODE_SAME_SESSION_E2E PASS')
    Invoke-Gate 'task4DeltaRollback' {
        Invoke-Go @('test','-count=1','-v','./internal/upgrade')
        Write-Output 'TASK163_FINAL_TASK4_UPGRADE PASS'
    } @('TASK163_FINAL_TASK4_UPGRADE PASS')
    Invoke-Gate 'task4PackageAndInstalledUpgrade' { Invoke-Script '.github/scripts/task163-task4-package-regression.ps1' } @('TASK163_TASK4_PACKAGE PASS kind=install','TASK163_TASK4_PACKAGE PASS kind=upgrade','TASK163_TASK4_INSTALLED_MANIFEST PASS')
    Invoke-Gate 'retained162Task1RealAgent' { Invoke-Script '.github/scripts/task162-review-reliability-task1-real-agent-e2e-v2.ps1' }
    Invoke-Gate 'retained162Task2SameSession' { Invoke-Script '.github/scripts/task162-review-reliability-task2-real-agent-e2e.ps1' }
    Invoke-Gate 'retained162RealPlainReview' { Invoke-Script '.github/scripts/task162-hotfix-task3-real-plain-review-e2e.ps1' }
    Invoke-Gate 'retained162AuthorityContracts' {
        Invoke-Script '.github/scripts/task162-review-reliability-task1-contract-regression.ps1'
        Invoke-Script '.github/scripts/task162-review-reliability-task2-contract-regression.ps1'
        Invoke-Script '.github/scripts/task162-hotfix-task1-agent-authority-regression.ps1'
        Invoke-Script '.github/scripts/task162-final-task2-invocation-contract-regression.ps1'
        Invoke-Go @('test','-count=1','./internal/reviewselection','./internal/reviewscope','./internal/reviewunit','./internal/reviewrules')
        Write-Output 'TASK163_FINAL_RETAINED_AUTHORITY PASS'
    } @('TASK163_FINAL_RETAINED_AUTHORITY PASS')
    Invoke-Gate 'releaseArtifacts' { Assert-ReleaseArtifacts } @('TASK163_FINAL_ARTIFACTS PASS')
    Invoke-Gate 'finalExactHead' {
        if ((git rev-parse HEAD).Trim() -ne $head) { throw 'HEAD changed during certification' }
        $dirty = @(git diff --name-only HEAD)
        if ($LASTEXITCODE -ne 0 -or $dirty.Count -ne 0) { throw "Tracked source changed during certification: $dirty" }
        Write-Output "TASK163_FINAL_EXACT_HEAD PASS head=$head"
    } @('TASK163_FINAL_EXACT_HEAD PASS')

    $failed = @($results.GetEnumerator() | Where-Object { $_.Value.status -ne 'PASS' } | ForEach-Object { $_.Key })
    if ($failed.Count -gt 0) {
        Write-Checklist 'BLOCKED'
        Write-Output "TASK163_FINAL_CERTIFICATION BLOCKED gates=$($failed -join ',')"
        exit 1
    }
    Write-Checklist 'PASS'
    # Emit the same checklist bytes uploaded with the candidate so the final
    # gate and artifact identities can also be audited from the exact-run log.
    Write-Output ([IO.File]::ReadAllText($checklistPath))
    Write-Output "TASK163_FINAL_CERTIFICATION PASS exactHead=$head"
    exit 0
} finally {
    Pop-Location
}
