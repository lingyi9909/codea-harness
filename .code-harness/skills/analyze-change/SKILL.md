---
name: analyze-change
description: 计算完整 Review Change Set，并按 FULL/TARGETED 意图通过受控 Code Navigation 与资源关系证据建立可机器验证的 Review Scope。
version: 4
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

所有 Review 都先计算同一个完整 Change Set：`merge-base(baseRef, HEAD) → HEAD` 的 committed，加上 staged、unstaged、untracked。1.4 正式 Review Scope 包含：

```text
sourceIncludes -> src/main/java/**/*.java
testIncludes   -> src/test/java/**/*.java
mapperIncludes -> src/main/resources/**/*Mapper.xml
configIncludes -> src/main/resources/**/*.yml
```

只新增 `mapperIncludes` 与 `configIncludes`；不得把 properties、pom.xml、Gradle、SQL migration 或任意 XML 扩进 Task 2。

之后根据意图分为：

```text
FULL      → 完整读取并机器验证整个 Change Set required scope
TARGETED  → 从完整 Change Set 的 confirmed callChains 中解析 target，只读取并机器验证 selectedCallChains + evidence-related resources + scopedFiles
LIST      → 仅列调用链，不执行 Finding Review
```

TARGETED 不是 sampled review。只有它声明的全部 scopedFiles 与 selectedCallChains 被完整覆盖后才允许 COMPLETE；不得把 TARGETED 结果表述成整个 Change Set 已评审。

## 关键原则

1. Change Set 是唯一入口，不因 target 改变 committed/staged/unstaged/untracked 的计算语义。
2. FULL 必须读取所有匹配 source/test/mapper/config scope 的 changedFiles；Mapper.xml/YML 不能静默跳过。
3. TARGETED 允许与 target 无关的 changed file 留在 Change Set 但不进入 scopedFiles；这些文件不计为 reviewed。
4. `selectedCallChains` 必须逐条来自 `ChangeAnalysis.callChains[]`，不得编造或压平。
5. 每次确定性 Code Navigation 都必须把 Java `symbol → exact repository path + role + source` 固化到 `ChangeAnalysis.symbolLocations[]`；由另一个已确认 symbol 导航得到时可记录 `from`。
6. Mapper.xml/YML 与 Java target/call-chain 的关系必须单独固化到 `ChangeAnalysis.resourceRelations[]`；不得把资源伪装成 Java symbolLocation。
7. Java scopedFiles 必须使用 `symbolLocations[]` exact path；资源 scopedFiles 必须使用 `resourceRelations[]` exact path + relation evidence。
8. 符号定位只能使用 `find_symbol` / `find_references` / `find_implementations`；不得靠 basename/简单类名猜路径。
9. Agent 声明 COMPLETE 不是事实，必须经过 Runtime machine gate。
10. **不允许 sampled review** 进入 COMPLETE/PASSED。

## A. 建立完整 Change Set

1. 读取 `harness.yaml.review.baseRef` / `includeWorkingTree` 以及 `scope.sourceIncludes/testIncludes/mapperIncludes/configIncludes`。
2. 校验 baseRef 本地存在；不存在 → `MANUAL_ACTION_REQUIRED`，不得自动换 main/develop。
3. 使用 `git_diff` 获取 committed/staged/unstaged/untracked。
4. 同路径多来源合并去重，保留 `sources`。
5. 对匹配资源 scope 的 changed file 使用显式 role：

```text
*Mapper.xml                         -> MapperXml
src/main/resources/**/*.yml        -> YamlConfig
```

不得把这两类官方 Scope 降级为 `Other`。
6. 生成完整 `changedFiles[]`；TARGETED 也不得丢掉 scope 外 changed file 的 Change Set 元数据。

## B. 建立 confirmed callChains 与 Navigation Evidence

