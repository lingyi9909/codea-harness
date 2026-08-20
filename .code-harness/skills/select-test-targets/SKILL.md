---
name: select-test-targets
description: 在 Review Coverage COMPLETE 后，根据 ChangeAnalysis.affectedControllers 确定本次 harness test 的 Controller 范围；多目标必须经宿主交互选择，并产出机器验证的 TestTargetSelection artifact。
version: 1
agent: orchestrator
tools:
  - validate_contract
output_schema: .code-harness/contracts/test-target-selection.schema.json
---

# 选择测试目标

## 输入前提

仅在以下条件全部满足后执行：

1. 当前意图是 `harness test`。
2. `ChangeAnalysis` 已通过 `change-analysis.schema.json`。
3. Runtime 机器 Review Coverage 为 `COMPLETE`。

Selection 必须发生在 Existing Test Coverage、Test Plan 和测试执行之前。

## Host Interaction Contract

逻辑交互固定为：

```text
request_test_target_selection(
  selectionId,
  options[],
  multiple=true,
  shortcuts=[ALL, DIRECT_ONLY]
)
```

每个 option 至少包含：

```json
{
  "id": "controller:OrderController",
  "label": "OrderController",
  "endpoints": ["POST /order/approve"],
  "impactType": "DIRECT_CHANGE",
  "recommended": true
}
```

宿主支持结构化多选 UI 时优先使用；不支持时必须降级为编号选择，例如 `1,3`、`ALL` 或 `DIRECT_ONLY`。不得要求用户输入 Controller 名称。

## 确定性状态机

```text
affectedControllers = 0
→ NO_TEST_TARGET
→ DONE

affectedControllers = 1
→ AUTO_SINGLE
→ 不询问用户
→ 生成 selection artifact
→ Runtime validate_contract
→ TEST_TARGETS_SELECTED

affectedControllers >= 2
→ WAITING_TEST_SELECTION
→ native multi-select 或 numbered fallback
→ 用户确认范围
→ 生成 selection artifact
→ Runtime validate_contract
→ TEST_TARGETS_SELECTED

用户取消
→ status=CANCELLED
→ 持久化并验证 artifact
→ STOP
```

多 Controller 时禁止默认全选。

快捷选择：

- `ALL`：选择全部 available Controller，mode=`USER_ALL`。
- `DIRECT_ONLY`：仅选择 `impactType=DIRECT_CHANGE`，mode=`USER_DIRECT_ONLY`。
- 编号 fallback：mode=`FALLBACK_NUMBERED`。
- 普通结构化多选：mode=`USER_MULTI`。

## Artifact

结果持久化到：

```text
.code-harness/runs/<runId>/test-target-selection.json
```

并必须使用：

```text
validate_contract(
  schema=.code-harness/contracts/test-target-selection.schema.json,
  input=.code-harness/runs/<runId>/test-target-selection.json
)
```

Runtime 除 JSON Schema 外还会执行 `selection.VerifyJSON`，机器校验：

- selectedControllerIds 无重复；
- availableControllerIds 无重复；
- `selected ⊆ available`；
- `AUTO_SINGLE` 必须恰好 1 个 available + 1 个 selected；
- `SELECTED` 必须至少选择 1 个 Controller。

验证失败不得进入 Integration Test Agent。

## Selection 与 Approval 严格分离

```text
TestTargetSelection
→ 只回答“测哪个 Controller”

批准 <planId>
→ 只授权本次 Test Plan 中的测试代码写入

批准 <fixPlanId>
→ 只授权对应 Fix Plan 的生产代码修改
```

任何 Selection 操作、`ALL`、`DIRECT_ONLY`、编号选择或宿主 UI 确认都不能替代 `planId` / `fixPlanId` 精确审批。

## 输出给下游

只传递：

```text
ChangeAnalysis
+ validated TestTargetSelection
+ selected affectedControllers
```

未选择 Controller 不得进入 Existing Test Coverage、Test Plan 或 Runtime execution。
