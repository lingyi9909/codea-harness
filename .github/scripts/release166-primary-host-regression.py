#!/usr/bin/env python3
"""Real OpenCode Host transport gate; fixture responses do not assess model reasoning."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import tempfile
import threading
import time
from urllib.request import ProxyHandler, build_opener
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

RUN_ID = "release166-primary-host"
OPTIONS_HASH = "a" * 64
TOOL_NAME = "codea-reviewer-submit"
ANALYSIS = {
    "changedFileRoles": [], "affectedControllers": [], "callChains": [],
    "symbolLocations": [], "resourceRelations": [], "externalDependencies": [],
    "riskAreas": [],
    "reviewCoverage": {"status": "COMPLETE", "reviewedFiles": [], "unresolvedSymbols": []},
}
SELECTION = {"runId": RUN_ID, "mode": "TARGETED", "optionsHash": OPTIONS_HASH, "selectionIds": ["C1"]}
MENU = f"Run: {RUN_ID}\nOptions: {OPTIONS_HASH}\nReview: FULL \nC1: First.entry -> First.service\nC2: Second.entry -> Second.service"


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


class Provider(BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def reply(self, body, *, content=None, call=None):
        base = {"id": "chatcmpl-primary166", "created": int(time.time()), "model": body.get("model", "primary166")}
        message = {"role": "assistant", "content": content}
        finish = "stop"
        if call is not None:
            message = {"role": "assistant", "tool_calls": [call]}
            finish = "tool_calls"
        self.send_response(200)
        if body.get("stream"):
            self.send_header("Content-Type", "text/event-stream")
            self.end_headers()
            delta = dict(message)
            if call is not None:
                delta["tool_calls"] = [{"index": 0, **call}]
            for part, reason in [(delta, None), ({}, finish)]:
                chunk = {**base, "object": "chat.completion.chunk", "choices": [{"index": 0, "delta": part, "finish_reason": reason}]}
                self.wfile.write(("data: " + json.dumps(chunk) + "\n\n").encode())
            self.wfile.write(b"data: [DONE]\n\n")
        else:
            raw = json.dumps({**base, "object": "chat.completion", "choices": [{"index": 0, "message": message, "finish_reason": finish}]}).encode()
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
            names = [t.get("function", {}).get("name", "") for t in body.get("tools", [])]
            messages = body.get("messages", [])
            self.server.requests.append({"phase": self.server.phase, "names": names, "messages": messages})
            print(f"PRIMARY166_PROVIDER request={len(self.server.requests)} phase={self.server.phase} tools={len(names)} messages={len(messages)}", flush=True)
            if not names:  # Native title/summary requests have no tool registry.
                self.reply(body, content="Primary Host regression")
                return
            submit = next((name for name in names if normalized(name) == normalized(TOOL_NAME)), None)
            require(submit is not None, f"primary submission tool not loaded: {names}")
            last_user = max((i for i, m in enumerate(messages) if m.get("role") == "user"), default=-1)
            require(last_user >= 0, "provider request missing user input")
            subsequent = messages[last_user + 1:]
            if any(m.get("role") == "tool" for m in subsequent):
                self.reply(body, content=MENU if self.server.phase == "analysis" else "Selection submitted.")
                return
            kind, proposal = "change-analysis", ANALYSIS
            if self.server.phase == "selection":
                require("选择 C1" in flatten(messages[last_user].get("content")), "selection call has no real user choice")
                kind, proposal = "selection", SELECTION
            call = {"id": "call_primary166_" + self.server.phase, "type": "function", "function": {
                "name": submit, "arguments": json.dumps({"kind": kind, "runId": RUN_ID, "proposal": json.dumps(proposal)})}}
            self.reply(body, call=call)
        except Exception as error:
            self.server.errors.append(str(error))
            raw = json.dumps({"error": str(error)}).encode()
            self.send_response(500)
            self.send_header("Content-Length", str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)


def command(argv, cwd, env, timeout=90):
    options = {"cwd": cwd, "env": env, "stdout": subprocess.PIPE, "stderr": subprocess.PIPE, "text": True, "encoding": "utf-8", "errors": "replace"}
    if os.name == "nt":
        options["creationflags"] = subprocess.CREATE_NEW_PROCESS_GROUP
    else:
        options["start_new_session"] = True
    process = subprocess.Popen(argv, **options)
    try:
        stdout, stderr = process.communicate(timeout=timeout)
    except subprocess.TimeoutExpired:
        if os.name == "nt":
            subprocess.run(["taskkill", "/PID", str(process.pid), "/T", "/F"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=15, check=False)
        else:
            os.killpg(process.pid, signal.SIGKILL)
        stdout, stderr = process.communicate(timeout=15)
        print(f"PRIMARY166_CHILD_TIMEOUT argv={argv}\nSTDOUT:\n{stdout}\nSTDERR:\n{stderr}", file=sys.stderr, flush=True)
        raise RuntimeError(f"command timed out after {timeout}s: {argv}")
    if process.returncode != 0:
        print(f"PRIMARY166_CHILD_FAILED argv={argv}\nSTDOUT:\n{stdout}\nSTDERR:\n{stderr}", file=sys.stderr, flush=True)
        raise RuntimeError(f"command failed ({process.returncode}): {argv}")
    return stdout


def read_json(path):
    return json.loads(path.read_text(encoding="utf-8"))


def verify_submission(project, exported, kind, expected):
    files = {
        "change-analysis": ("change-analysis-proposal.json", "change-analysis-reviewer-authority.json"),
        "selection": ("review-selection.json", "review-selection-authority.json"),
    }
    proposal_file, receipt_file = files[kind]
    root = project / ".code-harness" / "runs" / RUN_ID / "requests"
    receipt = read_json(root / receipt_file)
    proposal_bytes = (root / proposal_file).read_bytes()
    require(json.loads(proposal_bytes) == expected, f"{kind}: proposal mismatch")
    require(receipt["version"] == 2 and receipt["host"] == "opencode" and receipt["source"] == "opencode-tool-context", f"{kind}: wrong receipt protocol")
    require(receipt["runId"] == RUN_ID and receipt["proposalKind"] == kind, f"{kind}: wrong run or kind")
    require(receipt["proposalPath"] == f".code-harness/runs/{RUN_ID}/requests/{proposal_file}", f"{kind}: wrong proposal path")
    require(receipt["proposalSha256"] == hashlib.sha256(proposal_bytes).hexdigest(), f"{kind}: proposal hash mismatch")
    info = exported["info"]
    require(info["id"] == receipt["sessionId"] and not info.get("parentID"), f"{kind}: session is not primary")
    require(Path(info["directory"]).resolve() == project.resolve(), f"{kind}: session directory does not match project")
    require(receipt["agent"] and receipt["agent"] != "reviewer", f"{kind}: agent is not primary")
    messages = exported["messages"]
    matches = [(i, m) for i, m in enumerate(messages) if m["info"]["id"] == receipt["messageId"]]
    require(len(matches) == 1, f"{kind}: receipt message missing or ambiguous")
    index, message = matches[0]
    require(message["info"]["role"] == "assistant" and message["info"].get("agent") == receipt["agent"], f"{kind}: assistant identity mismatch")
    users = [(i, m) for i, m in enumerate(messages[:index]) if m["info"]["role"] == "user"]
    require(users and message["info"].get("parentID") == users[-1][1]["info"]["id"], f"{kind}: assistant not bound to latest user turn")
    completed = [p for p in message["parts"] if p.get("type") == "tool" and normalized(p.get("tool", "")) == normalized(TOOL_NAME) and p.get("state", {}).get("status") == "completed"]
    require(len(completed) == 1, f"{kind}: completed submission missing")
    state = completed[0]["state"]
    inputs = state["input"]
    require(inputs["runId"] == RUN_ID and inputs["kind"] == kind and json.loads(inputs["proposal"]) == expected, f"{kind}: completed input mismatch")
    require(f"REVIEWER_PROPOSAL_SUBMITTED kind={kind} runId={RUN_ID}" in state.get("output", ""), f"{kind}: successful tool result missing")
    for item in messages:
        require(item["info"].get("agent") != "reviewer", "Reviewer delegation appeared in primary session")
        require(not any(p.get("type") == "subtask" or p.get("type") == "tool" and normalized(p.get("tool", "")) == "task" for p in item["parts"]), "task/delegation appeared in primary session")
    if kind == "selection":
        user_index, user = users[-1]
        require(len(users) == 2, "selection did not follow a second real user turn")
        text_parts = [p for p in user["parts"] if p.get("type") == "text" and not p.get("synthetic") and not p.get("ignored")]
        require(any(p.get("text", "").strip() == "选择 C1" for p in text_parts), f"actual selection user reply missing: {user['parts']!r}")
        prior_user_index = users[-2][0]
        menus = [p.get("text", "") for m in messages[prior_user_index + 1:user_index] if m["info"]["role"] == "assistant" for p in m["parts"] if p.get("type") == "text"]
        require(MENU in menus, "current menu was not displayed before user selection")
    return receipt


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--opencode", required=True, type=Path)
    parser.add_argument("--tool-source", required=True, type=Path)
    parser.add_argument("--sdk-root", required=True, type=Path, help="npm install prefix containing real @opencode-ai/plugin@1.18.25 and its lockfile")
    args = parser.parse_args()
    binary, source, sdk_root = args.opencode.resolve(), args.tool_source.resolve(), args.sdk_root.resolve()
    require(binary.is_file() and source.is_file(), "native OpenCode binary and tool source are required")
    sdk = read_json(sdk_root / "node_modules" / "@opencode-ai" / "plugin" / "package.json")
    require(sdk.get("name") == "@opencode-ai/plugin" and sdk.get("version") == "1.18.25", "real pinned plugin SDK required")
    sdk_lock = read_json(sdk_root / "package-lock.json")
    require(sdk_lock.get("packages", {}).get("", {}).get("dependencies", {}).get("@opencode-ai/plugin") == "1.18.25", "pinned SDK lockfile required")
    server = ThreadingHTTPServer(("127.0.0.1", 0), Provider)
    server.requests, server.errors, server.phase = [], [], "analysis"
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    stop_heartbeat = threading.Event()
    def heartbeat():
        while not stop_heartbeat.wait(15):
            print(f"PRIMARY166_PROVIDER heartbeat phase={server.phase} requests={len(server.requests)} errors={server.errors}", flush=True)
    heartbeat_thread = threading.Thread(target=heartbeat, daemon=True)
    heartbeat_thread.start()
    try:
        with tempfile.TemporaryDirectory(prefix="Codea 166 primary Host & ") as temp:
            temp = Path(temp)
            project = temp / "project"
            (project / ".opencode" / "tools").mkdir(parents=True)
            shutil.copyfile(source, project / ".opencode" / "tools" / (TOOL_NAME + ".ts"))
            config = {
                "provider": {"fixture": {"npm": "@ai-sdk/openai-compatible", "name": "Loopback deterministic fixture", "options": {"baseURL": f"http://127.0.0.1:{server.server_port}/v1", "apiKey": "local-fixture"}, "models": {"primary166": {"name": "Primary Host fixture", "limit": {"context": 32000, "output": 2048}}}}},
                "permission": {"*": "deny", TOOL_NAME: "allow"},
            }
            (project / "opencode.json").write_text(json.dumps(config), encoding="utf-8")
            env = dict(os.environ)
            # Isolate global user configuration and sessions; never overwrite HOME.
            for key in ("XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME"):
                env[key] = str(temp / key.lower())
            # Config waits for embedded npm reify before loading tools. Supply a
            # genuine pinned install up front, including the matching npm lock,
            # so cold registry bootstrap is a separate visible CI step.
            for config_dir in (project / ".opencode", Path(env["XDG_CONFIG_HOME"]) / "opencode"):
                config_dir.mkdir(parents=True, exist_ok=True)
                for name in ("package.json", "package-lock.json"):
                    shutil.copyfile(sdk_root / name, config_dir / name)
                shutil.copytree(sdk_root / "node_modules", config_dir / "node_modules")
            print("PRIMARY166_SDK_BOOTSTRAP PASS package=@opencode-ai/plugin version=1.18.25 configs=2", flush=True)
            for key in ("NO_PROXY", "no_proxy"):
                env[key] = ",".join(filter(None, [env.get(key, ""), "127.0.0.1", "localhost", "::1"]))
            with build_opener(ProxyHandler({})).open(f"http://127.0.0.1:{server.server_port}/health", timeout=5) as health:
                require(json.load(health) == {"ready": True}, "fixture provider not ready")
            print("PRIMARY166_PROVIDER_READY PASS loopback=true", flush=True)
            env["OPENCODE_DISABLE_MODELS_FETCH"] = "true"
            env["OPENCODE_DISABLE_AUTOUPDATE"] = "true"
            env["OPENCODE_CONFIG_CONTENT"] = "{}"
            env.pop("OPENCODE_CONFIG", None)
            command(["git", "init"], project, env, timeout=20)
            command(["git", "-c", "user.name=Host Fixture", "-c", "user.email=host-fixture@example.test", "commit", "--allow-empty", "-m", "Host fixture"], project, env, timeout=20)
            version = command([str(binary), "--version"], project, env, timeout=20).strip()
            require(version == "1.18.25", f"expected pinned OpenCode 1.18.25, got {version}")
            run = [str(binary), "run", "--print-logs", "--dir", str(project), "--model", "fixture/primary166", "--format", "json"]
            output = command(run + ["PRIMARY166_ANALYSIS: submit the same-run analysis, display the two-option menu, then wait for the user."], project, env)
            require(not server.errors, f"fixture provider errors: {server.errors}")
            receipt_path = project / ".code-harness" / "runs" / RUN_ID / "requests" / "change-analysis-reviewer-authority.json"
            require(receipt_path.is_file(), f"primary tool did not produce receipt:\n{output[-4000:]}")
            session = read_json(receipt_path)["sessionId"]
            exported = json.loads(command([str(binary), "export", session], project, env))
            analysis_receipt = verify_submission(project, exported, "change-analysis", ANALYSIS)
            require(not (receipt_path.parent / "review-selection-authority.json").exists(), "assistant selected before actual user reply")
            print(f"RELEASE166_PRIMARY_HOST_ANALYSIS PASS agent={analysis_receipt['agent']} version=2 primary=true", flush=True)
            server.phase = "selection"
            # OpenCode quotes a single positional argument containing whitespace.
            # Separate words preserve the actual user text exactly as 选择 C1.
            command(run + ["--session", session, "选择", "C1"], project, env)
            require(not server.errors, f"fixture provider errors: {server.errors}")
            exported = json.loads(command([str(binary), "export", session], project, env))
            selection_receipt = verify_submission(project, exported, "selection", SELECTION)
            require(selection_receipt["sessionId"] == analysis_receipt["sessionId"] and selection_receipt["messageId"] != analysis_receipt["messageId"], "selection did not resume the primary session")
            for phase in ("analysis", "selection"):
                require(any(r["phase"] == phase and TOOL_NAME in r["names"] for r in server.requests), f"{phase}: actual submission tool name was not advertised")
            require(not (project / ".opencode" / "agents").exists(), "unexpected Reviewer registration")
            print("RELEASE166_PRIMARY_HOST_USER_SELECTION PASS resumed=true actualUserReply=true noDelegation=true", flush=True)
            print("RELEASE166_PRIMARY_HOST_REGRESSION PASS opencode=1.18.25", flush=True)
    finally:
        print(f"PRIMARY166_PROVIDER_FINAL phase={server.phase} requests={len(server.requests)} errors={server.errors}", flush=True)
        stop_heartbeat.set()
        heartbeat_thread.join(timeout=5)
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)


if __name__ == "__main__":
    main()
