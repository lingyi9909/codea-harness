---
name: discover-chain
description: 自包含地为当前 Change Set 建立或复用 Runtime 已认证的 ChangeAnalysis，再执行 change/target bounded 的业务 Chain 发现；只产生带 Runtime provenance 的 Run State DISCOVERED candidate，不修改项目长期 Chain。
version: 4
agent: reviewer
tools:
  - read_code
---

# Lazy Chain Discovery

## 用户意图

只支持：

```text
harness chain discover
harness chain discover OrderController
harness chain discover OrderController.approve
```

空 target 表示只发现当前 Change Set 已影响的生产入口；Class / Class.method target 只缩小当前已验证 evidence 的消费范围，不允许据此扫描整个仓库。

## Chain Discover Bootstrap（1.5.1）

`harness chain discover [target] 是自包含流程`。用户直接执行 discovery 时，不要求历史 Chain、既有 Review Run，**不得要求用户先执行 harness review**。

1.5.3 在保留 1.5.1 bootstrap 语义的基础上，把原来的 Schema/coverage 前置校验收敛进 Runtime certification：`analyze-change` 提案完成后，由 Runtime 执行 `ChangeAnalysis Schema validate` 与 `Runtime machine coverage verify`，再产出 Certified ChangeAnalysis。source revision / Change Set 不存在或已过期时自动重新 analyze-change，不复用 stale authority。

固定流程：

```text
current Change Set
→ analyze-change draft
→ ChangeAnalysis Schema validate
→ Runtime machine coverage verify
→ Runtime analysis certify
→ Certified ChangeAnalysis
→ chain discover
→ Runtime-owned DISCOVERED Chain candidate + provenance certificate
```

规则：

1. 当前完整 Change Set 覆盖 `COMMITTED / STAGED / UNSTAGED / UNTRACKED`。
2. 既有 ChangeAnalysis 只有在 `analysis.LoadCertified` 能证明与当前 HEAD / Change Set / inventory / Runtime version 一致时才可复用；否则重新认证。
3. Agent 只能创建 `requests/change-analysis-draft.json` 与 run-scoped request，不能创建/覆盖 authoritative `analysis/change-analysis.json`、inventory 或 certificate。
4. 只有 Certified ChangeAnalysis 才能进入 controlled `chain discover`。
5. 历史 `.code-harness/chains/**` 的 ACCEPTED Chain 是 Project State，不是 discovery 前置条件。
6. discovery 不调用 `review-code`、不生成 Finding、不生成 `review.md`，也不把 Chain 接入 Test/Debug/Fix/Verify。
7. 任何 `PARTIAL` / 未解析结果都必须保留明确原因（例如 `IMPLEMENTATION_NOT_FOUND`）并落在 `reviewCoverage.unresolvedSymbols` 或对应 Runtime limitation 中，禁止猜测补齐。

### 新增生产代码必须可直接发现

- 新增 production Controller Method 必须由当前 Change Set + Code Navigation 确认 `role=Controller`、生产路径和 exact method symbol。
- 新增 Service interface 必须按 verified implementation evidence 解析实现；ServiceImpl、Mapper 使用 exact symbol/path/role evidence。
- Mapper.xml 必须通过 `resourceRelations[]` 的 `MapperXml + MAPPER_STATEMENT` verified relation 进入 Chain。
- untracked 新文件与 committed/staged/unstaged 文件一样属于当前 Change Set，不得静默忽略。

## 事实来源

本 Skill 不建立第二套 Java parser，也不根据类名后缀推断角色。

正式事实只来自 Certified ChangeAnalysis：

```text
affectedControllers[]
callChains[]
symbolLocations[]
resourceRelations[]
externalDependencies[]
reviewCoverage.unresolvedSymbols[]
```

其中：

- EntryPoint 必须是 production `Controller` role 的生产 Controller Method，且存在唯一 exact repository path；
- `src/test/**`、demo/sample/mock source 不得成为 EntryPoint；
- role 必须来自 `symbolLocations[]`；不得根据类名后缀补 role；
- Mapper.xml/YML 只能通过 verified resource relation 进入 Chain；
- 内部 symbol 缺失或 exact path ambiguity 必须 `PARTIAL`。

