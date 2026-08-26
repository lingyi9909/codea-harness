---
name: analyze-change
description: 计算完整 Review Change Set，并按 FULL/TARGETED 意图通过受控 Code Navigation 与资源关系证据建立可机器验证的 Review Scope。
version: 6
agent: reviewer
tools:
  - git_diff
  - read_code
  - find_symbol
  - find_references
  - find_implementations
  - workspace_verify
  - workspace_inherited
  - workspace_superclass_call
  - workspace_template_dispatch
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
8. current project 符号定位优先使用 `find_symbol` / `find_references` / `find_implementations`；只有在 superclass/template inheritance 导致 current-project 导航断链时，才允许进入下述 VERIFIED workspace fallback。不得靠 basename/简单类名猜路径。
9. Agent 声明 COMPLETE 不是事实，必须经过 Runtime machine gate。
10. **不允许 sampled review** 进入 COMPLETE/PASSED。
11. 1.5.3 起本 Skill 产出的 ChangeAnalysis 只是 proposal；Agent 只能写同 run `requests/change-analysis-draft.json`，不得直接写 authoritative `analysis/change-analysis.json` / `entrypoint-inventory.json` / `change-analysis.cert.json`。
12. 后续 Review/Chain 只能消费 Controlled Runtime `analysis certify` 生成并经 `LoadCertified` 验证的 Certified ChangeAnalysis。

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

### Workspace inheritance fallback（1.5.2 Task 4）

当且仅当 current-project Code Navigation 已沿当前代码确认到 superclass / inherited method / template method 边界，且 `find_symbol` / `find_references` / `find_implementations` 无法继续确定性闭合调用链时，允许进入 workspace fallback：

1. 只读取显式 `harness.yaml.workspaceDependencies`；未配置即停止为机器 limitation，**不得扫描任意 sibling**，不得遍历 `../**` 猜依赖源码。
2. 对显式候选 dependency 先执行受控 `workspace verify <id>`（tool=`workspace_verify`）。只有 `VERIFIED` 才允许 workspace navigation；VERSION_UNRESOLVED / COORDINATE_MISMATCH / VERSION_MISMATCH / SOURCE_NOT_FOUND 均不得读取该 workspace 源码作为 confirmed evidence。
3. VERIFIED 后只允许调用 Controlled Runtime 的三类确定性导航（对应 tools=`workspace_inherited` / `workspace_superclass_call` / `workspace_template_dispatch`）：

```text
codea-harness-tools nav workspace-inherited --workspace <id> --from <symbol> --method <method>
codea-harness-tools nav workspace-superclass-call --workspace <id> --from <symbol> --method <method>
codea-harness-tools nav workspace-template-dispatch --workspace <id> --from <symbol> --hook <hook> [--concrete <class>]
```

4. Runtime 返回 COMPLETE fact 时只做事实拷贝，不补猜、不改写；写入 `ChangeAnalysis.symbolLocations[]`：

```text
workspace
symbol
exact path
role
source=WORKSPACE_INHERITANCE
from
```

例如：

```json
{
  "workspace":"company-framework",
  "symbol":"AbstractTemplate.execute",
  "path":"src/main/java/com/company/framework/AbstractTemplate.java",
  "role":"Service",
  "source":"WORKSPACE_INHERITANCE",
  "from":"XxxServiceImpl.submit"
}
```

