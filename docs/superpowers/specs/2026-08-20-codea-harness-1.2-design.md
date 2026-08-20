# Codea Harness 1.2 Design

> Status: APPROVED DESIGN
>
> Target version: **1.2.0**
>
> Baseline: Codea Harness 1.1.1 (`6e7e613b68bd748212d4918f320d398acad7e60b`)

## 1. 目标

Codea Harness 1.2 在不推翻 1.1.1 架构和安全边界的前提下，把 `harness test` 从“测试设计 + 执行 + 基础失败分类”升级为一条适合真实 Java/Spring Boot/Maven B 端项目的测试与诊断链。

本版本只新增三项核心能力：

1. **Test Target Selection**：当一个变更影响多个 Controller 时，先让用户通过宿主 UI 多选本次要测试的 Controller，再进入测试设计和执行。
2. **Database Evidence**：测试可以包含数据库断言；Runtime Debugger 可以自主探索测试库 Schema、自主生成并执行复杂只读 SQL，作为故障诊断证据。
3. **Failure Code Navigation**：Runtime Debugger 从 Surefire、日志、Stack Trace、DB Evidence 中提取 suspect symbols，复用 1.1.1 的 Code Navigation Contract 读取相关实现和调用链，在证据充分后才产出 Diagnosis。

## 2. 非目标

1. 不开发 Codea 自有 TUI/GUI。
2. 不把 `harness xxx` 变成新的用户 CLI；用户仍然对 Codex/OpenCode 等工程 Agent 表达自然语言意图。
3. 不改变 1.1.1 Review Change Set、Review Coverage、Approval、Existing Test 保护和 Upgrade replace 语义。
4. 不允许 Agent 直接执行任意 Shell。
5. 不允许 Agent 修改数据库；数据库能力只读。
6. 1.2.0 第一版数据库 Runtime **只支持 MySQL**。架构保留 Adapter 边界，后续可扩展 Oracle/PostgreSQL 等。
7. 不在 1.2.0 引入 MCP Toolbox、Vault、密钥中心或 connectionRef；数据库连接先使用本地配置文件。
8. 不新增自动化 HTTP 请求能力；`harness debug-service` 仍可等待人工/前端请求。

## 3. 总体架构

```text
User
  │
  ▼
Host Agent (OpenCode / Codex / other)
  │
  ▼
Orchestrator
  │
  ├─ Reviewer
  │    └─ ChangeAnalysis + affectedControllers
  │
  ├─ Test Target Selection
  │    ├─ native host selection UI (preferred)
  │    └─ numbered fallback
  │
  ├─ Integration Test Agent
  │    ├─ REUSE_EXISTING
  │    ├─ EXTEND_EXISTING
  │    └─ CREATE_NEW
  │
  └─ Runtime Debugger
       ├─ Maven/Surefire Evidence
       ├─ Application Log Evidence
       ├─ Database Evidence (optional)
       ├─ Code Navigation Evidence
       └─ Diagnosis
              │
              ├─ Test Repair
              ├─ Fix Plan
              ├─ Environment Report
              └─ Unknown/Manual
```

现有职责继续保持：

- Integration Test Agent：设计/生成/修复测试，不执行测试。
- Runtime Debugger：独占测试执行、日志采集和故障诊断。
- Fix Agent：只在 Diagnosis 已确认生产代码问题后生成 Fix Plan，审批后修改代码。
- Project Adapter：只做项目适配，不连接数据库、不执行测试。

---

# 4. Test Target Selection

## 4.1 触发时机

`harness test` 必须先完成 1.1.1 的 Review/ChangeAnalysis。只有 `reviewCoverage.status = COMPLETE` 才能进入 Target Selection。

顺序锁定为：

```text
harness test
→ Change Set
→ Review Coverage
→ affectedControllers
→ Test Target Selection
→ Existing Test Coverage
→ Test Plan / Approval
→ Test Execution
```

**选择发生在测试设计之前。** 未选择的 Controller 不做 Existing Test 分析、不生成 Test Plan、不执行测试。

## 4.2 选择规则

