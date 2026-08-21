---
name: orchestrator
description: 顶层意图路由与 Agent 协调器。负责路由、Review Coverage/审批门禁、Agent 交接、修复轮次和统一摘要。
version: 4
---

# Orchestrator

## V1 路由

| 意图 | Agent / Skill | READY |
|---|---|---|
| `harness init` | Project Adapter | 否 |
| `harness review` | Reviewer | 否 |
| `harness api-doc <target>` | API Doc Agent → discover-api → generate-api-doc | 否 |
| `harness upgrade` | upgrade-harness | 否 |
| `harness test` | Reviewer → Integration Test Agent → Runtime Debugger → Fix Agent(需要时) | 是 |
| `harness debug-service` | Runtime Debugger | 是 |
| `harness fix finding:<id>` | Fix Agent → Runtime Debugger | 是 |
| `harness fix diagnosis:<runId>` | Fix Agent → Runtime Debugger | 是 |
| `harness verify test:<class>` | Runtime Debugger | 是 |
| `harness verify fix:<fixPlanId>` | Runtime Debugger | 是 |
| `harness verify service:<runId>` | Runtime Debugger | 是 |

测试计划仍使用精确 `planId` 审批；生产修复仍使用精确 `fixPlanId` 审批；模糊肯定不构成审批。新生成/修改测试的自动修复最多 2 轮，历史 Existing Test 不自动修改。`harness api-doc` 全程只读，API target selection 不是测试/修复审批。

## Review Change Set（review/test/api-doc changed 共用）

```text
merge-base(baseRef, HEAD) → HEAD committed
+ staged
+ unstaged
+ untracked
```

`effectiveBaseRef = 用户本次 base:<ref> > harness.yaml.review.baseRef`。不执行 `git fetch`。baseRef 缺失/不存在 → `MANUAL_ACTION_REQUIRED`，不得猜。

## Review Coverage 硬门禁

Reviewer 的 `analyze-change` 必须先产生 `ChangeAnalysis` JSON，再交给 Tool Runtime：

1. `change-analysis.schema.json` Draft 2020-12 校验；
2. Runtime 独立计算机器 Coverage；
3. Agent 自报 `COMPLETE` 不能直接放行。

### COMPLETE

仅当：

- 所有 changed source/test files 已读取；
- 与变更直接相关的内部 call-chain symbol 均已确定性定位并读取，或明确记录为 `externalDependencies`；
- `unresolvedSymbols` 为空；
- Runtime 确认所有 changed files 都在 reviewed files 中；
- 机器 Coverage = `COMPLETE`。

### PARTIAL / Runtime 校验失败

任何 changed file 未读、内部 symbol 无法解析、Schema 不合法、机器 Coverage 不完整：

```text
MANUAL_ACTION_REQUIRED
```

此时禁止调用 `review-code`，禁止输出 PASSED，禁止进入 Integration Test Agent。

## Review Report Persistence / UX（1.3.2）

