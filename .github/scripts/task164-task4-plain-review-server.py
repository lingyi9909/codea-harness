#!/usr/bin/env python3
"""Deterministic OpenAI-compatible model for Task 4 packaged full review E2E.

The server never writes Harness artifacts. It only asks the real OpenCode host to
invoke real tools/Reviewer children. Runtime and Reviewer Host remain the only
authorities for review artifacts and receipts.
"""

from __future__ import annotations

import argparse
import json
import re
import threading
import time
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any


def flatten(value: Any) -> str:
    if isinstance(value, str):
        return value
    if isinstance(value, list):
        return "\n".join(flatten(item) for item in value)
    if isinstance(value, dict):
        return "\n".join(f"{key}:{flatten(item)}" for key, item in value.items())
    return str(value)


def message_text(value: Any) -> str:
    if isinstance(value, str):
        return value
    if isinstance(value, list):
        return "\n".join(message_text(item) for item in value)
    if isinstance(value, dict):
        if isinstance(value.get("text"), str):
            return value["text"]
        return message_text(value.get("content", ""))
    return ""


def completion_base(body: dict[str, Any]) -> dict[str, Any]:
    return {
        "id": "chatcmpl-" + uuid.uuid4().hex,
        "object": "chat.completion",
        "created": int(time.time()),
        "model": body.get("model", "task4"),
    }


