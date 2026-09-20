$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$packageScript = Join-Path $repoRoot '.github/scripts/task162-task2-package.ps1'
$installZip = Join-Path $repoRoot 'codea-harness-1.6.2-windows-x64-install.zip'
$upgradeZip = Join-Path $repoRoot 'codea-harness-1.6.2-windows-x64-upgrade.zip'
$accepted161 = '87ed05c5bbc56f4fdf904dfbb239d9125b8136e0'

function Write-Utf8NoBom([string]$Path, [string]$Content) {
    $parent = Split-Path -Parent $Path
    if ($parent) { New-Item -ItemType Directory -Force $parent | Out-Null }
    [IO.File]::WriteAllText($Path, $Content, [Text.UTF8Encoding]::new($false))
}

function Invoke-Git([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments) {
    & git @Arguments
    if ($LASTEXITCODE -ne 0) { throw "git $($Arguments -join ' ') failed with exit code $LASTEXITCODE" }
}

function Invoke-Runtime([string]$Runtime, [Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments) {
    $text = (& $Runtime @Arguments 2>&1 | Out-String)
    if ($LASTEXITCODE -ne 0) { throw "Runtime $($Arguments -join ' ') failed:`n$text" }
    return $text.Trim()
}

function Assert-NoRuntimeSource([string]$Root, [string]$Label) {
    if (Test-Path (Join-Path $Root 'tools-runtime')) {
        throw "$Label contains forbidden tools-runtime/"
    }
    $forbidden = @(Get-ChildItem -Path $Root -Recurse -File | Where-Object {
        $_.Extension -eq '.go' -or $_.Name -eq 'go.mod' -or $_.Name -eq 'go.sum'
    })
    if ($forbidden.Count -gt 0) {
        throw "$Label contains Go Runtime source: $($forbidden.FullName -join ', ')"
    }
}

function Assert-Package([string]$Zip, [string]$TopDir, [string]$Label) {
    if (-not (Test-Path $Zip -PathType Leaf)) { throw "$Label ZIP missing: $Zip" }
    $extract = Join-Path $env:RUNNER_TEMP ("task162-package-check-" + [guid]::NewGuid().ToString('N'))
    Expand-Archive -Path $Zip -DestinationPath $extract -Force
    $root = Join-Path $extract $TopDir
    if (-not (Test-Path $root -PathType Container)) { throw "$Label missing top-level $TopDir" }
    foreach ($required in @('VERSION','RELEASE-MANIFEST.json','bin/codea-dcep-tools.exe','bin/ast-grep.exe','AGENTS.md','bootstrap.md','upgrade.md','agents','skills','contracts','tools')) {
        if (-not (Test-Path (Join-Path $root $required))) { throw "$Label missing required $required" }
    }
    Assert-NoRuntimeSource $root $Label
    $manifest = Get-Content (Join-Path $root 'RELEASE-MANIFEST.json') -Raw | ConvertFrom-Json
    if ([string]$manifest.version -ne '1.6.2') { throw "$Label manifest version mismatch" }
    if ([string]$manifest.runtime -ne 'codea-dcep-tools.exe') { throw "$Label manifest runtime mismatch" }
    if ([string]$manifest.astGrepVersion -ne '0.42.1') { throw "$Label ast-grep version mismatch" }
    return $root
}

function New-ValidHarness([string]$Path) {
    Write-Utf8NoBom $Path @'
version: 2
project:
  type: maven
  root: .
  module: ""
review:
  baseRef: HEAD
  includeWorkingTree: true
integrationTest:
  executable: mvn
  args: [test]
  reportDir: target/surefire-reports
  timeoutSeconds: 600
service:
  executable: mvn
  args: [spring-boot:run]
  startupTimeoutSeconds: 120
  readiness:
    type: log
    pattern: Started
  logFile: null
stopService:
  mode: processTree
initialization:
  status: READY
  unresolved: []
scope:
  sourceIncludes: [src/main/java/**]
  testIncludes: [src/test/java/**]
  mapperIncludes: [src/main/resources/**/*Mapper.xml]
  configIncludes: [src/main/resources/**/*.yml]
write:
  allowedTestPaths: [src/test/**]
  allowedProductionPaths: [src/main/**]
  deniedPaths: []
runs:
  directory: .code-harness/runs
'@
}

# RED precondition: reproduce the 1.6.1 packaging defect using the exact old staging rule.
$legacyStage = Join-Path $env:RUNNER_TEMP ("task162-legacy-stage-" + [guid]::NewGuid().ToString('N'))
Copy-Item -Recurse (Join-Path $repoRoot '.code-harness') (Join-Path $legacyStage '.code-harness')
foreach ($state in @('harness.yaml','project.md','database.yaml','runs','chains')) {
    Remove-Item -Recurse -Force (Join-Path $legacyStage ".code-harness/$state") -ErrorAction SilentlyContinue
}
if (-not (Test-Path (Join-Path $legacyStage '.code-harness/tools-runtime'))) {
    throw 'Task 2 RED precondition invalid: legacy 1.6.1 staging no longer contains tools-runtime'
}
Write-Output 'TASK162_TASK2_LEGACY_PACKAGE_SOURCE_LEAK_REPRODUCED'

if (-not (Test-Path $packageScript -PathType Leaf)) {
    throw 'Task 2 packaging implementation missing: .github/scripts/task162-task2-package.ps1'
}

Push-Location $repoRoot
try {
    & $packageScript
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    $installRoot = Assert-Package $installZip '.code-harness' 'install package'
    $upgradePackageRoot = Assert-Package $upgradeZip '.code-harness-upgrade' 'upgrade package'
    Write-Output 'TASK162_TASK2_ARTIFACT_CLEAN PASS'

    # New install smoke in a clean directory. The Runtime phase deliberately runs with Go removed from PATH.
    $installProject = Join-Path $env:RUNNER_TEMP ("task162-install-project-" + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Force $installProject | Out-Null
    Copy-Item -Recurse $installRoot (Join-Path $installProject '.code-harness')
    New-Item -ItemType Directory -Force (Join-Path $installProject 'src/main/java/acme') | Out-Null
    Write-Utf8NoBom (Join-Path $installProject 'src/main/java/acme/InstallController.java') @'
package acme;
public class InstallController {
    public String create() { return "base"; }
}
'@
    Push-Location $installProject
    try {
        Invoke-Git init
        Invoke-Git config user.email 'task162@example.test'
        Invoke-Git config user.name 'Task 162 Package Cleanup'
        Invoke-Git config core.autocrlf false
        Invoke-Git add .
        Invoke-Git commit -m 'base install smoke'
        Write-Utf8NoBom (Join-Path $installProject 'src/main/java/acme/InstallController.java') @'
package acme;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RestController;
@RestController
public class InstallController {
    @PostMapping("/install")
    public String create() { return "changed"; }
}
'@
        New-ValidHarness (Join-Path $installProject '.code-harness/harness.yaml')
        $runID = 'task162-install-smoke'
        $requestDir = Join-Path $installProject ".code-harness/runs/$runID/requests"
        New-Item -ItemType Directory -Force $requestDir | Out-Null
        Write-Utf8NoBom (Join-Path $requestDir 'inventory.json') (([ordered]@{
            runId=$runID; baseRef='HEAD'; includeWorkingTree=$true; intent=[ordered]@{mode='FULL'}
        }) | ConvertTo-Json -Depth 10 -Compress)

        $savedPath = $env:PATH
        try {
            $env:PATH = (($savedPath -split ';') | Where-Object {
                $_ -and $_ -notmatch '(?i)hostedtoolcache[\\/]windows[\\/]go' -and $_ -notmatch '(?i)program files[\\/]go([\\/]|$)'
            }) -join ';'
            if (Get-Command go -ErrorAction SilentlyContinue) { throw 'Go is still available during no-Go install smoke' }

            $runtime = (Resolve-Path '.code-harness/bin/codea-dcep-tools.exe').Path
            $ast = (Resolve-Path '.code-harness/bin/ast-grep.exe').Path
            $usage = (& $runtime 2>&1 | Out-String)
            if ($LASTEXITCODE -eq 0 -or $usage -notmatch 'usage:') { throw 'installed Runtime usage smoke failed' }
            $astVersion = (& $ast --version 2>&1 | Out-String).Trim()
            if ($LASTEXITCODE -ne 0 -or $astVersion -notmatch '0\.42\.1') { throw "installed ast-grep smoke failed: $astVersion" }
            Invoke-Runtime $runtime validate --schema '.code-harness/contracts/harness-config.schema.json' --input '.code-harness/harness.yaml' --format yaml | Out-Null
            Invoke-Runtime $runtime analysis inventory --input ".code-harness/runs/$runID/requests/inventory.json" | Out-Null

            $draft = [ordered]@{
                reviewScope=[ordered]@{currentBranch=(git branch --show-current).Trim(); baseRef='HEAD'; baseCommit=(git rev-parse HEAD).Trim(); mergeBase=(git rev-parse HEAD).Trim(); headCommit=(git rev-parse HEAD).Trim(); includeWorkingTree=$true}
                changedFiles=@([ordered]@{path='src/main/java/acme/InstallController.java'; role='Controller'; sources=@('UNSTAGED')})
                affectedControllers=@([ordered]@{controller='InstallController'; endpoints=@('InstallController.create'); impactType='DIRECT_CHANGE'; sourceSymbols=@('InstallController.create')})
                callChains=@([ordered]@{entryPoint='InstallController.create'; chain=@('InstallController.create')})
                symbolLocations=@([ordered]@{symbol='InstallController.create'; path='src/main/java/acme/InstallController.java'; role='Controller'; source='FIND_SYMBOL'})
                resourceRelations=@(); externalDependencies=@(); riskAreas=@()
                reviewCoverage=[ordered]@{status='COMPLETE'; reviewedFiles=@([ordered]@{path='src/main/java/acme/InstallController.java'; role='Controller'; reason='CHANGED'}); unresolvedSymbols=@()}
            }
            Write-Utf8NoBom (Join-Path $requestDir 'change-analysis-draft.json') ($draft | ConvertTo-Json -Depth 30)
            Write-Utf8NoBom (Join-Path $requestDir 'certify.json') (([ordered]@{
                runId=$runID; draftPath=".code-harness/runs/$runID/requests/change-analysis-draft.json"; baseRef='HEAD'; includeWorkingTree=$true; intent=[ordered]@{mode='FULL'}
            }) | ConvertTo-Json -Depth 10 -Compress)
            Invoke-Runtime $runtime analysis certify --input ".code-harness/runs/$runID/requests/certify.json" | Out-Null
            Invoke-Runtime $runtime review units --run-id $runID | Out-Null
            $units = Get-Content ".code-harness/runs/$runID/analysis/review-units.json" -Raw | ConvertFrom-Json
            if (@($units.units).Count -lt 1) { throw 'installed package basic review smoke produced no ReviewUnit' }
        } finally {
            $env:PATH = $savedPath
        }
    } finally { Pop-Location }
    Assert-NoRuntimeSource (Join-Path $installProject '.code-harness') 'installed 1.6.2'
    Write-Output 'TASK162_TASK2_NEW_INSTALL_NO_GO_ANALYSIS_REVIEW PASS'

    # Real accepted 1.6.1 -> 1.6.2 upgrade. Accepted source tree intentionally contains tools-runtime.
    $upgradeProject = Join-Path $env:RUNNER_TEMP ("task162-upgrade-project-" + [guid]::NewGuid().ToString('N'))
    $baselineZip = Join-Path $env:RUNNER_TEMP ("task162-baseline-161-" + [guid]::NewGuid().ToString('N') + '.zip')
    New-Item -ItemType Directory -Force $upgradeProject | Out-Null
    git cat-file -e "$accepted161^{commit}"
    if ($LASTEXITCODE -ne 0) { throw "accepted 1.6.1 baseline unavailable: $accepted161" }
    git archive --format=zip --output $baselineZip $accepted161 .code-harness
    if ($LASTEXITCODE -ne 0) { throw 'failed to archive accepted 1.6.1 baseline' }
    Expand-Archive -Path $baselineZip -DestinationPath $upgradeProject -Force
    $target = Join-Path $upgradeProject '.code-harness'
    if ((Get-Content (Join-Path $target 'VERSION') -Raw).Trim() -ne '1.6.1') { throw 'accepted baseline is not VERSION=1.6.1' }
    if (-not (Test-Path (Join-Path $target 'tools-runtime'))) { throw 'accepted 1.6.1 fixture does not contain tools-runtime precondition' }

    New-ValidHarness (Join-Path $target 'harness.yaml')
    Write-Utf8NoBom (Join-Path $target 'project.md') "project-state-161`r`n"
    Write-Utf8NoBom (Join-Path $target 'database.yaml') "version: 1`r`nenvironment: TEST`r`npassword: sentinel-161`r`n"
    New-Item -ItemType Directory -Force (Join-Path $target 'runs/keep-run'), (Join-Path $target 'chains') | Out-Null
    [IO.File]::WriteAllBytes((Join-Path $target 'runs/keep.bin'), [byte[]](1,6,1,2))
    Write-Utf8NoBom (Join-Path $target 'runs/keep-run/evidence.json') '{"keep":"run-161"}'
    Write-Utf8NoBom (Join-Path $target 'chains/keep.yaml') "version: 1`r`nid: keep-161`r`nstatus: ACCEPTED`r`n"
    $stateHashes = @{
        harness=(Get-FileHash -Algorithm SHA256 (Join-Path $target 'harness.yaml')).Hash
        project=(Get-FileHash -Algorithm SHA256 (Join-Path $target 'project.md')).Hash
        database=(Get-FileHash -Algorithm SHA256 (Join-Path $target 'database.yaml')).Hash
        run=(Get-FileHash -Algorithm SHA256 (Join-Path $target 'runs/keep.bin')).Hash
        evidence=(Get-FileHash -Algorithm SHA256 (Join-Path $target 'runs/keep-run/evidence.json')).Hash
        chain=(Get-FileHash -Algorithm SHA256 (Join-Path $target 'chains/keep.yaml')).Hash
    }

    Copy-Item -Recurse $upgradePackageRoot (Join-Path $upgradeProject '.code-harness-upgrade')
    Push-Location $upgradeProject
    try {
        $savedPath = $env:PATH
        try {
            $env:PATH = (($savedPath -split ';') | Where-Object {
                $_ -and $_ -notmatch '(?i)hostedtoolcache[\\/]windows[\\/]go' -and $_ -notmatch '(?i)program files[\\/]go([\\/]|$)'
            }) -join ';'
            if (Get-Command go -ErrorAction SilentlyContinue) { throw 'Go is still available during no-Go upgrade smoke' }
            $upgradeRuntime = (Resolve-Path '.code-harness-upgrade/bin/codea-dcep-tools.exe').Path
            $upgradeOut = (& $upgradeRuntime upgrade 2>&1 | Out-String)
            if ($LASTEXITCODE -ne 0 -or $upgradeOut -notmatch 'UPGRADED') { throw "1.6.1 -> 1.6.2 upgrade failed: $upgradeOut" }
        } finally { $env:PATH = $savedPath }
    } finally { Pop-Location }

    if ((Get-Content (Join-Path $target 'VERSION') -Raw).Trim() -ne '1.6.2') { throw 'upgrade did not install VERSION=1.6.2' }
    Assert-NoRuntimeSource $target 'upgraded 1.6.2'
    if (Test-Path (Join-Path $upgradeProject '.code-harness-upgrade')) { throw 'consumed upgrade package was not cleaned' }
    foreach ($entry in @(
        @('harness','harness.yaml'), @('project','project.md'), @('database','database.yaml'),
        @('run','runs/keep.bin'), @('evidence','runs/keep-run/evidence.json'), @('chain','chains/keep.yaml')
    )) {
        $actual = (Get-FileHash -Algorithm SHA256 (Join-Path $target $entry[1])).Hash
        if ($actual -ne $stateHashes[$entry[0]]) { throw "Project State changed during upgrade: $($entry[1])" }
    }
    $installedRuntime = Join-Path $target 'bin/codea-dcep-tools.exe'
    $installedUsage = (& $installedRuntime 2>&1 | Out-String)
    if ($LASTEXITCODE -eq 0 -or $installedUsage -notmatch 'usage:') { throw 'upgraded installed Runtime smoke failed' }
    Write-Output 'TASK162_TASK2_REAL_161_TO_162_UPGRADE PASS'
    Write-Output 'TASK162_TASK2_PROJECT_STATE_PRESERVATION PASS'
    Write-Output 'TASK162_TASK2_RELEASE_PACKAGE_CLEANUP_E2E PASS'
} finally {
    Pop-Location
}
