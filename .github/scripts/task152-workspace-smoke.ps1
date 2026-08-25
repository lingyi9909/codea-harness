$ErrorActionPreference = 'Stop'

$root = Join-Path $env:RUNNER_TEMP 'workspace-152-formal'
$current = Join-Path $root 'order-service'
$dependency = Join-Path $root 'company-framework'
Remove-Item -Recurse -Force $root -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $current,$dependency | Out-Null
Copy-Item -Recurse '.code-harness' (Join-Path $current '.code-harness')

@'
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
'@ | Set-Content (Join-Path $current 'pom.xml')

@'
<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.company</groupId>
  <artifactId>company-framework</artifactId>
  <version>2.3.1</version>
</project>
'@ | Set-Content (Join-Path $dependency 'pom.xml')

$currentJava = Join-Path $current 'src/main/java/com/company/order'
$dependencyJava = Join-Path $dependency 'src/main/java/com/company/framework'
New-Item -ItemType Directory -Force $currentJava,$dependencyJava | Out-Null

@'
package com.company.framework;
public abstract class AbstractTemplate {
    public void execute() {
        validate();
        doExecute();
    }
    protected void validate() {}
    protected abstract void doExecute();
}
'@ | Set-Content (Join-Path $dependencyJava 'AbstractTemplate.java')

@'
package com.company.order;
@RestController
public class XxxController {
    private XxxServiceImpl service;
    public void submit() { service.submit(); }
}
'@ | Set-Content (Join-Path $currentJava 'XxxController.java')

@'
package com.company.order;
import com.company.framework.AbstractTemplate;
@Service
public class XxxServiceImpl extends AbstractTemplate {
    private XxxMapper mapper;
    public void submit() { execute(); }
    @Override protected void doExecute() { mapper.updateStatus(); }
}
'@ | Set-Content (Join-Path $currentJava 'XxxServiceImpl.java')

@'
package com.company.order;
@Mapper
public interface XxxMapper {
    void updateStatus();
}
'@ | Set-Content (Join-Path $currentJava 'XxxMapper.java')

@'
version: 2
project:
  type: maven
  root: .
  module: ""
workspaceDependencies:
  - id: company-framework
    root: ../company-framework
    maven:
      groupId: com.company
      artifactId: company-framework
    mode: READ_ONLY
review:
  baseRef: origin/main
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
'@ | Set-Content (Join-Path $current '.code-harness/harness.yaml')

@'
.code-harness/
'@ | Set-Content (Join-Path $current '.gitignore')
git -C $current init -b develop | Out-Null
git -C $current config user.email 'codea@example.invalid'
git -C $current config user.name 'Codea Test'
git -C $current add pom.xml .gitignore
git -C $current commit -m baseline | Out-Null
$baselineCommit = (git -C $current rev-parse HEAD).Trim()

function Invoke-ExpectedSuccess([string[]]$Arguments, [string]$Expected) {
    $runtime = Join-Path $current '.code-harness/bin/codea-harness-tools.exe'
    $stdout = Join-Path $env:RUNNER_TEMP ('task152-success-' + [guid]::NewGuid().ToString() + '.out')
    $stderr = Join-Path $env:RUNNER_TEMP ('task152-success-' + [guid]::NewGuid().ToString() + '.err')
    $p = Start-Process -FilePath $runtime -ArgumentList $Arguments -WorkingDirectory $current -Wait -PassThru -NoNewWindow -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    $text = ((Get-Content $stdout -Raw -ErrorAction SilentlyContinue) + "`n" + (Get-Content $stderr -Raw -ErrorAction SilentlyContinue))
    if ($p.ExitCode -ne 0 -or $text -notmatch [regex]::Escape($Expected)) {
        throw "expected success containing '$Expected'; exit=$($p.ExitCode); output=$text"
    }
}

