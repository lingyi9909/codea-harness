---
name: review-code
description: Codea Harness 1.8 ordinary Review 的主 Agent 语义评审规则；仅在 Runtime 已建立所选调用链 scope 后，对真实源码证据形成 findings，并通过 codea-review finish 提交。
version: 5
agent: orchestrator
tools:
  - read_code
---

# Codea Harness 1.8 Review Code

本 Skill 只服务当前 1.8 ordinary Review。执行入口、选择和最终写盘均由 `codea-review` + 受控 Runtime 决定；本 Skill 不拥有 run、scope、Git diff、Host identity 或最终报告 authority。

## 前置条件

- 本次 `/harness-review` 已由固定入口创建 INCOMPLETE `review.md`。
- 已有本 run 的 `runId`。
- `codea-review prepare` 已成功；单链已自动形成 scope，或多链已通过下一条真实用户消息完成 `select`。
- 主 Agent 只读取 `scope.reads` 允许的 source hash/range。若源码变化、scope 不完整或 Runtime 返回错误，停止并保留 INCOMPLETE 报告。

## Context 与 Finding 是两层范围

`scope.reads` 允许为了理解调用链读取完整 Controller / Service / Mapper / SQL 上下文，但“可读”不等于“可报问题”。Runtime 会保存独立的 formal finding range：

- Controller：只允许所选 endpoint/method。
- Service/ServiceImpl：只允许所选链实际调用的方法。
- Mapper Java：只允许所选 mapper method。
- Mapper XML：只允许与所选 mapper method 对应的 statement。

因此，同 Controller 的 sibling endpoint、共享 Service 的另一个 method、共享 Mapper XML 的另一个 statement 即使处在同一个可读文件中，也不得形成本次 finding。发现 scope 外潜在风险时，只说明需要新的 Review 范围，不把它塞入当前结果。

## Evidence 规则

每条 finding 必须填写：

- `id`
- `severity`: `CRITICAL | HIGH | MEDIUM | LOW`
- `problem`
- `impact`
- `recommendation`
- `verification`
- `evidence[]`: 每项包含 source `ref` 与精确 `quote`
- `introducedByChange`：仅在有确定性依据时填写

Evidence 必须满足：

1. source path/hash/range 来自本 run 已读取源码；
2. quote 真实存在于 ref 范围；
3. quote 实际位置属于所选链 formal finding range；
4. 不用类名、猜测、规则命中或模型 confidence 代替源码证据。

没有足够证据就不报 finding。findings 可以为空，但仍必须调用 `codea-review action=finish`。

## 变更归因

`introducedByChange` 不是模型自报事实：

- `CURRENT_IMPLEMENTATION`：不得设置为 `true`。
- `CHANGES`：只有 finding 的真实 evidence 命中当前 Git diff 的 changed line，才允许设置为 `true`。
- 仅仅“文件有变化”不足以证明某个问题由本次变化引入；问题证据位于未变化 sibling method 时不得标记。
- Runtime 在 finish 时会重新计算并校验，模型填写不能绕过。

## Java 生产代码

只报告有明确证据、会影响正确性/安全性/数据一致性/兼容性的实际问题，例如：

- 参数与状态校验缺失导致错误业务路径；
- 事务、幂等、并发、权限/租户边界错误；
- 异常吞噬、错误返回、空指针、资源泄漏；
- Controller → Service → Mapper 调用语义不一致；
- 状态流转、数据写入或读取条件明显错误。

不输出纯风格、命名、格式化、无证据重构建议。

## Mapper XML

仅对所选链对应 statement 的高价值风险形成 finding：

- UPDATE/DELETE 缺少必要 WHERE 或条件明显过宽；
- 租户/机构/用户隔离条件被移除或弱化；
- 动态 SQL 使关键过滤失效；
- statement id、参数、resultMap/resultType 与所选 Java Mapper method 明显不一致；
- 明显无边界批量写风险。

不因 XML 缩进、命名或排版形成 finding。

## 配置与资源

只有当配置/资源由所选调用链明确关联且处于 Runtime scope 时才评审。关注 datasource、timeout、线程池、MQ/RPC、profile、feature switch、敏感信息及 Java 配置绑定不一致等高价值风险；未变化、未关联或范围外内容不顺手扩审。

## 测试代码

测试代码不是 ordinary production finding 的默认来源。只有真实证据证明测试会 false-positive、绕过真实业务链或失去关键验证时，才作为有效性风险说明，例如禁用有效测试、吞异常、删除关键断言、错误 Mock 内部业务 Bean。普通测试代码风格不报问题。

## 完成语义

主 Agent完成读取和判断后，一次调用：

`codea-review action=finish`

result 包含本次 `reads/findings/pendingRisks/gaps`。Runtime 在同一调用内完成范围、evidence、变更归因、幂等/并发、结果保存、报告渲染和回读验证。

- COMPLETE + high/critical finding → `BLOCKING`
- COMPLETE + 其他 finding/pending risk → `ACTION_REQUIRED`
- COMPLETE + 无问题/风险 → `NO_ISSUES_FOUND`
- PARTIAL → `UNDETERMINED`

只有 finish 返回的 path/hash 与磁盘 `review.md` 一致且 `execution=COMPLETE`，才可宣布 ordinary Review 完成。任何失败都保留当前 INCOMPLETE run，不自动换 run，也不自动进入 Fix。

pre-1.8 ordinary Review 的历史机制只在 `.code-harness/history/ordinary-review-pre-1.8.md`，不得作为本 Skill 的执行步骤。
