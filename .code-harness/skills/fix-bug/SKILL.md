---
name: fix-bug
description: 针对已确认的生产代码缺陷，设计并应用经审批的最小修复。
version: 1
agent: fix-agent
tools:
  - apply_approved_patch
output_schema: contracts/fix-plan.schema.json
---

# 修复已确认的缺陷

## 目标

根据用户选定的评审发现或 `PRODUCTION_CODE_ERROR` Diagnosis，设计最小修复方案，经人工审批后通过受控工具应用修改。

## 适用场景

- 用户明确说「fix finding F-001」或「fix diagnosis run-001」
- Orchestrator 将 `harness fix finding:<id>` 或 `harness fix diagnosis:<runId>` 路由给 Fix Agent
- Runtime Debugger 输出 `nextAction: GENERATE_FIX_PLAN`

## 不适用场景

- 问题是测试代码错误 → 通过 Integration Test Agent 以 `REPAIR_TEST` 修复测试
- 问题是环境/数据问题 → 以 `REPORT_ENVIRONMENT` 报告用户
- 没有用户选定的具体发现或诊断

## 输入

- 用户选定的评审发现（以 finding `id` 标识，如 `F-001`）
- 或分类为 `PRODUCTION_CODE_ERROR` 的 Diagnosis（以 `runId` 标识）
- 受影响代码的完整源文件

## 允许使用的工具

- `read_code`——读取受影响的源文件
- `apply_approved_patch`——修改生产文件（仅在审批通过后）

## 前置条件

- 存在评审发现或 `PRODUCTION_CODE_ERROR` Diagnosis
- 用户已明确选择要修复的问题

## 执行步骤

1. **阅读受影响代码**：使用 `read_code` 获取发现或诊断中引用的文件的完整内容。
2. **追踪根因**：确定导致缺陷的具体行或条件。记录在 `rootCause` 中。
3. **设计修复**：确定能解决根因的最小改动。修改必须：
   - 只解决报告的缺陷
   - 不重构、重组或「清理」周边代码
   - 不弱化任何已有校验、断言或错误检查
   - 不删除或禁用任何测试
   - 不修改测试代码
4. **生成修复方案**：输出符合 Schema 的方案，带有唯一的 `fixPlanId`：
   ```json
   {
     "fixPlanId": "fix-plan-20260804-001",
     "rootCause": "...",
     "changes": [
       {
         "file": "src/main/java/com/example/OrderService.java",
         "reason": "状态流转前缺少状态守卫",
         "change": "增加前置条件检查：if (order.getStatus() != PENDING) throw new IllegalOrderStateException()"
       }
     ],
     "verification": [
       "运行 OrderControllerIT.shouldApproveOrder——应通过",
       "运行 OrderControllerIT.shouldRejectNonPendingOrder——守卫的新测试"
     ]
   }
   ```
5. **等待审批**：醒目地呈现方案。用户必须明确回复「批准 <fixPlanId>」或「approved <fixPlanId>」。模糊肯定（「好」「继续」「可以」）不算审批。方案修改后必须生成新的 `fixPlanId`——原审批不延续。
6. **应用修复**：明确审批通过后，使用 `apply_approved_patch(fixPlanId, changes)` 仅修改方案中列出的文件。
7. **报告完成**：输出修改了哪些文件，交由 Orchestrator 进行验证。

## 输出

- 通过 `contracts/fix-plan.schema.json` 校验的修复方案
- 修改后的生产文件（仅限审批通过方案的 `changes` 中列出的文件）

## 停止条件

- 修复方案未获审批 → 停止
- 目标文件在 `deniedPaths` 中 → 停止并报告
- 目标文件不在 `allowedProductionPaths` 中 → 停止并报告
- 修复后验证发现新问题 → 输出新诊断结果，**不得自动再次修复**

## 禁止行为

- 不得在人工明确审批 `fixPlanId` 之前修改生产代码
- 不得重构、重组或「改进」无关代码
- 不得删除测试、禁用测试、弱化断言或吞掉异常
- 不得提交、推送、创建 PR 或发布 Git 变更
- **不得重新运行测试**——验证由 Runtime Debugger 负责
- 不得直接执行 Shell 命令——只能使用受控工具

## 示例

用户：fix finding F-001

Agent：
```
修复方案：fix-plan-20260804-001

根因：OrderService.approve() 第 42 行直接将状态设为 APPROVED，
未校验当前状态，导致已取消或已驳回的订单可被重复审批。

修改：
  文件：src/main/java/com/example/OrderService.java:42
  增加：if (order.getStatus() != Status.PENDING) {
         throw new IllegalOrderStateException("订单状态不是 PENDING，无法审批");
       }

验证：
  1. 运行 OrderControllerIT.shouldApprovePendingOrder——应通过
  2. 人工确认 shouldRejectNonPendingOrder 能捕获新的守卫逻辑

请回复：批准 fix-plan-20260804-001
```
