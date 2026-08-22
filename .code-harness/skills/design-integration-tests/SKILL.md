---
name: design-integration-tests
description: 设计以 Controller 为入口的 Spring Boot 集成测试；先做 Existing Test 行为覆盖分析，写入型计划在审批前生成 exact unified diff 与 base hashes。
version: 2
agent: integration-test-agent
tools:
  - list_project_tree
  - read_code
output_schema: .code-harness/contracts/test-plan.schema.json
---

# 设计集成测试

## 目标

根据 ChangeAnalysis、Review Finding 和已经机器验证的 TestTargetSelection，只针对 selected targets 做 Existing Test Coverage Analysis，并按：

```text
REUSE_EXISTING
EXTEND_EXISTING
CREATE_NEW
```

选择最小测试策略。

Task 4 增量规则：**任何会写测试代码的 EXTEND_EXISTING / CREATE_NEW 计划，必须在请求人工批准之前就生成最终 exact patch identity**：

```text
planId
unifiedDiff
diffSha256
files[].path
files[].baseSha256
```

审批的是这份 exact patch，不是一个可在批准后自由变化的自然语言意图。

## 输入

- 已通过 `change-analysis.schema.json` + Runtime Coverage Gate 的 ChangeAnalysis
- 已通过 `test-target-selection.schema.json` + Runtime `selection.VerifyJSON` 的 TestTargetSelection
- 只包含 `selectedControllerIds` 对应的 affectedControllers
- Review findings（尤其 `needsTest=true`）
- 已有测试代码、测试配置、项目已有 Mock/fixture/assertion 约定

## 允许工具

- `list_project_tree`
- `read_code`

本 Skill **不写文件**。正式测试代码写入只允许 `codea-harness-tools apply --input ...`。

## 执行步骤

### 1. Selected-only 范围

只允许 validated `selectedControllerIds` 对应 target 进入分析。未选择 Controller：

- 不做 Existing Test Coverage Analysis；
- 不进入 Test Plan；
- 不产生测试 patch；
- 不进入 Runtime execution。

输入中出现越界 target → `SCOPE_VIOLATION` / STOP。

### 2. 追踪真实行为

对每个 selected endpoint 读取实际 Controller/Service/Repository/Mapper 调用链与外部边界，定义本次变化需要验证的行为：

- 正常路径；
- 错误/边界路径；
- 权限、tenant、幂等；
- 状态流转；
- 异常处理；
- 事务；
- 数据库写入。

### 3. Existing Test Coverage Analysis

在 `scope.testIncludes` 中查找现有测试并读取代码，逐个行为映射到具体 test method：

```text
COVERED
MISSING
```

COVERED 至少需要：

1. 能执行到本次变更调用链；
2. 前置条件触发实际修改路径；
3. 有能证明行为正确的断言；
4. DB/state risk 有对应 DB/状态证据；
5. 权限/tenant/幂等/事务等风险被实际验证。

不能用文件名、同名方法或代码行覆盖率代替行为证据。

### 4. Strategy

每个 target 独立决定：

```text
全部 COVERED                 → REUSE_EXISTING
部分 MISSING                 → EXTEND_EXISTING
无合适 Existing Test         → CREATE_NEW
```

REUSE_EXISTING：

- 无 planId；
- 无 unifiedDiff/diffSha256/files；
- 无审批；
- 零写入；
- 直接返回 Existing Test method/scenario provenance。

EXTEND_EXISTING：

- 只允许修改 `existingTests` 指定文件；
- 只新增/修改本次 `MISSING` 场景；
- 既有 COVERED method 与断言 byte-level 设计上应保持不动；
- 禁止另建重复测试类。

CREATE_NEW：

- 只有没有合理 Existing Test 时才能新建；
- 目标必须位于 `harness.yaml.write.allowedTestPaths`；
- 新文件的 `baseSha256 = SHA256(empty bytes)`，Runtime 还会确认目标当前不存在。

