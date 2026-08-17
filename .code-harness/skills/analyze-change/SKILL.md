---
name: analyze-change
description: 计算完整 Review Change Set（merge-base 分支差异 + 工作区变化），识别所有变更文件、追踪调用链，产出结构化的变更分析结果。
version: 1
agent: reviewer
tools:
  - git_diff
  - read_code
output_schema: .code-harness/contracts/change-analysis.schema.json
---

# 分析代码变更

## 目标

计算本次 Review 的完整 Change Set——基于 `merge-base(baseRef, HEAD)` 的分支差异，加上 staged、unstaged、untracked 工作区变化——识别所有变更文件，理解受影响的调用链，产出结构化的变更摘要，供下游评审和测试计划使用。

## 适用场景

- 用户说 `harness review` 或 `harness test`——这是所有流程的第一步
- 任何评审、测试计划或代码修改之前
- Reviewer、Integration Test Agent 或 Fix Agent 需要变更上下文时

## 不适用场景

- 没有 Git 仓库
- 工作区相对 base ref 没有任何变更

## 输入

- `.code-harness/harness.yaml` 的 `review.baseRef` 和 `review.includeWorkingTree`
- 当前分支名、HEAD commit、本地已有 Git refs
- `headRef`（可选）：默认 HEAD

## 允许使用的工具

- `git_diff`——列出变更文件和变更块
- `read_code`——读取变更的源文件及直接相关的上下游代码

## 前置条件

- 当前目录是 Git 仓库
- 存在可读取的本地 Git refs，且 `review.baseRef` 可解析

## 执行步骤

1. **读取 Review 配置**：读取 `.code-harness/harness.yaml` 的 `review` 段，得到 `baseRef` 和 `includeWorkingTree`。
2. **获取当前分支**：获取 `currentBranch`（Detached HEAD 记为 `DETACHED_HEAD`）。
3. **获取 HEAD commit**。
4. **获取 baseRef**。
5. **校验 baseRef 存在**：不存在 → 停止并报告，不得自行切换到 main/develop。
6. **计算 merge-base**：`mergeBase = merge-base(baseRef, HEAD)`。
7. **获取已提交变化**：获取 `merge-base → HEAD` 的所有已提交变化（即 `baseRef...HEAD`）。
8. **纳入工作区变化**：如果 `includeWorkingTree: true`：
   - 获取 staged
   - 获取 unstaged
   - 获取 untracked（主动枚举未被 Git 追踪的文件，普通 diff 看不到）
9. **合并变更**：将四部分来源合并。
10. **同一文件去重**：同一文件多个来源合并为一条，记录 `sources`。
11. **生成统一 Change Set**。
12. **生成 reviewScope**。
13. **围绕 Change Set 分析调用链**：后续调用链分析只能围绕该 Change Set 展开。

> 禁止把 base 分支自身在分叉后的新增提交误判为当前分支引入的变化——只 Review `merge-base(baseRef, HEAD) → HEAD` 区间内由当前分支引入的变化。

14. **分类变更**：按角色分组——Controller、Service、Repository/Mapper/DAO、Entity/DTO/VO、Validator、ExceptionHandler、Config、Utility、Other。
15. **读取变更文件**：对每个变更文件，若匹配 `scope.sourceIncludes` 或 `scope.testIncludes`，调用 `read_code` 获取完整文件内容（源代码与测试代码都要读，不能只看 `src/main`）。
16. **追踪调用链**：对每个变更的 Controller 或 Service 方法，识别：
    - 项目内的直接调用方（上游）和被调用方（下游）
    - 调用的 Repository/Mapper 方法
    - 外部依赖（RPC、MQ、第三方接口、缓存）
17. **识别风险区域**：标记涉及以下内容的方法：
    - 状态流转
    - 事务边界
    - 权限、身份或租户校验
    - 幂等机制
    - 异常处理路径
    - 数据库写操作（INSERT/UPDATE/DELETE）
18. **组装输出**：产出符合 Schema 的结构化摘要。

## 输出

必须通过 `.code-harness/contracts/change-analysis.schema.json` 校验：
- `reviewScope`：currentBranch、baseRef、baseCommit、mergeBase、headCommit、includeWorkingTree——本次 Review 范围标识
- `changedFiles[]`：路径、角色、sources（变更来源，去重后可能多来源）、可选的变更摘要
- `affectedControllers[]`：Controller 类及受影响的接口
- `callChains[]`：入口点及完整的 Controller → Service → Repository 调用链
- `externalDependencies[]`：变更涉及的外部系统
- `riskAreas[]`：方法及风险标签列表（stateTransition, transactional, authorization, tenancy, idempotency, exceptionHandling, databaseWrite）

## 停止条件

- `review.baseRef` 不存在 → 停止并报告 `MANUAL_ACTION_REQUIRED`，不得自行切换到 main/develop
- 无任何代码变化（committed=0 且 staged=0 且 unstaged=0 且 untracked=0）→ 报告空变化后停止
- 文件无法读取 → 报告错误并跳过该文件

## 禁止行为

- 不得扫描无关模块或整个仓库
- 不得把 base 分支自身在分叉后的新增提交误判为当前分支引入的变化
- 不得修改任何文件
- 不得直接执行 Shell 命令
- 不得自动执行 `git fetch`、`git pull` 或联网更新远端 Git 状态

## 示例

```
输入：baseRef=origin/master，当前分支 feature/order，共 3 个变更文件（merge-base 分支差异）

输出：
  reviewScope:
    currentBranch: "feature/order"
    baseRef: "origin/master"
    baseCommit: "abc111"
    mergeBase: "abc000"
    headCommit: "abc999"
    includeWorkingTree: true
  changedFiles:
    - { path: "OrderController.java", role: "Controller", sources: ["COMMITTED"], hunkSummary: "新增 approve 接口" }
    - { path: "OrderService.java", role: "Service", sources: ["COMMITTED", "UNSTAGED"], hunkSummary: "approve 业务逻辑" }
    - { path: "OrderRepository.java", role: "Repository", sources: ["COMMITTED"], hunkSummary: "updateStatus 方法" }
  affectedControllers:
    - { controller: "OrderController", endpoints: ["POST /api/order/approve"] }
  callChains:
    - { entryPoint: "OrderController.approve()", chain: ["OrderService.approve()", "OrderRepository.updateStatus()"] }
  externalDependencies: ["OrderRpcClient.notifyErp()"]
  riskAreas:
    - { method: "OrderService.approve()", risk: ["stateTransition", "transactional", "tenancy"] }
```