class Handler(BaseHTTPRequestHandler):
    server_version = "Task164Task4FullReview/1.0"

    def log_message(self, *_: object) -> None:
        return

    @property
    def log_path(self) -> Path:
        return self.server.log_path  # type: ignore[attr-defined]

    def append_log(self, value: dict[str, Any]) -> None:
        with self.log_path.open("a", encoding="utf-8") as stream:
            stream.write(json.dumps(value, ensure_ascii=False) + "\n")

    def send_json(self, status: int, value: Any) -> None:
        raw = json.dumps(value, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

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

    def respond_text(self, body: dict[str, Any], content: str) -> None:
        base = completion_base(body)
        if body.get("stream"):
            self.send_sse([
                {**base, "object": "chat.completion.chunk", "choices": [{"index": 0, "delta": {"role": "assistant", "content": content}, "finish_reason": None}]},
                {**base, "object": "chat.completion.chunk", "choices": [{"index": 0, "delta": {}, "finish_reason": "stop"}]},
            ])
            return
        self.send_json(200, {**base, "choices": [{"index": 0, "message": {"role": "assistant", "content": content}, "finish_reason": "stop"}]})

    def respond_tool(self, body: dict[str, Any], name: str, arguments: dict[str, Any]) -> None:
        base = completion_base(body)
        call = {"id": "call_" + uuid.uuid4().hex, "type": "function", "function": {"name": name, "arguments": json.dumps(arguments, ensure_ascii=False)}}
        if body.get("stream"):
            self.send_sse([
                {**base, "object": "chat.completion.chunk", "choices": [{"index": 0, "delta": {"role": "assistant", "tool_calls": [{"index": 0, **call}]}, "finish_reason": None}]},
                {**base, "object": "chat.completion.chunk", "choices": [{"index": 0, "delta": {}, "finish_reason": "tool_calls"}]},
            ])
            return
        self.send_json(200, {**base, "choices": [{"index": 0, "message": {"role": "assistant", "content": None, "tool_calls": [call]}, "finish_reason": "tool_calls"}]})

    def do_GET(self) -> None:  # noqa: N802
        if self.path == "/health":
            self.send_json(200, {"status": "ok"})
            return
        if self.path.rstrip("/") == "/v1/models":
            self.send_json(200, {"object": "list", "data": [{"id": "task4", "object": "model", "owned_by": "task164"}]})
            return
        self.send_json(404, {"error": "not found"})

    def reviewer_response(self, body: dict[str, Any], text: str, tool_result_text: str, names: list[str]) -> bool:
        submit = next((name for name in names if "reviewer" in name.lower() and "submit" in name.lower()), "")
        if not submit:
            return False
        run_match = re.search(r"runId=(review-[0-9a-f]+)", text)
        if not run_match:
            self.respond_text(body, "REVIEWER_MALFORMED_OUTPUT missing runId")
            return True
        run_id = run_match.group(1)
        if "phase=FINDINGS" in text:
            if "REVIEWER_PROPOSAL_SUBMITTED kind=findings" in tool_result_text:
                self.respond_text(body, "TASK4_FINDING_REVIEWER_COMPLETE")
            else:
                self.respond_tool(body, submit, {"kind": "findings", "runId": run_id, "proposal": "[]"})
            return True
        if "REVIEWER_PROPOSAL_SUBMITTED kind=change-analysis" in tool_result_text:
            self.respond_text(body, "TASK4_CHANGE_REVIEWER_COMPLETE")
            return True
        proposal = {
            "changedFileRoles": [{"path": "src/main/resources/application.yml", "role": "YamlConfig"}],
            "affectedControllers": [],
            "callChains": [],
            "symbolLocations": [],
            "resourceRelations": [],
            "externalDependencies": [],
            "riskAreas": [],
            "reviewCoverage": {
                "status": "COMPLETE",
                "reviewedFiles": [{"path": "src/main/resources/application.yml", "role": "YamlConfig", "reason": "CHANGED"}],
                "unresolvedSymbols": [],
            },
        }
        self.respond_tool(body, submit, {"kind": "change-analysis", "runId": run_id, "proposal": json.dumps(proposal, separators=(",", ":"))})
        return True

    def do_POST(self) -> None:  # noqa: N802
        if self.path.rstrip("/") != "/v1/chat/completions":
            self.send_json(404, {"error": "not found"})
            return
        body = json.loads(self.rfile.read(int(self.headers.get("Content-Length", "0"))))
        messages = body.get("messages") or []
        tools = body.get("tools") or []
        text = flatten(messages)
        system_text = flatten([item for item in messages if item.get("role") == "system"])
        tool_result_text = flatten([item for item in messages if item.get("role") not in {"assistant", "system"}])
        names = [str(item.get("function", {}).get("name", "")) for item in tools]
        self.append_log({"messages": messages, "toolNames": names})
        with self.server.request_lock:  # type: ignore[attr-defined]
            self.server.request_count += 1  # type: ignore[attr-defined]
            request_count = self.server.request_count  # type: ignore[attr-defined]
        if request_count > 64:
            self.respond_text(body, f"TASK4_REQUEST_LIMIT_EXCEEDED count={request_count} HARD STOP")
            return

        is_root = any(item.get("role") == "user" and message_text(item.get("content", "")).strip() == "harness review" for item in messages)
        is_reviewer = not is_root and "Reviewer 是只读 Agent" in system_text
        if is_reviewer and self.reviewer_response(body, text, tool_result_text, names):
            return
        if not tools or "harness review" not in text:
            self.respond_text(body, "Task 4 packaged full review E2E")
            return

        bash = "bash" if "bash" in names else ""
        task = "task" if "task" in names else ""
        if not bash:
            self.respond_text(body, "TASK4_ABORT missing bash tool")
            return

        run_match = re.search(r"TASK4_REVIEW_BEGIN runId=(review-[0-9a-f]+)", tool_result_text)
        run_id = run_match.group(1) if run_match else ""

        if "TASK4_STAGE_00 PASS" not in tool_result_text:
            cmd = "$paths=@('.code-harness/AGENTS.md','.code-harness/bootstrap.md','.code-harness/agents/orchestrator.md','.code-harness/contracts/reviewer-host-contract.md'); foreach($p in $paths){if(!(Test-Path $p)){throw ('missing '+$p)}; Get-Content -Raw $p|Out-Null}; Write-Output 'TASK4_STAGE_00 PASS'"
            self.respond_tool(body, bash, {"command": cmd, "description": "Read packaged Harness authority contracts"})
            return
        if not run_id:
            cmd = "$raw=(& ./.code-harness/bin/codea-dcep-tools.exe review begin 2>&1|Out-String); if($LASTEXITCODE -ne 0){throw $raw}; $j=$raw|ConvertFrom-Json; Write-Output ('TASK4_REVIEW_BEGIN runId='+[string]$j.runId); Write-Output 'TASK4_STAGE_01 PASS'"
            self.respond_tool(body, bash, {"command": cmd, "description": "Begin fresh Runtime-owned review"})
            return

        req = f".code-harness/runs/{run_id}/requests"
        ana = f".code-harness/runs/{run_id}/analysis"
        if "TASK4_STAGE_02 PASS" not in tool_result_text:
            cmd = f"New-Item -ItemType Directory -Force '{req}'|Out-Null; $v=[ordered]@{{runId='{run_id}';baseRef='HEAD';includeWorkingTree=$true}}; [IO.File]::WriteAllText('{req}/change-set-request.json',($v|ConvertTo-Json -Compress),[Text.UTF8Encoding]::new($false)); & ./.code-harness/bin/codea-dcep-tools.exe analysis snapshot --input {req}/change-set-request.json; if($LASTEXITCODE -ne 0){{exit $LASTEXITCODE}}; Write-Output 'TASK4_SNAPSHOT_COMPLETE'; Write-Output 'TASK4_STAGE_02 PASS'"
            self.respond_tool(body, bash, {"command": cmd, "description": "Create canonical Runtime snapshot"})
            return
        if "TASK4_CHANGE_REVIEWER_COMPLETE" not in tool_result_text:
            if not task:
                self.respond_text(body, "REVIEWER_UNAVAILABLE\nMANUAL_ACTION_REQUIRED\nHARD STOP")
                return
            args = {"prompt": f"runId={run_id} phase=CHANGE_ANALYSIS snapshotPath={ana}/change-set.json. Submit the semantic change-analysis proposal using codea-reviewer-submit exactly once.", "description": "Task 4 independent change-analysis Reviewer", "subagent_type": "reviewer", "command": "harness-review-reviewer"}
            self.respond_tool(body, task, args)
            return
        if "TASK4_STAGE_03 PASS" not in tool_result_text:
            cmd = f"if(!(Test-Path '{req}/change-analysis-proposal.json') -or !(Test-Path '{req}/change-analysis-reviewer-authority.json')){{throw 'change Reviewer proposal/receipt missing'}}; Write-Output 'TASK4_STAGE_03 PASS'"
            self.respond_tool(body, bash, {"command": cmd, "description": "Confirm change-analysis Reviewer authority receipt"})
            return
        if "TASK4_STAGE_04 PASS" not in tool_result_text:
            cmd = f"$s=Get-Content -Raw '{ana}/change-set.json'|ConvertFrom-Json; $v=[ordered]@{{runId='{run_id}';snapshotPath='{ana}/change-set.json';snapshotSha256=[string]$s.snapshotSha256;proposalPath='{req}/change-analysis-proposal.json';intent=[ordered]@{{mode='FULL'}}}}; [IO.File]::WriteAllText('{req}/analysis-certify-request.json',($v|ConvertTo-Json -Depth 10 -Compress),[Text.UTF8Encoding]::new($false)); & ./.code-harness/bin/codea-dcep-tools.exe analysis certify --input {req}/analysis-certify-request.json; if($LASTEXITCODE -ne 0){{exit $LASTEXITCODE}}; Write-Output 'TASK4_ANALYSIS_CERTIFIED'; Write-Output 'TASK4_STAGE_04 PASS'"
            self.respond_tool(body, bash, {"command": cmd, "description": "Runtime certify independent Reviewer analysis"})
            return
        if "TASK4_STAGE_05 PASS" not in tool_result_text:
            cmd = f"$v=[ordered]@{{runId='{run_id}';changeAnalysisPath='{ana}/change-analysis.json'}}; [IO.File]::WriteAllText('{req}/review-options-request.json',($v|ConvertTo-Json -Compress),[Text.UTF8Encoding]::new($false)); & ./.code-harness/bin/codea-dcep-tools.exe review options --input {req}/review-options-request.json; if($LASTEXITCODE -ne 0){{exit $LASTEXITCODE}}; Write-Output 'TASK4_REVIEW_OPTIONS'; Write-Output 'TASK4_STAGE_05 PASS'"
            self.respond_tool(body, bash, {"command": cmd, "description": "Runtime produce review options"})
            return
        if "TASK4_STAGE_06 PASS" not in tool_result_text:
            cmd = f"$o=Get-Content -Raw '{ana}/review-options.json'|ConvertFrom-Json; if([string]$o.decision -ne 'AUTO_FULL'){{throw ('expected AUTO_FULL got '+$o.decision)}}; $v=[ordered]@{{runId='{run_id}';optionsHash=[string]$o.optionsHash;mode='FULL';selectionIds=@()}}; [IO.File]::WriteAllText('{req}/review-selection-request.json',($v|ConvertTo-Json -Compress),[Text.UTF8Encoding]::new($false)); & ./.code-harness/bin/codea-dcep-tools.exe review select --input {req}/review-selection-request.json; if($LASTEXITCODE -ne 0){{exit $LASTEXITCODE}}; Write-Output 'TASK4_REVIEW_SELECTED'; Write-Output 'TASK4_STAGE_06 PASS'"
            self.respond_tool(body, bash, {"command": cmd, "description": "Runtime bind AUTO_FULL selection"})
            return
        if "TASK4_STAGE_07 PASS" not in tool_result_text:
            cmd = f"& ./.code-harness/bin/codea-dcep-tools.exe review units --run-id {run_id}; if($LASTEXITCODE -ne 0){{exit $LASTEXITCODE}}; Write-Output 'TASK4_REVIEW_UNITS'; Write-Output 'TASK4_STAGE_07 PASS'"
            self.respond_tool(body, bash, {"command": cmd, "description": "Runtime build review units"})
            return
        if "TASK4_STAGE_08 PASS" not in tool_result_text:
            cmd = f"& ./.code-harness/bin/codea-dcep-tools.exe review dispatch --run-id {run_id}; if($LASTEXITCODE -ne 0){{exit $LASTEXITCODE}}; Write-Output 'TASK4_RULE_DISPATCH'; Write-Output 'TASK4_STAGE_08 PASS'"
            self.respond_tool(body, bash, {"command": cmd, "description": "Runtime build rule dispatch"})
            return
        if "TASK4_FINDING_REVIEWER_COMPLETE" not in tool_result_text:
            if not task:
                self.respond_text(body, "REVIEWER_UNAVAILABLE\nMANUAL_ACTION_REQUIRED\nHARD STOP")
                return
            args = {"prompt": f"runId={run_id} phase=FINDINGS reviewUnitsPath={ana}/review-units.json. Submit the benign empty findings array using codea-reviewer-submit exactly once.", "description": "Task 4 independent finding Reviewer", "subagent_type": "reviewer", "command": "harness-review-reviewer"}
            self.respond_tool(body, task, args)
            return
        if "TASK4_STAGE_09 PASS" not in tool_result_text:
            cmd = f"if(!(Test-Path '{req}/finding-proposals.json') -or !(Test-Path '{req}/finding-reviewer-authority.json')){{throw 'finding Reviewer proposal/receipt missing'}}; Write-Output 'TASK4_STAGE_09 PASS'"
            self.respond_tool(body, bash, {"command": cmd, "description": "Confirm finding Reviewer authority receipt"})
            return
        if "TASK4_STAGE_10 PASS" not in tool_result_text:
            cmd = f"$v=[ordered]@{{runId='{run_id}';proposalsPath='{req}/finding-proposals.json'}}; [IO.File]::WriteAllText('{req}/finding-certify-request.json',($v|ConvertTo-Json -Compress),[Text.UTF8Encoding]::new($false)); & ./.code-harness/bin/codea-dcep-tools.exe review certify-findings --input {req}/finding-certify-request.json; if($LASTEXITCODE -ne 0){{exit $LASTEXITCODE}}; Write-Output 'TASK4_FINDINGS_CERTIFIED'; Write-Output 'TASK4_STAGE_10 PASS'"
            self.respond_tool(body, bash, {"command": cmd, "description": "Runtime certify Reviewer findings"})
            return
        if "TASK4_STAGE_11 PASS" not in tool_result_text:
            cmd = f"$a=Get-Content -Raw '{ana}/change-analysis.json'|ConvertFrom-Json; $paths=@($a.changedFiles|ForEach-Object{{[string]$_.path}}); $v=[ordered]@{{runId='{run_id}';harnessVersion='transport-not-authority';baseRef=[string]$a.reviewScope.baseRef;head=[string]$a.reviewScope.headCommit;result='FAILED';mode='FULL';reviewScope=[ordered]@{{changedFiles=$paths}};reviewCoverage=[ordered]@{{reviewedFiles=$paths;callChains=@();externalDependencies=@();unresolved=@();missingReviewedFiles=@();runtimeErrors=@();status='COMPLETE'}};findings=@()}}; [IO.File]::WriteAllText('{req}/review-report.json',($v|ConvertTo-Json -Depth 20 -Compress),[Text.UTF8Encoding]::new($false)); & ./.code-harness/bin/codea-dcep-tools.exe report review --input {req}/review-report.json; if($LASTEXITCODE -ne 0){{exit $LASTEXITCODE}}; Write-Output 'TASK4_REPORT_WRITTEN'; Write-Output 'TASK4_STAGE_11 PASS'"
            self.respond_tool(body, bash, {"command": cmd, "description": "Runtime render final authority-bound review report"})
            return
        if "TASK4_STAGE_12 PASS" not in tool_result_text:
            cmd = f"$raw=(& ./.code-harness/bin/codea-dcep-tools.exe review progress --run-id {run_id} 2>&1|Out-String); if($LASTEXITCODE -ne 0){{throw $raw}}; $p=$raw|ConvertFrom-Json; if([string]$p.status -ne 'SUCCEEDED' -or [string]$p.terminalStage -ne 'REPORT'){{throw ('progress not terminal success: '+$raw)}}; if(@($p.stages|Where-Object{{$_.status -ne 'SUCCEEDED'}}).Count -ne 0){{throw ('not all stages succeeded: '+$raw)}}; if(@($p.events|Where-Object{{$_.display -eq '[8/8] REPORT PASS'}}).Count -ne 1){{throw ('8/8 report event missing: '+$raw)}}; Write-Output $raw; Write-Output 'TASK4_RUNTIME_PROGRESS_8_OF_8'; Write-Output 'TASK4_STAGE_12 PASS'"
            self.respond_tool(body, bash, {"command": cmd, "description": "Read terminal Runtime progress state"})
            return
        self.respond_text(body, f"TASK4_FULL_REVIEW_COMPLETE runId={run_id}")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", required=True, type=int)
    parser.add_argument("--log", required=True, type=Path)
    args = parser.parse_args()
    args.log.parent.mkdir(parents=True, exist_ok=True)
    args.log.write_text("", encoding="utf-8")
    server = ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
    server.log_path = args.log  # type: ignore[attr-defined]
    server.request_count = 0  # type: ignore[attr-defined]
    server.request_lock = threading.Lock()  # type: ignore[attr-defined]
    server.serve_forever()


if __name__ == "__main__":
    main()
