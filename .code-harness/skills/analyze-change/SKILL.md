---
name: analyze-change
description: 计算完整 Review Change Set，并按 FULL/TARGETED 意图通过受控 Code Navigation Contract 建立可机器验证的调用链与 Review Scope。
version: 3
agent: reviewer
tools:
  - git_diff
  - read_code
  - find_symbol
  - find_references
  - find_implementations
  - validate_contract
output_schema: .code-harness/contracts/change-analysis.schema.json
---

# 分析代码变更

## 目标

所有 Review 都先计算同一个完整 Change Set：`merge-base(baseRef, HEAD) → HEAD` 的 committed，加上 staged、unstaged、untracked。之后根据意图分为：

```text
FULL      → 完整读取并机器验证整个 Change Set
TARGETED  → 从完整 Change Set 的 confirmed callChains 中解析 target，只读取并机器验证 selectedCallChains + scopedFiles
LIST      → 仅列调用链，不执行 Finding Review
```

TARGETED 不是 sampled review。只有它声明的全部 scopedFiles 与 selectedCallChains 被完整覆盖后才允许 COMPLETE；不得把 TARGETED 结果表述成整个 Change Set 已评审。

## 关键原则

1. Change Set 是唯一入口，不因 target 改变 committed/staged/unstaged/untracked 的计算语义。
2. FULL 保持 1.3.2：所有匹配 source/test scope 的 changedFiles 都必须 `read_code`。
3. TARGETED 允许与 target 无关的 changed file 留在 Change Set 但不进入 scopedFiles；这些文件不计为 reviewed。
4. `selectedCallChains` 必须逐条来自 `ChangeAnalysis.callChains[]`，不得编造或压平。
5. `scopedFiles` 必须由 target/selected chain 与 ChangeAnalysis 的证据关系支持。
6. 符号定位只能使用 `find_symbol` / `find_references` / `find_implementations`。
7. Agent 声明 COMPLETE 不是事实，必须经过 Runtime machine gate。
8. **不允许 sampled review** 进入 COMPLETE/PASSED。

## A. 建立完整 Change Set

1. 读取 `harness.yaml.review.baseRef` / `includeWorkingTree`。
2. 校验 baseRef 本地存在；不存在 → `MANUAL_ACTION_REQUIRED`，不得自动换 main/develop。
3. 使用 `git_diff` 获取 committed/staged/unstaged/untracked。
4. 同路径多来源合并去重，保留 `sources`。
5. 生成完整 `changedFiles[]`；TARGETED 也不得丢掉 scope 外 changed file 的 Change Set 元数据。

## B. 建立 confirmed callChains

6. 从 Diff 和必要的源码证据识别 changed symbols。
7. Controller 变更：从 Controller method 向下导航 Service/实现/Repository/Mapper。
8. Service/Repository 等下游变更：使用 `find_references` 反向定位上游 Controller/Service，同时向下导航到停止边界。
9. 接口必须 `find_implementations`；找不到实现记录 `IMPLEMENTATION_NOT_FOUND`，不得假装 confirmed。
10. 允许多层 Service，直到 Repository/Mapper/DAO 或明确外部边界。
11. confirmed chain 写入 `callChains[]`；candidate / unresolved 只进入 unresolved/limitation，不得包装进 confirmed chain。
12. `affectedControllers[]` 仍必须提供 `impactType/sourceSymbols` 的真实导航证据。

## C. FULL Review

13. 对所有匹配 source/test scope 的 changed file 读取完整内容。
14. 对 FULL 所有相关内部 call-chain 文件读取完整内容，非 changed 文件记 `reason: CALL_CHAIN`。
15. `reviewCoverage.status=COMPLETE` 仅当：
    - 全 changed source/test files 已读；
    - 全部相关项目内部 chain symbols 已解析/读取；
    - 外部边界已明确列为 `externalDependencies`；
    - `unresolvedSymbols` 为空。
16. 组装 ChangeAnalysis 后用 `change-analysis.schema.json` + Runtime FULL Coverage 验证；不得跳过现有 `coverage.VerifyAnalysisJSON` 语义。

## D. TARGETED Review

17. 先从 confirmed `ChangeAnalysis.callChains[]` 解析 target：

```text
Class         → TARGETED CLASS
Class.method  → TARGETED METHOD
```

18. target 对应 1 条 confirmed chain → 可自动选择。
19. target 对应 2+ 条 confirmed chain → 必须由 Orchestrator 让用户选择；不得默认 ALL。
20. 形成 `ReviewScopeSelection`：

```json
{
  "mode":"TARGETED",
  "target":{"symbol":"OrderController.approve","kind":"METHOD"},
  "selectedCallChains":[...],
  "scopedFiles":[...]
}
```

21. `selectedCallChains` 必须是 ChangeAnalysis confirmed chains 的真实子集。
22. `scopedFiles` 只允许包含 selected chains/target 能证明关联的文件；与 target 无关的 changed files 留在 Change Set，但不进入 scopedFiles。
23. 读取所有 scopedFiles 及 selectedCallChains 必需的项目内部文件；任一缺失都必须 PARTIAL。
24. 把 ReviewScopeSelection 交给 Controlled Runtime：

```text
validate
--schema .code-harness/contracts/review-scope.schema.json
--input <review-scope.json>
--format json
--change-analysis <change-analysis.json>
```

Runtime 必须先验证 `review-scope.schema.json` 与 `change-analysis.schema.json`，再执行 `reviewscope.Verify` 和 scoped coverage。机器结果不是 COMPLETE → `MANUAL_ACTION_REQUIRED` / STOP。
25. TARGETED 不允许直接调用 FULL `change-analysis` coverage 去要求所有 unrelated changed files 被读取；它的 required set 是经过机器验证的 `scopedFiles`。

## E. `harness review list`

26. LIST 只建立 Change Set + confirmed/candidate/unresolved chain 信息。
27. 输出分组：

```text
已确认调用链
候选/未解析
```

28. 不调用 `review-code`，不产生 Finding，不把 candidate/unresolved 伪装为 confirmed。

## 停止边界

向下导航在以下边界停止：

- Repository / Mapper / DAO
- 明确的 RPC / HTTP client
- MQ client
- Cache client
- 第三方 SDK
- JDK / Spring Framework

外部依赖进入 `externalDependencies[]`；不得为找不到符号扩大成无界全仓库扫描。

## 输出

基础 ChangeAnalysis 仍必须符合 `.code-harness/contracts/change-analysis.schema.json`：

- `reviewScope`
- `changedFiles[]`
- `affectedControllers[]`
- `callChains[]`
- `externalDependencies[]`
- `riskAreas[]`
- `reviewCoverage`

TARGETED 额外生成独立 `ReviewScopeSelection`，必须符合 `.code-harness/contracts/review-scope.schema.json` 并经过 Runtime 对照 ChangeAnalysis 验证。

## 禁止行为

- 不得跳过 Change Set 计算。
- FULL 不得跳过 changed source/test files。
- TARGETED 不得把 scope 外 changed files 标为 reviewed。
- 不得通过类名/文件名猜实现路径。
- 不得直接调用 `ast-grep.exe`。
- 不得跳过 Runtime machine gate 并相信 Agent COMPLETE。
- 不得 sampled review。
- 不得执行任意 Shell、`git fetch` 或 `git pull`。
- 不得修改文件。
