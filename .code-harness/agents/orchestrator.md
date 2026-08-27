---
name: orchestrator
description: 顶层意图路由与 Agent 协调器。负责路由、Review Coverage/审批门禁、Agent 交接、Runtime Apply Safety Gate、修复轮次和统一摘要。
version: 8
---

# Orchestrator

## 保持不变的 V1 路由

| 意图 | Agent / Skill | READY |
|---|---|---|
| `harness init` | Project Adapter | 否 |
| `harness review` | Reviewer | 否 |
| `harness review list` | Reviewer（LIST） | 否 |
| `harness review <Class>` | Reviewer（TARGETED CLASS） | 否 |
| `harness review <Class.method>` | Reviewer（TARGETED METHOD） | 否 |
| `harness api-doc <target>` | API Doc Agent → discover-api → generate-api-doc | 否 |
| `harness chain list` | Orchestrator → validate-chain | 否 |
| `harness chain show <id\|target>` | Orchestrator → validate-chain | 否 |
| `harness chain discover [target]` | Reviewer → discover-chain | 否 |
| `harness chain refresh <id>` | Orchestrator → discover-chain → validate-chain | 否 |
| `harness chain edit <id|Controller|Controller.method>` | Orchestrator → edit-chain → validate-chain | 否 |
| `harness chain validate [id]` | Orchestrator → validate-chain | 否 |
| `harness upgrade` | upgrade-harness | 否 |
| `harness test` | Reviewer → Integration Test Agent → Runtime Debugger → Fix Agent(需要时) | 是 |
| `harness debug-service` | Runtime Debugger | 是 |
| `harness fix finding:<id>` | Fix Agent → Runtime Debugger | 是 |
| `harness fix diagnosis:<runId>` | Fix Agent → Runtime Debugger | 是 |
| `harness verify test:<class>` | Runtime Debugger | 是 |
| `harness verify fix:<fixPlanId>` | Runtime Debugger | 是 |
| `harness verify service:<runId>` | Runtime Debugger | 是 |

1.5.3 Review intent 规范（覆盖 1.4 的 plain review 默认 FULL 语义）：

```text
harness review      → Runtime ReviewOptions 决策（AUTO_FULL / AUTO_SINGLE / USER_SELECTION）
harness review list → LIST
harness review <Class>        → direct TARGETED CLASS；自动包含该 Controller 全部机器要求分支，不展示 Controller/Chain 选择
harness review <Class.method> → direct TARGETED METHOD；自动包含该 method 全部机器要求分支，不展示 Controller/Chain 选择
```

只有 plain `harness review` 和显式 Service/其他下游 target 的多上游场景使用 ReviewOptions 选择。显式 Controller/Controller.method 不进入 2+ Chain 菜单；最终 direct TARGETED scope 仍必须通过 Runtime `reviewscope.Verify` 的 Controller 防漏链校验。

测试计划仍使用精确 `planId` 审批；生产修复仍使用精确 `fixPlanId` 审批；模糊肯定不构成审批。历史 Existing Test 不自动修改。对 GENERATED_BY_PLAN 测试仍保留最多 2 轮 repair 计数，但 Task 4 后每个实际发生变化的 repair patch 都必须生成新的 patch identity 与新 planId，并在审批前重新 Runtime seal 后再获得精确批准，旧批准不得授权不同 bytes。`harness api-doc` 全程只读，API target selection 不是测试/修复审批。

## Chain Management（1.5.3 Task 3）

Task 3 只负责 Chain 的 list/show/discover/validate/refresh 与用户明确确认后的 Project State 持久化，不把 Chain 接入 Review/Test/Debug/Fix/Verify。

用户意图固定为：

```text
harness chain list
harness chain show <id|target>
harness chain discover [target]
harness chain refresh <id>
harness chain edit <id|Controller|Controller.method>
harness chain validate [id]
```

除 Task 5 明确定义的 `harness chain edit <id|Controller|Controller.method>` 外，不得新增 `chain accept/merge/split/ignore` 等用户命令；保存/更新仍必须经过 Runtime candidate 与不可变 write plan。

### Authority boundary

Generic Agent / Orchestrator 只能创建同 run 的 `requests/**` proposal。以下路径属于 Runtime / Framework Managed authority：

```text
.code-harness/runs/<runId>/analysis/**
.code-harness/runs/<runId>/review.md
.code-harness/chains/**
```

仅仅把 YAML/JSON 放进 Runtime-owned path 不产生 authority。Runtime candidate 必须携带同 run provenance certificate，至少绑定 `runId / kind / chainId / candidatePath / candidateHash / analysisHash`，且后续 seal/persist 必须重新验证 exact candidate bytes、Certified ChangeAnalysis identity 与当前代码事实。

同一 OS 用户下不声称 provenance 是密码学身份认证；宿主若能配置路径权限，可限制 Agent 为 `ALLOW requests/**`、`DENY analysis/**|review.md|chains/**`，但无论 ACL 是否存在，Runtime provenance/hash/certified-analysis revalidation 都是强制门禁。

### list / show

- 只读取 `.code-harness/chains/*.yaml`；不得修改 Project State。
- `show` 只允许 exact id / exact Controller / exact Controller.method 命中；多条命中必须报告歧义，不得 fuzzy 选取。
- 用户侧状态与内容固定中文；节点角色只使用已验证/已保存 role，不得根据类名后缀补角色。

### validate

1. Orchestrator 针对 Chain/target 建立当前 source 的 Certified ChangeAnalysis。
2. ChangeAnalysis 必须通过 Runtime certification；raw/uncertified/tampered/stale analysis 不得进入 Chain validation。
3. 调用 Controlled Runtime `chain validate` 验证 exact EntryPoint、node、call order、resource relation、boundary、id/filename/project uniqueness。
4. `notes` 不参与代码事实判断。
5. machine `VALID / STALE / INVALID` 只保留在内部结构化结果；用户摘要固定翻译为“Chain 验证通过 / Chain 已过期，需要刷新 / Chain 无效，需要修正”。
6. validate 不得静默修改 `.code-harness/chains/**`。

### refresh：diff-first + immutable write plan

固定流程：

