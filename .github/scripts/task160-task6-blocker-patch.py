import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def mutate_line(content, line_no, marker):
    lines = content.splitlines(True)
    if not lines:
        return marker + "\n"
    idx = max(0, min(len(lines) - 1, int(line_no or 1) - 1))
    line = lines[idx]
    nl = "\n" if line.endswith("\n") else ""
    body = line[:-1] if nl else line
    lines[idx] = body + marker + nl
    return "".join(lines)


def default_before(case, file):
    content = file.get("content", "")
    line = 1
    for h in case.get("hunks", []):
        if h.get("path") == file.get("path"):
            line = h.get("newStart", 1)
            break
    suffix = " # benchmark-before" if file.get("path", "").endswith((".yml", ".yaml")) else " // benchmark-before"
    if file.get("path", "").endswith(".xml"):
        suffix = " <!-- benchmark-before -->"
    return mutate_line(content, line, suffix)


def semantic_before(case, file):
    cid, content, path = case["id"], file.get("content", ""), file.get("path", "")
    if cid == "01-mybatis-weak-where" and path.endswith("Mapper.xml"):
        return content.replace("</update>", " WHERE id = #{id}</update>")
    if cid == "02-mybatis-tenant-removed" and path.endswith("Mapper.xml"):
        return content.replace("</update>", " WHERE tenant_id = #{tenantId}</update>")
    if cid == "03-mybatis-dollar-bind" and path.endswith("Mapper.xml"):
        return content.replace("${sort}", "created_at")
    if cid in {"04-mapper-contract-mismatch", "12-cross-file-contract"} and path.endswith("Mapper.xml"):
        return content.replace('resultType="String"', 'resultType="com.acme.Order"')
    if cid == "05-tx-self-invocation" and path.endswith(".java") and "inner();" in content:
        return content.replace("inner();", "proxy.inner();", 1)
    if cid == "06-tx-checked-rollback" and path.endswith(".java"):
        return content.replace("@Transactional", "@Transactional(rollbackFor = Exception.class)", 1) if "@Transactional" in content else "@Transactional(rollbackFor = Exception.class)\n" + content
    if cid == "07-tx-readonly-write" and path.endswith(".java"):
        return content.replace("readOnly = true", "readOnly = false", 1)
    if cid == "08-auth-weakened" and path.endswith(".java"):
        return content.replace("public void", "@ProjectAuthorize public void", 1) if "public void" in content else "@ProjectAuthorize\n" + content
    if cid == "09-validation-omitted" and path.endswith(".java") and "repository.save" in content:
        return content.replace("repository.save", "if (amount <= 0) throw new IllegalArgumentException(); repository.save", 1)
    if cid == "10-dangerous-config" and path.endswith(".yml"):
        return content.replace("connection-timeout: 1", "connection-timeout: 30000", 1)
    if cid == "11-test-validity" and "/test/" in path.replace("\\", "/") and "service.submit();" in content:
        return content.replace("service.submit();", 'assertEquals("OK", service.submit());', 1)
    return default_before(case, file)