5. workspace fact 只用于 Navigation / Chain Context；若 template dispatch 唯一回到 current-project override，则继续使用 Runtime 返回的 `workspace=current` fact，并**继续 confirmed callChain**，直到现有 Repository/Mapper/外部停止边界。
6. 任一 workspace nav 返回 PARTIAL / ambiguity 时，按其机器 code 写入 limitation/unresolved；不得把候选包装成 confirmed callChain。
7. workspace dependency 绝不进入 `changedFiles[]`、`resourceRelations[]`、Review Scope 或 Write Scope；其路径始终是 dependency workspace 内的 exact relative path，不使用 `../company-framework/...` 伪装 current path。

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
22. 对 FULL 所有相关内部 call-chain 文件读取完整内容，非 changed Java 文件记 `reason: CALL_CHAIN`；这里的“内部”只指 current workspace。
23. changed Mapper.xml/YML 读取后进入 `reviewCoverage.reviewedFiles[]`，role 分别为 `MapperXml` / `YamlConfig`，reason=`CHANGED`。
24. `reviewCoverage.reviewedFiles[]` 的合法来源固定为：`changedFiles[]`、`workspace=current`（或旧证据缺省 current）的 `symbolLocations[].path`、current project `resourceRelations[].path`。`workspace != current` 的 dependency source 即使已用于 Navigation/Chain Context，也不得进入 reviewedFiles。
25. `reviewCoverage.status=COMPLETE` 仅当：
    - 全 changed Java/test/Mapper.xml/YML required files 已读；
    - 全部相关 current-project 内部 chain symbols 已解析/读取；
    - dependency workspace 仅作为 Navigation / Chain Context，不作为 Review 文件；
    - 外部边界已明确列为 `externalDependencies`；
    - `unresolvedSymbols` 为空。
26. 组装 ChangeAnalysis draft 后先按 schema 自检，但最终必须交给 Runtime `analysis certify` 重新计算并执行 FULL Coverage；不得仅凭 Agent `coverage.VerifyAnalysisJSON` 结果把 draft 视为 authoritative。Runtime 必须拒绝 dependency workspace path 混入 `reviewCoverage.reviewedFiles[]`。

## E. TARGETED Review

TARGETED 的选择数据继续使用独立 `.code-harness/contracts/review-scope.schema.json`；Task 2 只扩展该 Scope 的 Resource Evidence，不改变 Task 1 的 ReviewScopeSelection Contract。

27. 从 confirmed `ChangeAnalysis.callChains[]` 解析 target，并用 `symbolLocations[].role` 判断 target 属于 Controller 还是 Service/其他下游角色；不得靠命名后缀猜角色。
28. 多链语义保持 Task 1：

```text
Controller CLASS  → 自动包含该 Controller 当前 Change Set 中全部相关 confirmed chains
Controller METHOD → 自动包含该 method 当前 Change Set 中全部相关 confirmed chains
Service/其他下游 target → 1 条链自动继续；2+ 条上游业务链才要求用户选择
```

29. `selectedCallChains` 必须是 confirmed chains 的真实子集。
30. Java `scopedFiles` 从 `symbolLocations[]` exact path 推导；selected internal symbol 的 exact path 全部必须包含。
31. 对资源文件，Runtime 只接受以下同时成立的 relation：

```text
resource file ∈ changedFiles
resourceRelations.path == exact changed resource path
resourceRelations.fromSymbol 命中 selectedCallChains（METHOD exact match；CLASS 按 selected node 的 class match）
role/source 合法：MapperXml/MAPPER_STATEMENT 或 YamlConfig/CONFIG_REFERENCE
```

32. 满足上述条件的 changed Mapper.xml/YML 必须加入 TARGETED scopedFiles；遗漏时 Runtime 拒绝。与 selected chain 无关的 changed resource 必须留在完整 Change Set，但不得加入本次 scopedFiles。
33. **无法证明关联时不得加入 TARGETED scopedFiles**；不得为了“多看一点”把 UserMapper.xml 或无关 YML 塞进定向 Scope。
34. 读取所有 verified scopedFiles；任一缺失都必须 PARTIAL。
35. Controlled Runtime `reviewscope.Verify` 必须基于 Certified ChangeAnalysis 重新验证：selected chain、Controller 防漏链、Java exact path、resource relation exact path、required resources、scoped coverage。
36. TARGETED 不允许使用 FULL Coverage 去要求 unrelated changed files 被读取；但也不得仅凭 Agent 自报 COMPLETE 放行。

## F. `harness review list`