7. 从 Diff 和必要的源码证据识别 changed symbols。
8. Controller 变更：从 Controller method 向下导航 Service/实现/Repository/Mapper。
9. Service/Repository 等下游变更：使用 `find_references` 反向定位上游 Controller/Service，同时向下导航到停止边界。
10. 接口必须 `find_implementations`；找不到实现记录 `IMPLEMENTATION_NOT_FOUND`，不得假装 confirmed。
11. 允许多层 Service，直到 Repository/Mapper/DAO 或明确外部边界。
12. 每个确定性 Code Navigation 结果记录到 `symbolLocations[]`：

```json
{
  "symbol":"OrderMapper.updateStatus",
  "path":"src/main/java/com/acme/order/OrderMapper.java",
  "role":"Mapper",
  "source":"FIND_SYMBOL"
}
```

13. `path` 必须是 Navigation 返回并实际读取/确认的 repository-relative exact path；同一个 symbol 得到两个不同 exact path/role 时必须视为 ambiguity。
14. confirmed chain 写入 `callChains[]`；candidate/unresolved 只进入 unresolved/limitation。
15. `affectedControllers[]` 仍必须提供真实 `impactType/sourceSymbols` 证据。

## C. Resource Relation Evidence

### Mapper.xml

16. 对 changed `*Mapper.xml` 读取 XML，并把当前变更涉及的 statement 与 Java Mapper method 做证据关联。确认关系时写入：

```json
{
  "path":"src/main/resources/mapper/OrderMapper.xml",
  "role":"MapperXml",
  "resource":"OrderMapper.xml#updateStatus",
  "fromSymbol":"OrderMapper.updateStatus",
  "fromKind":"METHOD",
  "source":"MAPPER_STATEMENT",
  "evidence":"statement id updateStatus matches OrderMapper.updateStatus"
}
```

17. statement id / namespace / Java Mapper method 无法确定性对应时，不得伪造 relation；记录 limitation/unresolved，必要时使声明 Scope PARTIAL。
18. 动态 SQL 无法归一化时可以保留为待人工/LLM 结合源码证据判断，不得把不确定语义包装为确定性事实。

### YML

19. changed `.yml` 只有在能证明 key 与 Java target/call-chain consumer 存在直接关系时才写 `CONFIG_REFERENCE`：

```json
{
  "path":"src/main/resources/application.yml",
  "role":"YamlConfig",
  "resource":"order.timeout-ms",
  "fromSymbol":"OrderService",
  "fromKind":"CLASS",
  "source":"CONFIG_REFERENCE",
  "evidence":"OrderService configuration binding consumes order.timeout-ms"
}
```

证据可来自已读取代码中的 `@Value`、`@ConfigurationProperties` 或明确配置绑定关系。不得仅因 key 名相似建立 relation。
20. `resourceRelations[].path` 必须是 Change Set 中的 exact repository-relative resource path；`evidence` 不得为空。

## D. FULL Review

21. 对所有匹配 source/test/mapper/config scope 的 changed file 读取完整内容。
22. 对 FULL 所有相关内部 call-chain 文件读取完整内容，非 changed Java 文件记 `reason: CALL_CHAIN`。
23. changed Mapper.xml/YML 读取后进入 `reviewCoverage.reviewedFiles[]`，role 分别为 `MapperXml` / `YamlConfig`，reason=`CHANGED`。
24. `reviewCoverage.status=COMPLETE` 仅当：
    - 全 changed Java/test/Mapper.xml/YML required files 已读；
    - 全部相关项目内部 chain symbols 已解析/读取；
    - 外部边界已明确列为 `externalDependencies`；
    - `unresolvedSymbols` 为空。
25. 组装 ChangeAnalysis 后用 `change-analysis.schema.json` + Runtime FULL Coverage 验证；不得跳过 `coverage.VerifyAnalysisJSON`。

## E. TARGETED Review

TARGETED 的选择数据继续使用独立 `.code-harness/contracts/review-scope.schema.json`；Task 2 只扩展该 Scope 的 Resource Evidence，不改变 Task 1 的 ReviewScopeSelection Contract。

