#!/usr/bin/env python3
"""Codea Harness 1.8.0 packaged real-model final acceptance.

The driver uses only normal OpenCode user interaction. It never invokes review
prepare/select/finish directly and never injects findings. Every matrix run
starts from a fresh project extracted from the official install ZIP.
"""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import sys
import tempfile
import time
import zipfile

# GitHub Windows runners may default Python stdout to cp1252. Controller
# display names deliberately contain Unicode (including Chinese brackets).
# Force UTF-8 for stdout/stderr before any model-evidence logging, without
# weakening or escaping the source-grounded review assertions.
for stream in (sys.stdout, sys.stderr):
    if hasattr(stream, "reconfigure"):
        stream.reconfigure(encoding="utf-8", errors="backslashreplace")

HERE = Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location("review180_host", HERE / "review180-host-smoke.py")
host = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(host)

RISKY_SQL = "UPDATE orders SET status = #{status}"
CLEAN_LITERAL_METHOD = """    public int literalOne() {
        int value = mapper.literalOne();
        return value;
    }
"""
CLEAN_LITERAL_METHOD_CHANGED = """    public int literalOne() {
        int literal = mapper.literalOne();
        return literal;
    }
"""
CLEAN_LITERAL_SQL = "SELECT 1"
SAFE_SQL = "UPDATE orders SET status = #{status} WHERE id = #{id} AND tenant_id = #{tenantId} AND status = #{expectedStatus}"


def copy_sdk(sdk_root: Path, dest: Path):
    dest.mkdir(parents=True, exist_ok=True)
    for name in ("package.json", "package-lock.json"):
        shutil.copyfile(sdk_root / name, dest / name)
    if not (dest / "node_modules").exists():
        shutil.copytree(sdk_root / "node_modules", dest / "node_modules")