## Lazy Scope

```text
无 target          -> 当前 Change Set affectedControllers
Controller         -> 该 Controller 当前 affected endpoints
Controller.method  -> 该 method confirmed branches
Service/下游 target -> 当前 confirmed callChains 向上解析 production Controller Method
```

不得因为仓库存在其他 Controller 就顺带发现无关业务链。

## Exact Canonicalization

多个入口只有在 verified core path 完全一致时才允许 canonicalize。core facts 包括：

```text
nodes[] exact workspace + symbol + path + role + order
resources[] exact path + symbol + role
boundaries[] exact symbol + path + role
```

任一 verified fact 不一致都必须保留为不同 Chain。禁止名称相似度、模糊阈值或类/方法名相似驱动合并。

## Runtime 调用

Agent 只生成：

```text
.code-harness/runs/<runId>/requests/chain-discover.json
```

request：

```json
{
  "runId": "<runId>",
  "target": "OrderController.approve",
  "changeAnalysisPath": ".code-harness/runs/<runId>/analysis/change-analysis.json"
}
```

然后调用：

```text
codea-dcep-tools.exe chain discover --input .code-harness/runs/<runId>/requests/chain-discover.json
```

不得暴露 raw ast-grep pattern、任意 source scope、任意 output path 或 shell 参数。

## Runtime-owned candidate 与 provenance

Runtime discovery 的 candidate 固定写到：

```text
.code-harness/runs/<runId>/analysis/discovered-chains/<id>.yaml
.code-harness/runs/<runId>/analysis/discovered-chains/<id>.cert.json
```

certificate 至少绑定：

```text
runId
kind=DISCOVERED
chainId
candidatePath
candidateHash
analysisHash
```

YAML 只存在于正确目录并不构成 authority。后续 refresh / seal / persist 消费 candidate 时必须经过共享 Runtime provenance loader，验证：

```text
same run
expected artifact directory
model/id identity
exact candidate hash
Certified ChangeAnalysis identity
```

Agent 对 YAML 或 certificate 的手工修改都不能成为可信事实；缺 provenance 返回 `CHAIN_ARTIFACT_NOT_RUNTIME_OWNED`，candidate byte 变化返回 `CHAIN_CANDIDATE_HASH_MISMATCH` 或等价 fail-closed error。

所有 discovery candidate 保持：

```text
status: DISCOVERED
```

**不得写入 `.code-harness/chains/**`。** 用户明确要求保存时，必须进入 `validate-chain` 的 Runtime `seal-persist → exact planId confirmation → persist` 流程。

## 结果语义

```text
COMPLETE -> 当前请求范围内 verified Chain facts 全部解析完成
PARTIAL  -> 存在内部 unresolved、ambiguous exact path、入口无法确定等限制
```

`PARTIAL` 时必须展示 Runtime 的 exact limitation code / symbol / path，不得改写成猜测。Schema / certification / coverage 失败后 STOP，不得提示用户先跑 Review。

## 禁止行为

- 不得直接调用 `ast-grep.exe`。
- 不得根据 Controller/Service/Impl/Mapper 类名后缀猜 role。
- 不得扫描与 target/current Change Set 无关的所有 Controller。
- 不得把 unresolved candidate 伪装成 confirmed Chain。
- 不得修改生产代码、测试代码或 `.code-harness/chains/**`。
- Agent 不得创建/覆盖 `.code-harness/runs/**/analysis/**` 来伪造 Runtime authority。
- 不得把手工放进 discovered-chains 的 YAML 当成 Runtime-owned candidate。
- discovery 阶段不得执行 Project State 覆盖；保存必须使用 immutable write plan。
- 不得把历史 ACCEPTED Chain、历史 Review Run 或先执行 Review 当作 direct discovery 前置条件。
