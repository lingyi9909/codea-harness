---
name: validate-chain
description: 管理项目 Business Chain：list/show/validate/refresh，并在用户明确确认后把经过 Runtime 验证的 candidate 安全沉淀到 Project State。
version: 1
agent: orchestrator
tools:
  - read_code
---

# Chain Management

## 用户意图

第一版只支持：

```text
harness chain list
harness chain show <id|target>
harness chain discover [target]
harness chain refresh <id>
harness chain validate [id]
```

不增加 `chain accept/merge/split/edit/ignore` 用户命令。开发人员直接编辑 `.code-harness/chains/*.yaml`；编辑后必须 validate。

## 事实边界

Chain YAML 是 Project State，不是代码事实来源。以下事实必须来自已经通过 `change-analysis.schema.json` 与 Runtime machine coverage 校验的 ChangeAnalysis：

```text
affectedControllers[]
callChains[]
symbolLocations[]
resourceRelations[]
externalDependencies[]
```

禁止通过类名后缀、字符串包含、basename 或相似度推断 Controller/Service/Mapper、调用关系、资源关系或 path。

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

对每条待验证 Chain，Orchestrator 先建立与该 Chain/target 对应的最新 ChangeAnalysis，并完成 schema + machine coverage 验证，再调用 Controlled Runtime：

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

用户侧不得直接泄漏机器枚举，固定翻译为：

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
当前 source 的 verified ChangeAnalysis
↓
Lazy Discover 当前 candidate
↓
Controlled Runtime refresh
↓
生成 Run State refresh candidate + deterministic added/removed facts
↓
向用户展示差异
↓
等待用户明确确认
```

refresh request 只能写到：

```text
.code-harness/runs/<runId>/requests/chain-refresh.json
```

Runtime 只允许读取同 run：

```text
.code-harness/runs/<runId>/analysis/discovered-chains/*.yaml
```

并只允许输出：

```text
.code-harness/runs/<runId>/analysis/refresh-candidates/<id>.yaml
```

**refresh 本身绝不覆盖 Project State。**

## 用户确认后的 Project State 写入

用户明确表达“保存/沉淀/确认更新这条 Chain”后，Orchestrator 才允许生成：

```text
.code-harness/runs/<runId>/requests/chain-persist.json
```

内部 persist request 包含：

```json
{
  "runId": "<runId>",
  "candidatePath": ".code-harness/runs/<runId>/analysis/<discovered-chains|refresh-candidates>/<file>.yaml",
  "changeAnalysisPath": ".code-harness/runs/<runId>/analysis/change-analysis.json",
  "expectedExistingHash": "<refresh 时展示给用户的 existing hash；首次保存为空>"
}
```

然后调用：

```text
codea-harness-tools chain persist --input .code-harness/runs/<runId>/requests/chain-persist.json
```

Runtime 必须按顺序执行：

```text
candidate -> status=ACCEPTED
-> Validate == VALID
-> 检查 existing expected hash
-> 原子写入 .code-harness/chains/<id>.yaml
```

任何 validation failure / hash mismatch / duplicate id：

```text
0 Project State writes
```

已有同 id Chain 且没有用户确认流程提供的 expected hash，必须拒绝静默覆盖。

## 禁止行为

- 不得把 YAML status=ACCEPTED 当作真实性证明。
- 不得 validate 失败后仍保存 candidate。
- 不得 refresh 自动覆盖现有 Chain。
- 不得在用户确认前调用 persist。
- 不得用旧 expected hash 覆盖用户/其他进程已经修改过的 Chain。
- 不得新增规则引擎、chain-rules.yaml、全仓 Call Graph。
- 不得把 Chain 接入 Test/Debug/Fix/Verify；1.5 Task 3 只做 Chain Management。