```text
affectedControllers = 0
→ STOP: NO_TEST_TARGET

affectedControllers = 1
→ AUTO_SELECT_SINGLE
→ 不打断用户

affectedControllers >= 2
→ WAITING_TEST_SELECTION
→ 必须用户选择后继续
```

不得默认“全部测试”。

## 4.3 ChangeAnalysis 扩展

`affectedControllers[]` 在 1.2 新增：

```json
{
  "controller": "OrderController",
  "endpoints": ["POST /order/approve"],
  "impactType": "DIRECT_CHANGE",
  "sourceSymbols": ["OrderController.approve"]
}
```

`impactType` 枚举：

- `DIRECT_CHANGE`
- `AFFECTED_BY_CALL_CHAIN`

`sourceSymbols` 说明为什么该 Controller 被识别为受影响目标。

## 4.4 Host Interaction Contract

Harness 定义宿主无关的逻辑 Contract：

```text
request_test_target_selection(
  selectionId,
  options[],
  multiple=true,
  shortcuts=[ALL, DIRECT_ONLY]
)
```

每个 option 至少包含：

```json
{
  "id": "controller:OrderController",
  "label": "OrderController",
  "endpoints": ["POST /order/approve"],
  "impactType": "DIRECT_CHANGE",
  "recommended": true
}
```

宿主适配规则：

1. 宿主支持结构化选择 UI（如 OpenCode question/selection 能力）时，优先显示可点击多选。
2. 宿主不支持结构化选择时，降级为**编号选择**，例如 `1,3` 或 `ALL`；不要求用户手敲 Controller 名称。
3. 用户取消选择 → `CANCELLED`，不得继续测试。

快捷项：

- `ALL`：全选。
- `DIRECT_ONLY`：只选择 `impactType = DIRECT_CHANGE`。

## 4.5 Selection Artifact

新增：

```text
.code-harness/contracts/test-target-selection.schema.json
```

每次选择结果持久化：

```text
.code-harness/runs/<runId>/test-target-selection.json
```

核心字段：

```json
{
  "selectionId": "sel-...",
  "status": "SELECTED",
  "mode": "USER_MULTI",
  "selectedControllerIds": ["controller:OrderController"],
  "availableControllerIds": [
    "controller:OrderController",
    "controller:PaymentController"
  ]
}
```

`mode`：

- `AUTO_SINGLE`
- `USER_MULTI`
- `USER_ALL`
- `USER_DIRECT_ONLY`
- `FALLBACK_NUMBERED`

## 4.6 Selection 与 Approval 分离

Controller Selection 只回答“测哪个”，不代表允许修改代码。

```text
Selection
→ 测试范围

批准 <planId>
→ 允许新增/修改测试

批准 <fixPlanId>
→ 允许修改生产代码
```

三种动作不得互相替代。

---

# 5. Database Evidence

## 5.1 第一版配置模型

新增两个文件：

```text
.code-harness/database.template.yaml
.code-harness/database.yaml
```

语义：

- `database.template.yaml`：Framework Managed，随 Harness 分发和升级。
- `database.yaml`：Project State，本机真实连接配置，**不得提交 Git，升级必须保留**。

`.code-harness/.gitignore` 必须包含：

```text
database.yaml
```

`database.yaml` 不存在或 `enabled: false` 时，数据库能力整体关闭；其他 Harness 能力不受影响。

## 5.2 database.yaml V1

```yaml
version: 1
enabled: true
environment: TEST
dialect: mysql

connection:
  host: 10.10.10.20
  port: 3306
  database: order_test
  username: codea_readonly
  password: change-me
  charset: utf8mb4

safety:
  allowedSchemas:
    - order_test
  maxRows: 100
  timeoutSeconds: 10
  maxQueriesPerDiagnosis: 10
  allowSchemaDiscovery: true
  allowReadonlySql: true
```

约束：

- `environment` 只允许 `TEST` / `LOCAL`。
- `dialect` 1.2.0 只允许 `mysql`。
- `maxRows` 默认 100，最大 1000。
- `timeoutSeconds` 默认 10，最大 30。
- `maxQueriesPerDiagnosis` 默认 10，最大 20。
- `allowedSchemas` 至少 1 个，Runtime 不得访问范围外 Schema。

