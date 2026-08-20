---
name: analyze-failure
description: 关联 Surefire、日志、数据库证据和 Code Navigation，产出 evidence-backed Diagnosis。
version: 1
agent: runtime-debugger
tools:
  - read_test_report
  - read_service_logs
  - find_symbol
  - find_references
  - find_implementations
  - read_code
  - db_ping
  - db_list_tables
  - db_describe_table
  - db_query_readonly
output_schema: .code-harness/contracts/diagnosis.schema.json
---

# 分析故障

## 目标

从测试报告、stdout/stderr、应用日志、可选 Database Evidence 中提取失败信号，使用受控 Code Navigation 读取实际实现和调用链，在证据充分后产出 Diagnosis。不得只凭日志或模型推断把问题定为生产代码缺陷。

## 输入

- Maven stdout/stderr
- Surefire XML/TXT
- 运行窗口内应用日志
- 执行模式：`integration-test` / `service-debug`
- 当前 run 已存在的 Database Evidence（如有）

## 允许使用的工具

仅允许：

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

## Evidence-first 诊断顺序

1. 从 Surefire 结构化提取 `failedTests`，不得从自然语言 evidence 猜测。
2. 从 report/log 提取 exception、message、stack frame、project file:line、traceId、业务实际值等 Failure Signals。
3. 从 Failure Signals 提取项目内 `suspectSymbols`。
4. Stack Trace 指向项目内 `file:line` 时，在声称代码根因前 **MUST `read_code`** 该位置，并把实际读取范围写入 `codeEvidence`。
5. suspect 是 interface/抽象类型时，先 `find_implementations`，再 `read_code` 实现；不能只看接口猜实现行为。
6. 上游调用者不明确时使用 `find_references`；下游项目内符号不明确时使用 `find_symbol` + `read_code`。
7. report/log 暴露具体数据状态疑问时，可使用 Database Evidence：未知表结构先 list/describe，查询必须走 `db_query_readonly`，每条 query 有 purpose。
8. 到达外部 RPC/HTTP/SDK 边界且没有该服务源码时，将依赖写入 `externalDependencies` 并停止向服务端猜测根因。
9. 证据充分后才输出 Diagnosis；证据仍不足时继续 bounded evidence collection，预算耗尽则 `UNKNOWN / STOP_UNKNOWN`。

## 调查预算（硬限制）

每个 Diagnosis：

```text
navigation hops <= 6
source/test files read <= 30
DB queries <= database.yaml safety.maxQueriesPerDiagnosis
```

达到任何预算后不得继续对应探索。若已有证据仍不足，必须 `classification: UNKNOWN`、`nextAction: STOP_UNKNOWN`。

## 分类规则

原有分类和 nextAction 语义保持不变：

- `TEST_COMPILE_ERROR` → `REPAIR_TEST`
- `TEST_CODE_ERROR` → `REPAIR_TEST`
- `TEST_CONTEXT_ERROR` → `REPAIR_TEST`
- `TEST_DATA_OR_ENVIRONMENT_ERROR` → `REPORT_ENVIRONMENT`（临时性问题可由 Orchestrator 决定 retry）
- `SERVICE_START_ERROR` → 服务调试模式下 `RESTART_SERVICE` 或有代码证据时 `GENERATE_FIX_PLAN`
- `PRODUCTION_CODE_ERROR` → `GENERATE_FIX_PLAN`
- `UNKNOWN` → `STOP_UNKNOWN`

`SERVICE_START_ERROR` 仅服务调试模式有效；集成测试中的 Spring Context 启动失败仍是 `TEST_CONTEXT_ERROR`。

### 生产代码根因硬门禁

要输出：

```text
classification = PRODUCTION_CODE_ERROR
nextAction = GENERATE_FIX_PLAN
```

必须至少满足：

- 失败现象来自 report/log/test expectation；并且
- 对关联项目内实现做过 Code Navigation + `read_code`；并且
- `codeEvidence` 指向实际读取过的实现范围；并且
- 证据链能解释症状与实现行为之间的因果关系。

只有 DB 状态异常、只有日志异常、只有 stack symbol 名称，都不足以单独确认生产代码根因。

典型合法证据链：

```text
expected APPROVED / actual PENDING
+ DB Evidence 确认 order_info.status=PENDING
+ rollback log
+ read_code 读取 OrderServiceImpl.approve 的事务/异常路径
→ PRODUCTION_CODE_ERROR
```

## Diagnosis 输出字段

除既有字段外，可写：

- `suspectSymbols[]`：Failure Signals 提取出的项目内可疑符号。
- `codeEvidence[]`：实际 read_code 的 `path/symbol/lineStart/lineEnd/reason`。
- `databaseEvidence[]`：本次诊断实际使用的已落盘 queryId。
- `externalDependencies[]`：调查到达但无服务端源码证据的外部边界。

所有输出必须通过 `.code-harness/contracts/diagnosis.schema.json` 的 Schema + machine semantic validation。

## Golden 行为

```text
stack -> OrderServiceImpl.java:186 -> read_code -> codeEvidence
interface OrderService -> find_implementations -> implementation read
expected APPROVED/actual PENDING + DB + rollback log + source -> PRODUCTION_CODE_ERROR
PaymentRpcClient -> externalDependencies + stop, no server-side guess
budget exhausted + insufficient evidence -> UNKNOWN/STOP_UNKNOWN
```

## 停止条件

- 已形成充分 evidence-backed Diagnosis。
- 到达外部依赖边界且无可验证服务端源码。
- navigation/file/DB budget 任一耗尽。
- 证据不足且无法继续受控探索 → `UNKNOWN / STOP_UNKNOWN`。

## 禁止行为

- 不得修改测试或生产代码。
- 不得重新运行测试。
- 不得执行任意 Shell。
- 不得读取 `.code-harness/database.yaml` 或数据库 password。
- 不得直接调用 mysql.exe 或绕过 DB Runtime safety gate。
- 不得凭接口名、日志文本或外部依赖名称猜测服务端代码根因。
- 不得生成或批准 Fix Plan；这里只产出 Diagnosis。