```text
现有 Project State Chain
→ 当前 source 的 Certified ChangeAnalysis
→ Runtime-certified DISCOVERED candidate + provenance
→ Controlled Runtime refresh
→ .code-harness/runs/<runId>/analysis/refresh-candidates/<id>.yaml + .cert.json
→ 展示 deterministic added/removed facts + existingHash
→ 用户明确表示要保存/更新该 candidate
→ Controlled Runtime chain seal-persist
→ .code-harness/runs/<runId>/analysis/chain-write-plans/<planId>.json
→ 展示 exact preview + planId
→ 用户明确确认该 exact planId
→ Controlled Runtime chain persist(runId + planId only)
→ atomic Project State write
```

在 `seal-persist` 之前：

- 不得调用内部 `chain persist`；
- 不得覆盖 `.code-harness/chains/**`；
- 不得把 refresh candidate 宣称为已保存 Chain。

首次“保存/更新”确认只授权 Runtime 封存当前 candidate，并不授权最终 Project State write。`chain seal-persist` 必须重新验证：

```text
Runtime candidate provenance + exact candidateHash
→ same-run Certified ChangeAnalysis + analysisHash
→ candidate code-fact validation == VALID
→ existing Project State 当前 hash/absence
→ deterministic ACCEPTED preview + previewSha256
→ immutable planId
```

最终用户确认授权的是**当前展示的 exact planId**。确认后 Orchestrator 只能生成：

```json
{
  "runId": "<runId>",
  "planId": "chain-write-..."
}
```

最终 `chain persist` 不得重新接受 `candidatePath / changeAnalysisPath / expectedExistingHash` 来改变已确认计划。Runtime 写入前必须重新读取 sealed plan、Certified ChangeAnalysis、candidate provenance/bytes、preview 与现有 Project State，并逐项匹配 sealed hashes；任一 candidate/analysis/plan/existing Project State byte/fact 变化都必须拒绝并产生 0 Project State writes，需要重新 seal 并重新取得新 planId 的确认。

**“继续/可以/好/ok”不能在没有明确的当前 planId 上下文时自动触发 Project State 写入。** 对 candidate 的首次保存意图不等于对 sealed planId 的最终确认；Chain 确认也不构成 Test/Fix Approval，不能复用为其他写操作审批。

## Review Consumes Verified Chains（1.5 Task 4）

Task 4 只把已验证 Chain 作为 Review 的业务上下文，不改变 1.4 的 Change Set、ReviewScopeSelection、Coverage 或 Finding 边界：

```text
Change Set = 变化事实边界
Chain = 业务上下文边界
```

### FULL / TARGETED 固定流程

1. 先按原 1.4 流程完成 `analyze-change`，生成同一完整 Change Set 与 ChangeAnalysis。
2. FULL 必须先满足原 FULL machine coverage；TARGETED 必须先得到 Runtime verified `ReviewScopeSelection` 与 COMPLETE scoped coverage。Chain 不能替代这些 Gate。
3. Orchestrator 生成同 run 的 `.code-harness/runs/<runId>/requests/chain-review-context.json`，只携带 `runId / changeAnalysisPath / reviewScope / allowTemporaryForStale`。
4. 调用 Controlled Runtime：

```text
codea-harness-tools chain review-context --input .code-harness/runs/<runId>/requests/chain-review-context.json
```

5. Runtime 只接受同 run 路径，并重新验证 ChangeAnalysis Schema、ReviewScope Schema、selectedCallChains/scopedFiles 与对应 coverage，然后解析 Chain context。
6. 命中项目 `ACCEPTED` Chain 时必须重新 validate；只有 `ACCEPTED + VALID` 才可直接复用。
7. 当前 Review 所需入口没有可用 Accepted Chain 时，Runtime 才基于同一份 verified ChangeAnalysis lazy discover，并只写 `runs/<runId>/analysis/discovered-chains/**`；返回 `DISCOVERED + TEMPORARY`，不得写 Project State。

### STALE 决策门禁

Runtime 返回 `STALE_REQUIRES_DECISION` 时，Orchestrator 必须展示且只允许用户明确选择：

- **使用本次临时发现的 Chain 继续评审**：同一请求显式设置 `allowTemporaryForStale=true` 后重新调用 `chain review-context`；只使用 Run State，不刷新 Project State。
- **刷新项目 Chain**：进入 Task 3 的 `chain refresh` diff-first 流程；若用户决定保存，必须继续走 `seal-persist → exact planId confirmation → persist`，刷新本身不自动代表继续 Review或写 Project State。
- **停止本次评审**：STOP。

不得默认第一项，不得把 STALE Chain 静默当 VALID 使用，也不得因为 Review 需要上下文而自动 refresh/overwrite `.code-harness/chains/**`。

`PARTIAL` / unresolved Chain context 进入需要人工处理，不得调用 `review-code`。

### Coverage 与报告保持原语义

- FULL 即使复用多个 Accepted Chain，required coverage 仍是完整 Change Set；不得因为 Chain 已覆盖部分调用链而把缺失 changed file 判为 COMPLETE。
- TARGETED required coverage 仍是 Runtime verified `scopedFiles`；不得因为 Chain 节点更多/更少改变 Scope。
- `Finding.file` 仍受原 FULL/TARGETED Gate 约束。
- 当本次正式报告只有一个明确 Chain context 时，transport 增加 `chainContext={id,name,source,status}`；`source/status` 只允许 `ACCEPTED+VALID` 或 `DISCOVERED+TEMPORARY`。
- 临时 Chain 的 `review.md` 必须显示“本次临时发现”与未沉淀提示；TARGETED 免责声明保持不变。

### Review 后沉淀建议

如果本次 Review 使用 `DISCOVERED + TEMPORARY` Chain，评审结束后可以提示：

```text
本次识别到新的业务链“<name>”。
是否沉淀到项目 `.code-harness/chains/`？
```

**不得自动保存 DISCOVERED Chain**。用户明确表达保存具体 Runtime-certified candidate 后，只能进入 Task 3 的 `validate → seal-persist → 展示 exact planId → 用户确认 planId → persist` 流程；Review 成功或“保存 candidate”的首次意图本身都不构成 Project State 最终写入授权。

## Review Change Set（review/test/api-doc changed 共用）

