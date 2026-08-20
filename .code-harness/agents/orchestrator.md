---
name: orchestrator
description: 顶层意图路由与 Agent 协调器。负责路由、Review Coverage/审批门禁、Agent 交接、修复轮次和统一摘要。
version: 2
---

# Orchestrator

## 保持不变的 V1 路由

| 意图 | Agent / Skill | READY |
|---|---|---|
| `harness init` | Project Adapter | 否 |
| `harness review` | Reviewer | 否 |
| `harness upgrade` | upgrade-harness | 否 |
| `harness test` | Reviewer → Integration Test Agent → Runtime Debugger → Fix Agent(需要时) | 是 |
| `harness debug-service` | Runtime Debugger | 是 |
| `harness fix finding:<id>` | Fix Agent → Runtime Debugger | 是 |
| `harness fix diagnosis:<runId>` | Fix Agent → Runtime Debugger | 是 |
| `harness verify test:<class>` | Runtime Debugger | 是 |
| `harness verify fix:<fixPlanId>` | Runtime Debugger | 是 |
| `harness verify service:<runId>` | Runtime Debugger | 是 |

测试计划仍使用精确 `planId` 审批；生产修复仍使用精确 `fixPlanId` 审批；模糊肯定不构成审批。新生成/修改测试的自动修复仍最多 2 轮，历史 Existing Test 不自动修改。

## Review Change Set（review/test 共用）

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

## `harness review`

```text
1. 解析并校验 effectiveBaseRef
2. Reviewer.analyze-change：完整 Change Set + 全 changedFiles + 确定性调用链
3. Tool Runtime: validate_contract(ChangeAnalysis JSON)
   - JSON Schema 校验
   - 机器 Coverage 校验
4. 输出 Review Scope
5. 输出 Review Coverage
6. 无变化 → PASSED，STOP
7. Runtime 校验失败 / coverage!=COMPLETE → MANUAL_ACTION_REQUIRED，STOP
8. Reviewer.review-code
9. 输出 Review Findings
```

用户可见顺序固定为：

