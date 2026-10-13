#!/usr/bin/env python3
"""Codea Harness 1.8 native OpenCode Host transport smoke.

The loopback provider is deterministic and validates Host/tool plumbing only; it is
not evidence of semantic model quality. The driver sends only normal user input
(`/harness-review ...` and, for multi-chain, the next real user selection). It
never invokes review prepare/select/finish directly.
"""
import argparse
import concurrent.futures
import json
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import tempfile
import threading
import time
import zipfile
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.request import ProxyHandler, build_opener

TOOL_NAME = "codea-review"
RUN_ID_FIELD_RE = re.compile(r'"runId"\s*:\s*"(review-[A-Za-z0-9._-]+)"')


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def normalized(name):
    return "".join(c for c in name.lower() if c.isalnum())


def flatten(value):
    if isinstance(value, str):
        return value
    if isinstance(value, list):
        return "\n".join(flatten(v) for v in value)
    if isinstance(value, dict):
        return "\n".join(flatten(v) for v in value.values())
    return str(value)


def json_objects(messages):
    out = []
    for message in messages:
        content = message.get("content")
        values = content if isinstance(content, list) else [content]
        for value in values:
            candidates = []
            if isinstance(value, str):
                candidates.append(value)
            elif isinstance(value, dict):
                for key in ("content", "text"):
                    if isinstance(value.get(key), str):
                        candidates.append(value[key])
            for raw in candidates:
                raw = raw.strip()
                if not raw.startswith("{"):
                    continue
                try:
                    out.append(json.loads(raw))
                except json.JSONDecodeError:
                    pass
    return out


def latest_tool_payload(messages):
    for obj in reversed(json_objects(messages)):
        if isinstance(obj, dict) and ("runtime" in obj or "source" in obj):
            return obj
    return None


def call(tool_name, arguments, suffix):
    return {
        "id": f"call_review180_{suffix}_{time.time_ns()}",
        "type": "function",
        "function": {"name": tool_name, "arguments": json.dumps(arguments, ensure_ascii=False)},
    }


