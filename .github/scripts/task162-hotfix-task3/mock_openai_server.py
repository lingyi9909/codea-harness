#!/usr/bin/env python3
"""Deterministic OpenAI-compatible model used only by the Task 3 Agent-host E2E.

The server never touches the fixture itself. It only returns ordinary model tool calls.
OpenCode executes every read/write/bash action, so the E2E still crosses the real Agent
Host and the real Runtime boundary.
"""

from __future__ import annotations

import argparse
import json
import time
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any

RUN_ID = "task3-plain-review"


def ps_json_write(path: str, expression: str) -> str:
    return (
        f"$value = {expression}; "
        f"$json = $value | ConvertTo-Json -Depth 30 -Compress; "
        f"[System.IO.File]::WriteAllText('{path}', $json, [System.Text.UTF8Encoding]::new($false))"
    )


STAGES: list[tuple[str, str]] = [
    (
        "Read active Harness contracts before creating formal requests",
        """$paths = @(
'.code-harness/AGENTS.md',
'.code-harness/agents/orchestrator.md',
'.code-harness/agents/reviewer.md',
'.code-harness/skills/analyze-change/SKILL.md',
'.code-harness/skills/review-code/SKILL.md',
'.code-harness/contracts/change-set-request.schema.json',
'.code-harness/contracts/change-analysis-proposal.schema.json',
'.code-harness/contracts/analysis-certify-request.schema.json',
'.code-harness/contracts/review-options-request.schema.json',
'.code-harness/contracts/review-selection-request.schema.json',
'.code-harness/contracts/finding-proposals.schema.json'
); foreach ($p in $paths) { Write-Output ('TASK3_READ ' + $p); Get-Content -Raw $p }; Write-Output 'TASK3_STAGE_00 PASS'""",
    ),
    (
        "Create the Agent-owned canonical Snapshot request",
        "$run='task3-plain-review'; New-Item -ItemType Directory -Force \".code-harness/runs/$run/requests\" | Out-Null; "
        + ps_json_write(
            ".code-harness/runs/task3-plain-review/requests/change-set-request.json",
            "[ordered]@{runId=$run; baseRef='HEAD'; includeWorkingTree=$true}",
        )
        + "; Write-Output 'TASK3_STAGE_01 PASS'",
    ),
    (
        "Invoke the real Runtime Canonical Snapshot command",
        "& ./.code-harness/bin/codea-dcep-tools.exe analysis snapshot --input .code-harness/runs/task3-plain-review/requests/change-set-request.json; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; Write-Output 'TASK3_STAGE_02 PASS'",
    ),
    (
        "Read Runtime Snapshot and changed source before semantic analysis",
        "Get-Content -Raw .code-harness/runs/task3-plain-review/analysis/change-set.json; Get-Content -Raw src/main/resources/application.yml; Write-Output 'TASK3_STAGE_03 PASS'",
    ),
    (
        "Create semantic ChangeAnalysis proposal from the changed YML",
        ps_json_write(
            ".code-harness/runs/task3-plain-review/requests/change-analysis-proposal.json",
            "[ordered]@{changedFileRoles=@([ordered]@{path='src/main/resources/application.yml';role='YamlConfig'});affectedControllers=@();callChains=@();symbolLocations=@();resourceRelations=@();externalDependencies=@();riskAreas=@();reviewCoverage=[ordered]@{status='COMPLETE';reviewedFiles=@([ordered]@{path='src/main/resources/application.yml';role='YamlConfig';reason='CHANGED'});unresolvedSymbols=@()}}",
        )
        + "; Write-Output 'TASK3_STAGE_04 PASS'",
    ),
    (
        "Create canonical certify request bound to the Runtime Snapshot hash",
        "$run='task3-plain-review'; $s=Get-Content -Raw .code-harness/runs/$run/analysis/change-set.json | ConvertFrom-Json; "
        + ps_json_write(
            ".code-harness/runs/task3-plain-review/requests/analysis-certify-request.json",
            "[ordered]@{runId=$run;snapshotPath='.code-harness/runs/task3-plain-review/analysis/change-set.json';snapshotSha256=[string]$s.snapshotSha256;proposalPath='.code-harness/runs/task3-plain-review/requests/change-analysis-proposal.json';intent=[ordered]@{mode='FULL'}}",
        )
        + "; Write-Output 'TASK3_STAGE_05 PASS'",
    ),
    (
        "Invoke real Runtime canonical ChangeAnalysis certification",
        "& ./.code-harness/bin/codea-dcep-tools.exe analysis certify --input .code-harness/runs/task3-plain-review/requests/analysis-certify-request.json; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; Write-Output 'TASK3_STAGE_06 PASS'",
    ),
    (
        "Read certified analysis, inventory and certificate",
        "Get-Content -Raw .code-harness/runs/task3-plain-review/analysis/change-analysis.json; Get-Content -Raw .code-harness/runs/task3-plain-review/analysis/entrypoint-inventory.json; Get-Content -Raw .code-harness/runs/task3-plain-review/analysis/change-analysis.cert.json; Write-Output 'TASK3_STAGE_07 PASS'",
    ),
    (
        "Create Review Options request without a baseRef",
        ps_json_write(
            ".code-harness/runs/task3-plain-review/requests/review-options-request.json",
            "[ordered]@{runId='task3-plain-review';changeAnalysisPath='.code-harness/runs/task3-plain-review/analysis/change-analysis.json'}",
        )
        + "; Write-Output 'TASK3_STAGE_08 PASS'",
    ),
    (
        "Invoke real Runtime Review Options",
        "& ./.code-harness/bin/codea-dcep-tools.exe review options --input .code-harness/runs/task3-plain-review/requests/review-options-request.json; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; Write-Output 'TASK3_STAGE_09 PASS'",
    ),
    (
        "Read Runtime-owned Review Options before selection",
        "Get-Content -Raw .code-harness/runs/task3-plain-review/analysis/review-options.json; Write-Output 'TASK3_STAGE_10 PASS'",
    ),
    (
        "Create AUTO_FULL selection bound to the current Runtime optionsHash",
        "$run='task3-plain-review'; $o=Get-Content -Raw .code-harness/runs/$run/analysis/review-options.json | ConvertFrom-Json; "
        + ps_json_write(
            ".code-harness/runs/task3-plain-review/requests/review-selection-request.json",
            "[ordered]@{schemaVersion='1.5';runId=$run;optionsHash=[string]$o.optionsHash;mode='FULL';selectionIds=@()}",
        )
        + "; Write-Output 'TASK3_STAGE_11 PASS'",
    ),
    (
        "Invoke real Runtime Review selection",
        "& ./.code-harness/bin/codea-dcep-tools.exe review select --input .code-harness/runs/task3-plain-review/requests/review-selection-request.json; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; Write-Output 'TASK3_STAGE_12 PASS'",
    ),
    (
        "Read Runtime-verified FULL review scope",
        "Get-Content -Raw .code-harness/runs/task3-plain-review/analysis/review-scope.json; Write-Output 'TASK3_STAGE_13 PASS'",
    ),
    (
        "Build Runtime ReviewUnits",
        "& ./.code-harness/bin/codea-dcep-tools.exe review units --run-id task3-plain-review; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; Write-Output 'TASK3_STAGE_14 PASS'",
    ),
    (
        "Build Runtime Rule Dispatch",
        "& ./.code-harness/bin/codea-dcep-tools.exe review dispatch --run-id task3-plain-review; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; Write-Output 'TASK3_STAGE_15 PASS'",
    ),
    (
        "Read review units/dispatch then create the benign empty Finding proposal",
        "Get-Content -Raw .code-harness/runs/task3-plain-review/analysis/review-units.json; Get-Content -Raw .code-harness/runs/task3-plain-review/analysis/rule-dispatch.json; "
        "[System.IO.File]::WriteAllText('.code-harness/runs/task3-plain-review/requests/finding-proposals.json','[]',[System.Text.UTF8Encoding]::new($false)); "
        + ps_json_write(
            ".code-harness/runs/task3-plain-review/requests/finding-certify-request.json",
            "[ordered]@{runId='task3-plain-review';proposalsPath='.code-harness/runs/task3-plain-review/requests/finding-proposals.json'}",
        )
        + "; Write-Output 'TASK3_STAGE_16 PASS'",
    ),
    (
        "Invoke Runtime Finding certification",
        "& ./.code-harness/bin/codea-dcep-tools.exe review certify-findings --input .code-harness/runs/task3-plain-review/requests/finding-certify-request.json; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; Write-Output 'TASK3_STAGE_17 PASS'",
    ),
    (
        "Read Runtime Certified Findings",
        "Get-Content -Raw .code-harness/runs/task3-plain-review/analysis/certified-findings.json; Get-Content -Raw .code-harness/runs/task3-plain-review/analysis/certified-findings.cert.json; Write-Output 'TASK3_STAGE_18 PASS'",
    ),
    (
        "Create final report transport from same-run certified authority",
        "$run='task3-plain-review'; $a=Get-Content -Raw .code-harness/runs/$run/analysis/change-analysis.json | ConvertFrom-Json; $paths=@($a.changedFiles | ForEach-Object {[string]$_.path}); "
        + ps_json_write(
            ".code-harness/runs/task3-plain-review/requests/review-report.json",
            "[ordered]@{runId=$run;harnessVersion='runtime-owned';baseRef=[string]$a.reviewScope.baseRef;head=[string]$a.reviewScope.headCommit;result='PASSED';mode='FULL';reviewScope=[ordered]@{changedFiles=$paths};reviewCoverage=[ordered]@{reviewedFiles=$paths;callChains=@();externalDependencies=@();unresolved=@();missingReviewedFiles=@();runtimeErrors=@();status='COMPLETE'};findings=@()}",
        )
        + "; Write-Output 'TASK3_STAGE_19 PASS'",
    ),
    (
        "Invoke real Runtime deterministic review renderer",
        "& ./.code-harness/bin/codea-dcep-tools.exe report review --input .code-harness/runs/task3-plain-review/requests/review-report.json; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; Write-Output 'TASK3_STAGE_20 PASS'",
    ),
    (
        "Read the final formal review artifact",
        "Get-Content -Raw .code-harness/runs/task3-plain-review/review.md; Write-Output 'TASK3_STAGE_21 PASS'",
    ),
]