新增：

```text
.code-harness/contracts/database-config.schema.json
```

Runtime 在任何 DB 操作前必须使用成熟 YAML parser + JSON Schema Validator 校验配置。

## 5.3 凭据边界

第一版允许账号密码存在本地 `database.yaml`，但必须满足：

1. 文件被 `.gitignore` 忽略。
2. Agent 不得通过 `read_code` / `read_project_file` 读取 `database.yaml` 内容。
3. Runtime 读取配置后不得把 password 输出到 stdout/stderr、run artifact、Diagnosis。
4. 账号应使用数据库层物理只读账号。
5. Runtime 查询仍必须经过 AST Gate，不能仅依赖账号权限。

后续版本可把 connection 替换为 `connectionRef` / MCP / Secret Store，但不属于 1.2.0。

## 5.4 Database Tool Contract

新增受控工具：

```text
db_ping(runId) -> DatabaseConnectionResult

db_list_tables(schema, runId) -> DatabaseTableList

db_describe_table(schema, table, runId) -> DatabaseTableDescription

db_query_readonly(sql, params, runId, purpose) -> DatabaseQueryResult
```

其中：

- `db_list_tables` / `db_describe_table`：用于 Schema 探索。
- `db_query_readonly`：允许 Agent 自主构造复杂 SELECT / JOIN / CTE 进行故障分析。

Agent 不直接调用 mysql.exe，也不能绕过 Runtime 执行 SQL。

## 5.5 Runtime 固定入口

在 `codea-harness-tools.exe` 新增：

```text
codea-harness-tools.exe db ping --run-id <id>
codea-harness-tools.exe db list-tables --schema <schema> --run-id <id>
codea-harness-tools.exe db describe-table --schema <schema> --table <table> --run-id <id>
codea-harness-tools.exe db query --input <under .code-harness/runs/...>
```

`db query` 的 input JSON：

```json
{
  "runId": "run-...",
  "queryId": "dbq-001",
  "purpose": "verify order state after approve failure",
  "sql": "SELECT status, approved_by FROM order_info WHERE order_id = ?",
  "params": [10001]
}
```

SQL 不通过 Shell 参数拼接；Runtime 只读取 `.code-harness/runs/**` 下的结构化请求文件。

## 5.6 SQL Safety Gate

Runtime 必须在连接数据库前或执行 SQL 前做以下确定性校验：

1. 使用成熟 MySQL AST Parser 解析 SQL；禁止自行用正则判断只读。
2. 恰好一个 statement。
3. `db_query_readonly` 只允许：
   - `SELECT`
   - `WITH ... SELECT`
4. 拒绝所有写操作、DDL、管理语句和存储过程调用，包括但不限于：
   - INSERT / UPDATE / DELETE / REPLACE / MERGE
   - CREATE / ALTER / DROP / TRUNCATE
   - GRANT / REVOKE
   - CALL
   - SET / USE
   - LOAD DATA
5. 拒绝 `SELECT ... INTO OUTFILE` / `INTO DUMPFILE`。
6. 拒绝 `FOR UPDATE`、显式锁及其他写锁语义。
7. AST 提取所有表/Schema；任何显式 Schema 不在 `allowedSchemas` 时拒绝。
8. 数据条件值优先使用 `?` + params；不得把用户输入拼成 Shell 或命令。
9. Runtime 使用只读事务/只读连接语义执行查询。
10. 强制 context timeout。
11. 即使 SQL 没有 LIMIT，Runtime 结果读取也最多 `maxRows`；超过时 `truncated=true`。
12. 单次 Diagnosis 查询次数达到 `maxQueriesPerDiagnosis` 后必须停止自动查询。

Go Runtime 的 DB 实现边界：

```text
internal/dbconfig   # load + validate database.yaml
internal/dbguard    # AST validation + schema/statement restrictions
internal/dbmysql    # MySQL connection/query only
internal/dbevidence # sanitize + write evidence artifact
```

