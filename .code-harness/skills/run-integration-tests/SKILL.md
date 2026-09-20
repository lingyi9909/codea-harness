---
name: run-integration-tests
description: 执行指定的 Maven 集成测试，采集 Surefire 报告和日志。
version: 1
agent: runtime-debugger
tools:
  - run_maven_test
  - read_test_report
output_schema: null
---

# 执行集成测试

## 目标

通过配置的 Maven 命令执行指定集成测试类，强制执行超时，采集所有执行输出供下游诊断使用。

## 适用场景

- Integration Test Agent 生成或修复测试类之后
- Orchestrator 将测试执行交给 Runtime Debugger 时
- 用户明确要求运行指定测试类

## 不适用场景

- 测试计划尚未审批通过
- 测试类文件不存在

## 输入

- `testClass`：测试类全限定名（如 `com.example.OrderControllerIT`）
- `runId`：唯一运行标识，用于产物存储

## 允许使用的工具

- `run_maven_test`——执行配置的 Maven 命令
- `read_test_report`——读取 stdout/stderr 和 Surefire XML/TXT

## 前置条件

- `harness.yaml` 中 `integrationTest.executable` 和 `integrationTest.args` 配置有效
- 测试类存在于 `scope.testIncludes` 范围内
- 配置的 `integrationTest.timeoutSeconds` 可接受

## 执行步骤

1. **构造命令**：将配置的 `integrationTest.args` 中的 `${testClass}` 替换为实际测试类全限定名。执行前完整展示 `executable` + `args`。
2. **执行**：调用 `run_maven_test(testClass, runId)`。执行配置的 executable 和替换后的 args。不经过 Shell 求值、管道、重定向或命令链接。
3. **检查退出码**：记录 Maven 进程退出码（0 表示成功）。
4. **采集 stdout/stderr**：完整进程输出被采集供诊断使用。
5. **读取 Surefire 报告**：调用 `read_test_report(runId)` 读取配置的 `reportDir` 下的 XML 和 TXT 报告。
6. **返回结果**：将所有采集的输出传递给 `analyze-failure` 进行分类。

## 输出

- Maven 退出码
- 完整 stdout/stderr
- Surefire XML 和 TXT 报告内容（针对失败的测试）

## 停止条件

- Maven executable 不存在 → 报告后停止
- 超时 → 报告为 `TEST_DATA_OR_ENVIRONMENT_ERROR`
- 没有找到 Surefire 报告 → 标记后继续，仅使用 stdout/stderr

## 禁止行为

- 不得构造任意 Shell 命令——只能使用配置的 executable 和 args
- 不得运行审批通过的计划之外的测试
- 不得修改测试代码或生产代码
- 不得直接运行 `mvn`——始终使用 `run_maven_test`

## 示例

```
输入：testClass=com.example.OrderControllerIT, runId=run-20260804-001
命令：./mvnw -Dspring.profiles.active=test -Dtest=com.example.OrderControllerIT test
退出码：1
Surefire：共 8 个测试，6 个通过，2 个失败
```

## Task 7：Selected-only Execution Gate

以下规则叠加在原执行规则之上，并覆盖此前任何 class 级 selected-only / origin 表述：

### TestExecutionTarget handoff

Runtime Debugger 不再只接收裸 `testClass` 列表。Orchestrator 必须为每个可执行单元提供：

```text
TestExecutionTarget
- testClass
- testMethods[]        # 空数组仅表示“允许整类执行”
- selector             # testClass 或受控 Maven/Surefire 可表达的 Class#method / 方法集合
- controllerId
- origin = REUSED_EXISTING | GENERATED_BY_PLAN
- planId(optional)     # GENERATED_BY_PLAN 时必须可追溯到批准的 planId
```

`origin` 属于具体 method/scenario 的 provenance，不属于整个 class。`EXTEND_EXISTING` 的同一 class 可以同时包含 REUSED_EXISTING 与 GENERATED_BY_PLAN 方法，必须拆成不同 TestExecutionTarget。

### Selected-only 执行粒度

1. 每个 TestExecutionTarget 都必须追溯到 validated `selectedControllerIds`；任何 unselected controller 对应的方法都不得进入 selector。
2. **整类执行只允许在该 class 的本次相关测试全部属于 selected targets 时使用。** 如果一个 class 同时覆盖 selected + unselected targets，则禁止用裸 class selector 执行。
3. 混合 class 必须收窄到 selected methods，例如 `CommonControllerIT#shouldApproveOrder`；如果当前 `harness.yaml.integrationTest.args` / Maven Surefire 配置无法安全表达所需 method selector，则返回 `SCOPE_VIOLATION` / `MANUAL_ACTION_REQUIRED`，不得退化成整类执行。
4. `run_maven_test` 仍只使用现有受控入口：把已验证的 `selector` 作为 `${testClass}` 的值交给固定 Maven args；禁止 Shell 求值或用户命令拼接。
5. Runtime Debugger 不得为了完整性把 unselected method、class 或 Controller 补回执行范围。

### Method-level origin / repair provenance

1. `REUSED_EXISTING` method：直接执行，无写操作审批；若失败，历史 Existing Test 永不自动修改。
2. `GENERATED_BY_PLAN` method：必须带对应已精确批准的 `planId`；只有该方法失败且 Diagnosis=`REPAIR_TEST` 时，才可进入最多 2 轮自动 repair。
3. Surefire `failedTests.testClass + testMethod` 必须回查匹配的 TestExecutionTarget 决定 origin；不能用 class 级标签替代。
4. 若失败方法没有唯一 provenance、`testMethod=null` 且无法安全归因，默认走安全路径：不得自动 repair，返回测试修改计划或 `MANUAL_ACTION_REQUIRED`。
5. 执行失败后仍按现有 Runtime Debugger → `analyze-failure` 流程诊断；需要时使用受控 DB/code evidence，不改变 selected-only 范围。

### Golden blocker cases

```text
Affected = Order + User
Selection = Order

CommonControllerIT:
  shouldApproveOrder() -> Order
  shouldDisableUser()  -> User

允许：CommonControllerIT#shouldApproveOrder
禁止：CommonControllerIT 整类执行
无法表达 method selector -> MANUAL_ACTION_REQUIRED
```

```text
PaymentControllerIT.oldTestA
origin=REUSED_EXISTING

PaymentControllerIT.newMissingTest
origin=GENERATED_BY_PLAN
planId=test-plan-xxx

oldTestA failure -> never auto-edit
newMissingTest failure -> GENERATED_BY_PLAN repair gate, max 2 rounds
```
