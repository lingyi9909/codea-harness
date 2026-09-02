---
name: analyze-change
description: 消费 Runtime Canonical ChangeSet Snapshot，并按 FULL/TARGETED 意图通过受控 Code Navigation 与资源关系证据建立可机器验证的 semantic ChangeAnalysis Proposal。
version: 7
agent: reviewer
tools:
  - read_code
  - find_symbol
  - find_references
  - find_implementations
  - workspace_verify
  - workspace_inherited
  - workspace_superclass_call
  - workspace_template_dispatch
  - validate_contract
output_schema: .code-harness/contracts/change-analysis-proposal.schema.json
---

# 分析代码变更

## 目标

所有 Review 都先由 Controlled Runtime 生成同一个 Canonical ChangeSet Snapshot：

```text
analysis snapshot request
→ Runtime resolve baseRef
→ Snapshot requestedBaseRef provenance
→ resolvedBaseCommit / mergeBase / headCommit / currentBranch
→ committed + staged + unstaged + untracked
→ Runtime Review Scope filtering
→ canonical files[] + gitStateSha256 + snapshotSha256
→ .code-harness/runs/<runId>/analysis/change-set.json
```

本 Skill **不再计算 Git Change Set**。Agent 只能消费 same-run Runtime Snapshot；不能调用 `git_diff` 再生成第二套 Git facts，也不能自行生成/覆盖：

```text
baseRef identity
baseCommit
mergeBase
headCommit
currentBranch
includeWorkingTree
changedFiles.path
changedFiles.sources
```

Canonical Snapshot 的 Review Scope 继续保持：

```text
sourceIncludes -> src/main/java/**/*.java
testIncludes   -> src/test/java/**/*.java
mapperIncludes -> src/main/resources/**/*Mapper.xml
configIncludes -> src/main/resources/**/*.yml
```

不得把 properties、pom.xml、Gradle、SQL migration 或任意普通 XML 扩进 Review ChangeSet。非 Review Scope 文件可以同时存在于 Git working state，但不会进入 Runtime canonical `files[]`，Agent 不得把它们补回 proposal。

之后根据意图分为：

```text
FULL      → 完整读取并机器验证整个 Canonical ChangeSet required scope
TARGETED  → 从 Canonical ChangeSet 的 confirmed callChains 中解析 target，只读取并机器验证 selectedCallChains + evidence-related resources + scopedFiles
LIST      → 仅列调用链，不执行 Finding Review
```

TARGETED 不是 sampled review。只有它声明的全部 scopedFiles 与 selectedCallChains 被完整覆盖后才允许 COMPLETE；不得把 TARGETED 结果表述成整个 Change Set 已评审。

## 关键原则

1. Runtime Canonical Snapshot 是 Change Set 唯一入口与唯一 deterministic authority，不因 target 改变 committed/staged/unstaged/untracked 的计算语义。
2. `requestedBaseRef` 只作为 provenance。Agent 不比较 `main/origin/main/refs/heads/main` 字符串来判断 Git identity；Runtime 只有在它们实际 resolve 到相同 commit 且其余 canonical state 相同时才认定 identity 等价。
3. FULL 必须读取 Snapshot 中所有匹配 source/test/mapper/config scope 的 files；Mapper.xml/YML 不能静默跳过。
4. TARGETED 允许与 target 无关的 canonical changed file 留在 Change Set 但不进入 scopedFiles；这些文件不计为 reviewed。
5. `selectedCallChains` 必须逐条来自后续 Certified `ChangeAnalysis.callChains[]`，不得编造或压平。
6. 每次确定性 Code Navigation 都必须把 Java `symbol → exact repository path + role + source` 固化到 semantic proposal 的 `symbolLocations[]`；由另一个已确认 symbol 导航得到时可记录 `from`。
7. Mapper.xml/YML 与 Java target/call-chain 的关系必须单独固化到 semantic proposal 的 `resourceRelations[]`；不得把资源伪装成 Java symbolLocation。
8. Java scopedFiles 必须使用 `symbolLocations[]` exact path；资源 scopedFiles 必须使用 `resourceRelations[]` exact path + relation evidence。
9. current project 符号定位优先使用 `find_symbol` / `find_references` / `find_implementations`；只有在 superclass/template inheritance 导致 current-project 导航断链时，才允许进入下述 VERIFIED workspace fallback。不得靠 basename/简单类名猜路径。
10. Agent 声明 COMPLETE 不是事实，必须经过 Runtime machine gate。
11. **不允许 sampled review** 进入 COMPLETE/PASSED。
12. 本 Skill 只写同 run `.code-harness/runs/<runId>/requests/change-analysis-proposal.json`。它只是 semantic proposal，不是 authoritative ChangeAnalysis。
13. 后续 Review/Chain 只能消费 Controlled Runtime `analysis certify` 生成并经 `LoadCertified` 验证的 Certified ChangeAnalysis。

