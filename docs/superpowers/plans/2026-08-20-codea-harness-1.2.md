# Codea Harness 1.2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 Codea Harness 1.1.1 基线上交付 Test Target Selection、Database Evidence、Failure Code Navigation，使 `harness test` 能在真实 Java/Spring Boot/Maven 项目中选择测试目标、执行测试、采集日志与数据库证据并回溯代码根因。

**Architecture:** 保持现有 Orchestrator / Reviewer / Integration Test Agent / Runtime Debugger / Fix Agent / Project Adapter 分工。Test Selection 属于 Orchestrator/Host Interaction Gate；数据库连接、SQL AST Gate、Evidence 落盘属于受控 Go Runtime；失败诊断由 Runtime Debugger 联合 Surefire、日志、DB Evidence 和现有 Code Navigation 完成。

**Tech Stack:** Go 1.23 baseline、Windows x64、Java/Spring Boot/Maven Harness、JSON Schema Draft 2020-12、yaml.v3、ast-grep Code Navigation、MySQL、`github.com/go-sql-driver/mysql v1.9.3`、`vitess.io/vitess v0.21.6`（仅封装 SQL parser）、`github.com/DATA-DOG/go-sqlmock v1.5.2`（测试）。

**Spec:** `docs/superpowers/specs/2026-08-20-codea-harness-1.2-design.md`

## Global Constraints

- 基线版本为 Codea Harness 1.1.1；不得重做或放宽现有 Review Change Set、Review Coverage、Approval、Existing Test、Upgrade replace 规则。
- 用户继续使用 `harness review/test/debug-service/fix/verify/upgrade` 自然语言意图；不得新增独立用户 CLI 产品。
- 1.2.0 Database Runtime 只支持 MySQL，且只允许 TEST/LOCAL 数据源。
- Agent 永远不得获得数据库写能力；`db_query_readonly` 只允许单条 SELECT / WITH...SELECT。
- `database.yaml` 是 Project State：不得进包、不得被 upgrade replace、不得输出凭据。
- `database.template.yaml` 是 Framework Managed。
- Windows x64 与完全离线运行能力不得退化。
- 不允许任意 Shell、Shell 求值、管道、重定向或用户命令拼接。
- 每个 Task 必须先写失败测试，再做最小实现，再运行该 Task 的 targeted tests + `go test ./...`，最后独立提交。

---

## File Structure

### New framework files

```text
.code-harness/database.template.yaml
.code-harness/contracts/database-config.schema.json
.code-harness/contracts/database-evidence.schema.json
.code-harness/contracts/test-target-selection.schema.json
.code-harness/skills/select-test-targets/SKILL.md
.code-harness/skills/query-database/SKILL.md

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
.code-harness/skills/analyze-failure/SKILL.md

.code-harness/contracts/change-analysis.schema.json
.code-harness/contracts/diagnosis.schema.json

.code-harness/tools/README.md
.code-harness/tools-runtime/cmd/codea-harness-tools/main.go
.code-harness/tools-runtime/go.mod
.code-harness/tools-runtime/go.sum
.code-harness/tools-runtime/internal/upgrade/upgrade.go
.code-harness/tools-runtime/internal/upgrade/upgrade_test.go

.code-harness/.gitignore
README.md
VERSION
.github/workflows/package-windows-x64.yml
```

---

### Task 1: Test Target Selection Contract and Gate

**Files:**
- Create: `.code-harness/contracts/test-target-selection.schema.json`
- Create: `.code-harness/skills/select-test-targets/SKILL.md`
- Modify: `.code-harness/contracts/change-analysis.schema.json`
- Modify: `.code-harness/skills/analyze-change/SKILL.md`
- Modify: `.code-harness/agents/orchestrator.md`
- Modify: `.code-harness/agents/integration-test-agent.md`
- Test: `.code-harness/tools-runtime/internal/schema/schema_test.go`