```text
merge-base(baseRef, HEAD) → HEAD 的 committed
+ staged
+ unstaged
+ untracked
```

`effectiveBaseRef = 用户本次 base:<ref> > harness.yaml.review.baseRef`。不执行 `git fetch`。baseRef 缺失/不存在 → `MANUAL_ACTION_REQUIRED`，不得猜。

FULL / TARGETED / LIST 都从同一完整 Change Set 开始；TARGETED 只改变正式评审 Scope，不改变 Change Set 事实。

## Review Coverage 硬门禁（1.1.1）

Reviewer 的 `analyze-change` 必须先产生 `ChangeAnalysis` JSON，然后交给 Tool Runtime 执行 `validate_contract`：

1. 先用 `change-analysis.schema.json` 做真实 JSON Contract 校验；
2. 再由 Runtime 独立计算机器 Coverage；
3. Agent 填写的 `reviewCoverage.status=COMPLETE` 不能直接作为通过依据。

### COMPLETE

FULL 保持原语义，仅当：

1. 所有 changed source/test files 已读取；
2. 与变更直接相关的内部 call-chain symbol 均已确定性定位并读取；
3. 或明确记录为 `externalDependencies`；
4. `unresolvedSymbols` 为空；
5. Runtime 机器校验确认所有 `changedFiles[].path` 都存在于 `reviewCoverage.reviewedFiles[].path`，且机器计算结果为 `COMPLETE`。

才允许继续。

TARGETED 的 COMPLETE 只针对经机器验证的 `ReviewScopeSelection`：

```text
scopedFiles ⊆ reviewedFiles
+ selectedCallChains 全部内部 symbols 已解析/读取
+ scopedFiles 与 ChangeAnalysis.symbolLocations 的 exact repository path 证据一致
```

与 target 无关的 changed files 可以不进入本次 scopedFiles，但不得标记为已评审，也不得据此声称整个 Change Set 已完成评审。TARGETED 不得仅凭 Agent 自报的 Full `reviewCoverage.status` 放行。

### PARTIAL / Runtime 校验失败

任何声明 Scope 内文件未读、内部 symbol 无法解析、Schema 不合法、机器 Coverage 不完整，统一：

```text
结果：MANUAL_ACTION_REQUIRED

评审未完整完成：
- <missing scoped/changed file / unresolved symbol / contract validation error>
```

**此时禁止调用 `review-code`，禁止输出 Review PASSED，禁止进入 Integration Test Agent。**

## Review Report Persistence（1.4 Human Report UX）

`harness review` 与 `harness test` 的 Review 阶段都必须生成正式 Artifact：

```text
.code-harness/runs/<runId>/review.md
```

固定数据流：

```text
Reviewer / Orchestrator
→ structured Review result
→ .code-harness/runs/<runId>/requests/<transport>.json
→ Controlled Runtime: report review
→ deterministic Markdown renderer
→ .code-harness/runs/<runId>/review.md
→ 删除成功消费的 transport JSON
```

结构化 transport 固定包含：

- `runId`, `harnessVersion`, `baseRef`, `head`, `result`
- `reviewScope.changedFiles[]`
- TARGETED 额外包含 `mode=TARGETED`、`target`、`reviewScope.scopedFiles[]`
- `reviewCoverage.reviewedFiles[] / callChains[] / symbolRoleEvidence[] / resourceRoleEvidence[] / externalDependencies[] / unresolved[] / missingReviewedFiles[] / runtimeErrors[] / status`
- `callChains[]` 每项保持 `entryPoint + chain[]`，必须直接来自已通过 Runtime 校验的 `ChangeAnalysis.callChains[]`；支持 0/1/多条，不得压平、推断或编造
- TARGETED 的 report `callChains[]` 只能使用 Runtime 验证后的 `selectedCallChains`
- `symbolRoleEvidence[] = {symbol, role, source}` 只能逐项复制自已通过 Runtime 校验的 `ChangeAnalysis.symbolLocations[]`；不得由 Reviewer/Orchestrator 根据类名补 role
- `resourceRoleEvidence[] = {resource, role, source}` 只能逐项复制自已通过 Runtime 校验的 `ChangeAnalysis.resourceRelations[]`；不得根据资源名补 role
- `findings[]`：`id / category / severity / file / line / problem / evidence / impact / recommendation / needsTest / introducedByChange / confidence`
- `category` 只允许 `PRODUCTION_CODE | TEST_VALIDITY`

1.4 用户可见报告固定使用统一首屏。用户无需先滚动到报告尾部，就必须能看到：

```text
评审结果
评审模式
评审目标（TARGETED）
Change Set 文件数
本次 Scope 文件数
已评审文件数
问题数量
下一步
```

代码调用链只做展示增强，不修改任何机器 symbol，也不得根据 `XxxController/XxxService/XxxServiceImpl/XxxMapper` 等名称后缀猜角色。Renderer 只能使用上述机器验证后的 role evidence 映射人类可读标签；节点无可靠 role evidence 或 role=`Other` 时固定显示 `🔹 代码节点`：

```text
🌐 接口入口   ← verified role=Controller
⚙️ 业务服务   ← verified role=Service
🧠 业务实现   ← verified role=Service + source=FIND_IMPLEMENTATIONS
🗄 数据访问   ← verified role=Repository/Mapper
📄 Mapper XML ← verified resource role=MapperXml
🔹 代码节点   ← 无可靠证据 / Other / 其他角色
```

Finding 展示块固定为：

```text
### <severity emoji> <findingId>｜<中文级别>
📍 位置
❗ 问题
🔎 证据
💥 影响
🛠 修复建议
🧪 是否需要测试
```

报告末尾必须有 `## ➡️ 下一步`：

```text
❌ 未通过       → 优先处理阻断问题；可使用 harness fix finding:<id>
✅ 通过         → 无需处理阻断问题
⚠️ 需要人工处理 → 明确列出未解析项、缺失评审文件或运行时契约校验错误对应动作
```

TARGETED 报告必须保留固定免责声明：

```text
本结论只覆盖本次定向评审范围，不代表整个 Change Set 已完成评审。
```

硬规则：

