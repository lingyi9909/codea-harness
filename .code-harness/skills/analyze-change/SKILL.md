---
name: analyze-change
description: 计算完整 Review Change Set，读取所有变更文件，并通过受控 Code Navigation Contract 确定性展开与变更直接相关的项目内部调用链。
version: 2
agent: reviewer
tools:
  - git_diff
  - read_code
  - find_symbol
  - find_references
  - find_implementations
output_schema: .code-harness/contracts/change-analysis.schema.json
---

# 分析代码变更

## 目标

计算本次 Review 的完整 Change Set：`merge-base(baseRef, HEAD) → HEAD` 的已提交变化，加上 staged、unstaged、untracked；**读取所有 changed source/test files**，再通过 Code Navigation Contract 定位与本次变更直接相关的上游、下游和接口实现，直到 Repository / Mapper 或外部边界。最终输出可机器校验的 `reviewCoverage`。

## 关键原则

1. Change Set 是唯一入口，`harness review` 与 `harness test` 必须复用完全相同的语义。
2. 所有匹配 `scope.sourceIncludes` / `scope.testIncludes` 的 `changedFiles` 必须 `read_code`，不能只读 Controller。
3. Controller、Service、Repository、Mapper、DTO、Validator 等**任何发生变更的文件**都属于 Review 起点。
4. 符号定位必须使用 `find_symbol` / `find_references` / `find_implementations`，不得靠文件名猜路径。
5. Agent/Skill 禁止依赖 ast-grep 语法；ast-grep 只是 Code Navigation Contract 的当前实现。
6. 调用链只围绕本次 Change Set 的直接相关路径展开，不扫描整个仓库。

## 执行步骤

### A. 建立完整 Change Set

1. 读取 `harness.yaml.review.baseRef` / `includeWorkingTree`。
2. 校验 baseRef 本地存在；不存在则 `MANUAL_ACTION_REQUIRED`，不得自动换 main/develop。
3. 使用 `git_diff` 获取：
   - committed：`merge-base(baseRef, HEAD) → HEAD`
   - staged
   - unstaged
   - untracked
4. 同一路径多来源合并去重，保留 `sources`。
5. 生成 `reviewScope`。

### B. 强制读取所有变更文件

6. 分类全部 `changedFiles`。
7. 对每个匹配 source/test scope 的 changed file 调用 `read_code` 读取完整内容。测试文件同样必须读取。
8. 每个成功读取的 changed file 立即记入：

```json
{"path":"...","role":"Service","reason":"CHANGED"}
```

读取失败不得静默跳过：必须记入 unresolved / limitation，最终 coverage 为 `PARTIAL`。

### C. 从 changed symbols 双向展开调用链

9. 从 Diff + 完整文件识别本次变更涉及的类、接口和方法符号。
10. **Controller 变更**：从 Controller 方法向下，通过 `find_symbol` / `find_implementations` 定位 Service 实现，再继续向下。
11. **Service 自身变更**：Service 方法本身作为入口：
    - `find_references` 找项目内部上游调用者，定位可能受影响的 Controller / Service；
    - 同时向下定位直接调用的 Service / Repository / Mapper。
12. **接口必须解析实现**：例如 `OrderService` 被调用时：
    - `find_symbol(OrderService)` 确认接口；
    - `find_implementations(OrderService)` 定位 `OrderServiceImpl`；
    - 对实现文件 `read_code`；
    - 若找不到实现，记录 `IMPLEMENTATION_NOT_FOUND`，不得假装完成。
13. **允许多层 Service**：例如 Controller → ServiceA → ServiceB → Repository，与本次变更路径直接相关时必须继续追踪 ServiceB，直到边界。
14. 每个通过导航加入上下文并成功 `read_code` 的非 changed 文件，记录 `reason: CALL_CHAIN`。

### D. 停止边界

15. 在以下边界停止向下展开：
    - Repository / Mapper / DAO
    - 已确认的外部 RPC / HTTP client
    - MQ producer/consumer client
    - Cache client
    - 第三方 SDK
    - JDK / Spring Framework
16. 外部依赖写入 `externalDependencies[]`；确认 external 后不要求读取第三方源码。
17. 不得因一个符号难定位而扩大成全仓库无界扫描。

### E. Review Coverage 硬判定

18. `reviewCoverage.status = COMPLETE` **必须同时满足**：
    - 每个 changed source/test file 都已 `read_code`；
    - `callChains` 中每个项目内部符号都已确定性定位并读取；
    - 或该符号已明确记录为 `externalDependencies`；
    - `unresolvedSymbols` 为空。
19. 任一条件不满足 → `PARTIAL`。
20. 典型未解析项：

```json
{
  "symbol": "OrderService.approve",
  "from": "OrderController.approve",
  "reason": "IMPLEMENTATION_NOT_FOUND"
}
```

21. `PARTIAL` 不是 Review 成功：交给 Orchestrator 后必须停止，不得继续 `review-code` 或测试计划。

## 输出

输出必须通过 `.code-harness/contracts/change-analysis.schema.json`：

- `reviewScope`
- `changedFiles[]`
- `affectedControllers[]`
- `callChains[]`
- `externalDependencies[]`
- `riskAreas[]`
- `reviewCoverage`（REQUIRED）

## 禁止行为

- 不得只 Review Controller。
- 不得通过类名/文件名猜 Service 实现路径。
- 不得跳过 changed test/source files。
- 不得扫描整个仓库。
- 不得直接调用 `ast-grep.exe`；只能使用受控 Code Navigation Contract。
- 不得执行任意 Shell、`git fetch` 或 `git pull`。
- 不得修改文件。