**Interfaces:**
- Consumes: `ChangeAnalysis.affectedControllers[]`
- Produces: `.code-harness/runs/<runId>/test-target-selection.json`
- Required new `affectedControllers[]` fields:
  - `impactType: DIRECT_CHANGE | AFFECTED_BY_CALL_CHAIN`
  - `sourceSymbols: string[]`
- Selection modes: `AUTO_SINGLE | USER_MULTI | USER_ALL | USER_DIRECT_ONLY | FALLBACK_NUMBERED`

- [ ] **Step 1: Write contract validation tests that fail on the 1.1.1 schema**

Add table-driven cases to `schema_test.go` that validate the future selection schema:

```go
func TestTargetSelectionContract(t *testing.T) {
    valid := []byte(`{
      "selectionId":"sel-1",
      "status":"SELECTED",
      "mode":"USER_MULTI",
      "selectedControllerIds":["controller:OrderController"],
      "availableControllerIds":["controller:OrderController","controller:PaymentController"]
    }`)
    // load contracts/test-target-selection.schema.json and expect ValidateJSON == nil
}
```

Also add invalid cases:

```text
selectedControllerIds contains id not in availableControllerIds -> FAIL
status=SELECTED with empty selectedControllerIds -> FAIL
unknown mode -> FAIL
```

- [ ] **Step 2: Run the targeted schema test and confirm failure**

Run:

```text
cd .code-harness/tools-runtime
go test ./internal/schema -run TestTargetSelectionContract -count=1
```

Expected: FAIL because the contract does not exist / required schema semantics are not implemented in repository assets yet.

- [ ] **Step 3: Add the selection schema and extend ChangeAnalysis**

`test-target-selection.schema.json` must require:

```json
{
  "selectionId": "string",
  "status": "SELECTED|CANCELLED",
  "mode": "AUTO_SINGLE|USER_MULTI|USER_ALL|USER_DIRECT_ONLY|FALLBACK_NUMBERED",
  "selectedControllerIds": [],
  "availableControllerIds": []
}
```

Use Draft 2020-12 `contains`/conditional rules so `SELECTED` requires at least one selected id.

Update `change-analysis.schema.json` so each affected controller is:

```json
{
  "controller":"OrderController",
  "endpoints":["POST /order/approve"],
  "impactType":"DIRECT_CHANGE",
  "sourceSymbols":["OrderController.approve"]
}
```

- [ ] **Step 4: Implement the Host Interaction behavior in the Skill/Orchestrator**

`select-test-targets/SKILL.md` must encode exactly:

```text
0 target -> NO_TEST_TARGET
1 target -> AUTO_SINGLE, no user interruption
>=2 targets -> WAITING_TEST_SELECTION
host supports native structured selection -> multi-select UI
otherwise -> numbered fallback
shortcuts -> ALL / DIRECT_ONLY
cancel -> CANCELLED and stop
```

Orchestrator must write/validate `test-target-selection.json` before invoking Integration Test Agent.

- [ ] **Step 5: Enforce selected-only downstream scope**

Change Integration Test Agent input rule from “all affectedControllers” to:

```text
ChangeAnalysis + validated TestTargetSelection
```

Explicitly prohibit:

```text
unselected Controller -> coverage analysis
unselected Controller -> Test Plan
unselected Controller -> test execution
```

Selection must never count as `批准 <planId>` or `批准 <fixPlanId>`.

- [ ] **Step 6: Run tests**

