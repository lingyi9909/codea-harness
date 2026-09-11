#!/usr/bin/env python3
"""Deterministic OpenAI-compatible model for Codea Harness 1.6.4 product E2E.

The root session behaves like the formal Main Agent / Orchestrator contract:
- OpenCode write is used only for same-run requests/** JSON.
- OpenCode bash is used only for one exact Codea Runtime command per call.
- OpenCode task is used only to delegate the two semantic phases to Reviewer.
- Runtime progress events[].display are parsed from real Runtime output and echoed
  into the real OpenCode transcript; prompt-only stage markers are never used as
  authority.

The Reviewer child can only submit semantic proposals through the real
codea-reviewer-submit Host tool. Runtime remains the authority for analysis,
planning, finding certification, progress, and review.md.
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
from typing import Any, Callable


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


def tool_outputs(messages: list[dict[str, Any]]) -> list[str]:
    values: list[str] = []
    for item in messages:
        role = str(item.get("role", ""))
        if role in {"assistant", "system", "user"}:
            continue
        text = message_text(item.get("content", ""))
        if text:
            values.append(text)
    return values


def parse_json_output(value: str) -> Any | None:
    text = value.strip()
    try:
        return json.loads(text)
    except Exception:
        pass
    start = text.find("{")
    end = text.rfind("}")
    if start >= 0 and end > start:
        try:
            return json.loads(text[start : end + 1])
        except Exception:
            return None
    return None


def latest_json(messages: list[dict[str, Any]], predicate: Callable[[Any], bool]) -> Any | None:
    for value in reversed(tool_outputs(messages)):
        parsed = parse_json_output(value)
        if parsed is not None and predicate(parsed):
            return parsed
    return None


def assistant_calls(messages: list[dict[str, Any]]) -> list[tuple[str, dict[str, Any]]]:
    result: list[tuple[str, dict[str, Any]]] = []
    for item in messages:
        if item.get("role") != "assistant":
            continue
        for call in item.get("tool_calls") or []:
            function = call.get("function") or {}
            name = str(function.get("name", ""))
            raw = function.get("arguments", "{}")
            try:
                arguments = json.loads(raw) if isinstance(raw, str) else dict(raw or {})
            except Exception:
                arguments = {}
            result.append((name, arguments))
    return result


def assistant_content(messages: list[dict[str, Any]]) -> str:
    return "\n".join(
        message_text(item.get("content", ""))
        for item in messages
        if item.get("role") == "assistant"
    )


def progress_display_content(messages: list[dict[str, Any]]) -> str:
    progress = latest_json(
        messages,
        lambda obj: isinstance(obj, dict) and isinstance(obj.get("events"), list) and isinstance(obj.get("stages"), list),
    )
    if not isinstance(progress, dict):
        return ""
    prior = assistant_content(messages)
    displays: list[str] = []
    for event in progress.get("events") or []:
        if not isinstance(event, dict):
            continue
        display = event.get("display")
        if isinstance(display, str) and display and display not in prior:
            displays.append(display)
    return "\n".join(displays)


def json_text(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, separators=(",", ":")) + "\n"


class Handler(BaseHTTPRequestHandler):
    server_version = "Task164ClosureProductE2E/2.0"

    def log_message(self, *_: object) -> None:
        return

    @property
    def log_path(self) -> Path:
        return self.server.log_path  # type: ignore[attr-defined]

    @property
    def scenario(self) -> str:
        return self.server.scenario  # type: ignore[attr-defined]

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
        self.append_log({"responseType": "text", "content": content})
        base = completion_base(body)
        if body.get("stream"):
            self.send_sse(
                [
                    {
                        **base,
                        "object": "chat.completion.chunk",
                        "choices": [{"index": 0, "delta": {"role": "assistant", "content": content}, "finish_reason": None}],
                    },
                    {**base, "object": "chat.completion.chunk", "choices": [{"index": 0, "delta": {}, "finish_reason": "stop"}]},
                ]
            )
            return
        self.send_json(
            200,
            {**base, "choices": [{"index": 0, "message": {"role": "assistant", "content": content}, "finish_reason": "stop"}]},
        )

    def respond_tool(
        self,
        body: dict[str, Any],
        name: str,
        arguments: dict[str, Any],
        *,
        content: str = "",
        runtime_progress_source: bool = False,
    ) -> None:
        self.append_log(
            {
                "responseType": "tool",
                "tool": name,
                "arguments": arguments,
                "content": content,
                "runtimeProgressSource": runtime_progress_source,
            }
        )
        base = completion_base(body)
        call = {
            "id": "call_" + uuid.uuid4().hex,
            "type": "function",
            "function": {"name": name, "arguments": json.dumps(arguments, ensure_ascii=False)},
        }
        if body.get("stream"):
            delta: dict[str, Any] = {"role": "assistant", "tool_calls": [{"index": 0, **call}]}
            if content:
                delta["content"] = content
            self.send_sse(
                [
                    {**base, "object": "chat.completion.chunk", "choices": [{"index": 0, "delta": delta, "finish_reason": None}]},
                    {**base, "object": "chat.completion.chunk", "choices": [{"index": 0, "delta": {}, "finish_reason": "tool_calls"}]},
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
                        "message": {"role": "assistant", "content": content or None, "tool_calls": [call]},
                        "finish_reason": "tool_calls",
                    }
                ],
            },
        )

    def do_GET(self) -> None:  # noqa: N802
        if self.path == "/health":
            self.send_json(200, {"status": "ok", "scenario": self.scenario})
            return
        if self.path.rstrip("/") == "/v1/models":
            self.send_json(200, {"object": "list", "data": [{"id": "task4", "object": "model", "owned_by": "task164"}]})
            return
        self.send_json(404, {"error": "not found"})

    def reviewer_response(
        self,
        body: dict[str, Any],
        text: str,
        tool_result_text: str,
        names: list[str],
    ) -> bool:
        submit = next((name for name in names if "reviewer" in name.lower() and "submit" in name.lower()), "")
        if not submit:
            return False
        run_match = re.search(r"runId=(review-[0-9a-f]+)", text)
        if not run_match:
            self.respond_text(body, "REVIEWER_MALFORMED_OUTPUT missing runId")
            return True
        run_id = run_match.group(1)
        phase = "FINDINGS" if "phase=FINDINGS" in text else "CHANGE_ANALYSIS"
        if self.scenario == "reviewer-unavailable" and phase == "CHANGE_ANALYSIS":
            self.respond_text(body, "REVIEWER_UNAVAILABLE controlled independent Reviewer failure")
            return True
        if phase == "FINDINGS":
            if "REVIEWER_PROPOSAL_SUBMITTED kind=findings" in tool_result_text:
                self.respond_text(body, "Reviewer FINDINGS proposal submitted through codea-reviewer-submit")
            else:
                self.respond_tool(body, submit, {"kind": "findings", "runId": run_id, "proposal": "[]"})
            return True
        if "REVIEWER_PROPOSAL_SUBMITTED kind=change-analysis" in tool_result_text:
            self.respond_text(body, "Reviewer CHANGE_ANALYSIS proposal submitted through codea-reviewer-submit")
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
                "reviewedFiles": [
                    {"path": "src/main/resources/application.yml", "role": "YamlConfig", "reason": "CHANGED"}
                ],
                "unresolvedSymbols": [],
            },
        }
        self.respond_tool(
            body,
            submit,
            {"kind": "change-analysis", "runId": run_id, "proposal": json.dumps(proposal, separators=(",", ":"))},
        )
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
        outputs = tool_outputs(messages)
        tool_result_text = "\n".join(outputs)
        names = [str(item.get("function", {}).get("name", "")) for item in tools]
        self.append_log({"requestType": "completion", "messages": messages, "toolNames": names})
        with self.server.request_lock:  # type: ignore[attr-defined]
            self.server.request_count += 1  # type: ignore[attr-defined]
            request_count = self.server.request_count  # type: ignore[attr-defined]
        if request_count > 96:
            self.respond_text(body, f"TASK4_REQUEST_LIMIT_EXCEEDED count={request_count} HARD STOP")
            return

        is_root = any(
            item.get("role") == "user" and message_text(item.get("content", "")).strip() == "harness review"
            for item in messages
        )
        is_reviewer = not is_root and "Reviewer 是只读 Agent" in system_text
        if is_reviewer and self.reviewer_response(body, text, tool_result_text, names):
            return
        if not tools or "harness review" not in text:
            self.respond_text(body, "Codea Harness 1.6.4 product E2E")
            return

        bash = "bash" if "bash" in names else ""
        write = "write" if "write" in names else ""
        task = "task" if "task" in names else ""
        if not bash or not write or not task:
            self.respond_text(body, "MANUAL_ACTION_REQUIRED missing required root Host tool; HARD STOP")
            return

        calls = assistant_calls(messages)
        bash_commands = [str(args.get("command", "")) for name, args in calls if name == bash]
        write_paths = [str(args.get("filePath", "")) for name, args in calls if name == write]
        task_prompts = [str(args.get("prompt", "")) for name, args in calls if name == task]
        progress_calls = [cmd for cmd in bash_commands if " review progress --run-id " in f" {cmd} "]
        rendered = progress_display_content(messages)

        begin = latest_json(
            messages,
            lambda obj: isinstance(obj, dict)
            and isinstance(obj.get("runId"), str)
            and str(obj.get("runId")).startswith("review-")
            and obj.get("status") == "READY"
            and "runPath" in obj,
        )
        run_id = str(begin.get("runId")) if isinstance(begin, dict) else ""
        req = f".code-harness/runs/{run_id}/requests" if run_id else ""
        ana = f".code-harness/runs/{run_id}/analysis" if run_id else ""

        def send_runtime(command: str, description: str, *, show_progress: bool = False) -> None:
            self.respond_tool(
                body,
                bash,
                {"command": command, "description": description},
                content=rendered if show_progress else "",
                runtime_progress_source=show_progress and bool(rendered),
            )

        def send_write(path: str, value: Any, description: str, *, show_progress: bool = False) -> None:
            self.respond_tool(
                body,
                write,
                {"filePath": path, "content": json_text(value)},
                content=rendered if show_progress else "",
                runtime_progress_source=show_progress and bool(rendered),
            )

        if not any(cmd.endswith("codea-dcep-tools.exe review begin") for cmd in bash_commands):
            send_runtime(".code-harness/bin/codea-dcep-tools.exe review begin", "Begin fresh Runtime-owned review")
            return
        if not run_id:
            self.respond_text(body, "MANUAL_ACTION_REQUIRED Runtime review begin did not return runId; HARD STOP")
            return
        if len(progress_calls) == 0:
            send_runtime(
                f".code-harness/bin/codea-dcep-tools.exe review progress --run-id {run_id}",
                "Render Runtime review progress after REVIEW_BEGIN",
            )
            return

        snapshot_request = f"{req}/change-set-request.json"
        if snapshot_request not in write_paths:
            send_write(
                snapshot_request,
                {"runId": run_id, "baseRef": "HEAD", "includeWorkingTree": True},
                "Write same-run snapshot request",
                show_progress=True,
            )
            return
        snapshot_cmd = f".code-harness/bin/codea-dcep-tools.exe analysis snapshot --input {snapshot_request}"
        if snapshot_cmd not in bash_commands:
            send_runtime(snapshot_cmd, "Create canonical Runtime snapshot")
            return
        if len(progress_calls) == 1:
            send_runtime(
                f".code-harness/bin/codea-dcep-tools.exe review progress --run-id {run_id}",
                "Render Runtime review progress after SNAPSHOT",
            )
            return

        change_prompt_prefix = f"runId={run_id} phase=CHANGE_ANALYSIS"
        if not any(prompt.startswith(change_prompt_prefix) for prompt in task_prompts):
            self.respond_tool(
                body,
                task,
                {
                    "prompt": (
                        f"runId={run_id} phase=CHANGE_ANALYSIS snapshotPath={ana}/change-set.json. "
                        "Submit the semantic change-analysis proposal using codea-reviewer-submit exactly once."
                    ),
                    "description": "Delegate CHANGE_ANALYSIS to independent Reviewer",
                    "subagent_type": "reviewer",
                    "command": "harness-review-reviewer",
                },
                content=rendered,
                runtime_progress_source=bool(rendered),
            )
            return

        unavailable_cmd = f".code-harness/bin/codea-dcep-tools.exe review reviewer-unavailable --run-id {run_id}"
        if self.scenario == "reviewer-unavailable":
            if unavailable_cmd not in bash_commands:
                send_runtime(unavailable_cmd, "Record independent Reviewer unavailability at Runtime boundary")
                return
            if len(progress_calls) == 2:
                send_runtime(
                    f".code-harness/bin/codea-dcep-tools.exe review progress --run-id {run_id}",
                    "Render Runtime failure progress after Reviewer unavailability",
                )
                return
            progress = latest_json(
                messages,
                lambda obj: isinstance(obj, dict) and isinstance(obj.get("events"), list) and obj.get("status") == "FAILED",
            )
            content = rendered
            if isinstance(progress, dict):
                content = (content + "\n" if content else "") + "REVIEWER_UNAVAILABLE\nMANUAL_ACTION_REQUIRED\nHARD STOP"
            self.respond_text(body, content or "REVIEWER_UNAVAILABLE\nMANUAL_ACTION_REQUIRED\nHARD STOP")
            return

        if len(progress_calls) == 2:
            send_runtime(
                f".code-harness/bin/codea-dcep-tools.exe review progress --run-id {run_id}",
                "Render Runtime progress after Reviewer CHANGE_ANALYSIS proposal",
            )
            return

        snapshot = latest_json(
            messages,
            lambda obj: isinstance(obj, dict) and obj.get("status") == "SNAPSHOT_READY" and "snapshotSha256" in obj,
        )
        if not isinstance(snapshot, dict):
            self.respond_text(body, "MANUAL_ACTION_REQUIRED Runtime snapshot evidence missing; HARD STOP")
            return
        certify_request = f"{req}/analysis-certify-request.json"
        if certify_request not in write_paths:
            send_write(
                certify_request,
                {
                    "runId": run_id,
                    "snapshotPath": f"{ana}/change-set.json",
                    "snapshotSha256": str(snapshot["snapshotSha256"]),
                    "proposalPath": f"{req}/change-analysis-proposal.json",
                    "intent": {"mode": "FULL"},
                },
                "Write same-run analysis certification request",
                show_progress=True,
            )
            return
        certify_cmd = f".code-harness/bin/codea-dcep-tools.exe analysis certify --input {certify_request}"
        if certify_cmd not in bash_commands:
            send_runtime(certify_cmd, "Runtime certify independent Reviewer change analysis")
            return
        if len(progress_calls) == 3:
            send_runtime(
                f".code-harness/bin/codea-dcep-tools.exe review progress --run-id {run_id}",
                "Render Runtime progress after analysis certification",
            )
            return

        options_request = f"{req}/review-options-request.json"
        if options_request not in write_paths:
            send_write(
                options_request,
                {"runId": run_id, "changeAnalysisPath": f"{ana}/change-analysis.json"},
                "Write same-run review options request",
                show_progress=True,
            )
            return
        options_cmd = f".code-harness/bin/codea-dcep-tools.exe review options --input {options_request}"
        if options_cmd not in bash_commands:
            send_runtime(options_cmd, "Runtime produce review options")
            return
        options = latest_json(
            messages,
            lambda obj: isinstance(obj, dict)
            and obj.get("status") == "READY"
            and isinstance(obj.get("options"), dict)
            and "optionsHash" in obj["options"],
        )
        if not isinstance(options, dict) or str(options["options"].get("decision")) != "AUTO_FULL":
            self.respond_text(body, "MANUAL_ACTION_REQUIRED expected AUTO_FULL review options; HARD STOP")
            return
        selection_request = f"{req}/review-selection-request.json"
        if selection_request not in write_paths:
            send_write(
                selection_request,
                {
                    "runId": run_id,
                    "optionsHash": str(options["options"]["optionsHash"]),
                    "mode": "FULL",
                    "selectionIds": [],
                },
                "Write same-run review selection request",
            )
            return
        select_cmd = f".code-harness/bin/codea-dcep-tools.exe review select --input {selection_request}"
        if select_cmd not in bash_commands:
            send_runtime(select_cmd, "Runtime bind AUTO_FULL review selection")
            return
        units_cmd = f".code-harness/bin/codea-dcep-tools.exe review units --run-id {run_id}"
        if units_cmd not in bash_commands:
            send_runtime(units_cmd, "Runtime build review units")
            return
        dispatch_cmd = f".code-harness/bin/codea-dcep-tools.exe review dispatch --run-id {run_id}"
        if dispatch_cmd not in bash_commands:
            send_runtime(dispatch_cmd, "Runtime build rule dispatch")
            return
        if len(progress_calls) == 4:
            send_runtime(
                f".code-harness/bin/codea-dcep-tools.exe review progress --run-id {run_id}",
                "Render Runtime progress after review planning",
            )
            return

        finding_prompt_prefix = f"runId={run_id} phase=FINDINGS"
        if not any(prompt.startswith(finding_prompt_prefix) for prompt in task_prompts):
            self.respond_tool(
                body,
                task,
                {
                    "prompt": (
                        f"runId={run_id} phase=FINDINGS reviewUnitsPath={ana}/review-units.json. "
                        "Submit the benign empty findings array using codea-reviewer-submit exactly once."
                    ),
                    "description": "Delegate FINDINGS to independent Reviewer",
                    "subagent_type": "reviewer",
                    "command": "harness-review-reviewer",
                },
                content=rendered,
                runtime_progress_source=bool(rendered),
            )
            return
        if len(progress_calls) == 5:
            send_runtime(
                f".code-harness/bin/codea-dcep-tools.exe review progress --run-id {run_id}",
                "Render Runtime progress after Reviewer FINDINGS proposal",
            )
            return

        finding_request = f"{req}/finding-certify-request.json"
        if finding_request not in write_paths:
            send_write(
                finding_request,
                {"runId": run_id, "proposalsPath": f"{req}/finding-proposals.json"},
                "Write same-run finding certification request",
                show_progress=True,
            )
            return
        finding_cmd = f".code-harness/bin/codea-dcep-tools.exe review certify-findings --input {finding_request}"
        if finding_cmd not in bash_commands:
            send_runtime(finding_cmd, "Runtime certify independent Reviewer findings")
            return
        if len(progress_calls) == 6:
            send_runtime(
                f".code-harness/bin/codea-dcep-tools.exe review progress --run-id {run_id}",
                "Render Runtime progress after finding certification",
            )
            return

        report_request = f"{req}/review-report.json"
        if report_request not in write_paths:
            head = str(snapshot.get("headCommit", "HEAD"))
            base_ref = str(snapshot.get("resolvedBaseCommit", snapshot.get("mergeBase", "HEAD")))
            changed = ["src/main/resources/application.yml"]
            send_write(
                report_request,
                {
                    "runId": run_id,
                    "harnessVersion": "1.6.4",
                    "baseRef": base_ref,
                    "head": head,
                    "result": "PASSED",
                    "mode": "FULL",
                    "reviewScope": {"changedFiles": changed},
                    "reviewCoverage": {
                        "reviewedFiles": changed,
                        "callChains": [],
                        "externalDependencies": [],
                        "unresolved": [],
                        "missingReviewedFiles": [],
                        "runtimeErrors": [],
                        "status": "COMPLETE",
                    },
                    "findings": [],
                },
                "Write same-run final report request",
                show_progress=True,
            )
            return
        report_cmd = f".code-harness/bin/codea-dcep-tools.exe report review --input {report_request}"
        if report_cmd not in bash_commands:
            send_runtime(report_cmd, "Runtime render final review report")
            return
        if len(progress_calls) == 7:
            send_runtime(
                f".code-harness/bin/codea-dcep-tools.exe review progress --run-id {run_id}",
                "Render terminal Runtime review progress",
            )
            return

        progress = latest_json(
            messages,
            lambda obj: isinstance(obj, dict) and isinstance(obj.get("events"), list) and isinstance(obj.get("stages"), list),
        )
        if not isinstance(progress, dict) or progress.get("status") != "SUCCEEDED":
            self.respond_text(body, "MANUAL_ACTION_REQUIRED Runtime progress did not reach SUCCEEDED; HARD STOP")
            return
        final_content = rendered
        if final_content:
            final_content += "\n"
        final_content += f"TASK4_FULL_REVIEW_COMPLETE runId={run_id}"
        self.respond_text(body, final_content)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", required=True, type=int)
    parser.add_argument("--log", required=True, type=Path)
    parser.add_argument("--scenario", choices=["success", "reviewer-unavailable"], default="success")
    args = parser.parse_args()
    args.log.parent.mkdir(parents=True, exist_ok=True)
    args.log.write_text("", encoding="utf-8")
    server = ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
    server.log_path = args.log  # type: ignore[attr-defined]
    server.scenario = args.scenario  # type: ignore[attr-defined]
    server.request_count = 0  # type: ignore[attr-defined]
    server.request_lock = threading.Lock()  # type: ignore[attr-defined]
    server.serve_forever()


if __name__ == "__main__":
    main()
