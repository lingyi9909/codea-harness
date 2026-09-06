#!/usr/bin/env python3
"""Task 3 deterministic OpenAI entrypoint.

OpenCode may encode tool-result transport as role=user messages and may wrap terminal
user text in provider-specific content envelopes. This adapter extracts only exact
terminal user intents; USER_SELECTION stopping remains conditional on the active Task 3
contract marker inside the engine, so the no-contract negative control still advances in
the same Assistant turn.
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
        # Provider adapters use several wrappers for genuine text. Inspect only known
        # text-bearing fields and require the leaf itself to exactly equal an intent.
        for key in ("text", "content", "value", "input", "prompt"):
            if key not in value:
                continue
            intent = exact_intent_leaf(value[key])
            if intent:
                return intent
    return ""


def latest_explicit_user_intent(messages: list[dict[str, Any]]) -> str:
    """Return the newest explicit terminal-user intent, never inferred from tool output."""
    # Normal OpenAI-compatible shape: terminal input is a role=user message.
    for message in reversed(messages):
        if message.get("role") != "user":
            continue
        intent = exact_intent_leaf(message.get("content", ""))
        if intent:
            return intent

    # OpenCode/provider adapters can wrap the original terminal text outside the canonical
    # user content field on later tool-call rounds. Exact-leaf matching keeps this fallback
    # fail-closed: contract prose and command transcripts are larger strings and do not match.
    for message in reversed(messages):
        intent = exact_intent_leaf(message)
        if intent:
            return intent
    return ""


engine.latest_user_text = latest_explicit_user_intent

if __name__ == "__main__":
    engine.main()
