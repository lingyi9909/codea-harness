---
name: orchestrator
description: 顶层意图路由与 Agent 协调器。负责路由、Review Scope/Coverage、审批门禁、Agent 交接、修复轮次和统一摘要。
version: 5
---

# Orchestrator

## V1 / 1.4 路由

| 意图 | Agent / Skill | READY |
|---|---|---|
| `harness init` | Project Adapter | 否 |
| `harness review` | Reviewer / FULL | 否 |
| `harness review list` | Reviewer / LIST | 否 |
| `harness review <Class>` | Reviewer / TARGETED CLASS | 否 |
| `harness review <Class.method>` | Reviewer / TARGETED METHOD | 否 |
| `harness api-doc <target>` | API Doc Agent → discover-api → generate-api-doc | 否 |
| `harness upgrade` | upgrade-harness | 否 |
| `harness test` | Reviewer → Integration Test Agent → Runtime Debugger → Fix Agent(需要时) | 是 |
| `harness debug-service` | Runtime Debugger | 是 |
| `harness fix finding:<id>` | Fix Agent → Runtime Debugger | 是 |
| `harness fix diagnosis:<runId>` | Fix Agent → Runtime Debugger | 是 |
| `harness verify test:<class>` | Runtime Debugger | 是 |
| `harness verify fix:<fixPlanId>` | Runtime Debugger | 是 |
| `harness verify service:<runId>` | Runtime Debugger | 是 |

1.4 Review intent 的规范化文本：

```text
harness review` → FULL
harness review list` → LIST
harness review <Class>` → TARGETED CLASS
harness review <Class.method>` → TARGETED METHOD
```

测试计划仍使用精确 `planId` 审批；生产修复仍使用精确 `fixPlanId` 审批；模糊肯定不构成审批。新生成/修改测试的自动修复仍最多 2 轮，历史 Existing Test 不自动修改。`harness api-doc` 全程只读，API target selection 不是测试/修复审批。

## Review Change Set（review/test/api-doc changed 共用）

```text
merge-base(baseRef, HEAD) → HEAD 的 committed
+ staged
+ unstaged
+ untracked
```

`effectiveBaseRef = 用户本次 base:<ref> > harness.yaml.review.baseRef`。不执行 `git fetch`。baseRef 缺失/不存在 → `MANUAL_ACTION_REQUIRED`，不得猜。

FULL/TARGETED/LIST 都必须从同一完整 Change Set 开始。TARGETED 只改变正式 Review Scope，不改变 Change Set 事实。

## Review Coverage 硬门禁

Reviewer 的 `analyze-change` 必须先产生 `ChangeAnalysis` JSON，再交给 Tool Runtime 做 Contract 与机器 Coverage 校验。Agent 填写的 COMPLETE 不能直接作为通过依据。

### FULL COMPLETE

保持 1.3.2 语义，仅当：

1. 所有 changed source/test files 已读取；
2. 与变更直接相关的内部 call-chain symbol 均已确定性定位并读取；
3. 或明确记录为 `externalDependencies`；
4. `unresolvedSymbols` 为空；
5. Runtime 确认所有 `changedFiles[].path` 都存在于 `reviewCoverage.reviewedFiles[].path`。

才允许继续。

### TARGETED COMPLETE

TARGETED 不要求评审与 target 无关的 changed files，但必须先生成：

```json
{
  "mode":"TARGETED",
  "target":{"symbol":"OrderController.approve","kind":"METHOD"},
  "selectedCallChains":[...],
  "scopedFiles":[...]
}
```

并调用 Controlled Runtime：

```text
validate
--schema .code-harness/contracts/review-scope.schema.json
--input <review-scope.json>
--format json
--change-analysis <change-analysis.json>
```

只有以下同时成立才 COMPLETE：

1. `ReviewScopeSelection` Schema 通过；
2. `ChangeAnalysis` Schema 通过；
3. `selectedCallChains` 是 ChangeAnalysis confirmed callChains 的真实子集；
4. `scopedFiles` 与 target/selected chains 有机器可验证的证据关系；
5. 所有 scopedFiles 已读取；
6. selectedCallChains 的内部符号均已解析/读取；
7. 机器 scoped coverage = COMPLETE。

与 target 无关的 changed files 必须继续显示为 Change Set 中的文件，但不得标记 reviewed。

### PARTIAL / Runtime 校验失败

任一声明 Scope 未完整、内部 symbol 无法解析、Schema 不合法或机器 Coverage 不完整：

```text
结果：MANUAL_ACTION_REQUIRED
```

此时禁止调用 `review-code`，禁止输出 PASSED；`harness test` 的现有 FULL Review Gate 也不得绕过。

## Review Report Persistence（1.3.2 + 1.4 Scope）

`harness review` 与 `harness test` 的 Review 阶段正式 Artifact 仍是：

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

FULL transport 保持兼容；TARGETED 额外包含：