- `review.md` 是唯一正式 Review Artifact；不得把 `review.json` 作为最终产物。
- Reviewer/Orchestrator 不得使用 arbitrary write_file 自由生成 Markdown。
- `PASSED / FAILED / MANUAL_ACTION_REQUIRED` 都必须生成报告。
- 固定 UI 文案以及 `summary/problem/evidence/impact/recommendation` 默认中文；Java 类名、方法名、文件路径、SQL、异常名、RPC 名、技术名词保持原文。
- 机器 enum 保持英文：`PASSED/FAILED/MANUAL_ACTION_REQUIRED`、`CRITICAL/HIGH/MEDIUM/LOW`、`COMPLETE/PARTIAL`；它们只存在于机器 Contract/内部状态，不得直接作为最终用户摘要文案。
- 测试代码仍必须读取并参与 Coverage；普通测试代码质量不得产生 Finding，只有 Reviewer Test Validity Gate 允许 `TEST_VALIDITY`；用户侧统一显示“测试有效性问题”。
- 无代码变更也必须生成“✅ 通过 / 变更文件 0 / 问题数量 0”的中文报告。
- Coverage PARTIAL 或 Runtime Contract validation error 必须先把 unresolved、missing reviewed files、Runtime validation error 写入结构化 transport，再生成 `MANUAL_ACTION_REQUIRED` 报告，然后 STOP。
- Runtime 报告生成失败时不得继续后续流程，统一 `MANUAL_ACTION_REQUIRED`。
- TARGETED 正式报告中 `Finding.file` 必须属于 Runtime verified `scopedFiles`；Controlled Runtime Renderer 必须拒绝 scope 外 Finding。
- OpenCode 最终摘要必须同时展示中文 Review 结果和 `review.md` 路径，不得泄漏 machine enum。

## `harness review`（1.5.3 ReviewOptions）

plain `harness review` 不再预先固定为 FULL。它必须以同 run Certified ChangeAnalysis 和完整 EntrypointInventory 为事实基线，由 Controlled Runtime 决定本次 Review 模式：

```text
1. 解析并校验 effectiveBaseRef
2. Reviewer.analyze-change 形成 proposal → Runtime analysis certify → Certified ChangeAnalysis
3. EntrypointInventory 必须 COMPLETE；不完整 → MANUAL_ACTION_REQUIRED / STOP
4. Runtime `review options` 生成并持久化 Runtime-owned review-options.json
5. decision=AUTO_FULL（0 valid Chains）→ 不询问用户，立即 `review select`：mode=FULL、无 selectionIds、current optionsHash
6. decision=AUTO_SINGLE（1 valid Chain）→ 不询问用户，立即 `review select`：mode=TARGETED、exact autoSelectionIds、current optionsHash
7. decision=USER_SELECTION（2+ valid Chains）→ 此时才展示一级选择：
   1) 全部评审
   2) 按业务链评审
   3) 仅查看调用链
   - 选择“全部评审” → `review select` mode=FULL、无 Chain IDs
   - 选择“按业务链评审” → 再展示 Runtime 生成的 C1..Cn，多选/编号 fallback；不得默认 ALL
   - 选择“仅查看调用链” → `review select` mode=LIST；不授权 Finding Review
   - 空选择/取消 → STOP
8. Runtime `review select` 必须校验 current optionsHash、Runtime-bound selection IDs，并生成 FULL/TARGETED verified scope；stale/forged/invalid scope 全部 fail closed
9. FULL 使用完整 Change Set machine coverage；TARGETED 使用 Runtime verified scopedFiles/selectedCallChains coverage
10. Coverage 不完整或 Runtime validation 失败 → MANUAL_ACTION_REQUIRED review.md → STOP
11. COMPLETE 后才允许 Reviewer.review-code；findings 为空 → PASSED，非空 → FAILED
12. Controlled Runtime Renderer 生成唯一正式 `.code-harness/runs/<runId>/review.md`，最终摘要使用中文结果并展示 report path
```

`AUTO_SINGLE` 是机器执行规则，不得出现“请选择唯一 Controller/Chain”的冗余提示。`USER_SELECTION` 只是说明存在 2+ valid Chain options；用户仍可明确选择 FULL 或 LIST，只有选择“按业务链评审”时才提交 TARGETED Runtime Chain IDs。

显式 `harness review <Class>` / `<Class.method>` 不走上述 plain ReviewOptions 一级菜单，继续使用下文 direct TARGETED 流程，并由 Runtime 强制包含 Controller target 的全部 required confirmed branches。

用户可见顺序固定为：

```text
统一首屏（结果 / 模式 / 目标 / 数量 / 下一步）
→ 问题概览
→ 评审范围
→ 代码调用链
→ 评审覆盖
→ 问题清单（如允许执行）
→ 评审结论
→ 下一步 + .code-harness/runs/<runId>/review.md
```

## Targeted Review（1.4）

### `harness review list`

只分析当前完整 Change Set 并列出链：

```text
已确认调用链
- <confirmed ChangeAnalysis.callChains[]>

候选/未解析
- <candidate / unresolved symbol + reason>
```

硬规则：

- confirmed 只来自确定性 Code Navigation 已确认的 `ChangeAnalysis.callChains[]`；
- 候选/未解析必须单独展示；
- 不得把 candidate/unresolved 包装成 confirmed；
- 不调用 `review-code`；
- 不生成 Finding；
- LIST 不输出 Review PASSED/FAILED 结论。

### `harness review <Class>` / `<Class.method>`

固定流程：

```text
1. 解析 effectiveBaseRef，建立完整 Change Set
2. Reviewer.analyze-change 建立 ChangeAnalysis、confirmed callChains 与 symbolLocations exact path/role evidence
3. Class → target.kind=CLASS；Class.method → target.kind=METHOD
4. Runtime/Reviewer 使用 symbolLocations.role 判断 target 是否为 Controller；不得靠类名后缀猜角色
5. 从 confirmed callChains 中解析与 target 有证据关系的链
6. 0 条 → NO_REVIEW_TARGET → STOP
7. Controller CLASS → 自动包含该 Controller 当前 Change Set 中全部相关 confirmed chains
8. Controller METHOD → 自动包含该 method 当前 Change Set 中全部相关 confirmed chains
9. Service/其他下游 target：1 条 → AUTO_SINGLE；2+ 条上游业务链 → WAITING_REVIEW_SCOPE_SELECTION
10. scopedFiles 只能取自 symbolLocations 的 exact repository path；同 basename 的其他模块文件不能替代
11. 生成 ReviewScopeSelection(target/selectedCallChains/scopedFiles)
12. Controlled Runtime validate review-scope.schema.json --change-analysis <ChangeAnalysis>
13. Runtime 重新验证 Controller 防漏链、selected chains、exact scoped paths 与 scoped coverage
14. Runtime 机器 scoped coverage != COMPLETE → MANUAL_ACTION_REQUIRED review.md → STOP
15. COMPLETE 后才调用 reviewer.review-code；TARGETED 只允许 Runtime verified scopedFiles / selectedCallChains
16. Controlled Runtime Renderer 再验证 Finding.file ∈ verified scopedFiles 后生成 TARGETED review.md
```

