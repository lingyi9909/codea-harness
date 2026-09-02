$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$hostRoot = Join-Path $PSScriptRoot 'task162-hotfix-task3'
$modelServer = Join-Path $hostRoot 'mock_openai_server.py'

if (!(Test-Path $modelServer)) {
    throw 'TASK162_HOTFIX_TASK3_RED: real Agent Host model fixture is missing; scripted Runtime invocation is not an acceptable substitute'
}

throw 'TASK162_HOTFIX_TASK3_RED: implementation unexpectedly present on RED branch'
