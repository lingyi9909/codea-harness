#!/usr/bin/env python3
"""Codea Harness 1.8 native OpenCode Host transport smoke.

The loopback provider is deterministic and validates Host/tool plumbing only; it is
not evidence of semantic model quality. The driver sends only normal user input
(`/harness-review ...` and, for multi-chain, the next real user selection). It
never invokes review prepare/select/finish directly.
"""
import argparse
import json
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import sys
import tempfile
import threading
import time
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
        if isinstance(obj, dict) and "runtime" in obj:
            return obj
    return None


def latest_user_text(messages):
    for message in reversed(messages):
        if message.get("role") == "user":
            return flatten(message.get("content", ""))
    return ""


def call(tool_name, arguments, suffix):
    return {
        "id": f"call_review180_{suffix}_{time.time_ns()}",
        "type": "function",
        "function": {"name": tool_name, "arguments": json.dumps(arguments, ensure_ascii=False)},
    }


class Provider(BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def reply(self, body, *, content=None, tool_call=None):
        base = {"id": "chatcmpl-review180", "created": int(time.time()), "model": body.get("model", "review180")}
        finish = "stop"
        message = {"role": "assistant", "content": content}
        if tool_call is not None:
            message = {"role": "assistant", "content": None, "tool_calls": [tool_call]}
            finish = "tool_calls"
        self.send_response(200)
        if body.get("stream"):
            self.send_header("Content-Type", "text/event-stream")
            self.end_headers()
            delta = {"role": "assistant"}
            if tool_call is None:
                delta["content"] = content or ""
            else:
                delta["tool_calls"] = [{"index": 0, **tool_call}]
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
            if not names:
                self.reply(body, content="Codea Harness 1.8 Host smoke")
                return
            tool_name = next((name for name in names if normalized(name) == normalized(TOOL_NAME)), None)
            require(tool_name is not None, f"{TOOL_NAME} not advertised: {names}")

            whole = flatten(messages)
            match = RUN_ID_FIELD_RE.search(whole)
            require(match is not None, "review start runId was not injected before first model request")
            run_id = match.group(1)
            payload = latest_tool_payload(messages)
            user = latest_user_text(messages)

            if payload is None:
                self.reply(body, tool_call=call(tool_name, {
                    "action": "prepare",
                    "runId": run_id,
                    "intent": {"mode": "CHANGES", "target": "OrderController"},
                }, "prepare"))
                return

            runtime = payload.get("runtime", {}) if isinstance(payload, dict) else {}
            scope = payload.get("scope") if isinstance(payload, dict) else None
            if isinstance(runtime, dict) and "optionsHash" in runtime:
                chains = runtime.get("chains", [])
                if runtime.get("selectionRequired"):
                    if "选择 C1" not in user:
                        lines = [f"{run_id} options={runtime['optionsHash']}"]
                        lines.extend(f"{chain['id']} {chain['name']}" for chain in chains)
                        self.reply(body, content="\n".join(lines))
                        return
                    self.reply(body, tool_call=call(tool_name, {
                        "action": "select",
                        "runId": run_id,
                        "selection": {"optionsHash": runtime["optionsHash"], "ids": ["C1"]},
                    }, "select"))
                    return

            if isinstance(scope, dict) and scope.get("reads") is not None:
                self.reply(body, tool_call=call(tool_name, {
                    "action": "finish",
                    "runId": run_id,
                    "result": {
                        "reads": scope.get("reads", []),
                        "findings": [],
                        "pendingRisks": [],
                        "gaps": [],
                    },
                }, "finish"))
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
    (java / "OrderController.java").write_text('''package com.example;\nimport org.springframework.web.bind.annotation.PostMapping;\nimport org.springframework.web.bind.annotation.RestController;\n@RestController\npublic class OrderController {\n    private final OrderService service;\n    public OrderController(OrderService service) { this.service = service; }\n''' + endpoints + "}\n", encoding="utf-8")
    (java / "OrderService.java").write_text("package com.example;\npublic interface OrderService {\n" + service_methods + "}\n", encoding="utf-8")
    (java / "OrderServiceImpl.java").write_text("package com.example;\npublic class OrderServiceImpl implements OrderService {\n    private final OrderMapper mapper;\n    public OrderServiceImpl(OrderMapper mapper) { this.mapper = mapper; }\n" + impl_methods + "}\n", encoding="utf-8")
    (java / "OrderMapper.java").write_text("package com.example;\npublic interface OrderMapper {\n" + mapper_methods + "}\n", encoding="utf-8")
    (xml / "OrderMapper.xml").write_text('''<?xml version="1.0" encoding="UTF-8"?>\n<mapper namespace="com.example.OrderMapper">\n''' + sql + "</mapper>\n", encoding="utf-8")
    (project / "pom.xml").write_text("<project><modelVersion>4.0.0</modelVersion><groupId>com.example</groupId><artifactId>review180</artifactId><version>1</version></project>\n", encoding="utf-8")


def bootstrap_project(temp, args, sdk_root, port, multi):
    project = temp / ("multi" if multi else "single")
    agent_source = args.command_source.parent.parent / "agents" / "orchestrator.md"
    require(agent_source.resolve().is_file(), f"orchestrator agent source missing: {agent_source}")
    (project / ".opencode" / "tools").mkdir(parents=True)
    (project / ".opencode" / "commands").mkdir(parents=True)
    (project / ".opencode" / "agents").mkdir(parents=True)
    (project / ".code-harness" / "bin").mkdir(parents=True)
    shutil.copyfile(args.tool_source, project / ".opencode" / "tools" / "codea-review.ts")
    shutil.copyfile(args.command_source, project / ".opencode" / "commands" / "harness-review.md")
    shutil.copyfile(agent_source, project / ".opencode" / "agents" / "orchestrator.md")
    shutil.copyfile(args.runtime, project / ".code-harness" / "bin" / ("codea-dcep-tools.exe" if os.name == "nt" else "codea-dcep-tools"))
    shutil.copyfile(args.ast_grep, project / ".code-harness" / "bin" / ("ast-grep.exe" if os.name == "nt" else "ast-grep"))
    write_fixture(project, multi)
    (project / ".gitignore").write_text(".opencode/node_modules/\n", encoding="utf-8")
    config = {
        "provider": {"fixture": {"npm": "@ai-sdk/openai-compatible", "name": "Review180 deterministic fixture", "options": {"baseURL": f"http://127.0.0.1:{port}/v1", "apiKey": "local-fixture"}, "models": {"review180": {"name": "Review180 Host fixture", "limit": {"context": 32000, "output": 4096}}}}},
        "permission": {"*": "deny", TOOL_NAME: "allow", "bash": "allow"},
    }
    (project / "opencode.json").write_text(json.dumps(config), encoding="utf-8")
    for config_dir in (project / ".opencode", Path(os.environ["REVIEW180_XDG_CONFIG"]) / "opencode"):
        config_dir.mkdir(parents=True, exist_ok=True)
        for name in ("package.json", "package-lock.json"):
            shutil.copyfile(sdk_root / name, config_dir / name)
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


def run_scenario(binary, project, env, server, multi):
    scenario = "multi" if multi else "single"
    server.scenario = scenario
    run = [str(binary), "run", "--print-logs", "--dir", str(project), "--model", "fixture/review180", "--format", "json"]
    first = command(run + ["--command", "harness-review", "OrderController"], project, env)
    require(not server.errors, f"{scenario}: provider errors: {server.errors}")
    runs = list((project / ".code-harness" / "runs").glob("review-*"))
    require(len(runs) == 1, f"{scenario}: expected exactly one run, got {runs}")
    run_dir = runs[0]
    run_id = run_dir.name
    report = run_dir / "review.md"
    require(report.is_file(), f"{scenario}: report was not created by start before Agent work")

    sessions = list((Path(env["XDG_DATA_HOME"]) / "opencode" / "storage" / "session").rglob("*.json"))
    session_match = re.search(r'"sessionID"\s*:\s*"([^"]+)"', first)
    if session_match:
        session = session_match.group(1)
    else:
        listed = command([str(binary), "session", "list", "--format", "json"], project, env, timeout=30)
        rows = json.loads(listed)
        require(rows, f"{scenario}: no native OpenCode session found; storage={sessions}")
        session = rows[0]["id"]
    exported = json.loads(command([str(binary), "export", session], project, env, timeout=30))
    actions = tool_actions(exported)
    require("prepare" in actions, f"{scenario}: model never called prepare; actions={actions}")

    if not multi:
        require("finish" in actions, f"single: model never autonomously called finish; actions={actions}\n{first[-3000:]}")
        require("select" not in actions, f"single: unexpected selection; actions={actions}")
        final = report.read_text(encoding="utf-8")
        require('"execution":"COMPLETE"' in final, f"single: durable report not COMPLETE:\n{final}")
        print(f"REVIEW180_HOST_SINGLE PASS runId={run_id} actions={actions}", flush=True)
        return

    require("finish" not in actions and "select" not in actions, f"multi: model crossed human-selection boundary before user reply; actions={actions}")
    require(" options=" in flatten(exported), "multi: current run/hash menu was not displayed")
    command(run + ["--session", session, "选择", "C1"], project, env)
    require(not server.errors, f"multi selection: provider errors: {server.errors}")
    exported = json.loads(command([str(binary), "export", session], project, env, timeout=30))
    actions = tool_actions(exported)
    require(actions.count("select") == 1 and actions.count("finish") == 1, f"multi: expected one select and finish after real reply; actions={actions}")
    scope = json.loads((run_dir / "scope.json").read_text(encoding="utf-8"))
    require(scope.get("selectedIds") == ["C1"], f"multi: selected scope mismatch: {scope}")
    final = report.read_text(encoding="utf-8")
    require('"execution":"COMPLETE"' in final, f"multi: report not COMPLETE after selection:\n{final}")
    print(f"REVIEW180_HOST_MULTI PASS runId={run_id} actions={actions} actualUserReply=true", flush=True)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--opencode", required=True, type=Path)
    parser.add_argument("--runtime", required=True, type=Path)
    parser.add_argument("--ast-grep", required=True, type=Path)
    parser.add_argument("--tool-source", required=True, type=Path)
    parser.add_argument("--command-source", required=True, type=Path)
    parser.add_argument("--sdk-root", required=True, type=Path)
    args = parser.parse_args()
    for value in (args.opencode, args.runtime, args.ast_grep, args.tool_source, args.command_source):
        require(value.resolve().is_file(), f"required file missing: {value}")
    sdk_root = args.sdk_root.resolve()
    package = json.loads((sdk_root / "node_modules" / "@opencode-ai" / "plugin" / "package.json").read_text(encoding="utf-8"))
    require(package.get("version") == "1.18.25", f"expected @opencode-ai/plugin 1.18.25, got {package.get('version')}")

    server = ThreadingHTTPServer(("127.0.0.1", 0), Provider)
    server.requests, server.errors, server.scenario = [], [], "bootstrap"
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        with tempfile.TemporaryDirectory(prefix="Codea 180 Host & ") as raw:
            temp = Path(raw)
            env = dict(os.environ)
            env["REVIEW180_XDG_CONFIG"] = str(temp / "xdg-config")
            for key, suffix in (("XDG_CONFIG_HOME", "xdg-config"), ("XDG_DATA_HOME", "xdg-data"), ("XDG_CACHE_HOME", "xdg-cache"), ("XDG_STATE_HOME", "xdg-state")):
                env[key] = str(temp / suffix)
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
            version = command([str(args.opencode.resolve()), "--version"], temp, env, timeout=30).strip()
            require(version == "1.18.25", f"expected OpenCode 1.18.25, got {version}")
            for multi in (False, True):
                project = bootstrap_project(temp, args, sdk_root, server.server_port, multi)
                command(["git", "init"], project, env, timeout=20)
                command(["git", "add", "."], project, env, timeout=20)
                command(["git", "-c", "user.name=Host Fixture", "-c", "user.email=host-fixture@example.test", "commit", "-m", "baseline"], project, env, timeout=20)
                impl = project / "src" / "main" / "java" / "com" / "example" / "OrderServiceImpl.java"
                impl.write_text(impl.read_text(encoding="utf-8") + "// changed for review180\n", encoding="utf-8")
                run_scenario(args.opencode.resolve(), project, env, server, multi)
            print("REVIEW180_NATIVE_HOST_SMOKE PASS opencode=1.18.25 deterministic=true", flush=True)
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)


if __name__ == "__main__":
    main()