```text
cd .code-harness/tools-runtime
go test ./internal/schema -count=1
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 7: Manual Golden acceptance**

Exercise the Agent instructions with synthetic ChangeAnalysis fixtures:

```text
1 Controller -> no prompt, AUTO_SINGLE
3 Controllers -> user gets selectable options
DIRECT_ONLY -> only DIRECT_CHANGE remains
selected EXTEND_EXISTING -> still WAITING_APPROVAL
```

- [ ] **Step 8: Commit**

```text
git add .code-harness/contracts .code-harness/skills .code-harness/agents .code-harness/tools-runtime/internal/schema
git commit -m "feat: add test target selection gate"
```

---

### Task 2: Local Database Configuration and Safe Loading

**Files:**
- Create: `.code-harness/database.template.yaml`
- Create: `.code-harness/contracts/database-config.schema.json`
- Create: `.code-harness/tools-runtime/internal/dbconfig/config.go`
- Create: `.code-harness/tools-runtime/internal/dbconfig/config_test.go`
- Modify: `.code-harness/.gitignore`
- Modify: `.code-harness/agents/project-adapter.md`
- Modify: `.code-harness/tools/README.md`

**Interfaces:**

```go
type Config struct {
    Version     int
    Enabled     bool
    Environment string
    Dialect     string
    Connection  Connection
    Safety      Safety
}

func Load(path string, schemaPath string) (Config, error)
```

Defaults after validation:

```text
maxRows=100
timeoutSeconds=10
maxQueriesPerDiagnosis=10
```

Hard maximums:

```text
maxRows<=1000
timeoutSeconds<=30
maxQueriesPerDiagnosis<=20
```

- [ ] **Step 1: Write failing dbconfig tests**

Cover all cases:

```go
func TestLoadDatabaseConfigRejectsProduction(t *testing.T) {}
func TestLoadDatabaseConfigRejectsNonMySQL(t *testing.T) {}
func TestLoadDatabaseConfigRejectsEmptyAllowedSchemas(t *testing.T) {}
func TestLoadDatabaseConfigAppliesSafeDefaults(t *testing.T) {}
func TestLoadDatabaseConfigNeverFormatsPasswordInError(t *testing.T) {}
```

Use temp files, never a real DB.

- [ ] **Step 2: Run and confirm failure**

```text
cd .code-harness/tools-runtime
go test ./internal/dbconfig -count=1
```

Expected: FAIL because package does not exist.

- [ ] **Step 3: Add database template and JSON Schema**

The template must exactly follow the approved V1 shape:

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

The schema must reject `environment=PRODUCTION` and any dialect other than `mysql`.

- [ ] **Step 4: Implement `dbconfig.Load`**

Implementation requirements:

```text
read YAML
-> schema.ValidateYAML(database-config.schema.json)
-> unmarshal typed Config
-> enforce hard maximums
-> return typed config
```

Errors may mention field names but never the password value.

- [ ] **Step 5: Protect database.yaml**

`.code-harness/.gitignore` must contain:

```text
database.yaml
```

Project Adapter documentation must say:

```text
database.yaml is optional Project State
Project Adapter never connects to DB
Project Adapter never copies credentials into project.md/harness.yaml
```

`read_project_file` contract must explicitly reject `.code-harness/database.yaml`.

- [ ] **Step 6: Run tests**

```text
cd .code-harness/tools-runtime
go test ./internal/dbconfig -count=1
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```text
git add .code-harness/database.template.yaml .code-harness/.gitignore .code-harness/contracts/database-config.schema.json .code-harness/tools-runtime/internal/dbconfig .code-harness/agents/project-adapter.md .code-harness/tools/README.md
git commit -m "feat: add local database evidence configuration"
```

---

### Task 3: SQL AST Read-Only Safety Gate

**Files:**
- Create: `.code-harness/tools-runtime/internal/dbguard/guard.go`
- Create: `.code-harness/tools-runtime/internal/dbguard/guard_test.go`
- Modify: `.code-harness/tools-runtime/go.mod`
- Modify: `.code-harness/tools-runtime/go.sum`

**Interfaces:**

```go
type StatementInfo struct {
    StatementType string
    Schemas       []string
    Tables        []string
}

func ValidateReadonlyQuery(sqlText string, defaultSchema string, allowedSchemas []string) (StatementInfo, error)
```

Use `vitess.io/vitess v0.21.6` only behind `internal/dbguard`.

- [ ] **Step 1: Add failing Golden SQL tests**

Allowed cases:

