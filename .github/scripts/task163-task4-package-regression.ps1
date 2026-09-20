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
            $inventory = $manifest.managedFiles
            $shipped = @(Get-ChildItem $root -Recurse -Force -File | Where-Object { $_.Name -ne 'RELEASE-MANIFEST.json' })
            if (@($inventory.PSObject.Properties).Count -ne $shipped.Count) { throw "$kind ownership inventory is incomplete" }
            foreach ($file in $shipped) {
                $rel = [IO.Path]::GetRelativePath($root, $file.FullName).Replace('\', '/')
                if ($inventory.$rel -ne (Get-FileHash $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()) {
                    throw "$kind ownership hash mismatch: $rel"
                }
            }
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

    # Exercise installation state, not just ZIP metadata. Use the newly built
    # Runtime outside both trees so the source can be consumed after upgrading.
    $fixture = Join-Path $env:RUNNER_TEMP ('task163-task4-installed-' + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory $fixture | Out-Null
    try {
        Expand-Archive (Join-Path $repoRoot "codea-harness-$version-windows-x64-install.zip") $fixture
        Expand-Archive (Join-Path $repoRoot "codea-harness-$version-windows-x64-upgrade.zip") $fixture
        $target = Join-Path $fixture '.code-harness'
        $utf8 = [Text.UTF8Encoding]::new($false)
        [IO.File]::WriteAllText((Join-Path $target 'VERSION'), '1.6.1', $utf8)
        [IO.File]::WriteAllText((Join-Path $target 'bin/codea-dcep-tools.exe'), 'old installed runtime', $utf8)
        $oldManifest = Get-Content (Join-Path $target 'RELEASE-MANIFEST.json') -Raw | ConvertFrom-Json
        $oldManifest.version = '1.6.1'
        $oldManifest.buildCommit = 'old-installed-build'
        $oldManifest.runtimeSha256 = (Get-FileHash (Join-Path $target 'bin/codea-dcep-tools.exe')).Hash.ToLowerInvariant()
        $oldManifest.managedFiles.'VERSION' = (Get-FileHash (Join-Path $target 'VERSION')).Hash.ToLowerInvariant()
        $oldManifest.managedFiles.'bin/codea-dcep-tools.exe' = $oldManifest.runtimeSha256
        [IO.File]::WriteAllText((Join-Path $target 'RELEASE-MANIFEST.json'), ($oldManifest | ConvertTo-Json -Depth 10), $utf8)
        $config = (Get-Content (Join-Path $target 'harness.template.yaml') -Raw).Replace('baseRef: ""', 'baseRef: HEAD')
        [IO.File]::WriteAllText((Join-Path $target 'harness.yaml'), $config, $utf8)
        [IO.File]::WriteAllText((Join-Path $target 'tools/company-local-notes.txt'), 'keep user notes', $utf8)
        $expectedManifest = Get-Content (Join-Path $fixture '.code-harness-upgrade/RELEASE-MANIFEST.json') -Raw
        Push-Location $fixture
        try {
            $output = @(& (Join-Path $repoRoot '.code-harness/bin/codea-dcep-tools.exe') upgrade 2>&1)
            if ($LASTEXITCODE -ne 0) { throw "installed upgrade failed: $output" }
            $result = ($output | Out-String) | ConvertFrom-Json
        } finally { Pop-Location }
        if ($result.status -ne 'UPGRADED') { throw "unexpected installed upgrade status: $($result.status)" }
        $installedText = Get-Content (Join-Path $target 'RELEASE-MANIFEST.json') -Raw
        $installed = $installedText | ConvertFrom-Json
        $actualRuntime = (Get-FileHash (Join-Path $target 'bin/codea-dcep-tools.exe')).Hash.ToLowerInvariant()
        if ($installedText -cne $expectedManifest -or $installed.runtimeSha256 -ne $actualRuntime -or
            $installed.buildCommit -ne $head -or $installed.version -ne (Get-Content (Join-Path $target 'VERSION') -Raw).Trim()) {
            throw 'installed manifest does not describe installed Runtime/version/build'
        }
        if ((Get-Content (Join-Path $target 'tools/company-local-notes.txt') -Raw) -cne 'keep user notes') { throw 'user file lost' }
        if (Test-Path (Join-Path $fixture '.code-harness-upgrade')) { throw 'successful upgrade did not consume source' }
        Write-Output "TASK163_TASK4_INSTALLED_MANIFEST PASS head=$head runtimeSha256=$actualRuntime"
    } finally { Remove-Item -Recurse -Force $fixture }
} finally { Pop-Location }