37. LIST 只建立 Change Set + confirmed/candidate/unresolved chain 信息。
38. 不调用 `review-code`，不产生 Finding，不把 candidate/unresolved 伪装为 confirmed。

## 停止边界

向下 Java 导航仍在以下边界停止：Repository / Mapper / DAO、明确 RPC/HTTP/MQ/Cache/第三方 SDK、JDK/Spring Framework。Task 2 允许读取 changed Mapper.xml/YML 本身，但不因此继续深入 DB、Redis、MQ、RPC 内部链路。

## 1.5.3 Certification Boundary

完成上述分析后，本 Skill **只写 proposal**：

```text
.code-harness/runs/<runId>/requests/change-analysis-draft.json
```

随后创建同 run certify request，并由 Controlled Runtime 执行：

```text
codea-harness-tools analysis certify --input .code-harness/runs/<runId>/requests/<certify-request>.json
```

Runtime 固定执行：重新计算当前 Change Set → 重新生成 EntryPoint Inventory → exact changedFiles/source 对比 → EntryPoint completeness → symbol/resource evidence invariants → Coverage → canonical hash/certificate → 原子发布。

成功后唯一权威产物为：

```text
.code-harness/runs/<runId>/analysis/change-analysis.json
.code-harness/runs/<runId>/analysis/entrypoint-inventory.json
.code-harness/runs/<runId>/analysis/change-analysis.cert.json
```

Agent 不得直接创建、覆盖、补写或修复这三个文件。Certification 返回 `ENTRYPOINT_COMPLETENESS_INCOMPLETE`、`CHANGE_SET_MISMATCH`、evidence invariant failure 或其他错误时，固定停止为 `MANUAL_ACTION_REQUIRED` / `PARTIAL`；不得偷偷删除漏掉的 endpoint、changed file 或 evidence 后继续。后续如 Change Set 变化，原 certificate 视为 stale，必须重新分析并重新 certify。

## 输出

Proposal 的 JSON 结构继续符合 `.code-harness/contracts/change-analysis.schema.json`，可包含：

- `reviewScope`
- `changedFiles[]`
- `affectedControllers[]`
- `callChains[]`
- `symbolLocations[]`
- `resourceRelations[]`
- `externalDependencies[]`
- `riskAreas[]`
- `reviewCoverage`

但只有 `analysis certify` 成功后，同 run `analysis/change-analysis.json` 才能被称为 Certified ChangeAnalysis；`requests/change-analysis-draft.json` 永远不能直接作为 Review/Chain authority。

TARGETED 额外生成的 ReviewScopeSelection 必须继续通过 `.code-harness/contracts/review-scope.schema.json`，然后由 Controlled Runtime 对照 Certified ChangeAnalysis 验证。

`resourceRelations[]` 是可选证据集合；FULL 可没有 relation 但不能漏读 changed resources。TARGETED 只有 relation 被机器验证后才允许资源进入 Scope。

## Lazy Chain Discovery（1.5 Task 2）

`analyze-change` 仍然负责建立唯一的证据 proposal；Chain Discovery 不重新解析 Java，也不得覆盖 Certified ChangeAnalysis 事实。

当用户发起：

```text
harness chain discover
harness chain discover <Class>
harness chain discover <Class.method>
```

必须先完成当前 Change Set 的 draft 并通过 `analysis certify`。之后 `discover-chain` 只消费 Certified：

```text
ChangeAnalysis.affectedControllers[]
ChangeAnalysis.callChains[]
ChangeAnalysis.symbolLocations[]
ChangeAnalysis.resourceRelations[]
ChangeAnalysis.externalDependencies[]
ChangeAnalysis.reviewCoverage.unresolvedSymbols[]
```

EntryPoint 只允许 **生产 Controller Method**。exact path 与 role 必须来自 Certified `ChangeAnalysis.symbolLocations[]`；Mapper.xml/YML 只允许来自 Certified `ChangeAnalysis.resourceRelations[]`。不得根据类名后缀、basename 或同名文件猜 Controller/Service/Impl/Mapper role/path。