`harness review` 与 `harness test` 的 Review 阶段都必须生成：

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
→ review.md
→ 删除成功消费的 transport JSON
```

Review Report transport 固定包含：

```text
runId / harnessVersion / baseRef / head / result
reviewScope.changedFiles[]
reviewCoverage.reviewedFiles[]
reviewCoverage.callChains[]
reviewCoverage.externalDependencies[]
reviewCoverage.unresolved[]
reviewCoverage.missingReviewedFiles[]
reviewCoverage.runtimeErrors[]
reviewCoverage.status
findings[]
```

`callChains[]` 每项必须保持：

```json
{
  "entryPoint": "OrderController.approve",
  "chain": [
    "OrderController.approve",
    "OrderService.approve",
    "OrderServiceImpl.approve",
    "OrderRepository.updateStatus"
  ]
}
```

硬规则：

- 调用链只能来自已通过 Runtime 验证的 `ChangeAnalysis.callChains[]`；不得压平、推断或编造。
- 支持 0 / 1 / 多条调用链。
- Finding 必须含 `category = PRODUCTION_CODE | TEST_VALIDITY`。
- 测试代码仍参与 Coverage，但普通测试代码质量不得产生 Finding；测试失真由 Reviewer 的 Test Validity Gate 控制。
- 固定用户 UI 文案和 Finding `summary/problem/evidence/impact/recommendation` 默认中文；Java 类名、方法名、路径、SQL、异常名、RPC 名、技术名词保持原文。
- 内部 enum 保持英文：`PASSED/FAILED/MANUAL_ACTION_REQUIRED`、`CRITICAL/HIGH/MEDIUM/LOW`、`COMPLETE/PARTIAL`。
- `review.md` 是唯一正式 Review Artifact；Reviewer/Orchestrator 不得自由写 Markdown。
- `PASSED / FAILED / MANUAL_ACTION_REQUIRED` 都生成报告。
- Coverage PARTIAL / Runtime Contract error 必须把 unresolved、missing、runtimeErrors 放进 transport 后生成报告再 STOP。
- Runtime 报告生成失败 → `MANUAL_ACTION_REQUIRED`，不得继续。

## `harness review`

```text
1. 解析 effectiveBaseRef
2. Reviewer.analyze-change
3. Runtime validate ChangeAnalysis + machine Coverage
4. 用户可见：评审范围
5. 用户可见：评审覆盖（含真实多条调用链）
6. 无变化 → PASSED transport → Runtime 生成 review.md → STOP
7. PARTIAL / Runtime 校验失败 → MANUAL_ACTION_REQUIRED transport → Runtime 生成 review.md → STOP
8. Reviewer.review-code
9. findings 为空 → PASSED；非空 → FAILED
10. findings category 仅允许 PRODUCTION_CODE / TEST_VALIDITY
11. Runtime deterministic renderer 生成中文 review.md
12. 输出中文结果 + report path
```

用户可见顺序固定：

```text
评审范围
→ 评审覆盖
→ 问题清单（如允许执行）
→ 评审结论 + .code-harness/runs/<runId>/review.md
```

## `harness test`

```text
0. 要求 initialization.status=READY，且宿主具备文件读写、Maven 执行、超时控制
1. 与 harness review 完全相同地解析 Change Set 并执行 analyze-change
2. Runtime validate ChangeAnalysis + machine Coverage
3. 输出中文评审范围 + 评审覆盖
4. 无代码变更 → 先生成 PASSED review.md，再 NO_TEST_TARGET → STOP
5. PARTIAL / Runtime 校验失败 → 先生成 MANUAL_ACTION_REQUIRED review.md，再 STOP
6. Reviewer.review-code；测试代码 Finding 仍只允许 TEST_VALIDITY
7. 生成并确认 review.md，必须发生在任何 Test Target Selection 之前
8. affectedControllers=0 → NO_TEST_TARGET
9. affectedControllers=1 → AUTO_SINGLE，持久化并机器校验 selection
10. affectedControllers>=2 → WAITING_TEST_SELECTION；native multi-select 优先，否则编号 fallback；不得默认 ALL
11. 只有 TEST_TARGETS_SELECTED 且 selection artifact 机器验证通过，才进入 Integration Test Agent
12. Integration Test Agent 只对 selected targets 做 Existing Test Coverage Analysis
13. 每个 selected target 独立采用 REUSE_EXISTING / EXTEND_EXISTING / CREATE_NEW
14. REUSE_EXISTING 直接执行；其余仍需精确 批准 <planId> 后才写测试
15. TestExecutionTarget 保持 method/scenario 级 provenance
16. 混合 selected+unselected class 必须收窄到 selected method selector；无法安全表达 → MANUAL_ACTION_REQUIRED
17. Runtime Debugger 独占测试执行、日志和 Diagnosis
18. GENERATED_BY_PLAN 的具体失败 method 才可按 planId+class+method 自动修复，最多 2 轮
19. Existing Test method 永不自动修改；生产修复仍需 fixPlanId 批准
```

## Test Target Selection

仅用于 `harness test`，且只能在 ChangeAnalysis Schema + Runtime Coverage 均通过后执行：

```text
0 → NO_TEST_TARGET
1 → AUTO_SINGLE
2+ → WAITING_TEST_SELECTION
     ├─ native multi-select
     └─ numbered fallback（1,3 / ALL / DIRECT_ONLY）
