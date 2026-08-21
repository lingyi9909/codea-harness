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
5. 每次确定性 Code Navigation 都必须把 `symbol → exact repository path + role + source` 固化到 `ChangeAnalysis.symbolLocations[]`；由另一个已确认 symbol 导航得到时可记录 `from`。
6. `scopedFiles` 必须使用 `symbolLocations[]` 的 exact repository path 证据；禁止通过 basename、简单类名或文件名猜路径，多模块同名类不得互相替代。
7. 符号定位只能使用 `find_symbol` / `find_references` / `find_implementations`。
8. Agent 声明 COMPLETE 不是事实，必须经过 Runtime machine gate。
9. **不允许 sampled review** 进入 COMPLETE/PASSED。

## A. 建立完整 Change Set

1. 读取 `harness.yaml.review.baseRef` / `includeWorkingTree`。
2. 校验 baseRef 本地存在；不存在 → `MANUAL_ACTION_REQUIRED`，不得自动换 main/develop。
3. 使用 `git_diff` 获取 committed/staged/unstaged/untracked。
4. 同路径多来源合并去重，保留 `sources`。
5. 生成完整 `changedFiles[]`；TARGETED 也不得丢掉 scope 外 changed file 的 Change Set 元数据。

## B. 建立 confirmed callChains 与 Navigation Evidence

6. 从 Diff 和必要的源码证据识别 changed symbols。
7. Controller 变更：从 Controller method 向下导航 Service/实现/Repository/Mapper。
8. Service/Repository 等下游变更：使用 `find_references` 反向定位上游 Controller/Service，同时向下导航到停止边界。
9. 接口必须 `find_implementations`；找不到实现记录 `IMPLEMENTATION_NOT_FOUND`，不得假装 confirmed。
10. 允许多层 Service，直到 Repository/Mapper/DAO 或明确外部边界。
11. 每个 `find_symbol` / `find_references` / `find_implementations` 确定性结果必须记录到 `symbolLocations[]`：

```json
{
  "symbol":"OrderService.approve",
  "path":"module-a/src/main/java/com/acme/order/OrderService.java",
  "role":"Service",
  "source":"FIND_SYMBOL"
}
```

反向/下游关系需要为额外上下文文件证明直接关系时可记录：

```json
{
  "symbol":"OrderController.approve",
  "path":"module-a/src/main/java/com/acme/order/OrderController.java",
  "role":"Controller",
  "source":"FIND_REFERENCES",
  "from":"OrderService.approve"
}
```

12. `path` 必须是 Code Navigation 返回并实际读取/确认的 repository-relative exact path；不得根据类名重建路径。
13. 同一个 symbol 得到两个不同 exact path/role 时不得任选一个，必须视为 ambiguity 并停止 Targeted Scope 确认。
14. confirmed chain 写入 `callChains[]`；candidate / unresolved 只进入 unresolved/limitation，不得包装进 confirmed chain。
15. `affectedControllers[]` 仍必须提供 `impactType/sourceSymbols` 的真实导航证据。

## C. FULL Review

16. 对所有匹配 source/test scope 的 changed file 读取完整内容。
17. 对 FULL 所有相关内部 call-chain 文件读取完整内容，非 changed 文件记 `reason: CALL_CHAIN`。
18. `reviewCoverage.status=COMPLETE` 仅当：
    - 全 changed source/test files 已读；
    - 全部相关项目内部 chain symbols 已解析/读取；
    - 外部边界已明确列为 `externalDependencies`；
    - `unresolvedSymbols` 为空。
19. 组装 ChangeAnalysis 后用 `change-analysis.schema.json` + Runtime FULL Coverage 验证；不得跳过现有 `coverage.VerifyAnalysisJSON` 语义。

## D. TARGETED Review

20. 先从 confirmed `ChangeAnalysis.callChains[]` 解析 target，并用 `symbolLocations[].role` 判断 target 属于 Controller 还是 Service/其他下游角色；不得靠 `*Controller` 命名后缀猜角色。

