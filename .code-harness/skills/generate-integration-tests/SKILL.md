---
name: generate-integration-tests
description: 对已批准的写入型测试计划只执行审批前 Runtime-sealed exact request；正式测试写入由 Controlled Runtime Apply Safety Gate 完成。
version: 2
agent: integration-test-agent
tools:
  - read_code
output_schema: null
---

# 应用已批准的集成测试补丁

## 目标

本 Skill 不再在审批之后自由生成测试代码或 Runtime request。`design-integration-tests` 已经在**审批前**确定最终：

```text
unifiedDiff
diffSha256
files[].path/baseSha256
.code-harness/runs/<runId>/requests/apply.json
.code-harness/runs/<runId>/sealed-plans/<planId>.json
```

并调用：

```text
.code-harness/bin/codea-harness-tools seal-apply --input .code-harness/runs/<runId>/requests/apply.json
```

用户批准 `planId` 后，本 Skill 只能把**同一份 sealed exact request**交给 Controlled Runtime。

## 输入

- 已精确批准的 Test Plan
- `planId`
- `unifiedDiff`
- `diffSha256`
- `files[].path/baseSha256`
- 审批前 sealed apply request path
- `.code-harness/runs/<runId>/sealed-plans/<planId>.json`
- targets 的 strategy / scenarios / Existing Test provenance
- validated TestTargetSelection

## 前置门禁

1. `REUSE_EXISTING` target 不进入写入流程。
2. 只有存在 EXTEND_EXISTING / CREATE_NEW 才允许 apply。
3. `apply.json` 必须在用户批准前已经通过 `apply-request.schema.json` 和 `codea-harness-tools seal-apply --input`。
4. 对应 `.code-harness/runs/<runId>/sealed-plans/<planId>.json` 必须存在。
5. 用户必须精确回复 `批准 <planId>`。
6. 批准后禁止重新生成、改写或 rebase `apply.json`。变化必须回到设计阶段产生新 planId、新 request、新 seal、新批准。

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

### 2. 不重新生成 approved bytes/request

批准后不得重新“生成一次差不多的代码”，也不得重新构造 request。必须直接使用审批前 sealed 的：

```text
.code-harness/runs/<runId>/requests/apply.json
```

其中 exact identity 必须保持：

```text
planType=TEST
planId
unifiedDiff exact bytes
diffSha256
files[].path
files[].baseSha256
```

如 request 被替换成另一个自洽 Patch B，Runtime 会对照 sealed Patch A 返回 `APPROVAL_IDENTITY_MISMATCH`。如当前源码发生变化，Runtime 会返回 `BASE_CHANGED`。两种情况都不得沿用原批准自行修补。

### 3. Controlled Runtime apply

调用：

```text
.code-harness/bin/codea-harness-tools apply --input .code-harness/runs/<runId>/requests/apply.json
```

Runtime 独立验证：

- `--input` 路径在读取前必须严格匹配 `.code-harness/runs/<runId>/requests/*.json`；
- 读取后 body `runId` 必须与 path runId 一致；
- sealed snapshot 存在；
- `planId / planType / unifiedDiff exact bytes / diffSha256 / files[].path / files[].baseSha256` 与 sealed snapshot 完全一致；
- request Schema；
- exact diff hash；
- 当前 base file hash；
- patch touched set；
- `allowedTestPaths`；
- Runtime hard-deny `.git/**` / `.code-harness/**`；
- 用户 `deniedPaths`；
- traversal/binary/unsafe patch；
- 原子写入/rollback；
- duplicate planId。

TEST 计划无法写生产路径；Runtime hard-deny 不能被 `allowedTestPaths=["**"]` 或 `deniedPaths=[]` 覆盖。

### 4. 正式成功条件

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

`SEALED_PLAN_NOT_FOUND`、`APPROVAL_IDENTITY_MISMATCH` 或任何 Runtime reject/rollback → 不得报告成功，也不得改用 `write_test` / direct host write 兜底。

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
→ new apply request
→ 审批前 codea-harness-tools seal-apply --input
→ 新的精确批准
→ 同一 sealed request 的 planType=TEST Runtime apply
```

旧批准不得授权新的 repair patch。

## DB Assertion

如果 approved MISSING scenario 含 `expected.databaseAssertions[]`，补丁必须真实包含对应断言，并沿用已有 helper/repository/JdbcTemplate/fixture pattern；不得新增 Maven dependency。断言必须发生在 cleanup/rollback 隐藏状态之前。

## 禁止行为

- 不得调用 `write_test` 作为正式成功路径。
- 不得在批准后重算不同 patch/request 后继续使用旧 planId。
- 不得跳过审批前 sealed plan baseline。
- 不得写生产代码。
- 不得触及 unselected target。
- 不得删除/禁用/弱化 Existing Test。
- 不得自行执行测试、Shell、commit/push/PR。
