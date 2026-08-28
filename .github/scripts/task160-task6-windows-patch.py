from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
path = ROOT / '.github/scripts/task160-real-review-precision-regression.ps1'
s = path.read_text(encoding='utf-8')

if '<artifactId>company-framework</artifactId>' not in s:
    s = s.replace('  <version>1.0.0</version>\n</project>', '  <version>1.0.0</version>\n  <dependencies>\n    <dependency><groupId>com.company</groupId><artifactId>company-framework</artifactId><version>2.3.1</version></dependency>\n  </dependencies>\n</project>', 1)

config_block = """        Write-Utf8NoBom $configPath @'\nspring:\n  datasource:\n    hikari:\n      connection-timeout: 30000\n'@\n"""
if '$dependencyRoot' not in s:
    dep = """        $dependencyRoot = Join-Path (Split-Path -Parent $fixture) 'company-framework'\n        Write-Utf8NoBom (Join-Path $dependencyRoot 'pom.xml') @'\n<project><modelVersion>4.0.0</modelVersion><groupId>com.company</groupId><artifactId>company-framework</artifactId><version>2.3.1</version></project>\n'@\n        Write-Utf8NoBom (Join-Path $dependencyRoot 'src/main/java/com/company/framework/SharedPolicy.java') @'\npackage com.company.framework; public class SharedPolicy { public void check() {} }\n'@\n"""
    if config_block not in s: raise SystemExit('baseline config block not found')
    s = s.replace(config_block, config_block + dep, 1)

runtime_marker = "        $script:runtime = (Resolve-Path '.code-harness\\bin\\codea-harness-tools.exe').Path\n"
if 'workspaceDependencies:' not in s:
    harness = """        Write-Utf8NoBom (Join-Path $fixture '.code-harness/harness.yaml') @'\nworkspaceDependencies:\n  - id: company-framework\n    root: ../company-framework\n    maven:\n      groupId: com.company\n      artifactId: company-framework\n    mode: READ_ONLY\n'@\n"""
    if runtime_marker not in s: raise SystemExit('runtime marker not found')
    s = s.replace(runtime_marker, harness + runtime_marker, 1)

s = s.replace('symbolLocations = @()', "symbolLocations = @([ordered]@{ workspace='company-framework'; symbol='SharedPolicy.check'; path='src/main/java/com/company/framework/SharedPolicy.java'; role='Service'; source='WORKSPACE_NAVIGATION' })", 1)
s = s.replace('externalDependencies = @()', "externalDependencies = @('company-framework')", 1)
s = s.replace("path='workspace/shared/src/main/resources/application.yml'", "path='src/main/java/com/company/framework/SharedPolicy.java'", 1)

sentinel = "        Write-Output 'TASK160_DEPENDENCY_SCOPE_REJECTED PASS'\n"
if 'P-DEPENDENCY exact machine code' not in s:
    exact = """        # P-DEPENDENCY exact machine code: sentinel is emitted only after Runtime authority rejects dependency scope.\n        $dependencyRejection = @($certify.rejections | Where-Object { $_.proposalId -eq 'P-DEPENDENCY' }) | Select-Object -First 1\n        if ($null -eq $dependencyRejection -or [string]$dependencyRejection.code -ne 'FINDING_DEPENDENCY_SCOPE_FORBIDDEN') {\n            $actualCode = if ($null -eq $dependencyRejection) { '<missing>' } else { [string]$dependencyRejection.code }\n            throw \"Task160 P-DEPENDENCY expected FINDING_DEPENDENCY_SCOPE_FORBIDDEN, got $actualCode\"\n        }\n"""
    if sentinel not in s: raise SystemExit('dependency sentinel not found')
    s = s.replace(sentinel, exact + sentinel, 1)

path.write_text(s, encoding='utf-8')
