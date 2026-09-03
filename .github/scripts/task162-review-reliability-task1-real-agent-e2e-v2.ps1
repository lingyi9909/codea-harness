$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$source = Join-Path $PSScriptRoot 'task162-review-reliability-task1-real-agent-e2e.ps1'
if (!(Test-Path $source -PathType Leaf)) { throw "Task 1 E2E source missing: $source" }

$text = Get-Content -Raw $source
$oldUnknown = "                'unknown field',"
$newUnknown = "                'decode review report request: json: unknown field',"
$oldArray = "                'cannot unmarshal array',"
$newArray = "                'json: cannot unmarshal array',"
if (-not $text.Contains($oldUnknown)) { throw 'Task 1 E2E unknown-field assertion anchor missing' }
if (-not $text.Contains($oldArray)) { throw 'Task 1 E2E array assertion anchor missing' }
$text = $text.Replace($oldUnknown, $newUnknown).Replace($oldArray, $newArray)

$temp = Join-Path $PSScriptRoot '.task162-review-reliability-task1-real-agent-e2e-v2.tmp.ps1'
try {
    [IO.File]::WriteAllText($temp, $text, [Text.UTF8Encoding]::new($false))
    & $temp
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
finally {
    Remove-Item $temp -Force -ErrorAction SilentlyContinue
}
