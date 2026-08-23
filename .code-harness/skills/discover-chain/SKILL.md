---
name: discover-chain
description: 基于 Runtime 已验证的 ChangeAnalysis 进行 change/target bounded 的业务 Chain 发现；只产生 Run State DISCOVERED YAML，不修改项目长期 Chain。
version: 1
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

所有 Task 2 产物保持：

```text
status: DISCOVERED
```

**不得写入 `.code-harness/chains/**`**。从 `runs/**` 接受为 Project State 属于后续 Chain Management 流程，不在本 Skill 内完成。

结果语义：

```text
COMPLETE -> 当前请求范围内的 verified Chain facts 全部解析完成
PARTIAL  -> 存在内部 unresolved、ambiguous exact path、入口无法确定等限制
```

`PARTIAL` 时必须明确展示 unresolved/limitation，不得宣称 Chain 已完整发现，也不得把 DISCOVERED 改成 ACCEPTED。

## 禁止行为

- 不得直接调用 `ast-grep.exe`。
- 不得根据 Controller/Service/Impl/Mapper 类名后缀猜 role。
- 不得扫描与 target/current Change Set 无关的所有 Controller。
- 不得把 unresolved candidate 伪装成 confirmed Chain。
- 不得修改生产代码、测试代码或 `.code-harness/chains/**`。
- 不得执行 Chain validate/accept/refresh；这些属于后续 Task。
