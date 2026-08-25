$ErrorActionPreference = 'Stop'

# TASK5_REAL_DUAL_PROJECT_BUSINESS_REGRESSION
# Acceptance-only driver. It must use the built Controlled Runtime and the pinned real ast-grep.exe.
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$runtimeSource = Join-Path $repoRoot '.code-harness/bin/codea-harness-tools.exe'
$astGrepSource = Join-Path $repoRoot '.code-harness/bin/ast-grep.exe'
if (-not (Test-Path $runtimeSource)) { throw "Controlled Runtime missing: $runtimeSource" }
if (-not (Test-Path $astGrepSource)) { throw "pinned ast-grep.exe missing: $astGrepSource" }

function Write-Utf8NoBom([string]$Path, [string]$Content) {
    New-Item -ItemType Directory -Force (Split-Path $Path) | Out-Null
    [IO.File]::WriteAllText($Path, $Content, [Text.UTF8Encoding]::new($false))
}

function Invoke-RuntimeJson([string]$Current, [string[]]$Arguments) {
    $runtime = Join-Path $Current '.code-harness/bin/codea-harness-tools.exe'
    $stdout = Join-Path $env:RUNNER_TEMP ('task5-json-' + [guid]::NewGuid().ToString() + '.out')
    $stderr = Join-Path $env:RUNNER_TEMP ('task5-json-' + [guid]::NewGuid().ToString() + '.err')
    $p = Start-Process -FilePath $runtime -ArgumentList $Arguments -WorkingDirectory $Current -Wait -PassThru -NoNewWindow -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    $out = Get-Content $stdout -Raw -ErrorAction SilentlyContinue
    $err = Get-Content $stderr -Raw -ErrorAction SilentlyContinue
    if ($p.ExitCode -ne 0) {
        throw "runtime command failed: $($Arguments -join ' '); exit=$($p.ExitCode); stderr=$err; stdout=$out"
    }
    try { return ($out | ConvertFrom-Json) }
    catch { throw "runtime returned invalid JSON: $($Arguments -join ' '); stdout=$out; stderr=$err" }
}

function Invoke-RuntimeFailure([string]$Current, [string[]]$Arguments, [string]$ExpectedCode) {
    $runtime = Join-Path $Current '.code-harness/bin/codea-harness-tools.exe'
    $stdout = Join-Path $env:RUNNER_TEMP ('task5-fail-' + [guid]::NewGuid().ToString() + '.out')
    $stderr = Join-Path $env:RUNNER_TEMP ('task5-fail-' + [guid]::NewGuid().ToString() + '.err')
    $p = Start-Process -FilePath $runtime -ArgumentList $Arguments -WorkingDirectory $Current -Wait -PassThru -NoNewWindow -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    $text = ((Get-Content $stdout -Raw -ErrorAction SilentlyContinue) + "`n" + (Get-Content $stderr -Raw -ErrorAction SilentlyContinue))
    if ($p.ExitCode -eq 0 -or $text -notmatch [regex]::Escape($ExpectedCode)) {
        throw "expected runtime failure $ExpectedCode; command=$($Arguments -join ' '); exit=$($p.ExitCode); output=$text"
    }
}

function Assert-OneMatchPath($Result, [string]$ExpectedPath, [string]$Label) {
    $navMatches = @($Result.matches)
    if ($navMatches.Count -ne 1) {
        throw "$Label expected exactly one runtime match; got $($navMatches.Count): $($Result | ConvertTo-Json -Depth 8 -Compress)"
    }
    $actual = [string]($navMatches[0].path)
    if ($actual -ne $ExpectedPath) {
        throw "$Label path mismatch: got=$actual want=$ExpectedPath; runtime=$($Result | ConvertTo-Json -Depth 8 -Compress)"
    }
}

function Assert-HasMatchPath($Result, [string]$ExpectedPath, [string]$Label) {
    $paths = @($Result.matches | ForEach-Object { [string]($_.path) })
    if ($paths -notcontains $ExpectedPath) {
        throw "$Label missing source path $ExpectedPath; got=$($paths -join ','); runtime=$($Result | ConvertTo-Json -Depth 8 -Compress)"
    }
}