26. 从 confirmed `ChangeAnalysis.callChains[]` 解析 target，并用 `symbolLocations[].role` 判断 target 属于 Controller 还是 Service/其他下游角色；不得靠命名后缀猜角色。
27. 多链语义保持 Task 1：

```text
Controller CLASS  → 自动包含该 Controller 当前 Change Set 中全部相关 confirmed chains
Controller METHOD → 自动包含该 method 当前 Change Set 中全部相关 confirmed chains
Service/其他下游 target → 1 条链自动继续；2+ 条上游业务链才要求用户选择
```

28. `selectedCallChains` 必须是 confirmed chains 的真实子集。
29. Java `scopedFiles` 从 `symbolLocations[]` exact path 推导；selected internal symbol 的 exact path 全部必须包含。
30. 对资源文件，Runtime 只接受以下同时成立的 relation：

```text
resource file ∈ changedFiles
resourceRelations.path == exact changed resource path
resourceRelations.fromSymbol 命中 selectedCallChains（METHOD exact match；CLASS 按 selected node 的 class match）
role/source 合法：MapperXml/MAPPER_STATEMENT 或 YamlConfig/CONFIG_REFERENCE
```

31. 满足上述条件的 changed Mapper.xml/YML 必须加入 TARGETED scopedFiles；遗漏时 Runtime 拒绝。与 selected chain 无关的 changed resource 必须留在完整 Change Set，但不得加入本次 scopedFiles。
32. **无法证明关联时不得加入 TARGETED scopedFiles**；不得为了“多看一点”把 UserMapper.xml 或无关 YML 塞进定向 Scope。
33. 读取所有 verified scopedFiles；任一缺失都必须 PARTIAL。
34. Controlled Runtime `reviewscope.Verify` 必须重新验证：selected chain、Controller 防漏链、Java exact path、resource relation exact path、required resources、scoped coverage。
35. TARGETED 不允许使用 FULL Coverage 去要求 unrelated changed files 被读取；但也不得仅凭 Agent 自报 COMPLETE 放行。

## F. `harness review list`

36. LIST 只建立 Change Set + confirmed/candidate/unresolved chain 信息。
37. 不调用 `review-code`，不产生 Finding，不把 candidate/unresolved 伪装为 confirmed。

## 停止边界

向下 Java 导航仍在以下边界停止：Repository / Mapper / DAO、明确 RPC/HTTP/MQ/Cache/第三方 SDK、JDK/Spring Framework。Task 2 允许读取 changed Mapper.xml/YML 本身，但不因此继续深入 DB、Redis、MQ、RPC 内部链路。

## 输出

ChangeAnalysis 继续符合 `.code-harness/contracts/change-analysis.schema.json`，可包含：

- `reviewScope`
- `changedFiles[]`
- `affectedControllers[]`
- `callChains[]`
- `symbolLocations[]`
- `resourceRelations[]`
- `externalDependencies[]`
- `riskAreas[]`
- `reviewCoverage`

TARGETED 额外生成的 ReviewScopeSelection 必须继续通过 `.code-harness/contracts/review-scope.schema.json`，然后由 Controlled Runtime 对照 ChangeAnalysis 验证。

`resourceRelations[]` 是可选证据集合；FULL 可没有 relation 但不能漏读 changed resources。TARGETED 只有 relation 被机器验证后才允许资源进入 Scope。

## 禁止行为

- 不得跳过 Change Set 计算。
- FULL 不得跳过 changed Java/test/Mapper.xml/YML required files。
- TARGETED 不得把 scope 外 changed files 标为 reviewed。
- 不得通过类名/文件名猜实现路径或 scopedFiles。
- 不得用同 basename 的其他模块文件替代 exact path。
- 不得把 properties、pom.xml、Gradle、SQL migration 或任意 XML 加入 Task 2 Resource Review。
- 不得直接调用 `ast-grep.exe`。
- 不得跳过 Runtime machine gate 并相信 Agent COMPLETE。
- 不得 sampled review。
- 不得执行任意 Shell、`git fetch` 或 `git pull`。
- 不得修改文件。
