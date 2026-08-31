---
name: query-database
description: Runtime Debugger 在测试/服务失败诊断中通过受控 Runtime 探索 TEST/LOCAL MySQL 并生成只读 Database Evidence；禁止直接读取凭据或绕过安全门禁。
version: 1
agent: runtime-debugger
tools:
  - db_ping
  - db_list_tables
  - db_describe_table
  - db_query_readonly
---

# Query Database

## 使用前提

仅供 Runtime Debugger 在已有失败证据提出**具体数据库诊断问题**时使用。Database Evidence 是可选能力：未配置、disabled 或不可用时，记录 `DATABASE_EVIDENCE_UNAVAILABLE`，继续使用 test report / logs；不得仅因为 DB unavailable 自动判定测试失败类型。

## 受控工具

```text
db_ping(runId)
db_list_tables(schema, runId)
db_describe_table(schema, table, runId)
db_query_readonly(sql, params, runId, purpose)
```

这些逻辑工具只能映射到 `.code-harness/bin/codea-dcep-tools.exe db ...`。Agent 不得直接执行 runtime binary 参数拼接；Host/Runtime 负责把 `db_query_readonly` 请求序列化为：

```text
.code-harness/runs/<runId>/requests/<file>.json
```

再调用固定的 `db query --input <path>`。

## 调查顺序

1. 先从测试报告、日志、代码/映射信息形成明确问题，例如“失败后 order_info.status 是否仍为 PENDING”。
2. 表/字段未知时先 `db_list_tables` / `db_describe_table`，禁止猜表名或字段名。
3. 已知表结构后，Agent 可自主构造 `SELECT`、`JOIN`、subquery、CTE SELECT。
4. 动态值必须使用 `?` 参数，放入 `params`；不得把运行时值拼进 SQL 文本。
5. 每个 query 必须填写非空 `purpose`，说明它要回答的具体诊断问题。
6. Evidence 足够后立即停止查询；不要为了“多收集信息”继续调用 DB。

## 安全边界

- 永远不得执行 INSERT / UPDATE / DELETE / DDL / CALL / SET / locking SELECT / OUTFILE / 多语句。
- 永远不得访问 TEST / LOCAL 之外的数据库环境。
- 永远不得查询 allowedSchemas 之外的 schema。
- 永远不得调用 `mysql.exe` 或任意 Shell。
- 永远不得通过普通 read tool 读取 `.code-harness/database.yaml`。
- 永远不得复制、打印、总结数据库密码。
- 永远不得绕过 `dbguard`；SQL 是否可执行只由 controlled Runtime 的 AST Gate 决定。
- 每个 run 的自动查询数量受 `maxQueriesPerDiagnosis` 限制；达到上限必须接受 `QUERY_BUDGET_EXCEEDED` 并停止继续 DB 查询。

## Evidence

成功的 `db_query_readonly` 必须形成：

```text
.code-harness/runs/<runId>/evidence/db/<queryId>.json
```

Evidence 由 Runtime 完成 timeout、row cap、敏感列脱敏与 Contract validation。后续 Diagnosis 只能引用已落盘 Evidence，不得把未执行的 SQL 当成证据。