## A. 消费 Runtime Canonical ChangeSet Snapshot

1. Orchestrator 先解析用户/配置中的 `baseRef` 与 `includeWorkingTree`，只把这两个请求参数连同 `runId` 交给 Controlled Runtime `analysis snapshot`；本 Skill 不负责 Git ref resolution。Agent-facing request JSON 固定为：

```json
{
  "runId": "<runId>",
  "baseRef": "<baseRef>",
  "includeWorkingTree": true
}
```

Runtime Snapshot artifact 中的 `requestedBaseRef` 只保存上述 `baseRef` 请求值作为 provenance，不是 Agent-facing request 字段。
2. Runtime Snapshot 必须位于：

```text
.code-harness/runs/<runId>/analysis/change-set.json
```

并通过 `.code-harness/contracts/change-set.schema.json`。
3. 本 Skill 只读取 Snapshot 的：

```text
requestedBaseRef
resolvedBaseCommit
mergeBase
headCommit
currentBranch
includeWorkingTree
files[].path/status/sources/hunks
gitStateSha256
snapshotSha256
```

其中 identity fields 只用于 provenance/后续 request 引用，Agent 不得改写。
4. `files[]` 就是完整 Review ChangeSet。不得调用 `git_diff`、`git status`、`git merge-base` 或其他 Git 推导来补充/删减 canonical files。
5. 对 Snapshot 中每个 canonical file 建立 semantic `changedFileRoles[]` 引用。`path` 必须逐字引用 Snapshot path，不能提出 Snapshot 中不存在的 path；Runtime certify 会要求 role 引用与 canonical files 一一对应。
6. role 规则保持：

```text
src/test/java/**/*.java             -> Test
*Mapper.xml                          -> MapperXml
src/main/resources/**/*.yml         -> YamlConfig
```

`src/test/**` 必须映射为 `Test`，非 `src/test/**` 不得声明为 `Test`；MapperXml/YamlConfig 也不得降级为 `Other`。Java 生产文件继续按已有 semantic evidence 确认 Controller/Service/Repository/Mapper/Other role。
7. 同一路径多 source 的合并、`status/sources/hunks` 全部由 Runtime Snapshot 提供；Agent 不复制到 `changedFileRoles[]`，也不得重算。

## B. 建立 confirmed callChains 与 Navigation Evidence

8. 从 Snapshot 的 changed path/hunks 与必要源码证据识别 changed symbols。
9. Controller 变更：从 Controller method 向下导航 Service/实现/Repository/Mapper。
10. Service/Repository 等下游变更：使用 `find_references` 反向定位上游 Controller/Service，同时向下导航到停止边界。
11. 接口必须 `find_implementations`；找不到实现记录 `IMPLEMENTATION_NOT_FOUND`，不得假装 confirmed。
12. 允许多层 Service，直到 Repository/Mapper/DAO 或明确外部边界。
13. 每个确定性 Code Navigation 结果记录到 `symbolLocations[]`：

```json
{
  "symbol":"OrderMapper.updateStatus",
  "path":"src/main/java/com/acme/order/OrderMapper.java",
  "role":"Mapper",
  "source":"FIND_SYMBOL"
}
```

14. `path` 必须是 Navigation 返回并实际读取/确认的 repository-relative exact path；同一个 symbol 得到两个不同 exact path/role 时必须视为 ambiguity。
15. confirmed chain 写入 `callChains[]`；candidate/unresolved 只进入 unresolved/limitation。
16. `affectedControllers[]` 仍必须提供真实 `impactType/sourceSymbols` 证据。