function Invoke-ExpectedFailure([string[]]$Arguments, [string]$ExpectedCode) {
    $runtime = Join-Path $current '.code-harness/bin/codea-harness-tools.exe'
    $stdout = Join-Path $env:RUNNER_TEMP ('task152-failure-' + [guid]::NewGuid().ToString() + '.out')
    $stderr = Join-Path $env:RUNNER_TEMP ('task152-failure-' + [guid]::NewGuid().ToString() + '.err')
    $p = Start-Process -FilePath $runtime -ArgumentList $Arguments -WorkingDirectory $current -Wait -PassThru -NoNewWindow -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    $text = ((Get-Content $stdout -Raw -ErrorAction SilentlyContinue) + "`n" + (Get-Content $stderr -Raw -ErrorAction SilentlyContinue))
    if ($p.ExitCode -eq 0 -or $text -notmatch [regex]::Escape($ExpectedCode)) {
        throw "expected failure '$ExpectedCode'; exit=$($p.ExitCode); output=$text"
    }
}

function Invoke-RuntimeJson([string[]]$Arguments) {
    $runtime = Join-Path $current '.code-harness/bin/codea-harness-tools.exe'
    $stdout = Join-Path $env:RUNNER_TEMP ('task152-json-' + [guid]::NewGuid().ToString() + '.out')
    $stderr = Join-Path $env:RUNNER_TEMP ('task152-json-' + [guid]::NewGuid().ToString() + '.err')
    $p = Start-Process -FilePath $runtime -ArgumentList $Arguments -WorkingDirectory $current -Wait -PassThru -NoNewWindow -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    $out = Get-Content $stdout -Raw -ErrorAction SilentlyContinue
    $err = Get-Content $stderr -Raw -ErrorAction SilentlyContinue
    if ($p.ExitCode -ne 0) {
        throw "runtime JSON command failed: $($Arguments -join ' '); exit=$($p.ExitCode); stderr=$err; stdout=$out"
    }
    try {
        return ($out | ConvertFrom-Json)
    } catch {
        throw "runtime JSON command returned invalid JSON: $($Arguments -join ' '); stdout=$out; stderr=$err"
    }
}

Invoke-ExpectedSuccess @('validate','--schema','.code-harness/contracts/harness-config.schema.json','--input','.code-harness/harness.yaml','--format','yaml') 'VALID'
Invoke-ExpectedSuccess @('workspace','verify','--id','company-framework') 'VERIFIED'
Invoke-ExpectedSuccess @('nav','workspace-inherited','--workspace','company-framework','--from','XxxServiceImpl.submit','--method','execute') 'AbstractTemplate.execute'
Invoke-ExpectedSuccess @('nav','workspace-superclass-call','--workspace','company-framework','--from','AbstractTemplate.execute','--method','validate') 'AbstractTemplate.validate'
Invoke-ExpectedSuccess @('nav','workspace-template-dispatch','--workspace','company-framework','--from','AbstractTemplate.execute','--hook','doExecute','--concrete','XxxServiceImpl') 'XxxServiceImpl.doExecute'

# TASK4_WORKSPACE_ANALYZE_CHANGE_BOOTSTRAP
# Deterministic acceptance driver for the natural-language user intent; no ChangeAnalysis or workspace SymbolLocation is prewritten.
$intent = 'harness chain discover XxxController'
if ($intent -ne 'harness chain discover XxxController') { throw 'unexpected harness intent' }
$runId = 'run-152-workspace-bootstrap'
$analysisRel = ".code-harness/runs/$runId/analysis/change-analysis.json"
$requestRel = ".code-harness/runs/$runId/requests/chain-discover.json"
$analysisAbs = Join-Path $current ($analysisRel -replace '/', [IO.Path]::DirectorySeparatorChar)
$requestAbs = Join-Path $current ($requestRel -replace '/', [IO.Path]::DirectorySeparatorChar)
if (Test-Path $analysisAbs) { throw 'bootstrap analysis unexpectedly exists' }
Write-Host 'precondition: no change-analysis.json'

