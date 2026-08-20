# Codea Harness 1.2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 Codea Harness 1.1.1 基线上交付 Test Target Selection、Database Evidence、Failure Code Navigation，使 `harness test` 能在真实 Java/Spring Boot/Maven 项目中选择测试目标、执行测试、采集日志与数据库证据并回溯代码根因。

**Architecture:** 保持现有 Orchestrator / Reviewer / Integration Test Agent / Runtime Debugger / Fix Agent / Project Adapter 分工。Test Selection 属于 Orchestrator/Host Interaction Gate；数据库连接、SQL AST Gate、Evidence 落盘属于受控 Go Runtime；失败诊断由 Runtime Debugger 联合 Surefire、日志、DB Evidence 和现有 Code Navigation 完成。

**Tech Stack:** Go 1.23.x、Windows x64、JSON Schema Draft 2020-12、`gopkg.in/yaml.v3`、`github.com/santhosh-tekuri/jsonschema/v6`、ast-grep Code Navigation、MySQL、`github.com/go-sql-driver/mysql v1.9.3`、`vitess.io/vitess v0.21.6`（仅封装 SQL parser）、`github.com/DATA-DOG/go-sqlmock v1.5.2`（测试）。

**Spec:** `docs/superpowers/specs/2026-08-20-codea-harness-1.2-design.md`

## Global Constraints

- 基线为已验收的 Codea Harness 1.1.1；不得重做或放宽 Review Change Set、Review Coverage、Approval、Existing Test、Upgrade replace/self-upgrade 规则。
- Target version：`1.2.0`。
- 用户继续使用 `harness review/test/debug-service/fix/verify/upgrade` 自然语言意图；不得新增独立用户 CLI/TUI。
- 多 Controller 时不得默认全部执行；Selection 必须发生在测试设计之前。
- Selection 只代表“测哪个”，不得等价于 `批准 <planId>` 或 `批准 <fixPlanId>`。
- Database Runtime 1.2.0 只支持 MySQL，只允许 `TEST` / `LOCAL`。
- Agent 永远不得获得数据库写能力；DB 查询必须通过 controlled runtime。
- `database.yaml` 是本机 Project State：不得进包、不得被 upgrade replace、不得被普通 Agent read tools 读取；升级必须保留。
- `database.template.yaml` 是 Framework Managed。
- SQL 安全决策必须基于成熟 AST parser；禁止自研半成品 SQL parser、正则或字符串前缀作为安全边界。
- Database 未配置/禁用时，其余 Harness 能力正常退化。
- Runtime Debugger 可读取与失败证据相关的未变更代码，但禁止无界扫描；每次 Diagnosis 最多 6 navigation hops、30 个 source/test files，DB 查询受 `maxQueriesPerDiagnosis` 限制。
- 所有新增 JSON/YAML artifact 必须经过 deterministic Contract validation；需要跨字段语义时增加机器 verifier，不得让 Agent 自证。
- 不允许任意 Shell、Shell 求值、管道、重定向或用户命令拼接。
- Windows x64 与离线运行不得退化。
- 每个 Task：先失败测试 → 最小实现 → targeted tests → `go test ./...` → 独立提交 → 独立评审。

---

## File Structure

### New files

```text
.code-harness/database.template.yaml
.code-harness/contracts/database-config.schema.json
.code-harness/contracts/database-evidence.schema.json
.code-harness/contracts/test-target-selection.schema.json
.code-harness/skills/select-test-targets/SKILL.md
.code-harness/skills/query-database/SKILL.md

.code-harness/tools-runtime/internal/selection/selection.go
.code-harness/tools-runtime/internal/selection/selection_test.go
.code-harness/tools-runtime/internal/dbconfig/config.go
.code-harness/tools-runtime/internal/dbconfig/config_test.go
.code-harness/tools-runtime/internal/dbguard/guard.go
.code-harness/tools-runtime/internal/dbguard/guard_test.go
.code-harness/tools-runtime/internal/dbmysql/client.go
.code-harness/tools-runtime/internal/dbmysql/client_test.go
.code-harness/tools-runtime/internal/dbevidence/evidence.go
.code-harness/tools-runtime/internal/dbevidence/evidence_test.go
```

### Modified files

```text
.code-harness/agents/orchestrator.md
.code-harness/agents/integration-test-agent.md
.code-harness/agents/runtime-debugger.md
.code-harness/agents/project-adapter.md

.code-harness/skills/analyze-change/SKILL.md
.code-harness/skills/design-integration-tests/SKILL.md
.code-harness/skills/generate-integration-tests/SKILL.md
.code-harness/skills/run-integration-tests/SKILL.md
.code-harness/skills/analyze-failure/SKILL.md

.code-harness/contracts/change-analysis.schema.json
.code-harness/contracts/diagnosis.schema.json
.code-harness/tools/README.md

.code-harness/tools-runtime/cmd/codea-harness-tools/main.go
.code-harness/tools-runtime/cmd/codea-harness-tools/main_test.go
.code-harness/tools-runtime/go.mod
.code-harness/tools-runtime/go.sum
.code-harness/tools-runtime/internal/upgrade/upgrade.go
.code-harness/tools-runtime/internal/upgrade/upgrade_test.go
.code-harness/tools-runtime/internal/schema/schema_test.go

.code-harness/.gitignore
.code-harness/VERSION
.github/workflows/package-windows-x64.yml
README.md
CHANGELOG.md
THIRD_PARTY_NOTICES.md
```

