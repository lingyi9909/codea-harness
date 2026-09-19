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
import tempfile
import time
import zipfile

HERE = Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location("review180_host", HERE / "review180-host-smoke.py")
host = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(host)

RISKY_SQL = "UPDATE orders SET status = #{status}"
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


def write_fixture(project: Path, multi: bool):
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
            "read": "allow",
            "glob": "allow",
            "grep": "allow",
            "list": "allow",
        },
    }
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
    if scenario in {"single-issue", "two-chains-select-c1", "early-stop", "timeout"}:
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
        service.updateStatus(tenantId, id, status);
    }
"""
        vulnerable_method = """    @PostMapping("/orders/status")
    public void updateStatus(String tenantId, long id, String status) {
        service.updateStatus(tenantId, id, status);
    }
"""
        host.require(safe_method in source, "safe Controller seed missing")
        controller.write_text(source.replace(safe_method, vulnerable_method, 1), encoding="utf-8")
    elif scenario == "single-clean":
        text = controller.read_text(encoding="utf-8")
        before = 'throw new ResponseStatusException(HttpStatus.BAD_REQUEST, "unsupported status");'
        after = 'throw new ResponseStatusException(HttpStatus.BAD_REQUEST, "status must be PAID or CANCELLED");'
        host.require(before in text, "safe explicit status validation seed missing")
        controller.write_text(text.replace(before, after, 1), encoding="utf-8")
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
    for name in ("trajectory.json", "review.md", "result.json", "scope.json", "run.json"):
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


def successful_run(args, scenario: str, iteration: int, multi: bool, current_impl: bool = False):
    run_evidence = args.evidence_dir / f"{scenario}-{iteration}"
    with tempfile.TemporaryDirectory(prefix=f"Codea 180 {scenario} 空格 & # % ") as raw:
        temp = Path(raw)
        project = temp / "project"
        extract_install(args.install_zip, project)
        write_fixture(project, multi)
        env = make_env(temp, project, args.sdk_root, 16384)
        env["PATH"] = str(args.opencode.parent) + os.pathsep + env.get("PATH", "")
        init_git(project, env)
        mutate_for_scenario(project, scenario)

        model = "task15secret/deepseek-v4-pro"
        binary = args.opencode
        version = host.command([str(binary), "--version"], temp, env, timeout=30).strip()
        host.require(version == "1.18.25", f"OpenCode mismatch: {version}")
        command_args = ["--command", "harness-review"]
        if current_impl:
            command_args.append("请检查当前实现 OrderController.updateStatus")
        elif multi:
            # Class target is required to expose both endpoint chains and force
            # the real next-user selection boundary. A method target would
            # collapse this acceptance case to one chain.
            command_args.append("OrderController")
        else:
            command_args.append("OrderController.updateStatus")

        run = [str(binary), "run", "--print-logs", "--dir", str(project), "--model", model, "--format", "json"]
        started = time.monotonic()
        stdout = host.command(run + command_args, project, env, timeout=420)
        sid = session_id(stdout, binary, project, env)
        exported = json.loads(host.command([str(binary), "export", sid], project, env, timeout=30))
        first_actions = host.tool_actions(exported)

        user_selection = False
        if multi:
            host.require("prepare" in first_actions, f"{scenario}: prepare missing before selection: {first_actions}")
            host.require("select" not in first_actions and "finish" not in first_actions, f"{scenario}: crossed selection boundary: {first_actions}")
            runs = list((project / ".code-harness" / "runs").glob("review-*"))
            host.require(len(runs) == 1, f"{scenario}: durable run missing before selection")
            before_report = (runs[0] / "review.md").read_text(encoding="utf-8")
            host.require('"execution":"COMPLETE"' not in before_report, f"{scenario}: report completed before user selection")
            host.command(run + ["--session", sid, "选择", "C1"], project, env, timeout=420)
            user_selection = True
            exported = json.loads(host.command([str(binary), "export", sid], project, env, timeout=30))

        actions = host.tool_actions(exported)
        names = [host.normalized(name) for name in completed_tools(exported)]
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
        if multi:
            host.require(actions.count("select") == 1, f"{scenario}: expected one real-user select: {actions}")
        else:
            host.require("select" not in actions, f"{scenario}: single chain unexpectedly selected: {actions}")
        host.require("read" in names, f"{scenario}: no native source read: {names}")

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
        if scenario in {"single-issue", "two-chains-select-c1"}:
            expected = risky_found(findings)
            host.require(expected, f"{scenario}: expected high-risk seeded SQL/tenant finding missing: {findings}")
        elif scenario == "single-clean":
            expected = not findings and result.get("reviewConclusion") == "NO_ISSUES_FOUND"
            host.require(expected, f"{scenario}: clean control produced issues: {result}")

        if multi:
            host.require(scope.get("selectedIds") == ["C1"], f"{scenario}: selected scope mismatch: {scope.get('selectedIds')}")

        total_ms = int((time.monotonic() - started) * 1000)
        nav_ms = sum(tool_duration_ms(s) for a in ("prepare", "select") for s in host.tool_parts(exported, a))
        finish_ms = sum(tool_duration_ms(s) for s in finish_states)
        record = {
            "schemaVersion": 1,
            "head": args.head,
            "scenario": scenario,
            "iteration": iteration,
            "testKind": "REAL_MODEL_AUTONOMOUS",
            "model": model,
            "hostVersion": version,
            "runId": run_dir.name,
            "sessionId": sid,
            "userSelectionObserved": user_selection,
            "modelInvokedFinish": True,
            "driverInvokedRuntimeBusinessSteps": False,
            "actions": actions,
            "completedTools": names,
            "execution": finish_runtime.get("execution"),
            "coverage": finish_runtime.get("coverage"),
            "reviewConclusion": finish_runtime.get("reviewConclusion"),
            "reportPath": str(finish_runtime.get("reportPath")),
            "reportSha256": report_sha,
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
            f"RELEASE180_REAL_MODEL PASS scenario={scenario} iteration={iteration} "
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
        mutate_for_scenario(project, "single-issue" if scenario in {"early-stop", "timeout"} else "single-clean")
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
        },
        "status": "PASS",
    }
    write_json(args.evidence_dir / "summary.json", summary)
    print(
        "RELEASE180_REAL_MODEL_MATRIX PASS "
        "singleIssue=3 singleClean=3 twoChainsSelectC1=3 "
        "extra=currentImplementation,noRelevantChanges,earlyStop,toolFailure,timeout",
        flush=True,
    )


if __name__ == "__main__":
    main()
