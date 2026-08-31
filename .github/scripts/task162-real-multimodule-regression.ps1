$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$runtimeSource = Join-Path $repoRoot '.code-harness\bin\codea-dcep-tools.exe'
$astSource = Join-Path $repoRoot '.code-harness\bin\ast-grep.exe'
$versionSource = Join-Path $repoRoot '.code-harness\VERSION'
$catalogSource = Join-Path $repoRoot '.code-harness\review-rules\spring-v1.yaml'
$fixtureSource = Join-Path $repoRoot '.code-harness\tools-runtime\testdata\multi-module-fixture'

foreach ($required in @($runtimeSource, $astSource, $versionSource, $catalogSource, $fixtureSource)) {
    if (-not (Test-Path $required)) { throw "Task162 missing required path: $required" }
}

$fixture = Join-Path $env:RUNNER_TEMP ("task162-multimodule-" + [guid]::NewGuid().ToString('N'))
Copy-Item -Recurse $fixtureSource $fixture

function Write-Utf8NoBom([string]$Path, [string]$Content) {
    $target = $Path
    if (-not [System.IO.Path]::IsPathRooted($target)) {
        $target = Join-Path (Get-Location).Path $target
    }
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
    if (-not (Test-Path $source -PathType Leaf)) { throw "Task162 contract missing: $Name" }
    Copy-Item $source (Join-Path $fixture ('.code-harness\contracts\' + $Name)) -Force
}

try {
    Push-Location $fixture
    try {
        Invoke-Git init
        Invoke-Git config user.email 'task162@example.test'
        Invoke-Git config user.name 'Task 162 Multi Module'
        Invoke-Git config core.autocrlf false
        Invoke-Git add .
        Invoke-Git commit -m 'base multi-module fixture'
        $baseHead = (git rev-parse HEAD).Trim()
        $branch = (git branch --show-current).Trim()

        # Six real module changes: Controller, Service, Mapper Java/XML, YAML, Test.
        Write-Utf8NoBom 'order-api/src/main/java/com/acme/order/OrderController.java' @'
package com.acme.order;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RestController;
@RestController
public class OrderController {
    private final OrderService orderService = new OrderService();
    @PostMapping("/orders")
    public String createOrder() {
        return "api:" + orderService.createOrder();
    }
}
'@
        Write-Utf8NoBom 'order-service/src/main/java/com/acme/order/OrderService.java' @'
package com.acme.order;
public class OrderService {
    private final OrderMapper orderMapper = new OrderMapper();
    public String createOrder() {
        return "service:" + orderMapper.insertOrder();
    }
}
'@
        Write-Utf8NoBom 'order-dao/src/main/java/com/acme/order/OrderMapper.java' @'
package com.acme.order;
public class OrderMapper {
    public String insertOrder() {
        return "inserted";
    }
}
'@
        Write-Utf8NoBom 'order-dao/src/main/resources/mapper/OrderMapper.xml' @'
<mapper namespace="com.acme.order.OrderMapper">
  <insert id="insertOrder">insert into orders(id, status) values (1, 'NEW')</insert>
</mapper>
'@
        Write-Utf8NoBom 'order-service/src/main/resources/application.yml' @'
spring:
  application:
    name: order-service
feature:
  order:
    enabled: false
'@
        Write-Utf8NoBom 'order-service/src/test/java/com/acme/order/OrderServiceTest.java' @'
package com.acme.order;
public class OrderServiceTest {
    public void createOrderTest() {
        String result = new OrderService().createOrder();
        if (result == null) throw new AssertionError("result");
    }
}
'@

        New-Item -ItemType Directory -Force '.code-harness\bin', '.code-harness\contracts', '.code-harness\review-rules' | Out-Null
        Copy-Item $runtimeSource '.code-harness\bin\codea-dcep-tools.exe' -Force
        Copy-Item $astSource '.code-harness\bin\ast-grep.exe' -Force
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

        $script:runtime = (Resolve-Path '.code-harness\bin\codea-dcep-tools.exe').Path

        # Real navigation must retain the module prefixes.
        $navController = Invoke-Runtime nav find-symbol --symbol OrderController --scope order-api/src/main/java
        if ($navController -notmatch 'order-api[/\\]src[/\\]main[/\\]java') { throw "module Controller navigation lost path prefix: $navController" }
        $navService = Invoke-Runtime nav find-symbol --symbol OrderService --scope order-service/src/main/java
        if ($navService -notmatch 'order-service[/\\]src[/\\]main[/\\]java') { throw "module Service navigation lost path prefix: $navService" }
        $navMapper = Invoke-Runtime nav find-symbol --symbol OrderMapper --scope order-dao/src/main/java
        if ($navMapper -notmatch 'order-dao[/\\]src[/\\]main[/\\]java') { throw "module Mapper navigation lost path prefix: $navMapper" }
        $callers = Invoke-Runtime nav find-callers --symbol createOrder --scope order-api/src/main/java
        if ($callers -notmatch 'order-api[/\\]src[/\\]main[/\\]java') { throw "module caller navigation lost path prefix: $callers" }
        Write-Output 'TASK162_CODE_NAVIGATION PASS'

        $runID = 'task162-multimodule'
        $requestDir = Join-Path $fixture ".code-harness\runs\$runID\requests"
        New-Item -ItemType Directory -Force $requestDir | Out-Null
        $changed = @(
            [ordered]@{ path='order-api/src/main/java/com/acme/order/OrderController.java'; role='Controller'; sources=@('UNSTAGED') },
            [ordered]@{ path='order-service/src/main/java/com/acme/order/OrderService.java'; role='Service'; sources=@('UNSTAGED') },
            [ordered]@{ path='order-dao/src/main/java/com/acme/order/OrderMapper.java'; role='Mapper'; sources=@('UNSTAGED') },
            [ordered]@{ path='order-dao/src/main/resources/mapper/OrderMapper.xml'; role='MapperXml'; sources=@('UNSTAGED') },
            [ordered]@{ path='order-service/src/main/resources/application.yml'; role='YamlConfig'; sources=@('UNSTAGED') },
            [ordered]@{ path='order-service/src/test/java/com/acme/order/OrderServiceTest.java'; role='Test'; sources=@('UNSTAGED') }
        )
        $reviewed = @($changed | ForEach-Object { [ordered]@{ path=$_.path; role=$_.role; reason='CHANGED' } })
        $draft = [ordered]@{
            reviewScope = [ordered]@{
                currentBranch=$branch; baseRef='HEAD'; baseCommit=$baseHead; mergeBase=$baseHead; headCommit=$baseHead; includeWorkingTree=$true
            }
            changedFiles = $changed
            affectedControllers = @([ordered]@{
                controller='OrderController'; endpoints=@('OrderController.createOrder'); impactType='DIRECT_CHANGE'; sourceSymbols=@('OrderController.createOrder')
            })
            callChains = @([ordered]@{
                entryPoint='OrderController.createOrder'; chain=@('OrderController.createOrder','OrderService.createOrder','OrderMapper.insertOrder')
            })
            symbolLocations = @(
                [ordered]@{ symbol='OrderController.createOrder'; path='order-api/src/main/java/com/acme/order/OrderController.java'; role='Controller'; source='FIND_SYMBOL' },
                [ordered]@{ symbol='OrderService.createOrder'; path='order-service/src/main/java/com/acme/order/OrderService.java'; role='Service'; source='FIND_REFERENCES'; from='OrderController.createOrder' },
                [ordered]@{ symbol='OrderMapper.insertOrder'; path='order-dao/src/main/java/com/acme/order/OrderMapper.java'; role='Mapper'; source='FIND_REFERENCES'; from='OrderService.createOrder' }
            )
            resourceRelations = @(
                [ordered]@{ path='order-dao/src/main/resources/mapper/OrderMapper.xml'; role='MapperXml'; resource='OrderMapper.xml#insertOrder'; fromSymbol='OrderMapper.insertOrder'; fromKind='METHOD'; source='MAPPER_STATEMENT'; evidence='statement id insertOrder matches verified Mapper method' },
                [ordered]@{ path='order-service/src/main/resources/application.yml'; role='YamlConfig'; resource='feature.order.enabled'; fromSymbol='OrderService.createOrder'; fromKind='METHOD'; source='CONFIG_REFERENCE'; evidence='changed feature switch reviewed with selected service path' }
            )
            externalDependencies = @()
            riskAreas = @()
            reviewCoverage = [ordered]@{ status='COMPLETE'; reviewedFiles=$reviewed; unresolvedSymbols=@() }
        }
        $draftPath = Join-Path $requestDir 'change-analysis-draft.json'
        Write-Utf8NoBom $draftPath ($draft | ConvertTo-Json -Depth 30)
        $certifyRequest = [ordered]@{
            runId=$runID; draftPath=".code-harness/runs/$runID/requests/change-analysis-draft.json"; baseRef='HEAD'; includeWorkingTree=$true; intent=[ordered]@{mode='FULL'}
        }
        Write-Utf8NoBom (Join-Path $requestDir 'analysis-certify.json') ($certifyRequest | ConvertTo-Json -Depth 10 -Compress)
        Invoke-Runtime analysis certify --input ".code-harness/runs/$runID/requests/analysis-certify.json" | Out-Null

        $certifiedAnalysis = Get-Content ".code-harness\runs\$runID\analysis\change-analysis.json" -Raw | ConvertFrom-Json
        $actualPaths = @($certifiedAnalysis.changedFiles | ForEach-Object { [string]$_.path })
        foreach ($expected in @($changed | ForEach-Object { $_.path })) {
            if ($actualPaths -notcontains $expected) { throw "Certified ChangeAnalysis missing module path $expected" }
        }
        $inventory = Get-Content ".code-harness\runs\$runID\analysis\entrypoint-inventory.json" -Raw | ConvertFrom-Json
        $entry = @($inventory.expectedEntrypoints | Where-Object { $_.symbol -eq 'OrderController.createOrder' }) | Select-Object -First 1
        if ($null -eq $entry -or [string]$entry.path -ne 'order-api/src/main/java/com/acme/order/OrderController.java') { throw 'module Controller EntryPoint inventory mismatch' }
        Write-Output 'TASK162_CERTIFIED_ANALYSIS_AND_ENTRYPOINT PASS'

        Invoke-Runtime review units --run-id $runID | Out-Null
        $units = Get-Content ".code-harness\runs\$runID\analysis\review-units.json" -Raw | ConvertFrom-Json
        $unitPaths = @($units.units.files.path)
        foreach ($expected in @($changed | ForEach-Object { $_.path })) {
            if ($unitPaths -notcontains $expected) { throw "ReviewUnit missing module path $expected" }
        }
        $configUnit = @($units.units | Where-Object { @($_.files.path) -contains 'order-service/src/main/resources/application.yml' }) | Select-Object -First 1
        if ($null -eq $configUnit) { throw 'module YamlConfig ReviewUnit missing' }
        $configUnitId = [string]$configUnit.id
        $configHunk = @($configUnit.changedHunks | Where-Object { $_.path -eq 'order-service/src/main/resources/application.yml' }) | Select-Object -First 1
        if ($null -eq $configHunk) { throw 'module YamlConfig changed hunk missing' }
        $configLine = [int]$configHunk.newStart
        Write-Output 'TASK162_REVIEW_UNIT PASS'

        Invoke-Runtime review dispatch --run-id $runID | Out-Null
        $dispatch = Get-Content ".code-harness\runs\$runID\analysis\rule-dispatch.json" -Raw | ConvertFrom-Json
        if (@($dispatch.dispatches | Where-Object { $_.ruleId -eq 'SPRING-CONFIG-001' }).Count -lt 1) { throw 'module YamlConfig did not dispatch SPRING-CONFIG-001' }
        if (@($dispatch.dispatches | Where-Object { $_.ruleId -eq 'MYBATIS-CONTRACT-001' }).Count -lt 1) { throw 'module Mapper Java/XML did not dispatch MYBATIS-CONTRACT-001' }
        if (@($dispatch.dispatches | Where-Object { $_.ruleId -eq 'TEST-VALIDITY-001' }).Count -lt 1) { throw 'module test did not dispatch TEST-VALIDITY-001' }
        Write-Output 'TASK162_RULE_DISPATCH PASS'

        $proposal = @([ordered]@{
            proposalId='P-MODULE-CONFIG'; reviewUnitId=$configUnitId; ruleId='SPRING-CONFIG-001'; category='PRODUCTION_CODE'; severity='high';
            anchor=[ordered]@{kind='LINE'; path='order-service/src/main/resources/application.yml'; line=$configLine};
            evidenceRefs=@([ordered]@{kind='CHANGED_RANGE'; path='order-service/src/main/resources/application.yml'; startLine=$configLine; endLine=$configLine});
            problem='模块配置开关被关闭'; impact='订单能力可能被禁用'; recommendation='确认发布意图并恢复正确开关'; needsTest=$true; introducedByChange=$true; confidence=0.95
        })
        Write-Utf8NoBom (Join-Path $requestDir 'finding-proposals.json') ($proposal | ConvertTo-Json -Depth 20)
        $findingRequest = [ordered]@{runId=$runID; proposalsPath=".code-harness/runs/$runID/requests/finding-proposals.json"}
        Write-Utf8NoBom (Join-Path $requestDir 'finding-certify-request.json') ($findingRequest | ConvertTo-Json -Compress)
        Invoke-Runtime review certify-findings --input ".code-harness/runs/$runID/requests/finding-certify-request.json" | Out-Null
        $certifiedFindings = Get-Content ".code-harness\runs\$runID\analysis\certified-findings.json" -Raw | ConvertFrom-Json
        if (@($certifiedFindings.findings).Count -ne 1) { throw 'module Finding certification failed' }
        if ([string]$certifiedFindings.findings[0].anchor.path -ne 'order-service/src/main/resources/application.yml') { throw 'Finding anchor lost module prefix' }
        Write-Output 'TASK162_FINDING_CERTIFICATION PASS'

        $reportRequest = [ordered]@{
            runId=$runID; harnessVersion='runtime-owned'; baseRef='HEAD'; head=$baseHead; result='PASSED'; mode='FULL';
            reviewScope=[ordered]@{changedFiles=@($changed | ForEach-Object {$_.path})};
            reviewCoverage=[ordered]@{reviewedFiles=@($changed | ForEach-Object {$_.path}); callChains=@('OrderController.createOrder -> OrderService.createOrder -> OrderMapper.insertOrder'); externalDependencies=@(); unresolved=@(); missingReviewedFiles=@(); runtimeErrors=@(); status='COMPLETE'};
            findings=@()
        }
        Write-Utf8NoBom (Join-Path $requestDir 'review-report.json') ($reportRequest | ConvertTo-Json -Depth 20)
        Invoke-Runtime report review --input ".code-harness/runs/$runID/requests/review-report.json" | Out-Null
        $reportPath = ".code-harness\runs\$runID\review.md"
        if (-not (Test-Path $reportPath -PathType Leaf)) { throw 'Task162 review.md missing' }
        $report = Get-Content $reportPath -Raw
        if ($report -notmatch '模块配置开关被关闭') { throw 'Task162 review.md did not consume certified module finding' }
        if ($report -notmatch 'order-service/src/main/resources/application.yml') { throw 'Task162 review.md lost module path' }
        Write-Output 'TASK162_REVIEW_MD PASS'
        Write-Output 'TASK162_MULTI_MODULE_E2E PASS'
    }
    finally { Pop-Location }
}
finally {
    Remove-Item $fixture -Recurse -Force -ErrorAction SilentlyContinue
}
