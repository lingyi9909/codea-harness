---
name: validate-chain
description: 管理项目 Business Chain：list/show/validate/refresh，并在用户明确确认后把经过 Runtime 验证且由不可变 write plan 绑定的 candidate 安全沉淀到 Project State。
version: 3
agent: orchestrator
tools:
  - read_code
---

# Chain Management

## 用户意图

1.5.3 继续支持：

```text
harness chain list
harness chain show <id|target>
harness chain discover [target]
harness chain refresh <id>
harness chain edit <id|Controller|Controller.method>
harness chain validate [id]
```

Chain 的代码事实不能由 Agent 直接修改 Runtime-owned YAML 获得 authority。`.code-harness/chains/*.yaml` 是 Project State；Runtime candidate 是 Run State。二者都必须通过 Controlled Runtime 验证后才能参与持久化。

## 事实边界

Chain YAML 是 Project State，不是代码事实来源。以下事实必须来自 Certified ChangeAnalysis：

```text
affectedControllers[]
callChains[]
symbolLocations[]
resourceRelations[]
externalDependencies[]
```

禁止通过类名后缀、字符串包含、basename 或相似度推断 Controller/Service/Mapper、调用关系、资源关系或 path。

Agent 只能创建：

```text
.code-harness/runs/<runId>/requests/**
```

以下路径由 Runtime / Framework Managed code 拥有，Agent 直接写入不产生 authority：

```text
.code-harness/runs/<runId>/analysis/**
.code-harness/runs/<runId>/review.md
.code-harness/chains/**
```

仅仅把 YAML 放到正确目录，不代表它是 Runtime candidate。发现、刷新或后续编辑 candidate 必须有同 run provenance certificate，绑定 candidate exact bytes 与 Certified ChangeAnalysis identity。

## list / show

`harness chain list` 只读取 `.code-harness/chains/*.yaml`，按 id 稳定排序。用户侧状态固定中文：

```text
✅ 已确认
⚠️ 已过期
🔎 临时发现
```

`harness chain show <id|target>` 只做 exact id / exact Controller / exact Controller.method 匹配；多条命中必须报告歧义，不得 fuzzy 选一条。

show 必须展示：名称、状态、全部 entryPoints、按 YAML 保存顺序的 nodes、resources、boundaries、notes。节点角色只使用已保存且经过 Runtime 验证的 role；未知/OTHER 显示普通代码节点，不根据类名补角色。

## validate

对每条待验证 Chain，Orchestrator 先建立最新 Certified ChangeAnalysis，再调用 Controlled Runtime：

```text
codea-harness-tools chain validate \
  --id <chainId> \
  --change-analysis .code-harness/runs/<runId>/analysis/change-analysis.json
```

Runtime 必须验证：

- YAML/model contract；
- id / filename 与项目唯一性；
- EntryPoint 是 affected production Controller Method，exact symbol/path/role 唯一；
- node exact symbol/path/role；
- nodes 顺序对应 confirmed callChain；
- Mapper.xml / YML 文件存在且存在 exact resource relation；
- boundary symbol/path/role 有 evidence；
- `notes` 完全不参与代码事实判断。

机器状态：

```text
VALID   -> 当前核心事实仍成立
STALE   -> 已保存 Chain 的代码事实不再成立
INVALID -> Contract / Project State 不安全、矛盾或 candidate 事实无效
```

用户侧不得直接泄漏机器枚举：

```text
VALID   -> Chain 验证通过
STALE   -> Chain 已过期，需要刷新
INVALID -> Chain 无效，需要修正
```

validate 不得静默修改 `.code-harness/chains/**`。

## refresh：diff-first

`harness chain refresh <id>` 固定流程：

```text
现有 ACCEPTED Chain
+
当前 source 的 Certified ChangeAnalysis
↓
Runtime-certified DISCOVERED candidate
↓
Controlled Runtime refresh
↓
analysis/refresh-candidates/<id>.yaml
+ 同目录 candidate provenance certificate
↓
向用户展示 deterministic 差异
↓
等待用户决定是否保存
```

refresh request 只能写到：

```text
.code-harness/runs/<runId>/requests/chain-refresh.json
```

