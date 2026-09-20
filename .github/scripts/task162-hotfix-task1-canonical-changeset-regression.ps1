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

function New-Task1Repo([string]$Name) {
    $root = Join-Path $env:RUNNER_TEMP ("task162-hotfix-task1-$Name-" + [guid]::NewGuid().ToString('N'))
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
    Copy-Item $runtimeSource (Join-Path $root '.code-harness\bin\codea-dcep-tools.exe') -Force
    Copy-Item (Join-Path $repoRoot '.code-harness\VERSION') (Join-Path $root '.code-harness\VERSION') -Force
    foreach ($name in @('change-set.schema.json','change-set-request.schema.json','analysis-certify-request.schema.json','change-analysis-proposal.schema.json','change-analysis.schema.json','entrypoint-inventory.schema.json','change-analysis-cert.schema.json')) {
        Copy-Item (Join-Path $repoRoot ".code-harness\contracts\$name") (Join-Path $root ".code-harness\contracts\$name") -Force
    }
    return $root
}

function Invoke-RuntimeAt([string]$Root, [Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments) {
    $runtime = Join-Path $Root '.code-harness\bin\codea-dcep-tools.exe'
    Push-Location $Root
    try {
        $text = (& $runtime @Arguments 2>&1 | Out-String)
        if ($LASTEXITCODE -ne 0) { throw "Runtime $($Arguments -join ' ') failed:`n$text" }
        return $text.Trim()
    }
    finally { Pop-Location }
}

function Invoke-RuntimeExpectFailure([string]$Root, [Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments) {
    $runtime = Join-Path $Root '.code-harness\bin\codea-dcep-tools.exe'
    Push-Location $Root
    try {
        $text = (& $runtime @Arguments 2>&1 | Out-String)
        if ($LASTEXITCODE -eq 0) { throw "Runtime unexpectedly succeeded: $($Arguments -join ' ')" }
        return $text.Trim()
    }
    finally { Pop-Location }
}

function New-Snapshot([string]$Root, [string]$RunId, [string]$BaseRef, [bool]$IncludeWorkingTree) {
    $requestPath = Join-Path $Root ".code-harness\runs\$RunId\requests\change-set-request.json"
    $request = [ordered]@{ runId=$RunId; baseRef=$BaseRef; includeWorkingTree=$IncludeWorkingTree }
    Write-Utf8NoBom $requestPath ($request | ConvertTo-Json -Compress)
    Invoke-RuntimeAt $Root analysis snapshot --input ".code-harness/runs/$RunId/requests/change-set-request.json" | Out-Null
    return Get-Content (Join-Path $Root ".code-harness\runs\$RunId\analysis\change-set.json") -Raw | ConvertFrom-Json
}

function Assert-Sources($Snapshot, [string]$Path, [string[]]$Expected) {
    $item = @($Snapshot.files | Where-Object { [string]$_.path -eq $Path }) | Select-Object -First 1
    if ($null -eq $item) { throw "missing canonical file $Path" }
    $got = @($item.sources | ForEach-Object { [string]$_ })
    foreach ($source in $Expected) {
        if ($got -notcontains $source) { throw "$Path sources=$($got -join ',') missing $source" }
    }
}

$roots = [System.Collections.Generic.List[string]]::new()
try {
    # 1. Zero diff.
    $r = New-Task1Repo 'zero'; $roots.Add($r)
    $s = New-Snapshot $r 'zero' 'HEAD' $true
    if (@($s.files).Count -ne 0) { throw "zero diff files=$(@($s.files).Count)" }
    Write-Output 'TASK162_HOTFIX_ZERO_DIFF PASS'

    # 2/3/6. Same HEAD + working tree, mixed Review/non-Review, unstaged.
    $r = New-Task1Repo 'working'; $roots.Add($r)
    Write-Utf8NoBom (Join-Path $r 'src/main/resources/application.yml') "feature: true`n"
    Write-Utf8NoBom (Join-Path $r 'pom.xml') '<project />'
    Write-Utf8NoBom (Join-Path $r 'README.md') 'not review scope'
    Write-Utf8NoBom (Join-Path $r 'src/main/resources/application.properties') 'feature=true'
    Invoke-GitAt $r add src/main/resources/application.yml
    Invoke-GitAt $r commit -m 'seed review file'
    Write-Utf8NoBom (Join-Path $r 'src/main/resources/application.yml') "feature: false`n"
    $s = New-Snapshot $r 'working' 'HEAD' $true
    if (@($s.files).Count -ne 1 -or [string]$s.files[0].path -ne 'src/main/resources/application.yml') { throw "mixed filtering failed: $($s.files | ConvertTo-Json -Depth 5 -Compress)" }
    Assert-Sources $s 'src/main/resources/application.yml' @('UNSTAGED')
    Write-Output 'TASK162_HOTFIX_SAME_HEAD_WORKING_TREE PASS'
    Write-Output 'TASK162_HOTFIX_MIXED_REVIEW_NON_REVIEW PASS'
    Write-Output 'TASK162_HOTFIX_UNSTAGED PASS'

    # 4. Committed.
    $r = New-Task1Repo 'committed'; $roots.Add($r)
    Write-Utf8NoBom (Join-Path $r 'src/main/resources/application.yml') "feature: committed`n"
    Invoke-GitAt $r add src/main/resources/application.yml
    Invoke-GitAt $r commit -m 'review commit'
    $s = New-Snapshot $r 'committed' 'HEAD~1' $true
    Assert-Sources $s 'src/main/resources/application.yml' @('COMMITTED')
    Write-Output 'TASK162_HOTFIX_COMMITTED PASS'

    # 5. Staged.
    $r = New-Task1Repo 'staged'; $roots.Add($r)
    Write-Utf8NoBom (Join-Path $r 'src/main/resources/application.yml') "feature: base`n"
    Invoke-GitAt $r add src/main/resources/application.yml
    Invoke-GitAt $r commit -m 'review base'
    Write-Utf8NoBom (Join-Path $r 'src/main/resources/application.yml') "feature: staged`n"
    Invoke-GitAt $r add src/main/resources/application.yml
    $s = New-Snapshot $r 'staged' 'HEAD' $true
    Assert-Sources $s 'src/main/resources/application.yml' @('STAGED')
    Write-Output 'TASK162_HOTFIX_STAGED PASS'

    # 7. Untracked.
    $r = New-Task1Repo 'untracked'; $roots.Add($r)
    Write-Utf8NoBom (Join-Path $r 'src/main/resources/application.yml') "feature: untracked`n"
    $s = New-Snapshot $r 'untracked' 'HEAD' $true
    Assert-Sources $s 'src/main/resources/application.yml' @('UNTRACKED')
    Write-Output 'TASK162_HOTFIX_UNTRACKED PASS'

    # 8/9. Same file multiple sources and includeWorkingTree=false.
    $r = New-Task1Repo 'multisource'; $roots.Add($r)
    Write-Utf8NoBom (Join-Path $r 'src/main/resources/application.yml') "feature: base`n"
    Invoke-GitAt $r add src/main/resources/application.yml
    Invoke-GitAt $r commit -m 'review base'
    Write-Utf8NoBom (Join-Path $r 'src/main/resources/application.yml') "feature: committed`n"
    Invoke-GitAt $r add src/main/resources/application.yml
    Invoke-GitAt $r commit -m 'review committed'
    Write-Utf8NoBom (Join-Path $r 'src/main/resources/application.yml') "feature: staged`n"
    Invoke-GitAt $r add src/main/resources/application.yml
    Write-Utf8NoBom (Join-Path $r 'src/main/resources/application.yml') "feature: unstaged`n"
    Write-Utf8NoBom (Join-Path $r 'src/main/resources/extra.yml') "extra: untracked`n"
    $s = New-Snapshot $r 'multisource' 'HEAD~1' $true
    Assert-Sources $s 'src/main/resources/application.yml' @('COMMITTED','STAGED','UNSTAGED')
    Assert-Sources $s 'src/main/resources/extra.yml' @('UNTRACKED')
    Write-Output 'TASK162_HOTFIX_MULTI_SOURCE PASS'
    $without = New-Snapshot $r 'no-working' 'HEAD~1' $false
    if (@($without.files).Count -ne 1) { throw "includeWorkingTree=false leaked files: $($without.files | ConvertTo-Json -Depth 5 -Compress)" }
    Assert-Sources $without 'src/main/resources/application.yml' @('COMMITTED')
    if (@($without.files[0].sources).Count -ne 1) { throw 'includeWorkingTree=false leaked working-tree sources' }
    Write-Output 'TASK162_HOTFIX_INCLUDE_WORKING_TREE_FALSE PASS'

    # 10. Maven multi-module prefixes are canonical paths.
    $r = New-Task1Repo 'multimodule'; $roots.Add($r)
    Write-Utf8NoBom (Join-Path $r 'module-a/src/main/java/acme/AService.java') 'package acme; class AService {}'
    Write-Utf8NoBom (Join-Path $r 'module-b/src/test/java/acme/BServiceTest.java') 'package acme; class BServiceTest {}'
    Write-Utf8NoBom (Join-Path $r 'module-c/src/main/resources/mapper/CMapper.xml') '<mapper namespace="acme.CMapper" />'
    $s = New-Snapshot $r 'multimodule' 'HEAD' $true
    $paths = @($s.files | ForEach-Object { [string]$_.path })
    foreach ($required in @('module-a/src/main/java/acme/AService.java','module-b/src/test/java/acme/BServiceTest.java','module-c/src/main/resources/mapper/CMapper.xml')) {
        if ($paths -notcontains $required) { throw "multi-module canonical path missing $required; got=$($paths -join ',')" }
    }
    Write-Output 'TASK162_HOTFIX_MAVEN_MULTIMODULE PASS'

    # 11. Equivalent ref spellings resolving to one commit share identity.
    $r = New-Task1Repo 'equivalent'; $roots.Add($r)
    Invoke-GitAt $r update-ref refs/heads/main HEAD
    Invoke-GitAt $r update-ref refs/remotes/origin/main HEAD
    $a = New-Snapshot $r 'eq-main' 'main' $false
    $b = New-Snapshot $r 'eq-origin' 'origin/main' $false
    $c = New-Snapshot $r 'eq-full' 'refs/heads/main' $false
    if ([string]$a.resolvedBaseCommit -ne [string]$b.resolvedBaseCommit -or [string]$a.resolvedBaseCommit -ne [string]$c.resolvedBaseCommit) { throw 'equivalent refs did not resolve to same commit' }
    if ([string]$a.snapshotSha256 -ne [string]$b.snapshotSha256 -or [string]$a.snapshotSha256 -ne [string]$c.snapshotSha256) { throw 'equivalent refs produced different canonical identity' }
    Write-Output 'TASK162_HOTFIX_EQUIVALENT_BASE_REF PASS'

    # 12. Different resolved bases are distinct identities.
    $r = New-Task1Repo 'distinct'; $roots.Add($r)
    Invoke-GitAt $r update-ref refs/heads/base-a HEAD
    Write-Utf8NoBom (Join-Path $r 'src/main/resources/application.yml') "feature: commit2`n"
    Invoke-GitAt $r add src/main/resources/application.yml
    Invoke-GitAt $r commit -m 'second'
    Invoke-GitAt $r update-ref refs/heads/base-b HEAD
    $a = New-Snapshot $r 'base-a' 'base-a' $false
    $b = New-Snapshot $r 'base-b' 'base-b' $false
    if ([string]$a.resolvedBaseCommit -eq [string]$b.resolvedBaseCommit -or [string]$a.snapshotSha256 -eq [string]$b.snapshotSha256) { throw 'different resolved bases collapsed to same identity' }
    Write-Output 'TASK162_HOTFIX_DISTINCT_BASE_REF PASS'

    # 15. Successful certification copies every Git identity field from Runtime Snapshot.
    $r = New-Task1Repo 'certified'; $roots.Add($r)
    Write-Utf8NoBom (Join-Path $r 'src/main/resources/application.yml') "feature: base`n"
    Invoke-GitAt $r add src/main/resources/application.yml
    Invoke-GitAt $r commit -m 'review base'
    Write-Utf8NoBom (Join-Path $r 'src/main/resources/application.yml') "feature: true`n"
    $s = New-Snapshot $r 'certified' 'HEAD' $true
    $proposal = [ordered]@{
        changedFileRoles = @([ordered]@{ path='src/main/resources/application.yml'; role='YamlConfig' })
        affectedControllers = @(); callChains = @(); symbolLocations = @(); resourceRelations = @(); externalDependencies = @(); riskAreas = @()
        reviewCoverage = [ordered]@{ status='COMPLETE'; reviewedFiles=@([ordered]@{ path='src/main/resources/application.yml'; role='YamlConfig'; reason='CHANGED' }); unresolvedSymbols=@() }
    }
    Write-Utf8NoBom (Join-Path $r '.code-harness/runs/certified/requests/change-analysis-proposal.json') ($proposal | ConvertTo-Json -Depth 20)
    $certReq = [ordered]@{ runId='certified'; snapshotPath='.code-harness/runs/certified/analysis/change-set.json'; snapshotSha256=[string]$s.snapshotSha256; proposalPath='.code-harness/runs/certified/requests/change-analysis-proposal.json'; intent=[ordered]@{mode='FULL'} }
    Write-Utf8NoBom (Join-Path $r '.code-harness/runs/certified/requests/analysis-certify.json') ($certReq | ConvertTo-Json -Depth 10 -Compress)
    Invoke-RuntimeAt $r analysis certify --input '.code-harness/runs/certified/requests/analysis-certify.json' | Out-Null
    $analysis = Get-Content (Join-Path $r '.code-harness/runs/certified/analysis/change-analysis.json') -Raw | ConvertFrom-Json
    $cert = Get-Content (Join-Path $r '.code-harness/runs/certified/analysis/change-analysis.cert.json') -Raw | ConvertFrom-Json
    $scope = $analysis.reviewScope
    foreach ($check in @(
        @([string]$scope.baseRef, [string]$s.requestedBaseRef, 'baseRef'),
        @([string]$scope.baseCommit, [string]$s.resolvedBaseCommit, 'baseCommit'),
        @([string]$scope.mergeBase, [string]$s.mergeBase, 'mergeBase'),
        @([string]$scope.headCommit, [string]$s.headCommit, 'headCommit'),
        @([string]$scope.currentBranch, [string]$s.currentBranch, 'currentBranch'),
        @([string]$cert.snapshotSha256, [string]$s.snapshotSha256, 'snapshotSha256')
    )) { if ($check[0] -ne $check[1]) { throw "Certified Git identity mismatch $($check[2]): $($check[0]) != $($check[1])" } }
    if ([bool]$scope.includeWorkingTree -ne [bool]$s.includeWorkingTree -or [bool]$cert.includeWorkingTree -ne [bool]$s.includeWorkingTree) { throw 'Certified includeWorkingTree mismatch' }
    Write-Output 'TASK162_HOTFIX_CERTIFIED_GIT_IDENTITY PASS'

    # 13. Snapshot stale after same-hunk byte change: certify must fail closed.
    $r = New-Task1Repo 'stale'; $roots.Add($r)
    Write-Utf8NoBom (Join-Path $r 'src/main/resources/application.yml') "feature: base`n"
    Invoke-GitAt $r add src/main/resources/application.yml
    Invoke-GitAt $r commit -m 'review base'
    Write-Utf8NoBom (Join-Path $r 'src/main/resources/application.yml') "feature: true`n"
    $s = New-Snapshot $r 'stale' 'HEAD' $true
    $proposal = [ordered]@{
        changedFileRoles = @([ordered]@{ path='src/main/resources/application.yml'; role='YamlConfig' })
        affectedControllers = @(); callChains = @(); symbolLocations = @(); resourceRelations = @(); externalDependencies = @(); riskAreas = @()
        reviewCoverage = [ordered]@{ status='COMPLETE'; reviewedFiles=@([ordered]@{ path='src/main/resources/application.yml'; role='YamlConfig'; reason='CHANGED' }); unresolvedSymbols=@() }
    }
    Write-Utf8NoBom (Join-Path $r '.code-harness/runs/stale/requests/change-analysis-proposal.json') ($proposal | ConvertTo-Json -Depth 20)
    $certReq = [ordered]@{ runId='stale'; snapshotPath='.code-harness/runs/stale/analysis/change-set.json'; snapshotSha256=[string]$s.snapshotSha256; proposalPath='.code-harness/runs/stale/requests/change-analysis-proposal.json'; intent=[ordered]@{mode='FULL'} }
    Write-Utf8NoBom (Join-Path $r '.code-harness/runs/stale/requests/analysis-certify.json') ($certReq | ConvertTo-Json -Depth 10 -Compress)
    Write-Utf8NoBom (Join-Path $r 'src/main/resources/application.yml') "feature: maybe`n"
    $failure = Invoke-RuntimeExpectFailure $r analysis certify --input '.code-harness/runs/stale/requests/analysis-certify.json'
    if ($failure -notmatch 'CHANGE_SET_SNAPSHOT_STALE') { throw "expected CHANGE_SET_SNAPSHOT_STALE, got:`n$failure" }
    foreach ($name in @('change-analysis.json','entrypoint-inventory.json','change-analysis.cert.json')) {
        if (Test-Path (Join-Path $r ".code-harness/runs/stale/analysis/$name")) { throw "stale snapshot published $name" }
    }
    Write-Output 'TASK162_HOTFIX_STALE_SNAPSHOT_REJECTED PASS'

    Write-Output 'TASK162_HOTFIX_CANONICAL_CHANGESET_REGRESSION PASS'
}
finally {
    foreach ($root in $roots) { Remove-Item $root -Recurse -Force -ErrorAction SilentlyContinue }
}