### Workspace inheritance fallback（1.5.2 Task 4）

当且仅当 current-project Code Navigation 已沿当前代码确认到 superclass / inherited method / template method 边界，且 `find_symbol` / `find_references` / `find_implementations` 无法继续确定性闭合调用链时，允许进入 workspace fallback：

1. 只读取显式 `harness.yaml.workspaceDependencies`；未配置即停止为机器 limitation，**不得扫描任意 sibling**，不得遍历 `../**` 猜依赖源码。
2. 对显式候选 dependency 先执行受控 `workspace verify <id>`（tool=`workspace_verify`）。只有 `VERIFIED` 才允许 workspace navigation；VERSION_UNRESOLVED / COORDINATE_MISMATCH / VERSION_MISMATCH / SOURCE_NOT_FOUND 均不得读取该 workspace 源码作为 confirmed evidence。
3. VERIFIED 后只允许调用 Controlled Runtime 的三类确定性导航（对应 tools=`workspace_inherited` / `workspace_superclass_call` / `workspace_template_dispatch`）：

```text
codea-dcep-tools.exe nav workspace-inherited --workspace <id> --from <symbol> --method <method>
codea-dcep-tools.exe nav workspace-superclass-call --workspace <id> --from <symbol> --method <method>
codea-dcep-tools.exe nav workspace-template-dispatch --workspace <id> --from <symbol> --hook <hook> [--concrete <class>]
```

4. Runtime 返回 COMPLETE fact 时只做事实拷贝，不补猜、不改写；写入 semantic proposal 的 `symbolLocations[]`：

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
7. workspace dependency 绝不进入 Canonical Snapshot、`changedFileRoles[]`、`resourceRelations[]`、Review Scope 或 Write Scope；其路径始终是 dependency workspace 内的 exact relative path，不使用 `../company-framework/...` 伪装 current path。

## C. Resource Relation Evidence

### Mapper.xml

17. 对 Snapshot 中 changed `*Mapper.xml` 读取 XML，并把当前变更涉及的 statement 与 Java Mapper method 做证据关联。确认关系时写入：

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

18. statement id / namespace / Java Mapper method 无法确定性对应时，不得伪造 relation；记录 limitation/unresolved，必要时使声明 Scope PARTIAL。
19. 动态 SQL 无法归一化时可以保留为待人工/LLM 结合源码证据判断，不得把不确定语义包装为确定性事实。

### YML

20. Snapshot 中 changed `.yml` 只有在能证明 key 与 Java target/call-chain consumer 存在直接关系时才写 `CONFIG_REFERENCE`：

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
21. `resourceRelations[].path` 必须是 Canonical Snapshot 中的 exact repository-relative resource path；`evidence` 不得为空。

## D. FULL Review

22. 对 Canonical Snapshot 中所有 source/test/mapper/config files 读取完整内容。
23. 对 FULL 所有相关内部 call-chain 文件读取完整内容，非 changed Java 文件记 `reason: CALL_CHAIN`；这里的“内部”只指 current workspace。
24. changed Mapper.xml/YML 读取后进入 `reviewCoverage.reviewedFiles[]`，role 分别为 `MapperXml` / `YamlConfig`，reason=`CHANGED`。
25. `reviewCoverage.reviewedFiles[]` 的合法来源固定为：Canonical Snapshot files、`workspace=current`（或旧证据缺省 current）的 `symbolLocations[].path`、current project `resourceRelations[].path`。`workspace != current` 的 dependency source 即使已用于 Navigation/Chain Context，也不得进入 reviewedFiles。
26. `reviewCoverage.status=COMPLETE` 仅当：
    - 全 canonical Java/test/Mapper.xml/YML required files 已读；
    - 全部相关 current-project 内部 chain symbols 已解析/读取；
    - dependency workspace 仅作为 Navigation / Chain Context，不作为 Review 文件；
    - 外部边界已明确列为 `externalDependencies`；
    - `unresolvedSymbols` 为空。