def flatten(value: Any) -> str:
    if isinstance(value, str):
        return value
    if isinstance(value, list):
        return "\n".join(flatten(v) for v in value)
    if isinstance(value, dict):
        return "\n".join(f"{k}:{flatten(v)}" for k, v in value.items())
    return str(value)


def next_stage(messages: list[dict[str, Any]]) -> int:
    text = flatten(messages)
    highest = -1
    for i in range(len(STAGES)):
        if f"TASK3_STAGE_{i:02d} PASS" in text:
            highest = i
    return highest + 1


def has_failed_tool_result(messages: list[dict[str, Any]], expected_previous: int) -> bool:
    if expected_previous < 0 or not messages:
        return False
    last = messages[-1]
    if last.get("role") not in {"tool", "user"}:
        return False
    text = flatten(last.get("content", ""))
    # OpenCode versions differ in whether tool output is encoded as role=tool or a
    # structured user block. Only abort when a tool/error-shaped result is present.
    looks_like_result = last.get("role") == "tool" or "tool_result" in text or "exit code" in text.lower()
    return looks_like_result and f"TASK3_STAGE_{expected_previous:02d} PASS" not in text


class Handler(BaseHTTPRequestHandler):
    server_version = "Task3DeterministicModel/1.0"

    def log_message(self, fmt: str, *args: object) -> None:
        return

    @property
    def log_path(self) -> Path:
        return self.server.log_path  # type: ignore[attr-defined]

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
            self.send_json(200, {"object": "list", "data": [{"id": "task3", "object": "model", "owned_by": "task3"}]})
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
        stage = next_stage(messages)
        self.append_log({"event": "request", "stage": stage, "hasTools": bool(tools), "messages": messages})

        # Hidden title/summary/model-maintenance calls must not advance the E2E state.
        if not tools or "harness review" not in text:
            self.respond_text(body, "Task 3 Harness Review E2E")
            return

        if stage > 0 and has_failed_tool_result(messages, stage - 1):
            self.respond_text(body, f"TASK3_E2E_ABORT stage {stage - 1:02d} tool failed")
            return

        if stage >= len(STAGES):
            self.respond_text(body, "评审完成。正式报告：.code-harness/runs/task3-plain-review/review.md")
            return

        description, command = STAGES[stage]
        tool_names = [str(t.get("function", {}).get("name", "")) for t in tools]
        if "bash" not in tool_names:
            self.respond_text(body, "TASK3_E2E_ABORT OpenCode bash tool unavailable")
            return
        self.append_log({"event": "tool_call", "stage": stage, "tool": "bash", "description": description, "command": command})
        self.respond_tool(body, "bash", {"command": command, "description": description})

    def completion_base(self, body: dict[str, Any]) -> dict[str, Any]:
        return {
            "id": "chatcmpl-" + uuid.uuid4().hex,
            "object": "chat.completion",
            "created": int(time.time()),
            "model": body.get("model", "task3"),
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
    args = parser.parse_args()
    args.log.parent.mkdir(parents=True, exist_ok=True)
    args.log.write_text("", encoding="utf-8")
    server = ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
    server.log_path = args.log  # type: ignore[attr-defined]
    server.serve_forever()


if __name__ == "__main__":
    main()
