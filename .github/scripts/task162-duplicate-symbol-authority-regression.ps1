$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$runtimeSource = Join-Path $repoRoot '.code-harness\bin\codea-dcep-tools.exe'
$astSource = Join-Path $repoRoot '.code-harness\bin\ast-grep.exe'
$versionSource = Join-Path $repoRoot '.code-harness\VERSION'
$catalogSource = Join-Path $repoRoot '.code-harness\review-rules\spring-v1.yaml'
$fixtureSource = Join-Path $repoRoot '.code-harness\tools-runtime\testdata\multi-module-fixture'

foreach ($required in @($runtimeSource, $astSource, $versionSource, $catalogSource, $fixtureSource)) {
    if (-not (Test-Path $required)) { throw "Task162 duplicate-symbol missing required path: $required" }
}

$fixture = Join-Path $env:RUNNER_TEMP ("task162-duplicate-symbol-" + [guid]::NewGuid().ToString('N'))
Copy-Item -Recurse $fixtureSource $fixture

function Write-Utf8NoBom([string]$Path, [string]$Content) {
    $target = $Path
    if (-not [System.IO.Path]::IsPathRooted($target)) { $target = Join-Path (Get-Location).Path $target }
    $parent = Split-Path -Parent $target
    if ($parent) { New-Item -ItemType Directory -Force $parent | Out-Null }
    [System.IO.File]::WriteAllText($target, $Content, [System.Text.UTF8Encoding]::new($false))
}