def sha256_file(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def write_json(path: Path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def redact(value):
    if isinstance(value, list):
        return [redact(item) for item in value]
    if not isinstance(value, dict):
        return value
    out = {}
    for key, item in value.items():
        low = key.lower()
        if any(token in low for token in ("apikey", "api_key", "authorization", "password", "secret", "token")):
            out[key] = "<redacted>"
        else:
            out[key] = redact(item)
    return out


def session_id(stdout: str, binary: Path, project: Path, env: dict) -> str:
    match = re.search(r'"sessionID"\s*:\s*"([^"]+)"', stdout or "")
    if match:
        return match.group(1)
    rows = json.loads(host.command([str(binary), "session", "list", "--format", "json"], project, env, timeout=30))
    host.require(rows, "real-model e2e: native session missing")
    return rows[0]["id"]


def parse_tool_output(state):
    raw = state.get("output")
    if isinstance(raw, dict):
        return raw
    if isinstance(raw, str):
        return json.loads(raw)
    raise RuntimeError(f"completed tool has no JSON output: {state}")


def completed_tools(exported):
    names = []
    for message in exported.get("messages", []):
        for part in message.get("parts", []):
            if part.get("type") != "tool":
                continue
            state = part.get("state", {})
            if state.get("status") == "completed":
                names.append(part.get("tool", ""))
    return names



def audit_native_review_tools(exported, scope):
    """Audit every model tool ATTEMPT; never hide failed finish behind retry."""
    permitted = host.normalized("codea-review")
    authorized = {
        (ref["path"], ref["sha256"], ref["startLine"], ref["endLine"])
        for ref in scope.get("reads", [])
    }
    entries, failures, read_refs = [], [], set()
    for message in exported.get("messages", []):
        for part in message.get("parts", []):
            if part.get("type") != "tool":
                continue
            state = part.get("state") or {}
            tool_name = str(part.get("tool", ""))
            args = state.get("input") or {}
            action = args.get("action")
            status = state.get("status")
            entry = {"tool": tool_name, "action": action, "status": status,
                     "callID": part.get("callID"), "error": state.get("error")}
            if action == "read" and host.normalized(tool_name) == permitted:
                source = args.get("source") or {}
                key = (source.get("path"), source.get("sha256"),
                       source.get("startLine"), source.get("endLine"))
                entry["source"] = source
                if key not in authorized:
                    failures.append("unauthorized source read: " + str(source))
                elif status == "completed":
                    read_refs.add(key)
            if host.normalized(tool_name) != permitted:
                failures.append("forbidden tool attempted: " + tool_name)
            elif action not in {"prepare", "read", "finish"}:
                failures.append("invalid native review action: " + str(action))
            if status != "completed":
                failures.append("non-completed tool attempt: " + tool_name +
                                " action=" + str(action) + " status=" + str(status))
            entries.append(entry)
    acts = [e["action"] for e in entries]
    if acts.count("prepare") != 1 or acts.count("finish") != 1 or acts.count("read") < 1:
        failures.append("expected one prepare, scoped read(s), one finish: " + str(acts))
    if acts and (acts[0] != "prepare" or acts[-1] != "finish"):
        failures.append("native review tool order invalid: " + str(acts))
    if authorized and read_refs != authorized:
        failures.append("source reads did not cover prepared scope: " +
                        str(sorted(authorized - read_refs)))
    return {"schemaVersion": 1, "toolAttempts": entries, "violations": failures,
            "authorizedReadCount": len(authorized), "verifiedReadCount": len(read_refs)}

def tool_duration_ms(state):
    info = state.get("time") or {}
    start, end = info.get("start"), info.get("end")
    if isinstance(start, (int, float)) and isinstance(end, (int, float)) and end >= start:
        return int(end - start)
    return 0


def canonical_path(project: Path, value) -> str:
    raw = str(value or "").strip()
    host.require(raw, "reportPath missing")
    candidate = Path(raw)
    if not candidate.is_absolute():
        candidate = project / candidate
    return os.path.normcase(os.path.realpath(os.path.abspath(str(candidate))))


def validate_report(project: Path, report: Path, runtime: dict):
    host.require(canonical_path(project, runtime.get("reportPath")) == canonical_path(project, report), "finish reportPath mismatch")
    digest = sha256_file(report)
    host.require(runtime.get("reportSha256") == digest, "finish reportSha256 mismatch")
    return digest


def write_fixture(project: Path, multi: bool, clean_read: bool = False, multiple_controllers: bool = False):
    java = project / "src" / "main" / "java" / "com" / "example"
    xml = project / "src" / "main" / "resources" / "mapper"
    java.mkdir(parents=True, exist_ok=True)
    xml.mkdir(parents=True, exist_ok=True)

    controller_extra = ""
    service_extra = ""
    impl_extra = ""
    mapper_extra = ""
    sql_extra = ""
    if multi:
        controller_extra = '''
    @PreAuthorize("hasAuthority('ORDER_WRITE') and principal != null and principal.tenantId != null and principal.tenantId != ''")
    @PostMapping("/orders/void")
    public void voidOrder(@AuthenticationPrincipal(expression = "tenantId") String tenantId, long id) {
        service.voidOrder(tenantId, id);
    }
'''
        service_extra = "    void voidOrder(String tenantId, long id);\n"
        impl_extra = "    public void voidOrder(String tenantId, long id) { mapper.voidOrder(tenantId, id); }\n"
        mapper_extra = "    void voidOrder(String tenantId, long id);\n"
        sql_extra = '  <update id="voidOrder">UPDATE orders SET status = \'CANCELLED\' WHERE id = #{id} AND tenant_id = #{tenantId}</update>\n'

    (java / "OrderController.java").write_text(
        """package com.example;
import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.security.core.annotation.AuthenticationPrincipal;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.http.HttpStatus;
import org.springframework.web.server.ResponseStatusException;

@RestController
public class OrderController {
    private final OrderService service;
    public OrderController(OrderService service) { this.service = service; }

    @PreAuthorize("hasAuthority('ORDER_WRITE') and principal != null and principal.tenantId != null and principal.tenantId != ''")
    @PostMapping("/orders/status")
    public void updateStatus(
            @AuthenticationPrincipal(expression = "tenantId") String tenantId,
            @RequestParam("id") long id,
            @RequestParam("status") String status) {
        if (id <= 0) {
            throw new ResponseStatusException(HttpStatus.BAD_REQUEST, "invalid order id");
        }
        if (status == null) {
            throw new ResponseStatusException(HttpStatus.BAD_REQUEST, "status is required");
        }
        switch (status) {
            case "PAID", "CANCELLED" -> { }
            default -> throw new ResponseStatusException(HttpStatus.BAD_REQUEST, "unsupported status");
        }
        boolean updated = service.updateStatus(tenantId, id, status);
        if (!updated) {
            throw new ResponseStatusException(HttpStatus.CONFLICT, "order is not writable in current tenant/state");
        }
    }
""" + controller_extra + "}\n",
        encoding="utf-8",
    )
    (java / "OrderService.java").write_text(
        "package com.example;\npublic interface OrderService {\n"
        "    boolean updateStatus(String tenantId, long id, String status);\n"
        + service_extra + "}\n",
        encoding="utf-8",
    )
    (java / "OrderServiceImpl.java").write_text(
        "package com.example;\npublic class OrderServiceImpl implements OrderService {\n"
        "    private final OrderMapper mapper;\n"
        "    public OrderServiceImpl(OrderMapper mapper) { this.mapper = mapper; }\n"
        "    public boolean updateStatus(String tenantId, long id, String status) {\n"
        "        if (status == null) { return false; }\n"
        "        String expectedStatus;\n"
        "        switch (status) {\n"
        "            case \"PAID\" -> expectedStatus = \"PENDING\";\n"
        "            case \"CANCELLED\" -> expectedStatus = \"PAID\";\n"
        "            default -> { return false; }\n"
        "        }\n"
        "        int updated = mapper.updateStatus(tenantId, id, status, expectedStatus);\n"
        "        return updated == 1;\n"
        "    }\n"
        + impl_extra + "}\n",
        encoding="utf-8",
    )
    (java / "OrderMapper.java").write_text(
        "package com.example;\nimport org.apache.ibatis.annotations.Param;\npublic interface OrderMapper {\n"
        "    int updateStatus(@Param(\"tenantId\") String tenantId, @Param(\"id\") long id, @Param(\"status\") String status, @Param(\"expectedStatus\") String expectedStatus);\n"
        + mapper_extra + "}\n",
        encoding="utf-8",
    )
    (xml / "OrderMapper.xml").write_text(
        '<?xml version="1.0" encoding="UTF-8"?>\n'
        '<mapper namespace="com.example.OrderMapper">\n'
        f'  <update id="updateStatus">{SAFE_SQL}</update>\n'
        + sql_extra + "</mapper>\n",
        encoding="utf-8",
    )
    if multiple_controllers:
        host.require(multi, "multi-Controller fixture needs two real method chains")
        # A genuinely second Controller, independently identified from its
        # Java path/class and sharing both impacted downstream SQL statements.
        (java / "UserController.java").write_text(
            """package com.example;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RestController;
@RestController
public class UserController {
    private final OrderService service;
    public UserController(OrderService service) { this.service = service; }
    @PostMapping("/users/sync")
    public void syncUser(String tenantId, long id, String status) {
        service.updateStatus(tenantId, id, status);
    }
    @PostMapping("/users/close")
    public void closeUser(String tenantId, long id) {
        service.voidOrder(tenantId, id);
    }
}
""", encoding="utf-8")
    if clean_read:
        (java / "PublicSqlLiteralController.java").write_text(
            """package com.example;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class PublicSqlLiteralController {
    private final SqlLiteralService service;
    public PublicSqlLiteralController(SqlLiteralService service) { this.service = service; }

    @GetMapping("/public/reference/sql/literal-one")
    public int literalOne() {
        return service.literalOne();
    }
}
""",
            encoding="utf-8",
        )
        (java / "SqlLiteralService.java").write_text(
            "package com.example;\npublic interface SqlLiteralService {\n"
            "    int literalOne();\n"
            "}\n",
            encoding="utf-8",
        )
        (java / "SqlLiteralServiceImpl.java").write_text(
            """package com.example;
import org.springframework.stereotype.Service;

@Service
public class SqlLiteralServiceImpl implements SqlLiteralService {
    private final SqlLiteralMapper mapper;
    public SqlLiteralServiceImpl(SqlLiteralMapper mapper) { this.mapper = mapper; }
""" + CLEAN_LITERAL_METHOD + "}\n",
            encoding="utf-8",
        )
        (java / "SqlLiteralMapper.java").write_text(
            """package com.example;
import org.apache.ibatis.annotations.Mapper;

@Mapper
public interface SqlLiteralMapper {
    int literalOne();
}
""",
            encoding="utf-8",
        )
        # Co-locate mapper XML with the mapper interface classpath name so no
        # external mapper-locations configuration is required.
        clean_xml = project / "src" / "main" / "resources" / "com" / "example"
        clean_xml.mkdir(parents=True, exist_ok=True)
        (clean_xml / "SqlLiteralMapper.xml").write_text(
            '<?xml version="1.0" encoding="UTF-8"?>\n'
            '<mapper namespace="com.example.SqlLiteralMapper">\n'
            f'  <select id="literalOne" resultType="int">{CLEAN_LITERAL_SQL}</select>\n'
            "</mapper>\n",
            encoding="utf-8",
        )
    (project / "pom.xml").write_text(
        "<project><modelVersion>4.0.0</modelVersion><groupId>com.example</groupId>"
        "<artifactId>review180</artifactId><version>1</version></project>\n",
        encoding="utf-8",
    )
    (project / ".gitignore").write_text(".opencode/node_modules/\n.code-harness/runs/\n", encoding="utf-8")


def init_git(project: Path, env: dict):
    host.command(["git", "init"], project, env, timeout=20)
    host.command(["git", "add", "."], project, env, timeout=20)
    host.command(
        ["git", "-c", "user.name=Release 180 Fixture", "-c", "user.email=release180@example.test", "commit", "-m", "baseline"],
        project, env, timeout=20,
    )


def make_env(temp: Path, project: Path, sdk_root: Path, output_limit: int):
    env = dict(os.environ)
    for key, suffix in (
        ("XDG_CONFIG_HOME", "xdg-config"),
        ("XDG_DATA_HOME", "xdg-data"),
        ("XDG_CACHE_HOME", "xdg-cache"),
        ("XDG_STATE_HOME", "xdg-state"),
    ):
        env[key] = str(temp / suffix)
    env["OPENCODE_DISABLE_MODELS_FETCH"] = "true"
    env["OPENCODE_DISABLE_AUTOUPDATE"] = "true"
    env["OPENCODE_CONFIG_CONTENT"] = "{}"
    env.pop("OPENCODE_CONFIG", None)
    copy_sdk(sdk_root, project / ".opencode")
    copy_sdk(sdk_root, Path(env["XDG_CONFIG_HOME"]) / "opencode")

    config = {
        "provider": {
            "task15secret": {
                "npm": "@ai-sdk/openai-compatible",
                "name": "Configured private acceptance",
                "options": {
                    "baseURL": "{env:TASK15_OPENAI_BASE_URL}",
                    "apiKey": "{env:TASK15_OPENAI_API_KEY}",
                },
                "models": {
                    "deepseek-v4-pro": {
                        "name": "DeepSeek V4 Pro",
                        "limit": {"context": 128000, "output": output_limit},
                    }
                },
            }
        },
        "permission": {
            "*": "deny",
            "codea-review": "allow",
        },
    }
    # Enforce scope at tool availability (not merely with instructions).
    config["tools"] = {"read": False, "glob": False, "grep": False, "list": False, "bash": False}
    (project / "opencode.json").write_text(json.dumps(config), encoding="utf-8")
    return env


def extract_install(install_zip: Path, project: Path):
    project.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(install_zip) as archive:
        archive.extractall(project)
    for rel in (
        ".code-harness/bin/codea-dcep-tools.exe",
        ".code-harness/bin/ast-grep.exe",
        ".opencode/commands/harness-review.md",
        ".opencode/agents/orchestrator.md",
        ".opencode/tools/codea-review.ts",
    ):
        host.require((project / rel).is_file(), f"packaged required file missing: {rel}")


def mutate_for_scenario(project: Path, scenario: str):
    xml = project / "src" / "main" / "resources" / "mapper" / "OrderMapper.xml"
    controller = project / "src" / "main" / "java" / "com" / "example" / "OrderController.java"
    if scenario in {"single-issue", "single-affected-two-endpoint", "two-chains-select-c1", "early-stop", "timeout"}:
        text = xml.read_text(encoding="utf-8")
        host.require(SAFE_SQL in text, "safe SQL seed missing")
        xml.write_text(text.replace(SAFE_SQL, RISKY_SQL, 1), encoding="utf-8")
        source = controller.read_text(encoding="utf-8")
        safe_method = """    @PreAuthorize("hasAuthority('ORDER_WRITE') and principal != null and principal.tenantId != null and principal.tenantId != ''")
    @PostMapping("/orders/status")
    public void updateStatus(
            @AuthenticationPrincipal(expression = "tenantId") String tenantId,
            @RequestParam("id") long id,
            @RequestParam("status") String status) {
        if (id <= 0) {
            throw new ResponseStatusException(HttpStatus.BAD_REQUEST, "invalid order id");
        }
        if (status == null) {
            throw new ResponseStatusException(HttpStatus.BAD_REQUEST, "status is required");
        }
        switch (status) {
            case "PAID", "CANCELLED" -> { }
            default -> throw new ResponseStatusException(HttpStatus.BAD_REQUEST, "unsupported status");
        }
        boolean updated = service.updateStatus(tenantId, id, status);
        if (!updated) {
            throw new ResponseStatusException(HttpStatus.CONFLICT, "order is not writable in current tenant/state");
        }
    }
"""
        vulnerable_method = """    @PostMapping("/orders/status")
    public void updateStatus(String tenantId, long id, String status) {
        service.updateStatus(tenantId, id, status);
    }
"""
        host.require(safe_method in source, "safe Controller seed missing")
        controller.write_text(source.replace(safe_method, vulnerable_method, 1), encoding="utf-8")
        if scenario == "two-chains-select-c1":
            # A two-endpoint project is not necessarily a two-IMPACTED-chain
            # project. The updateStatus change above only affects one chain.
            # Independently modify voidOrder's own SQL statement, preserving
            # its tenant predicate and keeping C1's high-risk seed unchanged.
            # A file-wide attribution shortcut is forbidden.
            second_sql = "UPDATE orders SET status = 'CANCELLED' WHERE id = #{id} AND tenant_id = #{tenantId}"
            second_new = "UPDATE orders SET status = 'VOIDED' WHERE id = #{id} AND tenant_id = #{tenantId}"
            existing = xml.read_text(encoding="utf-8")
            host.require(existing.count(second_sql) == 1,
                         "two-chain fixture needs one independent voidOrder SQL statement")
            xml.write_text(existing.replace(second_sql, second_new, 1), encoding="utf-8")
    elif scenario == "single-clean":
        literal_impl = project / "src" / "main" / "java" / "com" / "example" / "SqlLiteralServiceImpl.java"
        text = literal_impl.read_text(encoding="utf-8")
        host.require(CLEAN_LITERAL_METHOD in text, "clean literal method seed missing")
        literal_impl.write_text(text.replace(CLEAN_LITERAL_METHOD, CLEAN_LITERAL_METHOD_CHANGED, 1), encoding="utf-8")
    elif scenario == "no-relevant-changes":
        (project / "README-review-note.txt").write_text("unrelated documentation change\n", encoding="utf-8")


def risky_found(findings):
    high = [item for item in findings if item.get("severity") in {"CRITICAL", "HIGH"}]
    if not high:
        return False
    for item in high:
        blob = json.dumps(item, ensure_ascii=False)
        if "UPDATE orders SET status" in blob or "tenant" in blob.lower() or "租户" in blob:
            return True
    return False


def persist_run(evidence_dir: Path, exported, run_dir: Path, record: dict):
    write_json(evidence_dir / "trajectory.json", redact(exported))
    for name in ("review.md", "result.json", "scope.json"):
        src = run_dir / name
        if src.is_file():
            shutil.copyfile(src, evidence_dir / name)
    write_json(evidence_dir / "run.json", redact(record))
    files = {}
    for name in ("trajectory.json", "review.md", "result.json", "scope.json", "run.json", "tool-audit.json"):
        path = evidence_dir / name
        if path.is_file():
            files[name] = {"sha256": sha256_file(path), "bytes": path.stat().st_size}
    write_json(evidence_dir / "manifest.json", {
        "schemaVersion": 1,
        "head": record.get("head"),
        "scenario": record.get("scenario"),
        "iteration": record.get("iteration"),
        "runId": record.get("runId"),
        "files": files,
    })


def successful_run(args, scenario: str, iteration: int, multi: bool, current_impl: bool = False, no_target: bool = False, expect_selection=None, auxiliary: bool = False):
    # Without a Controller target include all impacted Controllers; with
    # a target include only its impacted methods. No menu or select ever.
    scenario_label = f"{scenario}-auxiliary" if auxiliary else (f"{scenario}-no-target" if no_target else scenario)
    run_evidence = args.evidence_dir / f"{scenario_label}-{iteration}"
    with tempfile.TemporaryDirectory(prefix=f"Codea 180 {scenario} 空格 & # % ") as raw:
        temp = Path(raw)
        project = temp / "project"
        extract_install(args.install_zip, project)
        write_fixture(project, multi, clean_read=(scenario == "single-clean"), multiple_controllers=(scenario == "two-chains-select-c1"))
        env = make_env(temp, project, args.sdk_root, 16384)
        env["PATH"] = str(args.opencode.parent) + os.pathsep + env.get("PATH", "")
        init_git(project, env)
        mutate_for_scenario(project, scenario)

        model = "task15secret/deepseek-v4-pro"
        binary = args.opencode
        version = host.command([str(binary), "--version"], temp, env, timeout=30).strip()
        host.require(version == "1.18.25", f"OpenCode mismatch: {version}")
        command_args = ["--command", "harness-review"]
        if no_target:
            # The real Host must accept a bare /harness-review and prepare
            # CHANGES without requesting a Controller or inventing a target.
            pass
        elif current_impl:
            command_args.append("请检查当前实现 OrderController.updateStatus")
        elif scenario == "single-clean":
            command_args.append("PublicSqlLiteralController.literalOne")
        elif multi:
            # Explicit Controller class target is a scope filter, not a
            # requirement to select from a menu.
            command_args.append("OrderController")
        else:
            command_args.append("OrderController.updateStatus")

        run = [str(binary), "run", "--print-logs", "--dir", str(project), "--model", model, "--format", "json"]
        started = time.monotonic()
        stdout = host.command(run + command_args, project, env, timeout=420)
        sid = session_id(stdout, binary, project, env)
        exported = json.loads(host.command([str(binary), "export", sid], project, env, timeout=30))
        first_actions = host.tool_actions(exported)

        # Retain the actual autonomous prepare response and full tool trace
        # before asserting that prepare -> read -> finish completed.
        first_prepares = [state for state in host.tool_parts(exported, "prepare")
                          if state.get("status") == "completed"]
        first_prepare = parse_tool_output(first_prepares[-1]) if first_prepares else {}
        first_runtime = first_prepare.get("runtime", {})
        run_candidates = list((project / ".code-harness" / "runs").glob("review-*"))
        first_report = (run_candidates[0] / "review.md") if len(run_candidates) == 1 else None
        first_report_text = first_report.read_text(encoding="utf-8") if first_report and first_report.is_file() else ""
        proof = {
            "schemaVersion": 1,
            "head": args.head,
            "scenario": scenario_label,
            "iteration": iteration,
            "testKind": "REAL_MODEL_AUTONOMOUS",
            "sessionId": sid,
            "actions": first_actions,
            "runId": run_candidates[0].name if len(run_candidates) == 1 else None,
            "optionsHash": first_runtime.get("optionsHash"),
            "chains": first_runtime.get("chains"),
            "selectionRequired": first_runtime.get("selectionRequired"),
            "discoveryComplete": first_runtime.get("discoveryComplete"),
            "gaps": first_runtime.get("gaps"),
            "nextAction": first_prepare.get("nextAction"),
            "reportSha256AfterAutonomousReview": sha256_file(first_report) if first_report_text else None,
            "reportCompleteAfterAutonomousReview": '"execution":"COMPLETE"' in first_report_text,
        }
        write_json(run_evidence / "prepare-auto-scope.json", redact(proof))
        write_json(run_evidence / "trajectory-auto-scope.json", redact(exported))
        # Preserve all failed and forbidden attempts BEFORE any positive assertion.
        durable_scope = {}
        if len(run_candidates) == 1 and (run_candidates[0] / "scope.json").is_file():
            durable_scope = json.loads((run_candidates[0] / "scope.json").read_text(encoding="utf-8"))
        audit = audit_native_review_tools(exported, durable_scope)
        write_json(run_evidence / "tool-audit.json", redact(audit))
        host.require(not audit["violations"],
                     f"{scenario_label}: tool/evidence integrity violations: {audit['violations']}")
        if first_report_text:
            (run_evidence / "review-after-auto-scope.md").write_text(first_report_text, encoding="utf-8")
        # Emit ASCII-safe diagnostic JSON on Windows runners with cp1252 stdout.\n        # Full UTF-8 evidence is still persisted in prepare-auto-scope.json.\n        print("RELEASE180_PREPARE_AUTO_SCOPE " + json.dumps(redact(proof), ensure_ascii=True), flush=True)

        host.require(len(first_prepares) == 1,
                     f"{scenario_label}: expected one completed prepare: {first_actions}")
        host.require(first_runtime.get("discoveryComplete") is True,
                     f"{scenario_label}: expected fully resolved impact: {proof}")
        host.require(first_runtime.get("selectionRequired") is False,
                     f"{scenario_label}: ordinary review must never request C1 selection: {proof}")
        host.require(first_runtime.get("gaps") == [],
                     f"{scenario_label}: unexpected coverage gaps: {proof}")
        chains = first_runtime.get("chains")
        expected_controllers = (2 if scenario == "two-chains-select-c1" and no_target else 1)
        host.require(isinstance(chains,list) and len(chains)==expected_controllers,
                     f"{scenario_label}: target/all Controller filtering failed: {proof}")
        host.require("select" not in first_actions and "resume" not in first_actions,
                     f"{scenario_label}: unexpected human selection branch: {first_actions}")
        if scenario == "two-chains-select-c1":
            expected_names = ["OrderController"] + (["UserController"] if no_target else [])
            host.require(
                [str(ch.get("name","")).split("（")[0] for ch in chains] == expected_names
                and all(len(ch.get("methods",[]))==2 for ch in chains),
                f"{scenario_label}: missing true 2-method Controller groups: {proof}")
        if scenario == "single-affected-two-endpoint":
            host.require(len(chains[0].get("methods",[]))==1,
                         f"{scenario_label}: unrelated sibling method included: {proof}")
        host.require(first_report_text and proof["reportCompleteAfterAutonomousReview"],
                     f"{scenario_label}: report not completed in first Host turn: {proof}")

        actions = host.tool_actions(exported)
        names = [host.normalized(name) for name in completed_tools(exported)]
        host.require(all(host.normalized(name) == host.normalized("codea-review") for name in names),
                     f"{scenario_label}: forbidden tool execution: {names}")
        prepare_states = [state for state in host.tool_parts(exported, "prepare") if state.get("status") == "completed"]
        prepare_output = parse_tool_output(prepare_states[-1]) if prepare_states else {}
        prepare_next_action = prepare_output.get("nextAction")
        write_json(run_evidence / "precheck-trajectory.json", redact(exported))
        write_json(run_evidence / "precheck.json", {
            "scenario": scenario,
            "iteration": iteration,
            "actions": actions,
            "completedTools": names,
            "prepareNextAction": prepare_next_action,
        })
        print(
            f"RELEASE180_PRECHECK scenario={scenario} iteration={iteration} "
            f"actions={actions} completedTools={names} nextAction={prepare_next_action}",
            flush=True,
        )
        host.require(
            "prepare" in actions and "finish" in actions,
            f"{scenario}: autonomous path incomplete actions={actions} completedTools={names} nextAction={prepare_next_action}",
        )
        host.require("select" not in actions and "resume" not in actions,
                     f"{scenario_label}: legacy manual selection invoked: {actions}")
        host.require("read" in actions, f"{scenario}: no scope-constrained source read: {actions}")

        finish_states = [state for state in host.tool_parts(exported, "finish") if state.get("status") == "completed"]
        host.require(len(finish_states) == 1, f"{scenario}: finish count={len(finish_states)}")
        finish_runtime = parse_tool_output(finish_states[0]).get("runtime", {})
        host.require(finish_runtime.get("execution") == "COMPLETE", f"{scenario}: finish not complete {finish_runtime}")

        runs = list((project / ".code-harness" / "runs").glob("review-*"))
        host.require(len(runs) == 1, f"{scenario}: expected one run got {runs}")
        run_dir = runs[0]
        result = json.loads((run_dir / "result.json").read_text(encoding="utf-8"))
        scope = json.loads((run_dir / "scope.json").read_text(encoding="utf-8"))
        report = run_dir / "review.md"
        report_sha = validate_report(project, report, finish_runtime)
        findings = result.get("findings", [])

        expected = True
        if scenario in {"single-issue", "single-affected-two-endpoint", "two-chains-select-c1"}:
            expected = risky_found(findings)
            host.require(expected, f"{scenario}: expected high-risk seeded SQL/tenant finding missing: {findings}")
            # A model-provided external-schema note is not a Runtime call-chain
            # coverage gap. The FULLY resolved, changed unsafe SQL must stay
            # BLOCKING for all THREE independent real-model single-issue runs.
            # This guards the former single-issue-2 false CI PASS, where a
            # CRITICAL/HIGH/MEDIUM result became PARTIAL -> UNDETERMINED.
            if scenario == "single-issue":
                host.require(first_runtime.get("discoveryComplete") is True and
                             first_runtime.get("gaps") == [],
                             f"{scenario_label}: Runtime navigation incomplete: {proof}")
                host.require(result.get("coverage") == "COMPLETE" and
                             finish_runtime.get("coverage") == "COMPLETE" and
                             result.get("reviewConclusion") == "BLOCKING" and
                             finish_runtime.get("reviewConclusion") == "BLOCKING",
                             f"{scenario_label}: critical finding was wrongly downgraded; "
                             f"result={result.get('coverage')}/{result.get('reviewConclusion')} "
                             f"finish={finish_runtime}")
                host.require(result.get("coverageGaps") == [],
                             f"{scenario_label}: complete Runtime scope has coverage gaps: {result}")
                host.require(isinstance(result.get("informationalGaps"), list),
                             f"{scenario_label}: missing explicit model informational-gap evidence")
        elif scenario == "single-clean":
            expected = (
                not findings
                and not result.get("pendingRisks", [])
                and not result.get("gaps", [])
                and result.get("coverage") == "COMPLETE"
                and result.get("reviewConclusion") == "NO_ISSUES_FOUND"
            )
            host.require(expected, f"{scenario}: clean control produced issues: {result}")

        host.require(scope.get("selectedIds")==[f"C{i}" for i in range(1, expected_controllers+1)],
                     f"{scenario}: all authorized Controller groups not covered: {scope}")
        expected_method_chains = sum(len(c.get("methods",[])) for c in chains)
        host.require(len(scope.get("chains",[]))==expected_method_chains,
                     f"{scenario}: aggregated exact method scope mismatch: {scope}")

        total_ms = int((time.monotonic() - started) * 1000)
        nav_ms = sum(tool_duration_ms(s) for s in host.tool_parts(exported,"prepare"))
        finish_ms = sum(tool_duration_ms(s) for s in finish_states)
        record = {
            "schemaVersion": 1,
            "head": args.head,
            "scenario": scenario_label,
            "iteration": iteration,
            "testKind": "REAL_MODEL_AUTONOMOUS",
            "model": model,
            "hostVersion": version,
            "runId": run_dir.name,
            "sessionId": sid,
            "autonomousAllAffectedControllers": expected_controllers==2,
            "manualSelectionDisabled": True,
            "prepareOptionsHash": first_runtime.get("optionsHash"),
            "prepareChains": first_runtime.get("chains"),
            "prepareDiscoveryComplete": first_runtime.get("discoveryComplete"),
            "prepareSelectionRequired": first_runtime.get("selectionRequired"),
            "prepareGaps": first_runtime.get("gaps"),
            "modelInvokedFinish": True,
            "driverInvokedRuntimeBusinessSteps": False,
            "actions": actions,
            "completedTools": names,
            "execution": finish_runtime.get("execution"),
            "coverage": finish_runtime.get("coverage"),
            "reviewConclusion": finish_runtime.get("reviewConclusion"),
            "reportPath": str(finish_runtime.get("reportPath")),
            "reportSha256": report_sha,
            "toolAuditViolationCount": len(audit["violations"]),
            "expectedFindingsFound": expected,
            "selectedIds": scope.get("selectedIds", []),
            "timings": {
                "modelMs": max(0, total_ms - nav_ms - finish_ms),
                "navigationMs": nav_ms,
                "finishMs": finish_ms,
                "totalMs": total_ms,
            },
        }
        persist_run(run_evidence, exported, run_dir, record)
        print(
            f"RELEASE180_REAL_MODEL PASS scenario={scenario_label} iteration={iteration} "
            f"runId={run_dir.name} actions={actions} conclusion={result.get('reviewConclusion')} reportSha256={report_sha}",
            flush=True,
        )
        return record


def negative_run(args, scenario: str, output_limit: int = 16384, timeout: int = 120, remove_ast: bool = False):
    run_evidence = args.evidence_dir / scenario
    with tempfile.TemporaryDirectory(prefix=f"Codea 180 {scenario} 空格 & # % ") as raw:
        temp = Path(raw)
        project = temp / "project"
        extract_install(args.install_zip, project)
        write_fixture(project, False)
        env = make_env(temp, project, args.sdk_root, output_limit)
        env["PATH"] = str(args.opencode.parent) + os.pathsep + env.get("PATH", "")
        init_git(project, env)
        if scenario in {"early-stop", "timeout"}:
            mutate_for_scenario(project, "single-issue")
        if remove_ast:
            (project / ".code-harness" / "bin" / "ast-grep.exe").unlink()

        model = "task15secret/deepseek-v4-pro"
        run = [str(args.opencode), "run", "--print-logs", "--dir", str(project), "--model", model, "--format", "json"]
        errored = False
        stdout = ""
        try:
            stdout = host.command(run + ["--command", "harness-review", "OrderController.updateStatus"], project, env, timeout=timeout)
        except RuntimeError:
            errored = True

        runs = list((project / ".code-harness" / "runs").glob("review-*"))
        host.require(len(runs) <= 1, f"{scenario}: multiple runs created")
        if not runs:
            host.require(errored, f"{scenario}: no run without driver error")
            record = {
                "schemaVersion": 1, "head": args.head, "scenario": scenario,
                "testKind": "REAL_MODEL_AUTONOMOUS_NEGATIVE", "model": model,
                "hostVersion": "1.18.25", "runId": None, "modelInvokedFinish": False,
                "driverInvokedRuntimeBusinessSteps": False, "execution": "NOT_STARTED",
                "expectedFindingsFound": False,
            }
            write_json(run_evidence / "run.json", record)
            return record

        run_dir = runs[0]
        sid = None
        exported = {"messages": []}
        try:
            sid = session_id(stdout, args.opencode, project, env)
            exported = json.loads(host.command([str(args.opencode), "export", sid], project, env, timeout=30))
        except Exception:
            pass
        actions = host.tool_actions(exported)
        report = (run_dir / "review.md").read_text(encoding="utf-8")
        host.require('"execution":"COMPLETE"' not in report, f"{scenario}: failure scenario falsely completed")
        record = {
            "schemaVersion": 1,
            "head": args.head,
            "scenario": scenario,
            "testKind": "REAL_MODEL_AUTONOMOUS_NEGATIVE",
            "model": model,
            "hostVersion": "1.18.25",
            "runId": run_dir.name,
            "sessionId": sid,
            "userSelectionObserved": False,
            "modelInvokedFinish": "finish" in actions,
            "driverInvokedRuntimeBusinessSteps": False,
            "actions": actions,
            "execution": "INCOMPLETE",
            "expectedFindingsFound": False,
            "driverObservedErrorOrTimeout": errored,
        }
        persist_run(run_evidence, exported, run_dir, record)
        print(f"RELEASE180_NEGATIVE PASS scenario={scenario} runId={run_dir.name} actions={actions}", flush=True)
        return record


def no_relevant_changes_run(args):
    scenario = "no-relevant-changes"
    evidence = args.evidence_dir / scenario
    with tempfile.TemporaryDirectory(prefix="Codea 180 no relevant 空格 & # % ") as raw:
        temp = Path(raw)
        project = temp / "project"
        extract_install(args.install_zip, project)
        write_fixture(project, False)
        env = make_env(temp, project, args.sdk_root, 16384)
        env["PATH"] = str(args.opencode.parent) + os.pathsep + env.get("PATH", "")
        init_git(project, env)
        mutate_for_scenario(project, scenario)
        model = "task15secret/deepseek-v4-pro"
        run = [str(args.opencode), "run", "--print-logs", "--dir", str(project), "--model", model, "--format", "json"]
        stdout = host.command(run + ["--command", "harness-review", "OrderController.updateStatus"], project, env, timeout=300)
        sid = session_id(stdout, args.opencode, project, env)
        exported = json.loads(host.command([str(args.opencode), "export", sid], project, env, timeout=30))
        actions = host.tool_actions(exported)
        runs = list((project / ".code-harness" / "runs").glob("review-*"))
        host.require(len(runs) == 1, f"{scenario}: expected one run")
        run_dir = runs[0]
        report = (run_dir / "review.md").read_text(encoding="utf-8")
        host.require('"execution":"COMPLETE"' not in report, f"{scenario}: falsely completed unrelated target")
        record = {
            "schemaVersion": 1, "head": args.head, "scenario": scenario,
            "testKind": "REAL_MODEL_AUTONOMOUS_NEGATIVE", "model": model,
            "hostVersion": "1.18.25", "runId": run_dir.name, "sessionId": sid,
            "modelInvokedFinish": "finish" in actions,
            "driverInvokedRuntimeBusinessSteps": False, "actions": actions,
            "execution": "INCOMPLETE", "expectedFindingsFound": False,
        }
        persist_run(evidence, exported, run_dir, record)
        print(f"RELEASE180_NEGATIVE PASS scenario={scenario} runId={run_dir.name} actions={actions}", flush=True)
        return record


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--opencode", required=True, type=Path)
    parser.add_argument("--sdk-root", required=True, type=Path)
    parser.add_argument("--install-zip", required=True, type=Path)
    parser.add_argument("--evidence-dir", required=True, type=Path)
    parser.add_argument("--head", required=True)
    args = parser.parse_args()
    args.opencode = args.opencode.resolve()
    args.sdk_root = args.sdk_root.resolve()
    args.install_zip = args.install_zip.resolve()
    args.evidence_dir = args.evidence_dir.resolve()
    host.require(re.fullmatch(r"[0-9a-f]{40}", args.head), f"invalid exact head: {args.head}")
    host.require(args.install_zip.is_file(), "official install ZIP missing")
    host.require(os.environ.get("TASK15_OPENAI_BASE_URL", "").strip(), "REAL_MODEL_NOT_VERIFIED: TASK15_OPENAI_BASE_URL missing")
    host.require(os.environ.get("TASK15_OPENAI_API_KEY", "").strip(), "REAL_MODEL_NOT_VERIFIED: TASK15_OPENAI_API_KEY missing")
    args.evidence_dir.mkdir(parents=True, exist_ok=True)

    records = []
    for scenario, multi in (
        ("single-issue", False),
        ("single-clean", False),
        ("two-chains-select-c1", True),
    ):
        for iteration in range(1, 4):
            records.append(successful_run(args, scenario, iteration, multi))

    # A project with two actual entrypoints but only ONE impacted endpoint
    # must remain a single-chain review; never inflate the options menu.
    records.append(successful_run(args, "single-affected-two-endpoint", 1, True, expect_selection=False))
    # Both with-target and no-target two-impacted-chain acceptance must
    # independently PASS three fresh native real-model runs.
    records.append(successful_run(args, "single-issue", 1, False, no_target=True))
    for iteration in range(1, 4):
        records.append(successful_run(args, "two-chains-select-c1", iteration, True, no_target=True))
    # The Controller target must never drag in an unrelated second Controller.
    # No auxiliary menu-resume case exists: ordinary review has no menu.
    records.append(successful_run(args, "current-implementation-no-diff", 1, False, current_impl=True))
    records.append(no_relevant_changes_run(args))
    records.append(negative_run(args, "early-stop", output_limit=32, timeout=180))
    records.append(negative_run(args, "tool-failure", output_limit=16384, timeout=240, remove_ast=True))
    records.append(negative_run(args, "timeout", output_limit=16384, timeout=5))

    summary = {
        "schemaVersion": 1,
        "head": args.head,
        "model": "task15secret/deepseek-v4-pro",
        "hostVersion": "1.18.25",
        "matrix": records,
        "requiredFreshRuns": {
            "single-issue": 3,
            "single-clean": 3,
            "two-chains-select-c1": 3,
            "single-issue-no-target": 1,
            "two-chains-select-c1-no-target": 3,
            "single-affected-two-endpoint": 1,
    
        },
        "status": "PASS",
    }
    write_json(args.evidence_dir / "summary.json", summary)
    print(
        "RELEASE180_REAL_MODEL_MATRIX PASS "
        "singleIssue=3 singleClean=3 twoChainsSelectC1=3 twoChainsSelectC1NoTarget=3 "
        "extra=singleImpactedFromTwoEndpoints,targetedControllerOnly,zeroTargetSingle,currentImplementation,noRelevantChanges,earlyStop,toolFailure,timeout",
        flush=True,
    )


if __name__ == "__main__":
    main()
