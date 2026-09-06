---
name: discover-chain
description: 从当前项目源码发现业务调用链；公开 chain discover 使用 PROJECT SOURCE，Review 内部 affected discovery 继续消费 Certified ChangeAnalysis。
version: 10
---

# Discover Chain

## 1.6.3 Authority Split

`harness chain discover [target]` 的公开语义固定为 **PROJECT DISCOVERY**：

```text
harness chain discover
harness chain discover OrderController
harness chain discover OrderController.approve
```

执行语义：

```text
Current Source
→ Runtime Controller / EntryPoint inventory
→ AST Code Navigation
→ interface → implementation verification
→ Mapper method → Mapper.xml namespace + statement id verification
→ Runtime project-source.json
→ Runtime-owned DISCOVERED Chain candidate + provenance certificate
```

PROJECT DISCOVERY 不依赖非空 Git Change Set。即使 `HEAD == main` 且 working tree clean，也必须可以从现有源码发现真实 Chain。

PROJECT DISCOVERY 不得：

- 创建或伪造 `analysis/change-set.json`；
- 把整个项目伪装成 `changedFiles[]`；
- 创建 `analysis/change-analysis.json` 或 ChangeAnalysis certification；
- 修改 Review Snapshot / Canonical ChangeSet 算法；
- 创建 `review.md`；
- 不得写入 `.code-harness/chains/**`；
- 根据 `ServiceImpl`、`Mapper` 等类名后缀猜测语义关系。

公开 discover 的 Controlled Runtime request 固定为 run-scoped request：

```json
{
  "runId": "<runId>",
  "mode": "PROJECT",
  "target": "<optional Controller or Controller.method>"
}
```

然后调用：

```text
codea-dcep-tools.exe chain discover --input .code-harness/runs/<runId>/requests/chain-discover.json
```

Runtime 只允许把结果写入：

```text
.code-harness/runs/<runId>/analysis/project-source.json
.code-harness/runs/<runId>/analysis/discovered-chains/<id>.yaml
.code-harness/runs/<runId>/analysis/discovered-chains/<id>.cert.json
```

candidate 必须保持 `status: DISCOVERED`。certificate 必须绑定 exact candidate bytes，并以 `authorityKind=PROJECT_SOURCE` + `sourceHash` 绑定 Runtime-owned current-source inventory。source bytes 或 candidate bytes 变化后必须 fail closed。

## AFFECTED DISCOVERY — Review Internal Only

Review 内部的 affected discovery 继续保持既有 authority：

```text
Canonical ChangeSet
→ Certified ChangeAnalysis
→ affectedControllers / callChains
→ chain discover mode=AFFECTED
→ Runtime-owned DISCOVERED candidate
```

兼容 request：

```json
{
  "runId": "<runId>",
  "mode": "AFFECTED",
  "target": "<optional target>",
  "changeAnalysisPath": ".code-harness/runs/<runId>/analysis/change-analysis.json"
}
```

历史调用方未显式提供 `mode`、但提供 same-run `changeAnalysisPath` 时，Runtime 继续按 AFFECTED 处理；这只用于 Review/旧调用兼容，不改变公开 `harness chain discover` 的 PROJECT 语义。

AFFECTED DISCOVERY 的正式事实仍只来自 Certified ChangeAnalysis。`reviewCoverage.unresolvedSymbols`、`IMPLEMENTATION_NOT_FOUND`、ambiguity 或其他未解析事实必须保持 `PARTIAL`，并向用户展示“未解析”和“原因”，不得猜测补链。

## Target

公开 PROJECT discovery：

- 无 target：从 Runtime 验证出的 production Controller Method inventory 开始发现；
- `Controller`：只保留该 Controller 的已验证 endpoints；
- `Controller.method`：只保留该 exact endpoint；
- 无法唯一验证 target：返回 `PARTIAL`，不得 fuzzy 选择。

Role 只允许来自机器证据：Controller annotation / endpoint inventory、AST interface/implementation relation、`@Service` / `@Repository` / `@Mapper`、以及 Mapper.xml exact namespace + statement id。文本预筛选可以帮助缩小候选，但不能升级成 semantic authority。

## Output

成功时只说明发现了多少条 `DISCOVERED Chain` 以及 Runtime candidate path。不要把 candidate 表述成已保存 Project State Chain；保存/更新仍属于后续明确确认和 write-plan authority。

部分失败时输出：

```text
PARTIAL
未解析: <symbol/relation>
原因: <machine reason>
```

不得因为 partial discovery 生成 Finding，也不得输出 Review PASSED/FAILED。

## 1.5.1 Historical Compatibility Vocabulary

以下术语保留用于旧回归和 AFFECTED 路径说明，但 **1.6.3 已覆盖其公开 discover 语义**：

- `Chain Discover Bootstrap（1.5.1）`
- `harness chain discover [target] 是自包含流程`
- `current Change Set`
- `analyze-change`
- `ChangeAnalysis Schema validate`
- `Runtime machine coverage verify`
- `chain discover`
- `DISCOVERED Chain`
- `不得要求用户先执行 harness review`
- `source revision / Change Set`
- `不存在或已过期时自动重新 analyze-change`
- `COMMITTED / STAGED / UNSTAGED / UNTRACKED`
- `新增 production Controller Method`
- `PARTIAL`
- `未解析`
- `原因`
- `IMPLEMENTATION_NOT_FOUND`
- `reviewCoverage.unresolvedSymbols`

这些旧术语现在只描述 Review/AFFECTED compatibility；公开 PROJECT discovery 不再执行 `current Change Set → analyze-change` bootstrap。