27. 组装 semantic proposal 后先按 `.code-harness/contracts/change-analysis-proposal.schema.json` 自检；最终必须交给 Runtime `analysis certify`。Runtime 会从 Snapshot 自己组装 `reviewScope/changedFiles`，再执行 FULL Coverage；不得仅凭 Agent `reviewCoverage.status` 把 proposal 视为 authoritative。

## E. TARGETED Review

### 1.5.3 Review Selection Authority Override

本节旧的 1.4 target 解析规则继续定义“如何构造/验证 TARGETED scope”，但不再授权 Agent 自己决定 plain `harness review` 的模式。固定为：`analysis snapshot → semantic proposal → analysis certify → completeness gate → review options`；0 Chain=AUTO_FULL、1 Chain=AUTO_SINGLE、2+ Chain=USER_SELECTION。只有 2+ 且用户选择“按业务链评审”时才展示 Runtime C1..Cn。显式 Controller/Controller.method 继续 direct TARGETED，自动包含全部 machine-required branches，不展示 Controller/Chain 选择；显式 Service/其他下游 target 仅在 2+ 上游 Chain 时选择。所有最终 TARGETED scope 仍必须通过 Runtime `reviewscope.Verify`，Agent 不得发明 selectionId/optionsHash 或绕过 Controller 防漏链。

TARGETED 的选择数据继续使用独立 `.code-harness/contracts/review-scope.schema.json`；本 Hotfix 不改变 ReviewScopeSelection 业务语义。

28. 从 Certified `ChangeAnalysis.callChains[]` 解析 target，并用 `symbolLocations[].role` 判断 target 属于 Controller 还是 Service/其他下游角色；不得靠命名后缀猜角色。
29. 多链语义保持：

```text
Controller CLASS  → 自动包含该 Controller 当前 Change Set 中全部相关 confirmed chains
Controller METHOD → 自动包含该 method 当前 Change Set 中全部相关 confirmed chains
Service/其他下游 target → 1 条链自动继续；2+ 条上游业务链才要求用户选择
```

30. `selectedCallChains` 必须是 confirmed chains 的真实子集。
31. Java `scopedFiles` 从 `symbolLocations[]` exact path 推导；selected internal symbol 的 exact path 全部必须包含。
32. 对资源文件，Runtime 只接受以下同时成立的 relation：

```text
resource file ∈ canonical changedFiles
resourceRelations.path == exact changed resource path
resourceRelations.fromSymbol 命中 selectedCallChains（METHOD exact match；CLASS 按 selected node 的 class match）
role/source 合法：MapperXml/MAPPER_STATEMENT 或 YamlConfig/CONFIG_REFERENCE
```

33. 满足上述条件的 changed Mapper.xml/YML 必须加入 TARGETED scopedFiles；遗漏时 Runtime 拒绝。与 selected chain 无关的 changed resource 必须留在完整 Change Set，但不得加入本次 scopedFiles。
34. **无法证明关联时不得加入 TARGETED scopedFiles**；不得为了“多看一点”把 UserMapper.xml 或无关 YML 塞进定向 Scope。
35. 读取所有 verified scopedFiles；任一缺失都必须 PARTIAL。
36. Controlled Runtime `reviewscope.Verify` 必须基于 Certified ChangeAnalysis 重新验证：selected chain、Controller 防漏链、Java exact path、resource relation exact path、required resources、scoped coverage。
37. TARGETED 不允许使用 FULL Coverage 去要求 unrelated changed files 被读取；但也不得仅凭 Agent 自报 COMPLETE 放行。

## F. `harness review list`

38. LIST 只建立 Canonical Snapshot + confirmed/candidate/unresolved chain semantic information。
39. 不调用 `review-code`，不产生 Finding，不把 candidate/unresolved 伪装成 confirmed。

## 停止边界

向下 Java 导航仍在以下边界停止：Repository / Mapper / DAO、明确 RPC/HTTP/MQ/Cache/第三方 SDK、JDK/Spring Framework。Resource Review 允许读取 changed Mapper.xml/YML 本身，但不因此继续深入 DB、Redis、MQ、RPC 内部链路。

