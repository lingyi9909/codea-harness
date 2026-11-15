# 智能工作台 · Web Workbench

The local review workbench serves the React application and Fastify API on the same loopback port. It stores tasks, runs, messages, native checkpoints, permissions and report metadata in MySQL 8.4. Git snapshots and archived report files live under the configured data directory.

## Node.js and npm compatibility

The workbench supports Node.js 22.16+ (22.x), 24.x and 26.x, with npm 10 or 11. On macOS, the local-only chat launcher does **not** need MySQL, but the React assets **must** be built before launch:

```sh
cd web-workbench
npm ci --ignore-scripts --no-audit --no-fund
npm run web:build
./启动工作台.command "/Users/me/Projects/workbench"  # required startup workspace
```

Visit http://127.0.0.1:4176/. A missing build now produces `FRONTEND_BUILD_MISSING` instead of the misleading `Route GET:/ not found`. To verify the local HTTP homepage without OpenCode or MySQL, run `npm run smoke:local` after building. CI tests this on macOS and Windows with Node 22, 24 and 26. Real company OpenCode, model and MySQL integration remain separate acceptance requirements.

## Start the local workbench

Use one of the supported Node.js versions and the locked npm dependencies. Set `HARNESS_MYSQL_HOST`, `HARNESS_MYSQL_PORT`, `HARNESS_MYSQL_DATABASE`, `HARNESS_MYSQL_USER` and `HARNESS_MYSQL_PASSWORD` for the workbench database. Set `HARNESS_REPOSITORY_ROOTS` to the allowed repository locations, `HARNESS_DATA_DIR` to writable storage, and `HARNESS_REF_KEY` and `HARNESS_CURSOR_KEY` to stable secret values of at least 32 characters.