### 5. DB Assertion

对 `databaseWrite / transactional / stateTransition` 风险必须显式判断 DB Assertion 是否需要。

需要时填写具体 `expected.databaseAssertions[]`，例如：

```text
order_info.status == APPROVED for fixture order_id
audit_log contains exactly one APPROVE record
rollback keeps order_info.status == PENDING
```

生成代码时只能复用：

```text
existing helper/repository pattern
→ existing JdbcTemplate pattern
→ existing fixture/assertion utility
```

不得新增 Maven dependency。DB assertion 必须在 cleanup/rollback 隐藏状态之前执行。

### 6. 写入型计划在审批前生成 exact patch

对 EXTEND_EXISTING / CREATE_NEW，先基于当前已读取文件生成**最终计划写入后的完整测试代码**，但不要写磁盘。

然后构造标准 UTF-8 unified diff：

```text
EXTEND_EXISTING:
current exact bytes → proposed exact bytes

CREATE_NEW:
/dev/null → proposed exact bytes
```

对每个目标：

```text
files[].path
files[].baseSha256 = SHA256(current exact bytes)
```

其中新文件 current bytes 为空。

再计算：

```text
diffSha256 = SHA256(unifiedDiff exact UTF-8 bytes)
```

最终写入型 Test Plan 必须包含：

```text
planId
unifiedDiff
diffSha256
files[].path/baseSha256
targets[]
```

并通过 `.code-harness/contracts/test-plan.schema.json`。

### 7. 人工审批

只对写入型计划提示：

```text
请回复：批准 <planId>
```

用户批准时确认的是当前计划内 `diffSha256` 对应的 exact unifiedDiff。

任何变化，包括：

- 修改测试方法；
- 修改断言；
- 调整空白导致 diff bytes 变化；
- 目标文件发生变化；
- base hash 变化；

都必须生成新 patch identity；原批准不得复用。

批准后由 Generate Skill 生成 `planType=TEST` 的 apply request，并调用：

```text
.code-harness/bin/codea-harness-tools apply --input .code-harness/runs/<runId>/requests/apply.json
```

## Method-level provenance

计划必须能够在 apply 成功后形成 method/scenario 级 provenance：

```text
REUSED_EXISTING
GENERATED_BY_PLAN + planId
```

同一个 EXTEND_EXISTING class 可以同时存在两种 origin，禁止整类一刀切。

如果一个 Existing Test class 同时覆盖 selected + unselected Controllers：

- 只允许 selected method selector；
- 无法安全表达 method selector → `MANUAL_ACTION_REQUIRED`；
- 禁止退化成整类执行。

## Repair 边界

历史 Existing Test method 永不自动修改。

对 `GENERATED_BY_PLAN` 方法发生 TEST_ERROR：仍保留 method-level provenance 和最多 2 轮 repair 计数，但**每个实际写入的新 repair patch 都是新的 patch identity**。若 repair 改变 `unifiedDiff`，必须生成新的 `planId/diffSha256/baseSha256` 并重新获得精确批准；原批准不能授权不同 bytes。

## 输出

REUSE_EXISTING：只输出完整覆盖分析 + method provenance。

EXTEND/CREATE：输出通过 `test-plan.schema.json` 的计划，包含：

- targets / strategy / scenarios / coveredBy
- request / expected / databaseAssertions（适用时）
- `planId`
- `unifiedDiff`
- `diffSha256`
- `files[].path/baseSha256`

## 禁止行为

- 不得分析未选择 Controller。
- 不得在审批前写测试文件。
- 不得在审批后重新生成不同补丁并沿用旧 planId。
- 不得默认 Mock 内部 Service/Repository Bean。
- 不得弱化 Existing Test 断言、删除测试或添加 `@Disabled`。
- 不得用 `write_test` / arbitrary direct host write 作为正式写入路径。
- 正式写入只能 `planType=TEST` → `codea-harness-tools apply --input` → Apply evidence。
