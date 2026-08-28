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
        return content.replace("public void", "@ProjectAuthorize public void", 1) if "public void" in content else "@ProjectAuthorize " + content
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
            for p in case.get("proposals", []):
                p["ruleId"] = "TEST-VALIDITY-001"
                p["anchor"] = {"kind":"LINE","path":test_file["path"],"line":line}
                p["evidenceRefs"] = [{"kind":"CHANGED_RANGE","path":test_file["path"],"startLine":line,"endLine":line}]
        if case["id"] == "20-dependency-context-only":
            for symbol in case.get("symbols", []):
                if (symbol.get("workspace") or "current") != "current":
                    symbol["workspace"] = "company-framework"
                    symbol["path"] = "src/main/java/com/company/framework/SharedPolicy.java"
        case_path.write_text(json.dumps(case, ensure_ascii=False, separators=(",", ":")) + "\n", encoding="utf-8")

build = ROOT / ".code-harness/tools-runtime/internal/reviewunit/build.go"
text = build.read_text(encoding="utf-8")
needle = 'case strings.HasPrefix(p, "src/main/java/") && strings.HasSuffix(p, ".java"):\n\t\treturn true\n'
if 'src/test/java/' not in text[text.find('func isFindingScopePath160'):]:
    text = text.replace(needle, needle + '\tcase strings.HasPrefix(p, "src/test/java/") && strings.HasSuffix(p, ".java"):\n\t\treturn true\n', 1)
build.write_text(text, encoding="utf-8")

marker = ROOT / ".github/task160-red-marker.txt"
if marker.exists(): marker.unlink()
