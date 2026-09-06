#!/usr/bin/env python3
"""Task 3 deterministic OpenAI entrypoint.

OpenCode may encode tool-result transport as role=user messages. The underlying engine
must distinguish those internal messages from the explicit terminal user intents that
advance the multi-turn review contract. This adapter changes only user-intent extraction;
USER_SELECTION stopping remains conditional on the active Task 3 contract marker inside
the engine, so the no-contract negative control still advances in the same Assistant turn.
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


def latest_explicit_user_intent(messages: list[dict[str, Any]]) -> str:
    """Return only an actual terminal-user intent, ignoring tool-result transport."""
    for message in reversed(messages):
        if message.get("role") != "user":
            continue
        text = engine.flatten(message.get("content", "")).strip()
        if text in _EXPLICIT_TERMINAL_INTENTS:
            return text
    return ""


engine.latest_user_text = latest_explicit_user_intent

if __name__ == "__main__":
    engine.main()