for klass in ("positive", "negative"):
    base = ROOT / ".code-harness/tools-runtime/testdata/review-benchmark" / klass
    for case_path in sorted(base.glob("*/case.json")):
        case = json.loads(case_path.read_text(encoding="utf-8"))
        for file in case.get("files", []):
            workspace = (file.get("workspace") or "current").strip()
            if workspace == "current" and file.get("changed"):
                before = semantic_before(case, file) if klass == "positive" else default_before(case, file)
                if before == file.get("content", ""):
                    before = default_before(case, file)
                file["beforeContent"] = before
        for relation in case.get("relations", []):
            if relation.get("role") == "MapperXml": relation["source"] = "MAPPER_STATEMENT"
            if relation.get("role") == "YamlConfig": relation["source"] = "CONFIG_REFERENCE"
        if case["id"] == "11-test-validity":
            case["ruleId"] = "TEST-VALIDITY-001"
            test_file = next(f for f in case["files"] if f.get("changed") and f.get("role") == "Test")
            hunk = next(h for h in case.get("hunks", []) if h.get("path") == test_file["path"])
            line = int(hunk.get("newStart", 1))
            for proposal in case.get("proposals", []):
                proposal["ruleId"] = "TEST-VALIDITY-001"
                proposal["anchor"] = {"kind": "LINE", "path": test_file["path"], "line": line}
                proposal["evidenceRefs"] = [{"kind": "CHANGED_RANGE", "path": test_file["path"], "startLine": line, "endLine": line}]
        if case["id"] == "20-dependency-context-only":
            for symbol in case.get("symbols", []):
                if (symbol.get("workspace") or "current") != "current":
                    symbol["workspace"] = "company-framework"
                    symbol["path"] = "src/main/java/com/company/framework/SharedPolicy.java"
        case_path.write_text(json.dumps(case, ensure_ascii=False, separators=(",", ":")) + "\n", encoding="utf-8")

# Real ReviewUnit construction must admit changed tests for dedicated Test Validity authority.
build = ROOT / ".code-harness/tools-runtime/internal/reviewunit/build.go"
text = build.read_text(encoding="utf-8")
needle = 'case strings.HasPrefix(p, "src/main/java/") && strings.HasSuffix(p, ".java"):\n\t\treturn true\n'
if 'src/test/java/' not in text[text.find('func isFindingScopePath160'):]:
    if needle not in text:
        raise SystemExit("reviewunit test scope insertion point not found")
    text = text.replace(needle, needle + '\tcase strings.HasPrefix(p, "src/test/java/") && strings.HasSuffix(p, ".java"):\n\t\treturn true\n', 1)
build.write_text(text, encoding="utf-8")

# Test is a legal analysis/ReviewUnit role without changing the locked Spring rule catalog.
for rel in (".code-harness/contracts/change-analysis.schema.json", ".code-harness/contracts/review-unit.schema.json"):
    path = ROOT / rel
    text = path.read_text(encoding="utf-8")
    if '"MapperXml", "YamlConfig", "Test", "Entity"' not in text:
        old = '"MapperXml", "YamlConfig", "Entity"'
        if old not in text:
            raise SystemExit(f"fileRole insertion point not found in {rel}")
        text = text.replace(old, '"MapperXml", "YamlConfig", "Test", "Entity"', 1)
    path.write_text(text, encoding="utf-8")

# Certified Analysis uses an existing legal navigation evidence source.
benchmark = ROOT / ".code-harness/tools-runtime/internal/finding/benchmark_test.go"
text = benchmark.read_text(encoding="utf-8").replace('"source": "BENCHMARK_RUNTIME"', '"source": "FIND_SYMBOL"')
benchmark.write_text(text, encoding="utf-8")

# Real Windows P-DEPENDENCY: verified sibling Maven dependency and exact rejection code before sentinel.
script = ROOT / ".github/scripts/task160-real-review-precision-regression.ps1"
s = script.read_text(encoding="utf-8")
if "$dependencyRoot" not in s:
    fixture_line = '$fixture = Join-Path $env:RUNNER_TEMP ("task160-review-precision-" + [guid]::NewGuid().ToString(\'N\'))\nNew-Item -ItemType Directory -Force $fixture | Out-Null\n'
    dep_vars = fixture_line + '$dependencyName = "company-framework-" + [guid]::NewGuid().ToString(\'N\')\n$dependencyRoot = Join-Path (Split-Path -Parent $fixture) $dependencyName\n'
    if fixture_line not in s: raise SystemExit("dependency variable insertion point not found")
    s = s.replace(fixture_line, dep_vars, 1)

