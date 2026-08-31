$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
Push-Location $repoRoot
try {
    $baseline = 'c07f0a4e029a50de64d271fc4ea83015b06355a1'
    $upgradeZip = 'codea-harness-1.6.1-windows-x64-upgrade.zip'
    $checklistPath = 'codea-harness-1.6.1-release-checklist.json'
    if (!(Test-Path $upgradeZip)) { throw "missing 1.6.1 upgrade package: $upgradeZip" }
    if (!(Test-Path $checklistPath)) { throw "missing release checklist: $checklistPath" }

    $root = Join-Path $env:RUNNER_TEMP 'codea-real-runtime-upgrade-160-to-161'
    $baselineZip = Join-Path $env:RUNNER_TEMP 'codea-real-baseline-160.zip'
    $upgradeExtract = Join-Path $env:RUNNER_TEMP 'codea-real-upgrade-extract-161'
    Remove-Item -Recurse -Force $root,$upgradeExtract -ErrorAction SilentlyContinue
    Remove-Item -Force $baselineZip -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Force $root,$upgradeExtract | Out-Null

    git cat-file -e "$baseline^{commit}"
    if ($LASTEXITCODE -ne 0) { throw "accepted 1.6.0 baseline unavailable: $baseline" }
    git archive --format=zip --output $baselineZip $baseline .code-harness
    if ($LASTEXITCODE -ne 0) { throw 'failed to archive accepted 1.6.0 baseline' }
    Expand-Archive $baselineZip $root -Force
    Expand-Archive $upgradeZip $upgradeExtract -Force

    $target = Join-Path $root '.code-harness'
    $upgradeSource = Join-Path $upgradeExtract '.code-harness-upgrade'
    if ((Get-Content (Join-Path $target 'VERSION') -Raw).Trim() -ne '1.6.0') {
        throw 'accepted baseline is not VERSION=1.6.0'
    }

    Write-Host 'TASK161_REAL_160_UPGRADE: build accepted 1.6.0 Runtime from exact baseline source'
    Push-Location (Join-Path $target 'tools-runtime')
    try {
        $env:GOOS = 'windows'
        $env:GOARCH = 'amd64'
        go build -trimpath -ldflags '-s -w' -o ../bin/codea-harness-tools.exe ./cmd/codea-harness-tools
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    } finally {
        Pop-Location
        Remove-Item Env:GOOS -ErrorAction SilentlyContinue
        Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
    }

    $oldRuntime = Join-Path $target 'bin/codea-harness-tools.exe'
    if (!(Test-Path $oldRuntime)) { throw 'real accepted 1.6.0 Runtime was not built' }
    if (Test-Path (Join-Path $target 'bin/codea-dcep-tools.exe')) {
        throw 'accepted 1.6.0 target unexpectedly contains renamed Runtime'
    }
    $oldRuntimeHash = (Get-FileHash -Algorithm SHA256 $oldRuntime).Hash.ToLowerInvariant()
    if ((Get-Item $oldRuntime).Length -lt 100000) { throw 'accepted 1.6.0 Runtime is not a real compiled executable' }
    $oldUsage = & $oldRuntime 2>&1
    if ($LASTEXITCODE -eq 0 -or (($oldUsage -join "`n") -notmatch 'usage:')) {
        throw "accepted 1.6.0 Runtime execution smoke failed: $($oldUsage -join "`n")"
    }

    Write-Host 'TASK161_REAL_160_UPGRADE: create byte-preserved Project State'
    Copy-Item (Join-Path $target 'harness.template.yaml') (Join-Path $target 'harness.yaml') -Force
    $realHarnessText = Get-Content (Join-Path $target 'harness.yaml') -Raw
    $realHarnessText = $realHarnessText -replace '(?m)^(\s*baseRef:\s*).*$$', '${1}HEAD'
    if ($realHarnessText -notmatch '(?m)^\s*baseRef:\s*HEAD\s*$$') { throw 'failed to initialize real upgrade harness baseRef' }
    [IO.File]::WriteAllText((Join-Path $target 'harness.yaml'), $realHarnessText, [Text.UTF8Encoding]::new($false))
    [IO.File]::WriteAllText((Join-Path $target 'project.md'), "real-project-state-160`r`n", [Text.UTF8Encoding]::new($false))
    [IO.File]::WriteAllText((Join-Path $target 'database.yaml'), "version: 1`r`nenvironment: TEST`r`npassword: real-sentinel-secret`r`n", [Text.UTF8Encoding]::new($false))
    New-Item -ItemType Directory -Force (Join-Path $target 'runs/real-run/evidence'),(Join-Path $target 'chains') | Out-Null
    [IO.File]::WriteAllBytes((Join-Path $target 'runs/real-run/evidence/result.bin'), [byte[]](1,6,0,1,6,1,9,9))
    [IO.File]::WriteAllText((Join-Path $target 'chains/order-approve.yaml'), "# real user chain`r`nversion: 1`r`nid: order-approve`r`nstatus: ACCEPTED`r`n", [Text.UTF8Encoding]::new($false))

    $statePaths = @(
        'harness.yaml',
        'project.md',
        'database.yaml',
        'runs/real-run/evidence/result.bin',
        'chains/order-approve.yaml'
    )
    $before = @{}
    foreach ($rel in $statePaths) {
        $before[$rel] = (Get-FileHash -Algorithm SHA256 (Join-Path $target $rel)).Hash
    }

    Copy-Item -Recurse $upgradeSource (Join-Path $root '.code-harness-upgrade')
    $sourceRuntime = Join-Path $root '.code-harness-upgrade/bin/codea-dcep-tools.exe'
    if (!(Test-Path $sourceRuntime)) { throw '1.6.1 upgrade package missing renamed Runtime' }
    $newRuntimeHash = (Get-FileHash -Algorithm SHA256 $sourceRuntime).Hash.ToLowerInvariant()

    Write-Host 'TASK161_REAL_160_UPGRADE: execute 1.6.0 -> 1.6.1 using packaged renamed Runtime'
    Push-Location $root
    try {
        $upgradeOut = & '.\.code-harness-upgrade\bin\codea-dcep-tools.exe' upgrade 2>&1
        if ($LASTEXITCODE -ne 0) { throw "real 1.6.0 -> 1.6.1 upgrade failed: $($upgradeOut -join "`n")" }
        if (($upgradeOut -join "`n") -notmatch '"status": "UPGRADED"') {
            throw "unexpected real upgrade output: $($upgradeOut -join "`n")"
        }

        if ((Get-Content '.code-harness/VERSION' -Raw).Trim() -ne '1.6.1') { throw 'real upgrade target VERSION is not 1.6.1' }
        if (Test-Path '.code-harness/bin/codea-harness-tools.exe') { throw 'legacy Runtime survived real 1.6.0 -> 1.6.1 upgrade' }
        if (!(Test-Path '.code-harness/bin/codea-dcep-tools.exe')) { throw 'renamed Runtime missing after real upgrade' }
        if ((Get-FileHash -Algorithm SHA256 '.code-harness/bin/codea-dcep-tools.exe').Hash.ToLowerInvariant() -ne $newRuntimeHash) {
            throw 'installed renamed Runtime hash differs from packaged Runtime'
        }
        foreach ($rel in $statePaths) {
            $after = (Get-FileHash -Algorithm SHA256 (Join-Path '.code-harness' $rel)).Hash
            if ($after -ne $before[$rel]) { throw "Project State changed during real upgrade: $rel" }
        }
        if (Test-Path '.code-harness-upgrade') { throw 'consumed upgrade source survived successful real upgrade' }

        $installedUsage = & '.\.code-harness\bin\codea-dcep-tools.exe' 2>&1
        if ($LASTEXITCODE -eq 0 -or (($installedUsage -join "`n") -notmatch 'usage:')) {
            throw "installed 1.6.1 Runtime execution smoke failed: $($installedUsage -join "`n")"
        }
    } finally {
        Pop-Location
    }

    Write-Host 'TASK161_REAL_160_UPGRADE: attach evidence to final release checklist'
    $checklist = Get-Content $checklistPath -Raw | ConvertFrom-Json
    if ($null -eq $checklist.gates) { throw 'release checklist missing gates' }
    $checklist.gates | Add-Member -NotePropertyName real160To161RuntimeUpgrade -NotePropertyValue 'PASS' -Force
    $checklist | Add-Member -NotePropertyName realUpgrade -NotePropertyValue ([pscustomobject]@{
        baseline = $baseline
        oldRuntime = 'codea-harness-tools.exe'
        oldRuntimeSha256 = $oldRuntimeHash
        newRuntime = 'codea-dcep-tools.exe'
        newRuntimeSha256 = $newRuntimeHash
        projectState = 'BYTE_FOR_BYTE_PASS'
    }) -Force
    [IO.File]::WriteAllText($checklistPath, ($checklist | ConvertTo-Json -Depth 10), [Text.UTF8Encoding]::new($false))

    Write-Host "TASK161_REAL_160_TO_161_UPGRADE PASS baseline=$baseline oldRuntimeSha256=$oldRuntimeHash newRuntimeSha256=$newRuntimeHash"
} finally {
    Pop-Location
}
exit 0
