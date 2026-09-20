$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$scopeScript = Join-Path $PSScriptRoot 'task164-release-blocker-task2-scope.ps1'
if (-not (Test-Path $scopeScript -PathType Leaf)) { throw "missing Task 2 scope helper: $scopeScript" }

$savedModeVariable = Get-Variable -Name task164ScopeTestMode -Scope Global -ErrorAction SilentlyContinue
$savedHeadVariable = Get-Variable -Name task164ScopeTestHead -Scope Global -ErrorAction SilentlyContinue
$hadModeVariable = $null -ne $savedModeVariable
$hadHeadVariable = $null -ne $savedHeadVariable
$savedModeValue = if ($hadModeVariable) { $savedModeVariable.Value } else { $null }
$savedHeadValue = if ($hadHeadVariable) { $savedHeadVariable.Value } else { $null }
$global:task164ScopeTestMode = 'Clean'
$global:task164ScopeTestHead = '1111111111111111111111111111111111111111'

function git {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$GitArgs)
    $command = $GitArgs -join ' '
    if ($command -like 'diff --name-only *...HEAD') {
        if ($global:task164ScopeTestMode -eq 'GitFailure') { $global:LASTEXITCODE = 1; return }
        $global:LASTEXITCODE = 0
        if ($global:task164ScopeTestMode -eq 'Forbidden') { Write-Output '.github/unexpected-task2-file.txt' }
        else { Write-Output '.github/scripts/task164-release-blocker-task2-scope-self-test.ps1' }
        return
    }
    if ($command -eq 'diff --name-only HEAD') {
        $global:LASTEXITCODE = 0
        if ($global:task164ScopeTestMode -eq 'Dirty') { Write-Output '.github/scripts/task164-release-blocker-task2-scope.ps1' }
        return
    }
    if ($command -eq 'rev-parse HEAD') {
        $global:LASTEXITCODE = 0
        Write-Output $global:task164ScopeTestHead
        return
    }
    throw "scope self-test received unexpected git command: $command"
}

function Invoke-ScopeFailure([string]$Mode, [string]$Expected) {
    $global:task164ScopeTestMode = $Mode
    try {
        & $scopeScript | Out-Null
    } catch {
        if ($_.Exception.Message -notmatch [regex]::Escape($Expected)) {
            throw "scope $Mode failed for the wrong reason: $($_.Exception.Message)"
        }
        return
    }
    throw "scope $Mode case unexpectedly passed"
}

$hadGitHubSha = Test-Path Env:GITHUB_SHA
$savedGitHubSha = $env:GITHUB_SHA
try {
    $env:GITHUB_SHA = $global:task164ScopeTestHead
    $global:task164ScopeTestMode = 'Clean'
    & $scopeScript | Out-Null

    Invoke-ScopeFailure 'Forbidden' 'Task 2 scope expanded'
    Invoke-ScopeFailure 'Dirty' 'tracked source changed during Task 2 CI'

    $global:task164ScopeTestMode = 'Clean'
    $env:GITHUB_SHA = '2222222222222222222222222222222222222222'
    Invoke-ScopeFailure 'Clean' 'exact HEAD mismatch'

    $env:GITHUB_SHA = $global:task164ScopeTestHead
    Invoke-ScopeFailure 'GitFailure' 'cannot inspect Task 2 scope'
} finally {
    if ($hadGitHubSha) { $env:GITHUB_SHA = $savedGitHubSha }
    else { Remove-Item Env:GITHUB_SHA -ErrorAction SilentlyContinue }
    if ($hadModeVariable) { Set-Variable -Name task164ScopeTestMode -Scope Global -Value $savedModeValue }
    else { Remove-Variable -Name task164ScopeTestMode -Scope Global -ErrorAction SilentlyContinue }
    if ($hadHeadVariable) { Set-Variable -Name task164ScopeTestHead -Scope Global -Value $savedHeadValue }
    else { Remove-Variable -Name task164ScopeTestHead -Scope Global -ErrorAction SilentlyContinue }
}

Write-Output 'TASK164_TASK2_SCOPE_BEHAVIOR_SELF_TEST PASS cases=5'
