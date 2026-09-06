#!/usr/bin/env python3
"""Deterministic OpenAI-compatible model for Task 1 Review Reliability E2E.

The model never touches the fixture directly. It only returns ordinary OpenCode tool
calls. OpenCode executes all reads/writes/Runtime commands, so the test crosses the
real Agent Host and Controlled Runtime boundaries.
"""

from __future__ import annotations

import argparse
import json
import time
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any


def ps_json_write(path: str, expression: str) -> str:
    return (
        f"$value = {expression}; "
        f"$json = $value | ConvertTo-Json -Depth 30 -Compress; "
        f"[System.IO.File]::WriteAllText('{path}', $json, [System.Text.UTF8Encoding]::new($false))"
    )


def build_stages(scenario: str) -> tuple[str, list[tuple[str, str]]]:
    if scenario not in {"changed", "zero"}:
        raise ValueError(f"unsupported scenario {scenario}")
    run_id = f"task1-{scenario}-review"
    request_root = f".code-harness/runs/{run_id}/requests"
    analysis_root = f".code-harness/runs/{run_id}/analysis"

    if scenario == "changed":
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

    stages: list[tuple[str, str]] = [
        (
            "Read active Review contracts and discover the complete Runtime command set",
            """$paths = @(
'.code-harness/AGENTS.md',
'.code-harness/tools/README.md',
'.code-harness/agents/orchestrator.md',
'.code-harness/agents/reviewer.md',
'.code-harness/contracts/change-set-request.schema.json',
'.code-harness/contracts/change-analysis-proposal.schema.json',
'.code-harness/contracts/analysis-certify-request.schema.json',
'.code-harness/contracts/review-options-request.schema.json',
'.code-harness/contracts/review-selection-request.schema.json',
'.code-harness/contracts/finding-proposals.schema.json',
'.code-harness/contracts/finding-certify-request.schema.json',
'.code-harness/contracts/report-review-request.schema.json'
); foreach ($p in $paths) { Write-Output ('TASK1_ACTIVE_READ ' + $p); Get-Content -Raw $p }; Write-Output 'TASK1_STAGE_00 PASS'""",
        ),
        (
            "Create canonical Snapshot request from active contract",
            f"$run='{run_id}'; New-Item -ItemType Directory -Force '{request_root}' | Out-Null; "
            + ps_json_write(
                f"{request_root}/change-set-request.json",
                "[ordered]@{runId=$run;baseRef='HEAD';includeWorkingTree=$true}",
            )
            + "; Write-Output 'TASK1_STAGE_01 PASS'",
        ),
        (
            "Invoke Runtime Canonical Snapshot",
            f"& ./.code-harness/bin/codea-dcep-tools.exe analysis snapshot --input {request_root}/change-set-request.json; if ($LASTEXITCODE -ne 0) {{ exit $LASTEXITCODE }}; Write-Output 'TASK1_STAGE_02 PASS'",
        ),
        (
            "Read Runtime Snapshot and review-relevant source when present",
            f"Get-Content -Raw {analysis_root}/change-set.json; if (Test-Path 'src/main/resources/application.yml') {{ Get-Content -Raw 'src/main/resources/application.yml' }}; Write-Output 'TASK1_STAGE_03 PASS'",
        ),
        (
            "Create semantic ChangeAnalysis proposal",
            ps_json_write(f"{request_root}/change-analysis-proposal.json", semantic)
            + "; Write-Output 'TASK1_STAGE_04 PASS'",
        ),
        (
            "Create canonical ChangeAnalysis certification request",
            f"$run='{run_id}'; $s=Get-Content -Raw {analysis_root}/change-set.json | ConvertFrom-Json; "
            + ps_json_write(
                f"{request_root}/analysis-certify-request.json",
                f"[ordered]@{{runId=$run;snapshotPath='{analysis_root}/change-set.json';snapshotSha256=[string]$s.snapshotSha256;proposalPath='{request_root}/change-analysis-proposal.json';intent=[ordered]@{{mode='FULL'}}}}",
            )
            + "; Write-Output 'TASK1_STAGE_05 PASS'",
        ),
        (
            "Invoke Runtime Certified ChangeAnalysis",
            f"& ./.code-harness/bin/codea-dcep-tools.exe analysis certify --input {request_root}/analysis-certify-request.json; if ($LASTEXITCODE -ne 0) {{ exit $LASTEXITCODE }}; Write-Output 'TASK1_STAGE_06 PASS'",
        ),
        (
            "Read Certified ChangeAnalysis authority",
            f"Get-Content -Raw {analysis_root}/change-analysis.json; Get-Content -Raw {analysis_root}/entrypoint-inventory.json; Get-Content -Raw {analysis_root}/change-analysis.cert.json; Write-Output 'TASK1_STAGE_07 PASS'",
        ),
        (
            "Create Review Options request",
            ps_json_write(
                f"{request_root}/review-options-request.json",
                f"[ordered]@{{runId='{run_id}';changeAnalysisPath='{analysis_root}/change-analysis.json'}}",
            )
            + "; Write-Output 'TASK1_STAGE_08 PASS'",
        ),
        (
            "Invoke Runtime Review Options",
            f"& ./.code-harness/bin/codea-dcep-tools.exe review options --input {request_root}/review-options-request.json; if ($LASTEXITCODE -ne 0) {{ exit $LASTEXITCODE }}; Write-Output 'TASK1_STAGE_09 PASS'",
        ),
        (
            "Read Runtime Review Options",
            f"Get-Content -Raw {analysis_root}/review-options.json; Write-Output 'TASK1_STAGE_10 PASS'",
        ),
        (
            "Create AUTO_FULL selection bound to optionsHash",
            f"$run='{run_id}'; $o=Get-Content -Raw {analysis_root}/review-options.json | ConvertFrom-Json; "
            + ps_json_write(
                f"{request_root}/review-selection-request.json",
                "[ordered]@{runId=$run;optionsHash=[string]$o.optionsHash;mode='FULL';selectionIds=@()}",
            )
            + "; Write-Output 'TASK1_STAGE_11 PASS'",
        ),
        (
            "Invoke Runtime Review selection",
            f"& ./.code-harness/bin/codea-dcep-tools.exe review select --input {request_root}/review-selection-request.json; if ($LASTEXITCODE -ne 0) {{ exit $LASTEXITCODE }}; Write-Output 'TASK1_STAGE_12 PASS'",
        ),
        (
            "Read Runtime verified Review Scope",
            f"Get-Content -Raw {analysis_root}/review-scope.json; Write-Output 'TASK1_STAGE_13 PASS'",
        ),
        (
            "Build Runtime Review Units",
            f"& ./.code-harness/bin/codea-dcep-tools.exe review units --run-id {run_id}; if ($LASTEXITCODE -ne 0) {{ exit $LASTEXITCODE }}; Write-Output 'TASK1_STAGE_14 PASS'",
        ),
        (
            "Build Runtime Rule Dispatch",
            f"& ./.code-harness/bin/codea-dcep-tools.exe review dispatch --run-id {run_id}; if ($LASTEXITCODE -ne 0) {{ exit $LASTEXITCODE }}; Write-Output 'TASK1_STAGE_15 PASS'",
        ),
        (
            "Re-read Finding Certification request schema immediately before request creation",
            "Get-Content -Raw .code-harness/contracts/finding-certify-request.schema.json; Write-Output 'TASK1_FINDING_SCHEMA_READ'; Write-Output 'TASK1_STAGE_16 PASS'",
        ),
        (
            "Create empty Finding proposals and schema-bound Finding Certification request",
            f"Get-Content -Raw {analysis_root}/review-units.json; Get-Content -Raw {analysis_root}/rule-dispatch.json; "
            f"[System.IO.File]::WriteAllText('{request_root}/finding-proposals.json','[]',[System.Text.UTF8Encoding]::new($false)); "
            + ps_json_write(
                f"{request_root}/finding-certify-request.json",
                f"[ordered]@{{runId='{run_id}';proposalsPath='{request_root}/finding-proposals.json'}}",
            )
            + "; Write-Output 'TASK1_FINDING_REQUEST_WRITTEN'; Write-Output 'TASK1_STAGE_17 PASS'",
        ),
        (
            "Invoke Runtime Finding Certification",
            f"& ./.code-harness/bin/codea-dcep-tools.exe review certify-findings --input {request_root}/finding-certify-request.json; if ($LASTEXITCODE -ne 0) {{ exit $LASTEXITCODE }}; Write-Output 'TASK1_STAGE_18 PASS'",
        ),
        (
            "Read same-run Runtime Certified Findings",
            f"Get-Content -Raw {analysis_root}/certified-findings.json; Get-Content -Raw {analysis_root}/certified-findings.cert.json; Write-Output 'TASK1_STAGE_19 PASS'",
        ),
        (
            "Re-read Report Review request schema immediately before request creation",
            "Get-Content -Raw .code-harness/contracts/report-review-request.schema.json; Write-Output 'TASK1_REPORT_SCHEMA_READ'; Write-Output 'TASK1_STAGE_20 PASS'",
        ),
        (
            "Create schema-bound formal Report Review request with no raw Agent findings",
            f"$run='{run_id}'; $a=Get-Content -Raw {analysis_root}/change-analysis.json | ConvertFrom-Json; $paths=@($a.changedFiles | ForEach-Object {{[string]$_.path}}); "
            + ps_json_write(
                f"{request_root}/report-review.json",
                "[ordered]@{runId=$run;harnessVersion='runtime-owned';baseRef=[string]$a.reviewScope.baseRef;head=[string]$a.reviewScope.headCommit;result='PASSED';mode='FULL';reviewScope=[ordered]@{changedFiles=$paths};reviewCoverage=[ordered]@{reviewedFiles=$paths;callChains=@();externalDependencies=@();unresolved=@();missingReviewedFiles=@();runtimeErrors=@();status='COMPLETE'};findings=@()}",
            )
            + "; Write-Output 'TASK1_REPORT_REQUEST_WRITTEN'; Write-Output 'TASK1_STAGE_21 PASS'",
        ),
        (
            "Invoke Runtime deterministic Review renderer",
            f"& ./.code-harness/bin/codea-dcep-tools.exe report review --input {request_root}/report-review.json; if ($LASTEXITCODE -ne 0) {{ exit $LASTEXITCODE }}; Write-Output 'TASK1_STAGE_22 PASS'",
        ),
        (
            "Read final formal Review artifact",
            f"Get-Content -Raw .code-harness/runs/{run_id}/review.md; Write-Output 'TASK1_STAGE_23 PASS'",
        ),
    ]
    return run_id, stages


