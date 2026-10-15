# 代码评审报告｜T1 订单余额变更（人工格式示例）

> **排版与风险均为人工示例，绝非公司私有模型实际 Review 记录。** 仅用于核对 Harness 1.8 的五章结构。真实内网报告需另附脱敏 TUI 输出。

## 评审摘要

| 项目 | 内容 |
|---|---|
| 项目 / 需求 | T1 Fixture / 订单余额扣减调整（示例） |
| 评审结论 | 🔴 示例：存在阻断问题 |
| 评审模式 | 当前 Git 工作区变更评审 |
| Git 基线 | origin/master（示例；实际须按本地 Git 引用） |
| 评审目标 | `OrderController.charge` |
| 本次范围 | 演示 Service 与 Mapper XML 两个变更文件 |
| 变更来源 | 此 fixture 示例为 HEAD 已提交；实际 staged/unstaged/untracked 需按 TUI 结果填写 |
| 风险概览 | 严重 0 · 高 2 · 中 0 · 低 0（人工预期） |
| 待确认风险 | 0 项（仅示例） |

**核心发现：** 同一请求的幂等保护被移除；账户更新 SQL 中丢失租户条件。

**建议动作：** 补回可靠幂等和租户过滤，检查跨租户更新与重试场景。此建议不替代发布审批。

## 调用链与覆盖范围

### C1 · 订单扣减 · 本次样例

| 顺序 | 角色 | 代码位置 |
|---|---|---|
| 1 | 接口入口 | `OrderController.charge` |
| 2 | 业务实现 | `OrderService.charge` |
| 3 | 数据访问 | `OrderMapper.decreaseBalance` |
| 4 | SQL 执行 | `OrderMapper.xml#decreaseBalance` |

本样例中 Controller 代码本身没有变化，通过 Service 与 Mapper 的静态关系关联到业务入口。

**未纳入范围：** 实际部署的权限控制、数据库约束、其他动态业务入口均未核验；示例不代表完整调用图。

## 风险清单

| 编号 | 等级 | 问题摘要 | 业务影响 | 建议动作 |
|---|---|---|---|---|
| F-001 | 高 | 幂等保护被移除 | 相同请求重试可能重复扣减 | 建立可靠的请求幂等约束 |
| F-002 | 高 | UPDATE 缺少租户隔离条件 | 特定 ID 设计下可能误更新其他租户数据 | 补充可信 tenant_id 过滤 |

## 问题明细

### F-001 · 高 · 幂等保护被移除

**问题与触发条件**

新的 `OrderService.charge` 不再校验请求 ID 是否已处理，同一 `requestId` 重试可以重复调用扣减接口。

**业务影响**

可能重复扣减资金；真实系统中是否有其他持久化幂等约束需要继续核查。

**代码位置**

`src/main/java/demo/orders/OrderService.java`，`charge`。

**关键证据（人工示例）**

旧版本使用 `processed.add(requestId)` 阻止进程内重复调用；新版本直接 `mapper.decreaseBalance(...)`。此演示源于 fixture 的已提交差异，**证据来源：HEAD（已提交）**。若在真实项目中发生未提交修改，需重新标注 INDEX / WORKTREE / UNTRACKED。

**修复建议**

按业务流水号增加持久化幂等键，重复请求不得重复记账。

**验证建议**

正常提交一次；重复 requestId 不重复扣减；多实例重试结果一致。

### F-002 · 高 · UPDATE 缺少租户隔离条件

**问题与触发条件**

新 Mapper SQL 保留 `WHERE order_id = #{orderId}`，删除 `AND tenant_id = #{tenantId}`。

**业务影响**

如果订单 ID 不是全局唯一，或上游身份校验不充分，可能产生跨租户更新。

**代码位置**

`src/main/resources/mapper/OrderMapper.xml#decreaseBalance`。

**关键证据（人工示例）**

```sql
UPDATE account_balance
SET balance_cents = balance_cents - #{cents}
WHERE order_id = #{orderId}
```

**证据来源：HEAD（已提交）**，不应将来自工作区的 staged/unstaged 行冒充 HEAD 证据。

**修复建议**

使用可信当前租户 ID 限制 SQL 更新条件。

**验证建议**

当前租户正常更新；其他租户相同 orderId 不更新；无记录时返回明确错误。

## 后续处理与评审边界

| 待办 | 建议 |
|---|---|
| 修复 F-001 | 恢复可靠的幂等保护 |
| 修复 F-002 | 恢复租户隔离和影响行数校验 |
| 复评 | 修改后用 `/codea-review` 审查最终工作区代码 |

本例没有运行 OpenCode TUI、业务测试或数据库校验。实际企业模型表现、工具调用次数与耗时均待真实内网验收；代码来自人工编写的测试 fixture。

---

报告追溯：Codea Review 2.0（人工示例） · **EXAMPLE_ONLY / NOT_RUN**。