Controller target 不进入用户“挑部分链”流程：

```text
Controller CLASS  → 自动包含全部相关 confirmed chains
Controller METHOD → 自动包含该 method 全部相关 confirmed chains
```

只有 Service/其他下游 target 在解析到 2+ 条上游业务链时才允许用户选择：

- 宿主支持结构化多选 → native multi-select；
- 否则 numbered fallback：`1` / `1,3` / `ALL`；
- **不得默认 `ALL`**；
- 空选择或取消 → STOP；
- Review Scope Selection 不等于 Test/Fix Approval；
- `ALL`/编号选择都不能替代 `批准 <planId>` 或 `批准 <fixPlanId>`。

最终 `ReviewScopeSelection` 必须通过 Runtime `reviewscope.Verify`。如果 Controller CLASS/METHOD 漏掉 required confirmed chain，或 scopedFiles 不等于 Code Navigation exact path evidence，Runtime 必须拒绝，不能依赖 Agent 提示词自律。

TARGETED 报告必须包含：

```text
评审模式：定向评审
评审目标：<target>
Change Set 文件：N
本次 Scope 文件：M
本结论只覆盖本次定向评审范围，不代表整个 Change Set 已完成评审。
```

`harness test` 仍按既有 FULL Review 执行；1.4 Targeted Review 不自动改变 Test Target Selection。

## `harness api-doc`

支持：

```text
harness api-doc OrderController
harness api-doc OrderController.approve
harness api-doc changed
```

固定流程：

```text
1. 创建 runId；全程只读分析。
2. 显式 Controller / Controller.method：API Doc Agent 调用 discover-api，用受控 Code Navigation 唯一定位 target。
3. changed：Orchestrator 解析与 review/test 完全相同的 Review Change Set。
4. changed：Orchestrator 调用 Reviewer.analyze-change 生产 ChangeAnalysis；这里只允许 analyze-change，不调用 reviewer.review-code。
5. changed：把 ChangeAnalysis transport 交给 Controlled Runtime validate_contract(change-analysis.schema.json)：先 Draft 2020-12 Schema 校验，再执行机器 Review Coverage 校验。任一失败 → MANUAL_ACTION_REQUIRED / STOP。
6. changed：只从已通过 Runtime 校验的 ChangeAnalysis 读取 affectedControllers；不得读取未验证缓存，也不得要求用户先跑 harness review。
7. changed target selection：
   - 0 → NO_API_TARGET → STOP
   - 1 → AUTO_SINGLE → 继续
   - 2+ → WAITING_API_SELECTION；native multi-select 优先，否则 numbered fallback（1,3 / ALL）
   - 多 target 不得默认 ALL；空选择/取消 → STOP
8. 将 selected affectedControllers 交给 API Doc Agent；changed discovery 阶段不生成 Finding、不生成 review.md、不进入 Integration Test/Fix。
9. API Doc Agent 调用 generate-api-doc，分析深度固定：Controller → Request DTO → Response DTO/VO → Enum → Validation → Direct Service Method（最多一层）→ STOP。
10. 禁止进入 Repository / Mapper / DAO / DB / MQ / Redis / RPC Server；不得读取真实数据库。
11. 结构化 apiDoc 必须满足 api-doc.schema.json。CONFIRMED/INFERRED 必须带 evidence；无可靠证据优先空数组，禁止编造。
12. Orchestrator/Agent 只把 transport 写入 `.code-harness/runs/<runId>/requests/api-doc.json`，不得自由生成最终 Markdown。
13. 调用 Controlled Runtime：`report api-doc --input .code-harness/runs/<runId>/requests/api-doc.json`。
14. Runtime 再执行 Draft 2020-12 Schema 校验，通过后 deterministic renderer 生成 `.code-harness/runs/<runId>/api-doc.md`，并删除 transport。
15. 最终摘要仅展示 target、endpoint 数和 Api Doc Report path。
```

请求参数位置必须显式映射：`@RequestBody → BODY`、`@RequestParam → QUERY`、`@PathVariable → PATH`、明确业务 `@RequestHeader → HEADER`。这些 transport annotations 不得写入 `validation[]`。Validation 只从源码提取 `@NotNull/@NotBlank/@NotEmpty/@Size/@Length/@Min/@Max/@DecimalMin/@DecimalMax/@Pattern/@Valid`。DTO 递归最大深度 3 并做 cycle detection。Enum 未解析时只保留类型，不得编造值。Error code 只允许 Controller/Direct Service Method 中显式 BizException/ErrorCode/assert evidence。

API Documentation 的 semantic slots：

```text
permissions
preconditions
businessFlow
stateTransitions
dataEffects
externalEffects
transactions
idempotency
errorCodes
testCoverage
businessNotes
```

这些字段只是 Evidence-backed Contract，不授权 Task 3 新建深层权限/事务/测试覆盖分析引擎；只允许在既定分析深度内有证据时填充。

## Test Target Selection 硬门禁（1.2）

仅用于 `harness test`，并且只能在 ChangeAnalysis Schema + Runtime Review Coverage 均通过后执行。

```text
affectedControllers = 0 → NO_TEST_TARGET → DONE
affectedControllers = 1 → AUTO_SINGLE → persist + machine validate → TEST_TARGETS_SELECTED
affectedControllers >= 2 → WAITING_TEST_SELECTION
  ├─ 宿主支持结构化选择 → native multi-select
  └─ 否则 → numbered fallback（1,3 / ALL / DIRECT_ONLY）
取消 → CANCELLED → STOP
```

