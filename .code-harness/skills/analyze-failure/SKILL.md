---
name: analyze-failure
description: 关联测试输出、Surefire 报告、堆栈跟踪和日志，产出带确定 nextAction 的故障诊断结果。
version: 1
agent: runtime-debugger
tools:
  - read_test_report
  - read_service_logs
output_schema: .code-harness/contracts/diagnosis.schema.json
---

# 分析故障

## 目标

关联所有执行输出——Maven stdout/stderr、Surefire 报告、堆栈跟踪和运行窗口内的应用日志——对故障进行分类，并建议下一步动作。

## 适用场景

- `run-integration-tests` 返回非零退出码或存在失败测试断言后
- `debug-local-service` 检测到启动失败或运行异常后
- Orchestrator 需要在执行后决定下一步时

## 不适用场景

- 所有测试都已通过且服务健康
- 当前运行中已经诊断过该故障

## 输入

- Maven stdout/stderr（来自 `run_maven_test`）
- Surefire XML/TXT 报告（来自 `read_test_report`）
- 运行窗口内的应用日志（来自 `read_service_logs`）
- 执行模式：`integration-test` 或 `service-debug`

## 允许使用的工具

- `read_test_report`——必要时重新读取报告
- `read_service_logs`——必要时调整时间窗口重新读取日志

## 前置条件

- 测试执行或服务启动已完成（无论成功或失败）
- 原始输出可供分析

## 执行步骤

1. **检查编译错误**：扫描 stdout/stderr 中的编译失败标记。如找到 → `TEST_COMPILE_ERROR`，`nextAction: REPAIR_TEST`。
2. **检查断言失败**：解析 Surefire XML 中的 `<failure>` 或 `<error>` 元素。提取断言消息和堆栈。如果失败出自测试断言逻辑 → `TEST_CODE_ERROR`，`nextAction: REPAIR_TEST`。
3. **检查 Spring 上下文失败**：查找 `ApplicationContext` 加载失败、Bean 定义缺失、配置错误。
   - **集成测试模式下** → `TEST_CONTEXT_ERROR`，`nextAction: REPAIR_TEST`
   - **服务调试模式下** → `SERVICE_START_ERROR`，`nextAction: RESTART_SERVICE` 或 `GENERATE_FIX_PLAN`
4. **检查数据/环境问题**：查找连接拒绝、未知主机、表缺失、外部服务认证失败 → `TEST_DATA_OR_ENVIRONMENT_ERROR`，`nextAction: REPORT_ENVIRONMENT`。
5. **检查生产代码缺陷**：如果测试本身正确，但生产代码返回错误结果、违反业务规则或有未处理的边界情况 → `PRODUCTION_CODE_ERROR`，`nextAction: GENERATE_FIX_PLAN`。
6. **兜底**：如果没有明确模式匹配 → `UNKNOWN`，`nextAction: STOP_UNKNOWN`。附带所有原始证据。

### 分类优先级（集成测试模式）
1. `TEST_COMPILE_ERROR`——最高优先级，首先检查
2. `TEST_CONTEXT_ERROR`——Spring 上下文失败（**不是** `SERVICE_START_ERROR`）
3. `TEST_DATA_OR_ENVIRONMENT_ERROR`——外部连通性
4. `TEST_CODE_ERROR`——断言失败
5. `PRODUCTION_CODE_ERROR`——业务逻辑缺陷
6. `UNKNOWN`——兜底

### 分类优先级（服务调试模式）
1. `SERVICE_START_ERROR`——启动失败（仅此模式有效）
2. `TEST_DATA_OR_ENVIRONMENT_ERROR`——外部连通性
3. `PRODUCTION_CODE_ERROR`——运行时业务逻辑缺陷
4. `UNKNOWN`——兜底

**关键规则**：`SERVICE_START_ERROR` 仅在服务调试模式下有效。集成测试模式下，`@SpringBootTest` 的 Spring Boot 启动失败归类为 `TEST_CONTEXT_ERROR`。

## 输出

必须通过 `.code-harness/contracts/diagnosis.schema.json` 校验。
- `classification`：恰好一个枚举值
- `rootCause`：具体、可操作的描述
- `evidence`：具体日志行、堆栈或报告摘录列表
- `nextAction`：恰好一个枚举值

## 停止条件

- 证据不足以分类 → 分类为 `UNKNOWN`，包含所有可用证据

## 禁止行为

- 不得修改测试代码或生产代码
- 不得重新运行测试——那是 `run-integration-tests` 的职责
- 不得猜测分类——始终引用证据

## 示例

```json
{
  "classification": "PRODUCTION_CODE_ERROR",
  "rootCause": "OrderService.approve() 在将状态流转为 APPROVED 前未校验当前订单状态",
  "evidence": [
    "Surefire：shouldApproveOrder 失败——期望 200，实际 500",
    "应用日志：OrderService.java:42 抛出 IllegalStateException——订单状态已是 CANCELLED",
    "测试发送的请求 orderId=1，该订单在测试数据库中状态为 CANCELLED"
  ],
  "nextAction": "GENERATE_FIX_PLAN"
}
```
