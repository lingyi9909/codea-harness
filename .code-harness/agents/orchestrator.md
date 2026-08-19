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
3. Agent 填写的 `reviewCoverage.status=COMPLETE` 不能直接作为通过依据。

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

## `harness test`

```text
0. 要求 initialization.status=READY，且当前宿主具备文件读写、Maven 执行、超时控制
1. 与 harness review 完全相同地解析 Change Set 并执行 analyze-change
2. Tool Runtime: validate_contract(ChangeAnalysis JSON)，执行 JSON Schema + 机器 Coverage 校验
3. 输出 Review Scope + Review Coverage
4. Runtime 校验失败 / coverage != COMPLETE → MANUAL_ACTION_REQUIRED，STOP；不得设计测试
5. Reviewer.review-code
6. 如果没有受影响 Controller → 报告后 STOP
7. Integration Test Agent 做 Existing Test Coverage Analysis
8. 每个 target 独立采用 REUSE_EXISTING / EXTEND_EXISTING / CREATE_NEW
9. REUSE_EXISTING 直接执行；其余输出 Test Plan，精确 planId 审批后才写测试
10. Runtime Debugger 独占测试执行、日志和 Diagnosis
11. 新生成/修改测试若 TEST_ERROR：仅 GENERATED_BY_PLAN 可自动修复，最多 2 轮
12. Existing Test 失败禁止自动修改；PRODUCTION_CODE_ERROR 可自动生成 Fix Plan，但 fixPlanId 未批准前不得修改生产代码
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

## 禁止行为

- 不得跳过 Review Coverage / Runtime Contract 校验 / 审批门禁。
- 不得超过 2 轮自动测试修复。
- 不得直接执行任意 Shell。
- 不得自动 commit/push/PR。
