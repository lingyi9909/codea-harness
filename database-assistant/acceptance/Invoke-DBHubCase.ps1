# Test-only wrapper around the OFFICIAL, preinstalled MCP Inspector CLI.
# No npx/network installation, no custom MCP transport, no Agent runtime.
[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][string]$InspectorExecutable,
    [Parameter(Mandatory=$true)][string]$InspectorConfig,
    [Parameter(Mandatory=$true)][string]$Server,
    [Parameter(Mandatory=$true)][string]$ToolName,
    [Parameter(Mandatory=$true)][string]$ToolsSnapshot,
    [Parameter(Mandatory=$true)][string]$CaseId,
    [Parameter(Mandatory=$true)][string]$OutDirectory,
    [switch]$ConfirmIsolated
)
$ErrorActionPreference = 'Stop'
if (-not $ConfirmIsolated) { throw 'Only the isolated synthetic fixture is authorized.' }
if (Test-Path -LiteralPath $OutDirectory) { throw 'Use a new output directory; evidence is never overwritten.' }
# Native CLI output may be either a tools/list result or a JSON-RPC envelope.
$snapshot = Get-Content -LiteralPath $ToolsSnapshot -Raw -Encoding UTF8 | ConvertFrom-Json
if ($null -ne $snapshot.result) { $snapshot = $snapshot.result }
if ($snapshot.nextCursor) { throw 'Incomplete tools/list: collect all pages before testing.' }
$matches = @($snapshot.tools | Where-Object { $_.name -ceq $ToolName })
if ($matches.Count -ne 1) { throw 'Actual tool name must appear exactly once in tools/list.' }
if ($matches[0].inputSchema.properties.sql.type -ne 'string') { throw 'Installed tool SQL schema differs; stop and inspect compatibility.' }
$cases = Get-Content -LiteralPath (Join-Path $PSScriptRoot 'cases.json') -Raw -Encoding UTF8 | ConvertFrom-Json
$selected = @($cases | Where-Object { $_.id -ceq $CaseId })
if ($selected.Count -ne 1) { throw 'Unknown fixed acceptance case.' }
$case = $selected[0]
New-Item -ItemType Directory -Path $OutDirectory | Out-Null
# Tool/source mapping and mysql-direct checks MUST already be verified by operator.
# Inspector config may hold secrets: never copy it into outputs or GitHub.
$argsForInspector = @('--cli','--config',$InspectorConfig,'--server',$Server,
    '--format','json','--method','tools/call','--tool-name',$ToolName,'--tool-arg',('sql=' + $case.sql))
& $InspectorExecutable @argsForInspector 1> (Join-Path $OutDirectory 'response.local.txt') 2> (Join-Path $OutDirectory 'stderr.local.txt')
$nativeExit = $LASTEXITCODE
[ordered]@{ case=$CaseId; tool=$ToolName; utc=(Get-Date).ToUniversalTime().ToString('o');
    exit_code=$nativeExit; verdict='PENDING_REVIEW'; layer='DBHub-direct-not-OpenCode'; sql=$case.sql
} | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $OutDirectory 'metadata.json') -Encoding UTF8
Write-Output 'PENDING_REVIEW: inspect MCP isError/result, server evidence and expected values; exit code is not a safety verdict.'
# A nonzero result can be expected for a negative case. Never convert it to PASS.
exit $nativeExit