```sql
SELECT id,status FROM order_info WHERE id = ?
SELECT o.id,a.action FROM order_info o LEFT JOIN audit_log a ON a.order_id=o.id WHERE o.id=?
WITH recent AS (SELECT id FROM order_info WHERE id=?) SELECT * FROM recent
SELECT * FROM order_test.order_info WHERE id=?
```

Rejected cases:

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
```

- [ ] **Step 2: Run tests and confirm failure**

```text
cd .code-harness/tools-runtime
go test ./internal/dbguard -count=1
```

Expected: FAIL because package does not exist.

- [ ] **Step 3: Add pinned parser dependency**

Add:

```text
vitess.io/vitess v0.21.6
```

Do not expose Vitess AST types outside `internal/dbguard`.

- [ ] **Step 4: Implement AST validation**

Implementation rules:

```text
ParseMultipleIgnoreEmpty(sql)
-> statement count must equal 1
-> top-level must be SELECT-compatible read statement
-> reject locks
-> reject INTO OUTFILE/DUMPFILE
-> extract all table names
-> resolve unqualified tables to defaultSchema
-> every schema must be in allowedSchemas
-> return StatementInfo
```

Do not use `strings.HasPrefix(sql, "SELECT")` or regex as the security decision.

- [ ] **Step 5: Run targeted + full Go tests and Windows cross-build**

```text
cd .code-harness/tools-runtime
go test ./internal/dbguard -count=1
go test ./... -count=1
GOOS=windows GOARCH=amd64 go build ./cmd/codea-harness-tools
```

Expected: all PASS/build success on the project-supported environment.

- [ ] **Step 6: Commit**

```text
git add .code-harness/tools-runtime/go.mod .code-harness/tools-runtime/go.sum .code-harness/tools-runtime/internal/dbguard
git commit -m "feat: enforce readonly SQL with AST validation"
```

---

### Task 4: MySQL Runtime and Database Evidence Artifact

**Files:**
- Create: `.code-harness/contracts/database-evidence.schema.json`
- Create: `.code-harness/tools-runtime/internal/dbmysql/client.go`
- Create: `.code-harness/tools-runtime/internal/dbmysql/client_test.go`
- Create: `.code-harness/tools-runtime/internal/dbevidence/evidence.go`
- Create: `.code-harness/tools-runtime/internal/dbevidence/evidence_test.go`
- Modify: `.code-harness/tools-runtime/go.mod`
- Modify: `.code-harness/tools-runtime/go.sum`

**Interfaces:**

```go
type QueryRequest struct {
    RunID   string
    QueryID string
    Purpose string
    SQL     string
    Params  []any
}

type QueryResult struct {
    QueryID       string
    RunID         string
    Purpose       string
    Schema        string
    StatementType string
    Columns       []string
    Rows          []map[string]any
    RowCount      int
    Truncated     bool
    DurationMs    int64
}

