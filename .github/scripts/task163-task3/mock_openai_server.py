#!/usr/bin/env python3
"""Deterministic OpenAI-compatible model for Task 3 same-session OpenCode E2E.

The server only returns ordinary model tool calls. Real OpenCode executes all file and
Runtime operations. Turn 1 must stop after USER_SELECTION; Turn 2 resumes the same
session after the user explicitly chooses FULL.
"""
from __future__ import annotations

import argparse
import json
import time
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any

RUN_ID = "task3-multi-chain-review"


def ps_json_write(path: str, expression: str) -> str:
    return (
        f"$value = {expression}; $json = $value | ConvertTo-Json -Depth 40 -Compress; "
        f"[System.IO.File]::WriteAllText('{path}', $json, [System.Text.UTF8Encoding]::new($false))"
    )


STAGES: list[tuple[str, str]] = [
    (
        "Read active Task 3 contracts",
        """$paths=@('.code-harness/AGENTS.md','.code-harness/agents/orchestrator.md','.code-harness/agents/reviewer.md','.code-harness/skills/review-code/SKILL.md','.code-harness/contracts/change-set-request.schema.json','.code-harness/contracts/change-analysis-proposal.schema.json','.code-harness/contracts/analysis-certify-request.schema.json','.code-harness/contracts/review-options-request.schema.json','.code-harness/contracts/review-selection-request.schema.json','.code-harness/contracts/finding-proposals.schema.json'); foreach($p in $paths){Write-Output ('TASK163_READ '+$p);Get-Content -Raw $p}; Write-Output 'TASK163_STAGE_00 PASS'""",
    ),
    (
        "Create canonical Snapshot request",
        "$run='task3-multi-chain-review'; New-Item -ItemType Directory -Force \".code-harness/runs/$run/requests\"|Out-Null; "
        + ps_json_write(
            ".code-harness/runs/task3-multi-chain-review/requests/change-set-request.json",
            "[ordered]@{runId=$run;baseRef='HEAD';includeWorkingTree=$true}",
        )
        + "; Write-Output 'TASK163_STAGE_01 PASS'",
    ),
    (
        "Invoke Runtime Canonical Snapshot",
        "& ./.code-harness/bin/codea-dcep-tools.exe analysis snapshot --input .code-harness/runs/task3-multi-chain-review/requests/change-set-request.json; if($LASTEXITCODE-ne 0){exit $LASTEXITCODE}; Write-Output 'TASK163_STAGE_02 PASS'",
    ),
    (
        "Read Snapshot and both business chains",
        "Get-Content -Raw .code-harness/runs/task3-multi-chain-review/analysis/change-set.json; Get-Content -Raw src/main/java/com/acme/order/OrderController.java; Get-Content -Raw src/main/java/com/acme/order/OrderService.java; Get-Content -Raw src/main/java/com/acme/payment/PaymentController.java; Get-Content -Raw src/main/java/com/acme/payment/PaymentService.java; Write-Output 'TASK163_STAGE_03 PASS'",
    ),
    (
        "Create two-chain semantic proposal",
        ps_json_write(
            ".code-harness/runs/task3-multi-chain-review/requests/change-analysis-proposal.json",
            "[ordered]@{changedFileRoles=@([ordered]@{path='src/main/java/com/acme/order/OrderController.java';role='Controller'},[ordered]@{path='src/main/java/com/acme/payment/PaymentController.java';role='Controller'});affectedControllers=@([ordered]@{controller='OrderController';endpoints=@('OrderController.approve');impactType='DIRECT_CHANGE';sourceSymbols=@('OrderController.approve')},[ordered]@{controller='PaymentController';endpoints=@('PaymentController.pay');impactType='DIRECT_CHANGE';sourceSymbols=@('PaymentController.pay')});callChains=@([ordered]@{entryPoint='OrderController.approve';chain=@('OrderController.approve','OrderService.approve')},[ordered]@{entryPoint='PaymentController.pay';chain=@('PaymentController.pay','PaymentService.pay')});symbolLocations=@([ordered]@{symbol='OrderController.approve';path='src/main/java/com/acme/order/OrderController.java';role='Controller';source='FIND_SYMBOL'},[ordered]@{symbol='OrderService.approve';path='src/main/java/com/acme/order/OrderService.java';role='Service';source='FIND_SYMBOL'},[ordered]@{symbol='PaymentController.pay';path='src/main/java/com/acme/payment/PaymentController.java';role='Controller';source='FIND_SYMBOL'},[ordered]@{symbol='PaymentService.pay';path='src/main/java/com/acme/payment/PaymentService.java';role='Service';source='FIND_SYMBOL'});resourceRelations=@();externalDependencies=@();riskAreas=@();reviewCoverage=[ordered]@{status='COMPLETE';reviewedFiles=@([ordered]@{path='src/main/java/com/acme/order/OrderController.java';role='Controller';reason='CHANGED'},[ordered]@{path='src/main/java/com/acme/payment/PaymentController.java';role='Controller';reason='CHANGED'},[ordered]@{path='src/main/java/com/acme/order/OrderService.java';role='Service';reason='CALL_CHAIN'},[ordered]@{path='src/main/java/com/acme/payment/PaymentService.java';role='Service';reason='CALL_CHAIN'});unresolvedSymbols=@()}}",
        )
        + "; Write-Output 'TASK163_STAGE_04 PASS'",
    ),
    (
        "Create canonical analysis certification request",
        "$run='task3-multi-chain-review';$s=Get-Content -Raw .code-harness/runs/$run/analysis/change-set.json|ConvertFrom-Json; "
        + ps_json_write(
            ".code-harness/runs/task3-multi-chain-review/requests/analysis-certify-request.json",
            "[ordered]@{runId=$run;snapshotPath='.code-harness/runs/task3-multi-chain-review/analysis/change-set.json';snapshotSha256=[string]$s.snapshotSha256;proposalPath='.code-harness/runs/task3-multi-chain-review/requests/change-analysis-proposal.json';intent=[ordered]@{mode='FULL'}}",
        )
        + "; Write-Output 'TASK163_STAGE_05 PASS'",
    ),
    (
        "Invoke Runtime ChangeAnalysis certification",
        "& ./.code-harness/bin/codea-dcep-tools.exe analysis certify --input .code-harness/runs/task3-multi-chain-review/requests/analysis-certify-request.json; if($LASTEXITCODE-ne 0){exit $LASTEXITCODE}; Write-Output 'TASK163_STAGE_06 PASS'",
    ),
    (
        "Read certified semantic authority",
        "Get-Content -Raw .code-harness/runs/task3-multi-chain-review/analysis/change-analysis.json; Get-Content -Raw .code-harness/runs/task3-multi-chain-review/analysis/entrypoint-inventory.json; Get-Content -Raw .code-harness/runs/task3-multi-chain-review/analysis/change-analysis.cert.json; Write-Output 'TASK163_STAGE_07 PASS'",
    ),
    (
        "Create Review Options request",
        ps_json_write(
            ".code-harness/runs/task3-multi-chain-review/requests/review-options-request.json",
            "[ordered]@{runId='task3-multi-chain-review';changeAnalysisPath='.code-harness/runs/task3-multi-chain-review/analysis/change-analysis.json'}",
        )
        + "; Write-Output 'TASK163_STAGE_08 PASS'",
    ),
    (
        "Invoke Runtime Review Options",
        "& ./.code-harness/bin/codea-dcep-tools.exe review options --input .code-harness/runs/task3-multi-chain-review/requests/review-options-request.json; if($LASTEXITCODE-ne 0){exit $LASTEXITCODE}; Write-Output 'TASK163_STAGE_09 PASS'",
    ),
    (
        "Verify Runtime returned USER_SELECTION",
        "$o=Get-Content -Raw .code-harness/runs/task3-multi-chain-review/analysis/review-options.json|ConvertFrom-Json; if([string]$o.decision-ne'USER_SELECTION'){throw ('expected USER_SELECTION got '+$o.decision)}; if(@($o.options).Count-lt 2){throw 'expected 2+ review options'}; Get-Content -Raw .code-harness/runs/task3-multi-chain-review/analysis/review-options.json; Write-Output 'TASK163_STAGE_10 PASS USER_SELECTION'",
    ),
    (
        "Create FULL selection only after explicit second-turn user choice",
        "$run='task3-multi-chain-review';$o=Get-Content -Raw .code-harness/runs/$run/analysis/review-options.json|ConvertFrom-Json; "
        + ps_json_write(
            ".code-harness/runs/task3-multi-chain-review/requests/review-selection-request.json",
            "[ordered]@{runId=$run;optionsHash=[string]$o.optionsHash;mode='FULL';selectionIds=@()}",
        )
        + "; Write-Output 'TASK163_STAGE_11 PASS USER_SELECTED_FULL'",
    ),
    (
        "Invoke Runtime Review selection",
        "& ./.code-harness/bin/codea-dcep-tools.exe review select --input .code-harness/runs/task3-multi-chain-review/requests/review-selection-request.json; if($LASTEXITCODE-ne 0){exit $LASTEXITCODE}; Write-Output 'TASK163_STAGE_12 PASS'",
    ),
    (
        "Read Runtime verified FULL scope",
        "$s=Get-Content -Raw .code-harness/runs/task3-multi-chain-review/analysis/review-scope.json|ConvertFrom-Json;if([string]$s.mode-ne'FULL'){throw 'scope not FULL'};Get-Content -Raw .code-harness/runs/task3-multi-chain-review/analysis/review-scope.json;Write-Output 'TASK163_STAGE_13 PASS'",
    ),
    (
        "Build Runtime ReviewUnits",
        "& ./.code-harness/bin/codea-dcep-tools.exe review units --run-id task3-multi-chain-review;if($LASTEXITCODE-ne 0){exit $LASTEXITCODE};Write-Output 'TASK163_STAGE_14 PASS'",
    ),
    (
        "Build Runtime Rule Dispatch",
        "& ./.code-harness/bin/codea-dcep-tools.exe review dispatch --run-id task3-multi-chain-review;if($LASTEXITCODE-ne 0){exit $LASTEXITCODE};Write-Output 'TASK163_STAGE_15 PASS'",
    ),
    (
        "Create benign empty Finding Proposal and certify request",
        "Get-Content -Raw .code-harness/runs/task3-multi-chain-review/analysis/review-units.json;Get-Content -Raw .code-harness/runs/task3-multi-chain-review/analysis/rule-dispatch.json;[System.IO.File]::WriteAllText('.code-harness/runs/task3-multi-chain-review/requests/finding-proposals.json','[]',[System.Text.UTF8Encoding]::new($false));"
        + ps_json_write(
            ".code-harness/runs/task3-multi-chain-review/requests/finding-certify-request.json",
            "[ordered]@{runId='task3-multi-chain-review';proposalsPath='.code-harness/runs/task3-multi-chain-review/requests/finding-proposals.json'}",
        )
        + ";Write-Output 'TASK163_STAGE_16 PASS'",
    ),
    (
        "Invoke Runtime Finding certification",
        "& ./.code-harness/bin/codea-dcep-tools.exe review certify-findings --input .code-harness/runs/task3-multi-chain-review/requests/finding-certify-request.json;if($LASTEXITCODE-ne 0){exit $LASTEXITCODE};Write-Output 'TASK163_STAGE_17 PASS'",
    ),
    (
        "Read Certified Findings",
        "Get-Content -Raw .code-harness/runs/task3-multi-chain-review/analysis/certified-findings.json;Get-Content -Raw .code-harness/runs/task3-multi-chain-review/analysis/certified-findings.cert.json;Write-Output 'TASK163_STAGE_18 PASS'",
    ),
    (
        "Create final report transport",
        "$run='task3-multi-chain-review';$a=Get-Content -Raw .code-harness/runs/$run/analysis/change-analysis.json|ConvertFrom-Json;$paths=@($a.changedFiles|ForEach-Object{[string]$_.path});"
        + ps_json_write(
            ".code-harness/runs/task3-multi-chain-review/requests/review-report.json",
            "[ordered]@{runId=$run;harnessVersion='runtime-owned';baseRef=[string]$a.reviewScope.baseRef;head=[string]$a.reviewScope.headCommit;result='PASSED';mode='FULL';reviewScope=[ordered]@{changedFiles=$paths};reviewCoverage=[ordered]@{reviewedFiles=@($a.reviewCoverage.reviewedFiles|ForEach-Object{[string]$_.path});callChains=@($a.callChains);externalDependencies=@($a.externalDependencies);unresolved=@();missingReviewedFiles=@();runtimeErrors=@();status='COMPLETE'};findings=@()}",
        )
        + ";Write-Output 'TASK163_STAGE_19 PASS'",
    ),
    (
        "Invoke Runtime deterministic review renderer",
        "& ./.code-harness/bin/codea-dcep-tools.exe report review --input .code-harness/runs/task3-multi-chain-review/requests/review-report.json;if($LASTEXITCODE-ne 0){exit $LASTEXITCODE};Write-Output 'TASK163_STAGE_20 PASS'",
    ),
    (
        "Read final formal review artifact",
        "Get-Content -Raw .code-harness/runs/task3-multi-chain-review/review.md;Write-Output 'TASK163_STAGE_21 PASS'",
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
        if f"TASK163_STAGE_{i:02d} PASS" in text:
            highest = i
    return highest + 1


class Handler(BaseHTTPRequestHandler):
    server_version = "Task163MultiChainModel/1.0"

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
            self.send_json(200, {"object": "list", "data": [{"id": "task3", "object": "model", "owned_by": "task163"}]})
            return
        self.send_json(404, {"error": "not found"})

    def do_POST(self) -> None:  # noqa: N802
        if self.path.rstrip("/") != "/v1/chat/completions":
            self.send_json(404, {"error": "not found"})
            return
        raw = self.rfile.read(int(self.headers.get("Content-Length", "0")))
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

        if not tools or "harness review" not in text:
            self.respond_text(body, "Task 3 Multi-Chain Review E2E")
            return

        # This is the product behavior under test: after Runtime says USER_SELECTION,
        # Turn 1 ends with a question and absolutely no selection/units/report tool call.
        if stage == 11 and "TURN1_SELECTION_REQUIRED" not in text:
            self.append_log({"event": "turn1_hard_stop", "stage": 11})
            self.respond_text(body, "TURN1_SELECTION_REQUIRED\n检测到多个业务链，请明确选择：\n1) 全部评审\n2) 按业务链评审（可多选 C1..Cn）\n3) 仅查看调用链")
            return

        if stage == 11:
            same_session = "TURN1_SELECTION_REQUIRED" in text and "harness review" in text and "全部" in text
            self.append_log({"event": "turn2_resume", "stage": 11, "sameSession": same_session})
            if not same_session:
                self.respond_text(body, "TASK163_E2E_ABORT same-session user selection evidence missing")
                return

        if stage >= len(STAGES):
            self.respond_text(body, "评审完成。正式报告：.code-harness/runs/task3-multi-chain-review/review.md")
            return

        description, command = STAGES[stage]
        tool_names = [str(t.get("function", {}).get("name", "")) for t in tools]
        if "bash" not in tool_names:
            self.respond_text(body, "TASK163_E2E_TOOL_NAMES " + ",".join(tool_names))
            return
        self.append_log({"event": "tool_call", "stage": stage, "tool": "bash", "description": description, "command": command})
        self.respond_tool(body, "bash", {"command": command, "description": description})

    def completion_base(self, body: dict[str, Any]) -> dict[str, Any]:
        return {"id": "chatcmpl-" + uuid.uuid4().hex, "object": "chat.completion", "created": int(time.time()), "model": body.get("model", "task3")}

    def respond_text(self, body: dict[str, Any], text: str) -> None:
        base = self.completion_base(body)
        if body.get("stream"):
            self.send_sse([
                {**base, "object": "chat.completion.chunk", "choices": [{"index": 0, "delta": {"role": "assistant", "content": text}, "finish_reason": None}]},
                {**base, "object": "chat.completion.chunk", "choices": [{"index": 0, "delta": {}, "finish_reason": "stop"}]},
            ])
            return
        self.send_json(200, {**base, "choices": [{"index": 0, "message": {"role": "assistant", "content": text}, "finish_reason": "stop"}]})

    def respond_tool(self, body: dict[str, Any], name: str, arguments: dict[str, Any]) -> None:
        base = self.completion_base(body)
        call = {"id": "call_" + uuid.uuid4().hex, "type": "function", "function": {"name": name, "arguments": json.dumps(arguments, ensure_ascii=False)}}
        if body.get("stream"):
            self.send_sse([
                {**base, "object": "chat.completion.chunk", "choices": [{"index": 0, "delta": {"role": "assistant", "tool_calls": [{"index": 0, **call}]}, "finish_reason": None}]},
                {**base, "object": "chat.completion.chunk", "choices": [{"index": 0, "delta": {}, "finish_reason": "tool_calls"}]},
            ])
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