```text
Class         → TARGETED CLASS
Class.method  → TARGETED METHOD
```

21. 多链语义固定：

```text
Controller CLASS  → 自动包含该 Controller 在当前 Change Set 中全部相关 confirmed chains
Controller METHOD → 自动包含该 method 在当前 Change Set 中全部相关 confirmed chains
Service/其他下游 target → 1 条 confirmed chain 自动继续；2+ 条上游业务链才要求用户选择
```

22. Controller CLASS/METHOD 不提供“只选部分链”的降级路径；Runtime 会重新计算 required Controller chains，任何漏链都拒绝。
23. Service/其他下游 target 对应 2+ 条上游业务链时，由 Orchestrator 让用户选择；不得默认 ALL。空选择/取消 → STOP。
24. 形成 `ReviewScopeSelection`：

```json
{
  "mode":"TARGETED",
  "target":{"symbol":"OrderController.approve","kind":"METHOD"},
  "selectedCallChains":[...],
  "scopedFiles":[...]
}
```

25. `selectedCallChains` 必须是 ChangeAnalysis confirmed chains 的真实子集。
26. `scopedFiles` 必须从 `symbolLocations[]` 的 exact repository path 推导：
    - selectedCallChains 每个项目内部 symbol 的 exact path 都必须包含；
    - 只有通过 `from` 或 target-class 关系有确定性证据的额外文件才可加入；
    - 与 target 无关的 changed files 留在 Change Set，但不进入 scopedFiles；
    - basename 相同但 exact path 不同的文件不得替代。
27. 读取所有 scopedFiles 及 selectedCallChains 必需的项目内部文件；任一缺失都必须 PARTIAL。
28. 把 ReviewScopeSelection 交给 Controlled Runtime：

```text
validate
--schema .code-harness/contracts/review-scope.schema.json
--input <review-scope.json>
--format json
--change-analysis <change-analysis.json>
```

Runtime 必须先验证 `review-scope.schema.json` 与 `change-analysis.schema.json`，再执行 `reviewscope.Verify`：

- selected chain 是 confirmed chain；
- Controller CLASS/METHOD 不漏 required confirmed chains；
- `scopedFiles` 与 `symbolLocations` exact path 一致；
- selected internal symbol exact path 全部进入 scopedFiles；
- scoped coverage COMPLETE。

机器结果不是 COMPLETE → `MANUAL_ACTION_REQUIRED` / STOP。
29. TARGETED 不允许直接调用 FULL `change-analysis` coverage 去要求所有 unrelated changed files 被读取；它的 required set 是经过机器验证的 `scopedFiles`。
30. TARGETED 不得仅凭 Agent 自报的 `reviewCoverage.status=COMPLETE` 放行。

## E. `harness review list`

31. LIST 只建立 Change Set + confirmed/candidate/unresolved chain 信息。
32. 输出分组：

```text
已确认调用链
候选/未解析
```

33. 不调用 `review-code`，不产生 Finding，不把 candidate/unresolved 伪装为 confirmed。

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
- `symbolLocations[]`（TARGETED 必须有足够 exact navigation evidence；FULL 可继续兼容没有该可选字段的旧 transport）
- `externalDependencies[]`
- `riskAreas[]`
- `reviewCoverage`

TARGETED 额外生成独立 `ReviewScopeSelection`，必须符合 `.code-harness/contracts/review-scope.schema.json` 并经过 Runtime 对照 ChangeAnalysis 验证。

## 禁止行为

- 不得跳过 Change Set 计算。
- FULL 不得跳过 changed source/test files。
- TARGETED 不得把 scope 外 changed files 标为 reviewed。
- 不得通过类名/文件名猜实现路径或 scopedFiles。
- 不得用同 basename 的其他模块文件替代 Navigation 返回的 exact path。
- 不得直接调用 `ast-grep.exe`。
- 不得跳过 Runtime machine gate 并相信 Agent COMPLETE。
- 不得 sampled review。
- 不得执行任意 Shell、`git fetch` 或 `git pull`。
- 不得修改文件。
