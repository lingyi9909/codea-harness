from __future__ import annotations

import runpy
from pathlib import Path

# Compatibility wrapper for the accepted baseline's aliased analysis import.
script = Path(__file__).with_name("task162-review-reliability-task1-apply.py")
text = script.read_text(encoding="utf-8")

old_anchor = "'\\t\"codea-harness-tools/internal/analysis\"\\n\\t\"codea-harness-tools/internal/report\"'"
aliased_anchor = "'\\tanalysisruntime \"codea-harness-tools/internal/analysis\"\\n\\t\"codea-harness-tools/internal/report\"'"
if old_anchor in text:
    text = text.replace(old_anchor, aliased_anchor, 1)
elif aliased_anchor not in text:
    raise SystemExit("Task1 v2 could not find report import patch anchor in applier")

plain_target = "'\\t\"codea-harness-tools/internal/analysis\"\\n\\t\"codea-harness-tools/internal/requestcontract\"\\n\\t\"codea-harness-tools/internal/report\"'"
aliased_target = "'\\tanalysisruntime \"codea-harness-tools/internal/analysis\"\\n\\t\"codea-harness-tools/internal/requestcontract\"\\n\\t\"codea-harness-tools/internal/report\"'"
if plain_target in text:
    text = text.replace(plain_target, aliased_target, 1)

old_contract_line = "`review-output.schema.json` 不是 `report review` 的 Agent-facing request contract。正式 report request 的 `findings` 固定为 `[]`；"
new_contract_lines = "`review-output.schema.json` 只描述既有 Review 输出结构，不能作为本请求 schema。\n正式 `report review` 的 Agent-facing request contract 只能是 `report-review-request.schema.json`。正式 report request 的 `findings` 固定为 `[]`；"
if old_contract_line in text:
    text = text.replace(old_contract_line, new_contract_lines, 1)

script.write_text(text, encoding="utf-8", newline="\n")
runpy.run_path(str(script), run_name="__main__")
