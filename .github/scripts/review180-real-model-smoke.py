#!/usr/bin/env python3
"""Codea Harness 1.8 real-model autonomous single-chain acceptance smoke.

The driver sends only the normal /harness-review user request and observes the
real model/tool behavior. It never invokes review prepare/select/finish itself.
Acceptance evidence is retained in a caller-provided directory with secrets
redacted and every durable artifact hashed.
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

HERE = Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location("review180_host", HERE / "review180-host-smoke.py")
host = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(host)

SEED_TEXT = 'throw new IllegalStateException("always fails after successful create")'


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


def parse_tool_output(state):
    raw = state.get("output")
    if isinstance(raw, dict):
        return raw
    if isinstance(raw, str):
        return json.loads(raw)
    raise RuntimeError(f"real-model smoke: completed finish has no JSON output: {state}")


def redact_sensitive(value):
    if isinstance(value, list):
        return [redact_sensitive(item) for item in value]
    if not isinstance(value, dict):
        return value
    out = {}
    for key, item in value.items():
        lowered = key.lower()
        if any(token in lowered for token in ("apikey", "api_key", "token", "authorization", "password", "secret")):
            out[key] = "<redacted>"
        else:
            out[key] = redact_sensitive(item)
    return out


def sha256_file(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def write_json(path: Path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def canonical_report_path(project: Path, value) -> str:
    raw = str(value or "").strip()
    host.require(raw, "finish reportPath missing")
    candidate = Path(raw)
    if not candidate.is_absolute():
        candidate = project / candidate
    return os.path.normcase(os.path.realpath(os.path.abspath(str(candidate))))


def validate_report_identity(project: Path, report_path: Path, finish_runtime: dict) -> str:
    expected = canonical_report_path(project, report_path)
    actual = canonical_report_path(project, finish_runtime.get("reportPath"))
    host.require(
        actual == expected,
        f"finish reportPath mismatch: returned={finish_runtime.get('reportPath')} expected={report_path}",
    )
    report_sha = sha256_file(report_path)
    host.require(
        finish_runtime.get("reportSha256") == report_sha,
        f"finish reportSha256 mismatch: runtime={finish_runtime.get('reportSha256')} disk={report_sha}",
    )
    return report_sha


def write_manifest(evidence_dir: Path, run_id, model):
    manifest_files = ["trajectory.json", "review.md", "result.json", "scope.json", "run.json"]
    files = {}
    for name in manifest_files:
        path = evidence_dir / name
        if path.is_file():
            files[name] = {"sha256": sha256_file(path), "bytes": path.stat().st_size}
    manifest = {
        "schemaVersion": "codea.review180.acceptance-manifest.v1",
        "runId": run_id,
        "model": model,
        "files": files,
    }
    write_json(evidence_dir / "manifest.json", manifest)


def persist_partial_evidence(
    evidence_dir: Path,
    project: Path,
    exported,
    model: str,
    sid: str,
    actions,
    names,
):
    write_json(evidence_dir / "trajectory.json", redact_sensitive(exported))
    runs = list((project / ".code-harness" / "runs").glob("review-*"))
    run_id = runs[0].name if len(runs) == 1 else None
    copied = []
    if len(runs) == 1:
        for name in ("review.md", "result.json", "scope.json"):
            source = runs[0] / name
            if source.is_file():
                shutil.copyfile(source, evidence_dir / name)
                copied.append(name)
    run_evidence = {
        "schemaVersion": "codea.review180.acceptance.v1",
        "model": model,
        "runId": run_id,
        "sessionId": sid,
        "actions": actions,
        "completedTools": names,
        "availableArtifacts": copied,
        "acceptance": "PENDING_FINISH",
    }
    write_json(evidence_dir / "run.json", redact_sensitive(run_evidence))
    write_manifest(evidence_dir, run_id, model)


def persist_evidence(
    evidence_dir: Path,
    exported,
    report_path: Path,
    result_path: Path,
    scope_path: Path,
    run_evidence: dict,
):
    write_json(evidence_dir / "trajectory.json", redact_sensitive(exported))
    shutil.copyfile(report_path, evidence_dir / "review.md")
    shutil.copyfile(result_path, evidence_dir / "result.json")
    shutil.copyfile(scope_path, evidence_dir / "scope.json")
    write_json(evidence_dir / "run.json", redact_sensitive(run_evidence))
    write_manifest(evidence_dir, run_evidence.get("runId"), run_evidence.get("model"))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--opencode", required=True, type=Path)
    parser.add_argument("--runtime", required=True, type=Path)
    parser.add_argument("--ast-grep", required=True, type=Path)
    parser.add_argument("--tool-source", required=True, type=Path)
    parser.add_argument("--command-source", required=True, type=Path)
    parser.add_argument("--sdk-root", required=True, type=Path)
    parser.add_argument("--evidence-dir", required=True, type=Path)
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
    evidence_dir = args.evidence_dir.resolve()
    evidence_dir.mkdir(parents=True, exist_ok=True)

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
                            "limit": {"context": 128000, "output": 16384},
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
            f"public void create() {{ service.create(); {SEED_TEXT}; }}",
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
        names = [host.normalized(name) for name in completed_tools(exported)]
        persist_partial_evidence(evidence_dir, project, exported, model, sid, actions, names)
        host.require("prepare" in actions, f"real model never called prepare: {actions}\n{stdout[-5000:]}")
        host.require("finish" in actions, f"real model never autonomously called finish: {actions}\n{stdout[-5000:]}")
        host.require("select" not in actions, f"single-chain real-model smoke unexpectedly selected: {actions}")
        host.require("read" in names, f"real model did not use native source read tool: {names}")

        finish_states = [state for state in host.tool_parts(exported, "finish") if state.get("status") == "completed"]
        host.require(len(finish_states) == 1, f"expected exactly one completed finish call: {finish_states}")
        finish_payload = parse_tool_output(finish_states[0])
        finish_runtime = finish_payload.get("runtime", {})
        host.require(finish_runtime.get("execution") == "COMPLETE", f"finish runtime did not complete: {finish_runtime}")

        runs = list((project / ".code-harness" / "runs").glob("review-*"))
        host.require(len(runs) == 1, f"expected one durable run, got {runs}")
        run_dir = runs[0]
        result_path = run_dir / "result.json"
        scope_path = run_dir / "scope.json"
        report_path = run_dir / "review.md"
        for artifact in (result_path, scope_path, report_path):
            host.require(artifact.is_file(), f"required durable artifact missing: {artifact}")

        result = json.loads(result_path.read_text(encoding="utf-8"))
        findings = result.get("findings", [])
        seeded_issue_evidence = []
        for item in findings:
            for evidence in item.get("evidence", []):
                quote = evidence.get("quote", "")
                if "always fails after successful create" in quote or "IllegalStateException" in quote:
                    seeded_issue_evidence.append({"findingId": item.get("id"), "quote": quote, "ref": evidence.get("ref")})

        report = report_path.read_text(encoding="utf-8")
        report_sha = sha256_file(report_path)
        expected_report_path = f".code-harness/runs/{run_dir.name}/review.md"
        run_evidence = {
            "schemaVersion": "codea.review180.acceptance.v1",
            "model": model,
            "runId": run_dir.name,
            "sessionId": sid,
            "actions": actions,
            "completedTools": names,
            "finish_runtime": finish_runtime,
            "seeded_issue_evidence": seeded_issue_evidence,
            "reviewConclusion": result.get("reviewConclusion"),
            "findingCount": len(findings),
            "reportPath": expected_report_path,
            "returnedReportPath": finish_runtime.get("reportPath"),
            "reportSha256": report_sha,
            "acceptance": "PENDING_ASSERTIONS",
        }

        # Preserve redacted diagnostics before final acceptance assertions so a
        # later seeded-issue/path/hash failure still leaves a reviewable artifact.
        persist_evidence(evidence_dir, exported, report_path, result_path, scope_path, run_evidence)

        host.require(findings, f"real model missed explicit always-fail issue: {result}")
        host.require(all(item.get("evidence") for item in findings), f"real model produced finding without evidence: {findings}")
        host.require(seeded_issue_evidence, f"findings did not identify the seeded throw: {findings}")
        host.require("execution\\\":\\\"COMPLETE" in report or "评审完成" in report, f"durable report not complete:\\n{report}")
        host.require(result.get("reviewConclusion") in {"BLOCKING", "ACTION_REQUIRED"}, f"issue run has weak conclusion: {result}")
        report_sha = validate_report_identity(project, report_path, finish_runtime)
        host.require(result.get("runId") == run_dir.name, f"result runId mismatch: {result.get('runId')} != {run_dir.name}")

        run_evidence["reportSha256"] = report_sha
        run_evidence["acceptance"] = "PASS"
        persist_evidence(evidence_dir, exported, report_path, result_path, scope_path, run_evidence)

        print(
            "REVIEW180_REAL_MODEL PASS "
            f"model={model} runId={run_dir.name} actions={actions} findings={len(findings)} "
            f"seededIssue=true conclusion={result.get('reviewConclusion')} reportSha256={report_sha}",
            flush=True,
        )


if __name__ == "__main__":
    main()