class Provider(BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def reply(self, body, *, content=None, tool_call=None, tool_calls=None):
        base = {"id": "chatcmpl-review180", "created": int(time.time()), "model": body.get("model", "review180")}
        calls = tool_calls if tool_calls is not None else ([tool_call] if tool_call is not None else [])
        finish = "tool_calls" if calls else "stop"
        message = {"role": "assistant", "content": content}
        if calls:
            message = {"role": "assistant", "content": None, "tool_calls": calls}
        self.send_response(200)
        if body.get("stream"):
            self.send_header("Content-Type", "text/event-stream")
            self.end_headers()
            delta = {"role": "assistant"}
            if not calls:
                delta["content"] = content or ""
            else:
                delta["tool_calls"] = [{"index": i, **item} for i, item in enumerate(calls)]
            for part, reason in [(delta, None), ({}, finish)]:
                chunk = {**base, "object": "chat.completion.chunk", "choices": [{"index": 0, "delta": part, "finish_reason": reason}]}
                self.wfile.write(("data: " + json.dumps(chunk, ensure_ascii=False) + "\n\n").encode("utf-8"))
            self.wfile.write(b"data: [DONE]\n\n")
        else:
            raw = json.dumps({**base, "object": "chat.completion", "choices": [{"index": 0, "message": message, "finish_reason": finish}]}, ensure_ascii=False).encode("utf-8")
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)
        self.wfile.flush()

    def do_GET(self):
        raw = b'{"ready":true}'
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def do_POST(self):
        try:
            body = json.loads(self.rfile.read(int(self.headers.get("Content-Length", "0"))))
            tools = body.get("tools", [])
            names = [item.get("function", {}).get("name", "") for item in tools]
            messages = body.get("messages", [])
            self.server.requests.append({"scenario": self.server.scenario, "names": names, "messages": messages})
            if self.server.scenario == "orchestrator-permission-check":
                # OpenCode makes title/summary requests with NO advertised
                # tools, even for an unrestricted agent. These requests do
                # not exercise Agent tool permissions and must not count as
                # evidence for or against Test/Debug/Fix compatibility.
                if not names:
                    self.reply(body, content="non-tool title or summary request")
                    return
                for builtin in ("read", "glob", "bash"):
                    require(builtin in names, f"unrestricted orchestrator lost {builtin}: {names}")
                self.reply(body, content="orchestrator permissions preserved")
                return
            if not names:
                self.reply(body, content="Codea Harness 1.8 Host smoke")
                return
            # A fresh, ordinarily configured OpenCode project must rely only
            # on the installed Review Agent restriction, never on opencode.json
            # global tool disabling or a CI-injected permission override.
            leaked = [name for name in names if normalized(name) != normalized(TOOL_NAME)]
            require(not leaked, f"restricted Review exposed non-review tools: {leaked}")
            if self.server.scenario.startswith("forbidden-builtin-"):
                builtin = self.server.scenario.rsplit("-", 1)[-1]
                if self.server.scenario not in self.server.forbidden_issued:
                    self.server.forbidden_issued.add(self.server.scenario)
                    if builtin == "read":
                        args = {"filePath": str(self.server.project / ".opencode/agents/orchestrator.md")}
                    elif builtin == "glob":
                        args = {"pattern": "**/*", "path": str(self.server.project)}
                    elif builtin == "bash":
                        args = {"command": 'node -e "require(\'fs\').writeFileSync(\'isolation-breached.txt\',\'BREACH\')"'}
                    else:
                        raise RuntimeError("unexpected forbidden builtin")
                    self.reply(body, tool_call=call(builtin, args, "forbidden_" + builtin))
                else:
                    self.reply(body, content="forbidden tool attempt ended")
                return
            tool_name = next((name for name in names if normalized(name) == normalized(TOOL_NAME)), None)
            require(tool_name is not None, f"{TOOL_NAME} not advertised: {names}")

            whole = flatten(messages)
            if self.server.scenario == "missing-entry":
                if "CODEA_REVIEW_HOST_ENTRY_REQUIRED" in whole:
                    self.reply(body, content="missing entry correctly rejected")
                else:
                    self.reply(body, tool_call=call(tool_name, {"action": "prepare", "intent": {"mode": "CHANGES"}}, "unbound_prepare"))
                return
            match = RUN_ID_FIELD_RE.search(whole)
            require(match is not None, "review start runId was not injected before first model request")
            run_id = match.group(1)
            payload = latest_tool_payload(messages)

            if self.server.scenario == "wrong-run-id":
                if "CODEA_REVIEW_RUN_ID_MISMATCH" in whole:
                    self.reply(body, content="model run override correctly rejected")
                else:
                    wrong = run_id[:-1] + ("0" if run_id[-1] != "0" else "1")
                    self.reply(body, tool_call=call(tool_name, {"action": "prepare", "runId": wrong, "intent": {"mode": "CHANGES"}}, "wrong_run_prepare"))
                return

            if self.server.scenario == "forbidden-source" and "CODEA_REVIEW_SOURCE_OUTSIDE_SCOPE" in whole:
                self.reply(body, content="native source scope correctly rejected")
                return

            if self.server.scenario == "concurrent" and ("REVIEW_FINISH_OVERWRITE_REJECTED" in whole or '"execution": "COMPLETE"' in whole):
                self.reply(body, content="concurrent finish race observed")
                return

            if payload is None:
                self.reply(body, tool_call=call(tool_name, {
                    "action": "prepare",
                    "intent": {"mode": "CHANGES", **({} if self.server.scenario == "multi" else {"target": "OrderController"})},
                }, "prepare"))
                return

            runtime = payload.get("runtime", {}) if isinstance(payload, dict) else {}
            scope = payload.get("scope") if isinstance(payload, dict) else None
            if isinstance(runtime, dict) and "optionsHash" in runtime:
                chains = runtime.get("chains", [])
                require(runtime.get("selectionRequired") is False,
                        "ordinary Review must auto-scope all impacted Controllers; manual selection forbidden")


            if isinstance(payload, dict) and isinstance(payload.get("source"), dict):
                scoped = self.server.scope_by_run.get(run_id)
                require(isinstance(scoped, dict),
                        "native authorized source read returned without prepared scope")
                source = payload["source"]
                ref = source.get("ref", {})
                lines = source.get("lines", [])
                require(isinstance(lines, list) and lines,
                        "native authorized source read returned no source lines")
                allowed = [r for r in scoped.get("findingReads", [])
                           if r.get("path") == source["ref"]["path"] and
                           r.get("sha256") == source["ref"]["sha256"]]
                require(allowed, "native read missing scoped method evidence ranges")
                require(source.get("findingRanges") == allowed,
                        "native read method evidence ranges do not match immutable Runtime scope")
                for row in lines:
                    line = row.get("line")
                    expected = any(r["startLine"] <= line <= r["endLine"] for r in allowed)
                    require(row.get("evidenceEligible") is expected,
                            f"native read evidenceEligible mismatch: {source['ref']['path']}:{line}")
                if source["ref"]["path"].endswith("Controller.java"):
                    require(any(row["evidenceEligible"] for row in lines),
                            "Controller read contains no authorized method evidence")
                    require(any(not row["evidenceEligible"] for row in lines),
                            "Controller read failed to separate whole-file context from method evidence")
                    require(not any(row["evidenceEligible"] and
                                    row["text"].lstrip().startswith("import ")
                                    for row in lines),
                            "Controller import incorrectly authorized as method evidence")
                refs = scoped["reads"]
                index = self.server.read_index[run_id]
                require(ref == refs[index - 1],
                        f"native source read ref mismatch: expected {refs[index - 1]}, got {ref}")
                if index < len(refs):
                    self.server.read_index[run_id] = index + 1
                    self.reply(body, tool_call=call(tool_name, {
                        "action": "read", "source": refs[index],
                    }, f"authorized_read_{index}"))
                else:
                    self.reply(body, tool_call=call(tool_name, {
                        "action": "finish",
                        "result": {"reads": refs, "findings": [], "pendingRisks": [], "gaps": []},
                    }, "finish"))
                return

            if isinstance(scope, dict) and scope.get("reads") is not None:
                if self.server.scenario == "forbidden-source":
                    forged = dict(scope["reads"][0], path="src/main/java/com/example/OutsideController.java")
                    self.reply(body, tool_call=call(tool_name, {
                        "action": "read", "source": forged,
                    }, "scope_denied"))
                    return
                if self.server.scenario == "concurrent":
                    controller_ref = next(ref for ref in scope.get("reads", []) if ref.get("path", "").endswith("OrderController.java"))
                    common = {
                        "reads": scope.get("reads", []),
                        "pendingRisks": [],
                        "gaps": [],
                    }
                    a = {**common, "findings": []}
                    b = {**common, "findings": [{
                        "id": "F-concurrent-B",
                        "severity": "HIGH",
                        "problem": "concurrent B result",
                        "impact": "proves different finish payloads remain isolated",
                        "recommendation": "keep one immutable request per invocation",
                        "verification": "rerun concurrent Host regression",
                        "evidence": [{"ref": controller_ref, "quote": "service.create();"}],
                    }]}
                    self.reply(body, tool_calls=[
                        call(tool_name, {"action": "finish", "result": a}, "finish_a"),
                        call(tool_name, {"action": "finish", "result": b}, "finish_b"),
                    ])
                    return
                refs = scope.get("reads", [])
                require(refs, "prepared scope must supply authorized source reads")
                self.server.scope_by_run[run_id] = scope
                self.server.read_index[run_id] = 1
                self.reply(body, tool_call=call(tool_name, {
                    "action": "read", "source": refs[0],
                }, "authorized_read_0"))
                return

            if isinstance(runtime, dict) and runtime.get("execution") == "COMPLETE":
                self.reply(body, content=f"评审完成：{runtime.get('reportPath', '')}")
                return

            self.reply(body, content="Host smoke stopped without a valid next action.")
        except Exception as error:
            self.server.errors.append(str(error))
            raw = json.dumps({"error": str(error)}).encode("utf-8")
            self.send_response(500)
            self.send_header("Content-Length", str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)