1.2.0 建议依赖：

- MySQL driver：`github.com/go-sql-driver/mysql`（选择与 Go 1.23 基线兼容的固定版本）。
- AST parser：使用成熟 MySQL AST Parser，并在合入前通过 Go 1.23 / Windows x64 build gate；Parser 封装在 `internal/dbguard`，不得泄露到 Agent/Skill Contract。

## 5.7 DB Evidence Artifact

每次成功或失败的 Runtime 查询都写入：

```text
.code-harness/runs/<runId>/evidence/db/<queryId>.json
```

新增：

```text
.code-harness/contracts/database-evidence.schema.json
```

核心结构：

```json
{
  "queryId": "dbq-001",
  "runId": "run-...",
  "purpose": "verify order state",
  "schema": "order_test",
  "statementType": "SELECT",
  "columns": ["status", "approved_by"],
  "rows": [
    {"status": "PENDING", "approved_by": null}
  ],
  "rowCount": 1,
  "truncated": false,
  "durationMs": 31
}
```

不得保存 password。对于列名命中 password/token/secret/accessKey/privateKey 等敏感模式的结果值，写 Evidence 前必须脱敏。

## 5.8 测试内数据库断言

`test-plan.schema.json` 已有 `expected.databaseAssertions[]`，1.2 保留并强化语义：

- 对状态流转、事务、审计记录、关联表写入等业务，Integration Test Agent 应优先设计 DB Assertion。
- DB Assertion 应写进测试代码，在测试事务/清理发生前完成验证。
- 生成方式优先沿用项目已有 Repository/JdbcTemplate/Test Fixture 约定。
- 不为了加 DB Assertion 引入新的项目依赖。
- Runtime DB Query 是诊断/补充证据，不替代测试内正式断言。

---

# 6. Failure Code Navigation

## 6.1 当前问题

1.1.1 `analyze-failure` 能读取 Surefire / stdout / stderr / application log 并分类，但没有硬性要求使用 Code Navigation 读取相关实现，因此可能停留在“日志分类”，没有形成代码级证据链。

1.2 要把 Diagnosis 改成 Evidence-backed Diagnosis。

## 6.2 Runtime Debugger 新能力

`analyze-failure` 新增允许工具：

```text
read_test_report
read_service_logs
find_symbol
find_references
find_implementations
read_code

db_ping                 # database enabled 时
db_list_tables          # database enabled 时
db_describe_table       # database enabled 时
db_query_readonly       # database enabled 时
```

Runtime Debugger 仍然不得修改任何文件。

## 6.3 诊断流程

```text
Failure
→ collect Surefire/stdout/stderr/logs
→ extract Failure Signals
→ optional DB Evidence
→ extract suspectSymbols
→ Code Navigation
→ read implementation/call chain
→ evidence sufficient?
     ├─ yes → Diagnosis
     └─ no  → bounded additional evidence
→ still insufficient → UNKNOWN
```

Failure Signals 至少包括可获得的：

- exception type
- exception message
- stack trace
- testClass/testMethod
- source file + line
- HTTP/RPC error
- SQL exception
- traceId/requestId
- failed state/data assertion

## 6.4 Code Navigation 规则

1. Stack Trace 出现项目内 `file:line` 时必须 `read_code` 对应实现，不能只凭日志宣告根因。
2. 接口符号必须使用 `find_implementations` 定位实现。
3. 怀疑上游调用者时使用 `find_references`。
4. 怀疑下游 Service/Repository 时继续 `find_symbol` / `read_code`。
5. Debug Navigation **不限制为 changed files**；根因可能位于未修改的项目内部代码。
6. 仍然禁止无界扫描整个仓库。
7. 外部边界停止规则继续为：Repository/Mapper 后的 DB 边界、RPC、MQ、Cache Client、第三方 SDK、JDK/Spring。
8. 外部实现无法读取时记录 `externalDependencies`，不得猜实现。

第一版固定调查预算：

```text
max navigation hops per diagnosis: 6
max source/test files read per diagnosis: 30
max database queries per diagnosis: database.yaml safety.maxQueriesPerDiagnosis
```

