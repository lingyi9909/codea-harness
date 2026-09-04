#!/usr/bin/env python3
"""Stateful deterministic OpenAI-compatible model for Task 2 Fresh Review Lifecycle E2E.

The model never edits the fixture itself. It emits ordinary OpenCode bash tool calls.
Every top-level `harness review` first invokes Runtime `review begin`, then discovers
that invocation's actual Runtime-owned runId from the tool result and uses it for the
rest of the authority chain. No runId is hard-coded or shared between invocations.
"""

from __future__ import annotations

import argparse
import json
import re
import time
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any

RUN_RE = re.compile(r"review-[0-9a-f]{32}")


def flatten(value: Any) -> str:
    if isinstance(value, str):
        return value
    if isinstance(value, list):
        return "\n".join(flatten(v) for v in value)
    if isinstance(value, dict):
        return "\n".join(f"{k}:{flatten(v)}" for k, v in value.items())
    return str(value)


def review_user_count(messages: list[dict[str, Any]]) -> int:
    count = 0
    for message in messages:
        if message.get("role") == "user" and "harness review" in flatten(message.get("content", "")):
            count += 1
    return max(count, 1)


def current_run_id(messages: list[dict[str, Any]]) -> str | None:
    matches = RUN_RE.findall(flatten(messages))
    return matches[-1] if matches else None


def marker(invocation: int, stage: int) -> str:
    return f"TASK2_INV_{invocation}_STAGE_{stage:02d} PASS"


def next_stage(messages: list[dict[str, Any]], invocation: int, stage_count: int) -> int:
    text = flatten(messages)
    highest = -1
    for index in range(stage_count):
        if marker(invocation, index) in text:
            highest = index
    return highest + 1


def ps_json_write(path: str, expression: str) -> str:
    return (
        f"$value = {expression}; "
        f"$json = $value | ConvertTo-Json -Depth 30 -Compress; "
        f"[System.IO.File]::WriteAllText('{path}', $json, [System.Text.UTF8Encoding]::new($false))"
    )