---

### Task 1: Test Target Selection Contract + Deterministic Gate

**Files:**
- Create: `.code-harness/contracts/test-target-selection.schema.json`
- Create: `.code-harness/skills/select-test-targets/SKILL.md`
- Create: `.code-harness/tools-runtime/internal/selection/selection.go`
- Test: `.code-harness/tools-runtime/internal/selection/selection_test.go`
- Modify: `.code-harness/contracts/change-analysis.schema.json`
- Modify: `.code-harness/skills/analyze-change/SKILL.md`
- Modify: `.code-harness/agents/orchestrator.md`
- Modify: `.code-harness/agents/integration-test-agent.md`
- Modify: `.code-harness/tools-runtime/cmd/codea-harness-tools/main.go`
- Test: `.code-harness/tools-runtime/internal/schema/schema_test.go`

**Interfaces:**

`ChangeAnalysis.affectedControllers[]`:

```json
{
  "controller": "OrderController",
  "endpoints": ["POST /order/approve"],
  "impactType": "DIRECT_CHANGE",
  "sourceSymbols": ["OrderController.approve"]
}
```

Selection artifact:

```json
{
  "selectionId": "sel-001",
  "status": "SELECTED",
  "mode": "USER_MULTI",
  "selectedControllerIds": ["controller:OrderController"],
  "availableControllerIds": ["controller:OrderController", "controller:PaymentController"]
}
```

Machine verifier:

```go
func VerifyJSON(data []byte) error
```

- [ ] **Step 1: Write failing schema + semantic tests**

Test cases:

```text
valid AUTO_SINGLE -> PASS
valid USER_MULTI -> PASS
SELECTED + empty selectedControllerIds -> FAIL
unknown mode -> FAIL
selectedControllerIds contains id absent from availableControllerIds -> FAIL
selectedControllerIds contains duplicates -> FAIL
availableControllerIds contains duplicates -> FAIL
```

Important: `selected ⊆ available` is a cross-property invariant and MUST be checked in `internal/selection.VerifyJSON`; do not pretend JSON Schema can compare arbitrary values across sibling arrays.

Run:

```text
cd .code-harness/tools-runtime
go test ./internal/selection ./internal/schema -count=1
```

Expected: FAIL before implementation.

- [ ] **Step 2: Extend ChangeAnalysis contract**

`affectedControllers[]` must require:

```text
controller
endpoints
impactType = DIRECT_CHANGE | AFFECTED_BY_CALL_CHAIN
sourceSymbols (minItems=1)
```

Update `analyze-change`:

```text
directly changed Controller -> DIRECT_CHANGE
reverse-discovered Controller -> AFFECTED_BY_CALL_CHAIN
sourceSymbols explains why it is affected
```

- [ ] **Step 3: Add selection JSON Schema**

Required fields:

```text
selectionId
status = SELECTED | CANCELLED
mode = AUTO_SINGLE | USER_MULTI | USER_ALL | USER_DIRECT_ONLY | FALLBACK_NUMBERED
selectedControllerIds
availableControllerIds
```

Use `uniqueItems=true`. `status=SELECTED` requires at least one selected id. `status=CANCELLED` permits an empty selected list.

- [ ] **Step 4: Implement deterministic selection verifier**

`VerifyJSON` must parse the artifact and enforce:

```text
all selected ids exist in available ids
AUTO_SINGLE -> exactly 1 available and exactly 1 selected
USER_DIRECT_ONLY / USER_ALL / USER_MULTI / FALLBACK_NUMBERED -> non-empty selected when status SELECTED
CANCELLED -> no downstream continuation
```

Hook it into controlled runtime validation exactly like Review Coverage: when schema basename is `test-target-selection.schema.json`, schema PASS must be followed by `selection.VerifyJSON`; semantic failure returns nonzero.

- [ ] **Step 5: Implement Host Interaction behavior**

`select-test-targets/SKILL.md` + Orchestrator must encode exactly:

```text
0 target -> NO_TEST_TARGET -> DONE
1 target -> AUTO_SINGLE -> persist+validate artifact -> continue without prompt
>=2 targets -> WAITING_TEST_SELECTION
native structured multi-select supported -> use it
otherwise -> numbered fallback
shortcuts -> ALL / DIRECT_ONLY
cancel -> CANCELLED -> stop
```

Do not require users to type Controller names.

- [ ] **Step 6: Enforce selected-only downstream scope**

Integration Test Agent input becomes:

```text
ChangeAnalysis + validated TestTargetSelection
```

Hard rules:

```text
unselected Controller -> no existing-test coverage analysis
unselected Controller -> no TestPlan target
unselected Controller -> no Runtime execution
Selection != Test Approval != Fix Approval
```

