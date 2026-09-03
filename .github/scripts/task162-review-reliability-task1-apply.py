from __future__ import annotations

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


def write(path: str, text: str) -> None:
    (ROOT / path).write_text(text, encoding="utf-8", newline="\n")


def replace_once(path: str, old: str, new: str) -> None:
    text = read(path)
    if new in text:
        return
    if old not in text:
        raise SystemExit(f"Task1 patch anchor missing in {path}: {old[:120]!r}")
    text = text.replace(old, new, 1)
    write(path, text)


def append_once(path: str, marker: str, block: str) -> None:
    text = read(path)
    if marker in text:
        return
    if not text.endswith("\n"):
        text += "\n"
    write(path, text + "\n" + block.strip() + "\n")


finding_schema = {
    "$schema": "https://json-schema.org/draft/2020-12/schema",
    "$id": "finding-certify-request.schema.json",
    "title": "Finding Certification Request",
    "type": "object",
    "additionalProperties": False,
    "required": ["runId", "proposalsPath"],
    "properties": {
        "runId": {
            "type": "string",
            "pattern": "^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$",
        },
        "proposalsPath": {
            "type": "string",
            "pattern": r"^\.code-harness/runs/[A-Za-z0-9][A-Za-z0-9._-]{0,127}/requests/finding-proposals\.json$",
        },
    },
}

string_array = {"type": "array", "items": {"type": "string"}}
call_chain = {
    "type": "object",
    "additionalProperties": False,
    "required": ["entryPoint", "chain"],
    "properties": {
        "entryPoint": {"type": "string", "minLength": 1},
        "chain": {"type": "array", "items": {"type": "string", "minLength": 1}},
    },
}
symbol_role = {
    "type": "object",
    "additionalProperties": False,
    "required": ["symbol", "role", "source"],
    "properties": {
        "symbol": {"type": "string", "minLength": 1},
        "role": {
            "enum": [
                "Controller", "Service", "Repository", "Mapper", "Entity", "DTO", "VO",
                "Validator", "ExceptionHandler", "Config", "Utility", "Other",
            ]
        },
        "source": {"enum": ["FIND_SYMBOL", "FIND_REFERENCES", "FIND_IMPLEMENTATIONS"]},
    },
}
resource_role = {
    "type": "object",
    "additionalProperties": False,
    "required": ["resource", "role", "source"],
    "properties": {
        "resource": {"type": "string", "minLength": 1},
        "role": {"enum": ["MapperXml", "YamlConfig"]},
        "source": {"enum": ["MAPPER_STATEMENT", "CONFIG_REFERENCE"]},
    },
}
report_schema = {
    "$schema": "https://json-schema.org/draft/2020-12/schema",
    "$id": "report-review-request.schema.json",
    "title": "Formal Review Report Request",
    "type": "object",
    "additionalProperties": False,
    "required": [
        "runId", "harnessVersion", "baseRef", "head", "result",
        "reviewScope", "reviewCoverage", "findings",
    ],
    "properties": {
        "runId": {"type": "string", "pattern": "^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$"},
        "harnessVersion": {"type": "string", "minLength": 1},
        "baseRef": {"type": "string", "minLength": 1},
        "head": {"type": "string", "minLength": 1},
        "result": {"enum": ["PASSED", "FAILED", "MANUAL_ACTION_REQUIRED"]},
        "mode": {"enum": ["FULL", "TARGETED"]},
        "target": {
            "type": "object",
            "additionalProperties": False,
            "required": ["symbol", "kind"],
            "properties": {
                "symbol": {"type": "string", "minLength": 1},
                "kind": {"enum": ["CLASS", "METHOD"]},
            },
        },
        "chainContext": {
            "type": "object",
            "additionalProperties": False,
            "required": ["id", "name", "source", "status"],
            "properties": {
                "id": {"type": "string", "minLength": 1},
                "name": {"type": "string", "minLength": 1},
                "source": {"enum": ["ACCEPTED", "DISCOVERED"]},
                "status": {"enum": ["VALID", "TEMPORARY"]},
            },
        },
        "reviewScope": {
            "type": "object",
            "additionalProperties": False,
            "required": ["changedFiles"],
            "properties": {
                "changedFiles": string_array,
                "scopedFiles": string_array,
            },
        },
        "reviewCoverage": {
            "type": "object",
            "additionalProperties": False,
            "required": [
                "reviewedFiles", "callChains", "externalDependencies", "unresolved",
                "missingReviewedFiles", "runtimeErrors", "status",
            ],
            "properties": {
                "reviewedFiles": string_array,
                "callChains": {"type": "array", "items": call_chain},
                "symbolRoleEvidence": {"type": "array", "items": symbol_role},
                "resourceRoleEvidence": {"type": "array", "items": resource_role},
                "externalDependencies": string_array,
                "unresolved": string_array,
                "missingReviewedFiles": string_array,
                "runtimeErrors": string_array,
                "status": {"enum": ["COMPLETE", "PARTIAL"]},
            },
        },
        "findings": {
            "type": "array",
            "maxItems": 0,
            "description": "Agent-facing formal report requests must not carry raw findings; Runtime loads same-run Certified Findings.",
        },
    },
    "allOf": [
        {
            "if": {"properties": {"mode": {"const": "TARGETED"}}, "required": ["mode"]},
            "then": {
                "required": ["target"],
                "properties": {
                    "reviewScope": {
                        "required": ["scopedFiles"],
                        "properties": {"scopedFiles": {"minItems": 1}},
                    },
                    "reviewCoverage": {
                        "properties": {"callChains": {"minItems": 1}},
                    },
                },
            },
        },
        {
            "if": {
                "anyOf": [
                    {"properties": {"mode": {"const": "FULL"}}, "required": ["mode"]},
                    {"not": {"required": ["mode"]}},
                ]
            },
            "then": {"not": {"required": ["target"]}},
        },
    ],
}

