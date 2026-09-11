param(
    [Parameter(Mandatory = $true)]
    [string]$ProjectRoot
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$packageRoot = (Resolve-Path $PSScriptRoot).Path
$targetRoot = [IO.Path]::GetFullPath($ProjectRoot)
$sourceHarness = Join-Path $packageRoot '.code-harness'
$targetHarness = Join-Path $targetRoot '.code-harness'
$hostResources = @(
    '.opencode/agents/reviewer.md',
    '.opencode/commands/harness-review-reviewer.md',
    '.opencode/tools/codea-reviewer-submit.ts'
)

function Fail-Manual([string]$Reason, [string]$Code) {
    Write-Output 'MANUAL_ACTION_REQUIRED'
    Write-Output $Code
    Write-Output '0 destructive overwrite'
    throw $Reason
}

if (-not (Test-Path $sourceHarness -PathType Container)) {
    throw 'invalid Codea Harness install package: .code-harness is missing'
}
foreach ($rel in $hostResources) {
    if (-not (Test-Path (Join-Path $packageRoot $rel) -PathType Leaf)) {
        throw "invalid Codea Harness install package: $rel is missing"
    }
}
New-Item -ItemType Directory -Force $targetRoot | Out-Null

# First-install only. Existing framework state must use the separately certified
# upgrade transaction instead of merging an install package into live state.
if (Test-Path $targetHarness) {
    Fail-Manual 'existing .code-harness detected; use the certified upgrade package instead of first install' 'INSTALL_EXISTING_HARNESS_FAIL_CLOSED'
}

# Preflight every OpenCode Host destination before *any* framework/Host write.
# Exact bytes are safe/idempotent; different bytes are user-owned collisions.
foreach ($rel in $hostResources) {
    $source = Join-Path $packageRoot $rel
    $target = Join-Path $targetRoot $rel
    if (Test-Path $target -PathType Leaf) {
        $sourceHash = (Get-FileHash -Algorithm SHA256 $source).Hash
        $targetHash = (Get-FileHash -Algorithm SHA256 $target).Hash
        if ($sourceHash -ne $targetHash) {
            Fail-Manual "existing OpenCode Host resource differs from Codea-managed bytes: $rel" 'INSTALL_EXISTING_OPENCODE_CONFLICT'
        }
    } elseif (Test-Path $target) {
        Fail-Manual "OpenCode Host destination exists but is not a file: $rel" 'INSTALL_EXISTING_OPENCODE_CONFLICT'
    }
}

# Preflight is complete. All subsequent writes are deterministic copies from the
# release package; no user-provided shell/text is evaluated.
Copy-Item -LiteralPath $sourceHarness -Destination $targetHarness -Recurse
foreach ($rel in $hostResources) {
    $source = Join-Path $packageRoot $rel
    $target = Join-Path $targetRoot $rel
    if (Test-Path $target -PathType Leaf) { continue }
    $parent = Split-Path -Parent $target
    New-Item -ItemType Directory -Force $parent | Out-Null
    Copy-Item -LiteralPath $source -Destination $target
}

Write-Output 'INSTALL_CODEA_HARNESS_ROOT PASS'
Write-Output 'INSTALL_REVIEWER_HOST_RESOURCES PASS'
