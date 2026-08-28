$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$runtimeSource = Join-Path $repoRoot '.code-harness\bin\codea-harness-tools.exe'
$versionSource = Join-Path $repoRoot '.code-harness\VERSION'
$catalogSource = Join-Path $repoRoot '.code-harness\review-rules\spring-v1.yaml'

foreach ($required in @($runtimeSource, $versionSource, $catalogSource)) {
    if (-not (Test-Path $required -PathType Leaf)) { throw "Task160 missing required file: $required" }
}

$fixture = Join-Path $env:RUNNER_TEMP ("task160-review-precision-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force $fixture | Out-Null
$dependencyName = "company-framework-" + [guid]::NewGuid().ToString('N')
$dependencyRoot = Join-Path (Split-Path -Parent $fixture) $dependencyName

function Write-Utf8NoBom([string]$Path, [string]$Content) {
    $parent = Split-Path -Parent $Path
    if ($parent) { New-Item -ItemType Directory -Force $parent | Out-Null }
    [System.IO.File]::WriteAllText($Path, $Content, [System.Text.UTF8Encoding]::new($false))
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
    if (-not (Test-Path $source -PathType Leaf)) { throw "Task160 contract missing: $Name" }
    Copy-Item $source (Join-Path $fixture ('.code-harness\contracts\' + $Name)) -Force
}

try {
    Push-Location $fixture
    try {
        Invoke-Git init
        Invoke-Git config user.email 'task160@example.test'
        Invoke-Git config user.name 'Task 160 Real Regression'

        Write-Utf8NoBom (Join-Path $fixture 'pom.xml') @'
<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.acme</groupId>
  <artifactId>task160-fixture</artifactId>
  <version>1.0.0</version>
  <dependencies>
    <dependency><groupId>com.company</groupId><artifactId>company-framework</artifactId><version>2.3.1</version></dependency>
  </dependencies>
</project>
'@
        $configPath = Join-Path $fixture 'src/main/resources/application.yml'
        Write-Utf8NoBom $configPath @'
spring:
  datasource:
    hikari:
      connection-timeout: 30000
'@
        Write-Utf8NoBom (Join-Path $dependencyRoot 'pom.xml') @'
<project><modelVersion>4.0.0</modelVersion><groupId>com.company</groupId><artifactId>company-framework</artifactId><version>2.3.1</version></project>
'@
        Write-Utf8NoBom (Join-Path $dependencyRoot 'src/main/java/com/company/framework/SharedPolicy.java') @'
package com.company.framework; public class SharedPolicy { public void check() {} }
'@
        Invoke-Git add .
        Invoke-Git commit -m 'base Spring Maven fixture'
        $baseHead = (git rev-parse HEAD).Trim()
        $branch = (git branch --show-current).Trim()

        Write-Utf8NoBom $configPath @'
spring:
  datasource:
    hikari:
      connection-timeout: 1
'@

        New-Item -ItemType Directory -Force '.code-harness\bin' | Out-Null
        New-Item -ItemType Directory -Force '.code-harness\contracts' | Out-Null
        New-Item -ItemType Directory -Force '.code-harness\review-rules' | Out-Null
        Copy-Item $runtimeSource '.code-harness\bin\codea-harness-tools.exe' -Force
        Copy-Item $versionSource '.code-harness\VERSION' -Force
        Copy-Item $catalogSource '.code-harness\review-rules\spring-v1.yaml' -Force
        foreach ($contract in @(
            'change-analysis.schema.json',
            'entrypoint-inventory.schema.json',
            'change-analysis-cert.schema.json',
            'review-unit.schema.json',
            'rule-dispatch.schema.json',
            'finding-proposals.schema.json',
            'certified-findings.schema.json',
            'certified-findings-cert.schema.json'
        )) { Copy-Contract $contract }

        Write-Utf8NoBom (Join-Path $fixture '.code-harness/harness.yaml') @"
version: 2
workspaceDependencies:
  - id: company-framework
    root: ../$dependencyName
    maven:
      groupId: com.company
      artifactId: company-framework
    mode: READ_ONLY
"@
        $script:runtime = (Resolve-Path '.code-harness\bin\codea-harness-tools.exe').Path
        $runID = 'task160-real'
        $requestDir = Join-Path $fixture ".code-harness\runs\$runID\requests"
        New-Item -ItemType Directory -Force $requestDir | Out-Null

        $draft = [ordered]@{
            reviewScope = [ordered]@{
                currentBranch = $branch
                baseRef = 'HEAD'
                baseCommit = $baseHead
                mergeBase = $baseHead
                headCommit = $baseHead
                includeWorkingTree = $true
            }
            changedFiles = @([ordered]@{ path = 'src/main/resources/application.yml'; role = 'YamlConfig'; sources = @('UNSTAGED') })
            affectedControllers = @()
            callChains = @()
            symbolLocations = @([ordered]@{ workspace='company-framework'; symbol='SharedPolicy.check'; path='src/main/java/com/company/framework/SharedPolicy.java'; role='Service'; source='FIND_SYMBOL' })
            resourceRelations = @()
            externalDependencies = @('company-framework')
            riskAreas = @()
            reviewCoverage = [ordered]@{
                status = 'COMPLETE'
                reviewedFiles = @([ordered]@{ path = 'src/main/resources/application.yml'; role = 'YamlConfig'; reason = 'CHANGED' })
                unresolvedSymbols = @()
            }
        }
        $draftPath = Join-Path $requestDir 'change-analysis-draft.json'
        Write-Utf8NoBom $draftPath ($draft | ConvertTo-Json -Depth 20)
        $certifyRequest = [ordered]@{
            runId = $runID
            draftPath = ".code-harness/runs/$runID/requests/change-analysis-draft.json"
            baseRef = 'HEAD'
            includeWorkingTree = $true
            intent = [ordered]@{ mode = 'FULL' }
        }
        $analysisRequestPath = Join-Path $requestDir 'analysis-certify.json'
        Write-Utf8NoBom $analysisRequestPath ($certifyRequest | ConvertTo-Json -Depth 10 -Compress)
        Invoke-Runtime analysis certify --input ".code-harness/runs/$runID/requests/analysis-certify.json" | Out-Null

        Invoke-Runtime review units --run-id $runID | Out-Null
        $unitsPath = Join-Path $fixture ".code-harness\runs\$runID\analysis\review-units.json"
        $units = Get-Content $unitsPath -Raw | ConvertFrom-Json
        $unit = @($units.units | Where-Object { @($_.files.path) -contains 'src/main/resources/application.yml' }) | Select-Object -First 1
        if ($null -eq $unit) { throw 'Task160 ReviewUnit did not include changed application.yml' }
        $unitId = [string]$unit.id
        $hunk = @($unit.changedHunks | Where-Object { $_.path -eq 'src/main/resources/application.yml' }) | Select-Object -First 1
        if ($null -eq $hunk) { throw 'Task160 ReviewUnit missing config changed hunk' }
        $changedLine = [int]$hunk.newStart
        Write-Output 'TASK160_REVIEW_UNIT_VERIFIED PASS'

        Invoke-Runtime review dispatch --run-id $runID | Out-Null
        $dispatchPath = Join-Path $fixture ".code-harness\runs\$runID\analysis\rule-dispatch.json"
        $dispatch = Get-Content $dispatchPath -Raw | ConvertFrom-Json
        $configDispatch = @($dispatch.dispatches | Where-Object { $_.reviewUnitId -eq $unitId -and $_.ruleId -eq 'SPRING-CONFIG-001' })
        if ($configDispatch.Count -ne 1) { throw "Task160 expected exactly one SPRING-CONFIG-001 dispatch for $unitId" }
        Write-Output 'TASK160_RULE_DISPATCH_VERIFIED PASS'

        $changedEvidence = @([ordered]@{ kind='CHANGED_RANGE'; path='src/main/resources/application.yml'; startLine=$changedLine; endLine=$changedLine })
        $proposals = @(
            [ordered]@{ proposalId='P-VALID-A'; reviewUnitId=$unitId; ruleId='SPRING-CONFIG-001'; category='PRODUCTION_CODE'; severity='high'; anchor=[ordered]@{kind='LINE'; path='src/main/resources/application.yml'; line=$changedLine}; evidenceRefs=$changedEvidence; problem='连接超时被改为极小值'; impact='数据库请求可能大量超时'; recommendation='恢复经过容量验证的 timeout'; needsTest=$true; introducedByChange=$true; confidence=0.95 },
            [ordered]@{ proposalId='P-VALID-B'; reviewUnitId=$unitId; ruleId='SPRING-CONFIG-001'; category='PRODUCTION_CODE'; severity='high'; anchor=[ordered]@{kind='LINE'; path='src/main/resources/application.yml'; line=$changedLine}; evidenceRefs=$changedEvidence; problem='同一配置风险的另一种描述'; impact='另一种影响措辞'; recommendation='另一种修复措辞'; needsTest=$true; introducedByChange=$true; confidence=0.95 },
            [ordered]@{ proposalId='P-BAD-LINE'; reviewUnitId=$unitId; ruleId='SPRING-CONFIG-001'; category='PRODUCTION_CODE'; severity='high'; anchor=[ordered]@{kind='LINE'; path='src/main/resources/application.yml'; line=999}; evidenceRefs=$changedEvidence; problem='伪造行号'; impact='无'; recommendation='无'; needsTest=$false; introducedByChange=$false; confidence=0.8 },
            [ordered]@{ proposalId='P-INVENTED-SYMBOL'; reviewUnitId=$unitId; ruleId='SPRING-CONFIG-001'; category='PRODUCTION_CODE'; severity='high'; anchor=[ordered]@{kind='SYMBOL'; symbol='Invented.missing'}; evidenceRefs=$changedEvidence; problem='伪造 symbol'; impact='无'; recommendation='无'; needsTest=$false; introducedByChange=$false; confidence=0.8 },
            [ordered]@{ proposalId='P-DEPENDENCY'; reviewUnitId=$unitId; ruleId='SPRING-CONFIG-001'; category='PRODUCTION_CODE'; severity='high'; anchor=[ordered]@{kind='FILE'; path='src/main/java/com/company/framework/SharedPolicy.java'}; evidenceRefs=$changedEvidence; problem='scope 外 dependency finding'; impact='无'; recommendation='无'; needsTest=$false; introducedByChange=$false; confidence=0.8 }
        )
        $proposalPath = Join-Path $requestDir 'finding-proposals.json'
        Write-Utf8NoBom $proposalPath ($proposals | ConvertTo-Json -Depth 20)
        $findingRequest = [ordered]@{ runId=$runID; proposalsPath=".code-harness/runs/$runID/requests/finding-proposals.json" }
        $findingRequestPath = Join-Path $requestDir 'finding-certify-request.json'
        Write-Utf8NoBom $findingRequestPath ($findingRequest | ConvertTo-Json -Compress)
        $certifyText = Invoke-Runtime review certify-findings --input ".code-harness/runs/$runID/requests/finding-certify-request.json"
        $certify = $certifyText | ConvertFrom-Json
        $rejectedIds = @($certify.rejections | ForEach-Object { [string]$_.proposalId })
        foreach ($requiredRejected in @('P-BAD-LINE','P-INVENTED-SYMBOL','P-DEPENDENCY')) {
            if ($rejectedIds -notcontains $requiredRejected) { throw "Task160 expected rejected proposal $requiredRejected; got $($rejectedIds -join ', ')" }
        }
        Write-Output 'TASK160_INVALID_LINE_REJECTED PASS'
        Write-Output 'TASK160_INVENTED_SYMBOL_REJECTED PASS'
        $dependencyRejection = @($certify.rejections | Where-Object { $_.proposalId -eq 'P-DEPENDENCY' }) | Select-Object -First 1
        if ($null -eq $dependencyRejection -or [string]$dependencyRejection.code -ne 'FINDING_DEPENDENCY_SCOPE_FORBIDDEN') {
            $actualCode = if ($null -eq $dependencyRejection) { '<missing>' } else { [string]$dependencyRejection.code }
            throw "Task160 P-DEPENDENCY expected FINDING_DEPENDENCY_SCOPE_FORBIDDEN, got $actualCode"
        }
        Write-Output 'TASK160_DEPENDENCY_SCOPE_REJECTED PASS'

        $certifiedPath = Join-Path $fixture ".code-harness\runs\$runID\analysis\certified-findings.json"
        $certified = Get-Content $certifiedPath -Raw | ConvertFrom-Json
        if (@($certified.findings).Count -ne 1) { throw "Task160 semantic duplicate did not collapse: findingCount=$(@($certified.findings).Count)" }
        Write-Output 'TASK160_DUPLICATE_FINDING_COLLAPSED PASS'

        $reportRequest = [ordered]@{
            runId = $runID
            harnessVersion = 'runtime-owned'
            baseRef = 'HEAD'
            head = $baseHead
            result = 'PASSED'
            mode = 'FULL'
            reviewScope = [ordered]@{ changedFiles = @('src/main/resources/application.yml') }
            reviewCoverage = [ordered]@{
                reviewedFiles = @('src/main/resources/application.yml')
                callChains = @()
                externalDependencies = @('company-framework')
                unresolved = @()
                missingReviewedFiles = @()
                runtimeErrors = @()
                status = 'COMPLETE'
            }
            findings = @()
        }
        $reportRequestPath = Join-Path $requestDir 'review-report.json'
        Write-Utf8NoBom $reportRequestPath ($reportRequest | ConvertTo-Json -Depth 20)
        Invoke-Runtime report review --input ".code-harness/runs/$runID/requests/review-report.json" | Out-Null
        $reportPath = Join-Path $fixture ".code-harness\runs\$runID\review.md"
        if (-not (Test-Path $reportPath -PathType Leaf)) { throw 'Task160 formal review.md was not written' }
        $report = Get-Content $reportPath -Raw
        if ($report -notmatch '连接超时被改为极小值') { throw "Task160 review.md did not consume Certified Finding:`n$report" }
        Write-Output 'TASK160_CERTIFIED_FINDING_REPORT PASS'
        Write-Output 'TASK160_REVIEW_PRECISION_REGRESSION PASS'
    }
    finally { Pop-Location }
}
finally {
    Remove-Item $fixture -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item $dependencyRoot -Recurse -Force -ErrorAction SilentlyContinue
}
