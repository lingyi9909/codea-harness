$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

# Exercise the actual production scope guard in a disposable Git repository.
# The GitHub Actions checkout is deliberately detached; branch identity comes
# from GITHUB_REF, while the commit and ancestry are checked independently.
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$source = Join-Path $PSScriptRoot 'task163-final-certification.ps1'
$tokens = $null
$parseErrors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile($source, [ref]$tokens, [ref]$parseErrors)
if (@($parseErrors).Count -ne 0) { throw "Certification script does not parse: $parseErrors" }
$definition = @($ast.FindAll({ param($node) $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq 'Assert-ReleaseScope' }, $true))
if ($definition.Count -ne 1) { throw 'Expected exactly one production Assert-ReleaseScope function' }
. ([scriptblock]::Create($definition[0].Extent.Text))

function Invoke-Checked([string]$Executable, [string[]]$Arguments) {
    & $Executable @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$Executable $($Arguments -join ' ') exited $LASTEXITCODE" }
}
function Invoke-Git([string[]]$Arguments) {
    & git -C $root @Arguments
    if ($LASTEXITCODE -ne 0) { throw "git $($Arguments -join ' ') exited $LASTEXITCODE" }
}
function Assert-Case([string]$Name, [bool]$ShouldPass, [string]$ExpectedError = '') {
    $global:LASTEXITCODE = 0
    try {
        $output = @(& { Assert-ReleaseScope } 2>&1)
        if (-not $ShouldPass) { throw "Case unexpectedly passed: $Name" }
        if (($output | Out-String) -notmatch 'TASK163_FINAL_SCOPE PASS') { throw "Missing scope PASS evidence: $Name" }
    } catch {
        if ($ShouldPass) { throw "Case $Name failed: $_" }
        $message = ($_ | Out-String)
        if ($message.Contains('Case unexpectedly passed:')) { throw $message }
        if (-not $message.Contains($ExpectedError)) { throw "Case $Name failed for the wrong reason: $message" }
    }
    $global:LASTEXITCODE = 0
    Write-Output "TASK163_FINAL_CONTRACT PASS case=$Name"
}

$root = Join-Path ([IO.Path]::GetTempPath()) ('task163-final-contract-' + [guid]::NewGuid().ToString('N'))
$version = '1.6.3'
$oldRef = $env:GITHUB_REF
$oldRefName = $env:GITHUB_REF_NAME
$oldSha = $env:GITHUB_SHA
try {
    New-Item -ItemType Directory -Force (Join-Path $root '.code-harness') | Out-Null
    Invoke-Git @('init','-q')
    Invoke-Git @('config','user.name','Certification Test')
    Invoke-Git @('config','user.email','certification@example.invalid')
    [IO.File]::WriteAllText((Join-Path $root '.code-harness/VERSION'), '1.6.2')
    Invoke-Git @('add','.')
    Invoke-Git @('commit','-qm','fixture baseline')
    $base = (& git -C $root rev-parse HEAD).Trim()
    Invoke-Git @('checkout','-qb','release/1.6.3-final-certification')
    [IO.File]::WriteAllText((Join-Path $root '.code-harness/VERSION'), '1.6.3')
    Invoke-Git @('add','.')
    Invoke-Git @('commit','-qm','fixture release')
    $candidate = (& git -C $root rev-parse HEAD).Trim()
    Invoke-Git @('checkout','-q','--detach',$candidate)
    if (-not [string]::IsNullOrWhiteSpace((& git -C $root branch --show-current | Out-String).Trim())) { throw 'Fixture is not detached' }

    $expected = $candidate
    $env:GITHUB_SHA = $candidate
    $env:GITHUB_REF = 'refs/heads/release/1.6.3-final-certification'
    $env:GITHUB_REF_NAME = 'release/1.6.3-final-certification'
    Assert-Case 'detached-exact-head' $true

    $env:GITHUB_REF = 'refs/heads/develop'
    Assert-Case 'wrong-ref-rejected' $false 'Unexpected release ref'
    $env:GITHUB_REF = ''
    Assert-Case 'missing-ref-rejected' $false 'Unexpected release ref'
    $env:GITHUB_REF = 'refs/heads/release/1.6.3-final-certification'
    $expected = $base
    Assert-Case 'wrong-sha-rejected' $false 'Exact HEAD mismatch'
    $expected = $candidate

    New-Item -ItemType Directory -Force (Join-Path $root 'src') | Out-Null
    [IO.File]::WriteAllText((Join-Path $root 'src/unauthorized.txt'), 'not a release metadata change')
    Invoke-Git @('add','.')
    Invoke-Git @('commit','-qm','fixture unauthorized change')
    $expected = (& git -C $root rev-parse HEAD).Trim()
    $env:GITHUB_SHA = $expected
    Assert-Case 'scope-rejected' $false 'Unapproved release scope'

    Write-Output 'TASK163_FINAL_CONTRACT_REGRESSION PASS'
} finally {
    $env:GITHUB_REF = $oldRef
    $env:GITHUB_REF_NAME = $oldRefName
    $env:GITHUB_SHA = $oldSha
    Remove-Item $root -Recurse -Force -ErrorAction SilentlyContinue
}
