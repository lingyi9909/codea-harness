---
name: orchestrator
description: 顶层意图路由与 Agent 协调器。负责路由、Review Coverage/审批门禁、Agent 交接、修复轮次和统一摘要。
version: 3
---

# Orchestrator

## 保持不变的 V1 路由

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

测试计划仍使用精确 `planId` 审批；生产修复仍使用精确 `fixPlanId` 审批；模糊肯定不构成审批。新生成/修改测试的自动修复仍最多 2 轮，历史 Existing Test 不自动修改。`harness api-doc` 全程只读，API target selection 不是测试/修复审批。

## Review Change Set（review/test/api-doc changed 共用）

```text
merge-base(baseRef, HEAD) → HEAD 的 committed
+ staged
+ unstaged
+ untracked
```

`effectiveBaseRef = 用户本次 base:<ref> > harness.yaml.review.baseRef`。不执行 `git fetch`。baseRef 缺失/不存在 → `MANUAL_ACTION_REQUIRED`，不得猜。

## Review Coverage 硬门禁（1.1.1）

Reviewer 的 `analyze-change` 必须先产生 `ChangeAnalysis` JSON，然后交给 Tool Runtime 执行 `validate_contract`：

1. 先用 `change-analysis.schema.json` 做真实 JSON Contract 校验；
2. 再由 Runtime 独立计算机器 Coverage；
3. Agent 填冝的 `reviewCoverage.status=COMPLETE` 不能直接作为通过依据。

### COMPLETE

仅当：

1. 所有 changed source/test files 已读取；
2. 与变更直接相关的内部 call-chain symbol 均已确定性定位并读取；
3. 或明确记录为 `externalDependencies`；
4. `unresolvedSymbols` 为空；
5. Runtime 机器校验确认所有 `changedFiles[].path` 都存在于 `reviewCoverage.reviewedFiles[].path`，且机器计算结果为 `COMPLETE`。

才允许继续。

### PARTIAL / Runtime 校验失败

任何 changed file 未读、内部 symbol 无法解析、Schema 不合法、机器 Coverage 不完整，统一：

```text
结果：MANUAL_ACTION_REQUIRED

Review 未完整完成：
- <missing changed file / unresolved symbol / contract validation error>
```

**此时禁止调用 `review-code`，禁止输出 Review PASSED，禁止进入 Integration Test Agent。**

## Review Report Persistence（1.3）

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

硬规则：

- `review.md` 是唯一正式 Review Artifact；不得把 `review.json` 作为最终产物。
- Reviewer/Orchestrator 不得使用 arbitrary write_file 自由生成 Markdown。
- `PASSED / FAILED / MANUAL_ACTION_REQUIRED` 都必须生成报告。
- 无代码变更也必须生成 `Result: PASSED`、`Changed Files: 0`、`Findings: 0` 的报告。
- Coverage PARTIAL 或 Runtime Contract validation error 必须先把 unresolved、missing reviewed files、Runtime validation error 写入结构化 transport，再生成 `MANUAL_ACTION_REQUIRED` 报告，然后 STOP。
- Runtime 报告生成失败时不得继续后续流程，统一 `MANUAL_ACTION_REQUIRED`。
- OpenCode 最终摘要必须同时展示 Review 结果和 `review.md` 路径。

## `harness review`

```text
1. 解析并校验 effectiveBaseRef
2. Reviewer.analyze-change：完整 Change Set + 全 changedFiles + 确定性调用链
3. Tool Runtime: validate_contract(ChangeAnalysis JSON)
   - JSON Schema 校验
   - 机器 Coverage 校验
4. 输出 Review Scope
5. 输出 Review Coverage
6. 无变化 → 形成 Result=PASSED 的 structured Review result → Controlled Runtime 生成 review.md → 展示 report path → STOP
7. Runtime 校验失败 / coverage!=COMPLETE → 形成 Result=MANUAL_ACTION_REQUIRED（包含 unresolved/missing/runtimeErrors）→ Controlled Runtime 生成 review.md → 展示 report path → STOP
8. Reviewer.review-code
9. findings 为空 → Result=PASSED；findings 非空 → Result=FAILED
10. 形成 structured Review result → Controlled Runtime 生成 review.md
11. 输出 Review Findings + 最终结果 + report path
```

用户可见顺序固定为：

