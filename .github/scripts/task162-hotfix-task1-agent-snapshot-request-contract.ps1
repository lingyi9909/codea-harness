$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$runtimeSource = Join-Path $repoRoot '.code-harness\bin\codea-dcep-tools.exe'
if (-not (Test-Path $runtimeSource -PathType Leaf)) { throw "Task1 Runtime missing: $runtimeSource" }

function Write-Utf8NoBom([string]$Path, [string]$Content) {
    $parent = Split-Path -Parent $Path
    if ($parent) { New-Item -ItemType Directory -Force $parent | Out-Null }
    [System.IO.File]::WriteAllText($Path, $Content, [System.Text.UTF8Encoding]::new($false))
}

function Invoke-GitAt([string]$Root, [Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments) {
    Push-Location $Root
    try {
        & git @Arguments | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "git $($Arguments -join ' ') failed with exit code $LASTEXITCODE" }
    }
    finally { Pop-Location }
}

$root = Join-Path $env:RUNNER_TEMP ("task162-hotfix-agent-snapshot-contract-" + [guid]::NewGuid().ToString('N'))
try {
    New-Item -ItemType Directory -Force $root | Out-Null
    Invoke-GitAt $root init
    Invoke-GitAt $root config user.email 'task162-hotfix@example.test'
    Invoke-GitAt $root config user.name 'Task 162 Hotfix'
    Invoke-GitAt $root config core.autocrlf false
    Write-Utf8NoBom (Join-Path $root 'seed.txt') "seed`n"
    Invoke-GitAt $root add seed.txt
    Invoke-GitAt $root commit -m 'base'

    New-Item -ItemType Directory -Force (Join-Path $root '.code-harness\bin') | Out-Null
    New-Item -ItemType Directory -Force (Join-Path $root '.code-harness\contracts') | Out-Null
    New-Item -ItemType Directory -Force (Join-Path $root '.code-harness\runs\contract-test\requests') | Out-Null
    Copy-Item $runtimeSource (Join-Path $root '.code-harness\bin\codea-dcep-tools.exe') -Force
    Copy-Item (Join-Path $repoRoot '.code-harness\VERSION') (Join-Path $root '.code-harness\VERSION') -Force
    Copy-Item (Join-Path $repoRoot '.code-harness\contracts\change-set.schema.json') (Join-Path $root '.code-harness\contracts\change-set.schema.json') -Force
    Copy-Item (Join-Path $repoRoot '.code-harness\contracts\change-set-request.schema.json') (Join-Path $root '.code-harness\contracts\change-set-request.schema.json') -Force

    $requestPath = Join-Path $root '.code-harness\runs\contract-test\requests\change-set-request.json'
    Write-Utf8NoBom $requestPath '{"runId":"contract-test","baseRef":"HEAD","includeWorkingTree":true}'

    $runtime = Join-Path $root '.code-harness\bin\codea-dcep-tools.exe'
    Push-Location $root
    try {
        $output = (& $runtime analysis snapshot --input '.code-harness/runs/contract-test/requests/change-set-request.json' 2>&1 | Out-String)
        if ($LASTEXITCODE -ne 0) { throw "Agent-facing baseRef snapshot request failed:`n$output" }
    }
    finally { Pop-Location }

    $snapshotPath = Join-Path $root '.code-harness\runs\contract-test\analysis\change-set.json'
    if (-not (Test-Path $snapshotPath -PathType Leaf)) { throw 'Runtime did not publish canonical change-set.json' }
    $snapshot = Get-Content $snapshotPath -Raw | ConvertFrom-Json
    if ([string]$snapshot.requestedBaseRef -ne 'HEAD') { throw "requestedBaseRef provenance mismatch: $($snapshot.requestedBaseRef)" }
    if (-not [bool]$snapshot.includeWorkingTree) { throw 'includeWorkingTree was not preserved by Runtime Snapshot' }
    if ([string]::IsNullOrWhiteSpace([string]$snapshot.snapshotSha256)) { throw 'snapshotSha256 missing' }

    $global:LASTEXITCODE = 0
    Write-Output 'TASK162_HOTFIX_AGENT_SNAPSHOT_REQUEST_RUNTIME PASS'
}
finally {
    Remove-Item $root -Recurse -Force -ErrorAction SilentlyContinue
}
