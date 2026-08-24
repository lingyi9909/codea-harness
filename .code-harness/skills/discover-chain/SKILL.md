---
name: discover-chain
description: 自包含地为当前 Change Set 建立或复用 Runtime 已验证的 ChangeAnalysis，再执行 change/target bounded 的业务 Chain 发现；只产生 Run State DISCOVERED YAML，不修改项目长期 Chain。
version: 3
agent: reviewer
tools:
  - read_code
---

# Lazy Chain Discovery

## 用户意图

只支持以下用户意图：

```text
harness chain discover
harness chain discover OrderController
harness chain discover OrderController.approve
```

空 target 表示只发现当前 Change Set 已影响的生产入口；Class / Class.method target 只缩小当前已验证 evidence 的消费范围，不允许据此扫描整个仓库。

## Chain Discover Bootstrap（1.5.1）

**harness chain discover [target] 是自包含流程**。用户直接执行 discovery 时，不要求存在历史 Chain、不要求存在既有 Review Run，也**不得要求用户先执行 harness review**。

固定流程：

```text
current Change Set
→ analyze-change
→ ChangeAnalysis Schema validate
→ Runtime machine coverage verify
→ chain discover
→ DISCOVERED Chain
```

Bootstrap 规则：

1. 先按既有 `analyze-change` 语义取得当前完整 Change Set；Change Set 继续覆盖 `COMMITTED / STAGED / UNSTAGED / UNTRACKED`，不得因为是 Chain discovery 就减少 working-tree 来源。
2. 当前 run 如果已经存在 verified ChangeAnalysis，只有在能够证明它与当前 **source revision / Change Set** 完全一致时才允许复用。至少必须确认 `reviewScope.headCommit` 与当前 HEAD 一致，并重新取得当前 Change Set 验证 changed file path/role/sources 集合一致。
3. 无法证明完全一致、ChangeAnalysis 不存在、Schema/Coverage 未验证或 source/working tree 已变化，都视为过期；**不存在或已过期时自动重新 analyze-change**，不得把“先跑一次 Review”作为恢复方式。
4. 新建 ChangeAnalysis 必须先通过 `change-analysis.schema.json` 的真实 Schema validate，再由 Controlled Runtime machine coverage verify；Agent 自报 `reviewCoverage.status=COMPLETE` 不能替代机器校验。
5. 只有 verified ChangeAnalysis 才能进入 controlled `chain discover`。历史 `.code-harness/chains/**` 中的 `ACCEPTED` Chain 只是后续可管理/可复用的 Project State，不是 discovery 的前置条件。
6. `harness chain discover` bootstrap 本身不调用 `review-code`、不生成 Finding、不生成 `review.md`，也不把 Chain 接入 Test/Debug/Fix/Verify。

### 新增生产代码必须可直接发现

当前 Change Set 中新增 Controller / Service / ServiceImpl / Mapper / Mapper.xml 与修改既有文件使用完全相同的机器证据规则。

- 新增 production Controller Method 只要由当前 Change Set + Code Navigation 确认 `role=Controller`、生产路径和 exact method symbol，就可以直接成为 Candidate EntryPoint；不得因为它没有历史 Review/Chain 记录而拒绝。
- 新增 Service interface 必须按既有 `find_implementations` 解析实现；新增 ServiceImpl、Mapper 必须用 exact symbol/path/role evidence。
- 新增 Mapper.xml 必须继续通过 `resourceRelations[]` 的 `MapperXml + MAPPER_STATEMENT` verified relation 进入 Chain。
- untracked 新文件与 committed/staged/unstaged 文件一样属于当前 Change Set 事实，不得静默忽略。

## 事实来源

本 Skill 不建立第二套 Java parser，也不根据类名后缀推断 Controller、Service、ServiceImpl、Mapper 或文件路径。

正式事实只来自已经通过 `change-analysis.schema.json` 与 Controlled Runtime machine coverage 校验的：

```text
ChangeAnalysis.affectedControllers[]
ChangeAnalysis.callChains[]
ChangeAnalysis.symbolLocations[]
ChangeAnalysis.resourceRelations[]
ChangeAnalysis.externalDependencies[]
ChangeAnalysis.reviewCoverage.unresolvedSymbols[]
```

其中：

