# T01 smoke checks only. No installers, downloads, runtime changes or firewall changes.
$ErrorActionPreference = "Stop"
if ($env:OS -ne "Windows_NT") { throw "Must run on actual company Windows host" }
node --version
npm --version
git --version
if (-not $env:CODE_HARNESS_T01_MANIFEST) {
  throw "Set CODE_HARNESS_T01_MANIFEST to a local, sanitized, approved native API manifest JSON"
}
$manifest = (Resolve-Path $env:CODE_HARNESS_T01_MANIFEST).Path
npm run typecheck
npm test
npm run test:windows-smoke -- --manifest "$manifest"
if ($LASTEXITCODE -ne 0) {
  Write-Warning "T01 not READY; complete actual native session/permissions/report/prompt/interrupt verification"
}
