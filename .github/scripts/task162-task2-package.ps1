$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
Push-Location $repoRoot
try {
    $version = (Get-Content '.code-harness/VERSION' -Raw).Trim()
    if ($version -ne '1.6.2') { throw "unexpected Task 2 package version: $version" }

    $exactHead = (git rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0 -or $exactHead -notmatch '^[0-9a-f]{40}$') { throw 'cannot resolve exact HEAD' }

    $runtime = '.code-harness/bin/codea-dcep-tools.exe'
    $ast = '.code-harness/bin/ast-grep.exe'
    $runReadmeSource = Join-Path $repoRoot '.code-harness/runs/README.md'
    $installZip = 'codea-harness-1.6.2-windows-x64-install.zip'
    $upgradeZip = 'codea-harness-1.6.2-windows-x64-upgrade.zip'
    if (-not (Test-Path $runReadmeSource -PathType Leaf)) { throw 'Review Run README missing from source Harness' }

    Write-Host 'TASK162 Task 2: build Windows x64 Runtime'
    Push-Location '.code-harness/tools-runtime'
    try {
        $env:GOOS = 'windows'
        $env:GOARCH = 'amd64'
        go build -trimpath -ldflags '-s -w' -o ../bin/codea-dcep-tools.exe ./cmd/codea-dcep-tools
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    } finally {
        Pop-Location
        Remove-Item Env:GOOS -ErrorAction SilentlyContinue
        Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
    }
    if (-not (Test-Path $runtime -PathType Leaf)) { throw 'codea-dcep-tools.exe build missing' }

    Write-Host 'TASK162 Task 2: vendor pinned ast-grep Windows x64'
    $astVersion = '0.42.1'
    $astExpected = 'fe34f631bb24c08ad146f92ca2a92971a53d179461b509fd8d32dc863bff9f83'
    $astZip = Join-Path $env:RUNNER_TEMP 'ast-grep-162-task2.zip'
    $astDir = Join-Path $env:RUNNER_TEMP 'ast-grep-162-task2'
    Remove-Item -Recurse -Force $astDir -ErrorAction SilentlyContinue
    Invoke-WebRequest -Uri "https://github.com/ast-grep/ast-grep/releases/download/$astVersion/app-x86_64-pc-windows-msvc.zip" -OutFile $astZip
    $astActual = (Get-FileHash -Algorithm SHA256 $astZip).Hash.ToLowerInvariant()
    if ($astActual -ne $astExpected) { throw "ast-grep checksum mismatch: $astActual" }
    Expand-Archive -Path $astZip -DestinationPath $astDir -Force
    Copy-Item (Join-Path $astDir 'ast-grep.exe') $ast -Force
    $astVersionOut = (& $ast --version 2>&1 | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $astVersionOut -notmatch '0\.42\.1') { throw "ast-grep smoke failed: $astVersionOut" }

    $runtimeHash = (Get-FileHash -Algorithm SHA256 $runtime).Hash.ToLowerInvariant()
    $astHash = (Get-FileHash -Algorithm SHA256 $ast).Hash.ToLowerInvariant()

    $installStage = Join-Path $env:RUNNER_TEMP 'codea-release-install-162-task2'
    $upgradeStage = Join-Path $env:RUNNER_TEMP 'codea-release-upgrade-162-task2'
    Remove-Item -Recurse -Force $installStage,$upgradeStage -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Force $installStage,$upgradeStage | Out-Null
    Copy-Item -Recurse '.code-harness' (Join-Path $installStage '.code-harness')
    Copy-Item -Recurse '.code-harness' (Join-Path $upgradeStage '.code-harness-upgrade')

    function Assert-CleanReleaseTree([string]$Root, [string]$Label) {
        if (Test-Path (Join-Path $Root 'tools-runtime')) { throw "$Label contains forbidden tools-runtime/" }
        $goSource = @(Get-ChildItem -Path $Root -Recurse -File | Where-Object {
            $_.Extension -eq '.go' -or $_.Name -eq 'go.mod' -or $_.Name -eq 'go.sum'
        })
        if ($goSource.Count -gt 0) { throw "$Label contains Go Runtime source: $($goSource.FullName -join ', ')" }
        foreach ($required in @(
            'VERSION','AGENTS.md','bootstrap.md','upgrade.md','harness.template.yaml','project.template.md',
            'agents','skills','contracts','tools','bin/codea-dcep-tools.exe','bin/ast-grep.exe','runs/README.md'
        )) {
            if (-not (Test-Path (Join-Path $Root $required))) { throw "$Label missing required $required" }
        }
        foreach ($state in @('harness.yaml','project.md','database.yaml','chains')) {
            if (Test-Path (Join-Path $Root $state)) { throw "$Label contains Project State $state" }
        }
        $runsRoot = Join-Path $Root 'runs'
        $unexpectedRuns = @(Get-ChildItem -Path $runsRoot -Force | Where-Object { $_.Name -ne 'README.md' })
        if ($unexpectedRuns.Count -gt 0) { throw "$Label contains forbidden Run state: $($unexpectedRuns.FullName -join ', ')" }
    }

    foreach ($releaseRoot in @((Join-Path $installStage '.code-harness'), (Join-Path $upgradeStage '.code-harness-upgrade'))) {
        foreach ($state in @('harness.yaml','project.md','database.yaml','chains')) {
            Remove-Item -Recurse -Force (Join-Path $releaseRoot $state) -ErrorAction SilentlyContinue
        }
        # Review Reliability Task 3: runs/README.md is framework documentation; every other runs/** entry is Project State.
        $releaseRuns = Join-Path $releaseRoot 'runs'
        Remove-Item -Recurse -Force $releaseRuns -ErrorAction SilentlyContinue
        New-Item -ItemType Directory -Force $releaseRuns | Out-Null
        Copy-Item $runReadmeSource (Join-Path $releaseRuns 'README.md') -Force
        # Task 2 product boundary: Go Runtime source is development-only and must never ship.
        Remove-Item -Recurse -Force (Join-Path $releaseRoot 'tools-runtime') -ErrorAction SilentlyContinue
        Remove-Item -Force (Join-Path $releaseRoot 'RELEASE-MANIFEST.json') -ErrorAction SilentlyContinue
    }

    $manifest = [ordered]@{
        version = '1.6.2'
        platform = 'windows'
        arch = 'x64'
        runtime = 'codea-dcep-tools.exe'
        runtimeSha256 = $runtimeHash
        astGrepVersion = $astVersion
        astGrepSha256 = $astHash
        buildCommit = $exactHead
    } | ConvertTo-Json
    [IO.File]::WriteAllText((Join-Path $installStage '.code-harness/RELEASE-MANIFEST.json'), $manifest, [Text.UTF8Encoding]::new($false))
    [IO.File]::WriteAllText((Join-Path $upgradeStage '.code-harness-upgrade/RELEASE-MANIFEST.json'), $manifest, [Text.UTF8Encoding]::new($false))

    Assert-CleanReleaseTree (Join-Path $installStage '.code-harness') 'install release tree'
    Assert-CleanReleaseTree (Join-Path $upgradeStage '.code-harness-upgrade') 'upgrade release tree'

    Remove-Item -Force $installZip,$upgradeZip -ErrorAction SilentlyContinue
    Compress-Archive -Path (Join-Path $installStage '.code-harness') -DestinationPath $installZip -Force
    Compress-Archive -Path (Join-Path $upgradeStage '.code-harness-upgrade') -DestinationPath $upgradeZip -Force
    if (-not (Test-Path $installZip -PathType Leaf)) { throw 'install ZIP was not created' }
    if (-not (Test-Path $upgradeZip -PathType Leaf)) { throw 'upgrade ZIP was not created' }

    Write-Output "TASK162_TASK2_PACKAGE_BUILD PASS head=$exactHead"
    Write-Output 'TASK162_TASK2_PACKAGE_SOURCE_CLEAN PASS'
    Write-Output 'TASK162_REVIEW_RELIABILITY_TASK3_PACKAGE_README PASS'
} finally {
    Pop-Location
}