func (c *Client) Ping(ctx context.Context) error
func (c *Client) ListTables(ctx context.Context, schema string) ([]string, error)
func (c *Client) DescribeTable(ctx context.Context, schema, table string) ([]Column, error)
func (c *Client) QueryReadonly(ctx context.Context, req QueryRequest) (QueryResult, error)
func WriteEvidence(root string, result QueryResult) (string, error)
```

- [ ] **Step 1: Add dependencies and failing unit tests**

Add:

```text
github.com/go-sql-driver/mysql v1.9.3
github.com/DATA-DOG/go-sqlmock v1.5.2
```

Tests must cover:

```text
Ping success/failure
ListTables constrained to allowed schema
DescribeTable
Query uses params
ReadOnly transaction
Context timeout
500 rows with maxRows=100 -> 100 returned + truncated=true
sensitive columns are redacted before evidence write
password never serialized
```

- [ ] **Step 2: Run and confirm failure**

```text
cd .code-harness/tools-runtime
go test ./internal/dbmysql ./internal/dbevidence -count=1
```

Expected: FAIL because implementation does not exist.

- [ ] **Step 3: Implement MySQL connection boundary**

Build DSN only inside `dbmysql`; never return DSN or password.

Every data query must:

```text
context.WithTimeout(config timeout)
-> BeginTx(ReadOnly=true)
-> parameterized query
-> read at most maxRows+1
-> rollback/close transaction after read
```

If the extra row exists, set `truncated=true` and drop the extra row.

- [ ] **Step 4: Implement schema discovery**

`ListTables` and `DescribeTable` must verify requested schema/table through safe identifiers and `allowedSchemas` before sending SQL to MySQL.

Do not concatenate arbitrary user strings; identifiers must pass strict MySQL identifier validation (`[A-Za-z0-9_$]+`) and schema allowlist.

- [ ] **Step 5: Implement evidence sanitization**

Before writing `database-evidence`:

```text
column name case-insensitively contains
password | passwd | token | secret | accesskey | privatekey
-> value becomes "***REDACTED***"
```

Evidence path:

```text
.code-harness/runs/<runId>/evidence/db/<queryId>.json
```

Validate the produced JSON against `database-evidence.schema.json` before treating the write as successful.

- [ ] **Step 6: Run tests**

```text
cd .code-harness/tools-runtime
go test ./internal/dbmysql ./internal/dbevidence -count=1
go test ./... -count=1
```

Expected: PASS with no real external DB required.

- [ ] **Step 7: Commit**

```text
git add .code-harness/contracts/database-evidence.schema.json .code-harness/tools-runtime/internal/dbmysql .code-harness/tools-runtime/internal/dbevidence .code-harness/tools-runtime/go.mod .code-harness/tools-runtime/go.sum
git commit -m "feat: add mysql readonly evidence runtime"
```

---

### Task 5: Controlled DB Runtime Commands and Query Skill

**Files:**
- Modify: `.code-harness/tools-runtime/cmd/codea-harness-tools/main.go`
- Add/Modify test: `.code-harness/tools-runtime/cmd/codea-harness-tools/main_test.go`
- Create: `.code-harness/skills/query-database/SKILL.md`
- Modify: `.code-harness/tools/README.md`
- Modify: `.code-harness/agents/runtime-debugger.md`

**Interfaces:**

Runtime entrypoints:

```text
codea-harness-tools.exe db ping --run-id <id>
codea-harness-tools.exe db list-tables --schema <schema> --run-id <id>
codea-harness-tools.exe db describe-table --schema <schema> --table <table> --run-id <id>
codea-harness-tools.exe db query --input .code-harness/runs/<runId>/requests/<file>.json
```

Tool Contract:

```text
db_ping(runId)
db_list_tables(schema, runId)
db_describe_table(schema, table, runId)
db_query_readonly(sql, params, runId, purpose)
```

- [ ] **Step 1: Write failing command tests**

Test without real DB by injecting/stubbing the internal DB runner:

```text
unknown db action -> nonzero
query input outside .code-harness/runs -> nonzero
query missing runId/queryId/purpose -> nonzero
production database config -> nonzero before connect
write SQL -> nonzero before connect
valid query -> structured JSON result + evidence path
```

- [ ] **Step 2: Run and confirm failure**

```text
cd .code-harness/tools-runtime
go test ./cmd/codea-harness-tools -count=1
```

Expected: FAIL until `db` command exists.

- [ ] **Step 3: Add `db` command dispatch**

Update root usage to:

```text
codea-harness-tools <upgrade|validate|nav|db>
```

`db query` must read structured request JSON from `.code-harness/runs/**`; raw SQL must not be accepted as a CLI flag.

- [ ] **Step 4: Implement query budget**

Before executing `db_query_readonly`, count existing DB evidence artifacts for this run/diagnosis scope. If count reaches `maxQueriesPerDiagnosis`, return a deterministic `QUERY_BUDGET_EXCEEDED` error and do not connect.

- [ ] **Step 5: Write `query-database` Skill**

Required behavior:

```text
only Runtime Debugger may execute data queries
schema exploration before guessing table names
use params for data values
state a purpose for every query
prefer minimum columns/rows needed
stop querying when evidence is sufficient
never attempt writes
```

Database disabled/missing config must return `DATABASE_EVIDENCE_UNAVAILABLE` and allow non-DB diagnosis to continue.

- [ ] **Step 6: Update Tool Contract docs**

Document:

```text
DB tools are controlled runtime operations
Agent never invokes mysql.exe
Agent never reads database.yaml
Agent cannot bypass dbguard
```

- [ ] **Step 7: Run tests**

```text
cd .code-harness/tools-runtime
go test ./cmd/codea-harness-tools -count=1
go test ./... -count=1
go vet ./...
```

Expected: PASS.

- [ ] **Step 8: Commit**

```text
git add .code-harness/tools-runtime/cmd .code-harness/skills/query-database .code-harness/tools/README.md .code-harness/agents/runtime-debugger.md
git commit -m "feat: expose controlled database evidence tools"
```

---

### Task 6: Failure Code Navigation and Evidence-Backed Diagnosis

**Files:**
- Modify: `.code-harness/contracts/diagnosis.schema.json`
- Modify: `.code-harness/skills/analyze-failure/SKILL.md`
- Modify: `.code-harness/agents/runtime-debugger.md`
- Modify: `.code-harness/agents/fix-agent.md` only if needed to consume new fields without changing Fix Plan semantics
- Test: `.code-harness/tools-runtime/internal/schema/schema_test.go`

**Interfaces:**

Diagnosis adds optional:

```json
{
  "suspectSymbols": ["OrderServiceImpl.approve"],
  "codeEvidence": [{
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

- [ ] **Step 1: Add failing Diagnosis contract tests**

Valid case: PRODUCTION_CODE_ERROR with test/log/db/code evidence.

Invalid cases:

```text
lineStart <= 0
lineEnd < lineStart
empty suspect symbol
empty database evidence id
```

- [ ] **Step 2: Run and confirm failure**

```text
cd .code-harness/tools-runtime
go test ./internal/schema -run Diagnosis -count=1
```

Expected: FAIL until schema is extended.

- [ ] **Step 3: Extend Diagnosis schema without changing existing enums**

Do not rename/remove existing classifications or nextAction values.

- [ ] **Step 4: Upgrade analyze-failure tool permissions**

Allow exactly:

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

DB tools are conditional on database capability.

- [ ] **Step 5: Encode deterministic investigation order**

`analyze-failure/SKILL.md` must require:

```text
1. extract failedTests from Surefire
2. extract exception/message/stack/file:line/traceId
3. identify project-internal suspectSymbols
4. when file:line exists -> read_code is mandatory
5. interface -> find_implementations
6. upstream uncertainty -> find_references
7. downstream uncertainty -> find_symbol/read_code
8. DB symptom -> bounded DB evidence
9. stop as soon as evidence proves root cause
10. budget exhausted -> UNKNOWN
```

Fixed non-DB budgets:

```text
max navigation hops = 6
max source/test files read = 30
```

- [ ] **Step 6: Preserve role boundaries**

Runtime Debugger still must not:

```text
modify tests
modify production code
approve plans
rerun tests autonomously outside existing nextAction/orchestrator loop
```

Fix Agent may consume `codeEvidence` / `databaseEvidence` as input evidence but still requires exact `批准 <fixPlanId>`.

- [ ] **Step 7: Manual Golden diagnosis acceptance**

Use synthetic evidence fixtures to verify:

```text
stack -> internal file:line -> read_code -> PRODUCTION_CODE_ERROR
interface -> find_implementations -> read implementation
db status + rollback log + code path -> combined root cause
external RPC -> externalDependencies and stop
budget exhausted -> UNKNOWN
```

- [ ] **Step 8: Run tests**

```text
cd .code-harness/tools-runtime
go test ./internal/schema -count=1
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit**

```text
git add .code-harness/contracts/diagnosis.schema.json .code-harness/skills/analyze-failure .code-harness/agents/runtime-debugger.md .code-harness/agents/fix-agent.md .code-harness/tools-runtime/internal/schema
git commit -m "feat: add evidence-backed failure navigation"
```

---

### Task 7: Integration Test Database Assertions and Full Orchestrator Flow

**Files:**
- Modify: `.code-harness/skills/design-integration-tests/SKILL.md`
- Modify: `.code-harness/skills/generate-integration-tests/SKILL.md`
- Modify: `.code-harness/agents/integration-test-agent.md`
- Modify: `.code-harness/agents/orchestrator.md`
- Modify: `.code-harness/skills/run-integration-tests/SKILL.md`

**Interfaces:**
- Consumes validated `TestTargetSelection`.
- Uses existing `test-plan.schema.json expected.databaseAssertions[]`.
- Produces selected-only test class list for Runtime Debugger.

- [ ] **Step 1: Update design-integration-tests rules**

For targets with ChangeAnalysis risks:

```text
databaseWrite
transactional
stateTransition
```

require the Agent to explicitly decide whether DB Assertion is needed.

When needed, `expected.databaseAssertions[]` must describe concrete business state, e.g.:

```text
order_info.status == APPROVED for order_id=<fixture id>
audit_log contains one APPROVE record
```

- [ ] **Step 2: Update generation rules**

DB assertions must follow existing project conventions in this order:

```text
existing test helper/repository pattern
existing JdbcTemplate pattern
existing fixture/assertion utility
```

Do not add a new Maven dependency solely for DB assertion.

DB assertion executes inside the integration test before test cleanup/rollback hides evidence.

- [ ] **Step 3: Make selected-only execution explicit**

`run-integration-tests` can only receive test classes derived from validated selected targets.

If a proposed execution class belongs only to an unselected Controller, Orchestrator must stop and report a scope violation.

- [ ] **Step 4: Preserve Existing Test rules**

```text
REUSE_EXISTING -> run, no approval, no modification
EXTEND_EXISTING -> only MISSING + exact plan approval
CREATE_NEW -> exact plan approval
historical Existing Test failure -> never auto-edit
GENERATED_BY_PLAN repair -> max 2 rounds
```

- [ ] **Step 5: Manual end-to-end behavior check**

Scenario:

```text
3 affected Controllers
-> select Order + Payment
-> Order REUSE_EXISTING
-> Payment EXTEND_EXISTING
-> User unselected
-> approve Payment plan
-> execute only Order/Payment tests
-> failure triggers Runtime Debugger
```

Expected: no User test design/execution artifact exists.

- [ ] **Step 6: Commit**

```text
git add .code-harness/skills/design-integration-tests .code-harness/skills/generate-integration-tests .code-harness/skills/run-integration-tests .code-harness/agents/integration-test-agent.md .code-harness/agents/orchestrator.md
git commit -m "feat: integrate selected targets with database assertions"
```

---

### Task 8: Upgrade, Packaging, Version and Final Golden Acceptance

**Files:**
- Modify: `.code-harness/tools-runtime/internal/upgrade/upgrade.go`
- Modify: `.code-harness/tools-runtime/internal/upgrade/upgrade_test.go`
- Modify: `.github/workflows/package-windows-x64.yml`
- Modify: `.code-harness/tools/README.md`
- Modify: `README.md`
- Modify: `.code-harness/VERSION` or repository VERSION path used by current package
- Verify: `.code-harness/.gitignore`

**Interfaces:**
- Framework Managed includes `database.template.yaml` and new contracts/skills/runtime files.
- Project State includes `database.yaml`.

- [ ] **Step 1: Add failing upgrade regression tests**

Golden cases:

```text
1.1.1 -> 1.2.0 with no database.yaml -> UPGRADED
1.1.1 -> 1.2.0 with database.yaml -> byte-preserved
new database.template.yaml installed
stale framework file removed
runs/** preserved
harness.yaml preserved except registered migrations
project.md preserved
database config invalid -> upgrade still succeeds because DB config is optional Project State
```

- [ ] **Step 2: Run upgrade tests and confirm failure where new semantics are missing**

```text
cd .code-harness/tools-runtime
go test ./internal/upgrade -count=1
```

- [ ] **Step 3: Update Framework/Project State classification**

`database.yaml` must never be included in `removeManaged`, `copyManaged`, stale delete, backup cleanup logic that treats Framework files as replaceable.

`database.template.yaml` must be included in package completeness and Framework replace.

- [ ] **Step 4: Extend Windows packaging workflow**

Add steps after existing 1.1.1 gates:

```text
Database config schema smoke
SQL Guard Golden test
DB CLI no-config graceful failure smoke
package completeness: database.template.yaml exists
package leak check: database.yaml absent
```

Do not remove existing:

```text
Test controlled runtime
Build Windows x64 runtime
Vendor pinned ast-grep Windows x64
Code Navigation end-to-end smoke
Windows live upgrade lifecycle smoke
Assert package completeness
Build offline package
```

- [ ] **Step 5: Update README usage**

Document:

```text
harness test multi-controller selection
copy database.template.yaml -> database.yaml to enable DB Evidence
database.yaml is local-only and ignored
db evidence is optional
Agent DB capability is readonly TEST/LOCAL only
```

- [ ] **Step 6: Set version to 1.2.0 only after all prior tasks pass**

Update the repository's canonical Harness version file to:

```text
1.2.0
```

Do not change version earlier in development.

- [ ] **Step 7: Run complete local verification**

```text
cd .code-harness/tools-runtime
go test -count=1 ./...
go vet ./...
GOOS=windows GOARCH=amd64 go build ./cmd/codea-harness-tools
```

Expected: PASS.

- [ ] **Step 8: Run Final Golden Acceptance**

All must pass:

```text
A. single Controller auto-select
B. multi Controller selection
C. DIRECT_ONLY
D. selection != approval
E. unselected target never designed/executed
F. database disabled graceful degradation
G. database TEST config valid
H. production config rejected before connection
I. complex SELECT/JOIN/CTE accepted
J. UPDATE/DELETE/DDL/multi-statement/lock/outfile/cross-schema rejected
K. maxRows/timeout/query budget enforced
L. evidence sanitized and persisted
M. failure stack navigates to actual code
N. DB + log + code combined Diagnosis
O. external dependency stop
P. diagnosis budget exhausted -> UNKNOWN
Q. upgrade preserves database.yaml
R. package excludes database.yaml
S. Windows live upgrade lifecycle still success
```

- [ ] **Step 9: Commit**

```text
git add .code-harness .github/workflows/package-windows-x64.yml README.md
git commit -m "feat: release Codea Harness 1.2.0"
```

---

## Final Reviewer Acceptance Rules

Reviewer must return **FAIL + all blockers at once** if any of the following occur:

1. Multi-controller flow can bypass user selection.
2. Unselected Controller can reach Test Plan or Runtime execution.
3. Selection can be mistaken for plan/fix approval.
4. Agent can read database password or `database.yaml` through ordinary read tools.
5. Agent can execute SQL outside controlled Runtime.
6. SQL safety decision is regex/prefix based instead of AST based.
7. Any write/multi-statement/locking/outfile/cross-schema query can reach DB execution.
8. DB query lacks timeout, row cap or query budget.
9. DB failure blocks projects that have database evidence disabled.
10. Runtime Debugger declares production root cause from stack/log text without reading referenced internal code when code is available.
11. Debug Navigation performs unbounded repository scan.
12. `database.yaml` is packaged, tracked, overwritten or deleted by upgrade.
13. Existing 1.1.1 Review/Approval/Existing Test/Windows lifecycle gates regress.

Only after all Task 1-8 gates and the real Windows workflow are green may Codea Harness 1.2.0 be marked `PASS`.
