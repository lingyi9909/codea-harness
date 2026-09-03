from __future__ import annotations

import runpy
from pathlib import Path

# Compatibility wrapper for the accepted baseline's aliased analysis import.
script = Path(__file__).with_name("task162-review-reliability-task1-apply.py")
text = script.read_text(encoding="utf-8")
old = "'\\t\"codea-harness-tools/internal/analysis\"\\n\\t\"codea-harness-tools/internal/report\"'"
new = "'\\tanalysisruntime \"codea-harness-tools/internal/analysis\"\\n\\t\"codea-harness-tools/internal/report\"'"
if old not in text and new not in text:
    raise SystemExit("Task1 v2 could not find report import patch anchor in applier")
if old in text:
    text = text.replace(old, new, 1)
    script.write_text(text, encoding="utf-8", newline="\n")
runpy.run_path(str(script), run_name="__main__")
