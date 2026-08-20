---
name: design-integration-tests
description: 设计以 Controller 为入口的 Spring Boot 集成测试。先做现有测试覆盖分析，按 REUSE_EXISTING / EXTEND_EXISTING / CREATE_NEW 三种策略只补缺失场景，产出需人工审批的测试计划。
version: 1
agent: integration-test-agent
tools:
  - list_project_tree
  - read_code
output_schema: .code-harness/contracts/test-plan.schema.json
---

# 设计集成测试

## 目标

根据变更分析和评审发现，为每个受影响的 Controller 接口设计集成测试。核心是**先做现有测试覆盖分析**：先定义「本次变更需要验证的行为」，再查找并映射现有测试的覆盖情况，只找缺口。按「优先复用，其次补充，最后新建」原则为每个 target 确定策略（REUSE_EXISTING / EXTEND_EXISTING / CREATE_NEW），产出符合 Schema 的测试计划供人工审批。

## 适用场景

- `harness review` 完成且评审发现可用之后
- `harness test` 进入测试设计阶段时
- Integration Test Agent 收到 Reviewer 的变更分析后

## 不适用场景

- 尚未产出变更分析——先执行 `analyze-change`
- 变更中没有受影响的 Controller

## 输入

- `analyze-change` 产出的变更分析（通过 `change-analysis.schema.json` 校验）
- `review-code` 产出的评审发现（特别是 `needsTest: true` 的条目）
- 目标项目已有的测试配置、已有测试文件、外部依赖 Mock 方式

## 允许使用的工具

- `list_project_tree`——列出 `scope.testIncludes` 下的测试目录与测试类
- `read_code`——读取已有测试文件、Controller/Service/Repository 源码

## 前置条件

- 变更分析已完成
- 至少有一个受影响的 Controller
- 能读取 `scope.testIncludes` 下的测试文件

## 执行步骤

1. **识别受影响的 Controller**：从变更分析中列出每个有变更接口或其下游 Service/Repository 链被修改的 Controller。
2. **追踪真实调用链**：对每个受影响的接口，读取 Controller 方法并追踪：
   - 调用了哪些 Service 方法
   - 这些 Service 调用了哪些 Repository/Mapper 方法
   - 调用了哪些外部系统（RPC、MQ、第三方接口、缓存）
3. **定义本次变更需要验证的行为**：对每个受影响的 Endpoint，列出需要验证的行为，覆盖：
   - 正常路径、错误场景、边界场景
   - 本次变更涉及的风险：权限、tenant 隔离、幂等、状态流转、异常处理、事务、数据库写操作
4. **查找现有测试**：用 `list_project_tree` 在 `scope.testIncludes` 下查找测试类。优先按 Controller 名 / endpoint / Service 调用链 / 业务方法名搜索（如 `OrderController` → `OrderControllerIT`、`OrderControllerTest`、`OrderIntegrationTest`）。再用 `read_code` 读取内容确认是否真的相关，不能只按文件名判断。
5. **覆盖映射**：对第 3 步列出的每个「需要验证的行为」，逐项映射到现有测试方法，标记 `COVERED` 或 `MISSING`。是否覆盖必须遵循「覆盖判断标准」（见下），不能用「存在同名测试」或行覆盖率代替。
6. **确定 strategy**（每个 target 独立判定，混合场景不能整体一刀切）：
   - 全部行为 `COVERED` → `REUSE_EXISTING`（不生成修改、不要求审批、不修改代码）
   - 部分行为 `MISSING` → `EXTEND_EXISTING`（计划保留完整覆盖映射，但审批和代码修改范围仅限 `MISSING` 场景，在现有测试类中补充）
   - 找不到合适的现有测试，或现有测试结构与目标完全不匹配 → `CREATE_NEW`
7. **确定 Mock 策略**（EXTEND/CREATE 时）：识别外部依赖的 Mock 或替代方式（`@MockBean`、测试配置、WireMock、Testcontainers），默认**不 Mock** 项目内部的 Service 或 Repository Bean。
8. **生成测试计划**：
   - `REUSE_EXISTING`：输出覆盖分析结论，不生成 `planId`，不等待审批。
   - `EXTEND_EXISTING` / `CREATE_NEW`：生成带唯一 `planId` 的计划，保留完整覆盖映射（COVERED + MISSING），但审批与代码修改范围仅限 `MISSING` 场景；`existingTests` 记录可复用或可扩展的现有测试类。
9. **呈现等待审批**（仅 EXTEND/CREATE）：提示用户「请回复：批准 <planId>」。

## 覆盖判断标准

一个现有测试被认为能覆盖本次变更的某个行为，至少需满足：

1. 能执行到本次变更涉及的 Controller / Service 调用链
2. 前置条件能够触发本次修改的代码路径
3. 有能够证明行为正确的断言
4. 如果本次风险涉及数据库修改：必须包含对应 DB / 状态断言
5. 如果本次风险涉及权限、tenant、幂等、状态流转、异常处理、事务：现有测试必须实际验证对应行为

V1 做的是**基于代码语义和测试断言的行为覆盖分析**，不是代码行覆盖率。

## 输出

必须通过 `.code-harness/contracts/test-plan.schema.json` 校验：