达到预算仍无法确认 → `UNKNOWN / STOP_UNKNOWN`，附上已获得证据。

## 6.5 Diagnosis Contract 扩展

扩展 `.code-harness/contracts/diagnosis.schema.json`：

新增可选字段：

```json
{
  "suspectSymbols": [
    "OrderServiceImpl.approve"
  ],
  "codeEvidence": [
    {
      "path": "src/main/java/.../OrderServiceImpl.java",
      "symbol": "OrderServiceImpl.approve",
      "lineStart": 178,
      "lineEnd": 190,
      "reason": "stack trace target"
    }
  ],
  "databaseEvidence": [
    "dbq-002",
    "dbq-003"
  ],
  "externalDependencies": [
    "PaymentRpcClient"
  ]
}
```

已有字段和分类继续保留：

- TEST_COMPILE_ERROR
- TEST_CODE_ERROR
- TEST_CONTEXT_ERROR
- TEST_DATA_OR_ENVIRONMENT_ERROR
- SERVICE_START_ERROR
- PRODUCTION_CODE_ERROR
- UNKNOWN

Diagnosis 的 `rootCause` 必须能引用 Evidence；不能出现“证据不足但声明确定根因”。

## 6.6 证据停止原则

不是每次失败都必须查数据库或展开 6 层调用链：

```text
证据足够确认分类和根因
→ STOP INVESTIGATION

证据不足
→ 继续有界 Navigation / DB Query

预算用尽仍不足
→ UNKNOWN
```

目标是证据充分，不是最大化工具调用。

---

# 7. harness test 1.2 完整状态机

```text
START
│
├─ Review Change Set
│
├─ Review Coverage
│    └─ PARTIAL → MANUAL_ACTION_REQUIRED
│
├─ affectedControllers
│    ├─ 0 → NO_TEST_TARGET → DONE
│    ├─ 1 → AUTO_SELECT_SINGLE
│    └─ >1 → WAITING_TEST_SELECTION
│               └─ user selection → TEST_TARGETS_SELECTED
│
├─ Integration Test Agent
│    ├─ REUSE_EXISTING
│    ├─ EXTEND_EXISTING
│    └─ CREATE_NEW
│
├─ needs code write?
│    ├─ yes → WAITING_APPROVAL (`批准 <planId>`)
│    └─ no
│
├─ Runtime Debugger executes selected tests only
│
├─ PASS
│    └─ DONE
│
└─ FAIL
     ├─ collect test/log evidence
     ├─ optional DB evidence
     ├─ failure code navigation
     └─ Diagnosis
          ├─ REPAIR_TEST
          │    └─ only GENERATED_BY_PLAN, max 2 rounds
          ├─ GENERATE_FIX_PLAN
          │    └─ WAITING_FIX_APPROVAL (`批准 <fixPlanId>`)
          ├─ RETRY_TEST
          ├─ REPORT_ENVIRONMENT
          └─ STOP_UNKNOWN
```

硬规则：Runtime Debugger **只能执行 selection artifact 中已选 Controller 对应的测试目标**；不得偷偷把未选择 Controller 加回执行范围。

---

# 8. Agent / Skill 改动

## Orchestrator

修改 `.code-harness/agents/orchestrator.md`：

- 新增 `WAITING_TEST_SELECTION` / `TEST_TARGETS_SELECTED`。
- 多 Controller 必须 Selection Gate。
- Host UI 优先、编号 fallback。
- Selection 与 Plan Approval/Fix Approval 完全分离。
- 只把 selected targets 交给 Integration Test Agent。
- Diagnosis 前允许 Runtime Debugger 做 DB Evidence + Code Navigation。

## Reviewer

修改 `analyze-change` Skill + `change-analysis.schema.json`：

- affectedControllers 增加 `impactType` / `sourceSymbols`。
- 确保间接受影响 Controller 能解释来源。

## Integration Test Agent

修改：