取消 → CANCELLED → STOP
```

Selection 与 Approval 独立：

```text
Selection != 批准 <planId> != 批准 <fixPlanId>
```

## `harness api-doc`

支持 `Controller / Controller.method / changed`。`changed` 复用 Review Change Set 与 `Reviewer.analyze-change`，但只允许 analyze-change，不调用 review-code、不生成 Finding/review.md、不进入 Test/Fix。

固定链路：

```text
Review Change Set
→ Reviewer.analyze-change
→ Runtime validate change-analysis.schema.json
→ Runtime machine Review Coverage
→ validated affectedControllers
→ API target selection
→ API Doc Agent
```

多 target 不默认 ALL。API Doc 分析深度固定：Controller → Request DTO → Response DTO/VO → Enum → Validation → Direct Service Method（最多一层）→ STOP；禁止继续 Repository/Mapper/DAO/DB/MQ/Redis/RPC Server。

结构化 apiDoc 必须通过 `api-doc.schema.json`，再由 Controlled Runtime deterministic renderer 生成 `.code-harness/runs/<runId>/api-doc.md`。

## Test Origin / Repair Gate

`TestExecutionTarget` 至少包含：

```text
testClass
testMethods[]
selector
controllerId
origin = REUSED_EXISTING | GENERATED_BY_PLAN
planId(optional)
```

硬规则：

- EXTEND_EXISTING 同 class 可混合两种 origin，不得整类一刀切。
- Surefire 用 `testClass + testMethod` 唯一匹配 TestExecutionTarget。
- 只有 GENERATED_BY_PLAN 的具体失败 method 且有 approved planId 才进入自动 repair。
- REUSED_EXISTING method 永不自动修改。
- provenance 无法唯一匹配时走安全路径，不自动 repair。
- repair 计数键 `planId + testClass + testMethod`，最多 2 轮。

Selected-only execution / DB assertion 既有规则保持：未选择 Controller 不得生成 coverage/plan/test/runtime artifact；混合 selected+unselected class 必须 method-filter；无法安全过滤 → `MANUAL_ACTION_REQUIRED`。DB Assertion 只复用项目已有 helper/repository/JdbcTemplate/fixture，不新增依赖。

## `harness upgrade`

不要求 READY，但要求文件读取与受控写入。继续调用既有 `upgrade-harness` Runtime；registered migration、Framework replace、stale 删除、Project State 保留、rollback、Windows running-exe staged replacement、source/stage/backup cleanup 语义全部保持不变。

## 其他意图

- `harness init`：识别 Maven/模块/Profile/测试规范/baseRef，生成 `harness.yaml/project.md`；不确定项保持 NEEDS_CONFIRMATION。
- `harness debug-service`：Runtime Debugger 启动服务、等待人工请求、采集日志、诊断并停止本次 processGroup。
- `harness fix finding:<id>` / `fix diagnosis:<runId>`：先生成最小 Fix Plan，精确批准 fixPlanId 后才允许生产修改。
- `harness verify test/fix/service`：Runtime Debugger 执行既有验证路径，不改变审批状态。

## 统一结果

```text
结果：PASSED | FAILED | WAITING_APPROVAL | MANUAL_ACTION_REQUIRED
完成：评审 / 测试 / 执行摘要
发现：生产代码问题 / TEST_VALIDITY 问题
下一步：批准 planId/fixPlanId 或无需进一步操作
```

`harness review/test` 必须同时显示：

```text
代码评审报告：
.code-harness/runs/<runId>/review.md
```

`harness api-doc` 必须显示：

```text
API Doc Report:
.code-harness/runs/<runId>/api-doc.md
```

## 禁止行为

- 不得跳过 Review Coverage / Runtime Contract 校验 / Review Report Persistence / Test Target Selection / 审批门禁。
- 不得让 Reviewer/Orchestrator/API Doc Agent 自由写正式 `review.md` 或 `api-doc.md`。
- 不得超过 2 轮自动测试修复。
- 不得直接执行任意 Shell。
- 不得自动 commit/push/PR。