def stages_for(invocation: int, run_id: str | None) -> list[tuple[str, str]]:
    changed = invocation >= 2
    stage_mark = lambda i: marker(invocation, i)

    stages: list[tuple[str, str]] = [
        (
            "Read active Fresh Review Lifecycle contracts",
            "$paths=@('.code-harness/AGENTS.md','.code-harness/tools/README.md','.code-harness/agents/orchestrator.md'); "
            "foreach($p in $paths){ Write-Output ('TASK2_ACTIVE_READ '+$p); "
            "Select-String -Path $p -Pattern 'review begin|Fresh Review Lifecycle|每一次新的顶层|same-run|analysis snapshot' | ForEach-Object { $_.Line } }; "
            f"Write-Output '{stage_mark(0)}'",
        ),
        (
            "Begin a fresh Runtime Review invocation",
            "& ./.code-harness/bin/codea-dcep-tools.exe review begin; "
            "if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; "
            f"Write-Output '{stage_mark(1)}'",
        ),
    ]

    if run_id is None:
        # Commands after stage 1 are only emitted after the actual runId is visible in
        # OpenCode's message history. Keep placeholders impossible to execute early.
        run_id = "TASK2_RUN_ID_NOT_YET_DISCOVERED"

    request_root = f".code-harness/runs/{run_id}/requests"
    analysis_root = f".code-harness/runs/{run_id}/analysis"

    if changed:
        semantic = (
            "[ordered]@{changedFileRoles=@([ordered]@{path='src/main/resources/application.yml';role='YamlConfig'});"
            "affectedControllers=@();callChains=@();symbolLocations=@();resourceRelations=@();externalDependencies=@();riskAreas=@();"
            "reviewCoverage=[ordered]@{status='COMPLETE';reviewedFiles=@([ordered]@{path='src/main/resources/application.yml';role='YamlConfig';reason='CHANGED'});unresolvedSymbols=@()}}"
        )
    else:
        semantic = (
            "[ordered]@{changedFileRoles=@();affectedControllers=@();callChains=@();symbolLocations=@();resourceRelations=@();"
            "externalDependencies=@();riskAreas=@();reviewCoverage=[ordered]@{status='COMPLETE';reviewedFiles=@();unresolvedSymbols=@()}}"
        )

    stages.extend(
        [
            (
                "Create this invocation's canonical Snapshot request",
                f"$run='{run_id}'; New-Item -ItemType Directory -Force '{request_root}' | Out-Null; "
                + ps_json_write(
                    f"{request_root}/change-set-request.json",
                    "[ordered]@{runId=$run;baseRef='review-base';includeWorkingTree=$true}",
                )
                + f"; Write-Output '{stage_mark(2)}'",
            ),
            (
                "Recompute Runtime Canonical Snapshot for this fresh invocation",
                f"& ./.code-harness/bin/codea-dcep-tools.exe analysis snapshot --input {request_root}/change-set-request.json; "
                f"if ($LASTEXITCODE -ne 0) {{ exit $LASTEXITCODE }}; Write-Output '{stage_mark(3)}'",
            ),
            (
                "Read this invocation's Runtime Snapshot",
                f"Get-Content -Raw {analysis_root}/change-set.json; "
                "if (Test-Path 'src/main/resources/application.yml') { Get-Content -Raw 'src/main/resources/application.yml' }; "
                f"Write-Output '{stage_mark(4)}'",
            ),
            (
                "Create semantic ChangeAnalysis proposal for current fresh Snapshot",
                ps_json_write(f"{request_root}/change-analysis-proposal.json", semantic)
                + f"; Write-Output '{stage_mark(5)}'",
            ),
            (
                "Create same-run ChangeAnalysis certification request",
                f"$run='{run_id}'; $s=Get-Content -Raw {analysis_root}/change-set.json | ConvertFrom-Json; "
                + ps_json_write(
                    f"{request_root}/analysis-certify-request.json",
                    f"[ordered]@{{runId=$run;snapshotPath='{analysis_root}/change-set.json';snapshotSha256=[string]$s.snapshotSha256;proposalPath='{request_root}/change-analysis-proposal.json';intent=[ordered]@{{mode='FULL'}}}}",
                )
                + f"; Write-Output '{stage_mark(6)}'",
            ),
            (
                "Certify same-run ChangeAnalysis against live Git state",
                f"& ./.code-harness/bin/codea-dcep-tools.exe analysis certify --input {request_root}/analysis-certify-request.json; "
                f"if ($LASTEXITCODE -ne 0) {{ exit $LASTEXITCODE }}; Write-Output '{stage_mark(7)}'",
            ),
            (
                "Read Certified ChangeAnalysis authority",
                f"Get-Content -Raw {analysis_root}/change-analysis.json; Get-Content -Raw {analysis_root}/entrypoint-inventory.json; "
                f"Get-Content -Raw {analysis_root}/change-analysis.cert.json; Write-Output '{stage_mark(8)}'",
            ),
            (
                "Create Review Options request",
                ps_json_write(
                    f"{request_root}/review-options-request.json",
                    f"[ordered]@{{runId='{run_id}';changeAnalysisPath='{analysis_root}/change-analysis.json'}}",
                )
                + f"; Write-Output '{stage_mark(9)}'",
            ),
            (
                "Invoke Runtime Review Options",
                f"& ./.code-harness/bin/codea-dcep-tools.exe review options --input {request_root}/review-options-request.json; "
                f"if ($LASTEXITCODE -ne 0) {{ exit $LASTEXITCODE }}; Write-Output '{stage_mark(10)}'",
            ),
            (
                "Read Runtime Review Options",
                f"Get-Content -Raw {analysis_root}/review-options.json; Write-Output '{stage_mark(11)}'",
            ),
            (
                "Create AUTO_FULL Review selection",
                f"$run='{run_id}'; $o=Get-Content -Raw {analysis_root}/review-options.json | ConvertFrom-Json; "
                + ps_json_write(
                    f"{request_root}/review-selection-request.json",
                    "[ordered]@{runId=$run;optionsHash=[string]$o.optionsHash;mode='FULL';selectionIds=@()}",
                )
                + f"; Write-Output '{stage_mark(12)}'",
            ),
            (
                "Invoke Runtime Review selection",
                f"& ./.code-harness/bin/codea-dcep-tools.exe review select --input {request_root}/review-selection-request.json; "
                f"if ($LASTEXITCODE -ne 0) {{ exit $LASTEXITCODE }}; Write-Output '{stage_mark(13)}'",
            ),
            (
                "Read Runtime verified Review Scope",
                f"Get-Content -Raw {analysis_root}/review-scope.json; Write-Output '{stage_mark(14)}'",
            ),
            (
                "Build same-run Runtime Review Units",
                f"& ./.code-harness/bin/codea-dcep-tools.exe review units --run-id {run_id}; "
                f"if ($LASTEXITCODE -ne 0) {{ exit $LASTEXITCODE }}; Write-Output '{stage_mark(15)}'",
            ),
            (
                "Build same-run Runtime Rule Dispatch",
                f"& ./.code-harness/bin/codea-dcep-tools.exe review dispatch --run-id {run_id}; "
                f"if ($LASTEXITCODE -ne 0) {{ exit $LASTEXITCODE }}; Write-Output '{stage_mark(16)}'",
            ),
            (
                "Read Finding Certification request schema",
                f"Get-Content -Raw .code-harness/contracts/finding-certify-request.schema.json; Write-Output '{stage_mark(17)}'",
            ),
            (
                "Create empty Finding proposals and same-run certification request",
                f"Get-Content -Raw {analysis_root}/review-units.json; Get-Content -Raw {analysis_root}/rule-dispatch.json; "
                f"[System.IO.File]::WriteAllText('{request_root}/finding-proposals.json','[]',[System.Text.UTF8Encoding]::new($false)); "
                + ps_json_write(
                    f"{request_root}/finding-certify-request.json",
                    f"[ordered]@{{runId='{run_id}';proposalsPath='{request_root}/finding-proposals.json'}}",
                )
                + f"; Write-Output '{stage_mark(18)}'",
            ),
            (
                "Invoke Runtime Finding Certification",
                f"& ./.code-harness/bin/codea-dcep-tools.exe review certify-findings --input {request_root}/finding-certify-request.json; "
                f"if ($LASTEXITCODE -ne 0) {{ exit $LASTEXITCODE }}; Write-Output '{stage_mark(19)}'",
            ),
            (
                "Read same-run Certified Findings",
                f"Get-Content -Raw {analysis_root}/certified-findings.json; Get-Content -Raw {analysis_root}/certified-findings.cert.json; "
                f"Write-Output '{stage_mark(20)}'",
            ),
            (
                "Read formal Report Review request schema",
                f"Get-Content -Raw .code-harness/contracts/report-review-request.schema.json; Write-Output '{stage_mark(21)}'",
            ),
            (
                "Create same-run Report Review request",
                f"$run='{run_id}'; $a=Get-Content -Raw {analysis_root}/change-analysis.json | ConvertFrom-Json; "
                "$paths=@($a.changedFiles | ForEach-Object {[string]$_.path}); "
                + ps_json_write(
                    f"{request_root}/report-review.json",
                    "[ordered]@{runId=$run;harnessVersion='runtime-owned';baseRef=[string]$a.reviewScope.baseRef;head=[string]$a.reviewScope.headCommit;result='PASSED';mode='FULL';reviewScope=[ordered]@{changedFiles=$paths};reviewCoverage=[ordered]@{reviewedFiles=$paths;callChains=@();externalDependencies=@();unresolved=@();missingReviewedFiles=@();runtimeErrors=@();status='COMPLETE'};findings=@()}",
                )
                + f"; Write-Output '{stage_mark(22)}'",
            ),
            (
                "Invoke same-run Runtime Review renderer",
                f"& ./.code-harness/bin/codea-dcep-tools.exe report review --input {request_root}/report-review.json; "
                f"if ($LASTEXITCODE -ne 0) {{ exit $LASTEXITCODE }}; Write-Output '{stage_mark(23)}'",
            ),
            (
                "Read this fresh invocation's formal Review report",
                f"Get-Content -Raw .code-harness/runs/{run_id}/review.md; Write-Output '{stage_mark(24)}'",
            ),
        ]
    )
    return stages


