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

以下规则叠加在原执行规则之上：

1. 输入额外包含 validated `TestTargetSelection`、Orchestrator 交付的 selected-only test classes，以及每个 class 的 `origin = REUSED_EXISTING | GENERATED_BY_PLAN`。
2. 每个待执行 test class 必须能追溯到 `selectedControllerIds` 对应 target；仅属于未选择 Controller 的 proposed execution → `SCOPE_VIOLATION`，不得调用 `run_maven_test`。
3. `REUSE_EXISTING`：直接执行，无写操作审批；若失败，历史 Existing Test 永不自动修改。
4. `GENERATED_BY_PLAN`：只有对应 Test Plan 已获精确 `批准 <planId>` 才可执行新生成/修改版本；若 Diagnosis 为 `REPAIR_TEST`，最多自动修复 2 轮。
5. Runtime Debugger 不得“为了完整”把未选择 Controller 测试类重新加入执行列表。
6. 执行失败后仍按现有 Runtime Debugger → `analyze-failure` 流程诊断；需要时使用受控 DB/code evidence，不改变本 Skill 的 selected-only 范围。
