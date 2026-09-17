#!/usr/bin/env python3
"""Codea Harness 1.8 real-model autonomous single-chain acceptance smoke.

This imports transport helpers from review180-host-smoke.py but does not use its
fixture provider. Native OpenCode talks to the repository-configured
OpenAI-compatible secret model. The driver sends only the normal /harness-review
user request and observes model/tool behavior; it never invokes review
prepare/select/finish.
"""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import tempfile

HERE = Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location("review180_host", HERE / "review180-host-smoke.py")
host = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(host)


def copy_sdk(sdk_root: Path, dest: Path):
    dest.mkdir(parents=True, exist_ok=True)
    for name in ("package.json", "package-lock.json"):
        shutil.copyfile(sdk_root / name, dest / name)
    if not (dest / "node_modules").exists():
        shutil.copytree(sdk_root / "node_modules", dest / "node_modules")


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


def session_id(stdout: str, binary: Path, project: Path, env: dict) -> str:
    match = re.search(r'"sessionID"\s*:\s*"([^"]+)"', stdout)
    if match:
        return match.group(1)
    listed = host.command([str(binary), "session", "list", "--format", "json"], project, env, timeout=30)
    rows = json.loads(listed)
    host.require(rows, "real-model smoke: native OpenCode session missing")
    return rows[0]["id"]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--opencode", required=True, type=Path)
    parser.add_argument("--runtime", required=True, type=Path)
    parser.add_argument("--ast-grep", required=True, type=Path)
    parser.add_argument("--tool-source", required=True, type=Path)
    parser.add_argument("--command-source", required=True, type=Path)
    parser.add_argument("--sdk-root", required=True, type=Path)
    args = parser.parse_args()

    base_url = os.environ.get("TASK15_OPENAI_BASE_URL", "").strip()
    api_key = os.environ.get("TASK15_OPENAI_API_KEY", "").strip()
    host.require(base_url, "real-model smoke requires TASK15_OPENAI_BASE_URL")
    host.require(api_key, "real-model smoke requires TASK15_OPENAI_API_KEY")
    for value in (args.opencode, args.runtime, args.ast_grep, args.tool_source, args.command_source):
        host.require(value.resolve().is_file(), f"required file missing: {value}")
    agent_source = args.command_source.parent.parent / "agents" / "orchestrator.md"
    host.require(agent_source.resolve().is_file(), f"orchestrator agent source missing: {agent_source}")
    sdk_root = args.sdk_root.resolve()

    with tempfile.TemporaryDirectory(prefix="Codea 180 Real Model & ") as raw:
        temp = Path(raw)
        project = temp / "single-issue"
        (project / ".opencode" / "tools").mkdir(parents=True)
        (project / ".opencode" / "commands").mkdir(parents=True)
        (project / ".opencode" / "agents").mkdir(parents=True)
        (project / ".code-harness" / "bin").mkdir(parents=True)
        shutil.copyfile(args.tool_source, project / ".opencode" / "tools" / "codea-review.ts")
        shutil.copyfile(args.command_source, project / ".opencode" / "commands" / "harness-review.md")
        shutil.copyfile(agent_source, project / ".opencode" / "agents" / "orchestrator.md")
        shutil.copyfile(args.runtime, project / ".code-harness" / "bin" / ("codea-dcep-tools.exe" if os.name == "nt" else "codea-dcep-tools"))
        shutil.copyfile(args.ast_grep, project / ".code-harness" / "bin" / ("ast-grep.exe" if os.name == "nt" else "ast-grep"))
        host.write_fixture(project, False)
        (project / ".gitignore").write_text(".opencode/node_modules/\n", encoding="utf-8")

        env = dict(os.environ)
        for key, suffix in (("XDG_CONFIG_HOME", "xdg-config"), ("XDG_DATA_HOME", "xdg-data"), ("XDG_CACHE_HOME", "xdg-cache"), ("XDG_STATE_HOME", "xdg-state")):
            env[key] = str(temp / suffix)
        env["OPENCODE_DISABLE_MODELS_FETCH"] = "true"
        env["OPENCODE_DISABLE_AUTOUPDATE"] = "true"
        env["OPENCODE_CONFIG_CONTENT"] = "{}"
        env.pop("OPENCODE_CONFIG", None)
        env["PATH"] = str(args.opencode.resolve().parent) + os.pathsep + env.get("PATH", "")
        copy_sdk(sdk_root, project / ".opencode")
        copy_sdk(sdk_root, Path(env["XDG_CONFIG_HOME"]) / "opencode")

        config = {
            "provider": {
                "task15secret": {
                    "npm": "@ai-sdk/openai-compatible",
                    "name": "Configured secret real acceptance",
                    "options": {
                        "baseURL": "{env:TASK15_OPENAI_BASE_URL}",
                        "apiKey": "{env:TASK15_OPENAI_API_KEY}",
                    },
                    "models": {
                        "deepseek-v4-pro": {
                            "name": "DeepSeek V4 Pro",
                            "limit": {"context": 128000, "output": 8192},
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
            },
        }
        (project / "opencode.json").write_text(json.dumps(config), encoding="utf-8")

        host.command(["git", "init"], project, env, timeout=20)
        host.command(["git", "add", "."], project, env, timeout=20)
        host.command(["git", "-c", "user.name=Real Model Fixture", "-c", "user.email=real-model@example.test", "commit", "-m", "baseline"], project, env, timeout=20)

        controller = project / "src" / "main" / "java" / "com" / "example" / "OrderController.java"
        source = controller.read_text(encoding="utf-8")
        source = source.replace(
            "public void create() { service.create(); }",
            'public void create() { service.create(); throw new IllegalStateException("always fails after successful create"); }',
        )
        host.require("always fails after successful create" in source, "failed to inject explicit issue fixture")
        controller.write_text(source, encoding="utf-8")

        binary = args.opencode.resolve()
        version = host.command([str(binary), "--version"], temp, env, timeout=30).strip()
        host.require(version == "1.18.25", f"expected OpenCode 1.18.25, got {version}")
        model = "task15secret/deepseek-v4-pro"
        run = [str(binary), "run", "--print-logs", "--dir", str(project), "--model", model, "--format", "json"]
        stdout = host.command(run + ["--command", "harness-review", "OrderController"], project, env, timeout=240)
        sid = session_id(stdout, binary, project, env)
        exported = json.loads(host.command([str(binary), "export", sid], project, env, timeout=30))
        actions = host.tool_actions(exported)
        host.require("prepare" in actions, f"real model never called prepare: {actions}\n{stdout[-5000:]}")
        host.require("finish" in actions, f"real model never autonomously called finish: {actions}\n{stdout[-5000:]}")
        host.require("select" not in actions, f"single-chain real-model smoke unexpectedly selected: {actions}")
        names = [host.normalized(name) for name in completed_tools(exported)]
        host.require("read" in names, f"real model did not use native source read tool: {names}")

        runs = list((project / ".code-harness" / "runs").glob("review-*"))
        host.require(len(runs) == 1, f"expected one durable run, got {runs}")
        run_dir = runs[0]
        result = json.loads((run_dir / "result.json").read_text(encoding="utf-8"))
        findings = result.get("findings", [])
        host.require(findings, f"real model missed explicit always-fail issue: {result}")
        host.require(all(item.get("evidence") for item in findings), f"real model produced finding without evidence: {findings}")
        report = (run_dir / "review.md").read_text(encoding="utf-8")
        host.require("execution\":\"COMPLETE" in report or "评审完成" in report, f"durable report not complete:\n{report}")
        host.require(result.get("reviewConclusion") in {"BLOCKING", "ACTION_REQUIRED"}, f"issue run has weak conclusion: {result}")
        print(
            "REVIEW180_REAL_MODEL PASS "
            f"model={model} runId={run_dir.name} actions={actions} findings={len(findings)} "
            f"conclusion={result.get('reviewConclusion')}",
            flush=True,
        )


if __name__ == "__main__":
    main()