def command(argv, cwd, env, timeout=150):
    options = {
        "cwd": cwd, "env": env, "stdout": subprocess.PIPE, "stderr": subprocess.PIPE,
        "text": True, "encoding": "utf-8", "errors": "replace",
    }
    if os.name == "nt":
        options["creationflags"] = subprocess.CREATE_NEW_PROCESS_GROUP
    else:
        options["start_new_session"] = True
    process = subprocess.Popen(argv, **options)
    try:
        stdout, stderr = process.communicate(timeout=timeout)
    except subprocess.TimeoutExpired:
        if os.name == "nt":
            subprocess.run(["taskkill", "/PID", str(process.pid), "/T", "/F"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=False)
        else:
            os.killpg(process.pid, signal.SIGKILL)
        stdout, stderr = process.communicate(timeout=15)
        raise RuntimeError(f"command timeout: {argv}\nSTDOUT:\n{stdout}\nSTDERR:\n{stderr}")
    if process.returncode != 0:
        raise RuntimeError(f"command failed ({process.returncode}): {argv}\nSTDOUT:\n{stdout}\nSTDERR:\n{stderr}")
    return stdout


def write_fixture(project, multi):
    java = project / "src" / "main" / "java" / "com" / "example"
    xml = project / "src" / "main" / "resources" / "mapper"
    java.mkdir(parents=True)
    xml.mkdir(parents=True)
    endpoints = '''    @PostMapping("/orders/create")\n    public void create() { service.create(); }\n'''
    service_methods = "    public void create();\n"
    impl_methods = "    public void create() { mapper.insertOrder(); }\n"
    mapper_methods = "    void insertOrder();\n"
    sql = '  <insert id="insertOrder">INSERT INTO orders(id) VALUES (1)</insert>\n'
    if multi:
        endpoints += '''    @PostMapping("/orders/cancel")\n    public void cancel() { service.cancel(); }\n'''
        service_methods += "    public void cancel();\n"
        impl_methods += "    public void cancel() { mapper.cancelOrder(); }\n"
        mapper_methods += "    void cancelOrder();\n"
        sql += '  <update id="cancelOrder">UPDATE orders SET status = 0 WHERE id = 1</update>\n'
    if multi:
        (java / "UserController.java").write_text(
            'package com.example;\n'
            'import org.springframework.web.bind.annotation.PostMapping;\n'
            'import org.springframework.web.bind.annotation.RestController;\n'
            '@RestController\n'
            'public class UserController {\n'
            '    private final OrderService service;\n'
            '    public UserController(OrderService service) { this.service=service; }\n'
            '    @PostMapping("/users/create")\n'
            '    public void syncUser() { service.create(); }\n'
            '    @PostMapping("/users/cancel")\n'
            '    public void closeUser() { service.cancel(); }\n'
            '}\n',encoding="utf-8")
    (java / "OrderController.java").write_text('''package com.example;\nimport org.springframework.web.bind.annotation.PostMapping;\nimport org.springframework.web.bind.annotation.RestController;\n@RestController\npublic class OrderController {\n    private final OrderService service;\n    public OrderController(OrderService service) { this.service = service; }\n''' + endpoints + "}\n", encoding="utf-8")
    (java / "OrderService.java").write_text("package com.example;\npublic interface OrderService {\n" + service_methods + "}\n", encoding="utf-8")
    (java / "OrderServiceImpl.java").write_text("package com.example;\npublic class OrderServiceImpl implements OrderService {\n    private final OrderMapper mapper;\n    public OrderServiceImpl(OrderMapper mapper) { this.mapper = mapper; }\n" + impl_methods + "}\n", encoding="utf-8")
    (java / "OrderMapper.java").write_text("package com.example;\npublic interface OrderMapper {\n" + mapper_methods + "}\n", encoding="utf-8")
    (xml / "OrderMapper.xml").write_text('''<?xml version="1.0" encoding="UTF-8"?>\n<mapper namespace="com.example.OrderMapper">\n''' + sql + "</mapper>\n", encoding="utf-8")
    (project / "pom.xml").write_text("<project><modelVersion>4.0.0</modelVersion><groupId>com.example</groupId><artifactId>review180</artifactId><version>1</version></project>\n", encoding="utf-8")


def bootstrap_project(temp, args, sdk_root, port, multi, name=None):
    project = temp / (name or ("multi" if multi else "single"))
    project.mkdir(parents=True, exist_ok=True)
    install = args.package_stage / "install.ps1"
    require(install.is_file(), "official install package must include install.ps1")
    outcome = command(["pwsh", "-NoProfile", "-File", str(install),
                       "-ProjectRoot", str(project)], project, os.environ, timeout=90)
    require("INSTALL_REVIEWER_HOST_RESOURCES PASS" in outcome,
            f"real package installer did not install Host resources: {outcome}")
    for rel in (".opencode/agents/orchestrator.md",
                ".opencode/agents/codea-review-restricted.md",
                ".opencode/commands/harness-review.md",
                ".opencode/tools/codea-review.ts",
                ".code-harness/bin/codea-dcep-tools.exe",
                ".code-harness/bin/ast-grep.exe"):
        require((project / rel).is_file(), f"official installer omitted {rel}")
    require((project / ".opencode/agents/codea-review-restricted.md").read_text(
        encoding="utf-8").find('"codea-review": allow') >= 0,
        "installed Review Agent does not permit only native codea-review")
    write_fixture(project, multi)
    (project / ".gitignore").write_text(".opencode/node_modules/\n", encoding="utf-8")
    config = {
        "provider": {"fixture": {"npm": "@ai-sdk/openai-compatible", "name": "Review180 deterministic fixture", "options": {"baseURL": f"http://127.0.0.1:{port}/v1", "apiKey": "local-fixture"}, "models": {"review180": {"name": "Review180 Host fixture", "limit": {"context": 32000, "output": 4096}}}}},
        # No global permission/tools restrictions: verify the shipped agent.
    }
    (project / "opencode.json").write_text(json.dumps(config), encoding="utf-8")
    for config_dir in (project / ".opencode", Path(os.environ["REVIEW180_XDG_CONFIG"]) / "opencode"):
        config_dir.mkdir(parents=True, exist_ok=True)
        if sdk_root is not None:
            for package_name in ("package.json", "package-lock.json"):
                shutil.copyfile(sdk_root / package_name, config_dir / package_name)
            if not (config_dir / "node_modules").exists():
                shutil.copytree(sdk_root / "node_modules", config_dir / "node_modules")
    return project


def tool_actions(exported):
    actions = []
    for message in exported.get("messages", []):
        for part in message.get("parts", []):
            if part.get("type") != "tool" or normalized(part.get("tool", "")) != normalized(TOOL_NAME):
                continue
            state = part.get("state", {})
            if state.get("status") == "completed":
                actions.append(state.get("input", {}).get("action"))
    return actions


def tool_parts(exported, action):
    out = []
    for message in exported.get("messages", []):
        for part in message.get("parts", []):
            if part.get("type") != "tool" or normalized(part.get("tool", "")) != normalized(TOOL_NAME):
                continue
            state = part.get("state", {})
            if state.get("input", {}).get("action") == action:
                out.append(state)
    return out


def host_binding_violations(exported, run_id):
    """Compare native tool binding to the actual exported Host parent message."""
    violations = []
    session = exported.get("info", {}).get("id")
    messages = {m.get("info", {}).get("id"): m for m in exported.get("messages", [])}
    for message in exported.get("messages", []):
        info = message.get("info", {})
        for part in message.get("parts", []):
            state = part.get("state", {})
            if part.get("type") != "tool" or normalized(str(part.get("tool", ""))) != normalized(TOOL_NAME) or state.get("status") != "completed":
                continue
            try:
                output = state.get("output", {})
                output = json.loads(output) if isinstance(output, str) else output
                binding = output.get("hostBinding")
                expected = {"runId": run_id, "sessionID": session,
                            "userMessageID": info.get("parentID"), "toolMessageID": info.get("id")}
                require(session and info.get("id") and info.get("parentID") and binding == expected,
                        "native Host binding differs from session/parent/run")
                parent = messages.get(info["parentID"], {})
                require(parent.get("info", {}).get("role") == "user" and
                        parent.get("info", {}).get("sessionID") == session,
                        "native Host binding has no matching user entry")
                receipts = []
                for item in parent.get("parts", []):
                    if item.get("type") != "text":
                        continue
                    text = item.get("text", "").lstrip()
                    if not text.startswith("{"):
                        continue
                    receipt, end = json.JSONDecoder().raw_decode(text)
                    if text[end:].lstrip().startswith("# Codea Harness 1.8 Review"):
                        receipts.append(receipt)
                require(len(receipts) == 1 and receipts[0].get("runId") == run_id,
                        "native Host binding differs from command entry run")
            except (ValueError, TypeError, AttributeError, KeyError, RuntimeError) as error:
                violations.append("Host run binding invalid: " + str(error))
    return violations


def session_for_run(stdout, binary, project, env, scenario):
    sessions = list((Path(env["XDG_DATA_HOME"]) / "opencode" / "storage" / "session").rglob("*.json"))
    session_match = re.search(r'"sessionID"\s*:\s*"([^"]+)"', stdout)
    if session_match:
        return session_match.group(1)
    listed = command([str(binary), "session", "list", "--format", "json"], project, env, timeout=30)
    rows = json.loads(listed)
    require(rows, f"{scenario}: no native OpenCode session found; storage={sessions}")
    return rows[0]["id"]


def run_scenario(binary, project, env, server, multi):
    scenario = "multi" if multi else "single"
    server.scenario = scenario
    run = [str(binary), "run", "--print-logs", "--dir", str(project), "--model", "fixture/review180", "--format", "json"]
    first = command(run + (["--command", "harness-review"] if multi else ["--command", "harness-review", "OrderController"]), project, env)
    require(not server.errors, f"{scenario}: provider errors: {server.errors}")
    runs = list((project / ".code-harness" / "runs").glob("review-*"))
    require(len(runs) == 1, f"{scenario}: expected exactly one run, got {runs}")
    run_dir = runs[0]
    run_id = run_dir.name
    report = run_dir / "review.md"
    require(report.is_file(), f"{scenario}: report was not created by start before Agent work")

    session = session_for_run(first, binary, project, env, scenario)
    exported = json.loads(command([str(binary), "export", session], project, env, timeout=30))
    actions = tool_actions(exported)
    require("prepare" in actions, f"{scenario}: model never called prepare; actions={actions}")
    require(not host_binding_violations(exported, run_id),
            f"{scenario}: invalid Host run binding: {host_binding_violations(exported, run_id)}")

    # Both one and multiple Controllers must auto-finish in the first Host turn.
    require(actions.count("finish")==1 and actions.count("read") >= 1 and
            "select" not in actions and "resume" not in actions,
            f"{scenario}: expected autonomous prepare/finish: {actions}")
    scope=json.loads((run_dir / "scope.json").read_text(encoding="utf-8"))
    options=json.loads((run_dir / "options.json").read_text(encoding="utf-8"))["options"]
    expected=2 if multi else 1
    require(options.get("selectionRequired") is False and len(options["chains"])==expected,
            f"{scenario}: unexpected Controller options: {options}")
    require(scope.get("selectedIds")==[f"C{i}" for i in range(1,expected+1)],
            f"{scenario}: auto selection did not cover all Controllers: {scope}")
    require(len(scope.get("chains",[]))==(4 if multi else 1),
            f"{scenario}: method chains not preserved: {scope}")
    final=report.read_text(encoding="utf-8")
    require('"execution":"COMPLETE"' in final,
            f"{scenario}: incomplete report: {final}")
    print(f"REVIEW180_HOST_{'MULTI_AUTO' if multi else 'SINGLE'} PASS runId={run_id} actions={actions}",flush=True)


def run_concurrent_finish_scenario(binary, project, env, server):
    """Send sibling finish tool calls in one native Host turn.

    OpenCode dispatches sibling tool calls independently; both cross the real
    TypeScript codea-review tool and contend on the same Go Runtime run lock.
    """
    server.scenario = "concurrent"
    server.errors.clear()
    run = [str(binary), "run", "--print-logs", "--dir", str(project), "--model", "fixture/review180", "--format", "json"]
    stdout = command(run + ["--command", "harness-review", "OrderController"], project, env)
    require(not server.errors, f"concurrent: provider errors: {server.errors}")
    runs = list((project / ".code-harness" / "runs").glob("review-*"))
    require(len(runs) == 1, f"concurrent: expected one run, got {runs}")
    run_dir = runs[0]
    session = session_for_run(stdout, binary, project, env, "concurrent")
    exported = json.loads(command([str(binary), "export", session], project, env, timeout=30))
    finishes = tool_parts(exported, "finish")
    require(len(finishes) == 2, f"concurrent: expected exactly two finish attempts, got {finishes}")
    completed = [state for state in finishes if state.get("status") == "completed"]
    failed = [state for state in finishes if state.get("status") in {"error", "failed"}]
    require(len(completed) == 1 and len(failed) == 1, f"concurrent: different results must not both succeed: {finishes}")
    require("REVIEW_FINISH_OVERWRITE_REJECTED" in flatten(failed[0]), f"concurrent: loser did not fail at Runtime overwrite gate: {failed[0]}")

    request_paths = sorted((run_dir / "requests").glob("finish-*.json"))
    require(len(request_paths) == 2, f"concurrent: expected two invocation-unique request files, got {request_paths}")
    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
        payloads = list(pool.map(lambda p: p.read_text(encoding="utf-8"), request_paths))
    require(payloads[0] != payloads[1], "concurrent: A/B finish payloads were cross-written to identical request bytes")
    result = json.loads((run_dir / "result.json").read_text(encoding="utf-8"))
    require(result.get("runId") == run_dir.name, f"concurrent: durable result belongs to another run: {result}")
    final = (run_dir / "review.md").read_text(encoding="utf-8")
    require('"execution":"COMPLETE"' in final, f"concurrent: winning result did not create COMPLETE report:\n{final}")
    print(f"REVIEW180_HOST_CONCURRENT_FINISH PASS runId={run_dir.name} completed=1 rejected=1 uniqueRequests=2", flush=True)


def run_binding_rejection_scenario(binary, project, env, server, scenario):
    server.scenario = scenario
    server.errors.clear()
    run = [str(binary), "run", "--print-logs", "--dir", str(project),
           "--model", "fixture/review180", "--format", "json"]
    if scenario == "wrong-run-id":
        run += ["--command", "harness-review", "OrderController"]
        expected_error = "CODEA_REVIEW_RUN_ID_MISMATCH"
    else:
        run += ["--agent", "codea-review-restricted", "Prepare a review without a command entry."]
        expected_error = "CODEA_REVIEW_HOST_ENTRY_REQUIRED"
    stdout = command(run, project, env, timeout=150)
    require(not server.errors, f"{scenario}: provider errors: {server.errors}")
    session = session_for_run(stdout, binary, project, env, scenario)
    exported = json.loads(command([str(binary), "export", session], project, env, timeout=30))
    attempts = tool_parts(exported, "prepare")
    require(len(attempts) == 1 and attempts[0].get("status") == "error" and
            expected_error in str(attempts[0].get("error")),
            f"{scenario}: native Host did not reject run identity: {attempts}")
    # tool_actions intentionally returns only COMPLETED calls. A rejected
    # prepare is absent from it; audit every attempt here, including failures
    # and forbidden tools, so rejection passes but recovery cannot be hidden.
    all_attempts = [part for message in exported.get("messages", [])
                    for part in message.get("parts", []) if part.get("type") == "tool"]
    require(len(all_attempts) == 1, f"{scenario}: unexpected recovery attempt: {all_attempts}")
    runs = list((project / ".code-harness" / "runs").glob("review-*"))
    require(len(runs) == (1 if scenario == "wrong-run-id" else 0), f"{scenario}: unexpected run creation")
    for run_dir in runs:
        require(not (run_dir / "scope.json").exists() and not (run_dir / "result.json").exists(),
                f"{scenario}: rejected call mutated review scope/result")
        report = (run_dir / "review.md").read_text(encoding="utf-8")
        require('"execution":"INCOMPLETE"' in report, f"{scenario}: report not INCOMPLETE")
    print(f"REVIEW180_HOST_BINDING_DENIED PASS scenario={scenario} error={expected_error}", flush=True)



def run_forbidden_source_scenario(binary, project, env, server):
    """Real packaged native Host must fail closed for an out-of-scope read."""
    server.scenario = "forbidden-source"
    server.errors.clear()
    argv = [str(binary), "run", "--print-logs", "--dir", str(project),
            "--model", "fixture/review180", "--format", "json",
            "--command", "harness-review", "OrderController"]
    stdout = command(argv, project, env, timeout=150)
    require(not server.errors, f"forbidden-source: provider errors: {server.errors}")
    session = session_for_run(stdout, binary, project, env, "forbidden-source")
    exported = json.loads(command([str(binary), "export", session], project, env, timeout=30))
    attempts = tool_parts(exported, "read")
    require(len(attempts) == 1, f"expected one native out-of-scope read attempt: {attempts}")
    require(attempts[0].get("status") in {"error", "failed"} and
            "CODEA_REVIEW_SOURCE_OUTSIDE_SCOPE" in flatten(attempts[0]),
            f"native read was not rejected by exact scope authority: {attempts}")
    require("finish" not in tool_actions(exported),
            f"forbidden-source must not complete review: {tool_actions(exported)}")
    runs = list((project / ".code-harness" / "runs").glob("review-*"))
    require(len(runs) == 1, "forbidden-source: expected exactly one durable run")
    report = (runs[0] / "review.md").read_text(encoding="utf-8")
    require('"execution":"INCOMPLETE"' in report,
            f"forbidden-source unexpectedly completed the report: {report}")
    print(f"REVIEW180_HOST_SOURCE_SCOPE_DENIED PASS runId={runs[0].name} attempts=1", flush=True)


def run_forbidden_builtin_scenario(binary, project, env, server, builtin):
    """Actively inject forbidden read/glob/bash tool calls into the REAL Host.
    Verify none executes, even under ordinary (unrestricted global) config."""
    scenario = "forbidden-builtin-" + builtin
    server.scenario = scenario
    server.project = project
    server.errors.clear()
    argv = [str(binary), "run", "--print-logs", "--dir", str(project),
            "--model", "fixture/review180", "--format", "json",
            "--command", "harness-review", "OrderController"]
    try:
        stdout = command(argv, project, env, timeout=150)
    except RuntimeError as exc:
        # Rejecting an unadvertised tool can make the Host command exit with
        # an error. That is acceptable; execution of the tool is not.
        stdout = str(exc)
    require(not server.errors, f"{scenario}: provider errors: {server.errors}")
    require(scenario in server.forbidden_issued,
            f"{scenario}: attempted forbidden tool call never reached Host")
    # Verify the prohibited tool was never successfully dispatched.
    session = session_for_run(stdout, binary, project, env, scenario)
    exported = json.loads(command([str(binary), "export", session], project, env, timeout=30))
    states = [
        part.get("state", {}) for m in exported.get("messages", [])
        for part in m.get("parts", [])
        if part.get("type") == "tool" and normalized(part.get("tool", "")) == normalized(builtin)
    ]
    require(not any(st.get("status") == "completed" for st in states),
            f"{scenario}: forbidden builtin executed: {states}")
    require(not (project / "isolation-breached.txt").exists(),
            f"{scenario}: forbidden bash command wrote a file")
    runs = list((project / ".code-harness/runs").glob("review-*"))
    require(len(runs) == 1 and
            '"execution":"INCOMPLETE"' in (runs[0] / "review.md").read_text(encoding="utf-8"),
            f"{scenario}: a forbidden tool attempt completed the report")
    require("finish" not in tool_actions(exported),
            f"{scenario}: forbidden builtin could bypass the native review finish")
    print(f"REVIEW180_HOST_BUILTIN_DENIED PASS tool={builtin} "
          f"attempts={len(states)} globalToolOverrides=none", flush=True)


def run_orchestrator_unrestricted_scenario(binary, project, env, server):
    server.scenario = "orchestrator-permission-check"
    server.errors.clear()
    stdout = command([str(binary), "run", "--agent", "orchestrator",
                      "--dir", str(project), "--model", "fixture/review180",
                      "--format", "json", "Verify Host tool availability."],
                     project, env, timeout=100)
    require(not server.errors, f"orchestrator tool permissions unexpectedly restricted: {server.errors}")
    tool_requests = [req for req in server.requests
                     if req["scenario"] == "orchestrator-permission-check" and req["names"]]
    require(tool_requests, "orchestrator never made a real tool-bearing model request")
    for req in tool_requests:
        require(all(builtin in req["names"] for builtin in ("read", "glob", "bash")),
                f"orchestrator tool advertisement lacks normal builtins: {req['names']}")
    print(f"REVIEW180_OTHER_WORKFLOWS_UNRESTRICTED PASS tools=read,glob,bash "
          f"verifiedRequests={len(tool_requests)}", flush=True)


def init_fixture_repo(project, env):
    command(["git", "init"], project, env, timeout=20)
    command(["git", "add", "."], project, env, timeout=20)
    command(["git", "-c", "user.name=Host Fixture", "-c", "user.email=host-fixture@example.test", "commit", "-m", "baseline"], project, env, timeout=20)
    impl = project / "src" / "main" / "java" / "com" / "example" / "OrderServiceImpl.java"
    # A trailing file-level comment is NOT a method change. With exact
    # Java-method Git attribution it legitimately yields KEEP_REPORT_INCOMPLETE.
    # Change actual method bodies instead, preserving the source's original
    # CRLF/LF bytes; for the multi-chain scenario, change both methods.
    raw = impl.read_bytes()
    targets = (b"mapper.insertOrder();", b"mapper.cancelOrder();")
    changed = 0
    for call in targets:
        if call in raw:
            require(raw.count(call) == 1, f"ambiguous Host smoke fixture call {call!r}")
            raw = raw.replace(call, call + b" /* changed for review180 */", 1)
            changed += 1
    require(changed >= 1, "Host smoke fixture did not modify an actual Java method")
    impl.write_bytes(raw)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--opencode", required=True, type=Path)
    parser.add_argument("--runtime", required=True, type=Path)
    parser.add_argument("--ast-grep", required=True, type=Path)
    parser.add_argument("--tool-source", required=True, type=Path)
    parser.add_argument("--command-source", required=True, type=Path)
    parser.add_argument("--install-zip", required=True, type=Path)
    parser.add_argument("--sdk-root", type=Path)
    parser.add_argument("--offline", action="store_true",
                        help="No SDK preseed, npm install, or public network access")
    args = parser.parse_args()
    for value in (args.opencode, args.runtime, args.ast_grep, args.tool_source, args.command_source, args.install_zip):
        require(value.resolve().is_file(), f"required file missing: {value}")
    require(args.offline or args.sdk_root is not None, "choose --offline or --sdk-root")
    require(not (args.offline and args.sdk_root is not None), "offline test must not preseed the SDK")
    sdk_root = None if args.offline else args.sdk_root.resolve()
    if sdk_root is not None:
        package = json.loads((sdk_root / "node_modules" / "@opencode-ai" / "plugin" / "package.json").read_text(encoding="utf-8"))
        require(package.get("version") == "1.18.25", f"expected @opencode-ai/plugin 1.18.25, got {package.get('version')}")

    server = ThreadingHTTPServer(("127.0.0.1", 0), Provider)
    server.requests, server.errors, server.scenario = [], [], "bootstrap"
    server.scope_by_run, server.read_index = {}, {}
    server.forbidden_issued = set()
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        with tempfile.TemporaryDirectory(prefix="Codea 180 Host & ") as raw:
            temp = Path(raw)
            env = dict(os.environ)
            env["REVIEW180_XDG_CONFIG"] = str(temp / "xdg-config")
            for key, suffix in (("XDG_CONFIG_HOME", "xdg-config"), ("XDG_DATA_HOME", "xdg-data"), ("XDG_CACHE_HOME", "xdg-cache"), ("XDG_STATE_HOME", "xdg-state")):
                env[key] = str(temp / suffix)
            if args.offline:
                # Fail any accidental registry/network fetch. The native model
                # fixture remains reachable on loopback via NO_PROXY below.
                env["NPM_CONFIG_OFFLINE"] = "true"
                env["npm_config_offline"] = "true"
                for proxy in ("HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"):
                    env[proxy] = "http://127.0.0.1:1"
            env["OPENCODE_DISABLE_MODELS_FETCH"] = "true"
            env["OPENCODE_DISABLE_AUTOUPDATE"] = "true"
            env["OPENCODE_CONFIG_CONTENT"] = "{}"
            env.pop("OPENCODE_CONFIG", None)
            env["PATH"] = str(args.opencode.resolve().parent) + os.pathsep + env.get("PATH", "")
            for key in ("NO_PROXY", "no_proxy"):
                env[key] = ",".join(filter(None, [env.get(key, ""), "127.0.0.1", "localhost", "::1"]))
            os.environ["REVIEW180_XDG_CONFIG"] = env["REVIEW180_XDG_CONFIG"]
            with build_opener(ProxyHandler({})).open(f"http://127.0.0.1:{server.server_port}/health", timeout=5) as health:
                require(json.load(health) == {"ready": True}, "fixture provider not ready")
            args.package_stage = temp / "official-install"
            with zipfile.ZipFile(args.install_zip.resolve()) as archive:
                archive.extractall(args.package_stage)
            for rel, src in (
                (".opencode/commands/harness-review.md", args.command_source),
                (".opencode/tools/codea-review.ts", args.tool_source),
            ):
                require((args.package_stage / rel).read_bytes() == src.read_bytes(),
                        f"test source mismatches actual install package: {rel}")
            version = command([str(args.opencode.resolve()), "--version"], temp, env, timeout=30).strip()
            require(version == "1.18.25", f"expected OpenCode 1.18.25, got {version}")
            for multi in (False, True):
                project = bootstrap_project(temp, args, sdk_root, server.server_port, multi)
                init_fixture_repo(project, env)
                run_scenario(args.opencode.resolve(), project, env, server, multi)
            for scenario in ("wrong-run-id", "missing-entry"):
                project = bootstrap_project(temp, args, sdk_root, server.server_port, False, name=scenario)
                init_fixture_repo(project, env)
                run_binding_rejection_scenario(args.opencode.resolve(), project, env, server, scenario)
            forbidden_project = bootstrap_project(temp, args, sdk_root, server.server_port, False, name="forbidden-source")
            init_fixture_repo(forbidden_project, env)
            run_forbidden_source_scenario(args.opencode.resolve(), forbidden_project, env, server)
            concurrent_project = bootstrap_project(temp, args, sdk_root, server.server_port, False, name="concurrent")
            init_fixture_repo(concurrent_project, env)
            run_concurrent_finish_scenario(args.opencode.resolve(), concurrent_project, env, server)
            for builtin in ("glob", "read", "bash"):
                denied = bootstrap_project(temp, args, sdk_root, server.server_port,
                                           False, name="denied-" + builtin)
                init_fixture_repo(denied, env)
                run_forbidden_builtin_scenario(args.opencode.resolve(), denied, env, server, builtin)
            general = bootstrap_project(temp, args, sdk_root, server.server_port,
                                        False, name="general-orchestrator")
            init_fixture_repo(general, env)
            run_orchestrator_unrestricted_scenario(args.opencode.resolve(), general, env, server)
            print(f"REVIEW180_NATIVE_HOST_SMOKE PASS opencode=1.18.25 deterministic=true "
                  f"offlineSdkless={args.offline} shippedAgentIsolation=true globalToolOverrides=none",
                  flush=True)
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)


if __name__ == "__main__":
    main()
