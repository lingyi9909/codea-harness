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

首页提供直接对话、代码审查、日志排查和数据分析。日志排查支持按时间区间和 Trace ID 或关键字查询，通过独立的 OpenCode 会话排查和追问。可导入 UTF-8 日志并预览按时间、级别和文本筛选的行；点击开始排查时发送匹配摘录。没有导入文件时由当前项目可用的日志工具提供数据。数据页在浏览器本地读取 CSV，按列统计数值。

Windows 离线包解压后运行 `start-chat.cmd`，在浏览器配对并进入“直接对话”。第一次发送消息时，工作台通过官方客户端发现同一用户已注册的 OpenCode 2.x 后台，并使用注册的认证信息。离线包在发现失败时尝试使用已安装的 `opencode serve --service` 启动后台。公司版本需要支持这个服务协议。已有部署仍可使用服务端的 `HARNESS_OPENCODE_URL` 和 `HARNESS_OPENCODE_AUTHORIZATION`。

相对路径和引用文件默认位于启动包的 `chat-files` 子目录。已有配置优先使用 `HARNESS_CHAT_ROOTS` 或 `HARNESS_REPOSITORY_ROOTS` 的首个目录。OpenCode 继续执行公司的文件权限策略。可把测试文件放入该目录，让 OpenCode 读取或修改。

“后台访问日志”显示当前工作台进程最近 300 条记录，包含服务发现、工作台请求、OpenCode 请求、状态、耗时和关联编号。重启清空记录。认证信息、聊天正文与文件内容不写入访问日志。网页每 2.5 秒刷新一次。源码启动仍需先安装依赖并构建网页。

`npm run test:chat` 使用临时注册文件和模拟 HTTP 服务验证发现、会话、文件引用、权限、中断和错误日志。

网页新会话可点击“选择模型”加载当前目录已启用的模型。选择公司提供方与模型后再发送。公司配置仅在特定项目生效时，可在启动工作台的终端设置 HARNESS_CHAT_ROOTS 为该项目路径。

在“直接对话 → 工作目录 → 更改工作目录”中粘贴已有项目的绝对路径并保存。当前浏览器记住选择，之后模型列表与新会话使用该目录。切换项目会开始新对话。未设置目录范围的本机离线启动模式允许选取已有项目；配置了目录范围时仅允许范围内的目录。

直接对话与日志排查共用执行记录：按服务端消息顺序展示公开说明、Skill、工具输入和结果摘录、失败、重试与结束状态。运行中轮询刷新消息，并通过当前会话的 SSE 接收工具主动报告的进度。刷新后从原生消息恢复记录；瞬时进度只保留当前页面最近 12 条。

当 OpenCode 的助手消息含有 reasoning 段时，网页在该段的原始位置显示“思考过程”。点击可查看服务端向客户端提供的文本，刷新已有会话也可查看。访问日志不记录该文本。

直接对话和日志排查订阅当前会话的原生增量事件。生成时会自动展开本轮思考，并按到达顺序追加思考文本和答复。上滚可以暂停跟随。刷新或重连后以 OpenCode 保存的会话记录为准。

日志排查支持按最近 15 分钟、1 小时或 24 小时选取时间，也可手动指定本地起止时间。发送后会显示本次排查使用的范围。对话框左下角可搜索、收藏、切换当前项目已启用的模型。网页的收藏与最近使用记录保存在当前浏览器。

当当前会话发来提问或权限请求时，输入区上方会显示待办，浏览器标签页也会提示等待回应。提交回答、取消提问、本次允许或拒绝会回传给同一个 OpenCode 会话。直接对话和日志排查使用相同的交互控件。

助手回复与思考文本支持 Markdown 标题、引用、表格、任务列表和代码块。排版样式取自 [OpenCode Desktop v2.2.7](https://github.com/anomalyco/opencode/blob/v2.2.7/packages/session-ui/src/components/markdown.css)，React 渲染层适配本工作台，代码高亮及样式随网页离线分发。

日志排查绑定当前项目加载的自定义 Skill `log-query`。工作台按名称定位它，并在首轮及每次追问的 OpenCode prompt 的 `skills` 字段指定实际 ID。该 Skill 由内网项目提供。缺失、同名冲突或目录查询失败会保留用户输入并提示原因。