## 1.6.2 Certification Boundary

完成上述分析后，本 Skill **只写 semantic proposal**：

```text
.code-harness/runs/<runId>/requests/change-analysis-proposal.json
```

Proposal Contract 为：

```text
.code-harness/contracts/change-analysis-proposal.schema.json
```

它不能包含 Runtime-owned `reviewScope` 或 `changedFiles`。`changedFileRoles[].path` 只是对 Snapshot canonical path 的 semantic role 引用，必须与 Snapshot files 一一对应，不能产生新的 changed path。

随后创建同 run canonical certify request，至少携带：

```json
{
  "runId":"<runId>",
  "snapshotPath":".code-harness/runs/<runId>/analysis/change-set.json",
  "snapshotSha256":"<Runtime snapshotSha256>",
  "proposalPath":".code-harness/runs/<runId>/requests/change-analysis-proposal.json",
  "intent":{"mode":"FULL"}
}
```

由 Controlled Runtime 执行：

```text
codea-dcep-tools.exe analysis certify --input .code-harness/runs/<runId>/requests/<certify-request>.json
```

Runtime 固定执行：读取 canonical Snapshot → 重新计算 live Snapshot → 验证 `resolvedBaseCommit/mergeBase/headCommit/currentBranch/includeWorkingTree/gitStateSha256/snapshotSha256` → 重新生成 EntryPoint Inventory → 把 Runtime canonical path/source 与 Agent roles/semantic evidence 组装为 ChangeAnalysis → EntryPoint completeness → symbol/resource evidence invariants → Coverage → canonical hash/certificate → 原子发布。

成功后唯一权威产物为：

```text
.code-harness/runs/<runId>/analysis/change-analysis.json
.code-harness/runs/<runId>/analysis/entrypoint-inventory.json
.code-harness/runs/<runId>/analysis/change-analysis.cert.json
```

同时 `.code-harness/runs/<runId>/analysis/change-set.json` 继续是该次 certification 所绑定的 Runtime-owned Canonical Snapshot。

Agent 不得直接创建、覆盖、补写或修复任何 `analysis/**` authority artifact。Certification 返回 `ENTRYPOINT_COMPLETENESS_INCOMPLETE`、`CHANGE_SET_SNAPSHOT_STALE`、`CHANGE_SET_SNAPSHOT_IDENTITY_MISMATCH`、semantic evidence invariant failure 或其他错误时，固定停止为 `MANUAL_ACTION_REQUIRED` / `PARTIAL`；不得偷偷删除漏掉的 endpoint、canonical changed file 或 evidence 后继续。Snapshot 后 Git Review Scope bytes/state 变化，旧 snapshot/certificate 均必须重新生成。

## 输出

Semantic Proposal 可包含：

- `changedFileRoles[]`：只引用 Runtime Snapshot canonical paths，并给出 semantic role
- `affectedControllers[]`
- `callChains[]`
- `symbolLocations[]`
- `resourceRelations[]`
- `externalDependencies[]`
- `riskAreas[]`
- `reviewCoverage`

**不得包含：**

- `reviewScope`
- `changedFiles`
- `baseRef/baseCommit/mergeBase/headCommit/currentBranch/includeWorkingTree`
- `changedFiles.path/changedFiles.sources` 的替代结构

只有 `analysis certify` 成功后，同 run `analysis/change-analysis.json` 才能被称为 Certified ChangeAnalysis；`requests/change-analysis-proposal.json` 永远不能直接作为 Review/Chain authority。

TARGETED 额外生成的 ReviewScopeSelection 必须继续通过 `.code-harness/contracts/review-scope.schema.json`，然后由 Controlled Runtime 对照 Certified ChangeAnalysis 验证。

`resourceRelations[]` 是可选证据集合；FULL 可没有 relation 但不能漏读 changed resources。TARGETED 只有 relation 被机器验证后才允许资源进入 Scope。

## Lazy Chain Discovery（1.5 Task 2）

`analyze-change` 仍然负责建立唯一的 semantic evidence proposal；Chain Discovery 不重新解析 Java，也不得覆盖 Certified ChangeAnalysis 事实。

当用户发起：

