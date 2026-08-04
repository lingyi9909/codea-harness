---
name: analyze-change
description: 分析 Git Diff，识别所有变更文件、追踪调用链，产出结构化的变更分析结果。
version: 1
agent: reviewer
tools:
  - git_diff
  - read_code
output_schema: contracts/change-analysis.schema.json
---

# 分析代码变更

## 目标

分析当前 Git Diff，识别所有变更文件，理解受影响的调用链，产出结构化的变更摘要，供下游评审和测试计划使用。

## 适用场景

- 用户说 `harness review` 或 `harness test`——这是所有流程的第一步
- 任何评审、测试计划或代码修改之前
- Reviewer、Integration Test Agent 或 Fix Agent 需要变更上下文时

## 不适用场景

- 没有 Git 仓库
- 工作区相对 base ref 没有任何变更

## 输入

- `baseRef`（可选）：基准 Git ref，默认使用与 main/master 的 merge-base
- `headRef`（可选）：头部 Git ref，默认使用 HEAD

## 允许使用的工具

- `git_diff`——列出变更文件和变更块
- `read_code`——读取变更的源文件及直接相关的上下游代码

## 前置条件

- 当前目录是 Git 仓库
- 存在可读取的 Git Diff

## 执行步骤

1. **获取 Diff**：调用 `git_diff(baseRef, headRef)` 获取所有变更文件及变更块。
2. **分类变更**：按角色分组——Controller、Service、Repository/Mapper/DAO、Entity/DTO/VO、Validator、ExceptionHandler、Config、Utility、Other。
3. **读取变更文件**：对每个匹配 `scope.sourceIncludes` 的变更源文件，调用 `read_code` 获取完整文件内容。
4. **追踪调用链**：对每个变更的 Controller 或 Service 方法，识别：
   - 项目内的直接调用方（上游）和被调用方（下游）
   - 调用的 Repository/Mapper 方法
   - 外部依赖（RPC、MQ、第三方接口、缓存）
5. **识别风险区域**：标记涉及以下内容的方法：
   - 状态流转
   - 事务边界
   - 权限、身份或租户校验
   - 幂等机制
   - 异常处理路径
   - 数据库写操作（INSERT/UPDATE/DELETE）
6. **组装输出**：产出符合 Schema 的结构化摘要。

## 输出

必须通过 `contracts/change-analysis.schema.json` 校验：
- `changedFiles[]`：路径、角色、可选的变更摘要
- `affectedControllers[]`：Controller 类及受影响的接口
- `callChains[]`：入口点及完整的 Controller → Service → Repository 调用链
- `externalDependencies[]`：变更涉及的外部系统
- `riskAreas[]`：方法及风险标签列表（stateTransition, transactional, authorization, tenancy, idempotency, exceptionHandling, databaseWrite）

## 停止条件

- 没有匹配 `scope.sourceIncludes` 的变更文件 → 报告空 diff 后停止
- 文件无法读取 → 报告错误并跳过该文件

## 禁止行为

- 不得扫描无关模块或整个仓库
- 不得修改任何文件
- 不得直接执行 Shell 命令

## 示例

```
输入：feature 分支相对 main 的 git_diff，共 3 个变更文件

输出：
  changedFiles:
    - { path: "OrderController.java", role: "Controller", hunkSummary: "新增 approve 接口" }
    - { path: "OrderService.java", role: "Service", hunkSummary: "approve 业务逻辑" }
    - { path: "OrderRepository.java", role: "Repository", hunkSummary: "updateStatus 方法" }
  affectedControllers:
    - { controller: "OrderController", endpoints: ["POST /api/order/approve"] }
  callChains:
    - { entryPoint: "OrderController.approve()", chain: ["OrderService.approve()", "OrderRepository.updateStatus()"] }
  externalDependencies: ["OrderRpcClient.notifyErp()"]
  riskAreas:
    - { method: "OrderService.approve()", risk: ["stateTransition", "transactional", "tenancy"] }
```