contracts_dir = ROOT / ".code-harness/contracts"
(contracts_dir / "finding-certify-request.schema.json").write_text(
    json.dumps(finding_schema, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
)
(contracts_dir / "report-review-request.schema.json").write_text(
    json.dumps(report_schema, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
)

replace_once(
    ".code-harness/tools-runtime/cmd/codea-dcep-tools/review_precision_command.go",
    '\t"codea-harness-tools/internal/finding"\n\t"codea-harness-tools/internal/reviewrules"',
    '\t"codea-harness-tools/internal/finding"\n\t"codea-harness-tools/internal/requestcontract"\n\t"codea-harness-tools/internal/reviewrules"',
)
replace_once(
    ".code-harness/tools-runtime/cmd/codea-dcep-tools/review_precision_command.go",
    '\trequestBytes, err := os.ReadFile(cleanInput)\n\tif err != nil {\n\t\treturn fmt.Errorf("FINDING_CERTIFY_REQUEST_READ_FAILED: %w", err)\n\t}\n\tvar req findingCertifyRequest160',
    '\trequestBytes, err := os.ReadFile(cleanInput)\n\tif err != nil {\n\t\treturn fmt.Errorf("FINDING_CERTIFY_REQUEST_READ_FAILED: %w", err)\n\t}\n\tif err := requestcontract.Validate("finding-certify-request.schema.json", requestBytes); err != nil {\n\t\treturn fmt.Errorf("FINDING_CERTIFY_REQUEST_SCHEMA_INVALID: %w", err)\n\t}\n\tvar req findingCertifyRequest160',
)

replace_once(
    ".code-harness/tools-runtime/cmd/codea-dcep-tools/report.go",
    '\t"codea-harness-tools/internal/analysis"\n\t"codea-harness-tools/internal/report"',
    '\t"codea-harness-tools/internal/analysis"\n\t"codea-harness-tools/internal/requestcontract"\n\t"codea-harness-tools/internal/report"',
)
replace_once(
    ".code-harness/tools-runtime/cmd/codea-dcep-tools/report.go",
    '\tdata, err := os.ReadFile(cleanInput)\n\tif err != nil {\n\t\treturn fmt.Errorf("read review report request: %w", err)\n\t}\n\tproposal, err := decodeReviewTransport153(data)',
    '\tdata, err := os.ReadFile(cleanInput)\n\tif err != nil {\n\t\treturn fmt.Errorf("read review report request: %w", err)\n\t}\n\tif err := requestcontract.Validate("report-review-request.schema.json", data); err != nil {\n\t\treturn fmt.Errorf("REPORT_REVIEW_REQUEST_SCHEMA_INVALID: %w", err)\n\t}\n\tproposal, err := decodeReviewTransport153(data)',
)

common_contract = r'''
## 1.6.2 Reliability Hotfix — Complete Review Invocation Contract

正式 `harness review` 的 Active Runtime command set 是一个整体，Agent/Orchestrator 不得因为旧白名单遗漏而声称 Runtime 缺少 Finding Certification 或 Report 接口：

```text
codea-dcep-tools.exe review options --input .code-harness/runs/<runId>/requests/<file>.json
codea-dcep-tools.exe review select --input .code-harness/runs/<runId>/requests/<file>.json
codea-dcep-tools.exe review units --run-id <runId>
codea-dcep-tools.exe review dispatch --run-id <runId>
codea-dcep-tools.exe review certify-findings --input .code-harness/runs/<runId>/requests/<file>.json
codea-dcep-tools.exe report review --input .code-harness/runs/<runId>/requests/<file>.json
```

在 Agent 创建正式 request 之前必须先读取对应 machine-readable contract：

```text
finding-certify-request.json → .code-harness/contracts/finding-certify-request.schema.json
report-review.json           → .code-harness/contracts/report-review-request.schema.json
```

`review-output.schema.json` 不是 `report review` 的 Agent-facing request contract。正式 report request 的 `findings` 固定为 `[]`；Agent raw Finding 只能进入 `requests/finding-proposals.json`，正式 Finding 必须由 Runtime `review certify-findings` 生成 same-run `analysis/certified-findings.json` + `certified-findings.cert.json` 后再由 `report review` 加载。

`changedFiles=[]` 不是提前成功返回条件。0 Change 仍必须执行 `review units → review dispatch → finding-proposals.json=[] → review certify-findings → report review`，并生成 0 Change / 0 Finding 的正式 `review.md`。
'''

for path in [
    ".code-harness/AGENTS.md",
    ".code-harness/agents/orchestrator.md",
    ".code-harness/agents/reviewer.md",
]:
    append_once(path, "## 1.6.2 Reliability Hotfix — Complete Review Invocation Contract", common_contract)

tools_extra = common_contract + r'''

### `report review` canonical Agent-facing requests

FULL：

```json
{
  "runId": "<runId>",
  "harnessVersion": "1.6.2",
  "baseRef": "<baseRef>",
  "head": "<headCommit>",
  "result": "PASSED",
  "mode": "FULL",
  "reviewScope": {
    "changedFiles": []
  },
  "reviewCoverage": {
    "reviewedFiles": [],
    "callChains": [],
    "externalDependencies": [],
    "unresolved": [],
    "missingReviewedFiles": [],
    "runtimeErrors": [],
    "status": "COMPLETE"
  },
  "findings": []
}
```

TARGETED：

```json
{
  "runId": "<runId>",
  "harnessVersion": "1.6.2",
  "baseRef": "<baseRef>",
  "head": "<headCommit>",
  "result": "PASSED",
  "mode": "TARGETED",
  "target": {
    "symbol": "OrderController.create",
    "kind": "METHOD"
  },
  "reviewScope": {
    "changedFiles": ["src/main/java/acme/OrderController.java"],
    "scopedFiles": ["src/main/java/acme/OrderController.java"]
  },
  "reviewCoverage": {
    "reviewedFiles": ["src/main/java/acme/OrderController.java"],
    "callChains": [
      {
        "entryPoint": "OrderController.create",
        "chain": ["OrderController.create"]
      }
    ],
    "externalDependencies": [],
    "unresolved": [],
    "missingReviewedFiles": [],
    "runtimeErrors": [],
    "status": "COMPLETE"
  },
  "findings": []
}
```

`harnessVersion / baseRef / head / reviewScope / reviewCoverage` 的最终正式值仍由 Runtime 根据 same-run Certified ChangeAnalysis/verified scope 重建；上述 transport 不能覆盖 Runtime Authority。`findings` 只能为空，正式问题清单只来自 same-run Certified Findings。
'''
append_once(
    ".code-harness/tools/README.md",
    "## 1.6.2 Reliability Hotfix — Complete Review Invocation Contract",
    tools_extra,
)

print("TASK162_REVIEW_RELIABILITY_TASK1_APPLY PASS")