def flatten(value: Any) -> str:
    if isinstance(value, str):
        return value
    if isinstance(value, list):
        return "\n".join(flatten(v) for v in value)
    if isinstance(value, dict):
        return "\n".join(f"{k}:{flatten(v)}" for k, v in value.items())
    return str(value)


def next_stage(messages: list[dict[str, Any]], stages: list[tuple[str, str]]) -> int:
    text = flatten(messages)
    highest = -1
    for i in range(len(stages)):
        if f"TASK1_STAGE_{i:02d} PASS" in text:
            highest = i
    return highest + 1


def has_failed_tool_result(messages: list[dict[str, Any]], expected_previous: int) -> bool:
    if expected_previous < 0 or not messages:
        return False
    last = messages[-1]
    if last.get("role") not in {"tool", "user"}:
        return False
    text = flatten(last.get("content", ""))
    looks_like_result = last.get("role") == "tool" or "tool_result" in text or "exit code" in text.lower()
    return looks_like_result and f"TASK1_STAGE_{expected_previous:02d} PASS" not in text


class Handler(BaseHTTPRequestHandler):
    server_version = "Task1ReviewReliabilityModel/1.0"

    def log_message(self, fmt: str, *args: object) -> None:
        return

    @property
    def log_path(self) -> Path:
        return self.server.log_path  # type: ignore[attr-defined]

    @property
    def stages(self) -> list[tuple[str, str]]:
        return self.server.stages  # type: ignore[attr-defined]

    @property
    def run_id(self) -> str:
        return self.server.run_id  # type: ignore[attr-defined]

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
            self.send_json(200, {"object": "list", "data": [{"id": "task1", "object": "model", "owned_by": "task1"}]})
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
        stage = next_stage(messages, self.stages)
        self.append_log({"event": "request", "stage": stage, "hasTools": bool(tools), "toolNames": [str(t.get("function", {}).get("name", "")) for t in tools], "messages": messages})

        if not tools or "harness review" not in text:
            self.respond_text(body, "Task 1 Harness Review Reliability E2E")
            return

        if stage > 0 and has_failed_tool_result(messages, stage - 1):
            self.respond_text(body, f"TASK1_E2E_ABORT stage {stage - 1:02d} tool failed")
            return

        if stage >= len(self.stages):
            self.respond_text(body, f"评审完成。正式报告：.code-harness/runs/{self.run_id}/review.md")
            return

        description, command = self.stages[stage]
        tool_names = [str(t.get("function", {}).get("name", "")) for t in tools]
        if "bash" not in tool_names:
            self.respond_text(body, "TASK1_E2E_TOOL_NAMES " + ",".join(tool_names))
            return
        self.append_log({"event": "tool_call", "stage": stage, "tool": "bash", "description": description, "command": command})
        self.respond_tool(body, "bash", {"command": command, "description": description})

    def completion_base(self, body: dict[str, Any]) -> dict[str, Any]:
        return {
            "id": "chatcmpl-" + uuid.uuid4().hex,
            "object": "chat.completion",
            "created": int(time.time()),
            "model": body.get("model", "task1"),
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
    parser.add_argument("--scenario", choices=["changed", "zero"], required=True)
    args = parser.parse_args()
    run_id, stages = build_stages(args.scenario)
    args.log.parent.mkdir(parents=True, exist_ok=True)
    args.log.write_text("", encoding="utf-8")
    server = ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
    server.log_path = args.log  # type: ignore[attr-defined]
    server.run_id = run_id  # type: ignore[attr-defined]
    server.stages = stages  # type: ignore[attr-defined]
    server.serve_forever()


if __name__ == "__main__":
    main()
