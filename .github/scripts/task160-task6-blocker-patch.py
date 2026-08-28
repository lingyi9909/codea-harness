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
    hunk_line = 1
    for h in case.get("hunks", []):
        if h.get("path") == file.get("path"):
            hunk_line = h.get("newStart", 1)
            break
    suffix = " # benchmark-before" if file.get("path", "").endswith((".yml", ".yaml")) else " // benchmark-before"
    if file.get("path", "").endswith(".xml"):
        suffix = " <!-- benchmark-before -->"
    return mutate_line(content, hunk_line, suffix)


def semantic_before(case_id, file):
    content = file.get("content", "")
    path = file.get("path", "")
    if case_id == "01-mybatis-weak-where" and path.endswith("Mapper.xml"):
        return content.replace("</update>", " WHERE id = #{id}</update>")
    if case_id == "02-mybatis-tenant-removed" and path.endswith("Mapper.xml"):
        return content.replace("</update>", " WHERE tenant_id = #{tenantId}</update>")
    if case_id == "03-mybatis-dollar-bind" and path.endswith("Mapper.xml"):
        before = content.replace("${sort}", "created_at")
        if "ORDER BY created_at" not in before:
            before = before.replace("ORDER BY", "ORDER BY created_at /*") + (" */" if "ORDER BY" in content else "")
        return before
    if case_id in {"04-mapper-contract-mismatch", "12-cross-file-contract"} and path.endswith("Mapper.xml"):
        return content.replace('resultType="String"', 'resultType="com.acme.Order"')
    if case_id == "05-tx-self-invocation" and path.endswith(".java"):
        if "inner();" in content:
            return content.replace("inner();", "proxy.inner();", 1)
        return default_before({"hunks": case.get("hunks", [])}, file)
    if case_id == "06-tx-checked-rollback" and path.endswith(".java"):
        if "@Transactional" in content:
            return content.replace("@Transactional", "@Transactional(rollbackFor = Exception.class)", 1)
        return "@Transactional(rollbackFor = Exception.class)\n" + content
    if case_id == "07-tx-readonly-write" and path.endswith(".java"):
        return content.replace("readOnly = true", "readOnly = false", 1)
    if case_id == "08-auth-weakened" and path.endswith(".java"):
        if "public void" in content:
            return content.replace("public void", "@ProjectAuthorize public void", 1)
        return "@ProjectAuthorize\n" + content
    if case_id == "09-validation-omitted" and path.endswith(".java"):
        if "repository.save" in content:
            return content.replace("repository.save", "if (amount <= 0) throw new IllegalArgumentException(); repository.save", 1)
        return default_before({"hunks": case.get("hunks", [])}, file)
    if case_id == "10-dangerous-config" and path.endswith(".yml"):
        return content.replace("connection-timeout: 1", "connection-timeout: 30000", 1)
    if case_id == "11-test-validity" and "/test/" in path.replace("\\", "/"):
        if "service.submit();" in content:
            return content.replace("service.submit();", 'assertEquals("OK", service.submit());', 1)
        return default_before({"hunks": case.get("hunks", [])}, file)
    return default_before({"hunks": case.get("hunks", [])}, file)


for klass in ("positive", "negative"):
    for case_path in sorted((ROOT / ".code-harness/tools-runtime/testdata/review-benchmark" / klass).glob("*/case.json")):
        case = json.loads(case_path.read_text(encoding="utf-8"))
        cid = case["id"]
        for file in case.get("files", []):
            workspace = (file.get("workspace") or "current").strip()
            if workspace == "current" and file.get("changed"):
                before = semantic_before(cid, file) if klass == "positive" else default_before(case, file)
                if before == file.get("content", ""):
                    before = default_before(case, file)
                file["beforeContent"] = before
        for relation in case.get("relations", []):
            if relation.get("role") == "MapperXml":
                relation["source"] = "MAPPER_STATEMENT"
            elif relation.get("role") == "YamlConfig":
                relation["source"] = "CONFIG_REFERENCE"
        if cid == "11-test-validity":
            case["ruleId"] = "TEST-VALIDITY-001"
            test_file = next(f for f in case["files"] if f.get("changed") and f.get("role") == "Test")
            hunk = next(h for h in case.get("hunks", []) if h.get("path") == test_file["path"])
            line = int(hunk.get("newStart", 1))
            for proposal in case.get("proposals", []):
                proposal["ruleId"] = "TEST-VALIDITY-001"
                proposal["anchor"] = {"kind": "LINE", "path": test_file["path"], "line": line}
                proposal["evidenceRefs"] = [{"kind": "CHANGED_RANGE", "path": test_file["path"], "startLine": line, "endLine": line}]
        if cid == "20-dependency-context-only":
            for symbol in case.get("symbols", []):
                if (symbol.get("workspace") or "current") != "current":
                    symbol["workspace"] = "company-framework"
                    symbol["path"] = "src/main/java/com/company/framework/SharedPolicy.java"
        case_path.write_text(json.dumps(case, ensure_ascii=False, separators=(",", ":")) + "\n", encoding="utf-8")