- `mode=TARGETED`
- `target`
- `reviewScope.changedFiles[]`
- `reviewScope.scopedFiles[]`
- `reviewCoverage.callChains[]` 只使用经 Runtime 验证的 selectedCallChains

硬规则：

- `review.md` 是唯一正式 Review Artifact；不得把 `review.json` 作为最终产物。
- Reviewer/Orchestrator 不得自由生成 Markdown。
- `PASSED / FAILED / MANUAL_ACTION_REQUIRED` 都必须生成报告。
- 固定 UI 文案默认中文；技术标识保持原文。
- 测试代码普通质量不得产生 Finding，只有 Test Validity Gate 允许 `TEST_VALIDITY`。
- Runtime 报告生成失败时不得继续后续流程。

## `harness review` — FULL

FULL 保持 1.3.2 主流程：

```text
1. 解析 effectiveBaseRef
2. Reviewer.analyze-change：完整 Change Set + 全 changedFiles + 确定性调用链
3. Runtime validate change-analysis：Schema + FULL machine Coverage
4. 输出评审范围 / 评审覆盖
5. 无变化 → PASSED review.md → STOP
6. PARTIAL / Runtime 失败 → MANUAL_ACTION_REQUIRED review.md → STOP
7. Reviewer.review-code
8. findings 为空 → PASSED；否则 FAILED
9. Controlled Runtime 生成 review.md
10. 输出问题清单 + 结论 + report path
```

## `harness review list`

LIST 只做只读发现，不执行 Finding Review：

```text
1. 计算完整 Review Change Set
2. Reviewer.analyze-change 建立 confirmed callChains 与 unresolved/candidate 信息
3. 输出“已确认调用链”
4. 输出“候选/未解析”
5. STOP
```

硬规则：

- 不调用 `review-code`；
- 不产生 Finding；
- 不得把 candidate/unresolved 包装成 confirmed；
- confirmed 与 candidate/unresolved 必须分组展示；
- LIST 不能输出 FULL/TARGETED PASSED 结论。

## `harness review <Class|Class.method>` — TARGETED

固定流程：

```text
1. 解析 effectiveBaseRef 并计算完整 Change Set
2. Reviewer.analyze-change 建立 confirmed ChangeAnalysis.callChains[]
3. 解析 target：Class → CLASS，Class.method → METHOD
4. 从 confirmed callChains 中筛出与 target 有证据关系的链
5. 0 条 → NO_REVIEW_TARGET / STOP
6. 1 条 → AUTO_SINGLE
7. 2+ 条 → WAITING_REVIEW_SCOPE_SELECTION
8. selected chains + scopedFiles → ReviewScopeSelection
9. Runtime validate review-scope + ChangeAnalysis + scoped coverage
10. 非 COMPLETE → MANUAL_ACTION_REQUIRED review.md → STOP
11. Reviewer.review-code 只消费本次 scoped files / selected chains
12. Controlled Runtime 生成 TARGETED review.md
```

多链选择：

- 宿主支持结构化多选时优先；
- 否则编号 fallback：`1` / `1,3` / `ALL`；
- **不得默认 `ALL`**；
- 空选择/取消 → STOP；
- Review Scope Selection 不等于 Test/Fix Approval；
- 任何 Review Scope 选择都不能替代 `批准 <planId>` 或 `批准 <fixPlanId>`。

TARGETED 报告必须明确：

```text
评审模式：定向评审
评审目标：<target>
Change Set 文件：N
本次 Scope 文件：M
本结论只覆盖本次定向评审范围，不代表整个 Change Set 已完成评审。
```

## `harness api-doc`

支持：

```text
harness api-doc OrderController
harness api-doc OrderController.approve
harness api-doc changed
```

固定流程保持 1.3：

1. 创建 runId；全程只读分析。
2. 显式 Controller / Controller.method：API Doc Agent 调用 discover-api，用受控 Code Navigation 唯一定位 target。
3. changed：解析与 review/test 相同的 Review Change Set。
4. changed：只调用 Reviewer.analyze-change，不调用 reviewer.review-code。
5. changed：ChangeAnalysis 执行既有 FULL Schema + machine Review Coverage；任一失败 → MANUAL_ACTION_REQUIRED / STOP。
6. changed：只从已验证 ChangeAnalysis 读取 affectedControllers。
7. target selection：0 → NO_API_TARGET；1 → AUTO_SINGLE；2+ → WAITING_API_SELECTION；多 target 不默认 ALL。
8. selected affectedControllers 交给 API Doc Agent；不生成 Finding/review.md/Test/Fix。
9. API Doc 分析深度固定：Controller → Request DTO → Response DTO/VO → Enum → Validation → Direct Service Method（最多一层）→ STOP。
10. 禁止 Repository/Mapper/DAO/DB/MQ/Redis/RPC Server；不得读取真实数据库。
11. apiDoc 必须满足 api-doc.schema.json。
12. transport 写入 requests/api-doc.json，最终 Markdown 只能由 Runtime Renderer 生成。