- 输入从“全部 affectedControllers”改成“selected affectedControllers”。
- 未选择 target 禁止设计测试。
- 对 `databaseWrite` / `transactional` / `stateTransition` 风险优先设计 `databaseAssertions`。
- 不执行数据库数据查询；如需要理解表结构，可使用项目代码/映射信息，第一版不把任意 DB Query 权限交给 Integration Test Agent。

## Runtime Debugger

修改：

- 新增 DB Evidence Tools。
- 新增 Code Navigation Tools。
- Diagnosis 必须 Evidence-backed。
- Database 未配置时正常退化到 report/log/code navigation，不得阻塞测试。

## Skills

新增：

```text
.code-harness/skills/select-test-targets/SKILL.md
.code-harness/skills/query-database/SKILL.md
```

修改：

```text
.code-harness/skills/analyze-change/SKILL.md
.code-harness/skills/design-integration-tests/SKILL.md
.code-harness/skills/run-integration-tests/SKILL.md
.code-harness/skills/analyze-failure/SKILL.md
```

---

# 9. Runtime / 文件结构

新增：

```text
.code-harness/database.template.yaml

.code-harness/contracts/
  database-config.schema.json
  database-evidence.schema.json
  test-target-selection.schema.json

.code-harness/skills/
  select-test-targets/SKILL.md
  query-database/SKILL.md

.code-harness/tools-runtime/internal/
  dbconfig/
  dbguard/
  dbmysql/
  dbevidence/
```

修改：

```text
.code-harness/agents/orchestrator.md
.code-harness/agents/integration-test-agent.md
.code-harness/agents/runtime-debugger.md

.code-harness/skills/analyze-change/SKILL.md
.code-harness/skills/design-integration-tests/SKILL.md
.code-harness/skills/analyze-failure/SKILL.md

.code-harness/contracts/change-analysis.schema.json
.code-harness/contracts/diagnosis.schema.json
.code-harness/tools/README.md
.code-harness/tools-runtime/cmd/codea-harness-tools/main.go
.code-harness/tools-runtime/go.mod
.code-harness/.gitignore
README.md
VERSION
.github/workflows/package-windows-x64.yml
```

---

# 10. Upgrade 语义

1.1.1 → 1.2.0 必须保持现有 staged/rollback/self-exe replace 机制。

Framework Managed 新增：

```text
database.template.yaml
contracts/database-*.schema.json
contracts/test-target-selection.schema.json
skills/select-test-targets/**
skills/query-database/**
```

Project State 新增：

```text
database.yaml
```

升级要求：

1. 已存在 `database.yaml` 必须字节级保留。
2. 新版本不得自动创建带真实凭据的 `database.yaml`。
3. `database.template.yaml` 随 Framework replace。
4. `.gitignore` 新版必须忽略 `database.yaml`。
5. 1.1.1 项目没有 database.yaml 时升级照常成功，DB Evidence 默认为 disabled。
6. 任何 DB 配置校验失败都**不应该阻止 Harness Upgrade 本身**；数据库是可选项目状态。运行 DB Tool 时再 fail closed。

---

# 11. 关键验收用例

## A. Test Target Selection

### A1 单 Controller

- affectedControllers=1
- 不打断用户
- mode=AUTO_SINGLE
- 只设计/执行该 Controller 测试

### A2 多 Controller

- affectedControllers=3
- 必须进入 WAITING_TEST_SELECTION
- 宿主支持 UI 时显示多选
- 用户选 1、3 后只处理 1、3

### A3 DIRECT_ONLY

- 1 个 DIRECT_CHANGE + 2 个 AFFECTED_BY_CALL_CHAIN
- 选择“仅直接变更”后只保留 DIRECT_CHANGE

### A4 Selection != Approval

- 用户选择 PaymentController
- strategy=EXTEND_EXISTING
- Harness 仍必须输出 planId 并等待精确审批

### A5 无交互宿主

- 降级编号选择
- 用户不用输入 Controller 名称

## B. Database Safety

### B1 database.yaml 缺失

- harness test 正常工作
- DB Evidence 显示 unavailable/disabled

### B2 非 TEST/LOCAL

- environment=PRODUCTION
- schema validation / runtime load 必须失败
- 0 次数据库连接

