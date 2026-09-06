$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

# Inspect actual archives produced by the retained release builder, not its text.
# This is a Task 4 regression gate; it does not certify or publish a release.
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
Push-Location $repoRoot
try {
    $head = (git rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0) { throw 'cannot resolve package HEAD' }
    $version = (Get-Content '.code-harness/VERSION' -Raw).Trim()
    foreach ($kind in @('install', 'upgrade')) {
        $zip = Join-Path $repoRoot "codea-harness-$version-windows-x64-$kind.zip"
        $extract = Join-Path $env:RUNNER_TEMP ("task163-task4-package-$kind-" + [guid]::NewGuid().ToString('N'))
        Expand-Archive -Path $zip -DestinationPath $extract -Force
        try {
            $top = if ($kind -eq 'install') { '.code-harness' } else { '.code-harness-upgrade' }
            $root = Join-Path $extract $top
            if (-not (Test-Path $root -PathType Container)) { throw "$kind package has no $top" }
            if (Test-Path (Join-Path $root 'tools-runtime')) { throw "$kind package contains tools-runtime" }
            $forbidden = @(Get-ChildItem $root -Recurse -Force -File | Where-Object {
                $_.Extension -eq '.go' -or $_.Name -in @('go.mod', 'go.sum')
            })
            if ($forbidden.Count -ne 0) { throw "$kind package contains Go source: $($forbidden.FullName -join ', ')" }
            foreach ($state in @('harness.yaml','project.md','database.yaml','chains')) {
                if (Test-Path (Join-Path $root $state)) { throw "$kind package contains Project State $state" }
            }
            $runEntries = @(Get-ChildItem (Join-Path $root 'runs') -Force)
            if ($runEntries.Count -ne 1 -or $runEntries[0].Name -ne 'README.md' -or $runEntries[0].PSIsContainer) {
                throw "$kind package must contain only runs/README.md"
            }
            $readmeHash = (Get-FileHash (Join-Path $root 'runs/README.md') -Algorithm SHA256).Hash
            if ($readmeHash -ne (Get-FileHash '.code-harness/runs/README.md' -Algorithm SHA256).Hash) {
                throw "$kind package managed README bytes mismatch"
            }
            $manifest = Get-Content (Join-Path $root 'RELEASE-MANIFEST.json') -Raw | ConvertFrom-Json
            if ($manifest.buildCommit -ne $head -or $manifest.version -ne $version) { throw "$kind manifest HEAD/version mismatch" }
            if ($manifest.platform -ne 'windows' -or $manifest.arch -ne 'x64' -or $manifest.runtime -ne 'codea-dcep-tools.exe') {
                throw "$kind manifest platform/runtime mismatch"
            }
            $runtimeHash = (Get-FileHash (Join-Path $root 'bin/codea-dcep-tools.exe') -Algorithm SHA256).Hash.ToLowerInvariant()
            $astHash = (Get-FileHash (Join-Path $root 'bin/ast-grep.exe') -Algorithm SHA256).Hash.ToLowerInvariant()
            if ($runtimeHash -ne $manifest.runtimeSha256 -or $astHash -ne $manifest.astGrepSha256 -or $manifest.astGrepVersion -ne '0.42.1') {
                throw "$kind manifest binary hash/version mismatch"
            }
            if ($runtimeHash -ne (Get-FileHash '.code-harness/bin/codea-dcep-tools.exe' -Algorithm SHA256).Hash.ToLowerInvariant()) {
                throw "$kind package Runtime differs from current built Runtime"
            }
            Write-Output "TASK163_TASK4_PACKAGE PASS kind=$kind head=$head runtimeSha256=$runtimeHash"
        } finally {
            Remove-Item -Recurse -Force $extract
        }
    }
} finally { Pop-Location }