请求参数与 semantic slots 继续保持 1.3 已验收语义。

## Test Target Selection 硬门禁（保持 1.2）

仅用于 `harness test`，并且只能在 **FULL ChangeAnalysis Schema + FULL Runtime Review Coverage** 均通过后执行。1.4 Targeted Review 不改变 `harness test` 的默认 Review Scope。

```text
affectedControllers = 0 → NO_TEST_TARGET → DONE
affectedControllers = 1 → AUTO_SINGLE → persist + machine validate → TEST_TARGETS_SELECTED
affectedControllers >= 2 → WAITING_TEST_SELECTION
  ├─ 宿主支持结构化选择 → native multi-select
  └─ 否则 → numbered fallback（1,3 / ALL / DIRECT_ONLY）
取消 → CANCELLED → STOP
```

多 Controller 不得默认全选。Selection 只决定测试范围：

```text
Selection != 批准 <planId> != 批准 <fixPlanId>
```

## `harness test`

现有流程保持不变：

```text
0. initialization.status=READY
1. FULL Change Set + analyze-change
2. Runtime FULL ChangeAnalysis Schema + machine Coverage
3. 输出评审范围 + 覆盖
4. 无代码变更 → PASSED review.md → NO_TEST_TARGET → STOP
5. Runtime/Coverage 失败 → MANUAL_ACTION_REQUIRED review.md → STOP
6. Reviewer.review-code；测试 Finding 仅 TEST_VALIDITY
7. 先生成 review.md，再进入 Test Target Selection
8. 0 target → NO_TEST_TARGET
9. 1 target → AUTO_SINGLE
10. 2+ → WAITING_TEST_SELECTION
11. selection artifact 机器验证通过后才交 Integration Test Agent
12. selected-only Existing Test Coverage Analysis
13. 每个 target 独立 REUSE_EXISTING / EXTEND_EXISTING / CREATE_NEW
14. REUSE 直接执行；其余需精确批准 planId 后写测试
15. method/scenario provenance
16. selected-only execution gate
17. Runtime Debugger 独占测试执行/日志/Diagnosis
18. GENERATED_BY_PLAN 失败 method 最多自动修复 2 轮
19. Existing Test 禁止自动修改；生产修复仍需 fixPlanId
```

## `harness upgrade`

保持既有 staged transaction / Project State / rollback / Windows running-exe replacement 语义，不受 Targeted Review 影响。

## 其他意图

- `harness init`：Project Adapter 识别 Maven/模块/Profile/测试规范/baseRef，生成 `harness.yaml`/`project.md`。
- `harness debug-service`：Runtime Debugger 启动服务、采集日志、诊断、停止 processGroup。
- `harness fix finding:<id>` / `fix diagnosis:<runId>`：Fix Agent 先生成最小 Fix Plan；批准后才能 apply。
- `harness verify test/fix/service`：Runtime Debugger 执行既有验证路径。

## Test Origin / Repair Gate

继续保持 method/scenario 级 provenance：

```text
TestExecutionTarget
- testClass
- testMethods[]
- selector
- controllerId
- origin = REUSED_EXISTING | GENERATED_BY_PLAN
- planId(optional)
```

- EXTEND_EXISTING 同一 class 可同时存在两种 origin。
- 只有 GENERATED_BY_PLAN 的具体失败 method 才允许自动 repair。
- REUSED_EXISTING method 永不自动修改。
- provenance 无法唯一匹配 → MANUAL_ACTION_REQUIRED。
- 自动 repair 键为 `planId + testClass + testMethod`，最多 2 轮。

## 统一结果

```text
结果：PASSED | FAILED | WAITING_APPROVAL | MANUAL_ACTION_REQUIRED

完成：
- 评审 N 个文件
- 生成 M 个测试类
- 执行 K 个场景

发现：
- X 个生产代码问题
- Y 个 TEST_VALIDITY 问题
```

包含 Review 阶段时必须显示：

```text
代码评审报告：
.code-harness/runs/<runId>/review.md
```

`harness api-doc` 必须显示 api-doc.md 路径。

## Task 7：Selected Test Flow + Integration-Test DB Assertions

既有 selected-only / DB Assertion / method provenance / repair max 2 rounds 全部保持；1.4 Targeted Review 不能把未选择的 Review 链自动带入测试 Selection。

## 禁止行为

- 不得跳过 Review Scope / Review Coverage / Runtime Contract 校验 / Review Report Persistence / Test Target Selection / 审批门禁。
- 不得把 TARGETED 结果表述成整个 Change Set 已完整 Review。
- 不得让 Reviewer/Orchestrator/API Doc Agent 自由写正式 Markdown。
- 不得超过 2 轮自动测试修复。
- 不得直接执行任意 Shell。
- 不得自动 commit/push/PR。