### B3 写 SQL

以下全部必须在 DB 连接/执行前拒绝：

```sql
UPDATE order_info SET status='X'
DELETE FROM order_info
DROP TABLE order_info
WITH x AS (...) DELETE ...
SELECT * FROM t FOR UPDATE
SELECT * FROM t INTO OUTFILE 'x'
```

### B4 多语句

```sql
SELECT 1; DELETE FROM t;
```

必须拒绝。

### B5 Schema 越界

allowedSchemas=[order_test]，查询 `other_db.user` → 拒绝。

### B6 复杂只读 SQL

JOIN / subquery / CTE SELECT → 允许。

### B7 row limit

查询返回 5000 行、maxRows=100 → 只返回 100 行，`truncated=true`。

### B8 timeout

超过 timeoutSeconds → 中断并输出可诊断错误，不挂住 Agent。

### B9 密码安全

password 不得出现在 stdout/stderr/evidence/diagnosis。

### B10 Evidence

合法查询必须生成符合 `database-evidence.schema.json` 的 run artifact。

## C. Failure Code Navigation

### C1 Stack Trace 内部代码

- stack trace 指向 OrderServiceImpl.java:186
- analyze-failure 必须 read_code 该位置
- Diagnosis.codeEvidence 必须引用实际文件和行范围

### C2 接口到实现

- stack trace/日志只有 OrderService
- 必须 find_implementations → OrderServiceImpl → read_code

### C3 DB + Code 联合根因

- Test expected APPROVED actual PENDING
- DB query 证明 status=PENDING
- 日志证明 transaction rollback
- read_code 证明异常路径
- Diagnosis=PRODUCTION_CODE_ERROR，evidence 同时引用 test/log/db/code

### C4 外部 RPC

- 根因链到 PaymentRpcClient
- 记录 externalDependencies
- 不猜 RPC 服务端实现

### C5 预算用尽

- 6 hops / 30 files / query budget 用尽仍不足
- Diagnosis=UNKNOWN / STOP_UNKNOWN

---

# 12. CI / Packaging Gate

`package-windows-x64` 必须新增：

1. `go test -count=1 ./...`
2. `go vet ./...`
3. Windows x64 build
4. Existing Code Navigation smoke
5. Existing Windows live upgrade lifecycle smoke
6. Database config validation smoke
7. SQL Safety Golden Tests
8. MySQL DB Runtime 单元测试使用 fake/driver mock，不要求 GitHub Runner 外连数据库
9. package completeness：必须含 `database.template.yaml`，不得含 `database.yaml`
10. Offline package 解压后运行时不得要求公网访问

现有 1.1.1 Gate 不能回退。

---

# 13. 1.2.0 Definition of Done

只有同时满足以下条件才可最终 PASS：

1. 多 Controller Selection Gate 真实工作，未选择 Controller 不进入测试设计/执行。
2. 单 Controller 自动继续，不增加无意义交互。
3. Selection 与 Test/Fix Approval 完全独立。
4. `database.yaml` 被忽略、升级保留、不进入离线发布包。
5. Runtime Debugger 能自主 list/describe/query 测试 MySQL。
6. `db_query_readonly` 支持复杂 SELECT/JOIN/CTE，但写 SQL/多语句/锁/跨 Schema 全部 fail closed。
7. DB query 有 timeout、row cap、query budget、Evidence artifact 和敏感字段脱敏。
8. Database 未配置时 Harness 功能正常退化。
9. Integration Test Agent 对适用业务生成数据库断言，但不因为 DB Assertion 引入新项目依赖。
10. Runtime Debugger 能从 Failure Evidence 自动进入 Code Navigation，读取实际实现后再确认生产代码根因。
11. Diagnosis Contract 能记录 suspectSymbols/codeEvidence/databaseEvidence/externalDependencies。
12. 所有新 JSON/YAML Artifact 都经过 deterministic runtime/schema validation。
13. Windows x64 CI、offline package、upgrade lifecycle 全绿。
14. 1.1.1 Review/Approval/Existing Test/Upgrade 安全规则无退化。
