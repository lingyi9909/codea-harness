#!/usr/bin/env python3
"""Real packaged Runtime + native OpenCode transport, not model reasoning proof."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import threading
import time
from http.server import ThreadingHTTPServer

spec = importlib.util.spec_from_file_location("host166", Path(__file__).with_name("release166-primary-host-regression.py"))
host = importlib.util.module_from_spec(spec)
spec.loader.exec_module(host)
require, command, read_json = host.require, host.command, host.read_json


class Provider(host.Provider):
    def do_POST(self):
        try:
            body = json.loads(self.rfile.read(int(self.headers.get("Content-Length", "0"))))
            messages = body.get("messages", [])
            names = [t.get("function", {}).get("name", "") for t in body.get("tools", [])]
            self.server.requests.append({"phase": self.server.phase, "names": names})
            if not names:
                self.reply(body, content="Review integration fixture")
                return
            phase = self.server.phase
            if phase == "begin":
                text = host.flatten(messages)
                ids = set(re.findall(r"review-[a-f0-9]{32}", text))
                require(len(ids) == 1, "command did not inject exactly one Runtime-created runId")
                self.server.run_id = ids.pop()
                self.reply(body, content="Runtime run created; preparing snapshot.")
                return
            if phase == "menu":
                self.reply(body, content=self.server.menu)
                return
            last_user = max(i for i, m in enumerate(messages) if m.get("role") == "user")
            if any(m.get("role") == "tool" for m in messages[last_user + 1:]):
                self.reply(body, content="Proposal submitted.")
                return
            submit = next((n for n in names if host.normalized(n) == host.normalized(host.TOOL_NAME)), None)
            require(submit is not None, "installed submission tool unavailable")
            if phase == "selection":
                require(host.flatten(messages[last_user].get("content")).strip() == "选择 C1", "missing real next-user choice")
            payload = self.server.payload
            self.reply(body, call={"id": "call_167_" + phase, "type": "function", "function": {
                "name": submit, "arguments": json.dumps({"kind": phase, "runId": self.server.run_id, "proposal": json.dumps(payload)})}})
        except Exception as error:
            self.server.errors.append(str(error))
            raw = json.dumps({"error": str(error)}).encode()
            self.send_response(500)
            self.send_header("Content-Length", str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2) + "\n", encoding="utf-8")


def scenario(args, server, multiple):
    started = time.monotonic()
    name = "multiple" if multiple else "single"
    project = args.work_root / name
    project.mkdir(parents=True)
    source = args.harness_root
    shutil.copytree(source, project / ".code-harness", ignore=shutil.ignore_patterns("tools-runtime", "runs", "bin", "host"))
    bins = project / ".code-harness" / "bin"
    bins.mkdir()
    for file in ("codea-dcep-tools.exe", "ast-grep.exe"):
        shutil.copy2(source / "bin" / file, bins / file)
    runtime = bins / "codea-dcep-tools.exe"
    config_dir = project / ".opencode"
    (config_dir / "tools").mkdir(parents=True)
    (config_dir / "commands").mkdir()
    host_source = source.parent / ".opencode" if (source.parent / ".opencode").is_dir() else source
    shutil.copyfile(host_source / "tools" / "codea-reviewer-submit.ts", config_dir / "tools" / "codea-reviewer-submit.ts")
    shutil.copyfile(host_source / "commands" / "harness-review.md", config_dir / "commands" / "harness-review.md")
    env = dict(os.environ)
    for key in ("XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME"):
        env[key] = str(args.work_root / (name + "-" + key.lower()))
    for dest in (config_dir, Path(env["XDG_CONFIG_HOME"]) / "opencode"):
        dest.mkdir(parents=True, exist_ok=True)
        for file in ("package.json", "package-lock.json"):
            shutil.copyfile(args.sdk_root / file, dest / file)
        shutil.copytree(args.sdk_root / "node_modules", dest / "node_modules")
    # Runtime finds the real native binary via PATH, with no export shim.
    native_dir = args.work_root / "native"
    native_dir.mkdir(exist_ok=True)
    native = native_dir / ("opencode.exe" if os.name == "nt" else "opencode")
    if not native.exists():
        shutil.copy2(args.opencode, native)
    env["PATH"] = str(native_dir) + os.pathsep + env.get("PATH", "")
    for key in ("NO_PROXY", "no_proxy"):
        env[key] = ",".join(filter(None, [env.get(key, ""), "127.0.0.1", "localhost"]))
    env.update(OPENCODE_DISABLE_MODELS_FETCH="true", OPENCODE_DISABLE_AUTOUPDATE="true", OPENCODE_CONFIG_CONTENT="{}")
    env.pop("OPENCODE_CONFIG", None)
    write_json(project / "opencode.json", {
        "provider": {"fixture": {"npm": "@ai-sdk/openai-compatible", "options": {"baseURL": f"http://127.0.0.1:{server.server_port}/v1", "apiKey": "local-fixture"}, "models": {"primary167": {"name": "Deterministic transport", "limit": {"context": 64000, "output": 4096}}}}},
        "permission": {"*": "deny", host.TOOL_NAME: "allow"},
    })
    (project / ".gitignore").write_text(".code-harness/\n.opencode/\nopencode.json\n", encoding="utf-8")
    java = project / "src/main/java/example"
    java.mkdir(parents=True)
    methods = ["get", "list"] if multiple else ["get"]
    ctrl_path, svc_path = "src/main/java/example/OrderController.java", "src/main/java/example/OrderService.java"
    mapper_path, sql_path = "src/main/java/example/OrderMapper.java", "src/main/resources/mapper/OrderMapper.xml"
    def controller(changed):
        return "package example;\n@RestController\npublic class OrderController {\n private OrderService service;\n" + "".join(
            f' @GetMapping("/{m}")\n public int {m}() {{ return ' + (f"service.{m}()" if changed else "0") + "; }\n" for m in methods) + "}\n"
    (project / ctrl_path).write_text(controller(False), encoding="utf-8")
    (project / svc_path).write_text("package example;\n@Service\npublic class OrderService {\n private OrderMapper mapper;\n" + "".join(f" public int {m}() {{ return mapper.{m}(); }}\n" for m in methods) + "}\n", encoding="utf-8")
    (project / mapper_path).write_text("package example;\n@Mapper\npublic interface OrderMapper {\n" + "".join(f" int {m}();\n" for m in methods) + "}\n", encoding="utf-8")
    (project / sql_path).parent.mkdir(parents=True)
    (project / sql_path).write_text('<mapper namespace="example.OrderMapper">' + ''.join(f'<select id="{m}" resultType="int">SELECT 1</select>' for m in methods) + '</mapper>', encoding="utf-8")
    command(["git", "init"], project, env)
    command(["git", "add", "."], project, env)
    command(["git", "-c", "user.name=Review Fixture", "-c", "user.email=review@example.test", "commit", "-m", "base"], project, env)
    (project / ctrl_path).write_text(controller(True), encoding="utf-8")
    (project / sql_path).write_text((project / sql_path).read_text(encoding="utf-8").replace("SELECT 1", "SELECT 2"), encoding="utf-8")
    cfg = (source / "harness.template.yaml").read_text(encoding="utf-8").replace('baseRef: ""', "baseRef: HEAD").replace("status: NEEDS_CONFIRMATION", "status: READY").replace("unresolved:\n    - projectNotInitialized", "unresolved: []")
    (project / ".code-harness/harness.yaml").write_text(cfg, encoding="utf-8")
    def rt(*argv):
        then = time.monotonic()
        result = command([str(runtime), *argv], project, env, timeout=90)
        print(f"RELEASE167_RUNTIME scenario={name} action={' '.join(argv[:2])} elapsed={time.monotonic()-then:.3f}s", flush=True)
        return json.loads(result)
    nav_start = time.monotonic()
    nav = rt("nav", "find-by-annotation", "--annotation", "RestController", "--scope", "src/main/java")
    require("OrderController" in json.dumps(nav), "real annotation query lost Controller")
    nav_seconds = time.monotonic() - nav_start
    server.phase, server.run_id = "begin", None
    run = [str(native), "run", "--dir", str(project), "--model", "fixture/primary167", "--format", "json"]
    begin_output = command(run + ["--command", "harness-review", "OrderController"], project, env)
    require(server.run_id and not server.errors, f"entry failed: {server.errors}\n{begin_output}")
    run_id = server.run_id
    run_root = project / ".code-harness/runs" / run_id
    require(read_json(run_root / "runtime/review-progress.json")["currentStage"] == "SNAPSHOT", "entry did not create real run state")
    sessions = [json.loads(line).get("sessionID") for line in begin_output.splitlines() if line.startswith("{")]
    session = next((s for s in sessions if s), None)
    require(session, "entry session missing")
    def request(file, data):
        rel = f".code-harness/runs/{run_id}/requests/{file}.json"
        write_json(project / rel, data)
        return rel
    snapshot = rt("analysis", "snapshot", "--input", request("snapshot", {"runId": run_id, "baseRef": "HEAD", "includeWorkingTree": True}))
    refs = lambda m: [{"path": ctrl_path, "symbol": "OrderController." + m}, {"path": svc_path, "symbol": "OrderService." + m}, {"path": mapper_path, "symbol": "OrderMapper." + m}]
    proposal = {
        "changedFileRoles": [{"path": ctrl_path, "role": "Controller"}, {"path": sql_path, "role": "MapperXml"}],
        "affectedControllers": [{"controller": "OrderController", "endpoints": ["OrderController." + m for m in methods], "impactType": "DIRECT_CHANGE", "sourceSymbols": ["OrderController." + m for m in methods]}],
        "callChains": [{"entryPoint": "OrderController." + m, "entryPointRef": refs(m)[0], "chain": [r["symbol"] for r in refs(m)], "chainRefs": refs(m)} for m in methods],
        "symbolLocations": [{**r, "role": "Controller" if r["path"] == ctrl_path else "Service" if r["path"] == svc_path else "Mapper", "source": "FIND_SYMBOL"} for m in methods for r in refs(m)],
        "resourceRelations": [{"path": sql_path, "role": "MapperXml", "resource": "OrderMapper.xml#"+m, "fromSymbol": "OrderMapper."+m, "fromKind": "METHOD", "source": "MAPPER_STATEMENT", "evidence": "statement id " + m + " matches OrderMapper."+m} for m in methods], "externalDependencies": [], "riskAreas": [],
        "reviewCoverage": {"status": "COMPLETE", "reviewedFiles": [{"path": ctrl_path, "role": "Controller", "reason": "CHANGED"}, {"path": svc_path, "role": "Service", "reason": "CALL_CHAIN"}, {"path": mapper_path, "role": "Mapper", "reason": "CALL_CHAIN"}, {"path": sql_path, "role": "MapperXml", "reason": "CHANGED"}], "unresolvedSymbols": []},
    }
    def host_phase(kind, payload, words):
        server.phase, server.payload = kind, payload
        output = command(run + ["--session", session, *words], project, env)
        require(not server.errors, f"Host errors: {server.errors}\n{output}")
        require('"type":"error"' not in output, f"Host failure: {output}")
    # Submit a broken parallel chain array through the actual installed tool.
    # The tool must reject before writing either proposal or attestation.
    malformed = json.loads(json.dumps(proposal))
    malformed["callChains"][0]["chainRefs"].pop()
    host_phase("change-analysis", malformed, ["Submit", "malformed", "analysis"])
    require(not (run_root / "requests/change-analysis-proposal.json").exists(), "malformed chainRefs published a proposal")
    require(not (run_root / "requests/change-analysis-reviewer-authority.json").exists(), "malformed chainRefs published authority")
    require(read_json(run_root / "runtime/review-progress.json")["currentStage"] == "CHANGE_ANALYSIS", "malformed submission advanced progress")
    host_phase("change-analysis", proposal, ["Submit", "analysis"])
    rt("analysis", "certify", "--input", request("certify", {"runId": run_id, "snapshotPath": snapshot["artifactPath"], "snapshotSha256": snapshot["snapshotSha256"], "proposalPath": f".code-harness/runs/{run_id}/requests/change-analysis-proposal.json", "intent": {"mode": "TARGETED", "target": "OrderController"}}))
    options_result = rt("review", "options", "--input", request("options", {"runId": run_id, "changeAnalysisPath": f".code-harness/runs/{run_id}/analysis/change-analysis.json", "target": "OrderController"}))
    options = read_json(run_root / "analysis/review-options.json")
    require(options["decision"] == ("USER_SELECTION" if multiple else "AUTO_SINGLE"), f"wrong selection decision {options}")
    selection = {"runId": run_id, "mode": "TARGETED", "optionsHash": options["optionsHash"], "selectionIds": ["C1"]}
    if multiple:
        # A fabricated selection must be rejected before any scope is written.
        early = request("early-selection", selection)
        import subprocess
        rejected = subprocess.run([str(runtime), "review", "select", "--input", early], cwd=project, env=env, capture_output=True, text=True, encoding="utf-8", timeout=90)
        require(rejected.returncode != 0 and not (run_root / "analysis/review-scope.json").exists(), "multiple chains accepted without human selection")
        server.menu = options_result["selectionPrompt"]
        host_phase("menu", None, ["Display", "options"])
        host_phase("selection", selection, ["选择", "C1"])
        selected_input = f".code-harness/runs/{run_id}/requests/review-selection.json"
    else:
        selected_input = request("select", selection)
    rt("review", "select", "--input", selected_input)
    rt("review", "units", "--run-id", run_id)
    rt("review", "dispatch", "--run-id", run_id)
    host_phase("findings", [], ["Submit", "findings"])
    rt("review", "certify-findings", "--input", request("findings-certify", {"runId": run_id, "proposalsPath": f".code-harness/runs/{run_id}/requests/finding-proposals.json"}))
    scope = read_json(run_root / "analysis/review-scope.json")
    transport = {"runId": run_id, "harnessVersion": "1.6.7", "baseRef": "HEAD", "head": snapshot["headCommit"], "result": "PASSED", "mode": scope["mode"], "target": scope["target"], "reviewScope": {"changedFiles": [ctrl_path, sql_path], "scopedFiles": scope["scopedFiles"]}, "reviewCoverage": {"reviewedFiles": scope["scopedFiles"], "callChains": [{"entryPoint": c["entryPoint"], "chain": c["chain"]} for c in scope["selectedCallChains"]], "externalDependencies": [], "unresolved": [], "missingReviewedFiles": [], "runtimeErrors": [], "status": "COMPLETE"}, "findings": []}
    rt("report", "review", "--input", request("report", transport))
    report_path = run_root / "review.md"
    require(report_path.is_file() and "本次评审通过" in report_path.read_text(encoding="utf-8"), "Runtime did not generate passed review.md")
    require(read_json(run_root / "runtime/review-progress.json")["status"] == "SUCCEEDED", "run did not finish")
    elapsed = time.monotonic() - started
    result = {"scenario": name, "status": "PASS", "runId": run_id, "sessionId": session, "navSeconds": round(nav_seconds, 3), "totalSeconds": round(elapsed, 3), "reportPath": str(report_path), "humanSelection": multiple, "modelReasoningVerified": False}
    print("RELEASE167_FULL_REVIEW " + json.dumps(result), flush=True)
    return result


def main():
    parser = argparse.ArgumentParser()
    for option in ("opencode", "sdk-root", "harness-root", "work-root"):
        parser.add_argument("--" + option, type=Path, required=True)
    args = parser.parse_args()
    for key, value in vars(args).items():
        setattr(args, key, value.resolve())
    args.work_root.mkdir(parents=True, exist_ok=True)
    server = ThreadingHTTPServer(("127.0.0.1", 0), Provider)
    server.phase, server.requests, server.errors = "begin", [], []
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    stop = threading.Event()
    def heartbeat():
        while not stop.wait(15):
            print(f"RELEASE167_HEARTBEAT phase={server.phase} requests={len(server.requests)} errors={server.errors}", flush=True)
    threading.Thread(target=heartbeat, daemon=True).start()
    try:
        results = [scenario(args, server, multiple) for multiple in (False, True)]
        write_json(args.work_root / "review-evidence.json", {"status": "PASS", "scenarios": results, "boundary": "Real Host transport and Runtime lifecycle; deterministic provider does not validate private-model reasoning."})
    finally:
        stop.set()
        server.shutdown()
        server.server_close()


if __name__ == "__main__":
    main()