多 Controller **不得默认全选**。选择结果写入 `.code-harness/runs/<runId>/test-target-selection.json`，并通过 `test-target-selection.schema.json` + Runtime `selection.VerifyJSON` 后，才允许进入 Integration Test Agent。

Selection 只决定测试范围：

```text
Selection != 批准 <planId> != 批准 <fixPlanId>
```

宿主 UI 的确认、`ALL`、`DIRECT_ONLY`、编号选择都不能替代任何写操作审批。

## Runtime Apply Safety Gate（1.4 Task 4）

所有经过人工批准的生产代码/测试代码正式写入统一走：

```text
Agent 设计 exact patch
→ planId/fixPlanId + unifiedDiff + diffSha256 + files[].baseSha256
→ .code-harness/runs/<runId>/requests/apply.json
→ apply-request.schema.json
→ .code-harness/bin/codea-harness-tools seal-apply --input <request>
→ .code-harness/runs/<runId>/sealed-plans/<planId>.json
→ 用户精确批准该 planId
→ 保持同一份 requests/apply.json，不得重新生成
→ .code-harness/bin/codea-harness-tools apply --input <same request>
→ Runtime 对照 sealed snapshot 验证 approval identity
→ Runtime 原子应用
→ apply-result.schema.json
→ .code-harness/runs/<runId>/evidence/apply/<planId>.json
```

### Approval identity

**seal 必须发生在用户批准之前。** 用户批准的是 Runtime 已封存的 immutable approval baseline，而不是“当前碰巧自洽”的 request。

sealed snapshot 固定并在 apply 时逐字段比对：

```text
planId
planType
unifiedDiff exact bytes
diffSha256
files[].path
files[].baseSha256
```

因此：

```text
审批前 seal Patch A
→ 用户批准 Patch A 的 planId
→ request 被替换成自洽 Patch B
→ Runtime compare sealed Patch A vs request Patch B
→ APPROVAL_IDENTITY_MISMATCH
→ 0 写入
```

审批之后只要上述任一字段或 patch bytes 改变，原批准立即失效，必须生成新 planId、新 request，并重新 seal/精确批准。

### Runtime 强制校验

Runtime 不信任 Agent 自报结果，必须独立检查，且顺序固定：

1. **读取 request 之前**先校验 `--input` 路径，只允许精确 `.code-harness/runs/<pathRunId>/requests/*.json`；outside、nested、非 JSON 或 symlink/junction escape 必须在读取前拒绝。
2. 路径通过后才允许读取/解析 JSON；body `runId` 必须与 path runId 一致，否则 `RUN_ID_PATH_MISMATCH`。
3. apply 前必须存在 `.code-harness/runs/<runId>/sealed-plans/<planId>.json`，并逐字段验证 approval identity；缺失 → `SEALED_PLAN_NOT_FOUND`，不一致 → `APPROVAL_IDENTITY_MISMATCH`。
4. `apply-request.schema.json` 合法且无 unknown fields/duplicate paths。
5. `diffSha256` 与 exact unifiedDiff 一致。
6. 当前目标文件 exact bytes SHA256 与 `files[].baseSha256` 一致；否则 `BASE_CHANGED`，0 写入。
7. unified diff 实际 touched path set 与 `files[]` 完全一致。
8. `.git/**` 与 `.code-harness/**` 是 **Runtime hard-deny**，在用户 `harness.yaml.write` 之前执行；即使 `allowedProductionPaths=["**"]` / `allowedTestPaths=["**"]` 且 `deniedPaths=[]` 也必须拒绝，用户配置不得覆盖。
9. hard-deny 通过后，再执行用户 `deniedPaths` 与普通 allowlist：`planType=TEST` 只能写 `allowedTestPaths`；`planType=FIX` 只能写 `allowedProductionPaths`。
10. traversal、absolute path、binary patch、unsafe rename 等不安全 patch 拒绝。
11. 所有文件先读取/校验/stage；任一前置失败必须 0 业务文件写入。
12. 多文件正式提交期间任一失败必须 restore 全部原始 bytes；不得留下半应用状态。
13. 同一成功 planId 已有 evidence 时返回 `PLAN_ALREADY_APPLIED`，不得重复应用。

### 正式完成证据

只有以下条件全部成立才允许 Orchestrator 报告“代码修改完成”：

```text
Runtime status=APPLIED
apply-result.schema.json VALID
.code-harness/runs/<runId>/evidence/apply/<planId>.json 存在
rollbackPerformed=false
```

Evidence 至少包含：

```text
runId
planType
planId
diffSha256
appliedAt
files[].path/beforeSha256/afterSha256
rollbackPerformed
```

`SEALED_PLAN_NOT_FOUND`、`APPROVAL_IDENTITY_MISMATCH`、`RUN_ID_PATH_MISMATCH`、`BASE_CHANGED`、`PLAN_ALREADY_APPLIED`、hard-deny/路径拒绝、diff/hash mismatch、patch set mismatch、rollback 或任何 apply error 都必须 STOP。**direct host write、`write_test`、`apply_approved_patch`、arbitrary write_file 不能作为正式完成或兜底路径。**

## `harness test`

