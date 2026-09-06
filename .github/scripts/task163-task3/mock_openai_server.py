#!/usr/bin/env python3
"""Task 3 deterministic OpenAI entrypoint.

OpenCode sends one Assistant turn as multiple model requests while tool calls execute.
Later requests may contain only tool transport and omit the original terminal user text.
This adapter remembers only the most recent *explicit terminal user intent* across those
requests. It does not remember or enforce USER_SELECTION state: the hard stop remains
conditional on the active Task 3 contract marker inside the engine, so removing that
contract still makes the same E2E advance past USER_SELECTION for the negative control.
"""
from __future__ import annotations

import importlib.util
from pathlib import Path
from typing import Any

_ENGINE_PATH = Path(__file__).with_name("mock_openai_engine.py")
_SPEC = importlib.util.spec_from_file_location("task163_mock_openai_engine", _ENGINE_PATH)
if _SPEC is None or _SPEC.loader is None:
    raise RuntimeError(f"cannot load Task 3 mock engine: {_ENGINE_PATH}")
engine = importlib.util.module_from_spec(_SPEC)
_SPEC.loader.exec_module(engine)

_EXPLICIT_TERMINAL_INTENTS = {"harness review", "全部", "全部评审"}
_TOOL_ENVELOPE_TYPES = {
    "tool-call",
    "tool_call",
    "tool-use",
    "tool_use",
    "tool-result",
    "tool_result",
    "function-call",
    "function_call",
    "function-result",
    "function_result",
}
_last_terminal_intent = ""


def exact_intent_leaf(value: Any) -> str:
    """Find an exact terminal-intent leaf without flattening tool transport or contracts."""
    if isinstance(value, str):
        candidate = value.strip()
        return candidate if candidate in _EXPLICIT_TERMINAL_INTENTS else ""
    if isinstance(value, list):
        for item in reversed(value):
            intent = exact_intent_leaf(item)
            if intent:
                return intent
        return ""
    if isinstance(value, dict):
        envelope_type = str(value.get("type", "")).strip().lower()
        if envelope_type in _TOOL_ENVELOPE_TYPES:
            return ""
        for key in ("text", "content", "value", "input", "prompt"):
            if key not in value:
                continue
            intent = exact_intent_leaf(value[key])
            if intent:
                return intent
    return ""


def current_explicit_user_intent(messages: list[dict[str, Any]]) -> str:
    """Extract a newly supplied terminal-user intent from the current model request."""
    for message in reversed(messages):
        if message.get("role") != "user":
            continue
        intent = exact_intent_leaf(message.get("content", ""))
        if intent:
            return intent
    # Provider adapters can place terminal input in another message envelope. Because the
    # leaf must exactly equal one of the three E2E user inputs, contract prose/tool output
    # cannot accidentally become an intent.
    for message in reversed(messages):
        intent = exact_intent_leaf(message)
        if intent:
            return intent
    return ""


def latest_explicit_user_intent(messages: list[dict[str, Any]]) -> str:
    """Carry the real terminal intent across tool-call rounds of the same Assistant turn."""
    global _last_terminal_intent
    current = current_explicit_user_intent(messages)
    if current:
        _last_terminal_intent = current
    return _last_terminal_intent


engine.latest_user_text = latest_explicit_user_intent

if __name__ == "__main__":
    engine.main()