- [ ] **Step 7: Run tests**

```text
cd .code-harness/tools-runtime
go test ./internal/selection ./internal/schema -count=1
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```text
git add .code-harness/contracts/test-target-selection.schema.json .code-harness/contracts/change-analysis.schema.json .code-harness/skills/select-test-targets .code-harness/skills/analyze-change .code-harness/agents/orchestrator.md .code-harness/agents/integration-test-agent.md .code-harness/tools-runtime/internal/selection .code-harness/tools-runtime/cmd/codea-harness-tools/main.go .code-harness/tools-runtime/internal/schema/schema_test.go
git commit -m "feat: add deterministic test target selection"
```

**Acceptance:** 单 Controller 自动继续；多 Controller 必须选择；未选择 Controller 无法进入测试设计/执行；selection artifact 由机器验证且不能授权写操作。

---

### Task 2: Local Database Configuration + Safe Loading

**Files:**
- Create: `.code-harness/database.template.yaml`
- Create: `.code-harness/contracts/database-config.schema.json`
- Create: `.code-harness/tools-runtime/internal/dbconfig/config.go`
- Test: `.code-harness/tools-runtime/internal/dbconfig/config_test.go`
- Modify: `.code-harness/.gitignore`
- Modify: `.code-harness/agents/project-adapter.md`
- Modify: `.code-harness/tools/README.md`

**Interfaces:**

```go
type Config struct {
    Version int
    Enabled bool
    Environment string
    Dialect string
    Connection Connection
    Safety Safety
}

type Connection struct {
    Host string
    Port int
    Database string
    Username string
    Password string
    Charset string
}

type Safety struct {
    AllowedSchemas []string
    MaxRows int
    TimeoutSeconds int
    MaxQueriesPerDiagnosis int
    AllowSchemaDiscovery bool
    AllowReadonlySQL bool
}

func Load(path, schemaPath string) (Config, error)
```

- [ ] **Step 1: Write failing dbconfig tests**

Cover:

```text
missing database.yaml -> database capability unavailable/disabled, not global Harness failure
enabled valid TEST config -> PASS
enabled valid LOCAL config -> PASS
environment=PRODUCTION -> FAIL
non-mysql dialect -> FAIL
empty allowedSchemas -> FAIL
maxRows > 1000 -> FAIL
timeoutSeconds > 30 -> FAIL
maxQueriesPerDiagnosis > 20 -> FAIL
unknown YAML field -> FAIL
password value never appears in formatted errors
```

Run:

```text
cd .code-harness/tools-runtime
go test ./internal/dbconfig -count=1
```

Expected: FAIL before package exists.

- [ ] **Step 2: Add template + Contract**

`database.template.yaml`:

```yaml
version: 1
enabled: false
environment: TEST
dialect: mysql
connection:
  host: 127.0.0.1
  port: 3306
  database: replace_me
  username: codea_readonly
  password: replace_me
  charset: utf8mb4
safety:
  allowedSchemas:
    - replace_me
  maxRows: 100
  timeoutSeconds: 10
  maxQueriesPerDiagnosis: 10
  allowSchemaDiscovery: true
  allowReadonlySql: true
```

Schema uses Draft 2020-12 and `additionalProperties=false` at controlled object levels. `environment` only LOCAL/TEST; `dialect` only mysql; limits match spec.

- [ ] **Step 3: Implement `dbconfig.Load`**

Flow:

```text
read YAML
-> existing schema.ValidateYAML(database-config.schema.json)
-> typed yaml.Unmarshal
-> enforce hard limits/semantic safety
-> return typed Config
```

Do not duplicate JSON Schema implementation. Do not log/format the password.

- [ ] **Step 4: Protect credentials**

`.code-harness/.gitignore` must contain:

```text
database.yaml
```

Update Tool Contract and Project Adapter:

```text
Project Adapter never connects to DB
database.yaml is optional Project State
database.yaml credentials never copied into harness.yaml/project.md
read_code/read_project_file must not expose .code-harness/database.yaml
```

- [ ] **Step 5: Run tests + commit**

```text
cd .code-harness/tools-runtime
go test ./internal/dbconfig -count=1
go test ./... -count=1
```

Commit:

```text
git add .code-harness/database.template.yaml .code-harness/.gitignore .code-harness/contracts/database-config.schema.json .code-harness/tools-runtime/internal/dbconfig .code-harness/agents/project-adapter.md .code-harness/tools/README.md
git commit -m "feat: add local database evidence configuration"
```

**Acceptance:** 本地 DB 配置可校验；PRODUCTION/非 MySQL/越界参数 fail closed；普通 Agent 永远看不到 DB 密码。

---

### Task 3: SQL AST Read-Only Safety Gate

**Files:**
- Create: `.code-harness/tools-runtime/internal/dbguard/guard.go`
- Test: `.code-harness/tools-runtime/internal/dbguard/guard_test.go`
- Modify: `.code-harness/tools-runtime/go.mod`
- Modify: `.code-harness/tools-runtime/go.sum`

**Interfaces:**

```go
type StatementInfo struct {
    StatementType string
    Schemas []string
    Tables []string
}