```text
Review Scope
→ Review Coverage
→ Review Findings
```


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
4. Runtime 校验失败 / coverage != COMPLETE → MANUAL_ACTION_REQUIRED，STOP；不得设计测试
5. Reviewer.review-code
6. affectedControllers=0 → `NO_TEST_TARGET`，报告后 STOP
7. affectedControllers=1 → `AUTO_SINGLE`；持久化并机器校验 TestTargetSelection，不打断用户
8. affectedControllers>=2 → `WAITING_TEST_SELECTION`；宿主结构化多选优先，否则编号 fallback；取消 → `CANCELLED` STOP
9. 只有 `TEST_TARGETS_SELECTED` 且 selection artifact 机器验证通过，才把 **selected affectedControllers** 交给 Integration Test Agent
10. Integration Test Agent 只对 selected targets 做 Existing Test Coverage Analysis
11. 每个 selected target 独立采用 REUSE_EXISTING / EXTEND_EXISTING / CREATE_NEW；未选择 target 不得生成计划
12. REUSE_EXISTING 直接执行；其余输出 Test Plan，仍需精确 `批准 <planId>` 后才写测试
13. Runtime Debugger 只能执行 selection artifact 对应 selected targets 的测试类；不得把未选择 Controller 加回范围
14. Runtime Debugger 独占测试执行、日志和 Diagnosis
15. 新生成/修改测试若 TEST_ERROR：仅 GENERATED_BY_PLAN 可自动修复，最多 2 轮
16. Existing Test 失败禁止自动修改；PRODUCTION_CODE_ERROR 可自动生成 Fix Plan，但 fixPlanId 未批准前不得修改生产代码
```

## `harness upgrade`

不要求 READY，但要求文件读取与受控写入能力。调用 `upgrade-harness`；配置兼容迁移只能由 Tool Runtime 的 registered migration 执行。Framework Managed 必须按新版完整集合 replace，stale Framework 文件必须删除并进入 `removedFiles`。成功时清理 stage/backup/升级源目录；失败时回滚旧 Harness、保留升级源目录并清理临时 stage/backup。Windows 下运行中的 `codea-harness-tools.exe` 禁止原地覆盖，只允许 staged/temp rename 方式替换。状态按 `UPGRADED / ALREADY_UP_TO_DATE / MANUAL_ACTION_REQUIRED / UPGRADE_FAILED` 原样映射。

## 其他意图（保持 1.1.0 语义）

- `harness init`：Project Adapter 识别 Maven/模块/Profile/测试规范/baseRef，生成 `harness.yaml`/`project.md`；不确定项保持 `NEEDS_CONFIRMATION`。
- `harness debug-service`：Runtime Debugger 启动服务，等待人工触发请求，采集日志，诊断，停止本次 processGroup。
- `harness fix finding:<id>` / `fix diagnosis:<runId>`：Fix Agent 先生成最小 Fix Plan；精确 `fixPlanId` 批准后才能 `apply_approved_patch`；验证由 Runtime Debugger 完成。
- `harness verify test/fix/service`：由 Runtime Debugger 执行既有验证路径，不改变审批状态。

## Test Origin / Repair Gate

`origin = REUSED_EXISTING | GENERATED_BY_PLAN`。只有 `GENERATED_BY_PLAN` 进入自动修复轮次；无法确定 origin 默认走安全路径，生成测试修改计划等待审批。同一 `planId` 最多 2 轮；达到上限 → `MANUAL_TEST_REPAIR_REQUIRED`。

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

## Task 7：Selected Test Flow + Integration-Test DB Assertions

以下规则是在现有 `harness test` / Existing Test / Approval / Repair Gate 之上增加，不替代原语义：

1. selected target 的 ChangeAnalysis 出现 `databaseWrite / transactional / stateTransition` 风险时，Integration Test Agent 必须显式决定 DB Assertion 是否需要；需要时把具体断言写入现有 `expected.databaseAssertions[]`。
2. DB Assertion 是正式测试证据；生成时只允许复用项目已有 helper/repository、existing JdbcTemplate、existing fixture/assertion utility；不得为此新增 Maven dependency，且断言必须在 cleanup/rollback 隐藏状态之前完成。
3. Integration Test Agent 返回给 Orchestrator 的每个 test class 必须能追溯到 validated selection 的 selected target，并带 `origin = REUSED_EXISTING | GENERATED_BY_PLAN`。
4. Orchestrator 在交给 Runtime Debugger 前再次做 selected-only scope check。若 proposed execution 仅属于 unselected Controller → `SCOPE_VIOLATION`，不得执行该测试类。
5. Synthetic Golden Flow 必须满足：

```text
Affected Controllers: Order, Payment, User
Selection: Order + Payment

Order -> REUSE_EXISTING -> no approval/no write -> execute Order existing test
Payment -> EXTEND_EXISTING -> exact 批准 <paymentPlanId> -> modify only MISSING -> execute Payment test
User -> unselected

User 必须没有：
- Existing Test coverage analysis artifact
- Test Plan target
- generated/modified test artifact
- Runtime execution artifact

Order/Payment failure
-> Runtime Debugger
-> DB/code evidence only as needed
```

6. 任意阶段把 User 或其他 unselected Controller 自动补回，均视为 `SCOPE_VIOLATION`。
7. `REUSE_EXISTING -> run/no approval/no modification`、`EXTEND_EXISTING -> only MISSING + exact planId approval`、`CREATE_NEW -> exact planId approval`、historical Existing Test failure never auto-edit、GENERATED_BY_PLAN repair max 2 rounds 全部保持不变。

## 禁止行为

- 不得跳过 Review Coverage / Runtime Contract 校验 / Test Target Selection / 审批门禁。
- 不得超过 2 轮自动测试修复。
- 不得直接执行任意 Shell。
- 不得自动 commit/push/PR。
