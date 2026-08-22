---
name: generate-integration-tests
description: 对已批准的写入型测试计划只执行 exact approved patch handoff；正式测试写入由 Controlled Runtime Apply Safety Gate 完成。
version: 2
agent: integration-test-agent
tools:
  - read_code
output_schema: null
---

# 应用已批准的集成测试补丁

## 目标

本 Skill 不再在审批之后自由生成测试代码。`design-integration-tests` 已经在审批前确定最终：

```text
unifiedDiff
diffSha256
files[].path/baseSha256
```

用户批准 `planId` 后，本 Skill 只能把**同一份 exact patch**交给 Controlled Runtime。

## 输入

- 已精确批准的 Test Plan
- `planId`
- `unifiedDiff`
- `diffSha256`
- `files[].path/baseSha256`
- targets 的 strategy / scenarios / Existing Test provenance
- validated TestTargetSelection

## 前置门禁

1. `REUSE_EXISTING` target 不进入写入流程。
2. 只有存在 EXTEND_EXISTING / CREATE_NEW 才允许 apply。
3. 用户必须精确回复 `批准 <planId>`。
4. 当前计划 bytes 必须仍与批准时的 `diffSha256` 一致。
5. 如果计划自批准后发生任何变化，STOP，回到设计阶段生成新 planId。

## 执行流程

### 1. 保持 Existing Test 保护

EXTEND_EXISTING：

- 只能触及计划 `existingTests` 指定文件；
- 只能实现计划中 MISSING 场景；
- COVERED Existing Test method 不得被删除、禁用、弱化或重写。

CREATE_NEW：

- 只允许计划中已经批准的新文件；
- 不得临时换文件名、包名或目标路径。

未选择 Controller 的测试文件不得出现在 approved patch 中。

### 2. 不重新生成 approved bytes

批准后不得重新“生成一次差不多的代码”。必须直接使用 Test Plan 中的 exact：

```text
unifiedDiff
diffSha256
files[].baseSha256
```

如当前源码发生变化，Runtime 会返回 `BASE_CHANGED`；不得自行 rebase 并沿用原 planId。

### 3. 生成 Runtime request

把 approved identity 原样写入：

```text
.code-harness/runs/<runId>/requests/apply.json
```

Contract：`.code-harness/contracts/apply-request.schema.json`。

```json
{
  "runId":"<runId>",
  "planType":"TEST",
  "planId":"<approved planId>",
  "diffSha256":"<approved diffSha256>",
  "files":[{"path":"...","baseSha256":"..."}],
  "unifiedDiff":"<approved exact unifiedDiff>"
}
```

固定语义：`planType=TEST`。

### 4. Controlled Runtime apply

调用：

```text
.code-harness/bin/codea-harness-tools apply --input .code-harness/runs/<runId>/requests/apply.json
```

Runtime 独立验证：

- request Schema；
- exact diff hash；
- 当前 base file hash；
- patch touched set；
- `allowedTestPaths`；
- `deniedPaths`；
- traversal/binary/unsafe patch；
- 原子写入/rollback；
- duplicate planId。

TEST 计划无法写生产路径。

### 5. 正式成功条件

只有 Runtime 返回：

```text
status=APPLIED
```

且生成：

```text
.code-harness/runs/<runId>/evidence/apply/<planId>.json
```

并符合 `apply-result.schema.json`，才算测试代码修改完成。

Evidence 至少包含：

```text
runId
planType=TEST
planId
diffSha256
appliedAt
files[].path/beforeSha256/afterSha256
rollbackPerformed=false
```

任何 Runtime reject/rollback → 不得报告成功，也不得改用 `write_test` / direct host write 兜底。

## Apply 后 provenance

Runtime APPLIED 后，继续按既有 method/scenario 粒度返回：

```text
历史 Existing Test method → REUSED_EXISTING
本次 approved patch 新增/修改 method → GENERATED_BY_PLAN + planId
```

同一个 EXTEND_EXISTING class 中两种 origin 必须保持分离。

## Repair

只有失败 method `origin=GENERATED_BY_PLAN` 且能唯一追溯 planId 时才可进入 repair 分析；历史 `REUSED_EXISTING` method 永不自动修改。

最多 2 轮 repair 计数继续保留。但只要 repair 产生不同代码 bytes：

```text
new unifiedDiff
→ new diffSha256
→ new files[].baseSha256
→ new planId
→ 新的精确批准
→ planType=TEST Runtime apply
```

旧批准不得授权新的 repair patch。

## DB Assertion

如果 approved MISSING scenario 含 `expected.databaseAssertions[]`，补丁必须真实包含对应断言，并沿用已有 helper/repository/JdbcTemplate/fixture pattern；不得新增 Maven dependency。断言必须发生在 cleanup/rollback 隐藏状态之前。

## 禁止行为

- 不得调用 `write_test` 作为正式成功路径。
- 不得在批准后重算不同 patch 后继续使用旧 planId。
- 不得写生产代码。
- 不得触及 unselected target。
- 不得删除/禁用/弱化 Existing Test。
- 不得自行执行测试、Shell、commit/push/PR。
