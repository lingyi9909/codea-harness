#!/usr/bin/env python3
"""Deterministic OpenAI-compatible model for the Task 2 packaged plain-review E2E.

The model can only ask OpenCode to use its real tools.  It never creates a run,
snapshot, Reviewer receipt, certificate, or ReviewOptions artifact itself.
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


def flatten(value: Any) -> str:
    if isinstance(value, str):
        return value
    if isinstance(value, list):
        return "\n".join(flatten(item) for item in value)
    if isinstance(value, dict):
        return "\n".join(f"{key}:{flatten(item)}" for key, item in value.items())
    return str(value)


def completion_base(body: dict[str, Any]) -> dict[str, Any]:
    return {
        "id": "chatcmpl-" + uuid.uuid4().hex,
        "object": "chat.completion",
        "created": int(time.time()),
        "model": body.get("model", "task2-entry"),
    }


class Handler(BaseHTTPRequestHandler):
    server_version = "Task164Task2EntryModel/1.0"

    def log_message(self, *_: object) -> None:
        return

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

    @property
    def scenario(self) -> str:
        return self.server.scenario  # type: ignore[attr-defined]

    @property
    def log_path(self) -> Path:
        return self.server.log_path  # type: ignore[attr-defined]

    def append_log(self, value: dict[str, Any]) -> None:
        with self.log_path.open("a", encoding="utf-8") as stream:
            stream.write(json.dumps(value, ensure_ascii=False) + "\n")

    def do_GET(self) -> None:  # noqa: N802
        if self.path == "/health":
            self.send_json(200, {"status": "ok"})
            return
        if self.path.rstrip("/") == "/v1/models":
            self.send_json(200, {"object": "list", "data": [{"id": "task2-entry", "object": "model", "owned_by": "task164"}]})
            return
        self.send_json(404, {"error": "not found"})

    def do_POST(self) -> None:  # noqa: N802
        if self.path.rstrip("/") != "/v1/chat/completions":
            self.send_json(404, {"error": "not found"})
            return
        body = json.loads(self.rfile.read(int(self.headers.get("Content-Length", "0"))))
        messages = body.get("messages") or []
        tools = body.get("tools") or []
        text = flatten(messages)
        system_text = flatten([item for item in messages if item.get("role") == "system"])
        tool_result_text = flatten([
            item for item in messages
            if item.get("role") not in {"assistant", "system"}
            and flatten(item.get("content", "")).strip() != "harness review"
        ])
        names = [str(item.get("function", {}).get("name", "")) for item in tools]
        self.append_log({"scenario": self.scenario, "messages": messages, "toolNames": names})

        submit = next((name for name in names if "reviewer" in name.lower() and "submit" in name.lower()), "")
        is_root = any(item.get("role") == "user" and flatten(item.get("content", "")).strip() == "harness review" for item in messages)
        is_reviewer = not is_root and submit != "" and "Reviewer 是只读 Agent" in system_text
        if is_reviewer:
            if re.search(r"REVIEWER_PROPOSAL_SUBMITTED kind=change-analysis\b", tool_result_text):
                self.respond_text(body, "TASK2_ENTRY_REVIEWER_CHILD_COMPLETE")
                return
            run_match = re.search(r"runId=(review-[0-9a-f]+)", text)
            if not run_match:
                self.respond_text(body, "REVIEWER_MALFORMED_OUTPUT: missing fresh runId")
                return
            proposal = {
                "changedFileRoles": [{"path": "src/main/resources/application.yml", "role": "YamlConfig"}],
                "affectedControllers": [], "callChains": [], "symbolLocations": [],
                "resourceRelations": [], "externalDependencies": [], "riskAreas": [],
                "reviewCoverage": {"status": "COMPLETE", "reviewedFiles": [{"path": "src/main/resources/application.yml", "role": "YamlConfig", "reason": "CHANGED"}], "unresolvedSymbols": []},
            }
            self.respond_tool(body, submit, {"kind": "change-analysis", "runId": run_match.group(1), "proposal": json.dumps(proposal, separators=(",", ":"))})
            return

        if not tools or "harness review" not in text:
            self.respond_text(body, "Task 2 packaged entry E2E")
            return
        required_root_contract = [
            "codea-dcep-tools.exe review begin",
            "harness-review-reviewer",
            "independent reviewer child session",
            "Runtime analysis certify",
            "review options",
        ]
        missing_contract = [directive for directive in required_root_contract if directive not in system_text]
        if missing_contract:
            self.respond_text(body, "TASK2_ENTRY_CONTRACT_MISSING " + " | ".join(missing_contract))
            return
        bash = "bash" if "bash" in names else ""
        if not bash:
            self.respond_text(body, "TASK2_ENTRY_ABORT missing OpenCode bash tool")
            return

        run_match = re.search(r"TASK2_ENTRY_REVIEW_BEGIN runId=(review-[0-9a-f]+)", tool_result_text)
        run_id = run_match.group(1) if run_match else ""
        if "TASK2_ENTRY_STAGE_00 PASS" not in tool_result_text:
            command = "$paths=@('.code-harness/AGENTS.md','.code-harness/agents/orchestrator.md','.code-harness/contracts/reviewer-host-contract.md'); foreach($p in $paths){Get-Content -Raw $p}; Write-Output 'TASK2_ENTRY_STAGE_00 PASS'"
            self.respond_tool(body, bash, {"command": command, "description": "Read packaged review routing and Reviewer host contracts"})
            return
        if "TASK2_ENTRY_STAGE_01 PASS" not in tool_result_text:
            command = "$raw=(& ./.code-harness/bin/codea-dcep-tools.exe review begin 2>&1 | Out-String); if($LASTEXITCODE -ne 0){throw $raw}; $value=$raw|ConvertFrom-Json; Write-Output ('TASK2_ENTRY_REVIEW_BEGIN runId='+[string]$value.runId); Write-Output 'TASK2_ENTRY_STAGE_01 PASS'"
            self.respond_tool(body, bash, {"command": command, "description": "Begin a fresh review through the packaged Runtime"})
            return
        if not run_id:
            self.respond_text(body, "TASK2_ENTRY_ABORT fresh Runtime runId missing")
            return
        request_root = f".code-harness/runs/{run_id}/requests"
        analysis_root = f".code-harness/runs/{run_id}/analysis"
        if "TASK2_ENTRY_STAGE_02 PASS" not in tool_result_text:
            command = (
                f"$run='{run_id}'; New-Item -ItemType Directory -Force '{request_root}'|Out-Null; "
                f"$value=[ordered]@{{runId=$run;baseRef='HEAD';includeWorkingTree=$true}}; $json=$value|ConvertTo-Json -Compress; "
                f"[IO.File]::WriteAllText('{request_root}/change-set-request.json',$json,[Text.UTF8Encoding]::new($false)); "
                f"& ./.code-harness/bin/codea-dcep-tools.exe analysis snapshot --input {request_root}/change-set-request.json; "
                "if($LASTEXITCODE -ne 0){exit $LASTEXITCODE}; Write-Output 'TASK2_ENTRY_SNAPSHOT'; Write-Output 'TASK2_ENTRY_STAGE_02 PASS'"
            )
            self.respond_tool(body, bash, {"command": command, "description": "Create the same-run canonical snapshot"})
            return
        if self.scenario == "disabled" and "TASK2_ENTRY_STAGE_03 PASS" not in tool_result_text:
            command = (
                "$inventory=(& opencode agent list 2>&1|Out-String); if($LASTEXITCODE -eq 0 -and $inventory -match '(?m)^reviewer\\b'){throw 'disabled Reviewer remained resolvable'}; "
                f"$s=Get-Content -Raw '{analysis_root}/change-set.json'|ConvertFrom-Json; "
                f"$value=[ordered]@{{runId='{run_id}';snapshotPath='{analysis_root}/change-set.json';snapshotSha256=[string]$s.snapshotSha256;proposalPath='{request_root}/change-analysis-proposal.json';intent=[ordered]@{{mode='FULL'}}}}; "
                f"[IO.File]::WriteAllText('{request_root}/analysis-certify-request.json',($value|ConvertTo-Json -Depth 10 -Compress),[Text.UTF8Encoding]::new($false)); "
                f"$raw=(& ./.code-harness/bin/codea-dcep-tools.exe analysis certify --input {request_root}/analysis-certify-request.json 2>&1|Out-String); $exit=$LASTEXITCODE; "
                "if($exit -eq 0){throw 'Reviewer-disabled certification unexpectedly succeeded'}; foreach($m in @('REVIEWER_UNAVAILABLE','MANUAL_ACTION_REQUIRED','HARD STOP')){if($raw -notmatch [regex]::Escape($m)){throw ('missing hard-stop marker '+$m+': '+$raw)}}; "
                "Write-Output 'TASK2_ENTRY_RUNTIME_HARD_STOP_BEGIN'; Write-Output $raw; Write-Output 'TASK2_ENTRY_STAGE_03 PASS'"
            )
            self.respond_tool(body, bash, {"command": command, "description": "Prove Reviewer-disabled Runtime hard stop"})
            return
        if "TASK2_ENTRY_REVIEWER_CHILD_COMPLETE" not in tool_result_text and "TASK2_ENTRY_STAGE_03 PASS" not in tool_result_text:
            if "task" not in names:
                self.respond_text(body, "REVIEWER_UNAVAILABLE\nMANUAL_ACTION_REQUIRED\nHARD STOP")
                return
            args = {"prompt": f"runId={run_id} phase=CHANGE_ANALYSIS snapshotPath={analysis_root}/change-set.json. Submit the semantic proposal using codea-reviewer-submit exactly once, then output TASK2_ENTRY_REVIEWER_PROPOSAL_SUBMITTED.", "description": "Task 2 entry E2E independent Reviewer", "subagent_type": "reviewer", "command": "harness-review-reviewer"}
            self.respond_tool(body, "task", args)
            return
        if "TASK2_ENTRY_STAGE_03 PASS" not in tool_result_text:
            command = f"if(!(Test-Path '{request_root}/change-analysis-proposal.json') -or !(Test-Path '{request_root}/change-analysis-reviewer-authority.json')){{throw 'Reviewer proposal or receipt missing'}}; Write-Output 'TASK2_ENTRY_PROPOSAL'; Write-Output 'TASK2_ENTRY_STAGE_03 PASS'"
            self.respond_tool(body, bash, {"command": command, "description": "Confirm Reviewer proposal and Host authority receipt"})
            return
        if self.scenario == "disabled":
            self.respond_text(body, "REVIEWER_UNAVAILABLE\nMANUAL_ACTION_REQUIRED\nHARD STOP")
            return
        if "TASK2_ENTRY_STAGE_04 PASS" not in tool_result_text:
            command = (
                f"$s=Get-Content -Raw '{analysis_root}/change-set.json'|ConvertFrom-Json; "
                f"$value=[ordered]@{{runId='{run_id}';snapshotPath='{analysis_root}/change-set.json';snapshotSha256=[string]$s.snapshotSha256;proposalPath='{request_root}/change-analysis-proposal.json';intent=[ordered]@{{mode='FULL'}}}}; "
                f"[IO.File]::WriteAllText('{request_root}/analysis-certify-request.json',($value|ConvertTo-Json -Depth 10 -Compress),[Text.UTF8Encoding]::new($false)); "
                f"& ./.code-harness/bin/codea-dcep-tools.exe analysis certify --input {request_root}/analysis-certify-request.json; if($LASTEXITCODE -ne 0){{exit $LASTEXITCODE}}; Write-Output 'TASK2_ENTRY_CERTIFY'; Write-Output 'TASK2_ENTRY_STAGE_04 PASS'"
            )
            self.respond_tool(body, bash, {"command": command, "description": "Certify the Reviewer proposal with the packaged Runtime"})
            return
        if "TASK2_ENTRY_STAGE_05 PASS" not in tool_result_text:
            command = (
                f"$value=[ordered]@{{runId='{run_id}';changeAnalysisPath='{analysis_root}/change-analysis.json'}}; "
                f"[IO.File]::WriteAllText('{request_root}/review-options-request.json',($value|ConvertTo-Json -Compress),[Text.UTF8Encoding]::new($false)); "
                f"& ./.code-harness/bin/codea-dcep-tools.exe review options --input {request_root}/review-options-request.json; if($LASTEXITCODE -ne 0){{exit $LASTEXITCODE}}; Write-Output 'TASK2_ENTRY_REVIEW_OPTIONS'; Write-Output 'TASK2_ENTRY_STAGE_05 PASS'"
            )
            self.respond_tool(body, bash, {"command": command, "description": "Produce same-run Runtime ReviewOptions"})
            return
        self.respond_text(body, f"Task 2 entry chain reached Runtime ReviewOptions for {run_id}")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", required=True, type=int)
    parser.add_argument("--log", required=True, type=Path)
    parser.add_argument("--scenario", required=True, choices=["positive", "disabled"])
    args = parser.parse_args()
    args.log.parent.mkdir(parents=True, exist_ok=True)
    args.log.write_text("", encoding="utf-8")
    server = ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
    server.log_path = args.log  # type: ignore[attr-defined]
    server.scenario = args.scenario  # type: ignore[attr-defined]
    server.serve_forever()


if __name__ == "__main__":
    main()
