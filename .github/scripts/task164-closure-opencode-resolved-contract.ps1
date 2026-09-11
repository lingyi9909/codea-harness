$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$installZip = Join-Path $repoRoot 'codea-harness-1.6.4-windows-x64-install.zip'
if (-not (Test-Path $installZip -PathType Leaf)) { throw "missing install package: $installZip" }
if (-not (Get-Command opencode -ErrorAction SilentlyContinue)) { throw 'opencode is required' }

$utf8 = [Text.UTF8Encoding]::new($false)
function Write-Utf8NoBom([string]$Path, [string]$Content) {
    $parent = Split-Path -Parent $Path
    if ($parent) { New-Item -ItemType Directory -Force $parent | Out-Null }
    [IO.File]::WriteAllText($Path, $Content, $utf8)
}

function Invoke-OpenCodeJson([string[]]$Arguments) {
    $stderr = Join-Path $env:RUNNER_TEMP ('task164-opencode-debug-' + [guid]::NewGuid().ToString('N') + '.stderr.log')
    try {
        $raw = (& opencode @Arguments 2> $stderr | Out-String)
        $exit = $LASTEXITCODE
        if ($exit -ne 0) {
            $err = if (Test-Path $stderr) { Get-Content -Raw $stderr } else { '' }
            throw "opencode $($Arguments -join ' ') failed exit=$exit`n$raw`n$err"
        }
        return ($raw | ConvertFrom-Json)
    }
    finally { Remove-Item $stderr -Force -ErrorAction SilentlyContinue }
}

$fixture = Join-Path $env:RUNNER_TEMP ('task164-opencode-resolved-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force $fixture | Out-Null
try {
    Expand-Archive -Path $installZip -DestinationPath $fixture -Force
    foreach ($required in @(
        '.opencode/agents/reviewer.md',
        '.opencode/commands/harness-review-reviewer.md',
        '.opencode/tools/codea-reviewer-submit.ts'
    )) {
        if (-not (Test-Path (Join-Path $fixture $required) -PathType Leaf)) { throw "install package missing $required" }
    }

    # debug agent resolves tools only when a model exists. This provider definition is
    # local metadata; no model request is made by the debug commands.
    $config = @{
        '$schema' = 'https://opencode.ai/config.json'
        model = 'task164-local/task164'
        provider = @{
            'task164-local' = @{
                npm = '@ai-sdk/openai-compatible'
                name = 'Task164 Local'
                options = @{ baseURL = 'http://127.0.0.1:9/v1'; apiKey = 'unused' }
                models = @{ 'task164' = @{ name = 'Task164 Local'; limit = @{ context = 16000; output = 1024 } } }
            }
        }
    } | ConvertTo-Json -Depth 20
    Write-Utf8NoBom (Join-Path $fixture 'opencode.json') $config

    Push-Location $fixture
    try {
        $version = (& opencode --version | Out-String).Trim()
        if ($version -ne '1.18.25') { throw "expected opencode 1.18.25, got $version" }
        $agent = Invoke-OpenCodeJson @('debug','agent','reviewer')
        $resolved = Invoke-OpenCodeJson @('debug','config')
    }
    finally { Pop-Location }

    if ([string]$agent.mode -ne 'subagent') { throw "reviewer mode must resolve to subagent" }
    foreach ($denied in @('bash','task','write','edit','apply_patch')) {
        $prop = $agent.tools.PSObject.Properties[$denied]
        if ($null -ne $prop -and [bool]$prop.Value) { throw "Reviewer tool must be denied by resolved OpenCode config: $denied" }
    }
    $submit = $agent.tools.PSObject.Properties['codea-reviewer-submit']
    if ($null -eq $submit -or -not [bool]$submit.Value) { throw 'codea-reviewer-submit must be enabled for Reviewer' }

    $permissionJson = $agent.permission | ConvertTo-Json -Depth 20 -Compress
    foreach ($permissionName in @('bash','task','edit','codea-reviewer-submit')) {
        if (-not $permissionJson.Contains('"permission":"' + $permissionName + '"')) {
            throw "resolved Reviewer ruleset missing explicit $permissionName permission"
        }
    }

    $command = $resolved.command.PSObject.Properties['harness-review-reviewer']
    if ($null -eq $command) { throw 'resolved command harness-review-reviewer missing' }
    if ([string]$command.Value.agent -ne 'reviewer') { throw 'resolved reviewer command agent must equal reviewer' }
    if ($command.Value.subtask -ne $true) { throw 'resolved reviewer command subtask must equal true' }

    Write-Output 'OPENCODE_11825_REVIEWER_PERMISSION_RESOLVED PASS'
    Write-Output 'OPENCODE_11825_REVIEWER_COMMAND_SUBTASK_RESOLVED PASS'
    Write-Output 'REVIEWER_BASH_DENIED PASS'
    Write-Output 'REVIEWER_TASK_DENIED PASS'
    Write-Output 'REVIEWER_RUNTIME_ARTIFACT_WRITE_DENIED PASS'
    Write-Output 'REVIEWER_SUBMIT_TOOL_ALLOWED PASS'
}
finally {
    Remove-Item -Recurse -Force $fixture -ErrorAction SilentlyContinue
}