func ValidateReadonlyQuery(sqlText, defaultSchema string, allowedSchemas []string) (StatementInfo, error)
```

`vitess.io/vitess` AST types remain internal to `dbguard`.

- [ ] **Step 1: Pin parser dependency and verify baseline compatibility**

Add:

```text
vitess.io/vitess v0.21.6
```

Before writing guard code run:

```text
cd .code-harness/tools-runtime
go mod tidy
go test ./...
GOOS=windows GOARCH=amd64 go build ./cmd/codea-harness-tools
```

Gate: must build on the repository's Go 1.23.x baseline without toolchain-upgrade downloads or linker workaround flags. If this exact dependency cannot meet the gate, stop and amend the design dependency choice; do NOT substitute regex/handwritten parsing.

- [ ] **Step 2: Write Golden SQL tests**

ALLOW:

```sql
SELECT id,status FROM order_info WHERE id = ?
SELECT o.id,a.action FROM order_info o LEFT JOIN audit_log a ON a.order_id=o.id WHERE o.id=?
WITH recent AS (SELECT id FROM order_info WHERE id=?) SELECT * FROM recent
SELECT * FROM order_test.order_info WHERE id=?
```

REJECT before DB execution:

```sql
UPDATE order_info SET status='X'
DELETE FROM order_info
INSERT INTO order_info(id) VALUES(1)
DROP TABLE order_info
TRUNCATE TABLE order_info
SELECT * FROM order_info FOR UPDATE
SELECT * FROM order_info INTO OUTFILE 'x'
SELECT 1; DELETE FROM order_info
SELECT * FROM other_db.user
CALL dangerous_proc()
SET autocommit=0
WITH doomed AS (SELECT id FROM order_info) DELETE FROM order_info WHERE id IN (SELECT id FROM doomed)
```

- [ ] **Step 3: Implement AST-only validation**

Required flow:

```text
parse SQL with mature parser
-> exactly one statement
-> top-level read query only (SELECT / WITH...SELECT)
-> reject locks
-> reject INTO OUTFILE / DUMPFILE
-> collect all referenced tables/schemas
-> resolve unqualified tables to defaultSchema
-> every schema must be in allowedSchemas
-> return StatementInfo
```

Parse failure = reject. No regex/prefix acceptance fallback.

- [ ] **Step 4: Run tests + commit**

```text
cd .code-harness/tools-runtime
go test ./internal/dbguard -count=1
go test ./... -count=1
GOOS=windows GOARCH=amd64 go build ./cmd/codea-harness-tools
```

Commit:

```text
git add .code-harness/tools-runtime/go.mod .code-harness/tools-runtime/go.sum .code-harness/tools-runtime/internal/dbguard
git commit -m "feat: enforce readonly SQL with AST validation"
```

**Acceptance:** JOIN/subquery/CTE SELECT 可执行；写 SQL、多语句、锁、outfile、跨 schema 全部在 DB 执行前拒绝。

---

### Task 4: MySQL Read-Only Runtime + Database Evidence

**Files:**
- Create: `.code-harness/contracts/database-evidence.schema.json`
- Create: `.code-harness/tools-runtime/internal/dbmysql/client.go`
- Test: `.code-harness/tools-runtime/internal/dbmysql/client_test.go`
- Create: `.code-harness/tools-runtime/internal/dbevidence/evidence.go`
- Test: `.code-harness/tools-runtime/internal/dbevidence/evidence_test.go`
- Modify: `.code-harness/tools-runtime/go.mod`
- Modify: `.code-harness/tools-runtime/go.sum`

**Interfaces:**

```go
type QueryRequest struct {
    RunID string
    QueryID string
    Purpose string
    SQL string
    Params []any
}

type QueryResult struct {
    QueryID string
    RunID string
    Purpose string
    Schema string
    StatementType string
    Columns []string
    Rows []map[string]any
    RowCount int
    Truncated bool
    DurationMs int64
}

