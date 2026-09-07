#!/usr/bin/env python3
"""Deterministic OpenAI-compatible model for the Task 3 Agent-host E2E.

The server only returns ordinary model tool calls. Real OpenCode executes every file and
Runtime action. The model does not own a Task 3 run id and does not own a stage-number
USER_SELECTION wait state: it reads the Runtime-created review id from conversation
output and obeys the active Agent contract when that contract requires a next-user-turn
boundary.
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

RUN_ID_RE = re.compile(r"\breview-[0-9a-f]{32}\b")
CHAIN_OPTION_RE = re.compile(
    r"TASK163_RUNTIME_CHAIN_OPTION\|(C[1-9][0-9]*)\|([^|\r\n]+)\|([^\r\n]+)"
)
HARD_STOP_CONTRACT_MARKER = "TASK163_USER_SELECTION_TURN_HARD_STOP"
USER_SELECTION_MARKER = "TASK163_STAGE_11 PASS USER_SELECTION"
SELECTION_CREATED_MARKER = "TASK163_STAGE_12 PASS USER_SELECTED_FULL"
LAST_STAGE = 22


def ps_json_write(path: str, expression: str) -> str:
    return (
        f"$value = {expression}; $json = $value | ConvertTo-Json -Depth 40 -Compress; "
        f"[System.IO.File]::WriteAllText('{path}', $json, [System.Text.UTF8Encoding]::new($false))"
    )


def flatten(value: Any) -> str:
    if isinstance(value, str):
        return value
    if isinstance(value, list):
        return "\n".join(flatten(v) for v in value)
    if isinstance(value, dict):
        return "\n".join(f"{k}:{flatten(v)}" for k, v in value.items())
    return str(value)


def latest_user_text(messages: list[dict[str, Any]]) -> str:
    for message in reversed(messages):
        if message.get("role") == "user":
            return flatten(message.get("content", "")).strip()
    return ""


def runtime_run_id(text: str) -> str | None:
    matches = RUN_ID_RE.findall(text)
    return matches[-1] if matches else None


def runtime_chain_options(text: str) -> list[tuple[str, str, str]]:
    seen: set[str] = set()
    result: list[tuple[str, str, str]] = []
    for selection_id, chain_id, entry_points in CHAIN_OPTION_RE.findall(text):
        if selection_id in seen:
            continue
        seen.add(selection_id)
        result.append((selection_id, chain_id, entry_points.strip()))
    return result


def next_stage(messages: list[dict[str, Any]]) -> int:
    text = flatten(messages)
    highest = -1
    for i in range(LAST_STAGE + 1):
        if f"TASK163_STAGE_{i:02d} PASS" in text:
            highest = i
    return highest + 1


def stage_command(stage: int, run_id: str | None) -> tuple[str, str]:
    if stage == 0:
        return (
            "Read active Harness contracts",
            """$paths=@('.code-harness/AGENTS.md','.code-harness/agents/orchestrator.md','.code-harness/agents/reviewer.md','.code-harness/skills/review-code/SKILL.md','.code-harness/contracts/change-set-request.schema.json','.code-harness/contracts/change-analysis-proposal.schema.json','.code-harness/contracts/analysis-certify-request.schema.json','.code-harness/contracts/review-options-request.schema.json','.code-harness/contracts/review-selection-request.schema.json','.code-harness/contracts/finding-proposals.schema.json','.code-harness/contracts/finding-certify-request.schema.json','.code-harness/contracts/report-review-request.schema.json'); foreach($p in $paths){Write-Output ('TASK163_READ '+$p);Get-Content -Raw $p}; Write-Output 'TASK163_STAGE_00 PASS'""",
        )
    if stage == 1:
        return (
            "Begin a fresh Runtime-owned review invocation",
            "& ./.code-harness/bin/codea-dcep-tools.exe review begin; if($LASTEXITCODE-ne 0){exit $LASTEXITCODE}; Write-Output 'TASK163_STAGE_01 PASS REVIEW_BEGIN'",
        )
    if not run_id:
        raise ValueError("fresh Runtime review runId is not visible after review begin")

    root = f".code-harness/runs/{run_id}"
    requests = f"{root}/requests"
    analysis = f"{root}/analysis"
    run_prefix = f"$run='{run_id}';"

    if stage == 2:
        return (
            "Create canonical Snapshot request in the fresh run",
            run_prefix
            + f"New-Item -ItemType Directory -Force '{requests}'|Out-Null; "
            + ps_json_write(
                f"{requests}/change-set-request.json",
                "[ordered]@{runId=$run;baseRef='HEAD';includeWorkingTree=$true}",
            )
            + "; Write-Output 'TASK163_STAGE_02 PASS'",
        )
    if stage == 3:
        return (
            "Invoke Runtime Canonical Snapshot",
            f"& ./.code-harness/bin/codea-dcep-tools.exe analysis snapshot --input {requests}/change-set-request.json; if($LASTEXITCODE-ne 0){{exit $LASTEXITCODE}}; Write-Output 'TASK163_STAGE_03 PASS'",
        )
    if stage == 4:
        return (
            "Read Snapshot and both business chains",
            f"Get-Content -Raw {analysis}/change-set.json; Get-Content -Raw src/main/java/com/acme/order/OrderController.java; Get-Content -Raw src/main/java/com/acme/order/OrderService.java; Get-Content -Raw src/main/java/com/acme/payment/PaymentController.java; Get-Content -Raw src/main/java/com/acme/payment/PaymentService.java; Write-Output 'TASK163_STAGE_04 PASS'",
        )
    if stage == 5:
        return (
            "Create two-chain semantic proposal",
            ps_json_write(
                f"{requests}/change-analysis-proposal.json",
                "[ordered]@{changedFileRoles=@([ordered]@{path='src/main/java/com/acme/order/OrderController.java';role='Controller'},[ordered]@{path='src/main/java/com/acme/payment/PaymentController.java';role='Controller'});affectedControllers=@([ordered]@{controller='OrderController';endpoints=@('OrderController.approve');impactType='DIRECT_CHANGE';sourceSymbols=@('OrderController.approve')},[ordered]@{controller='PaymentController';endpoints=@('PaymentController.pay');impactType='DIRECT_CHANGE';sourceSymbols=@('PaymentController.pay')});callChains=@([ordered]@{entryPoint='OrderController.approve';chain=@('OrderController.approve','OrderService.approve')},[ordered]@{entryPoint='PaymentController.pay';chain=@('PaymentController.pay','PaymentService.pay')});symbolLocations=@([ordered]@{symbol='OrderController.approve';path='src/main/java/com/acme/order/OrderController.java';role='Controller';source='FIND_SYMBOL'},[ordered]@{symbol='OrderService.approve';path='src/main/java/com/acme/order/OrderService.java';role='Service';source='FIND_SYMBOL'},[ordered]@{symbol='PaymentController.pay';path='src/main/java/com/acme/payment/PaymentController.java';role='Controller';source='FIND_SYMBOL'},[ordered]@{symbol='PaymentService.pay';path='src/main/java/com/acme/payment/PaymentService.java';role='Service';source='FIND_SYMBOL'});resourceRelations=@();externalDependencies=@();riskAreas=@();reviewCoverage=[ordered]@{status='COMPLETE';reviewedFiles=@([ordered]@{path='src/main/java/com/acme/order/OrderController.java';role='Controller';reason='CHANGED'},[ordered]@{path='src/main/java/com/acme/payment/PaymentController.java';role='Controller';reason='CHANGED'},[ordered]@{path='src/main/java/com/acme/order/OrderService.java';role='Service';reason='CALL_CHAIN'},[ordered]@{path='src/main/java/com/acme/payment/PaymentService.java';role='Service';reason='CALL_CHAIN'});unresolvedSymbols=@()}}",
            )
            + "; Write-Output 'TASK163_STAGE_05 PASS'",
        )
    if stage == 6:
        return (
            "Create canonical analysis certification request",
            run_prefix
            + f"$s=Get-Content -Raw {analysis}/change-set.json|ConvertFrom-Json; "
            + ps_json_write(
                f"{requests}/analysis-certify-request.json",
                f"[ordered]@{{runId=$run;snapshotPath='{analysis}/change-set.json';snapshotSha256=[string]$s.snapshotSha256;proposalPath='{requests}/change-analysis-proposal.json';intent=[ordered]@{{mode='FULL'}}}}",
            )
            + "; Write-Output 'TASK163_STAGE_06 PASS'",
        )
    if stage == 7:
        return (
            "Invoke Runtime ChangeAnalysis certification",
            f"& ./.code-harness/bin/codea-dcep-tools.exe analysis certify --input {requests}/analysis-certify-request.json; if($LASTEXITCODE-ne 0){{exit $LASTEXITCODE}}; Write-Output 'TASK163_STAGE_07 PASS'",
        )
    if stage == 8:
        return (
            "Read certified semantic authority",
            f"Get-Content -Raw {analysis}/change-analysis.json; Get-Content -Raw {analysis}/entrypoint-inventory.json; Get-Content -Raw {analysis}/change-analysis.cert.json; Write-Output 'TASK163_STAGE_08 PASS'",
        )
    if stage == 9:
        return (
            "Create Review Options request",
            ps_json_write(
                f"{requests}/review-options-request.json",
                f"[ordered]@{{runId='{run_id}';changeAnalysisPath='{analysis}/change-analysis.json'}}",
            )
            + "; Write-Output 'TASK163_STAGE_09 PASS'",
        )
    if stage == 10:
        return (
            "Invoke Runtime Review Options",
            f"& ./.code-harness/bin/codea-dcep-tools.exe review options --input {requests}/review-options-request.json; if($LASTEXITCODE-ne 0){{exit $LASTEXITCODE}}; Write-Output 'TASK163_STAGE_10 PASS'",
        )
    if stage == 11:
        return (
            "Read and expose Runtime USER_SELECTION chain options",
            f"$o=Get-Content -Raw {analysis}/review-options.json|ConvertFrom-Json; if([string]$o.decision-ne'USER_SELECTION'){{throw ('expected USER_SELECTION got '+$o.decision)}}; if(@($o.chains).Count-lt 2){{throw 'expected 2+ review chains'}}; foreach($c in @($o.chains)){{$entries=(@($c.entryPoints)-join ' -> ');Write-Output ('TASK163_RUNTIME_CHAIN_OPTION|'+[string]$c.selectionId+'|'+[string]$c.chainId+'|'+$entries)}}; Get-Content -Raw {analysis}/review-options.json; Write-Output 'TASK163_STAGE_11 PASS USER_SELECTION'",
        )
    if stage == 12:
        return (
            "Create FULL selection after explicit user choice",
            run_prefix
            + f"$o=Get-Content -Raw {analysis}/review-options.json|ConvertFrom-Json; "
            + ps_json_write(
                f"{requests}/review-selection-request.json",
                "[ordered]@{runId=$run;optionsHash=[string]$o.optionsHash;mode='FULL';selectionIds=@()}",
            )
            + "; Write-Output 'TASK163_STAGE_12 PASS USER_SELECTED_FULL'",
        )
    if stage == 13:
        return (
            "Invoke Runtime Review selection",
            f"& ./.code-harness/bin/codea-dcep-tools.exe review select --input {requests}/review-selection-request.json; if($LASTEXITCODE-ne 0){{exit $LASTEXITCODE}}; Write-Output 'TASK163_STAGE_13 PASS'",
        )
    if stage == 14:
        return (
            "Read Runtime verified FULL scope",
            f"$s=Get-Content -Raw {analysis}/review-scope.json|ConvertFrom-Json;if([string]$s.mode-ne'FULL'){{throw 'scope not FULL'}};Get-Content -Raw {analysis}/review-scope.json;Write-Output 'TASK163_STAGE_14 PASS'",
        )
    if stage == 15:
        return (
            "Build Runtime ReviewUnits",
            f"& ./.code-harness/bin/codea-dcep-tools.exe review units --run-id {run_id};if($LASTEXITCODE-ne 0){{exit $LASTEXITCODE}};Write-Output 'TASK163_STAGE_15 PASS'",
        )
    if stage == 16:
        return (
            "Build Runtime Rule Dispatch",
            f"& ./.code-harness/bin/codea-dcep-tools.exe review dispatch --run-id {run_id};if($LASTEXITCODE-ne 0){{exit $LASTEXITCODE}};Write-Output 'TASK163_STAGE_16 PASS'",
        )
    if stage == 17:
        return (
            "Create benign empty Finding Proposal and certify request",
            f"Get-Content -Raw {analysis}/review-units.json;Get-Content -Raw {analysis}/rule-dispatch.json;[System.IO.File]::WriteAllText('{requests}/finding-proposals.json','[]',[System.Text.UTF8Encoding]::new($false));"
            + ps_json_write(
                f"{requests}/finding-certify-request.json",
                f"[ordered]@{{runId='{run_id}';proposalsPath='{requests}/finding-proposals.json'}}",
            )
            + ";Write-Output 'TASK163_STAGE_17 PASS'",
        )
    if stage == 18:
        return (
            "Invoke Runtime Finding certification",
            f"& ./.code-harness/bin/codea-dcep-tools.exe review certify-findings --input {requests}/finding-certify-request.json;if($LASTEXITCODE-ne 0){{exit $LASTEXITCODE}};Write-Output 'TASK163_STAGE_18 PASS'",
        )
    if stage == 19:
        return (
            "Read Certified Findings",
            f"Get-Content -Raw {analysis}/certified-findings.json;Get-Content -Raw {analysis}/certified-findings.cert.json;Write-Output 'TASK163_STAGE_19 PASS'",
        )
    if stage == 20:
        return (
            "Create final report transport",
            run_prefix
            + f"$a=Get-Content -Raw {analysis}/change-analysis.json|ConvertFrom-Json;$paths=@($a.changedFiles|ForEach-Object{{[string]$_.path}});"
            + ps_json_write(
                f"{requests}/review-report.json",
                "[ordered]@{runId=$run;harnessVersion='runtime-owned';baseRef=[string]$a.reviewScope.baseRef;head=[string]$a.reviewScope.headCommit;result='PASSED';mode='FULL';reviewScope=[ordered]@{changedFiles=$paths};reviewCoverage=[ordered]@{reviewedFiles=@($a.reviewCoverage.reviewedFiles|ForEach-Object{[string]$_.path});callChains=@($a.callChains);externalDependencies=@($a.externalDependencies);unresolved=@();missingReviewedFiles=@();runtimeErrors=@();status='COMPLETE'};findings=@()}",
            )
            + ";Write-Output 'TASK163_STAGE_20 PASS'",
        )
    if stage == 21:
        return (
            "Invoke Runtime deterministic review renderer",
            f"& ./.code-harness/bin/codea-dcep-tools.exe report review --input {requests}/review-report.json;if($LASTEXITCODE-ne 0){{exit $LASTEXITCODE}};Write-Output 'TASK163_STAGE_21 PASS'",
        )
    if stage == 22:
        return (
            "Read final formal review artifact",
            f"Get-Content -Raw {root}/review.md;Write-Output 'TASK163_STAGE_22 PASS'",
        )
    raise ValueError(f"unknown stage {stage}")


def selection_prompt(options: list[tuple[str, str, str]]) -> str:
    lines = [
        "TURN1_SELECTION_REQUIRED",
        "检测到多个业务链，请明确选择：",
        "1) 全部评审",
        "2) 按业务链评审（可多选）：",
    ]
    for selection_id, _chain_id, entry_points in options:
        lines.append(f"{selection_id} - {entry_points}")
    lines.append("3) 仅查看调用链")
    return "\n".join(lines)


class Handler(BaseHTTPRequestHandler):
    server_version = "Task163MultiChainModel/2.0"

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
        run_id = runtime_run_id(text)
        user_text = latest_user_text(messages)
        self.append_log(
            {
                "event": "request",
                "stage": stage,
                "runId": run_id,
                "lastUser": user_text,
                "hasTools": bool(tools),
            }
        )

        if not tools or "harness review" not in text:
            self.respond_text(body, "Task 3 Multi-Chain Review E2E")
            return

        # Contract-driven checkpoint. Without the active Task 3 contract marker this
        # deterministic model continues the ordinary authority chain in the SAME turn;
        # the negative control depends on that behavior. There is no stage-number stop.
        selection_pending = USER_SELECTION_MARKER in text and SELECTION_CREATED_MARKER not in text
        hard_stop_active = HARD_STOP_CONTRACT_MARKER in text
        if selection_pending and hard_stop_active:
            # USER_SELECTION itself is the turn boundary. Do not require the original
            # `harness review` text to survive every OpenCode tool-call round: the current
            # Runtime state plus active contract is authoritative. Only an explicit next-
            # turn FULL selection is allowed to advance past this checkpoint in this E2E.
            if user_text not in {"全部", "全部评审"}:
                options = runtime_chain_options(text)
                if len(options) < 2:
                    self.respond_text(body, "TASK163_E2E_ABORT Runtime chain options unavailable for user display")
                    return
                self.append_log(
                    {
                        "event": "contract_hard_stop",
                        "runId": run_id,
                        "selectionIds": [item[0] for item in options],
                    }
                )
                self.respond_text(body, selection_prompt(options))
                return
            self.append_log(
                {
                    "event": "turn2_resume",
                    "runId": run_id,
                    "sameSession": "harness review" in text and USER_SELECTION_MARKER in text,
                }
            )

        if stage > LAST_STAGE:
            if not run_id:
                self.respond_text(body, "TASK163_E2E_ABORT fresh Runtime runId missing at completion")
                return
            self.respond_text(body, f"评审完成。正式报告：.code-harness/runs/{run_id}/review.md")
            return

        try:
            description, command = stage_command(stage, run_id)
        except ValueError as exc:
            self.respond_text(body, f"TASK163_E2E_ABORT {exc}")
            return
        tool_names = [str(t.get("function", {}).get("name", "")) for t in tools]
        if "bash" not in tool_names:
            self.respond_text(body, "TASK163_E2E_TOOL_NAMES " + ",".join(tool_names))
            return
        self.append_log(
            {
                "event": "tool_call",
                "stage": stage,
                "runId": run_id,
                "tool": "bash",
                "description": description,
                "command": command,
            }
        )
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
            self.send_sse(
                [
                    {
                        **base,
                        "object": "chat.completion.chunk",
                        "choices": [
                            {
                                "index": 0,
                                "delta": {"role": "assistant", "content": text},
                                "finish_reason": None,
                            }
                        ],
                    },
                    {
                        **base,
                        "object": "chat.completion.chunk",
                        "choices": [{"index": 0, "delta": {}, "finish_reason": "stop"}],
                    },
                ]
            )
            return
        self.send_json(
            200,
            {
                **base,
                "choices": [
                    {
                        "index": 0,
                        "message": {"role": "assistant", "content": text},
                        "finish_reason": "stop",
                    }
                ],
            },
        )

    def respond_tool(self, body: dict[str, Any], name: str, arguments: dict[str, Any]) -> None:
        base = self.completion_base(body)
        call = {
            "id": "call_" + uuid.uuid4().hex,
            "type": "function",
            "function": {"name": name, "arguments": json.dumps(arguments, ensure_ascii=False)},
        }
        if body.get("stream"):
            self.send_sse(
                [
                    {
                        **base,
                        "object": "chat.completion.chunk",
                        "choices": [
                            {
                                "index": 0,
                                "delta": {"role": "assistant", "tool_calls": [{"index": 0, **call}]},
                                "finish_reason": None,
                            }
                        ],
                    },
                    {
                        **base,
                        "object": "chat.completion.chunk",
                        "choices": [{"index": 0, "delta": {}, "finish_reason": "tool_calls"}],
                    },
                ]
            )
            return
        self.send_json(
            200,
            {
                **base,
                "choices": [
                    {
                        "index": 0,
                        "message": {"role": "assistant", "content": None, "tool_calls": [call]},
                        "finish_reason": "tool_calls",
                    }
                ],
            },
        )

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
