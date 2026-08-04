---
name: review-code
description: 评审变更代码的正确性——包括状态流转、事务、权限、租户、幂等性、异常处理和数据一致性，产出有证据支持的评审发现。
version: 1
agent: reviewer
tools:
  - read_code
output_schema: contracts/review-output.schema.json
---

# 评审变更代码

## 目标

评审变更代码的正确性和安全性，产出有具体证据支撑的评审发现，输出需通过 `review-output.schema.json` 校验。

## 适用场景

- `analyze-change` 完成后——这是 `harness review` 和 `harness test` 的第二步
- 需要验证变更代码正确性时
- 在 `harness test` 中，评审总是在测试计划生成之前执行

## 不适用场景

- 尚未产出变更分析——先执行 `analyze-change`
- Diff 为空（没有需要评审的变更）
- 用户明确要求跳过评审

## 输入

- `analyze-change` 产出的变更分析（通过 `change-analysis.schema.json` 校验）
- 所有变更源文件及直接相关的上下游代码的完整内容

## 允许使用的工具

- `read_code`——读取评审发现中引用的源文件以提取证据

## 前置条件

- 变更分析已完成
- 变更源文件可读取

## 执行步骤

1. **参数校验**：对每个变更方法检查：
   - 必要参数是否做了校验（null、空值、范围）
   - 输入类型是否匹配预期 Schema
   - DTO/VO 字段是否有合适的约束

2. **业务规则**：验证变更逻辑是否正确实现：
   - Diff 上下文中体现的业务需求
   - 边界情况（空列表、null 值、边界值）

3. **状态流转**：对任何状态变更检查：
   - 所有合法流转是否被显式处理
   - 非法流转是否被拒绝并给出合适错误
   - 是否考虑了并发状态修改

4. **事务边界**：对数据库写操作验证：
   - `@Transactional` 边界是否合适（不过宽也不过窄）
   - 回滚条件是否正确
   - 只读操作是否被不必要地包裹在写事务中

5. **权限和租户**：确认：
   - 敏感操作前是否有身份/角色校验
   - 租户/机构数据隔离是否在查询中实施
   - 用户上下文是否在调用链中正确传递

6. **幂等性**：对变更类操作检查：
   - 重复请求是否被安全处理
   - 是否有唯一约束或幂等键

7. **异常处理**：验证：
   - 异常是否在正确的层级被捕获
   - 错误响应是否遵循项目统一响应格式
   - 错误信息中不泄露敏感信息
   - 资源（连接、流）是否在 finally/try-with-resources 中关闭

8. **数据一致性**：检查：
   - 查询条件是否符合业务意图（特别是软删除过滤、状态过滤）
   - Update 语句是否包含合适的 WHERE 条件
   - 缓存失效是否在成功写入之后执行

9. **记录发现**：每条发现记录：文件、行号、Diff 或代码中的具体证据、影响范围、最小修复建议、严重程度、是否由本次变更引入、置信度、是否需要补充集成测试。

## 输出

必须通过 `contracts/review-output.schema.json` 校验。每条发现必须包含：
- `id`、`severity`、`file`、`line`、`evidence`、`impact`、`recommendation`
- `needsTest`（boolean）、`introducedByChange`（boolean）、`confidence`（0-1）

## 停止条件

- 没有发现任何问题 → 输出空发现列表，摘要注明未发现问题
- 证据所需的文件无法读取 → 在发现中注明该限制

## 禁止行为

- 不得修改任何文件
- 不得提出无关重构或代码风格调整
- 不得在缺少 Diff 或引用代码中具体证据的情况下报告问题
- 不得扫描整个仓库

## 示例

```json
{
  "id": "F-001",
  "severity": "high",
  "file": "src/main/java/com/example/OrderService.java",
  "line": 42,
  "evidence": "OrderService.approve() 直接将状态设为 APPROVED，未检查当前状态是否为 PENDING",
  "impact": "已审批或已取消的订单可能被重复审批，导致重复的 ERP 通知",
  "recommendation": "增加状态守卫：if (order.getStatus() != Status.PENDING) throw new IllegalOrderStateException()",
  "needsTest": true,
  "introducedByChange": true,
  "confidence": 0.95
}
```