Runtime 只允许读取同 run、带有效 Runtime provenance 的：

```text
.code-harness/runs/<runId>/analysis/discovered-chains/*.yaml
```

并只允许输出 Runtime-owned：

```text
.code-harness/runs/<runId>/analysis/refresh-candidates/<id>.yaml
.code-harness/runs/<runId>/analysis/refresh-candidates/<id>.cert.json
```

**refresh 本身绝不覆盖 Project State。**

## 不可变保存计划

保存不是“用户确认一个可变 candidate path 后直接 persist”。必须先让 Runtime 封存 exact bytes/facts：

```text
Runtime candidate
↓
codea-harness-tools chain seal-persist
↓
analysis/chain-write-plans/<planId>.json
↓
向用户展示该 exact plan 的 preview / planId
↓
用户明确确认该 planId
↓
codea-harness-tools chain persist
```

Orchestrator 创建 sealing request：

```json
{
  "runId": "<runId>",
  "candidatePath": ".code-harness/runs/<runId>/analysis/<discovered-chains|refresh-candidates|edit-candidates>/<id>.yaml",
  "expectedExistingHash": "<可选；刷新/覆盖时可携带当前展示的 hash>"
}
```

然后调用：

```text
codea-harness-tools chain seal-persist --input .code-harness/runs/<runId>/requests/chain-seal-persist.json
```

Runtime write plan 必须绑定：

```text
planId
runId
chainId
candidatePath
candidateHash
analysisHash
expectedExistingHash
previewSha256
```

最终确认后，Agent/Orchestrator 只能提交：

```json
{
  "runId": "<runId>",
  "planId": "chain-write-..."
}
```

并调用：

```text
codea-harness-tools chain persist --input .code-harness/runs/<runId>/requests/chain-persist.json
```

Runtime 在写 Project State 前必须重新读取并验证：

```text
sealed write plan identity
→ Certified ChangeAnalysis 当前仍有效且 analysisHash 相同
→ Runtime candidate provenance + exact candidateHash
→ 当前 source Chain validation
→ deterministic preview hash
→ 当前 existing Project State hash/absence 与 sealed expectedExistingHash 一致
→ 原子写入 .code-harness/chains/<id>.yaml
```

candidate / Certified Analysis / existing Project State 任一 byte/fact 在 seal 后发生变化，旧 plan 必须失效并产生 **0 Project State writes**。需要重新 seal，并重新取得用户对新 planId 的确认。

用户确认边界是“确认 exact planId”，不是“确认某个以后还可以变化的文件路径”。Runtime 不声称能够以密码学方式证明同一 OS 用户下是谁键入确认；实际用户意图仍由 Orchestrator 解释。

## 禁止行为

- 不得把 YAML `status=ACCEPTED` 当作真实性证明。
- 不得把手工创建在 `runs/**/analysis/**` 的 YAML 当作 Runtime-owned candidate。
- 不得 validate/provenance/hash 校验失败后仍保存 candidate。
- 不得 refresh 自动覆盖现有 Chain。
- 不得在用户确认 exact sealed planId 前调用 persist。
- 最终 persist request 不得重新携带 `candidatePath` / `changeAnalysisPath` / `expectedExistingHash` 来改变已确认计划。
- 不得用旧 plan 覆盖用户/其他进程已经修改过的 Chain。
- 不得新增规则引擎、chain-rules.yaml、全仓 Call Graph。
- 不得把 Chain 接入 Task 4–6 的新功能；本 Task 只强化 Chain Artifact 与 Write Authority。


## Semantic edit（1.5.3 Task 5）

`harness chain edit <id|Controller|Controller.method>` 路由到 `edit-chain`。Runtime 对六类 operation 做最终完整 Chain 事实验证，成功后只输出：

```text
.code-harness/runs/<runId>/analysis/edit-candidates/<id>.yaml
.code-harness/runs/<runId>/analysis/edit-candidates/<id>.cert.json  # kind=EDIT
```

EDIT candidate 不是 Project State，不得直接写 `.code-harness/chains/**`。保存继续固定复用 `chain seal-persist → exact planId → 用户确认当前 planId → chain persist`；候选、分析或 existing Project State 变化后旧 plan 必须失效。