- EntryPoint 必须是 production `Controller` role 的 **生产 Controller Method**，且存在唯一 exact repository path；
- `src/test/**`、demo/sample/mock source 不得成为 EntryPoint；
- role 必须来自 `ChangeAnalysis.symbolLocations[]`；不得根据类名后缀补 role；
- Mapper.xml/YML 只能通过 `ChangeAnalysis.resourceRelations[]` 的 verified relation 进入 Chain；
- 内部 symbol 缺失或 exact path ambiguity 必须 `PARTIAL`，不得包装成完整 Chain。

## Lazy Scope

```text
无 target      -> 只消费当前 Change Set 的 affectedControllers
Controller     -> 只消费该 Controller 的当前 affected endpoints
Controller.method -> 只消费该 method 的 confirmed branches
Service/下游 target -> 只沿当前 confirmed callChains 向上解析 production Controller Method
```

不得因为仓库存在其他 Controller 就顺带发现 User/Payment 等无关业务链。

## Exact Canonicalization

V1/V2 或多个入口只有在 **verified core path 完全一致** 时才允许合并为一个 Chain 的多个 `entryPoints`。

core facts 固定包括：

```text
nodes[] exact symbol + path + role + order
resources[] exact path + symbol + role
boundaries[] exact symbol + path + role
```

任何一个 verified fact 不一致都必须保留为不同 Chain。禁止名称相似度、模糊阈值、类名相同或方法名相同驱动合并。

## Runtime 调用

Agent 只生成 run-scoped controlled request：

```text
.code-harness/runs/<runId>/requests/chain-discover.json
```

request 只携带：

```json
{
  "runId": "<runId>",
  "target": "OrderController.approve",
  "changeAnalysisPath": ".code-harness/runs/<runId>/analysis/change-analysis.json"
}
```

然后调用：

```text
codea-harness-tools chain discover --input .code-harness/runs/<runId>/requests/chain-discover.json
```

不得暴露 raw ast-grep pattern、任意 source scope、任意 output path 或 shell 参数。

## 输出与状态

Runtime 只能把发现结果写到：

```text
.code-harness/runs/<runId>/analysis/discovered-chains/<id>.yaml
```

所有发现产物保持：

```text
status: DISCOVERED
```

**不得写入 `.code-harness/chains/**`**。如果用户明确要求保存/沉淀发现结果，必须交给 `validate-chain` Chain Management 流程：先 Runtime validate，再经过用户确认后的 controlled persist。

结果语义：

```text
COMPLETE -> 当前请求范围内的 verified Chain facts 全部解析完成
PARTIAL  -> 存在内部 unresolved、ambiguous exact path、入口无法确定等限制
```

### PARTIAL 必须输出机器事实

`PARTIAL` 时不得用“可能”“建议再看一下”“需要额外 Code Navigation”等模糊解释代替已经存在的机器事实。

如果 `ChangeAnalysis.reviewCoverage.unresolvedSymbols[]` 有对应项，用户侧必须直接展示其 `symbol` 与 `reason`，`from` 可作为定位上下文。例如：

```text
PARTIAL

未解析：
- ApprovalService.approve

原因：
- IMPLEMENTATION_NOT_FOUND
```

`SYMBOL_NOT_FOUND / IMPLEMENTATION_NOT_FOUND / REFERENCE_NOT_FOUND / AMBIGUOUS_IMPLEMENTATION` 必须保持机器 reason 原值，不得自行改写成猜测性原因。若 limitation 只来自 Controlled Runtime discovery（例如 `AMBIGUOUS_ENTRYPOINT`、`CALL_CHAIN_NOT_FOUND`），同样展示 Runtime 返回的 exact code + symbol/path，不得发明原因。

Schema validate 或 Runtime machine coverage verify 在 discovery 前失败时，也必须列出实际 contract/coverage error；STOP 后不得转而提示用户“先执行 harness review”。

`PARTIAL` 时不得宣称 Chain 已完整发现，也不得把 DISCOVERED 改成 ACCEPTED。

## 禁止行为

- 不得直接调用 `ast-grep.exe`。
- 不得根据 Controller/Service/Impl/Mapper 类名后缀猜 role。
- 不得扫描与 target/current Change Set 无关的所有 Controller。
- 不得把 unresolved candidate 伪装成 confirmed Chain。
- 不得修改生产代码、测试代码或 `.code-harness/chains/**`。
- 不得在 discovery 阶段执行 Project State 覆盖；validate/refresh/persist 必须交给 Chain Management，并遵守用户确认与 expected-hash 门禁。
- 不得把历史 ACCEPTED Chain、历史 Review Run 或先执行 Review 当作 direct discovery 的前置条件。
