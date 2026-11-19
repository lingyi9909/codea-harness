# 机构报文嵌入页：直接复用工作台原有执行链路

## 默认部署（无需第二套 OpenCode）

直接使用原 `启动工作台.cmd` 启动的 OpenCode 服务和 `.workbench/scenarios.json`。
公管问题排查、报文问题排查、机构报文排查走同一套 `registerChat`、原生 `session.prompt`、Skill/Command/Subagent 分发、SSE、停止与追问 API。
独立机构页只在前端入口、访客会话和权限范围上与普通登录后的工作台不同。

### Skill 如何复用

- 如果 `institution-message` 显式配置了 Skill，就使用它。
- 如果 `institution-message` 仍为默认 `builtin/chat`，而 `message` 已经配置为 Skill，则自动继承原报文排查 Skill 和其工作目录；不用再手动配置一遍。
- 如果上述两个场景都没有配置 Skill，页面会提示缺少业务执行器，不把普通 Chat 的“阶段 1”描述当作已执行查询。
- 不在匿名共享服务内执行任意自定义命令或子 Agent：它们可能以另一套本机权限运行，绕过匿名 Session 的工具限制。如业务确实需要这些能力，须采用可信用户认证与独立执行服务的方案。

### 多人并发

每次页面访问创建随机访问令牌、独立用户归属和原生 Session。不同访客的消息、活动 SSE、模型状态、中断/结束操作互不混用；访问者不能查询其他人的 Session。服务重启后保存可恢复的绑定，但不能自动重复提交原任务。

### 匿名会话安全策略

共享 OpenCode 服务使用 OpenCode V2 的有序规则：默认拒绝；只允许 `skill`、`execute`（Code Mode）、已授权的 `dbhub_search_objects` 和 `dbhub_execute_sql`，以及旧 Database Assistant Agent 中已审核的四条**逐字精确** TeamAI recall 命令：

- `teamai recall "帮我查询数据库"`
- `teamai recall "数据库数据字典和表关联"`
- `teamai recall "数据库业务指标定义和统计口径"`
- `teamai recall "数据库业务流程和历史异常案例"`

默认策略拒绝 `shell:*` 和任意 Node/PowerShell 脚本；管理员可使用下述显式开关单独放行机构页 Shell。自定义 Command/Subagent 仍要求隔离执行环境。

**Code Mode 说明**：原先匿名策略拒绝 `execute`，会导致 V2 的 Code Mode 消失，模型只能看到不完整的工具列表。现在恢复 `execute`，但其嵌套工具仍受各自权限规则约束。此项仅解决工具目录和执行编排入口缺失；任意 Shell 需要下述单独开关。

**特别重要**：这些权限仅限定模型可用的工具名称，不能把任意 SQL 自动变成只读 SQL。实际数据库连接必须由公司管理员在既有 DBHub 配置中开启 `readonly=true`，使用数据库只读账号、限定可访问库表、查询超时与最大返回行数。若未验证这套真实限制，禁止在可处理银行业务数据的环境中对匿名访客开放 SQL 查询。
匿名入口无登录信息并不等于具有任意数据库查询权限。跨用户的原始数据库数据访问边界取决于受控数据源和机构侧身份授权；前端 Session 隔离只解决会话串读。

### 可选：机构嵌入页放开 Shell 与文件工具

需要从机构页运行工作目录中的脚本或写入数据时，由工作台操作员在 Windows 启动 CMD 中主动开启：

```cmd
set "HARNESS_INSTITUTION_ALLOW_SHELL=1"
.\启动工作台.cmd "D:\teamai"
```

启用后只对共享 OpenCode 服务的**机构匿名 Session**下发 `shell:*`，同时允许原生 `read/write/edit/glob/grep/list/external_directory:*`，不更改任意 MCP 的权限规则，也不改变公管与普通报文排查的权限。要关闭时删除该环境变量并重启工作台，且新建机构 Session；已有 Session 权限不会追溯修改。

**高风险警告**：这不是工作目录隔离。Shell 可读取、修改 Windows 进程账号能访问的任意文件，也可能执行网络请求或修改数据库。如果同一时间启用了 `HARNESS_ENABLE_LAN=1`，能访问匿名机构页面的其他电脑用户也可能触发这些操作。必须通过认证网关、Windows 防火墙的最小 IP 白名单、低权限运行账户与必要的审计限制访问。不要向不受信任用户开放这一模式，更不要公开到互联网。需要真实目录隔离时，应使用独立低权限 Windows 账户或隔离工作进程。

注意：此模式仅放开 Shell 与常用文件操作工具；匿名自定义 Command/Subagent 仍受服务端的隔离检查约束，以避免绕过会话的 MCP 限制。

## 可选：完全隔离的服务部署

如果原有报文查询 Skill 必须使用上述四条固定命令以外的 Shell、浏览器自动化、带副作用的自定义命令或子 Agent，那么它超出了匿名共享服务的权限边界。此时可使用已提供的 `HARNESS_EMBED_EXECUTION_REGISTRY`、`HARNESS_EMBED_ISOLATED_CONFIG` 和独立 `HARNESS_SQLITE_PATH` 配置隔离工作进程。它不是默认必选，也不是普通 Skill 场景部署所必需的额外步骤。

## 验收

使用一个真实报文编号：检查首次提交后是否出现真实 Skill 加载与 DBHub 工具调用、工具结果是否进入后续分析、任务是否通过原生 idle / lifecycle 结束，而不是要求用户反复输入“继续”。再同时打开三个嵌入页，验证 A/B/C 独立 msgSn、各自模型、日志与停止按钮。

CI 能证明会话/权限配置、模拟原生调用与 Windows 页面行为；**真实内网 Kimi、TeamAI、DBHub 查询效果仍必须在有只读数据授权的环境中验证**。如果真实 Skill 使用的不是上述 DBHub 工具名，应根据工具真实名称做安全审查，不得直接放开 `*`。

### 真正的内网验收要求

不能再只验证 `session.prompt` 收到 `skills` 字段。必须检查真实模型是否调用 `execute`、TeamAI 完整命令和授权 `search_objects/execute_sql`，且每条返回可追溯。若模型仍提示“缺少 Shell 或执行类工具”，导出对应原生工具拒绝的 **action、resource、错误码**（不要包含数据库账号、SQL 敏感值或业务原文），按真实已审查命令精确授权；无权限时明确报告中断，不自动发“继续”掩盖问题。仓库不能访问公司的私有 Skill，CI 不能代替内网联调。
