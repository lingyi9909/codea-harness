#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import time
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any


def flatten(value: Any) -> str:
    if isinstance(value, str):
        return value
    if isinstance(value, list):
        return "\n".join(flatten(v) for v in value)
    if isinstance(value, dict):
        return "\n".join(f"{k}:{flatten(v)}" for k, v in value.items())
    return str(value)


def completion_base(body: dict[str, Any]) -> dict[str, Any]:
    return {
        "id": "chatcmpl-" + uuid.uuid4().hex,
        "object": "chat.completion",
        "created": int(time.time()),
        "model": body.get("model", "reviewer-e2e"),
    }


def ps_json_write(path: str, expression: str) -> str:
    return (
        f"$value = {expression}; "
        f"$json = $value | ConvertTo-Json -Depth 40 -Compress; "
        f'[IO.File]::WriteAllText("{path}", $json, [Text.UTF8Encoding]::new($false))'
    )


BEGIN = (
    "$raw = (& ./.code-harness/bin/codea-dcep-tools.exe review begin 2>&1 | Out-String); "
    "$exit = $LASTEXITCODE; if ($exit -ne 0) { throw \"review begin failed exit=$exit`n$raw\" }; "
    "$obj = $raw | ConvertFrom-Json; $run = [string]$obj.runId; "
    "[IO.File]::WriteAllText('.task164-run-id', $run, [Text.UTF8Encoding]::new($false)); "
    "Write-Output $raw; Write-Output ('TASK164_PLAIN_STAGE_BEGIN PASS run=' + $run)"
)

SNAPSHOT = (
    "$run=(Get-Content -Raw '.task164-run-id').Trim(); "
    "New-Item -ItemType Directory -Force \".code-harness/runs/$run/requests\" | Out-Null; "
    + ps_json_write(
        ".code-harness/runs/$run/requests/change-set-request.json",
        "[ordered]@{runId=$run;baseRef='HEAD';includeWorkingTree=$true}",
    )
    + "; & ./.code-harness/bin/codea-dcep-tools.exe analysis snapshot --input \".code-harness/runs/$run/requests/change-set-request.json\"; "
    "if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; "
    "Write-Output 'TASK164_PLAIN_STAGE_SNAPSHOT PASS'"
)

REVIEWER = r'''$run=(Get-Content -Raw '.task164-run-id').Trim();
$input="runId=$run phase=CHANGE_ANALYSIS snapshotPath=.code-harness/runs/$run/analysis/change-set.json proposalPath=.code-harness/runs/$run/requests/change-analysis-proposal.json. Use codea-reviewer-submit exactly once.";
$ErrorActionPreference='Continue';
$reviewerRaw = (& opencode run --command harness-review-reviewer --model mock/reviewer-e2e --format json --title task164-plain-reviewer $input 2>&1 | Out-String);
$reviewerExit = $LASTEXITCODE;
$ErrorActionPreference='Stop';
[IO.File]::WriteAllText('.task164-reviewer-transcript.jsonl',$reviewerRaw,[Text.UTF8Encoding]::new($false));
$proposal=".code-harness/runs/$run/requests/change-analysis-proposal.json";
$receipt=".code-harness/runs/$run/requests/change-analysis-reviewer-authority.json";
if ($reviewerExit -ne 0 -or $reviewerRaw -notmatch '"subagent_type":"reviewer"' -or !(Test-Path $proposal -PathType Leaf) -or !(Test-Path $receipt -PathType Leaf)) {
  $snapshot=Get-Content -Raw ".code-harness/runs/$run/analysis/change-set.json" | ConvertFrom-Json;
  $cert=[ordered]@{runId=$run;snapshotPath=".code-harness/runs/$run/analysis/change-set.json";snapshotSha256=[string]$snapshot.snapshotSha256;proposalPath=$proposal;intent=[ordered]@{mode='FULL'}};
  $certJson=$cert|ConvertTo-Json -Depth 20 -Compress;
  [IO.File]::WriteAllText(".code-harness/runs/$run/requests/analysis-certify-request.json",$certJson,[Text.UTF8Encoding]::new($false));
  $ErrorActionPreference='Continue';
  $rt = (& ./.code-harness/bin/codea-dcep-tools.exe analysis certify --input ".code-harness/runs/$run/requests/analysis-certify-request.json" 2>&1 | Out-String);
  $rtExit=$LASTEXITCODE;
  $ErrorActionPreference='Stop';
  if ($rtExit -eq 0) { throw "Reviewer unavailable path unexpectedly certified analysis`n$rt" };
  foreach($marker in @('REVIEWER_UNAVAILABLE','MANUAL_ACTION_REQUIRED','HARD STOP')) { if ($rt -notmatch [regex]::Escape($marker)) { throw "missing hard-stop marker $marker`n$rt" } };
  foreach($authority in @('analysis/change-analysis.json','analysis/change-analysis.cert.json','analysis/review-options.json','analysis/review-scope.json','analysis/review-units.json','analysis/rule-dispatch.json','analysis/certified-findings.json','analysis/certified-findings.cert.json','review.md')) { if (Test-Path ".code-harness/runs/$run/$authority") { throw "Reviewer unavailable path published downstream authority $authority" } };
  Write-Output $reviewerRaw; Write-Output $rt; Write-Output 'TASK164_PLAIN_REVIEW_REVIEWER_UNAVAILABLE_HARD_STOP PASS'; exit 0
}
Write-Output $reviewerRaw;
Write-Output 'TASK164_PLAIN_STAGE_REVIEWER PASS' '''