```text
npm ci --ignore-scripts --no-audit --no-fund
npm run web:build
npm run server:start -- --workspace "/absolute/path/to/workbench"
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

启动智能工作台时 **必须明确指定一个已经存在的工作目录**。它是工作台配置与登录资料的存储根，不等于所有子场景的执行目录。

- Windows：`start-chat.cmd "D:\\projects\\my-workbench"`
- macOS：`./启动工作台.command "/Users/me/Projects/my-workbench"`
- Node CLI：`npm run server:start -- --workspace "/absolute/path/to/my-workbench"`（如使用本地模式，可添加 `--local-only`）

路径支持空格，建议使用绝对路径。省略或指向不存在的目录会拒绝启动，不会回退到 `chat-files`。更换启动目录后，需关闭旧进程、使用新目录重新启动。

首页仍然是 OpenCode 原生对话；侧栏包括代码审查，以及「问题排查 → 公管问题排查 / 报文问题排查」「数据分析 → 公管数据分析」。公管数据分析已经改为直接对话，不再使用旧版 CSV 浏览器统计表单。代码审查继续保留原有固定 Git 版本执行、报告和授权流程。

两个问题排查表单只包含开始时间、结束时间、Trace ID、关键词。时间范围必须有效，Trace ID 和关键词至少填写一项。可使用最近 15 分钟、1 小时、24 小时预设，也可手动选择时间。起止时间以当前浏览器时区传给执行入口，范围为包含起点、不包含终点。无日志级别、文件上传、自动 SQL 查询等隐式参数。

### 配置文件与安全边界

启动时会在工作目录下自动创建 **`.workbench/scenarios.json`**（初始 `{"version":1,"scenes":{}}`）和 **`.workbench/config-access.json`**（初始 `{"version":1,"editors":[]}`），**不会自动授予编辑权限**。本地登录成功后才生成或更新 **`.workbench/user.json`**，因此未登录时它尚不存在。这些文件位于启动目录而非代码仓库；macOS Finder 中以点开头的目录默认隐藏，可按 Command+Shift+. 显示，或通过 `ls -la` 查看。配置页面现在显示两份配置文件的绝对路径。

通过「场景配置」页面保存 `.workbench/scenarios.json`。此文件将子场景 ID 映射到**绝对路径**工作目录和原生执行入口，示例如下（名称只是示例，须在对应目录的 OpenCode 2.x 中真实可用）：

```json
{
  "version": 1,
  "scenes": {
    "logs": {
      "directory": "D:\\\\projects\\\\public-service",
      "executor": {"type": "skill", "name": "log-query"}
    },
    "message": {
      "directory": "D:\\\\projects\\\\message-service",
      "executor": {"type": "subagent", "name": "message-diagnoser"}
    },
    "data": {
      "directory": "D:\\\\projects\\\\public-data",
      "executor": {"type": "command", "name": "public-analysis"}
    }
  }
}
```

三个配置键：`logs` 对应公管问题排查，`message` 对应报文问题排查，`data` 对应公管数据分析。可以分别绑定不同目录。**未配置场景不允许执行**，不会默认跑到首页目录。所有绑定必须为已存在的绝对目录；服务端按实际文件解析路径并检查。Skill、命令和 Agent 通过 OpenCode 的目录级 catalog 验证，不存在、重名或接口不可用时不执行。Command 通过原生 `session.command` 提交而不是把斜杠命令当普通提示词。Subagent 经原生 Agent catalog 验证并附加 Agent 提及；是否真正创建子会话和运行结果须以实际 OpenCode 工具轨迹为准，不能把文本提及视为已经执行成功。

配置页面的**编辑权限**独立保存在启动目录的 **`.workbench/config-access.json`**。首次使用时系统默认只读，不会自动授权登录用户。启动时权限文件已创建，默认 `editors` 是空数组。打开配置页可以看到当前身份标识（SHA-256 字符串）；请在本机文件系统中由受信任的管理员**编辑已有权限文件**，将该标识加入 `editors`：

```json
{
  "version": 1,
  "editors": ["从配置页面复制的当前身份标识"]
}
```

保存权限文件后刷新配置页面。前端会根据 `canEdit` 切换编辑状态；服务端对每次保存独立校验会话、CSRF、ACL、配置结构、目录存在性和文件版本。**不能在 Web 页面修改此权限文件**，防止自我提权。编辑的是本机工作区配置，权限基于本地登录资料，不代表企业级统一身份认证；登录手机号没有短信核验，使用者必须控制本机目录和服务的访问权限。

`.workbench/user.json` 包含本机个人资料，请勿将这些文件与内部业务路径和用户资料提交到公共 Git 仓库。场景配置文件和权限文件可包含敏感路径与身份标识，同样应排除在共享版本控制之外。建议工作台启动目录与受控业务仓库分离。

### 对话与执行

每个子场景使用自身的已绑定工作目录获取模型、Skill、Command、Agent 和文件引用。配置缺失时，页面会明确显示「当前子场景尚未配置」，不会试图在默认目录执行；页面的「重新检查配置」可在保存配置后重新读取。网络无法读取配置时会单独提示本地服务连接问题并允许重试。历史会话、草稿与时间范围按场景区分；旧的公管日志会话保留既有 `harness.logs` 存储键。切换场景不自动创建 OpenCode 会话，当前会话也不会因为管理员修改配置而无提示迁移目录；执行时仍在服务端检查当前绑定。

模型选择、流式输出、工具执行、授权等待、原生表单补参、中断和恢复与之前一致。等待权限时仍只有「拒绝」「仅本次允许」，不会新增自动授权。测试中的模拟工具或会话输出不是企业内网真实运行证据。

### 登录资料供项目 Skill 读取

使用打包版启动后，在登录页填写姓名和手机号。登录成功后，工作台把当前使用者的信息写入**启动项目目录**下的 `.workbench/user.json`。例如用 `start-chat.cmd "D:\projects\order-service"` 启动时，文件位于 `D:\projects\order-service\.workbench\user.json`。项目需要对当前系统用户可写。

Skill 需要使用姓名或手机号时，从项目根目录按 UTF-8 读取该文件，解析 `name` 和 `phone` 两个字符串字段。以下为示例数据。

```json
{"name":"示例用户","phone":"+12025550123"}
```

这是本机使用者自行填写的资料。手机号不经过短信验证。退出登录会结束网页会话，文件保留供 Skill 使用；下次成功登录会更新文件。请将这份个人资料排除在项目提交和共享归档之外。日志排查中由内网业务发起的验证码仍在对应的 OpenCode 提问卡内提交。
