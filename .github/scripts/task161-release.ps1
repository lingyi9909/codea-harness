$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
Push-Location $repoRoot
try {
    $version = (Get-Content '.code-harness/VERSION' -Raw).Trim()
    if ($version -ne '1.6.1') { throw "unexpected release version: $version" }
    $exactHead = (git rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0 -or $exactHead -notmatch '^[0-9a-f]{40}$') { throw 'cannot resolve exact HEAD' }
    $goVersion = (go env GOVERSION).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($goVersion)) { throw 'cannot resolve Go version' }

    $runtime = '.code-harness/bin/codea-dcep-tools.exe'
    $legacyRuntime = '.code-harness/bin/codea-harness-tools.exe'
    $cmdPackage = './cmd/codea-dcep-tools'

    Write-Host 'TASK161: full Go/runtime semantic regression'
    Push-Location '.code-harness/tools-runtime'
    try {
        go test -count=1 ./internal/workspace ./internal/nav ./internal/changeset ./internal/analysis ./internal/reviewselection ./internal/chain ./internal/reviewscope ./internal/coverage ./internal/reviewunit ./internal/reviewrules ./internal/finding ./internal/report ./internal/apply ./internal/schema ./internal/upgrade $cmdPackage
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        go test -count=1 ./...
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        go vet ./...
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

        $env:GOOS = 'windows'
        $env:GOARCH = 'amd64'
        go build -trimpath -ldflags '-s -w' -o ../bin/codea-dcep-tools.exe $cmdPackage
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    } finally {
        Pop-Location
        Remove-Item Env:GOOS -ErrorAction SilentlyContinue
        Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
    }
    if (!(Test-Path $runtime)) { throw 'codea-dcep-tools.exe was not built' }
    if (Test-Path $legacyRuntime) { throw 'production bin contains legacy codea-harness-tools.exe' }

    Write-Host 'TASK161: vendor pinned ast-grep Windows x64'
    $astVersion = '0.42.1'
    $astExpected = 'fe34f631bb24c08ad146f92ca2a92971a53d179461b509fd8d32dc863bff9f83'
    $astZip = Join-Path $env:RUNNER_TEMP 'ast-grep-161.zip'
    $astDir = Join-Path $env:RUNNER_TEMP 'ast-grep-161'
    Remove-Item -Recurse -Force $astDir -ErrorAction SilentlyContinue
    Invoke-WebRequest -Uri "https://github.com/ast-grep/ast-grep/releases/download/$astVersion/app-x86_64-pc-windows-msvc.zip" -OutFile $astZip
    $astActual = (Get-FileHash -Algorithm SHA256 $astZip).Hash.ToLowerInvariant()
    if ($astActual -ne $astExpected) { throw "ast-grep checksum mismatch: $astActual" }
    Expand-Archive -Path $astZip -DestinationPath $astDir -Force
    Copy-Item (Join-Path $astDir 'ast-grep.exe') '.code-harness/bin/ast-grep.exe' -Force
    & '.code-harness/bin/ast-grep.exe' --version
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    Write-Host 'TASK161: renamed Runtime smoke and existing real regressions'
    $runtimeUsage = & $runtime 2>&1
    if ($LASTEXITCODE -eq 0 -or (($runtimeUsage -join "`n") -notmatch 'usage:')) { throw 'renamed runtime executable smoke failed' }
    Remove-Item '.code-harness/runs/.gitkeep' -ErrorAction SilentlyContinue
    ./.github/scripts/task153-task1-real-entrypoint-inventory.ps1
    ./.github/scripts/task152-workspace-smoke.ps1
    ./.github/scripts/task152-task5-real-business-regression.ps1
    ./.github/scripts/task153-real-review-chain-regression.ps1
    ./.github/scripts/task160-real-review-precision-regression.ps1

    # Harness init Runtime boundary: initialize a clean Harness state from templates, then validate it through the formal renamed Runtime.
    $initRoot = Join-Path $env:RUNNER_TEMP ('task161-init-' + [guid]::NewGuid().ToString())
    New-Item -ItemType Directory -Force $initRoot | Out-Null
    Copy-Item -Recurse '.code-harness' (Join-Path $initRoot '.code-harness')
    Push-Location $initRoot
    try {
        $initHarness = @'
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
        $initHarnessPath = Join-Path $initRoot '.code-harness/harness.yaml'
        [IO.File]::WriteAllText($initHarnessPath, $initHarness, [Text.UTF8Encoding]::new($false))
        if (!(Test-Path $initHarnessPath)) { throw 'release harness fixture was not created' }
        $initExe = (Resolve-Path '.code-harness/bin/codea-dcep-tools.exe').Path
        $initOut = & $initExe validate --schema '.code-harness/contracts/harness-config.schema.json' --input '.code-harness/harness.yaml' --format yaml 2>&1
        if ($LASTEXITCODE -ne 0 -or (($initOut -join "`n") -notmatch 'VALID')) { throw "harness init Runtime validation failed: $($initOut -join "`n")" }
    } finally { Pop-Location }

    Write-Host 'TASK161: generate whitelist evidence from formal binary'
    $runtimeHash = (Get-FileHash -Algorithm SHA256 $runtime).Hash.ToLowerInvariant()
    $runtimeSize = (Get-Item $runtime).Length
    $signature = Get-AuthenticodeSignature $runtime
    $whitelistLines = @(
        'Product:',
        'Codea Harness',
        'Version:',
        '1.6.1',
        'Binary:',
        'codea-dcep-tools.exe',
        'SHA256:',
        $runtimeHash,
        'File Size:',
        [string]$runtimeSize,
        'Build Commit:',
        $exactHead,
        'GOOS:',
        'windows',
        'GOARCH:',
        'amd64',
        'Go Version:',
        $goVersion
    )
    if ($null -ne $signature.SignerCertificate) {
        $whitelistLines += @('Signature Status:', [string]$signature.Status, 'Publisher:', [string]$signature.SignerCertificate.Subject)
    } else {
        $whitelistLines += @('Signature Status:', 'Unsigned')
    }
    [IO.File]::WriteAllLines('codea-dcep-tools-whitelist.txt', $whitelistLines, [Text.UTF8Encoding]::new($false))

    Write-Host 'TASK161: stage install/upgrade release trees'
    $installStage = Join-Path $env:RUNNER_TEMP 'codea-release-install-161'
    $upgradeStage = Join-Path $env:RUNNER_TEMP 'codea-release-upgrade-161'
    Remove-Item -Recurse -Force $installStage,$upgradeStage -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Force $installStage,$upgradeStage | Out-Null
    Copy-Item -Recurse '.code-harness' (Join-Path $installStage '.code-harness')
    Copy-Item -Recurse '.code-harness' (Join-Path $upgradeStage '.code-harness-upgrade')
    foreach ($releaseRoot in @((Join-Path $installStage '.code-harness'), (Join-Path $upgradeStage '.code-harness-upgrade'))) {
        foreach ($state in @('harness.yaml','project.md','database.yaml','runs','chains')) {
            Remove-Item -Recurse -Force (Join-Path $releaseRoot $state) -ErrorAction SilentlyContinue
        }
        if (Test-Path (Join-Path $releaseRoot 'bin/codea-harness-tools.exe')) { throw 'release tree contains legacy runtime' }
        if (!(Test-Path (Join-Path $releaseRoot 'bin/codea-dcep-tools.exe'))) { throw 'release tree missing renamed runtime' }
    }
    $astHash = (Get-FileHash -Algorithm SHA256 '.code-harness/bin/ast-grep.exe').Hash.ToLowerInvariant()
    $manifest = [ordered]@{
        version = '1.6.1'
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

    $installZip = 'codea-harness-1.6.1-windows-x64-install.zip'
    $upgradeZip = 'codea-harness-1.6.1-windows-x64-upgrade.zip'
    Compress-Archive -Path (Join-Path $installStage '.code-harness') -DestinationPath $installZip -Force
    Compress-Archive -Path (Join-Path $upgradeStage '.code-harness-upgrade') -DestinationPath $upgradeZip -Force

    Write-Host 'TASK161: real accepted 1.6.0 -> 1.6.1 Windows live upgrade'
    $baseline = 'c07f0a4e029a50de64d271fc4ea83015b06355a1'
    $root = Join-Path $env:RUNNER_TEMP 'codea-real-upgrade-160-to-161'
    $baselineZip = Join-Path $env:RUNNER_TEMP 'codea-baseline-160.zip'
    $upgradeExtract = Join-Path $env:RUNNER_TEMP 'codea-upgrade-extract-161'
    Remove-Item -Recurse -Force $root,$upgradeExtract -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Force $root,$upgradeExtract | Out-Null
    git cat-file -e "$baseline^{commit}"
    if ($LASTEXITCODE -ne 0) { throw "accepted 1.6.0 baseline unavailable: $baseline" }
    git archive --format=zip --output $baselineZip $baseline .code-harness
    if ($LASTEXITCODE -ne 0) { throw 'failed to archive accepted 1.6.0 baseline' }
    Expand-Archive $baselineZip $root -Force
    Expand-Archive $upgradeZip $upgradeExtract -Force
    $target = Join-Path $root '.code-harness'
    if ((Get-Content (Join-Path $target 'VERSION') -Raw).Trim() -ne '1.6.0') { throw 'accepted baseline is not VERSION=1.6.0' }

    New-Item -ItemType Directory -Force (Join-Path $target 'bin') | Out-Null
    [IO.File]::WriteAllBytes((Join-Path $target 'bin/codea-harness-tools.exe'), [byte[]](76,69,71,65,67,89,45,49,54,48))
    if (Test-Path (Join-Path $target 'bin/codea-dcep-tools.exe')) { Remove-Item -Force (Join-Path $target 'bin/codea-dcep-tools.exe') }

    # Deterministic Project State sentinels.
    $harnessYaml = @'
version: 2
project:
  type: maven
  root: .
  module: ""
review:
  baseRef: origin/release-user-custom
  includeWorkingTree: false
integrationTest:
  executable: mvn
  args: [test]
  reportDir: target/surefire-reports
  timeoutSeconds: 777
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
    [IO.File]::WriteAllText((Join-Path $target 'harness.yaml'), $harnessYaml, [Text.UTF8Encoding]::new($false))
    [IO.File]::WriteAllText((Join-Path $target 'project.md'), "project-state-160`r`n", [Text.UTF8Encoding]::new($false))
    [IO.File]::WriteAllText((Join-Path $target 'database.yaml'), "version: 1`r`nenvironment: TEST`r`npassword: sentinel-secret`r`n", [Text.UTF8Encoding]::new($false))
    New-Item -ItemType Directory -Force (Join-Path $target 'runs/keep-run/requests'),(Join-Path $target 'chains') | Out-Null
    [IO.File]::WriteAllBytes((Join-Path $target 'runs/keep.bin'), [byte[]](1,6,0,1,6,1))
    [IO.File]::WriteAllText((Join-Path $target 'runs/keep-run/requests/agent-proposal.json'), '{"proposal":"preserve-160"}', [Text.UTF8Encoding]::new($false))
    [IO.File]::WriteAllText((Join-Path $target 'chains/order-approve.yaml'), "# user chain`r`nversion: 1`r`nid: order-approve`r`nstatus: ACCEPTED`r`n", [Text.UTF8Encoding]::new($false))
    $stateHashes = @{
        harness = (Get-FileHash -Algorithm SHA256 (Join-Path $target 'harness.yaml')).Hash
        project = (Get-FileHash -Algorithm SHA256 (Join-Path $target 'project.md')).Hash
        database = (Get-FileHash -Algorithm SHA256 (Join-Path $target 'database.yaml')).Hash
        runs = (Get-FileHash -Algorithm SHA256 (Join-Path $target 'runs/keep.bin')).Hash
        proposal = (Get-FileHash -Algorithm SHA256 (Join-Path $target 'runs/keep-run/requests/agent-proposal.json')).Hash
        chain = (Get-FileHash -Algorithm SHA256 (Join-Path $target 'chains/order-approve.yaml')).Hash
    }

    Copy-Item -Recurse (Join-Path $upgradeExtract '.code-harness-upgrade') (Join-Path $root '.code-harness-upgrade')
    $expectedRuntimeHash = (Get-FileHash -Algorithm SHA256 (Join-Path $root '.code-harness-upgrade/bin/codea-dcep-tools.exe')).Hash.ToLowerInvariant()
    Push-Location $root
    try {
        $upgradeOut = & '.\.code-harness-upgrade\bin\codea-dcep-tools.exe' upgrade 2>&1
        if ($LASTEXITCODE -ne 0) { throw "1.6.0 -> 1.6.1 live upgrade failed: $($upgradeOut -join "`n")" }
        if (($upgradeOut -join "`n") -notmatch '"status": "UPGRADED"') { throw "unexpected upgrade output: $($upgradeOut -join "`n")" }
        if ((Get-Content '.code-harness/VERSION' -Raw).Trim() -ne '1.6.1') { throw 'target VERSION is not 1.6.1' }
        if (Test-Path '.code-harness/bin/codea-harness-tools.exe') { throw 'legacy codea-harness-tools.exe survived 1.6.0 -> 1.6.1 upgrade' }
        if (!(Test-Path '.code-harness/bin/codea-dcep-tools.exe')) { throw 'renamed runtime missing after upgrade' }
        if ((Get-FileHash -Algorithm SHA256 '.code-harness/bin/codea-dcep-tools.exe').Hash.ToLowerInvariant() -ne $expectedRuntimeHash) { throw 'installed renamed runtime hash mismatch' }
        if ((Get-FileHash -Algorithm SHA256 '.code-harness/harness.yaml').Hash -ne $stateHashes.harness) { throw 'harness.yaml changed during upgrade' }
        if ((Get-FileHash -Algorithm SHA256 '.code-harness/project.md').Hash -ne $stateHashes.project) { throw 'project.md changed during upgrade' }
        if ((Get-FileHash -Algorithm SHA256 '.code-harness/database.yaml').Hash -ne $stateHashes.database) { throw 'database.yaml changed during upgrade' }
        if ((Get-FileHash -Algorithm SHA256 '.code-harness/runs/keep.bin').Hash -ne $stateHashes.runs) { throw 'runs/** changed during upgrade' }
        if ((Get-FileHash -Algorithm SHA256 '.code-harness/runs/keep-run/requests/agent-proposal.json').Hash -ne $stateHashes.proposal) { throw 'runs request changed during upgrade' }
        if ((Get-FileHash -Algorithm SHA256 '.code-harness/chains/order-approve.yaml').Hash -ne $stateHashes.chain) { throw 'chains/** changed during upgrade' }

        # Upgrade Runtime capability probe through installed renamed binary.
        $probe = & '.\.code-harness\bin\codea-dcep-tools.exe' review units --run-id __missing__ 2>&1
        if ($LASTEXITCODE -eq 0 -or (($probe -join "`n") -notmatch 'missing|not found|requires')) { throw "installed renamed Runtime capability probe did not reach controlled boundary: $($probe -join "`n")" }
    } finally { Pop-Location }

    Write-Host 'TASK161: validate whitelist against release binary and exact HEAD'
    $wl = Get-Content 'codea-dcep-tools-whitelist.txt'
    function Value-After([string]$Label) {
        $i = [Array]::IndexOf($wl, $Label)
        if ($i -lt 0 -or $i + 1 -ge $wl.Count) { throw "whitelist missing $Label" }
        return $wl[$i + 1].Trim()
    }
    if ((Value-After 'Version:') -ne '1.6.1') { throw 'whitelist Version mismatch' }
    if ((Value-After 'Binary:') -ne 'codea-dcep-tools.exe') { throw 'whitelist Binary mismatch' }
    if ((Value-After 'SHA256:') -ne $runtimeHash) { throw 'whitelist SHA256 mismatch' }
    if ((Value-After 'File Size:') -ne [string]$runtimeSize) { throw 'whitelist File Size mismatch' }
    if ((Value-After 'Build Commit:') -ne $exactHead) { throw 'whitelist Build Commit mismatch' }
    if ((Value-After 'GOOS:') -ne 'windows') { throw 'whitelist GOOS mismatch' }
    if ((Value-After 'GOARCH:') -ne 'amd64') { throw 'whitelist GOARCH mismatch' }
    if ((Value-After 'Go Version:') -ne $goVersion) { throw 'whitelist Go Version mismatch' }

    $installExtract = Join-Path $env:RUNNER_TEMP 'codea-install-extract-161'
    Remove-Item -Recurse -Force $installExtract -ErrorAction SilentlyContinue
    Expand-Archive $installZip $installExtract -Force
    $installRuntime = Join-Path $installExtract '.code-harness/bin/codea-dcep-tools.exe'
    if (!(Test-Path $installRuntime)) { throw 'install ZIP missing renamed runtime' }
    if (Test-Path (Join-Path $installExtract '.code-harness/bin/codea-harness-tools.exe')) { throw 'install ZIP contains legacy runtime' }
    if ((Get-FileHash -Algorithm SHA256 $installRuntime).Hash.ToLowerInvariant() -ne $runtimeHash) { throw 'install ZIP runtime does not match whitelist SHA256' }
    $upgradeRuntime = Join-Path $upgradeExtract '.code-harness-upgrade/bin/codea-dcep-tools.exe'
    if ((Get-FileHash -Algorithm SHA256 $upgradeRuntime).Hash.ToLowerInvariant() -ne $runtimeHash) { throw 'upgrade ZIP runtime does not match whitelist SHA256' }

    $installHash = (Get-FileHash -Algorithm SHA256 $installZip).Hash.ToLowerInvariant()
    $upgradeHash = (Get-FileHash -Algorithm SHA256 $upgradeZip).Hash.ToLowerInvariant()
    $checklist = [ordered]@{
        version = '1.6.1'
        exactHeadSha = $exactHead
        baseline = $baseline
        runtime = [ordered]@{ binary = 'codea-dcep-tools.exe'; sha256 = $runtimeHash; size = $runtimeSize; goVersion = $goVersion }
        whitelist = [ordered]@{ file = 'codea-dcep-tools-whitelist.txt'; runtimeSha256 = (Value-After 'SHA256:'); buildCommit = (Value-After 'Build Commit:'); signatureStatus = (Value-After 'Signature Status:') }
        artifacts = [ordered]@{
            install = [ordered]@{ file = $installZip; sha256 = $installHash; size = (Get-Item $installZip).Length }
            upgrade = [ordered]@{ file = $upgradeZip; sha256 = $upgradeHash; size = (Get-Item $upgradeZip).Length }
        }
        gates = [ordered]@{
            task161RuntimeRename = 'PASS'
            harnessInitRuntime = 'PASS'
            certifiedChangeAnalysis = 'PASS'
            reviewUnit = 'PASS'
            ruleDispatch = 'PASS'
            findingCertification = 'PASS'
            chainRuntime = 'PASS'
            upgradeRuntime = 'PASS'
            projectStatePreservation = 'PASS'
            whitelistEvidence = 'PASS'
            reviewPrecision = 'PASS'
            workspaceDependency = 'PASS'
        }
    } | ConvertTo-Json -Depth 8
    [IO.File]::WriteAllText('codea-harness-1.6.1-release-checklist.json', $checklist, [Text.UTF8Encoding]::new($false))

    Write-Host "TASK161_RELEASE_CERTIFICATION PASS exactHead=$exactHead runtimeSha256=$runtimeHash"
} finally {
    Pop-Location
}
exit 0