```text
Review Scope
→ Review Coverage
→ Review Findings（如允许执行）
→ 结果 + .code-harness/runs/<runId>/review.md
```

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
3. changed：复用 Review Change Set + ChangeAnalysis 的 affectedControllers。
4. changed target selection：
   - 0 → NO_API_TARGET → STOP
   - 1 → AUTO_SINGLE → 继续
   - 2+ → WAITING_API_SELECTION；native multi-select 优先，否则 numbered fallback（1,3 / ALL）
   - 多 target 不得默认 ALL；空选择/取消 → STOP
5. API Doc Agent 调用 generate-api-doc，分析深度固定：
   Controller → Request DTO → Response DTO/VO → Enum → Validation → Direct Service Method（最多一层）→ STOP。
6. 禁止进入 Repository / Mapper / DAO / DB / MQ / Redis / RPC Server；不得读取真实数据库。
7. 结构化 apiDoc 必须满足 api-doc.schema.json。CONFIRMED/INFERRED 必须带 evidence；无可靠证据优先空数组，禁止编造。
8. Orchestrator/Agent 只把 transport 写入 `.code-harness/runs/<runId>/requests/api-doc.json`，不得自由生成最终 Markdown。
9. 调用 Controlled Runtime：`report api-doc --input .code-harness/runs/<runId>/requests/api-doc.json`。
10. Runtime 再执行 Draft 2020-12 Schema 校验，通过后 deterministic renderer 生成 `.code-harness/runs/<runId>/api-doc.md`，并删除 transport。
11. 最终摘要仅展示 target、endpoint 数和 Api Doc Report path。
```

请求参数只识别：`@RequestBody / @RequestParam / @PathVariable / 明确业务 @RequestHeader`。Validation 只从源码提取 `@NotNull/@NotBlank/@NotEmpty/@Size/@Length/@Min/@Max/@DecimalMin/@DecimalMax/@Pattern/@Valid`。DTO 递归最大深度 3 并做 cycle detection。Enum 未解析时只保留类型，不得编造值。Error code 只允许 Controller/Direct Service Method 中显式 BizException/ErrorCode/assert evidence。

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

## `harness test`

```text
0. 要求 initialization.status=READY，且当前宿主具备文件读写、Maven 执行、超时控制
1. 与 harness review 完全相同地解析 Change Set 并执行 analyze-change
2. Tool Runtime: validate_contract(ChangeAnalysis JSON)，执行 JSON Schema + 机器 Coverage 校验
3. 输出 Review Scope + Review Coverage
4. 无代码变更 → 先生成 Result=PASSED、Changed Files=0、Findings=0 的 review.md，再 `NO_TEST_TARGET` → STOP
5. Runtime 校验失败 / coverage != COMPLETE → 先生成 Result=MANUAL_ACTION_REQUIRED 的 review.md，再 STOP；不得设计测试
6. Reviewer.review-code
7. findings 为空 → Result=PASSED；findings 非空 → Result=FAILED；在任何 Test Target Selection 之前生成并确认 `.code-harness/runs/<runId>/review.md`
8. affectedControllers=0 → `NO_TEST_TARGET`，报告后 STOP
9. affectedControllers=1 → `AUTO_SINGLE`；持久化并机器校验 TestTargetSelection，不打断用户
10. affectedControllers>=2 → `WAITING_TEST_SELECTION`；宿主结构化多选优先，否则编号 fallback；取消 → `CANCELLED` STOP
11. 只有 `TEST_TARGETS_SELECTED` 且 selection artifact 机器验证通过，才把 **selected affectedControllers** 交给 Integration Test Agent
12. Integration Test Agent 只对 selected targets 做 Existing Test Coverage Analysis
13. 每个 selected target 独立采用 REUSE_EXISTING / EXTEND_EXISTING / CREATE_NEW；未选择 target 不得生成计划
14. REUSE_EXISTING 直接执行；其余输出 Test Plan，仍需精确 `批准 <planId>` 后才写测试
15. Integration Test Agent 返回 method/scenario 级 provenance；Orchestrator 形成 `TestExecutionTarget(testClass,testMethods[],selector,controllerId,origin,planId?)`
16. selected-only execution gate：整类 selector 仅允许该 class 本次相关 methods 全部属于 selected targets；混合 selected+unselected class 必须收窄到 selected method selector；无法安全表达 method selector → `MANUAL_ACTION_REQUIRED`，不得整类执行
17. Runtime Debugger 独占测试执行、日志和 Diagnosis；Surefire `failedTests.testClass + testMethod` 必须回查具体 TestExecutionTarget 判定 method-level origin
18. 新生成/修改测试若 TEST_ERROR：仅失败方法 `origin=GENERATED_BY_PLAN` 且能唯一追溯 approved planId 时可自动修复，按 `planId+testClass+testMethod` 最多 2 轮；同 class 历史 Existing Test method 永不自动修改
19. Existing Test 失败禁止自动修改；PRODUCTION_CODE_ERROR 可自动生成 Fix Plan，但 fixPlanId 未批准前不得修改生产代码
```

## `harness upgrade`

不要求 READY，但要求文件读取与受控写入能力。调用 `upgrade-harness`；配置兼容迁移只能由 Tool Runtime 的 registered migration 执行。Framework Managed 必须按新版完整集合 replace，stale Framework 文件必须删除并进入 `removedFiles`。成功时清理 stage/backup/升级源目录；失败时回滚旧 Harness、保留升级源目录并清理临时 stage/backup。Windows 下运行中的 `codea-harness-tools.exe` 禁止原地覆盖，只允许 staged/temp rename 方式替换。状态按 `UPGRADED / ALREADY_UP_TO_DATE / MANUAL_ACTION_REQUIRED / UPGRADE_FAILED` 原样映射。

## 其他意图（保持 1.1.0 语义）

- `harness init`：Project Adapter 识别 Maven/模块/Profile/测试规范/baseRef，生成 `harness.yaml`/`project.md`；不确定项保持 `NEEDS_CONFIRMATION`。
- `harness debug-service`：Runtime Debugger 启动服务，等待人工触发请求，采集日志，诊断，停止本次 processGroup。
- `harness fix finding:<id>` / `fix diagnosis:<runId>`：Fix Agent 先生成最小 Fix Plan；精确 `fixPlanId` 批准后才能 `apply_approved_patch`；验证由 Runtime Debugger 完成。
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
- 只有匹配到 `GENERATED_BY_PLAN` 的具体失败 method 才进入自动修复轮次；对应 approved `planId` 必须存在。
- `REUSED_EXISTING` method 永不自动修改，即使它与 GENERATED_BY_PLAN method 位于同一个 class。
- `testMethod=null`、provenance 冲突或无法唯一匹配时默认走安全路径：不得自动 repair，进入测试修改计划 / `MANUAL_ACTION_REQUIRED`。
- 自动 repair 计数键为 `planId + testClass + testMethod`，最多 2 轮；达到上限 → `MANUAL_TEST_REPAIR_REQUIRED`。

## 统一结果

```text
结果：PASSED | FAILED | WAITING_APPROVAL | MANUAL_ACTION_REQUIRED