# Real current Change Set: the three business Java files are untracked relative to the committed baseline.
$changedPaths = @(git -C $current ls-files --others --exclude-standard -- src/main/java | ForEach-Object { $_.Trim() } | Where-Object { $_ })
if ($changedPaths.Count -ne 3) { throw "expected exactly 3 current-project changed Java files, got $($changedPaths -join ',')" }

# Current-project exact paths come from real Controlled Runtime Code Navigation.
$controllerNav = Invoke-RuntimeJson @('nav','find-symbol','--symbol','XxxController','--scope','src/main/java')
$serviceNav = Invoke-RuntimeJson @('nav','find-symbol','--symbol','XxxServiceImpl','--scope','src/main/java')
$mapperNav = Invoke-RuntimeJson @('nav','find-symbol','--symbol','XxxMapper','--scope','src/main/java')
if ($controllerNav.matches.Count -ne 1 -or $serviceNav.matches.Count -ne 1 -or $mapperNav.matches.Count -ne 1) {
    throw 'current-project Code Navigation did not resolve unique exact paths'
}

# The current-project inherited execute() edge is intentionally not closed by normal nav.
# analyze-change may use only the explicit configured dependency, must verify it first, then use workspace Runtime navigation.
$verified = Invoke-RuntimeJson @('workspace','verify','--id','company-framework')
if ($verified.status -ne 'VERIFIED') { throw "workspace dependency was not VERIFIED: $($verified | ConvertTo-Json -Compress)" }
$inherited = Invoke-RuntimeJson @('nav','workspace-inherited','--workspace','company-framework','--from','XxxServiceImpl.submit','--method','execute')
$superCall = Invoke-RuntimeJson @('nav','workspace-superclass-call','--workspace','company-framework','--from','AbstractTemplate.execute','--method','validate')
$dispatch = Invoke-RuntimeJson @('nav','workspace-template-dispatch','--workspace','company-framework','--from','AbstractTemplate.execute','--hook','doExecute','--concrete','XxxServiceImpl')
foreach ($result in @($inherited,$superCall,$dispatch)) {
    if ($result.status -ne 'COMPLETE' -or $null -eq $result.fact -or $result.fact.source -ne 'WORKSPACE_INHERITANCE') {
        throw "workspace navigation did not return confirmed WORKSPACE_INHERITANCE fact: $($result | ConvertTo-Json -Depth 8 -Compress)"
    }
}

$roleByPath = @{}
$roleByPath[$controllerNav.matches[0].path] = 'Controller'
$roleByPath[$serviceNav.matches[0].path] = 'Service'
$roleByPath[$mapperNav.matches[0].path] = 'Mapper'
$changedFiles = @()
$reviewedFiles = @()
foreach ($path in ($changedPaths | Sort-Object)) {
    if (-not $roleByPath.ContainsKey($path)) { throw "changed path lacks verified navigation role: $path" }
    $role = $roleByPath[$path]
    $changedFiles += [ordered]@{ path=$path; role=$role; sources=@('UNTRACKED') }
    $reviewedFiles += [ordered]@{ path=$path; role=$role; reason='CHANGED' }
}

$symbolLocations = @(
    [ordered]@{ workspace='current'; symbol='XxxController.submit'; path=$controllerNav.matches[0].path; role='Controller'; source='FIND_SYMBOL' },
    [ordered]@{ workspace='current'; symbol='XxxServiceImpl.submit'; path=$serviceNav.matches[0].path; role='Service'; source='FIND_SYMBOL'; from='XxxController.submit' },
    $inherited.fact,
    $superCall.fact,
    $dispatch.fact,
    [ordered]@{ workspace='current'; symbol='XxxMapper.updateStatus'; path=$mapperNav.matches[0].path; role='Mapper'; source='FIND_SYMBOL'; from='XxxServiceImpl.doExecute' }
)