CERTIFY = (
    "$run=(Get-Content -Raw '.task164-run-id').Trim(); "
    "$snapshot=Get-Content -Raw \".code-harness/runs/$run/analysis/change-set.json\" | ConvertFrom-Json; "
    + ps_json_write(
        ".code-harness/runs/$run/requests/analysis-certify-request.json",
        "[ordered]@{runId=$run;snapshotPath=\".code-harness/runs/$run/analysis/change-set.json\";snapshotSha256=[string]$snapshot.snapshotSha256;proposalPath=\".code-harness/runs/$run/requests/change-analysis-proposal.json\";intent=[ordered]@{mode='FULL'}}",
    )
    + "; & ./.code-harness/bin/codea-dcep-tools.exe analysis certify --input \".code-harness/runs/$run/requests/analysis-certify-request.json\"; "
    "if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; Write-Output 'TASK164_PLAIN_STAGE_RUNTIME_CERTIFY PASS'"
)

OPTIONS = (
    "$run=(Get-Content -Raw '.task164-run-id').Trim(); "
    + ps_json_write(
        ".code-harness/runs/$run/requests/review-options-request.json",
        "[ordered]@{runId=$run;changeAnalysisPath=\".code-harness/runs/$run/analysis/change-analysis.json\"}",
    )
    + "; & ./.code-harness/bin/codea-dcep-tools.exe review options --input \".code-harness/runs/$run/requests/review-options-request.json\"; "
    "if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }; "
    "$options=Get-Content -Raw \".code-harness/runs/$run/analysis/review-options.json\" | ConvertFrom-Json; "
    "Write-Output ('TASK164_PLAIN_STAGE_REVIEW_OPTIONS PASS decision=' + [string]$options.decision)"
)


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *_: object) -> None:
        return

    @property
    def log_path(self) -> Path:
        return self.server.log_path  # type: ignore[attr-defined]

    def append_log(self, obj: dict[str, Any]) -> None:
        with self.log_path.open("a", encoding="utf-8") as fh:
            fh.write(json.dumps(obj, ensure_ascii=False) + "\n")

    def send_json(self, status: int, obj: Any) -> None:
        raw = json.dumps(obj, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def respond_text(self, body: dict[str, Any], text: str) -> None:
        base = completion_base(body)
        message = {"role": "assistant", "content": text}
        if body.get("stream"):
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.send_header("Cache-Control", "no-cache")
            self.end_headers()
            chunks = [
                {**base, "object": "chat.completion.chunk", "choices": [{"index": 0, "delta": message, "finish_reason": None}]},
                {**base, "object": "chat.completion.chunk", "choices": [{"index": 0, "delta": {}, "finish_reason": "stop"}]},
            ]
            for chunk in chunks:
                self.wfile.write(("data: " + json.dumps(chunk, ensure_ascii=False) + "\n\n").encode("utf-8"))
            self.wfile.write(b"data: [DONE]\n\n")
            return
        self.send_json(200, {**base, "choices": [{"index": 0, "message": message, "finish_reason": "stop"}]})

    def respond_tool(self, body: dict[str, Any], name: str, args: dict[str, Any]) -> None:
        base = completion_base(body)
        call = {
            "id": "call_" + uuid.uuid4().hex,
            "type": "function",
            "function": {"name": name, "arguments": json.dumps(args, ensure_ascii=False)},
        }
        if body.get("stream"):
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.send_header("Cache-Control", "no-cache")
            self.end_headers()
            chunks = [
                {**base, "object": "chat.completion.chunk", "choices": [{"index": 0, "delta": {"role": "assistant", "tool_calls": [{"index": 0, **call}]}, "finish_reason": None}]},
                {**base, "object": "chat.completion.chunk", "choices": [{"index": 0, "delta": {}, "finish_reason": "tool_calls"}]},
            ]
            for chunk in chunks:
                self.wfile.write(("data: " + json.dumps(chunk, ensure_ascii=False) + "\n\n").encode("utf-8"))
            self.wfile.write(b"data: [DONE]\n\n")
            return
        self.send_json(200, {**base, "choices": [{"index": 0, "message": {"role": "assistant", "content": None, "tool_calls": [call]}, "finish_reason": "tool_calls"}]})

    def do_GET(self) -> None:
        if self.path.rstrip("/").endswith("/models"):
            self.send_json(200, {"object": "list", "data": [{"id": "reviewer-e2e", "object": "model", "owned_by": "task164"}]})
            return
        self.send_json(200, {"ok": True})

    def do_POST(self) -> None:
        size = int(self.headers.get("Content-Length", "0"))
        try:
            body = json.loads(self.rfile.read(size))
        except Exception as exc:
            self.send_json(400, {"error": str(exc)})
            return
        messages = body.get("messages") or []
        tools = body.get("tools") or []
        text = flatten(messages)
        names = [str(t.get("function", {}).get("name", "")) for t in tools]
        self.append_log({"toolNames": names, "messages": messages})

        if "phase=CHANGE_ANALYSIS" in text:
            if any(marker in text for marker in ("REVIEWER_PROPOSAL_SUBMITTED", "REVIEWER_MALFORMED_OUTPUT", "MAIN_AGENT_REVIEWER_FALLBACK_FORBIDDEN")):
                self.respond_text(body, "TASK164_REVIEWER_SUBMISSION_COMPLETE")
                return
            submit = ""
            for tool in tools:
                fn = tool.get("function", {})
                desc = str(fn.get("description", ""))
                name = str(fn.get("name", ""))
                if "Submit a Codea Harness semantic proposal" in desc or ("reviewer" in name.lower() and "submit" in name.lower()):
                    submit = name
                    break
            if not submit:
                self.respond_text(body, "REVIEWER_SUBMISSION_TOOL_UNAVAILABLE")
                return
            run = ""
            for token in text.replace("\n", " ").split():
                if token.startswith("runId="):
                    run = token.split("=", 1)[1].strip().strip(".,")
                    break
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
            self.respond_tool(body, submit, {"kind": "change-analysis", "runId": run, "proposal": json.dumps(proposal, separators=(",", ":"))})
            return

        if "harness review" not in text or not tools:
            self.respond_text(body, "Task164 Harness Review E2E")
            return
        if "TASK164_PLAIN_REVIEW_REVIEWER_UNAVAILABLE_HARD_STOP PASS" in text:
            self.respond_text(body, "REVIEWER_UNAVAILABLE\nMANUAL_ACTION_REQUIRED\nHARD STOP")
            return
        stages = [
            ("TASK164_PLAIN_STAGE_BEGIN PASS", BEGIN, "Start Runtime review run"),
            ("TASK164_PLAIN_STAGE_SNAPSHOT PASS", SNAPSHOT, "Create canonical Runtime snapshot"),
            ("TASK164_PLAIN_STAGE_REVIEWER PASS", REVIEWER, "Delegate semantic analysis to independent Reviewer"),
            ("TASK164_PLAIN_STAGE_RUNTIME_CERTIFY PASS", CERTIFY, "Runtime certify Reviewer proposal"),
            ("TASK164_PLAIN_STAGE_REVIEW_OPTIONS PASS", OPTIONS, "Build Runtime review options"),
        ]
        for marker, command, description in stages:
            if marker not in text:
                if "bash" not in names:
                    self.respond_text(body, "TASK164_E2E_BASH_TOOL_UNAVAILABLE")
                    return
                self.respond_tool(body, "bash", {"command": command, "description": description})
                return
        self.respond_text(body, "TASK164_PLAIN_HARNESS_REVIEW_COMPLETE")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", type=int, required=True)
    parser.add_argument("--log", type=Path, required=True)
    args = parser.parse_args()
    args.log.parent.mkdir(parents=True, exist_ok=True)
    server = ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
    server.log_path = args.log  # type: ignore[attr-defined]
    server.serve_forever()


if __name__ == "__main__":
    main()