完成：
- 评审 N 个文件
- 生成 M 个测试类
- 执行 K 个场景

发现：
- X 个生产代码问题
- Y 个测试代码问题

下一步：
- 请批准 <planId> | <fixPlanId>
- 或：所有测试通过，无需进一步操作
```

对于包含 Review 阶段的 `harness review` / `harness test`，统一结果中还必须显示：

```text
Review Report:
.code-harness/runs/<runId>/review.md
```

对于 `harness api-doc`，统一结果必须显示：

```text
API Doc Report:
.code-harness/runs/<runId>/api-doc.md
```

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

`oldTestA` 失败永不自动改；只有 `newMissingTest` 失败且 Diagnosis=`REPAIR_TEST` 才进入最多 2 轮 repair。
6. Synthetic Golden Flow 必须满足：

```text
Affected Controllers: Order, Payment, User
Selection: Order + Payment

Order -> REUSE_EXISTING -> no approval/no write -> selected-only TestExecutionTarget
Payment -> EXTEND_EXISTING -> exact 批准 <paymentPlanId> -> modify only MISSING -> method-level provenance
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
8. `REUSE_EXISTING -> run/no approval/no modification`、`EXTEND_EXISTING -> only MISSING + exact planId approval`、`CREATE_NEW -> exact planId approval`、historical Existing Test method never auto-edit、GENERATED_BY_PLAN method repair max 2 rounds 全部保持不变。

## 禁止行为

- 不得跳过 Review Coverage / Runtime Contract 校验 / Review Report Persistence / Test Target Selection / 审批门禁。
- `harness api-doc` 不得跳过 api-doc Schema Validation / API Target Selection / Controlled Runtime Renderer。
- 不得让 Reviewer/Orchestrator/API Doc Agent 自由写 `review.md` 或 `api-doc.md`；只能调用 Controlled Runtime Renderer。
- 不得超过 2 轮自动测试修复。
- 不得直接执行任意 Shell。
- 不得自动 commit/push/PR。