```text
0. 要求 initialization.status=READY，且当前宿主具备文件读取、Maven 执行、超时控制；写入型步骤还要求可调用 Controlled Runtime apply
1. 与 harness review 完全相同地解析 Change Set 并执行 analyze-change
2. Tool Runtime: validate_contract(ChangeAnalysis JSON)，执行 JSON Schema + 机器 Coverage 校验
3. 输出 评审范围 + 评审覆盖（含 validated callChains[]）
4. 无代码变更 → 先生成 Result=PASSED 的中文 review.md，再 `NO_TEST_TARGET` → STOP
5. Runtime 校验失败 / coverage != COMPLETE → 先生成 Result=MANUAL_ACTION_REQUIRED 的中文 review.md，再 STOP；不得设计测试
6. Reviewer.review-code；测试代码 Finding 只允许 TEST_VALIDITY
7. findings 为空 → Result=PASSED；findings 非空 → Result=FAILED；在任何 Test Target Selection 之前生成并确认 `.code-harness/runs/<runId>/review.md`
8. affectedControllers=0 → `NO_TEST_TARGET`，报告后 STOP
9. affectedControllers=1 → `AUTO_SINGLE`；持久化并机器校验 TestTargetSelection，不打断用户
10. affectedControllers>=2 → `WAITING_TEST_SELECTION`；宿主结构化多选优先，否则编号 fallback；取消 → `CANCELLED` STOP
11. 只有 `TEST_TARGETS_SELECTED` 且 selection artifact 机器验证通过，才把 **selected affectedControllers** 交给 Integration Test Agent
12. Integration Test Agent 只对 selected targets 做 Existing Test Coverage Analysis
13. 每个 selected target 独立采用 REUSE_EXISTING / EXTEND_EXISTING / CREATE_NEW；未选择 target 不得生成计划
14. REUSE_EXISTING 直接执行、无审批、零写入；EXTEND/CREATE 必须在审批前产出最终 `unifiedDiff/diffSha256/files[].baseSha256`，生成 `planType=TEST` apply request 并通过 `codea-harness-tools seal-apply --input` 形成 `sealed-plans/<planId>.json`；seal 成功后才允许精确 `批准 <planId>`，批准后只能让同一 request Runtime apply，只有 `APPLIED + evidence/apply/<planId>.json` 才算写入完成
15. Integration Test Agent 返回 method/scenario 级 provenance；Orchestrator 形成 `TestExecutionTarget(testClass,testMethods[],selector,controllerId,origin,planId?)`
16. selected-only execution gate：整类 selector 仅允许该 class 本次相关 methods 全部属于 selected targets；混合 selected+unselected class 必须收窄到 selected method selector；无法安全表达 method selector → `MANUAL_ACTION_REQUIRED`，不得整类执行
17. Runtime Debugger 独占测试执行、日志和 Diagnosis；Surefire `failedTests.testClass + testMethod` 必须回查具体 TestExecutionTarget 判定 method-level origin
18. 新生成/修改测试若 TEST_ERROR：仅失败方法 `origin=GENERATED_BY_PLAN` 且能唯一追溯原 planId 时进入 repair 分析，最多 2 轮；每个不同 repair bytes 必须生成新的 patch identity/new planId/new request，并在审批前重新 Runtime seal，再精确批准后让同一 request apply；同 class 历史 Existing Test method 永不自动修改
19. Existing Test 失败禁止自动修改；PRODUCTION_CODE_ERROR 可生成 Fix Plan，但 Fix Plan 同样必须在审批前生成 apply request 并 Runtime seal exact patch identity，fixPlanId 未批准且同一 sealed request Runtime apply 未成功前不得修改生产代码
```

## `harness upgrade`

不要求 READY，但要求文件读取与受控写入能力。调用 `upgrade-harness`；配置兼容迁移只能由 Tool Runtime 的 registered migration 执行。Framework Managed 必须按新版完整集合 replace，stale Framework 文件必须删除并进入 `removedFiles`。成功时清理 stage/backup/升级源目录；失败时回滚旧 Harness、保留升级源目录并清理临时 stage/backup。Windows 下运行中的 `codea-harness-tools.exe` 禁止原地覆盖，只允许 staged/temp rename 方式替换。状态按 `UPGRADED / ALREADY_UP_TO_DATE / MANUAL_ACTION_REQUIRED / UPGRADE_FAILED` 原样映射。

## 其他意图（保持 1.1.0 语义）

- `harness init`：Project Adapter 识别 Maven/模块/Profile/测试规范/baseRef，生成 `harness.yaml`/`project.md`；不确定项保持 `NEEDS_CONFIRMATION`。
- `harness debug-service`：Runtime Debugger 启动服务，等待人工触发请求，采集日志，诊断，停止本次 processGroup。
- `harness fix finding:<id>` / `fix diagnosis:<runId>`：Fix Agent 先生成带 `unifiedDiff/diffSha256/files[].baseSha256` 的最小 Fix Plan；审批前生成 apply request 并调用 `codea-harness-tools seal-apply --input` 形成 `sealed-plans/<fixPlanId>.json`；只有 seal 成功后才请求精确 `批准 <fixPlanId>`，批准后只允许同一 request 通过 Runtime Apply Safety Gate 正式写入；成功 Apply evidence 后再由 Runtime Debugger 验证。
- `harness verify test/fix/service`：由 Runtime Debugger 执行既有验证路径，不改变审批状态。

## Test Origin / Repair Gate

Origin/provenance 至少细化到 test method/scenario：

```text
TestExecutionTarget
- testClass
- testMethods[]
- selector
- controllerId
- origin = REUSED_EXISTING | GENERATED_BY_PLAN
- planId(optional)
```

硬规则：

- `EXTEND_EXISTING` 的同一 class 可以同时包含两种 origin；禁止把整个 class 一刀切标成 GENERATED_BY_PLAN 或 REUSED_EXISTING。
- Surefire 返回失败方法后，用 `failedTests.testClass + testMethod` 唯一匹配 TestExecutionTarget。
- 只有匹配到 `GENERATED_BY_PLAN` 的具体失败 method 才进入 repair 轮次；对应来源 planId 必须存在。
- `REUSED_EXISTING` method 永不自动修改，即使它与 GENERATED_BY_PLAN method 位于同一个 class。
- `testMethod=null`、provenance 冲突或无法唯一匹配时默认走安全路径：不得 repair，进入测试修改计划 / `MANUAL_ACTION_REQUIRED`。
- repair 计数仍按来源 `planId + testClass + testMethod`，最多 2 轮；达到上限 → `MANUAL_TEST_REPAIR_REQUIRED`。
- 每轮 repair 如果产生不同代码 bytes，就必须生成新的 `unifiedDiff/diffSha256/files[].baseSha256/new planId/new request`，审批前重新 Runtime seal，再获得新的精确批准；旧批准不能直接 authorize 新 patch。

## 统一结果

以下是**用户可见统一摘要**；机器状态只保留在 JSON/内部状态，不能直接输出到该摘要：

```text
评审结果：✅ 通过 | ❌ 未通过 | ⏳ 等待批准 | ⚠️ 需要人工处理

完成：
- 评审 N 个文件
- 生成 M 个测试类
- 执行 K 个场景

发现：
- X 个生产代码问题
- Y 个测试有效性问题

下一步：
- 请批准 <planId> | <fixPlanId>
- 或：所有测试通过，无需进一步操作
```

