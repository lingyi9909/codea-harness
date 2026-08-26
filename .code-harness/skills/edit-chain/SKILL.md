---
name: edit-chain
description: 对已确认 Business Chain 执行受控语义编辑；Agent 只提交 operation proposal，Runtime 基于 same-run Certified ChangeAnalysis 生成 EDIT candidate，最终保存复用不可变 write plan。
version: 1
agent: orchestrator
tools:
  - read_code
---

# Semantic Chain Editing

## 用户意图

```text
harness chain edit <id|Controller|Controller.method>
```

只允许 exact id / exact Controller / exact Controller.method 解析目标；多条命中必须报告歧义，不得 fuzzy 选择。EntryPoint identity 在 1.5.3 Task 5 不允许通过 edit 改写。

## Authority boundary

Generic Agent / Orchestrator 只能写同 run 的 `requests/**` proposal。Agent 不得直接创建、覆盖或修改 `.code-harness/runs/<runId>/analysis/**`、`.code-harness/runs/<runId>/analysis/edit-candidates/**` 或 `.code-harness/chains/**`。

代码事实必须来自 same-run Certified ChangeAnalysis；dependency workspace 只能用于 navigation，不能成为 current Project State 写 authority。

## 允许的 operation

```text
REPLACE_NODE
ADD_NODE
REMOVE_NODE
REORDER_NODE
RENAME_CHAIN
UPDATE_NOTES
```

`REPLACE_NODE / ADD_NODE / REMOVE_NODE / REORDER_NODE` 必须由 Runtime 验证最终完整 ordered core path；不能逐 operation 放行后拼出未验证关系。`RENAME_CHAIN / UPDATE_NOTES` 只能改变描述性元数据。禁止 ADD/REMOVE/REPLACE EntryPoint。

## 固定流程

```text
现有 ACCEPTED Project State Chain
→ same-run Certified ChangeAnalysis
→ requests/chain-edit.json
→ codea-harness-tools chain edit --input ...
→ Runtime 验证六类 operation + 最终完整 Chain
→ analysis/edit-candidates/<id>.yaml + provenance(kind=EDIT)
→ 展示 deterministic diff
→ 用户首次保存意图
→ chain seal-persist
→ 展示 exact preview + planId
→ 用户明确确认当前 planId
→ chain persist(runId + planId only)
→ atomic 写入 .code-harness/chains/<id>.yaml
```

`chain edit` 本身不得直接修改 `.code-harness/chains/**`。candidate / Certified Analysis / sealed plan / existing Project State 任一变化都使旧 planId 失效，必须重新 seal 并重新确认。

## Fail closed

request 不在 same-run `requests/**`、ChangeAnalysis 未认证/已变化、operation 不在 allowlist、最终 node/order/symbol/path/role 没有 exact certified fact、dependency workspace 被当成写 authority、EDIT candidate 被篡改、或最终确认不是当前 exact planId 时，必须拒绝并产生 0 Project State writes。