# Changed tests are legitimate ReviewUnit scope only for TEST_VALIDITY authority.
build = ROOT / ".code-harness/tools-runtime/internal/reviewunit/build.go"
text = build.read_text(encoding="utf-8")
needle = 'case strings.HasPrefix(p, "src/main/java/") && strings.HasSuffix(p, ".java"):\n\t\treturn true\n'
replacement = needle + '\tcase strings.HasPrefix(p, "src/test/java/") && strings.HasSuffix(p, ".java"):\n\t\treturn true\n'
if 'src/test/java/' not in text[text.find('func isFindingScopePath160'):]:
    if needle not in text:
        raise SystemExit("reviewunit scope insertion point not found")
    text = text.replace(needle, replacement, 1)
build.write_text(text, encoding="utf-8")

# Formal quality workflow follows go.mod exactly.
workflow = ROOT / ".github/workflows/task160-review-precision.yml"
w = workflow.read_text(encoding="utf-8").replace("go-version: '1.26.5'", "go-version: '1.23.10'")
workflow.write_text(w, encoding="utf-8")

# Real Windows P-DEPENDENCY: verified sibling Maven dependency + exact rejection code.
script = ROOT / ".github/scripts/task160-real-review-precision-regression.ps1"
s = script.read_text(encoding="utf-8")
s = s.replace("""  <version>1.0.0</version>\n</project>""", """  <version>1.0.0</version>\n  <dependencies>\n    <dependency><groupId>com.company</groupId><artifactId>company-framework</artifactId><version>2.3.1</version></dependency>\n  </dependencies>\n</project>""", 1)
insert_after = """        Write-Utf8NoBom $configPath @'\nspring:\n  datasource:\n    hikari:\n      connection-timeout: 30000\n'@\n"""
dep_setup = """        $dependencyRoot = Join-Path (Split-Path -Parent $fixture) 'company-framework'\n        Write-Utf8NoBom (Join-Path $dependencyRoot 'pom.xml') @'\n<project><modelVersion>4.0.0</modelVersion><groupId>com.company</groupId><artifactId>company-framework</artifactId><version>2.3.1</version></project>\n'@\n        Write-Utf8NoBom (Join-Path $dependencyRoot 'src/main/java/com/company/framework/SharedPolicy.java') @'\npackage com.company.framework; public class SharedPolicy { public void check() {} }\n'@\n"""
if "$dependencyRoot" not in s:
    s = s.replace(insert_after, insert_after + dep_setup, 1)
harness_marker = """        $script:runtime = (Resolve-Path '.code-harness\\bin\\codea-harness-tools.exe').Path\n"""
harness_setup = """        Write-Utf8NoBom (Join-Path $fixture '.code-harness/harness.yaml') @'\nworkspaceDependencies:\n  - id: company-framework\n    root: ../company-framework\n    maven:\n      groupId: com.company\n      artifactId: company-framework\n    mode: READ_ONLY\n'@\n"""
if "workspaceDependencies:" not in s:
    s = s.replace(harness_marker, harness_setup + harness_marker, 1)
s = s.replace("symbolLocations = @()", "symbolLocations = @([ordered]@{ workspace='company-framework'; symbol='SharedPolicy.check'; path='src/main/java/com/company/framework/SharedPolicy.java'; role='Service'; source='WORKSPACE_NAVIGATION' })", 1)
s = s.replace("externalDependencies = @()", "externalDependencies = @('company-framework')", 1)
s = s.replace("path='workspace/shared/src/main/resources/application.yml'", "path='src/main/java/com/company/framework/SharedPolicy.java'", 1)
old_assert = """        foreach ($rejectedId in @('P-BAD-LINE','P-INVENTED-SYMBOL','P-DEPENDENCY')) {\n            if ($rejectedIds -notcontains $rejectedId) { throw \"Task160 expected rejection for $rejectedId\" }\n        }\n"""
new_assert = old_assert + """        $dependencyRejection = @($certified.rejections | Where-Object { $_.proposalId -eq 'P-DEPENDENCY' }) | Select-Object -First 1\n        if ($null -eq $dependencyRejection -or [string]$dependencyRejection.code -ne 'FINDING_DEPENDENCY_SCOPE_FORBIDDEN') {\n            throw \"Task160 P-DEPENDENCY expected FINDING_DEPENDENCY_SCOPE_FORBIDDEN, got $($dependencyRejection.code)\"\n        }\n"""
if "P-DEPENDENCY expected FINDING_DEPENDENCY_SCOPE_FORBIDDEN" not in s:
    if old_assert not in s:
        raise SystemExit("dependency rejection assertion insertion point not found")
    s = s.replace(old_assert, new_assert, 1)
script.write_text(s, encoding="utf-8")

# RED marker is intentionally transient.
marker = ROOT / ".github/task160-red-marker.txt"
if marker.exists():
    marker.unlink()