对于包含 Review 阶段的 `harness review` / `harness test`，统一摘要中还必须显示：

```text
代码评审报告：
.code-harness/runs/<runId>/review.md
```

对于 `harness api-doc`，统一摘要必须显示：

```text
API 文档报告：
.code-harness/runs/<runId>/api-doc.md
```

对于成功执行 Runtime Apply 的测试/修复，统一摘要还必须显示：

```text
代码修改证据：
.code-harness/runs/<runId>/evidence/apply/<planId>.json
```

不得把宿主 direct host write 的成功响应当作该证据。

## Task 7：Selected Test Flow + Integration-Test DB Assertions

以下规则是在现有 `harness test` / Existing Test / Approval / Repair Gate 之上增加，不替代原语义；本节的 method-level 规则覆盖此前 Task 7 的 class-level handoff/origin 表述：

1. selected target 的 ChangeAnalysis 出现 `databaseWrite / transactional / stateTransition` 风险时，Integration Test Agent 必须显式决定 DB Assertion 是否需要；需要时把具体断言写入现有 `expected.databaseAssertions[]`。
2. DB Assertion 是正式测试证据；生成时只允许复用项目已有 helper/repository、existing JdbcTemplate、existing fixture/assertion utility；不得为此新增 Maven dependency，且断言必须在 cleanup/rollback 隐藏状态之前完成。
3. Integration Test Agent 返回 method/scenario 级 provenance，Orchestrator 生成 TestExecutionTarget，不再以“class 能追溯到 selected target”作为充分放行条件。
4. Selected-only 执行门禁：
   - 专属 selected Controller 的 class 可整类执行；
   - 同时覆盖 selected + unselected Controller 的 class 必须只执行 selected methods；
   - 当前 Maven/Surefire 固定配置无法安全表达所需 `Class#method`/方法集合 selector 时 → `SCOPE_VIOLATION / MANUAL_ACTION_REQUIRED`；禁止退化成整类。
5. Method-level repair provenance：

```text
PaymentControllerIT.oldTestA
origin=REUSED_EXISTING

PaymentControllerIT.newMissingTest
origin=GENERATED_BY_PLAN
planId=test-plan-xxx
```

`oldTestA` 失败永不自动改；只有 `newMissingTest` 失败且 Diagnosis=`REPAIR_TEST` 才进入最多 2 轮 repair 分析；任何不同 repair patch 仍需新 patch identity + 新 planId + 审批前 Runtime seal + 精确批准 + 同一 request Runtime apply。
6. Synthetic Golden Flow 必须满足：

```text
Affected Controllers: Order, Payment, User
Selection: Order + Payment

Order -> REUSE_EXISTING -> no approval/no write -> selected-only TestExecutionTarget
Payment -> EXTEND_EXISTING -> exact patch identity -> planType=TEST request -> pre-approval Runtime seal -> 精确批准 <paymentPlanId> -> same request Runtime apply -> modify only MISSING -> method-level provenance
User -> unselected

User 必须没有：
- Existing Test coverage analysis artifact
- Test Plan target
- generated/modified test artifact
- Runtime execution artifact

若 CommonControllerIT 同时包含 Order + User methods：
- 只允许执行 Order method selector
- 无法安全 method-filter -> MANUAL_ACTION_REQUIRED
```

7. 任意阶段把 User 或其他 unselected Controller 自动补回，均视为 `SCOPE_VIOLATION`。
8. `REUSE_EXISTING -> run/no approval/no modification`、`EXTEND_EXISTING -> only MISSING + pre-approval seal + exact planId approval + same request Runtime Apply`、`CREATE_NEW -> pre-approval seal + exact planId approval + same request Runtime Apply`、historical Existing Test method never auto-edit、GENERATED_BY_PLAN method repair max 2 rounds 全部保持不变。

## 禁止行为

- 不得跳过 Review Coverage / Runtime Contract 校验 / Review Report Persistence / Test Target Selection / 审批门禁。
- `harness api-doc` 不得跳过 api-doc Schema Validation / API Target Selection / Controlled Runtime Renderer。
- 不得让 Reviewer/Orchestrator/API Doc Agent 自由写 `review.md` 或 `api-doc.md`；只能调用 Controlled Runtime Renderer。
- 不得把 TARGETED 结果表述成整个 Change Set 已完整评审。
- 不得把任何 direct host write / `write_test` / `apply_approved_patch` / arbitrary write_file 作为生产或测试代码正式写入成功。
- 不得跳过审批前 `codea-harness-tools seal-apply --input` 或修改 sealed request 后复用旧批准。
- 不得把手工写入 Runtime-owned Chain candidate/certificate/write-plan path 的内容视为 authority；最终 Chain Project State 写入必须经过 Runtime provenance、Certified Analysis、immutable write plan 与 exact planId confirmation。
- 不得超过 2 轮 GENERATED_BY_PLAN repair 计数。
- 不得直接执行任意 Shell。
- 不得自动 commit/push/PR。


### Semantic Chain Editing（1.5.3 Task 5）

`harness chain edit <id|Controller|Controller.method>` 固定路由到 `edit-chain`。Orchestrator 只能在同 run `requests/**` 提交 `REPLACE_NODE / ADD_NODE / REMOVE_NODE / REORDER_NODE / RENAME_CHAIN / UPDATE_NOTES` proposal。

```text
现有 ACCEPTED Chain
→ same-run Certified ChangeAnalysis
→ Controlled Runtime chain edit
→ analysis/chain-edit-candidates/<id>.yaml + provenance(kind=EDIT)
→ 展示 deterministic diff
→ 用户首次保存意图
→ chain seal-persist
→ exact preview + planId
→ 用户明确确认当前 planId
→ chain persist(runId + planId only)
→ atomic Project State write
```

`chain edit` 本身不得直接写 `.code-harness/chains/**`；不得改 EntryPoint；不得用 dependency workspace、类名后缀、basename、字符串包含或 fuzzy relation 获得写 authority。candidate/analysis/plan/existing Project State 任一变化都使旧 planId 失效。
