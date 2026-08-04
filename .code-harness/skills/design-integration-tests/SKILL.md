---
name: design-integration-tests
description: 设计以 Controller 为入口的 Spring Boot 集成测试。将受影响的 Controller 映射到真实 Service/Repository 调用链，识别外部 Mock，产出需人工审批的测试计划。
version: 1
agent: integration-test-agent
tools:
  - read_code
output_schema: contracts/test-plan.schema.json
---

# 设计集成测试

## 目标

根据变更分析和评审发现，设计以 Controller 为入口的集成测试。将受影响的 Controller 映射到真实 Service 和 Repository 调用链，识别需要 Mock 的外部依赖，产出符合 Schema 的测试计划供人工审批。

## 适用场景

- `harness review` 完成且评审发现可用之后
- `harness test` 进入测试计划阶段时
- Integration Test Agent 收到 Reviewer 的变更分析后

## 不适用场景

- 尚未产出变更分析——先执行 `analyze-change`
- 变更中没有受影响的 Controller
- 测试计划已审批通过且测试已生成

## 输入

- `analyze-change` 产出的变更分析（通过 `change-analysis.schema.json` 校验）
- `review-code` 产出的评审发现（特别是 `needsTest: true` 的条目）
- 目标项目已有的测试配置和外部依赖 Mock 方式

## 允许使用的工具

- `read_code`——读取已有测试文件以了解项目约定
- `read_code`——读取 Controller、Service、Repository 源码以追踪调用链

## 前置条件

- 变更分析已完成
- 评审发现可用
- 至少有一个受影响的 Controller

## 执行步骤

1. **识别受影响的 Controller**：从变更分析中列出每个有变更接口或其下游 Service/Repository 链被修改的 Controller。
2. **追踪真实调用链**：对每个受影响的接口，读取 Controller 方法并追踪：
   - 调用了哪些 Service 方法
   - 这些 Service 调用了哪些 Repository/Mapper 方法
   - 调用了哪些外部系统（RPC、MQ、第三方接口、缓存）
3. **确定 Mock 策略**：对每个外部依赖，识别项目在测试中已有的 Mock 或替代方式（如 `@MockBean`、测试配置、WireMock、Testcontainers）。默认**不 Mock** 项目内部的 Service 或 Repository Bean。
4. **设计场景**：对每个接口至少设计：
   - **正常路径**：有效请求，预期 2xx 响应，正确的状态流转和数据库效果
   - **错误场景**：无效输入，预期 4xx 响应及错误体
   - **边界场景**：边界值、空输入、重复请求（如涉及幂等性）
5. **定义前置条件**：对每个场景指定：
   - 需要的数据库状态（优先通过 Controller 请求创建，或使用已有测试数据）
   - 外部 Mock 的响应
   - 认证/租户上下文
6. **定义预期结果**：对每个场景指定：
   - HTTP 状态码
   - 响应体断言（关键字段，不必穷举）
   - 数据库状态断言（插入/更新/删除的行、具体列值）
   - 状态流转（from → to）
7. **生成 planId**：分配唯一 ID，如 `test-plan-YYYYMMDD-NNN`。
8. **呈现等待审批**：计划初始为未审批状态。提示用户：「请回复：批准 <planId>」。

## 输出

必须通过 `contracts/test-plan.schema.json` 校验。关键字段：
- `planId`：计划唯一标识
- `targets[]`：每个接口——controller、endpoint、serviceChain、repositoryChain、externalMocks、scenarios

## 停止条件

- 没有受影响的 Controller → 报告后停止
- 无法确定外部依赖的 Mock 策略 → 在计划中标记为待确认，不要猜测

## 禁止行为

- 不得在计划审批通过前编写任何测试文件
- 不得默认 Mock 项目内部的 Service 或 Repository Bean
- 不得设计访问生产数据或系统的测试
- 不得为了让测试更容易通过而弱化断言

## 示例

```json
{
  "planId": "test-plan-20260804-001",
  "targets": [{
    "controller": "OrderController",
    "endpoint": "POST /api/order/approve",
    "serviceChain": ["OrderService"],
    "repositoryChain": ["OrderRepository"],
    "externalMocks": ["OrderRpcClient"],
    "scenarios": [{
      "name": "正常审批待处理订单",
      "preconditions": ["存在 id=1 且状态为 PENDING 的订单"],
      "request": {
        "method": "POST",
        "path": "/api/order/approve",
        "headers": {"Content-Type": "application/json", "X-Tenant-Id": "t1"},
        "body": {"orderId": 1}
      },
      "expected": {
        "httpStatus": 200,
        "responseAssertions": ["$.status == 'APPROVED'"],
        "databaseAssertions": ["order(id=1).status = 'APPROVED'"],
        "stateTransition": {"from": "PENDING", "to": "APPROVED"}
      }
    }]
  }]
}
```