Lazy 范围固定：无 target 只处理当前 Change Set 的 affectedControllers；有 target 只处理当前 confirmed callChains 中与 target 有关系的入口/分支。Service 等下游 target 可以沿 verified callChains 向上解析生产 Controller Method，但不得因此全仓扫描所有 Controller。

存在内部 unresolved、entry/core exact path ambiguity 或缺失 confirmed path 时，Discovery 必须 `PARTIAL`；不得为了生成 Chain 补猜 evidence。V1/V2 只有 verified core facts 完全一致时才允许合并 entryPoints，不使用名称相似度或 fuzzy threshold。

Task 2 发现结果只写：

```text
.code-harness/runs/<runId>/analysis/discovered-chains/<id>.yaml
```

即 `runs/<runId>/analysis/discovered-chains/`。所有结果保持 `status: DISCOVERED`；不得写 `.code-harness/chains/**`，不得提前执行 Task 3 的 validate/accept/refresh。

## Review Chain Context（1.5 Task 4）

Review 使用 Chain 时仍必须**先完成 Certified ChangeAnalysis**；Chain 解析不建立第二套 Java/resource 事实源。

固定顺序：

1. 按本 Skill 原流程建立完整 Change Set、ChangeAnalysis draft、symbolLocations/resourceRelations/callChains。
2. 调用 `analysis certify`，获得同 run Certified ChangeAnalysis；失败立即停止。
3. FULL 使用 Certified ChangeAnalysis 的 FULL machine coverage；TARGETED 基于 Certified ChangeAnalysis 生成并通过 Runtime verified ReviewScopeSelection + scoped coverage。
4. 将已经验证的 Review Scope 连同同 run `changeAnalysisPath` 写入 controlled request。
5. 由 Orchestrator 调用：

```text
codea-harness-tools chain review-context --input .code-harness/runs/<runId>/requests/chain-review-context.json
```

6. Runtime 才能决定复用 `ACCEPTED + VALID`、缺失时 lazy discover `DISCOVERED + TEMPORARY`，或返回 STALE/partial 决策状态。

**不得因为存在 Chain 而减少 changedFiles**；FULL 的 Change Set 与 required coverage 完全不变。
**不得因为存在 Chain 而减少 scopedFiles**；TARGETED 的 scopedFiles 仍只能由 Certified ChangeAnalysis + ReviewScopeSelection 决定。

Chain context 只能补充业务理解；不得反向修改 ChangeAnalysis 的 symbol/path/role/resource relation，不得用 `.code-harness/chains/*.yaml` 替代 Code Navigation evidence。

## 禁止行为

- 不得跳过 Change Set 计算。
- 不得把 `requests/change-analysis-draft.json` 当成权威 ChangeAnalysis。
- 不得直接写/改 Runtime-owned `analysis/change-analysis.json`、`analysis/entrypoint-inventory.json`、`analysis/change-analysis.cert.json`。
- 不得跳过 `analysis certify`。
- FULL 不得跳过 changed Java/test/Mapper.xml/YML required files。
- FULL 不得把 `workspace != current` 的 dependency source 写入 `reviewCoverage.reviewedFiles[]`。
- TARGETED 不得把 scope 外 changed files 标为 reviewed。
- 不得通过类名/文件名猜实现路径或 scopedFiles。
- 不得用同 basename 的其他模块文件替代 exact path。
- 不得扫描任意 sibling；workspace source 只能来自显式 `harness.yaml.workspaceDependencies` + Runtime `VERIFIED`。
- 不得把 properties、pom.xml、Gradle、SQL migration 或任意 XML 加入 Task 2 Resource Review。
- 不得直接调用 `ast-grep.exe`。
- 不得跳过 Runtime machine gate 并相信 Agent COMPLETE。
- 不得 sampled review。
- 不得执行任意 Shell、`git fetch` 或 `git pull`。
- 不得修改生产代码或测试代码。