func Open(cfg dbconfig.Config) (*Client, error)
func (c *Client) Ping(ctx context.Context) error
func (c *Client) ListTables(ctx context.Context, schema string) ([]string, error)
func (c *Client) DescribeTable(ctx context.Context, schema, table string) ([]Column, error)
func (c *Client) QueryReadonly(ctx context.Context, req QueryRequest) (QueryResult, error)
func WriteEvidence(root string, result QueryResult) (string, error)
```

- [ ] **Step 1: Add dependencies + failing unit tests**

Pin:

```text
github.com/go-sql-driver/mysql v1.9.3
github.com/DATA-DOG/go-sqlmock v1.5.2
```

Tests with mock/fake driver, no external DB:

```text
Ping success/failure
ListTables allowlist
DescribeTable allowlist
Query passes params
Query calls dbguard before DB
ReadOnly transaction/session path
Context timeout
500 rows + maxRows=100 -> return 100, truncated=true
sensitive columns redacted
password absent from errors/evidence
```

- [ ] **Step 2: Implement connection safely**

Use `mysql.Config`; never hand-build/log DSN. Never serialize password. Apply configured host/port/database/user/password/charset and timeout.

- [ ] **Step 3: Implement read-only query flow**

```text
validate dbconfig
-> dbguard.ValidateReadonlyQuery
-> timeout context
-> read-only transaction/connection semantics
-> parameterized query
-> read at most maxRows+1
-> if extra row exists: drop extra + truncated=true
-> sanitize
-> evidence write
```

Physical DB account is expected to be readonly, but AST/runtime gates remain mandatory.

- [ ] **Step 4: Implement schema discovery**

`ListTables` / `DescribeTable`:

```text
allowSchemaDiscovery must be true
schema must be in allowedSchemas
schema/table identifier must match ^[A-Za-z0-9_$]+$
```

Reject before query otherwise.

- [ ] **Step 5: Add Evidence Contract + sanitizer**

Evidence example:

```json
{
  "queryId":"dbq-001",
  "runId":"run-001",
  "purpose":"verify order state",
  "schema":"order_test",
  "statementType":"SELECT",
  "columns":["status"],
  "rows":[{"status":"PENDING"}],
  "rowCount":1,
  "truncated":false,
  "durationMs":31
}
```

Mask values when normalized column name contains:

```text
password passwd token secret accesskey privatekey
```

Replacement: `***REDACTED***`.

Write to:

```text
.code-harness/runs/<runId>/evidence/db/<queryId>.json
```

Validate artifact against `database-evidence.schema.json` before success.

- [ ] **Step 6: Run tests + commit**

```text
cd .code-harness/tools-runtime
go test ./internal/dbmysql ./internal/dbevidence -count=1
go test ./... -count=1
go vet ./...
```

Commit:

```text
git add .code-harness/contracts/database-evidence.schema.json .code-harness/tools-runtime/internal/dbmysql .code-harness/tools-runtime/internal/dbevidence .code-harness/tools-runtime/go.mod .code-harness/tools-runtime/go.sum
git commit -m "feat: add mysql readonly evidence runtime"
```

**Acceptance:** DB 结果受 timeout/row cap/脱敏约束并形成 deterministic evidence artifact，不需要 GitHub Runner 外连 MySQL。

---

### Task 5: Controlled DB Commands + Query Database Skill

**Files:**
- Modify: `.code-harness/tools-runtime/cmd/codea-harness-tools/main.go`
- Test: `.code-harness/tools-runtime/cmd/codea-harness-tools/main_test.go`
- Create: `.code-harness/skills/query-database/SKILL.md`
- Modify: `.code-harness/tools/README.md`
- Modify: `.code-harness/agents/runtime-debugger.md`

**Interfaces:**

Runtime:

```text
codea-harness-tools.exe db ping --run-id <id>
codea-harness-tools.exe db list-tables --schema <schema> --run-id <id>
codea-harness-tools.exe db describe-table --schema <schema> --table <table> --run-id <id>
codea-harness-tools.exe db query --input .code-harness/runs/<runId>/requests/<file>.json
```

Logical tools:

```text
db_ping(runId)
db_list_tables(schema, runId)
db_describe_table(schema, table, runId)
db_query_readonly(sql, params, runId, purpose)
```

- [ ] **Step 1: Write failing CLI tests**

```text
unknown db action -> nonzero
query input outside .code-harness/runs -> nonzero
query request missing runId/queryId/purpose -> nonzero
PRODUCTION config -> nonzero before connect
write SQL -> nonzero before connect
database.yaml missing -> controlled DATABASE_EVIDENCE_UNAVAILABLE, no panic
```

- [ ] **Step 2: Add `db` command dispatch**

Root usage:

```text
codea-harness-tools <upgrade|validate|nav|db>
```

Raw SQL must NOT be accepted as CLI argument. `db query` reads structured request JSON only from `.code-harness/runs/**`.

Request:

```json
{
  "runId":"run-001",
  "queryId":"dbq-001",
  "purpose":"verify order state after failure",
  "sql":"SELECT status FROM order_info WHERE order_id = ?",
  "params":[10001]
}
```

- [ ] **Step 3: Enforce query budget**

One test execution/diagnosis uses its current `runId`; before automatic query execution, count DB evidence entries for that diagnostic run. When count reaches `maxQueriesPerDiagnosis`, return deterministic `QUERY_BUDGET_EXCEEDED` and do not connect.

- [ ] **Step 4: Add `query-database` Skill**

Required behavior:

```text
Runtime Debugger only
schema exploration before guessing unknown tables
Agent may autonomously build SELECT/JOIN/CTE
use ? params for dynamic values
state purpose for each query
query only to resolve a concrete diagnostic question
stop when evidence is sufficient
never mutate data
```

DB unavailable is optional capability loss, not automatic test failure classification.

- [ ] **Step 5: Update Tool Contract docs**

Explicitly state:

```text
Agent never invokes mysql.exe
Agent never reads database.yaml
Agent never bypasses dbguard
DB tools use controlled runtime only
```

- [ ] **Step 6: Run tests + commit**

```text
cd .code-harness/tools-runtime
go test ./cmd/codea-harness-tools -count=1
go test ./... -count=1
go vet ./...
```

Commit:

```text
git add .code-harness/tools-runtime/cmd .code-harness/skills/query-database .code-harness/tools/README.md .code-harness/agents/runtime-debugger.md
git commit -m "feat: expose controlled database evidence tools"
```

**Acceptance:** Runtime Debugger 可以自主探索/查询测试库，但所有查询都经过 config + AST + budget + timeout + row cap + evidence gate。

---

### Task 6: Failure Code Navigation + Evidence-Backed Diagnosis

**Files:**
- Modify: `.code-harness/contracts/diagnosis.schema.json`
- Modify: `.code-harness/skills/analyze-failure/SKILL.md`
- Modify: `.code-harness/agents/runtime-debugger.md`
- Modify: `.code-harness/agents/fix-agent.md` only if needed to consume evidence; do not change approval semantics
- Test: `.code-harness/tools-runtime/internal/schema/schema_test.go`

**Interfaces:**

Diagnosis extensions:

```json
{
  "suspectSymbols":["OrderServiceImpl.approve"],
  "codeEvidence":[{
    "path":"src/main/java/.../OrderServiceImpl.java",
    "symbol":"OrderServiceImpl.approve",
    "lineStart":178,
    "lineEnd":190,
    "reason":"stack trace target"
  }],
  "databaseEvidence":["dbq-002"],
  "externalDependencies":["PaymentRpcClient"]
}
```

- [ ] **Step 1: Write failing Diagnosis schema tests**

Valid combined evidence case. Reject:

```text
lineStart <= 0
lineEnd < lineStart
empty suspect symbol
empty database evidence id
```

Keep existing classification and nextAction enums unchanged.

- [ ] **Step 2: Expand analyze-failure allowed tools**

Exactly:

```text
read_test_report
read_service_logs
find_symbol
find_references
find_implementations
read_code
db_ping
db_list_tables
db_describe_table
db_query_readonly
```

DB tools conditional on DB capability.

- [ ] **Step 3: Encode evidence-first investigation order**

```text
1 extract failedTests from Surefire
2 extract exception/message/stack/file:line/traceId
3 identify project-internal suspectSymbols
4 project file:line -> MUST read_code before claiming code root cause
5 interface -> find_implementations -> read implementation
6 upstream uncertainty -> find_references
7 downstream internal uncertainty -> find_symbol/read_code
8 DB symptom -> bounded DB evidence
9 stop once evidence proves classification/root cause
10 budget exhausted -> UNKNOWN/STOP_UNKNOWN
```

Budgets:

```text
navigation hops <= 6
source/test files read <= 30
DB queries <= maxQueriesPerDiagnosis
```

- [ ] **Step 4: Preserve role boundaries**

Runtime Debugger may not modify code/tests, approve plans, or bypass existing Orchestrator retry/repair loops. Fix Agent may consume new evidence but still requires exact `批准 <fixPlanId>`.

- [ ] **Step 5: Golden diagnosis cases**

```text
stack -> OrderServiceImpl.java:186 -> read_code -> codeEvidence
interface OrderService -> find_implementations -> implementation read
expected APPROVED/actual PENDING + DB + rollback log + source -> PRODUCTION_CODE_ERROR
PaymentRpcClient -> externalDependencies + stop, no server-side guess
budget exhausted -> UNKNOWN/STOP_UNKNOWN
```

- [ ] **Step 6: Run tests + commit**

```text
cd .code-harness/tools-runtime
go test ./internal/schema -count=1
go test ./... -count=1
```

Commit:

```text
git add .code-harness/contracts/diagnosis.schema.json .code-harness/skills/analyze-failure .code-harness/agents/runtime-debugger.md .code-harness/agents/fix-agent.md .code-harness/tools-runtime/internal/schema/schema_test.go
git commit -m "feat: add evidence-backed failure navigation"
```

**Acceptance:** Runtime Debugger 不再只做日志分类；可用代码存在时，确认生产代码根因前必须读取实际实现；证据不足必须 UNKNOWN。

---

### Task 7: Selected Test Flow + Integration-Test DB Assertions

**Files:**
- Modify: `.code-harness/skills/design-integration-tests/SKILL.md`
- Modify: `.code-harness/skills/generate-integration-tests/SKILL.md`
- Modify: `.code-harness/skills/run-integration-tests/SKILL.md`
- Modify: `.code-harness/agents/integration-test-agent.md`
- Modify: `.code-harness/agents/orchestrator.md`

**Interfaces:**
- Consumes validated TestTargetSelection.
- Uses existing `test-plan.schema.json expected.databaseAssertions[]`.
- Produces selected-only test classes for Runtime Debugger.

- [ ] **Step 1: Strengthen test design for DB/state risks**

For ChangeAnalysis risks:

```text
databaseWrite
transactional
stateTransition
```

Integration Test Agent must explicitly decide whether DB Assertion is required. When required, describe concrete state, e.g.:

```text
order_info.status == APPROVED for fixture order_id
audit_log contains one APPROVE record
```

- [ ] **Step 2: Generate DB assertions using project conventions only**

Priority:

```text
existing test helper/repository pattern
existing JdbcTemplate pattern
existing fixture/assertion utility
```

Do not add Maven dependency solely for DB assertion. Assert before cleanup/rollback hides the state.

- [ ] **Step 3: Lock selected-only execution**

`run-integration-tests` receives test classes only from selected targets. If a proposed execution belongs only to an unselected Controller, Orchestrator stops with scope violation.

- [ ] **Step 4: Preserve Existing Test/Approval rules**

```text
REUSE_EXISTING -> run, no approval, no modification
EXTEND_EXISTING -> only MISSING + exact plan approval
CREATE_NEW -> exact plan approval
historical Existing Test failure -> never auto-edit
GENERATED_BY_PLAN repair -> max 2 rounds
```

- [ ] **Step 5: End-to-end synthetic flow**

```text
3 affected Controllers
-> select Order + Payment
-> Order REUSE_EXISTING
-> Payment EXTEND_EXISTING
-> User unselected
-> exact approve Payment plan
-> execute only Order/Payment
-> failure -> Runtime Debugger -> DB/code evidence as needed
```

Assert no User coverage/plan/execution artifact exists.

- [ ] **Step 6: Commit**

```text
git add .code-harness/skills/design-integration-tests .code-harness/skills/generate-integration-tests .code-harness/skills/run-integration-tests .code-harness/agents/integration-test-agent.md .code-harness/agents/orchestrator.md
git commit -m "feat: integrate selected targets with database assertions"
```

**Acceptance:** 测试范围完全由 Selection 控制；DB Assertion 是正式测试证据；Existing Test 和审批安全规则不退化。

---

### Task 8: Upgrade 1.1.1 -> 1.2.0 + Project-State Preservation

**Files:**
- Modify: `.code-harness/tools-runtime/internal/upgrade/upgrade.go`
- Test: `.code-harness/tools-runtime/internal/upgrade/upgrade_test.go`
- Modify: `.code-harness/.gitignore` if needed by package template

**Interfaces:**

Framework Managed adds:

```text
database.template.yaml
contracts/database-config.schema.json
contracts/database-evidence.schema.json
contracts/test-target-selection.schema.json
skills/select-test-targets/**
skills/query-database/**
new tools-runtime DB/selection runtime files and built exe
```

Project State adds:

```text
database.yaml
```

- [ ] **Step 1: Write failing upgrade golden tests**

```text
1.1.1 -> 1.2.0 without database.yaml -> UPGRADED
1.1.1 -> 1.2.0 with database.yaml -> exact bytes preserved
database.template.yaml installed
invalid preserved database.yaml -> upgrade still succeeds
project.md preserved
runs/** preserved
harness.yaml follows existing migration semantics
stale Framework file removed
```

- [ ] **Step 2: Update managed/state classification**

`database.yaml` must never participate in Framework replace/stale delete and must not be copied from source upgrade package over target. `database.template.yaml` participates normally in Framework replace.

Do not alter staged transaction, rollback or Windows running-exe replacement design.

- [ ] **Step 3: Run upgrade regression**

```text
cd .code-harness/tools-runtime
go test ./internal/upgrade -count=1
go test ./... -count=1
```

Expected: all existing 1.1.1 tests + new DB state cases PASS.

- [ ] **Step 4: Commit**

```text
git add .code-harness/tools-runtime/internal/upgrade .code-harness/.gitignore
git commit -m "feat: preserve database state across harness upgrade"
```

**Acceptance:** Harness upgrade never destroys/overwrites local DB credentials and invalid optional DB config does not block Harness upgrade.

---

### Task 9: Windows Packaging + Docs + Final 1.2 Acceptance

**Files:**
- Modify: `.github/workflows/package-windows-x64.yml`
- Modify: `.code-harness/VERSION`
- Modify: `README.md`
- Modify: `CHANGELOG.md`
- Modify: `THIRD_PARTY_NOTICES.md`

**Interfaces:**
- Produces final `codea-harness-1.2.0-windows-x64.zip`.

- [ ] **Step 1: Extend Windows CI without removing existing gates**

Keep:

```text
Test controlled runtime
Build Windows x64 runtime
Vendor pinned ast-grep Windows x64
Code Navigation end-to-end smoke
Windows live upgrade lifecycle smoke
Assert package completeness
Build offline package
Run actions/upload-artifact
```

Add:

```text
Selection verifier/schema tests
Database config validation tests
SQL Safety Golden Tests
DB runtime mock/unit tests
package leak assertion for database.yaml
```

- [ ] **Step 2: Extend live upgrade smoke**

Old Harness fixture must contain a sentinel `.code-harness/database.yaml`. After real running-exe upgrade assert:

```text
VERSION == 1.2.0
database.yaml exact bytes unchanged
database.template.yaml exists
stale framework file removed
new exe installed
stage/backup/source cleanup remains correct
```

- [ ] **Step 3: Package completeness/exclusion**

ZIP MUST contain:

```text
.code-harness/database.template.yaml
.code-harness/contracts/database-config.schema.json
.code-harness/contracts/database-evidence.schema.json
.code-harness/contracts/test-target-selection.schema.json
.code-harness/skills/select-test-targets/SKILL.md
.code-harness/skills/query-database/SKILL.md
```

ZIP MUST NOT contain:

```text
.code-harness/database.yaml
```

- [ ] **Step 4: Update docs/notices**

README documents only user-facing flow:

```text
multi-controller harness test selection
database.template.yaml -> local database.yaml
database.yaml ignored/local only
DB Evidence optional
readonly TEST/LOCAL MySQL requirement
```

CHANGELOG `1.2.0` records exactly the three headline features: Test Target Selection, Database Evidence, Failure Code Navigation.

THIRD_PARTY_NOTICES adds notices/licenses required by:

```text
github.com/go-sql-driver/mysql v1.9.3
vitess.io/vitess v0.21.6
github.com/DATA-DOG/go-sqlmock v1.5.2 (development/test dependency as applicable)
```

- [ ] **Step 5: Set canonical version only after Tasks 1-8 pass**

Update:

```text
.code-harness/VERSION
```

to:

```text
1.2.0
```

No root `VERSION` file is used.

- [ ] **Step 6: Final local verification**

```text
cd .code-harness/tools-runtime
go test -count=1 ./...
go vet ./...
GOOS=windows GOARCH=amd64 go build ./cmd/codea-harness-tools
```

Expected: PASS.

- [ ] **Step 7: Final Golden Acceptance**

All must pass:

```text
A1 single Controller AUTO_SINGLE
A2 multi Controller selection
A3 DIRECT_ONLY
A4 Selection != Approval
A5 numbered fallback
B1 database.yaml missing graceful degradation
B2 PRODUCTION rejected before connect
B3 write SQL rejected
B4 multi-statement rejected
B5 cross-schema rejected
B6 JOIN/subquery/CTE SELECT allowed
B7 row cap
B8 timeout
B9 password never leaks
B10 evidence validates
C1 stack trace internal file -> mandatory read_code
C2 interface -> find_implementations
C3 DB + log + code -> evidence-backed production diagnosis
C4 external RPC boundary stop
C5 budget exhausted -> UNKNOWN
Upgrade preserves database.yaml
Package excludes database.yaml
Existing 1.1.1 Review/Approval/Existing Test/Upgrade tests all green
```

- [ ] **Step 8: Run real `package-windows-x64` on final HEAD**

Final PASS requires GitHub Actions `completed / success` on the same final develop HEAD, including live Windows upgrade smoke and artifact upload.

- [ ] **Step 9: Commit release docs/workflow/version**

```text
git add .github/workflows/package-windows-x64.yml .code-harness/VERSION README.md CHANGELOG.md THIRD_PARTY_NOTICES.md
git commit -m "release: prepare Codea Harness 1.2.0"
```

**Acceptance:** offline artifact complete、无真实 `database.yaml`、Windows lifecycle 全绿、1.1.1 安全规则无回归。

---

## Review Checkpoints

每个 Task 单独验收，Reviewer 只输出：`PASS` 或 `FAIL + all blockers`，不要把已知问题拖到后续 Task。

```text
Task 1 -> Selection Contract/Runtime Gate
Task 2 -> DB Config/Credential Gate
Task 3 -> SQL Safety Gate
Task 4 -> MySQL Evidence Runtime Gate
Task 5 -> DB Tool/Agent Gate
Task 6 -> Failure Navigation Gate
Task 7 -> Test Flow Gate
Task 8 -> Upgrade Gate
Task 9 -> Release/Windows Gate
```

## Final Reviewer Blockers

出现任一项，1.2 必须 FAIL：

1. 多 Controller 可绕过 Selection 或默认全选。
2. 未选择 Controller 可进入 Test Plan/执行。
3. Selection 可被当作 Test/Fix approval。
4. Agent 可通过普通 read tools 读取 `database.yaml` 或 password。
5. Agent 可绕过 controlled runtime 执行 SQL。
6. SQL 安全决策使用 regex/prefix/自研不完整 parser，而不是成熟 AST parser。
7. 写 SQL、多语句、锁、outfile、跨 schema 任一能到达 DB execution。
8. DB query 缺 timeout、row cap、query budget 或 evidence sanitization。
9. database.yaml 缺失/disabled 导致非 DB Harness 功能失败。
10. Runtime Debugger 在可读实际代码时，仅凭日志/stack text 就确认生产代码根因。
11. Debug Navigation 无界扫描仓库或超过预算仍猜根因。
12. `database.yaml` 被 track/package/overwrite/delete。
13. `.code-harness/VERSION` 与发布包版本不一致。
14. 新依赖没有进入 THIRD_PARTY_NOTICES。
15. 1.1.1 Review/Approval/Existing Test/Windows upgrade lifecycle 任一回归。

只有 Task 1-9 全部 PASS，且最终 Windows workflow 在同一 HEAD 上真实成功，Codea Harness 1.2.0 才可标记最终 `PASS`。