```text
harness chain discover
harness chain discover <Class>
harness chain discover <Class.method>
```

必须先完成 `analysis snapshot → change-analysis-proposal.json → analysis certify`。之后 `discover-chain` 只消费 Certified：

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

即 `runs/<runId>/analysis/discovered-chains/`；所有结果保持 `status: DISCOVERED`；不得写 `.code-harness/chains/**`，不得提前执行 Task 3 的 validate/accept/refresh。

## Review Chain Context（1.5 Task 4）

Review 使用 Chain 时仍必须**先完成 Canonical Snapshot + ChangeAnalysis certification，获得 Certified ChangeAnalysis**；Chain 解析不建立第二套 Git/Java/resource 事实源。

固定顺序：

1. Runtime `analysis snapshot` 建立完整 Canonical ChangeSet。
2. 本 Skill 消费 Snapshot 建立 semantic proposal、symbolLocations/resourceRelations/callChains。
3. 调用 `analysis certify`，Runtime 重新验证 Snapshot identity 后获得同 run Certified ChangeAnalysis；失败立即停止。
4. FULL 使用 Certified ChangeAnalysis 的 FULL machine coverage；TARGETED 基于 Certified ChangeAnalysis 生成并通过 Runtime verified ReviewScopeSelection + scoped coverage。
5. 将已经验证的 Review Scope 连同同 run `changeAnalysisPath` 写入 controlled request。
6. 由 Orchestrator 调用：

```text
codea-dcep-tools.exe chain review-context --input .code-harness/runs/<runId>/requests/chain-review-context.json
```

7. Runtime 才能决定复用 `ACCEPTED + VALID`、缺失时 lazy discover `DISCOVERED + TEMPORARY`，或返回 STALE/partial 决策状态。

**不得因为存在 Chain 而减少 Canonical Change Set**；FULL 的 required coverage 完全不变。
**不得因为存在 Chain 而减少 changedFiles**；`changedFiles` 只能由 Runtime Canonical Snapshot 组装，Chain 不得删减或重写。
**不得因为存在 Chain 而减少 scopedFiles**；TARGETED 的 scopedFiles 仍只能由 Certified ChangeAnalysis + ReviewScopeSelection 决定。

Chain context 只能补充业务理解；不得反向修改 ChangeAnalysis 的 symbol/path/role/resource relation，不得用 `.code-harness/chains/*.yaml` 替代 Code Navigation evidence。

## 禁止行为

- 不得调用 `git_diff` 或其他 Agent Git 推导建立第二套 Change Set。
- 不得跳过 Runtime `analysis snapshot`。
- 不得自行填写/修改 `requestedBaseRef/resolvedBaseCommit/mergeBase/headCommit/currentBranch/includeWorkingTree/files[].path/status/sources/hunks/gitStateSha256/snapshotSha256`。
- 不得把非 Review Scope 的 properties、pom.xml、Gradle、SQL migration、普通 XML 补回 Canonical Snapshot。
- 不得把 `requests/change-analysis-proposal.json` 当成权威 ChangeAnalysis。
- 不得直接写/改 Runtime-owned `analysis/change-set.json`、`analysis/change-analysis.json`、`analysis/entrypoint-inventory.json`、`analysis/change-analysis.cert.json`。
- 不得跳过 `analysis certify`。
- FULL 不得跳过 canonical changed Java/test/Mapper.xml/YML required files。
- FULL 不得把 `workspace != current` 的 dependency source 写入 `reviewCoverage.reviewedFiles[]`。
- TARGETED 不得把 scope 外 changed files 标为 reviewed。
- 不得通过类名/文件名猜实现路径或 scopedFiles。
- 不得用同 basename 的其他模块文件替代 exact path。
- 不得扫描任意 sibling；workspace source 只能来自显式 `harness.yaml.workspaceDependencies` + Runtime `VERIFIED`。
- 不得直接调用 `ast-grep.exe`。
- 不得跳过 Runtime machine gate 并相信 Agent COMPLETE。
- 不得 sampled review。
- 不得执行任意 Shell、`git fetch` 或 `git pull`。
- 不得修改生产代码或测试代码。