- 每个 target：`controller`、`endpoint`、`strategy`（必填）、`existingTests`、`scenarios`
- 每个 scenario：`name`、`coverageStatus`（COVERED/MISSING）、`coveredBy`；`MISSING` 场景还需 `request`/`expected`
- `REUSE_EXISTING` 的 target 仍在计划中体现（`strategy=REUSE_EXISTING`，scenarios 全部 COVERED），但不产生 `planId` 审批
- 计划保留完整覆盖映射（COVERED + MISSING），但审批与后续代码修改范围仅限 `MISSING` 场景；`COVERED` 场景不进入审批、不重新生成

## 停止条件

- 没有受影响的 Controller → 报告后停止
- 无法确定外部依赖的 Mock 策略 → 在计划中标记为待确认，不要猜测

## 禁止行为

- 不得在计划审批通过前编写任何测试文件
- 不得默认 Mock 项目内部的 Service 或 Repository Bean
- 不得设计访问生产数据或系统的测试
- 不得为了让测试更容易通过而弱化断言
- 不得为「生成测试」而重复创建已有能力的测试类
- 不得用代码行覆盖率代替行为覆盖判定

## 示例

### REUSE_EXISTING

现有 `OrderControllerIT` 已覆盖全部行为：

```
Existing Test Coverage

OrderControllerIT
  ✓ 正常审批
  ✓ 非法状态
  ✓ tenant 隔离
  ✓ DB 状态断言

结论：现有测试已经充分覆盖本次代码变化。
策略：REUSE_EXISTING（不要求审批、不修改测试代码、直接执行）
```

### EXTEND_EXISTING

现有 `OrderControllerIT` 缺「重复审批」：

```json
{
  "planId": "test-plan-20260818-001",
  "targets": [{
    "controller": "OrderController",
    "endpoint": "POST /api/order/approve",
    "strategy": "EXTEND_EXISTING",
    "existingTests": [
      { "className": "OrderControllerIT", "path": "src/test/java/com/example/order/OrderControllerIT.java" }
    ],
    "serviceChain": ["OrderService"],
    "repositoryChain": ["OrderRepository"],
    "externalMocks": ["OrderRpcClient"],
    "scenarios": [
      {
        "name": "正常审批待处理订单",
        "coverageStatus": "COVERED",
        "coveredBy": ["OrderControllerIT.shouldApprovePendingOrder"]
      },
      {
        "name": "重复审批应满足幂等要求",
        "coverageStatus": "MISSING",
        "coveredBy": [],
        "preconditions": ["存在 id=1 且状态为 APPROVED 的订单"],
        "request": {
          "method": "POST",
          "path": "/api/order/approve",
          "headers": {"Content-Type": "application/json", "X-Tenant-Id": "t1"},
          "body": {"orderId": 1}
        },
        "expected": {
          "httpStatus": 200,
          "responseAssertions": ["$.status == 'APPROVED'"],
          "databaseAssertions": ["order(id=1).status 保持 'APPROVED'"],
          "stateTransition": {"from": "APPROVED", "to": "APPROVED"}
        }
      }
    ]
  }]
}
```

### CREATE_NEW

找不到相关现有测试：

```json
{
  "planId": "test-plan-20260818-002",
  "targets": [{
    "controller": "RefundController",
    "endpoint": "POST /api/refund/apply",
    "strategy": "CREATE_NEW",
    "existingTests": [],
    "serviceChain": ["RefundService"],
    "repositoryChain": ["RefundRepository"],
    "externalMocks": ["PayRpcClient"],
    "scenarios": [
      {
        "name": "正常发起退款",
        "coverageStatus": "MISSING",
        "coveredBy": [],
        "request": { "method": "POST", "path": "/api/refund/apply", "body": {"orderId": 1} },
        "expected": { "httpStatus": 200, "responseAssertions": ["$.status == 'REFUNDING'"] }
      }
    ]
  }]
}
```

## Task 7：Selected-only + DB Assertion 增量规则

以下规则在上述 Existing Test / Approval 语义之上增加，不替代任何原规则：

1. 本 Skill 额外消费已通过 `test-target-selection.schema.json` + Runtime `selection.VerifyJSON` 的 `TestTargetSelection`；只允许 `selectedControllerIds` 对应的 affectedControllers 进入步骤 1-9。
2. 未选择 Controller：不得做 Existing Test Coverage Analysis，不得进入 Test Plan target；若输入 target 不属于 validated selection，立即返回 `SCOPE_VIOLATION` 并停止该越界 target。
3. 对 ChangeAnalysis 风险 `databaseWrite / transactional / stateTransition`，每个相关 scenario 必须显式判断是否需要 DB Assertion。
4. 需要 DB Assertion 时，使用现有 `expected.databaseAssertions[]`，必须写成具体可验证状态，例如：
   - `order_info.status == APPROVED for fixture order_id`
   - `audit_log contains exactly one APPROVE record for fixture order_id`
   - `rollback path keeps order_info.status == PENDING`
5. 若判断不需要 DB Assertion，必须能说明 HTTP/response assertion 为什么已经足以证明该风险，不能静默省略。
6. `databaseAssertions[]` 是正式测试证据；不能用 Task 5/6 的诊断期 Database Evidence 替代测试本身应有的 DB Assertion。
7. Selection 仅定义“测哪些 target”，仍然不能替代 `批准 <planId>` 或 `批准 <fixPlanId>`。