def has_failed_tool_result(messages: list[dict[str, Any]], invocation: int, expected_previous: int) -> bool:
    if expected_previous < 0 or not messages:
        return False
    last = messages[-1]
    if last.get("role") not in {"tool", "user"}:
        return False
    text = flatten(last.get("content", ""))
    looks_like_result = last.get("role") == "tool" or "tool_result" in text or "exit code" in text.lower()
    return looks_like_result and marker(invocation, expected_previous) not in text


class Handler(BaseHTTPRequestHandler):
    server_version = "Task2FreshReviewLifecycleModel/1.0"

    def log_message(self, fmt: str, *args: object) -> None:
        return

    @property
    def log_path(self) -> Path:
        return self.server.log_path  # type: ignore[attr-defined]

    @property
    def seen_runs(self) -> dict[int, str]:
        return self.server.seen_runs  # type: ignore[attr-defined]

    def append_log(self, item: dict[str, Any]) -> None:
        with self.log_path.open("a", encoding="utf-8") as fh:
            fh.write(json.dumps(item, ensure_ascii=False) + "\n")

    def send_json(self, status: int, obj: Any) -> None:
        data = json.dumps(obj, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self) -> None:  # noqa: N802
        if self.path == "/health":
            self.send_json(200, {"status": "ok"})
            return
        if self.path.rstrip("/") == "/v1/models":
            self.send_json(200, {"object": "list", "data": [{"id": "task2", "object": "model", "owned_by": "task2"}]})
            return
        self.send_json(404, {"error": "not found"})

    def do_POST(self) -> None:  # noqa: N802
        if self.path.rstrip("/") != "/v1/chat/completions":
            self.send_json(404, {"error": "not found"})
            return
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length)
        try:
            body = json.loads(raw)
        except json.JSONDecodeError as exc:
            self.send_json(400, {"error": str(exc)})
            return

        messages = body.get("messages") or []
        tools = body.get("tools") or []
        text = flatten(messages)
        invocation = review_user_count(messages)
        run_id = current_run_id(messages)
        stages = stages_for(invocation, run_id)
        stage = next_stage(messages, invocation, len(stages))

        self.append_log({
            "event": "request",
            "invocation": invocation,
            "reviewUserCount": invocation,
            "stage": stage,
            "runId": run_id,
            "hasTools": bool(tools),
            "toolNames": [str(t.get("function", {}).get("name", "")) for t in tools],
        })

        if invocation >= 1 and run_id and invocation not in self.seen_runs:
            self.seen_runs[invocation] = run_id
            self.append_log({"event": "invocation_run", "invocation": invocation, "runId": run_id})

        if not tools or "harness review" not in text:
            self.respond_text(body, "Task 2 Fresh Review Lifecycle E2E")
            return

        if stage > 0 and has_failed_tool_result(messages, invocation, stage - 1):
            self.respond_text(body, f"TASK2_E2E_ABORT invocation {invocation} stage {stage - 1:02d} tool failed")
            return

        if stage >= len(stages):
            final_run = self.seen_runs.get(invocation) or run_id or "unknown-run"
            self.respond_text(body, f"评审完成。正式报告：.code-harness/runs/{final_run}/review.md")
            return

        if stage >= 2 and run_id is None:
            self.respond_text(body, f"TASK2_E2E_ABORT invocation {invocation} Runtime review begin did not expose a runId")
            return

        description, command = stages[stage]
        tool_names = [str(t.get("function", {}).get("name", "")) for t in tools]
        if "bash" not in tool_names:
            self.respond_text(body, "TASK2_E2E_TOOL_NAMES " + ",".join(tool_names))
            return
        self.append_log({"event": "tool_call", "invocation": invocation, "stage": stage, "runId": run_id, "tool": "bash", "description": description, "command": command})
        self.respond_tool(body, "bash", {"command": command, "description": description})

    def completion_base(self, body: dict[str, Any]) -> dict[str, Any]:
        return {
            "id": "chatcmpl-" + uuid.uuid4().hex,
            "object": "chat.completion",
            "created": int(time.time()),
            "model": body.get("model", "task2"),
        }

    def respond_text(self, body: dict[str, Any], text: str) -> None:
        base = self.completion_base(body)
        if body.get("stream"):
            chunks = [
                {**base, "object": "chat.completion.chunk", "choices": [{"index": 0, "delta": {"role": "assistant", "content": text}, "finish_reason": None}]},
                {**base, "object": "chat.completion.chunk", "choices": [{"index": 0, "delta": {}, "finish_reason": "stop"}]},
            ]
            self.send_sse(chunks)
            return
        self.send_json(200, {**base, "choices": [{"index": 0, "message": {"role": "assistant", "content": text}, "finish_reason": "stop"}]})

    def respond_tool(self, body: dict[str, Any], name: str, arguments: dict[str, Any]) -> None:
        base = self.completion_base(body)
        call_id = "call_" + uuid.uuid4().hex
        call = {"id": call_id, "type": "function", "function": {"name": name, "arguments": json.dumps(arguments, ensure_ascii=False)}}
        if body.get("stream"):
            delta_call = {"index": 0, **call}
            chunks = [
                {**base, "object": "chat.completion.chunk", "choices": [{"index": 0, "delta": {"role": "assistant", "tool_calls": [delta_call]}, "finish_reason": None}]},
                {**base, "object": "chat.completion.chunk", "choices": [{"index": 0, "delta": {}, "finish_reason": "tool_calls"}]},
            ]
            self.send_sse(chunks)
            return
        self.send_json(200, {**base, "choices": [{"index": 0, "message": {"role": "assistant", "content": None, "tool_calls": [call]}, "finish_reason": "tool_calls"}]})

    def send_sse(self, chunks: list[dict[str, Any]]) -> None:
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-cache")
        self.send_header("Connection", "close")
        self.end_headers()
        for chunk in chunks:
            self.wfile.write(("data: " + json.dumps(chunk, ensure_ascii=False) + "\n\n").encode("utf-8"))
            self.wfile.flush()
        self.wfile.write(b"data: [DONE]\n\n")
        self.wfile.flush()


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, required=True)
    parser.add_argument("--log", type=Path, required=True)
    parser.add_argument("--scenario", choices=["working-tree", "head"], required=True)
    args = parser.parse_args()
    args.log.parent.mkdir(parents=True, exist_ok=True)
    args.log.write_text("", encoding="utf-8")
    server = ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
    server.log_path = args.log  # type: ignore[attr-defined]
    server.seen_runs = {}  # type: ignore[attr-defined]
    server.scenario = args.scenario  # type: ignore[attr-defined]
    server.serve_forever()


if __name__ == "__main__":
    main()
