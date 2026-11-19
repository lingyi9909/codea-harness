# 机构报文嵌入页：直接复用工作台原有执行链路

## 默认部署（无需第二套 OpenCode）

直接使用原 `启动工作台.cmd` 启动的 OpenCode 服务和 `.workbench/scenarios.json`。
公管问题排查、报文问题排查、机构报文排查走同一套 `registerChat`、原生 `session.prompt`、Skill/Command/Subagent 分发、SSE、停止与追问 API。
独立机构页只在前端入口、访客会话和权限范围上与普通登录后的工作台不同。

### Skill 如何复用

- 如果 `institution-message` 显式配置了 Skill，就使用它。
- 如果 `institution-message` 仍为默认 `builtin/chat`，而 `message` 已经配置为 Skill、且工作目录相同，自动继承已配置的报文排查 Skill；不用再手动配置一遍。
- 如果上述两个场景都没有配置 Skill，页面会提示缺少业务执行器，不把普通 Chat 的“阶段 1”描述当作已执行查询。
- 不在匿名共享服务内执行任意自定义命令或子 Agent：它们可能以另一套本机权限运行，绕过匿名 Session 的工具限制。如业务确实需要这些能力，须采用可信用户认证与独立执行服务的方案。

### 多人并发

每次页面访问创建随机访问令牌、独立用户归属和原生 Session。不同访客的消息、活动 SSE、模型状态、中断/结束操作互不混用；访问者不能查询其他人的 Session。服务重启后保存可恢复的绑定，但不能自动重复提交原任务。

### 匿名会话安全策略

共享 OpenCode 服务不再一刀切全部工具拒绝，而是 OpenCode V2 的有序许可规则：默认拒绝、只允许读取 Skill 定义与 `dbhub_search_schema`、`dbhub_execute_sql` 两个已知 DBHub 工具；通用 Shell、文件读写、外部目录、子 Agent 等继续拒绝。

**特别重要**：这些权限仅限定模型可用的工具名称，不能把任意 SQL 自动变成只读 SQL。实际数据库连接必须由公司管理员在既有 DBHub 配置中开启 `readonly=true`，使用数据库只读账号、限定可访问库表、查询超时与最大返回行数。若未验证这套真实限制，禁止在可处理银行业务数据的环境中对匿名访客开放 SQL 查询。
匿名入口无登录信息并不等于具有任意数据库查询权限。跨用户的原始数据库数据访问边界取决于受控数据源和机构侧身份授权；前端 Session 隔离只解决会话串读。

## 可选：完全隔离的服务部署

如果原有报文查询 Skill 必须使用 Shell、浏览器自动化、带副作用的自定义命令或子 Agent，那么它超出了匿名共享服务的权限边界。此时可使用已提供的 `HARNESS_EMBED_EXECUTION_REGISTRY`、`HARNESS_EMBED_ISOLATED_CONFIG` 和独立 `HARNESS_SQLITE_PATH` 配置隔离工作进程。它不是默认必选，也不是普通 Skill 场景部署所必需的额外步骤。

## 验收

使用一个真实报文编号：检查首次提交后是否出现真实 Skill 加载与 DBHub 工具调用、工具结果是否进入后续分析、任务是否通过原生 idle / lifecycle 结束，而不是要求用户反复输入“继续”。再同时打开三个嵌入页，验证 A/B/C 独立 msgSn、各自模型、日志与停止按钮。

CI 能证明会话/权限配置、模拟原生调用与 Windows 页面行为；**真实内网 Kimi、TeamAI、DBHub 查询效果仍必须在有只读数据授权的环境中验证**。如果真实 Skill 使用的不是上述 DBHub 工具名，应根据工具真实名称做安全审查，不得直接放开 `*`。