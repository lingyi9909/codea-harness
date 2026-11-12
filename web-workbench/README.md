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

Windows 离线包解压后运行 `start-chat.cmd`，打开本机工作台地址即可进入“直接对话”。第一次发送消息时，工作台通过官方客户端发现同一用户已注册的 OpenCode 2.x 后台，并使用注册的认证信息。离线包在发现失败时尝试使用已安装的 `opencode serve --service` 启动后台。公司版本需要支持这个服务协议。已有部署仍可使用服务端的 `HARNESS_OPENCODE_URL` 和 `HARNESS_OPENCODE_AUTHORIZATION`。

启动智能工作台时指定已有项目文件夹。Windows 可运行 `start-chat.cmd "D:\projects\order-service"`，macOS 可在启动脚本旁运行 `./启动工作台.command "/Users/me/Projects/order-service"`。路径包含空格时保留引号。省略路径时使用已有的目录环境配置，未配置则使用包内的 `chat-files`。更换项目时关闭工作台，用新路径重新启动。直接对话、文件引用、模型列表、项目 Skill 和会话历史都使用启动目录。

新会话按用户首条输入概括命名，后续追问沿用原题。可从“历史对话”切回原会话。升级后用原项目目录启动即可查看该目录的历史记录。对话框左下角可选择当前项目已启用的模型。

`npm run test:chat` 使用临时注册文件和模拟 HTTP 服务验证发现、会话、文件引用、权限、中断和错误处理。

直接对话和日志排查按用户提问组织回复。回答正文支持流式 Markdown。点击这轮的“执行过程”可以查看工具调用、公开思考片段、进度、重试与结束状态，再展开单个工具查看参数和结果。工具输出保留在对应轮次。执行失败或中断会在正文位置附近提示，待回答的表单仍显示在输入区。

日志排查支持按最近 15 分钟、1 小时或 24 小时选取时间，也可手动指定本地起止时间。工作台将带时区偏移的起止时间传给 OpenCode，查询范围包含开始时刻，到结束时刻为止（不含）。发送后可核对本次使用的范围。对话框左下角可搜索、收藏、切换当前项目已启用的模型。网页的收藏与最近使用记录保存在当前浏览器。

当当前会话发来提问或权限请求时，输入区上方会显示待办，浏览器标签页也会提示等待回应。提交回答、取消提问、本次允许或拒绝会回传给同一个 OpenCode 会话。直接对话和日志排查使用相同的交互控件。

助手回复与思考文本支持 Markdown 标题、引用、表格、任务列表和代码块。排版样式取自 [OpenCode Desktop v2.2.7](https://github.com/anomalyco/opencode/blob/v2.2.7/packages/session-ui/src/components/markdown.css)，React 渲染层适配本工作台，代码高亮及样式随网页离线分发。

日志排查绑定当前项目加载的自定义 Skill `log-query`。工作台按名称定位它，并在首轮及每次追问的 OpenCode prompt 的 `skills` 字段指定实际 ID。该 Skill 由内网项目提供。缺失、同名冲突或目录查询失败会保留用户输入并提示原因。

使用打包版的本机启动脚本时，浏览器自动建立本机会话。日志排查中的验证码由内网业务发起，填写后提交给发起提问的 OpenCode 表单。