function New-Task5Fixture([string]$Scenario) {
    $root = Join-Path $env:RUNNER_TEMP ('task5-real-' + $Scenario + '-' + [guid]::NewGuid().ToString())
    $current = Join-Path $root 'order-service'
    $dependency = Join-Path $root 'company-framework'
    New-Item -ItemType Directory -Force $current,$dependency | Out-Null
    Copy-Item -Recurse (Join-Path $repoRoot '.code-harness') (Join-Path $current '.code-harness')

    $currentJava = Join-Path $current 'src/main/java/com/company/order'
    $dependencyJava = Join-Path $dependency 'src/main/java/com/company/framework'
    $mapperDir = Join-Path $current 'src/main/resources/mapper'
    New-Item -ItemType Directory -Force $currentJava,$dependencyJava,$mapperDir | Out-Null

    Write-Utf8NoBom (Join-Path $current 'pom.xml') @'
<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.company</groupId>
  <artifactId>order-service</artifactId>
  <version>1.0.0</version>
  <dependencies>
    <dependency>
      <groupId>com.company</groupId>
      <artifactId>company-framework</artifactId>
      <version>2.3.1</version>
    </dependency>
  </dependencies>
</project>
'@

    if ($Scenario -ne 'SOURCE_NOT_FOUND') {
        $depVersion = if ($Scenario -eq 'VERSION_MISMATCH') { '2.4.0' } else { '2.3.1' }
        Write-Utf8NoBom (Join-Path $dependency 'pom.xml') ("<project><modelVersion>4.0.0</modelVersion><groupId>com.company</groupId><artifactId>company-framework</artifactId><version>$depVersion</version></project>")
    }

    $dependencySource = @'
package com.company.framework;
public abstract class AbstractTemplate {
    public void execute() {
        before();
        doExecute();
        after();
    }
    protected void before() {}
    protected abstract void doExecute();
    protected void after() {}
}
'@
    Write-Utf8NoBom (Join-Path $dependencyJava 'AbstractTemplate.java') $dependencySource

    Write-Utf8NoBom (Join-Path $currentJava 'XxxService.java') @'
package com.company.order;
interface XxxService {
    void submit();
}
'@
    Write-Utf8NoBom (Join-Path $currentJava 'XxxMapper.java') @'
package com.company.order;
interface XxxMapper {
    void updateStatus();
}
'@
    Write-Utf8NoBom (Join-Path $currentJava 'XxxController.java') @'
package com.company.order;
class XxxController {
    private XxxService service;
    public void health() {}
}
'@
    Write-Utf8NoBom (Join-Path $currentJava 'XxxServiceImpl.java') @'
package com.company.order;
import com.company.framework.AbstractTemplate;
public class XxxServiceImpl extends AbstractTemplate implements XxxService {
    private XxxMapper mapper;
    public void submit() {}
    @Override protected void doExecute() {}
}
'@

    $harness = @'
version: 2
project:
  type: maven
  root: .
  module: ""
review:
  baseRef: HEAD
  includeWorkingTree: true
integrationTest:
  executable: mvn
  args: [test]
  reportDir: target/surefire-reports
  timeoutSeconds: 600
service:
  executable: mvn
  args: [spring-boot:run]
  startupTimeoutSeconds: 120
  readiness:
    type: log
    pattern: Started
  logFile: null
stopService:
  mode: processTree
initialization:
  status: READY
  unresolved: []
scope:
  sourceIncludes: [src/main/java/**/*.java]
  testIncludes: [src/test/java/**/*.java]
  mapperIncludes: [src/main/resources/**/*Mapper.xml]
  configIncludes: [src/main/resources/**/*.yml]
write:
  allowedTestPaths: [src/test/java/**]
  allowedProductionPaths: [src/main/java/**]
  deniedPaths: [.git/**, .github/**, target/**, .code-harness/agents/**, .code-harness/skills/**, .code-harness/contracts/**, .code-harness/tools/**]
runs:
  directory: .code-harness/runs
'@
    if ($Scenario -ne 'NOT_CONFIGURED') {
        $harness += "`n" + @'
workspaceDependencies:
  - id: company-framework
    root: ../company-framework
    maven:
      groupId: com.company
      artifactId: company-framework
    mode: READ_ONLY
'@
    }
    Write-Utf8NoBom (Join-Path $current '.code-harness/harness.yaml') $harness
    Write-Utf8NoBom (Join-Path $current '.gitignore') ".code-harness/`n"

    git -C $current init -b develop | Out-Null
    git -C $current config user.email 'codea@example.invalid'
    git -C $current config user.name 'Codea Harness Task5'
    git -C $current add pom.xml .gitignore src/main/java
    git -C $current commit -m baseline | Out-Null
    $baseline = (git -C $current rev-parse HEAD).Trim()

    # Real mixed working tree required by Task 5.
    Write-Utf8NoBom (Join-Path $currentJava 'XxxController.java') @'
package com.company.order;
class XxxController {
    private XxxService service;
    @PostMapping
    public void submit() { service.submit(); }
}
'@
    git -C $current add src/main/java/com/company/order/XxxController.java

    Write-Utf8NoBom (Join-Path $currentJava 'XxxServiceImpl.java') @'
package com.company.order;
import com.company.framework.AbstractTemplate;
public class XxxServiceImpl extends AbstractTemplate implements XxxService {
    private XxxMapper mapper;
    @Override public void submit() { execute(); }
    @Override protected void doExecute() { mapper.updateStatus(); }
}
'@

    Write-Utf8NoBom (Join-Path $mapperDir 'XxxMapper.xml') @'
<?xml version="1.0" encoding="UTF-8"?>
<mapper namespace="com.company.order.XxxMapper">
  <update id="updateStatus">UPDATE t_order SET status = 1 WHERE id = #{id}</update>
</mapper>
'@

    if ($Scenario -eq 'AMBIGUOUS_TEMPLATE_DISPATCH') {
        Write-Utf8NoBom (Join-Path $currentJava 'AnotherServiceImpl.java') @'
package com.company.order;
import com.company.framework.AbstractTemplate;
public class AnotherServiceImpl extends AbstractTemplate {
    @Override protected void doExecute() {}
}
'@
    }

    return [pscustomobject]@{
        Root = $root
        Current = $current
        Dependency = $dependency
        Baseline = $baseline
        DependencySource = $dependencySource
    }
}

function Invoke-Task5FailureScenario([string]$Scenario, [string]$ExpectedCode) {
    $fixture = New-Task5Fixture $Scenario
    if ($Scenario -eq 'AMBIGUOUS_TEMPLATE_DISPATCH') {
        $verified = Invoke-RuntimeJson $fixture.Current @('workspace','verify','--id','company-framework')
        if ($verified.status -ne 'VERIFIED') { throw "ambiguous fixture failed Maven verification: $($verified | ConvertTo-Json -Compress)" }
        Invoke-RuntimeFailure $fixture.Current @('nav','workspace-template-dispatch','--workspace','company-framework','--from','AbstractTemplate.execute','--hook','doExecute') $ExpectedCode
    } else {
        Invoke-RuntimeFailure $fixture.Current @('workspace','verify','--id','company-framework') $ExpectedCode
    }
    if (Test-Path (Join-Path $fixture.Current '.code-harness/runs')) {
        $unexpected = @(Get-ChildItem (Join-Path $fixture.Current '.code-harness/runs') -Recurse -File -ErrorAction SilentlyContinue)
        if ($unexpected.Count -ne 0) { throw "$Scenario wrote run artifacts despite PARTIAL gate" }
    }
    if (Test-Path (Join-Path $fixture.Dependency '.code-harness')) { throw "$Scenario wrote dependency workspace state" }
    Write-Host "TASK5 FAILURE $Scenario -> $ExpectedCode PASS"
}

$fixture = New-Task5Fixture 'SUCCESS'
$current = $fixture.Current
$dependency = $fixture.Dependency
$dependencySourcePath = Join-Path $dependency 'src/main/java/com/company/framework/AbstractTemplate.java'
$dependencyHashBefore = (Get-FileHash -Algorithm SHA256 $dependencySourcePath).Hash

$validatedConfig = Invoke-RuntimeJson $current @('validate','--schema','.code-harness/contracts/harness-config.schema.json','--input','.code-harness/harness.yaml','--format','yaml')
if ($validatedConfig.status -ne 'VALID') { throw 'harness config was not valid' }

$staged = @(git -C $current diff --cached --name-only -- | ForEach-Object { $_.Trim() } | Where-Object { $_ })
$unstaged = @(git -C $current diff --name-only -- | ForEach-Object { $_.Trim() } | Where-Object { $_ })
$untracked = @(git -C $current ls-files --others --exclude-standard -- | ForEach-Object { $_.Trim() } | Where-Object { $_ })
if ($staged -notcontains 'src/main/java/com/company/order/XxxController.java') { throw "STAGED change missing Controller: $($staged -join ',')" }
if ($unstaged -notcontains 'src/main/java/com/company/order/XxxServiceImpl.java') { throw "UNSTAGED change missing ServiceImpl: $($unstaged -join ',')" }
if ($untracked -notcontains 'src/main/resources/mapper/XxxMapper.xml') { throw "UNTRACKED change missing Mapper.xml: $($untracked -join ',')" }
foreach ($p in @($staged + $unstaged + $untracked)) {
    if ($p -like '../*' -or $p -match 'company-framework') { throw "dependency leaked into current Change Set: $p" }
}
Write-Host 'STAGED + UNSTAGED + UNTRACKED PASS'

# Current-project facts must come from real Controlled Runtime navigation.
$controller = Invoke-RuntimeJson $current @('nav','find-symbol','--symbol','XxxController','--scope','src/main/java')
$service = Invoke-RuntimeJson $current @('nav','find-symbol','--symbol','XxxService','--scope','src/main/java')
$impl = Invoke-RuntimeJson $current @('nav','find-implementations','--symbol','XxxService','--scope','src/main/java')
$mapper = Invoke-RuntimeJson $current @('nav','find-symbol','--symbol','XxxMapper','--scope','src/main/java')
$serviceRefs = Invoke-RuntimeJson $current @('nav','find-references','--symbol','XxxService','--scope','src/main/java')
$submitRefs = Invoke-RuntimeJson $current @('nav','find-references','--symbol','XxxService.submit','--scope','src/main/java')
$mapperRefs = Invoke-RuntimeJson $current @('nav','find-references','--symbol','XxxMapper.updateStatus','--scope','src/main/java')

Assert-OneMatchPath $controller 'src/main/java/com/company/order/XxxController.java' 'Controller find-symbol'
Assert-OneMatchPath $service 'src/main/java/com/company/order/XxxService.java' 'Service find-symbol'
Assert-OneMatchPath $impl 'src/main/java/com/company/order/XxxServiceImpl.java' 'Service find-implementations'
Assert-OneMatchPath $mapper 'src/main/java/com/company/order/XxxMapper.java' 'Mapper find-symbol'
Assert-HasMatchPath $serviceRefs 'src/main/java/com/company/order/XxxController.java' 'Controller -> Service find-references'
Assert-HasMatchPath $submitRefs 'src/main/java/com/company/order/XxxController.java' 'Controller submit call find-references'
Assert-HasMatchPath $mapperRefs 'src/main/java/com/company/order/XxxServiceImpl.java' 'override -> Mapper find-references'

$serviceSource = Get-Content (Join-Path $current 'src/main/java/com/company/order/XxxService.java') -Raw
$controllerSource = Get-Content (Join-Path $current 'src/main/java/com/company/order/XxxController.java') -Raw
$implSource = Get-Content (Join-Path $current 'src/main/java/com/company/order/XxxServiceImpl.java') -Raw
if ($serviceSource -notmatch 'void\s+submit\s*\(') { throw 'XxxService.submit source evidence missing' }
if ($controllerSource -notmatch 'service\.submit\s*\(') { throw 'Controller -> XxxService.submit source evidence missing' }
if ($implSource -notmatch 'implements\s+XxxService' -or $implSource -notmatch 'execute\s*\(') { throw 'XxxServiceImpl submit/inheritance source evidence missing' }

$verified = Invoke-RuntimeJson $current @('workspace','verify','--id','company-framework')
if ($verified.status -ne 'VERIFIED') { throw "Maven identity was not VERIFIED: $($verified | ConvertTo-Json -Depth 8 -Compress)" }
Write-Host 'Maven VERIFIED PASS'

$inherited = Invoke-RuntimeJson $current @('nav','workspace-inherited','--workspace','company-framework','--from','XxxServiceImpl.submit','--method','execute')
$superCall = Invoke-RuntimeJson $current @('nav','workspace-superclass-call','--workspace','company-framework','--from','AbstractTemplate.execute','--method','before')
$dispatch = Invoke-RuntimeJson $current @('nav','workspace-template-dispatch','--workspace','company-framework','--from','AbstractTemplate.execute','--hook','doExecute','--concrete','XxxServiceImpl')
foreach ($result in @($inherited,$superCall,$dispatch)) {
    if ($result.status -ne 'COMPLETE' -or $null -eq $result.fact -or $result.fact.source -ne 'WORKSPACE_INHERITANCE') {
        throw "workspace navigation was not confirmed: $($result | ConvertTo-Json -Depth 8 -Compress)"
    }
}
if ($inherited.fact.symbol -ne 'AbstractTemplate.execute' -or $inherited.fact.workspace -ne 'company-framework') { throw 'dependency AbstractTemplate.execute fact mismatch' }
if ($dispatch.fact.symbol -ne 'XxxServiceImpl.doExecute' -or $dispatch.fact.workspace -ne 'current') { throw 'template dispatch did not return to current override' }
Write-Host 'workspace inheritance + template dispatch PASS'

$mapperXmlPath = 'src/main/resources/mapper/XxxMapper.xml'
$mapperXml = Get-Content (Join-Path $current $mapperXmlPath) -Raw
if ($mapperXml -notmatch '<mapper\s+namespace="com\.company\.order\.XxxMapper"' -or $mapperXml -notmatch '<update\s+id="updateStatus"') {
    throw 'Mapper.xml namespace / statement id source evidence mismatch'
}
Write-Host 'Mapper -> Mapper.xml source evidence PASS'

$changedFiles = @(
    [ordered]@{ path='src/main/java/com/company/order/XxxController.java'; role='Controller'; sources=@('STAGED') },
    [ordered]@{ path='src/main/java/com/company/order/XxxServiceImpl.java'; role='Service'; sources=@('UNSTAGED') },
    [ordered]@{ path=$mapperXmlPath; role='MapperXml'; sources=@('UNTRACKED') }
)
$reviewedFiles = @(
    [ordered]@{ path='src/main/java/com/company/order/XxxController.java'; role='Controller'; reason='CHANGED' },
    [ordered]@{ path='src/main/java/com/company/order/XxxServiceImpl.java'; role='Service'; reason='CHANGED' },
    [ordered]@{ path=$mapperXmlPath; role='MapperXml'; reason='CHANGED' }
)
$symbolLocations = @(
    [ordered]@{ workspace='current'; symbol='XxxController.submit'; path=$controller.matches[0].path; role='Controller'; source='FIND_SYMBOL' },
    [ordered]@{ workspace='current'; symbol='XxxService.submit'; path=$service.matches[0].path; role='Service'; source='FIND_SYMBOL'; from='XxxController.submit' },
    [ordered]@{ workspace='current'; symbol='XxxServiceImpl.submit'; path=$impl.matches[0].path; role='Service'; source='FIND_IMPLEMENTATIONS'; from='XxxService.submit' },
    $inherited.fact,
    $dispatch.fact,
    [ordered]@{ workspace='current'; symbol='XxxMapper.updateStatus'; path=$mapper.matches[0].path; role='Mapper'; source='FIND_SYMBOL'; from='XxxServiceImpl.doExecute' }
)
$resourceRelations = @(
    [ordered]@{ path=$mapperXmlPath; role='MapperXml'; resource='XxxMapper.xml#updateStatus'; fromSymbol='XxxMapper.updateStatus'; fromKind='METHOD'; source='MAPPER_STATEMENT'; evidence='namespace com.company.order.XxxMapper + statement id updateStatus verified from fixture source' }
)
$callChains = @(
    [ordered]@{ entryPoint='XxxController.submit'; chain=@('XxxController.submit','XxxService.submit','XxxServiceImpl.submit','AbstractTemplate.execute','XxxServiceImpl.doExecute','XxxMapper.updateStatus') }
)
$analysis = [ordered]@{
    reviewScope = [ordered]@{ currentBranch='develop'; baseRef='HEAD'; baseCommit=$fixture.Baseline; mergeBase=$fixture.Baseline; headCommit=$fixture.Baseline; includeWorkingTree=$true }
    changedFiles = $changedFiles
    affectedControllers = @([ordered]@{ controller='XxxController'; endpoints=@('XxxController.submit'); impactType='DIRECT_CHANGE'; sourceSymbols=@('XxxController.submit') })
    callChains = $callChains
    symbolLocations = $symbolLocations
    resourceRelations = $resourceRelations
    externalDependencies = @()
    riskAreas = @()
    reviewCoverage = [ordered]@{ status='COMPLETE'; reviewedFiles=$reviewedFiles; unresolvedSymbols=@() }
}

$runId = 'run-task5-real-business'
$analysisRel = ".code-harness/runs/$runId/analysis/change-analysis.json"
$requestRel = ".code-harness/runs/$runId/requests/chain-discover.json"
$analysisAbs = Join-Path $current ($analysisRel -replace '/', [IO.Path]::DirectorySeparatorChar)
$requestAbs = Join-Path $current ($requestRel -replace '/', [IO.Path]::DirectorySeparatorChar)
if (Test-Path $analysisAbs) { throw 'precondition violated: change-analysis.json already existed' }
if (Test-Path (Join-Path $current '.code-harness/chains')) { throw 'precondition violated: historical chains/** exists' }
New-Item -ItemType Directory -Force (Split-Path $analysisAbs),(Split-Path $requestAbs) | Out-Null
$analysis | ConvertTo-Json -Depth 12 | Set-Content $analysisAbs

$coverageResult = Invoke-RuntimeJson $current @('validate','--schema','.code-harness/contracts/change-analysis.schema.json','--input',$analysisRel,'--format','json')
if ($coverageResult.status -ne 'VALID' -or $coverageResult.reviewCoverage.status -ne 'COMPLETE') {
    throw "Schema/FULL Coverage verification failed: $($coverageResult | ConvertTo-Json -Depth 10 -Compress)"
}
foreach ($reviewed in @($coverageResult.reviewCoverage.reviewedFiles)) {
    if ([string]($reviewed.path) -match 'company-framework|\.\./') { throw "dependency leaked into FULL Coverage: $($reviewed.path)" }
}
Write-Host 'Coverage COMPLETE PASS'

$request = [ordered]@{ runId=$runId; target='XxxController'; changeAnalysisPath=$analysisRel }
$request | ConvertTo-Json -Depth 6 | Set-Content $requestAbs
$discovery = Invoke-RuntimeJson $current @('chain','discover','--input',$requestRel)
if ($discovery.status -ne 'COMPLETE') { throw "chain discover not COMPLETE: $($discovery | ConvertTo-Json -Depth 10 -Compress)" }
$discoveredDir = Join-Path $current ".code-harness/runs/$runId/analysis/discovered-chains"
$discovered = @(Get-ChildItem $discoveredDir -Filter '*.yaml')
if ($discovered.Count -ne 1) { throw "expected exactly one DISCOVERED Chain, got $($discovered.Count)" }
$chainText = Get-Content $discovered[0].FullName -Raw
foreach ($expected in @('status: DISCOVERED','XxxController.submit','XxxService.submit','XxxServiceImpl.submit','workspace: company-framework','AbstractTemplate.execute','XxxServiceImpl.doExecute','XxxMapper.updateStatus','XxxMapper.xml')) {
    if ($chainText -notmatch [regex]::Escape($expected)) { throw "DISCOVERED Chain missing ${expected}: $chainText" }
}
Write-Host 'exactly one DISCOVERED Chain PASS'

if (Test-Path (Join-Path $dependency '.code-harness')) { throw 'dependency received .code-harness runs/state' }
$dependencyHashAfter = (Get-FileHash -Algorithm SHA256 $dependencySourcePath).Hash
if ($dependencyHashBefore -ne $dependencyHashAfter) { throw 'dependency source was modified' }
if (Test-Path (Join-Path $current ".code-harness/runs/$runId/review.md")) { throw 'Task 5 discovery created dependency/current Finding artifact' }
$runRoots = @(Get-ChildItem (Join-Path $current '.code-harness/runs') -Directory -ErrorAction SilentlyContinue)
if ($runRoots.Count -ne 1 -or $runRoots[0].Name -ne $runId) { throw "Project State was not current-project-only: $($runRoots.Name -join ',')" }
Write-Host 'current runs/** only + dependency no write PASS'

# The four PARTIAL regressions use the same real fixture/Runtime driver and must not return hard-coded test codes.
Invoke-Task5FailureScenario 'VERSION_MISMATCH' 'WORKSPACE_DEPENDENCY_VERSION_MISMATCH'
Invoke-Task5FailureScenario 'NOT_CONFIGURED' 'WORKSPACE_DEPENDENCY_NOT_CONFIGURED'
Invoke-Task5FailureScenario 'AMBIGUOUS_TEMPLATE_DISPATCH' 'AMBIGUOUS_TEMPLATE_DISPATCH'
Invoke-Task5FailureScenario 'SOURCE_NOT_FOUND' 'WORKSPACE_DEPENDENCY_SOURCE_NOT_FOUND'

Write-Host 'TASK5_REAL_DUAL_PROJECT_BUSINESS_REGRESSION PASS'