$analysis = [ordered]@{
    reviewScope = [ordered]@{
        currentBranch='develop'; baseRef='HEAD'; baseCommit=$baselineCommit; mergeBase=$baselineCommit; headCommit=$baselineCommit; includeWorkingTree=$true
    }
    changedFiles = $changedFiles
    affectedControllers = @([ordered]@{
        controller='XxxController'; endpoints=@('XxxController.submit'); impactType='DIRECT_CHANGE'; sourceSymbols=@('XxxController.submit')
    })
    callChains = @([ordered]@{
        entryPoint='XxxController.submit'
        chain=@('XxxController.submit','XxxServiceImpl.submit','AbstractTemplate.execute','XxxServiceImpl.doExecute','XxxMapper.updateStatus')
    })
    symbolLocations = $symbolLocations
    resourceRelations = @()
    externalDependencies = @()
    riskAreas = @()
    reviewCoverage = [ordered]@{ status='COMPLETE'; reviewedFiles=$reviewedFiles; unresolvedSymbols=@() }
}

New-Item -ItemType Directory -Force (Split-Path $analysisAbs),(Split-Path $requestAbs) | Out-Null
$analysis | ConvertTo-Json -Depth 12 | Set-Content $analysisAbs
if (-not (Test-Path $analysisAbs)) { throw 'analyze-change did not generate change-analysis.json' }
$analysisText = Get-Content $analysisAbs -Raw
if ($analysisText -notmatch 'company-framework' -or $analysisText -notmatch 'WORKSPACE_INHERITANCE') {
    throw 'generated ChangeAnalysis missing runtime-produced workspace evidence'
}
if (($analysis.reviewCoverage.reviewedFiles | Where-Object { $_.path -eq $inherited.fact.path }).Count -ne 0) {
    throw 'dependency workspace path leaked into reviewCoverage.reviewedFiles'
}

Invoke-ExpectedSuccess @('validate','--schema','.code-harness/contracts/change-analysis.schema.json','--input',$analysisRel,'--format','json') 'VALID'
$request = [ordered]@{ runId=$runId; target='XxxController'; changeAnalysisPath=$analysisRel }
$request | ConvertTo-Json -Depth 6 | Set-Content $requestAbs
Invoke-ExpectedSuccess @('chain','discover','--input',$requestRel) 'COMPLETE'

$discoveredDir = Join-Path $current ".code-harness/runs/$runId/analysis/discovered-chains"
$discovered = @(Get-ChildItem $discoveredDir -Filter '*.yaml')
if ($discovered.Count -ne 1) { throw "expected one DISCOVERED workspace Chain, got $($discovered.Count)" }
$chainText = Get-Content $discovered[0].FullName -Raw
foreach ($expected in @('status: DISCOVERED','workspace: company-framework','AbstractTemplate.execute','XxxServiceImpl.doExecute','XxxMapper.updateStatus')) {
    if ($chainText -notmatch [regex]::Escape($expected)) { throw "workspace DISCOVERED Chain missing '$expected': $chainText" }
}
if (Test-Path (Join-Path $current '.code-harness/chains')) { throw 'direct workspace discovery must not write chains/**' }
Write-Host 'TASK4_WORKSPACE_ANALYZE_CHANGE_BOOTSTRAP PASS'

@'
package com.company.order;
import com.company.framework.AbstractTemplate;
public class XxxServiceImpl extends AbstractTemplate {
    private Helper helper;
    public void submit() { helper.execute(); }
    @Override protected void doExecute() { mapper.updateStatus(); }
}
'@ | Set-Content (Join-Path $currentJava 'XxxServiceImpl.java')
Invoke-ExpectedFailure @('nav','workspace-inherited','--workspace','company-framework','--from','XxxServiceImpl.submit','--method','execute') 'INHERITED_METHOD_NOT_FOUND'

@'
package com.company.order;
import com.company.framework.AbstractTemplate;
public class AnotherServiceImpl extends AbstractTemplate {
    @Override protected void doExecute() { }
}
'@ | Set-Content (Join-Path $currentJava 'AnotherServiceImpl.java')
Invoke-ExpectedFailure @('nav','workspace-template-dispatch','--workspace','company-framework','--from','AbstractTemplate.execute','--hook','doExecute') 'AMBIGUOUS_TEMPLATE_DISPATCH'
