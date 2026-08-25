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
import com.company.framework.AbstractTemplate;
public class XxxServiceImpl extends AbstractTemplate {
    public void submit() { execute(); }
    @Override protected void doExecute() { mapper.updateStatus(); }
}
'@ | Set-Content (Join-Path $currentJava 'XxxServiceImpl.java')

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

Invoke-ExpectedSuccess @('validate','--schema','.code-harness/contracts/harness-config.schema.json','--input','.code-harness/harness.yaml','--format','yaml') 'VALID'
Invoke-ExpectedSuccess @('workspace','verify','--id','company-framework') 'VERIFIED'
Invoke-ExpectedSuccess @('nav','workspace-inherited','--workspace','company-framework','--from','XxxServiceImpl.submit','--method','execute') 'AbstractTemplate.execute'
Invoke-ExpectedSuccess @('nav','workspace-superclass-call','--workspace','company-framework','--from','AbstractTemplate.execute','--method','validate') 'AbstractTemplate.validate'
Invoke-ExpectedSuccess @('nav','workspace-template-dispatch','--workspace','company-framework','--from','AbstractTemplate.execute','--hook','doExecute','--concrete','XxxServiceImpl') 'XxxServiceImpl.doExecute'

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
