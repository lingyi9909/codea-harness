# Code Harness Web Workbench

The local review workbench serves the React application and Fastify API on the same loopback port. It stores tasks, runs, messages, native checkpoints, permissions and report metadata in MySQL 8.4. Git snapshots and archived report files live under the configured data directory.

## Start the local workbench

Use Node 22 and the locked npm dependencies. Set `HARNESS_MYSQL_HOST`, `HARNESS_MYSQL_PORT`, `HARNESS_MYSQL_DATABASE`, `HARNESS_MYSQL_USER` and `HARNESS_MYSQL_PASSWORD` for the workbench database. Set `HARNESS_REPOSITORY_ROOTS` to the allowed repository locations, `HARNESS_DATA_DIR` to writable storage, and `HARNESS_REF_KEY` and `HARNESS_CURSOR_KEY` to stable secret values of at least 32 characters.

```text
npm ci --ignore-scripts --no-audit --no-fund
npm run web:build
npm run server:start
```

Apply the versioned migrations with the migration account before starting the service. Set `HARNESS_MIGRATION_CONFIRM_DATABASE` to the exact database name, set `HARNESS_MIGRATION_USER` and `HARNESS_MIGRATION_PASSWORD`, then run `npm run migrate`. Open the address printed by the service and enter the pairing code from its console.

The default launcher supports repository registration, preview and browsing persisted evidence. Native dispatch requires the pinned company build integration. Its local launcher can call `startWorkbench(native)` with the verified adapter, version, worker identity, repository roots and data directory. Point the API and worker to the same storage and roots. No HTTP endpoint accepts an adapter, a capability override or an observation decoder.

## Native execution and recovery

Supply build-specific decoders for attributable public observations. Tool summaries, public assistant text and verified terminal/write evidence are persisted separately from raw native frames. Live-only builds also require a message decoder and a terminal-state decoder. They record a continuity gap and reconcile from snapshots. Durable streams resume from a native cursor committed with the corresponding writes.

The worker records control sends before contacting the native service. An uncertain answer or interrupt stays out of the send queue. An expired lease quarantines the slot. Automatic reattachment requires an already accepted input, matching native build, independent read-only proof of the bound session and input, an external-writer audit, and a fresh lease fence. Uncertain sends and unresolved interactions stay quarantined for investigation.

## Verification

- `npm run typecheck`, `npm run web:typecheck` and `npm run web:build` check both applications.
- `npm run test:mysql` migrates and writes fixture records to the dedicated disposable database `harness_ci_test`. It exercises native event replay, Unicode message deltas, control delivery and run reconciliation with a scripted native peer.
- After building the Web app, migrating the disposable database and installing Playwright Chromium, `npm run test:e2e` drives the browser against Fastify, Git and MySQL. The review, permission and model output in this test come from a scripted adapter.
- The Windows workflows run the adapter, storage-contract, Git snapshot and application regressions. The separate Linux job runs the MySQL 8.4 service and Chromium.
- T13 must capture actual company Windows/OpenCode build, authentication, protocol mapping and private-model evidence. See [the Windows runbook](docs/windows-t01-runbook.md) and [the capability matrix](docs/capability-matrix.md).

## 内网使用多场景工作台与 OpenCode 对话

首页提供直接对话、代码审查、日志排查和数据分析。日志页按关键词与级别筛选本地 UTF-8 文本；数据页读取 CSV，按列统计数值。两个页面在浏览器中处理所选文件。

直接对话使用官方 `@opencode/client@2.0.27`。在能访问同一工作目录的机器上运行工作台和公司 OpenCode 服务，配置原生服务使用公司的 Agent、模型和工具权限。以下 PowerShell 示例在 `web-workbench` 目录执行，地址和路径换成实际值。

```powershell
$env:HARNESS_OPENCODE_URL = "http://127.0.0.1:4096"
$env:HARNESS_CHAT_ROOTS = "D:\projects\demo;D:\projects\service"
npm ci --ignore-scripts --no-audit --no-fund
npm run web:build
npm run server:start -- --local-only
```

OpenCode 要求鉴权时，通过服务端环境变量 `HARNESS_OPENCODE_AUTHORIZATION` 提供完整的 Authorization 请求头值。远程地址使用 HTTPS。工作台和 OpenCode 需要看到同一绝对路径。控制台会显示网页地址和一次性配对码。`--local-only` 支持直接对话和两个本地分析页面，已有 MySQL 的完整部署沿用 `npm run server:start`。

配对后打开“直接对话”，选择工作目录，输入“请读取 README 并总结启动步骤”。“引用文件”支持当前目录内的相对路径，每行一个。工作台将规范文件 URI 传给 OpenCode；读取、编辑、写入均由 OpenCode 在原生会话中执行。工作台展示公开回复、工具状态、涉及的文件及待审批资源。选择“仅本次允许”或“拒绝”会答复该会话的原生权限请求。目录列表限制工作台可以绑定的目录及显式引用；工具实际能访问的位置由公司 OpenCode 的权限策略决定。

内网验收建议使用一个临时工作目录。先建一份文本文件，让 OpenCode 读出其中一个值，再要求它修改该值，直接打开磁盘文件核对。刷新网页后继续在同一会话追问；最后验证拒绝授权和中断执行。`npm run test:chat` 用临时 HTTP 对端检查官方客户端的协议调用，不启动模型。