function Invoke-Git([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments) {
    & git @Arguments
    if ($LASTEXITCODE -ne 0) { throw "git $($Arguments -join ' ') failed with exit code $LASTEXITCODE" }
}

function Invoke-Runtime([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments) {
    $text = (& $script:runtime @Arguments 2>&1 | Out-String)
    if ($LASTEXITCODE -ne 0) { throw "Runtime $($Arguments -join ' ') failed:`n$text" }
    return $text.Trim()
}

function Copy-Contract([string]$Name) {
    $source = Join-Path $repoRoot ('.code-harness\contracts\' + $Name)
    if (-not (Test-Path $source -PathType Leaf)) { throw "Task162 duplicate-symbol contract missing: $Name" }
    Copy-Item $source (Join-Path $fixture ('.code-harness\contracts\' + $Name)) -Force
}

function New-SymbolRef([string]$Path, [string]$Symbol) {
    return [ordered]@{ workspace='current'; path=$Path; symbol=$Symbol }
}

try {
    Push-Location $fixture
    try {
        Invoke-Git init
        Invoke-Git config user.email 'task162-duplicate@example.test'
        Invoke-Git config user.name 'Task 162 Duplicate Symbol'
        Invoke-Git config core.autocrlf false
        Invoke-Git add .
        Invoke-Git commit -m 'base duplicate-symbol fixture'
        $baseHead = (git rev-parse HEAD).Trim()
        $branch = (git branch --show-current).Trim()

        $controllerA = 'module-a/src/main/java/com/acme/UserController.java'
        $serviceA = 'module-a/src/main/java/com/acme/UserService.java'
        $controllerB = 'module-b/src/main/java/com/acme/UserController.java'
        $serviceB = 'module-b/src/main/java/com/acme/UserService.java'

        Write-Utf8NoBom $controllerA @'
package com.acme;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RestController;
@RestController
public class UserController {
    private final UserService userService = new UserService();
    @PostMapping("/a/users")
    public String create() {
        return "a:" + userService.create();
    }
}
'@
        Write-Utf8NoBom $serviceA @'
package com.acme;
public class UserService {
    public String create() {
        return "module-a-updated";
    }
}
'@
        Write-Utf8NoBom $controllerB @'
package com.acme;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RestController;
@RestController
public class UserController {
    private final UserService userService = new UserService();
    @PostMapping("/b/users")
    public String create() {
        return "b:" + userService.create();
    }
}
'@
        Write-Utf8NoBom $serviceB @'
package com.acme;
public class UserService {
    public String create() {
        return "module-b-updated";
    }
}
'@

        New-Item -ItemType Directory -Force '.code-harness\bin', '.code-harness\contracts', '.code-harness\review-rules' | Out-Null
        Copy-Item $runtimeSource '.code-harness\bin\codea-dcep-tools.exe' -Force
        Copy-Item $astSource '.code-harness\bin\ast-grep.exe' -Force
        Copy-Item $versionSource '.code-harness\VERSION' -Force
        Copy-Item $catalogSource '.code-harness\review-rules\spring-v1.yaml' -Force
        foreach ($contract in @(
            'analysis-certify-request.schema.json',
            'change-analysis.schema.json',
            'entrypoint-inventory.schema.json',
            'change-analysis-cert.schema.json',
            'review-unit.schema.json',
            'rule-dispatch.schema.json',
            'finding-proposals.schema.json',
            'certified-findings.schema.json',
            'certified-findings-cert.schema.json'
        )) { Copy-Contract $contract }
        $script:runtime = (Resolve-Path '.code-harness\bin\codea-dcep-tools.exe').Path

        $changed = @(
            [ordered]@{ path=$controllerA; role='Controller'; sources=@('UNSTAGED') },
            [ordered]@{ path=$serviceA; role='Service'; sources=@('UNSTAGED') },
            [ordered]@{ path=$controllerB; role='Controller'; sources=@('UNSTAGED') },
            [ordered]@{ path=$serviceB; role='Service'; sources=@('UNSTAGED') }
        )
        $reviewed = @($changed | ForEach-Object { [ordered]@{ path=$_.path; role=$_.role; reason='CHANGED' } })
        $locations = @(
            [ordered]@{ symbol='UserController.create'; path=$controllerA; role='Controller'; source='FIND_SYMBOL' },
            [ordered]@{ symbol='UserService.create'; path=$serviceA; role='Service'; source='FIND_REFERENCES'; from='UserController.create' },
            [ordered]@{ symbol='UserController.create'; path=$controllerB; role='Controller'; source='FIND_SYMBOL' },
            [ordered]@{ symbol='UserService.create'; path=$serviceB; role='Service'; source='FIND_REFERENCES'; from='UserController.create' }
        )
        $affected = @([ordered]@{
            controller='UserController'; endpoints=@('UserController.create'); impactType='DIRECT_CHANGE'; sourceSymbols=@('UserController.create')
        })

        # A bare duplicate chain must fail closed; it must not confirm both module EntryPoints.
        $negativeRun = 'task162-duplicate-negative'
        $negativeRequestDir = Join-Path $fixture ".code-harness\runs\$negativeRun\requests"
        New-Item -ItemType Directory -Force $negativeRequestDir | Out-Null
        $negativeDraft = [ordered]@{
            reviewScope=[ordered]@{ currentBranch=$branch; baseRef='HEAD'; baseCommit=$baseHead; mergeBase=$baseHead; headCommit=$baseHead; includeWorkingTree=$true }
            changedFiles=$changed
            affectedControllers=$affected
            callChains=@([ordered]@{ entryPoint='UserController.create'; chain=@('UserController.create','UserService.create') })
            symbolLocations=$locations
            resourceRelations=@()
            externalDependencies=@()
            riskAreas=@()
            reviewCoverage=[ordered]@{ status='COMPLETE'; reviewedFiles=$reviewed; unresolvedSymbols=@() }
        }
        Write-Utf8NoBom (Join-Path $negativeRequestDir 'change-analysis-draft.json') ($negativeDraft | ConvertTo-Json -Depth 30)
        $negativeRequest = [ordered]@{ runId=$negativeRun; draftPath=".code-harness/runs/$negativeRun/requests/change-analysis-draft.json"; baseRef='HEAD'; includeWorkingTree=$true; intent=[ordered]@{mode='FULL'} }
        Write-Utf8NoBom (Join-Path $negativeRequestDir 'analysis-certify.json') ($negativeRequest | ConvertTo-Json -Depth 10 -Compress)
        $negativeText = (& $script:runtime analysis certify --input ".code-harness/runs/$negativeRun/requests/analysis-certify.json" 2>&1 | Out-String)
        $negativeExit = $LASTEXITCODE
        if ($negativeExit -eq 0) { throw 'bare duplicate symbol chain unexpectedly certified both module EntryPoints' }
        if ($negativeText -notmatch 'ENTRYPOINT_EVIDENCE_AMBIGUOUS|ENTRYPOINT_COMPLETENESS_INCOMPLETE') {
            throw "bare duplicate chain did not fail with Authority ambiguity/completeness gate:`n$negativeText"
        }
        Write-Output 'TASK162_DUPLICATE_BARE_CHAIN_FAIL_CLOSED PASS'

        # Exact workspace/path/symbol refs distinguish the two otherwise identical chains.
        $runID = 'task162-duplicate-full'
        $requestDir = Join-Path $fixture ".code-harness\runs\$runID\requests"
        New-Item -ItemType Directory -Force $requestDir | Out-Null
        $controllerRefA = New-SymbolRef $controllerA 'UserController.create'
        $serviceRefA = New-SymbolRef $serviceA 'UserService.create'
        $controllerRefB = New-SymbolRef $controllerB 'UserController.create'
        $serviceRefB = New-SymbolRef $serviceB 'UserService.create'
        $chains = @(
            [ordered]@{
                entryPoint='UserController.create'; chain=@('UserController.create','UserService.create');
                entryPointRef=$controllerRefA; chainRefs=@($controllerRefA,$serviceRefA)
            },
            [ordered]@{
                entryPoint='UserController.create'; chain=@('UserController.create','UserService.create');
                entryPointRef=$controllerRefB; chainRefs=@($controllerRefB,$serviceRefB)
            }
        )
        $draft = [ordered]@{
            reviewScope=[ordered]@{ currentBranch=$branch; baseRef='HEAD'; baseCommit=$baseHead; mergeBase=$baseHead; headCommit=$baseHead; includeWorkingTree=$true }
            changedFiles=$changed
            affectedControllers=$affected
            callChains=$chains
            symbolLocations=$locations
            resourceRelations=@()
            externalDependencies=@()
            riskAreas=@()
            reviewCoverage=[ordered]@{ status='COMPLETE'; reviewedFiles=$reviewed; unresolvedSymbols=@() }
        }
        Write-Utf8NoBom (Join-Path $requestDir 'change-analysis-draft.json') ($draft | ConvertTo-Json -Depth 40)
        $certifyRequest = [ordered]@{ runId=$runID; draftPath=".code-harness/runs/$runID/requests/change-analysis-draft.json"; baseRef='HEAD'; includeWorkingTree=$true; intent=[ordered]@{mode='FULL'} }
        Write-Utf8NoBom (Join-Path $requestDir 'analysis-certify.json') ($certifyRequest | ConvertTo-Json -Depth 10 -Compress)
        Invoke-Runtime analysis certify --input ".code-harness/runs/$runID/requests/analysis-certify.json" | Out-Null

        $certified = Get-Content ".code-harness\runs\$runID\analysis\change-analysis.json" -Raw | ConvertFrom-Json
        $serviceLocations = @($certified.symbolLocations | Where-Object { $_.symbol -eq 'UserService.create' })
        if ($serviceLocations.Count -ne 2) { throw "expected two certified UserService.create locations, got $($serviceLocations.Count)" }
        $serviceLocationPaths = @($serviceLocations | ForEach-Object { [string]$_.path })
        foreach ($expected in @($serviceA,$serviceB)) { if ($serviceLocationPaths -notcontains $expected) { throw "missing certified duplicate symbol location $expected" } }
        $certifiedChains = @($certified.callChains | Where-Object { $_.entryPoint -eq 'UserController.create' })
        if ($certifiedChains.Count -ne 2) { throw "expected two certified duplicate call chains, got $($certifiedChains.Count)" }
        $entryRefPaths = @($certifiedChains | ForEach-Object { [string]$_.entryPointRef.path })
        foreach ($expected in @($controllerA,$controllerB)) { if ($entryRefPaths -notcontains $expected) { throw "certified chain lost entryPoint exact ref $expected" } }

        $inventory = Get-Content ".code-harness\runs\$runID\analysis\entrypoint-inventory.json" -Raw | ConvertFrom-Json
        $duplicateEntries = @($inventory.expectedEntrypoints | Where-Object { $_.symbol -eq 'UserController.create' })
        if ($duplicateEntries.Count -ne 2) { throw "expected two same-name EntryPoints in inventory, got $($duplicateEntries.Count)" }
        $entryPaths = @($duplicateEntries | ForEach-Object { [string]$_.path })
        foreach ($expected in @($controllerA,$controllerB)) { if ($entryPaths -notcontains $expected) { throw "EntryPoint inventory lost module path $expected" } }
        Write-Output 'TASK162_DUPLICATE_CERTIFIED_AUTHORITY PASS'

        Invoke-Runtime review units --run-id $runID | Out-Null
        $units = Get-Content ".code-harness\runs\$runID\analysis\review-units.json" -Raw | ConvertFrom-Json
        $branchUnits = @($units.units | Where-Object { $_.entryPoint -eq 'UserController.create' })
        if ($branchUnits.Count -ne 2) { throw "expected two duplicate-symbol ReviewUnits, got $($branchUnits.Count)" }
        $unitA = @($branchUnits | Where-Object { @($_.files.path) -contains $serviceA }) | Select-Object -First 1
        $unitB = @($branchUnits | Where-Object { @($_.files.path) -contains $serviceB }) | Select-Object -First 1
        if ($null -eq $unitA -or $null -eq $unitB) { throw 'duplicate-symbol ReviewUnit did not bind both module paths' }
        if ([string]$unitA.id -eq [string]$unitB.id) { throw 'duplicate-symbol ReviewUnits collapsed to one ID' }
        if (@($unitA.files.path) -contains $serviceB -or @($unitA.files.path) -contains $controllerB) { throw 'module-a ReviewUnit leaked module-b files' }
        if (@($unitB.files.path) -contains $serviceA -or @($unitB.files.path) -contains $controllerA) { throw 'module-b ReviewUnit leaked module-a files' }
        foreach ($expected in @($controllerA,$serviceA)) { if (@($unitA.files.path) -notcontains $expected) { throw "module-a ReviewUnit missing $expected" } }
        foreach ($expected in @($controllerB,$serviceB)) { if (@($unitB.files.path) -notcontains $expected) { throw "module-b ReviewUnit missing $expected" } }
        Write-Output 'TASK162_DUPLICATE_REVIEW_UNIT_BINDING PASS'

        Invoke-Runtime review dispatch --run-id $runID | Out-Null
        $dispatch = Get-Content ".code-harness\runs\$runID\analysis\rule-dispatch.json" -Raw | ConvertFrom-Json
        foreach ($unitId in @([string]$unitA.id,[string]$unitB.id)) {
            if (@($dispatch.dispatches | Where-Object { $_.reviewUnitId -eq $unitId -and $_.ruleId -eq 'SPRING-TX-001' }).Count -lt 1) {
                throw "SPRING-TX-001 not dispatched to duplicate-symbol ReviewUnit $unitId"
            }
        }

        $proposals = @(
            [ordered]@{
                proposalId='P-DUP-MODULE-A'; reviewUnitId=[string]$unitA.id; ruleId='SPRING-TX-001'; category='PRODUCTION_CODE'; severity='high';
                anchor=[ordered]@{kind='SYMBOL'; path=$serviceA; symbol='UserService.create'};
                evidenceRefs=@(
                    [ordered]@{kind='CHAIN'; value='UserService.create'},
                    [ordered]@{kind='SYMBOL'; value='UserService.create'; path=$serviceA}
                );
                problem='module-a duplicate symbol finding'; impact='module-a impact'; recommendation='verify module-a transaction path'; needsTest=$true; introducedByChange=$false; confidence=0.95
            },
            [ordered]@{
                proposalId='P-DUP-MODULE-B'; reviewUnitId=[string]$unitB.id; ruleId='SPRING-TX-001'; category='PRODUCTION_CODE'; severity='high';
                anchor=[ordered]@{kind='SYMBOL'; path=$serviceB; symbol='UserService.create'};
                evidenceRefs=@(
                    [ordered]@{kind='CHAIN'; value='UserService.create'},
                    [ordered]@{kind='SYMBOL'; value='UserService.create'; path=$serviceB}
                );
                problem='module-b duplicate symbol finding'; impact='module-b impact'; recommendation='verify module-b transaction path'; needsTest=$true; introducedByChange=$false; confidence=0.95
            }
        )
        Write-Utf8NoBom (Join-Path $requestDir 'finding-proposals.json') (ConvertTo-Json -InputObject $proposals -Depth 30)
        $findingRequest = [ordered]@{ runId=$runID; proposalsPath=".code-harness/runs/$runID/requests/finding-proposals.json" }
        Write-Utf8NoBom (Join-Path $requestDir 'finding-certify-request.json') ($findingRequest | ConvertTo-Json -Compress)
        Invoke-Runtime review certify-findings --input ".code-harness/runs/$runID/requests/finding-certify-request.json" | Out-Null
        $certifiedFindings = Get-Content ".code-harness\runs\$runID\analysis\certified-findings.json" -Raw | ConvertFrom-Json
        if (@($certifiedFindings.findings).Count -ne 2) { throw "expected two certified duplicate-symbol findings, got $(@($certifiedFindings.findings).Count)" }
        $anchorPaths = @($certifiedFindings.findings | ForEach-Object { [string]$_.anchor.path })
        foreach ($expected in @($serviceA,$serviceB)) { if ($anchorPaths -notcontains $expected) { throw "Finding Anchor did not resolve exact module path $expected" } }
        Write-Output 'TASK162_DUPLICATE_FINDING_ANCHOR PASS'

        $reportRequest = [ordered]@{
            runId=$runID; harnessVersion='runtime-owned'; baseRef='HEAD'; head=$baseHead; result='PASSED'; mode='FULL';
            reviewScope=[ordered]@{ changedFiles=@($changed | ForEach-Object {$_.path}) };
            reviewCoverage=[ordered]@{
                reviewedFiles=@($changed | ForEach-Object {$_.path});
                callChains=@(
                    [ordered]@{entryPoint='UserController.create'; chain=@('UserController.create','UserService.create')},
                    [ordered]@{entryPoint='UserController.create'; chain=@('UserController.create','UserService.create')}
                );
                externalDependencies=@(); unresolved=@(); missingReviewedFiles=@(); runtimeErrors=@(); status='COMPLETE'
            };
            findings=@()
        }
        Write-Utf8NoBom (Join-Path $requestDir 'review-report.json') ($reportRequest | ConvertTo-Json -Depth 30)
        Invoke-Runtime report review --input ".code-harness/runs/$runID/requests/review-report.json" | Out-Null
        $reportPath = ".code-harness\runs\$runID\review.md"
        if (-not (Test-Path $reportPath -PathType Leaf)) { throw 'duplicate-symbol FULL review.md missing' }
        $report = Get-Content $reportPath -Raw
        foreach ($expected in @('module-a duplicate symbol finding','module-b duplicate symbol finding',$serviceA,$serviceB)) {
            if ($report -notmatch [regex]::Escape($expected)) { throw "FULL review.md missing duplicate-symbol authority evidence: $expected" }
        }
        Write-Output 'TASK162_DUPLICATE_FULL_REVIEW PASS'
        Write-Output 'TASK162_DUPLICATE_SYMBOL_AUTHORITY_E2E PASS'
    }
    finally { Pop-Location }
}
finally {
    Remove-Item $fixture -Recurse -Force -ErrorAction SilentlyContinue
}