if '<artifactId>company-framework</artifactId>' not in s.split("Invoke-Git add .", 1)[0]:
    old_pom = '  <version>1.0.0</version>\n</project>'
    new_pom = '  <version>1.0.0</version>\n  <dependencies>\n    <dependency><groupId>com.company</groupId><artifactId>company-framework</artifactId><version>2.3.1</version></dependency>\n  </dependencies>\n</project>'
    if old_pom not in s: raise SystemExit("current project pom insertion point not found")
    s = s.replace(old_pom, new_pom, 1)

base_config = """        Write-Utf8NoBom $configPath @'\nspring:\n  datasource:\n    hikari:\n      connection-timeout: 30000\n'@\n"""
if "SharedPolicy.java" not in s:
    dep_setup = """        Write-Utf8NoBom (Join-Path $dependencyRoot 'pom.xml') @'\n<project><modelVersion>4.0.0</modelVersion><groupId>com.company</groupId><artifactId>company-framework</artifactId><version>2.3.1</version></project>\n'@\n        Write-Utf8NoBom (Join-Path $dependencyRoot 'src/main/java/com/company/framework/SharedPolicy.java') @'\npackage com.company.framework; public class SharedPolicy { public void check() {} }\n'@\n"""
    if base_config not in s: raise SystemExit("dependency source insertion point not found")
    s = s.replace(base_config, base_config + dep_setup, 1)

runtime_marker = "        $script:runtime = (Resolve-Path '.code-harness\\bin\\codea-harness-tools.exe').Path\n"
if "workspaceDependencies:" not in s:
    harness_setup = """        Write-Utf8NoBom (Join-Path $fixture '.code-harness/harness.yaml') @\"\nversion: 2\nworkspaceDependencies:\n  - id: company-framework\n    root: ../$dependencyName\n    maven:\n      groupId: com.company\n      artifactId: company-framework\n    mode: READ_ONLY\n\"@\n"""
    if runtime_marker not in s: raise SystemExit("harness config insertion point not found")
    s = s.replace(runtime_marker, harness_setup + runtime_marker, 1)

s = s.replace("symbolLocations = @()", "symbolLocations = @([ordered]@{ workspace='company-framework'; symbol='SharedPolicy.check'; path='src/main/java/com/company/framework/SharedPolicy.java'; role='Service'; source='FIND_SYMBOL' })", 1)
s = s.replace("externalDependencies = @()", "externalDependencies = @('company-framework')", 1)
s = s.replace("path='workspace/shared/src/main/resources/application.yml'", "path='src/main/java/com/company/framework/SharedPolicy.java'", 1)

sentinel = "        Write-Output 'TASK160_DEPENDENCY_SCOPE_REJECTED PASS'\n"
if "P-DEPENDENCY expected FINDING_DEPENDENCY_SCOPE_FORBIDDEN" not in s:
    assertion = """        $dependencyRejection = @($certify.rejections | Where-Object { $_.proposalId -eq 'P-DEPENDENCY' }) | Select-Object -First 1\n        if ($null -eq $dependencyRejection -or [string]$dependencyRejection.code -ne 'FINDING_DEPENDENCY_SCOPE_FORBIDDEN') {\n            $actualCode = if ($null -eq $dependencyRejection) { '<missing>' } else { [string]$dependencyRejection.code }\n            throw \"Task160 P-DEPENDENCY expected FINDING_DEPENDENCY_SCOPE_FORBIDDEN, got $actualCode\"\n        }\n"""
    if sentinel not in s: raise SystemExit("dependency sentinel insertion point not found")
    s = s.replace(sentinel, assertion + sentinel, 1)

cleanup = "    Remove-Item $fixture -Recurse -Force -ErrorAction SilentlyContinue\n"
if "Remove-Item $dependencyRoot" not in s:
    if cleanup not in s: raise SystemExit("dependency cleanup insertion point not found")
    s = s.replace(cleanup, cleanup + "    Remove-Item $dependencyRoot -Recurse -Force -ErrorAction SilentlyContinue\n", 1)
script.write_text(s, encoding="utf-8")

marker = ROOT / ".github/task160-red-marker.txt"
if marker.exists(): marker.unlink()
