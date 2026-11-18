> 最新单工作目录、免 MySQL 的机构入口请使用 [SQLite 独立部署说明](institution-standalone-deployment.md)。本文保留早期接口契约及旧 MySQL/独立目录部署记录；其中多套业务目录及 resetCommand 配置不适用于新的共享目录模式。

# 报文排查嵌入页 V1

## 交付范围

基于 PR #112 的多人基础（1842914c8dfe105d7215756aa2a604207a36159b），单独提供 `/embed/message`，不修改 PR #110/#113，不合并前序分支。

- 页面只有报文排查、当前模型与结束会话，没有工作台登录、手机号、侧栏、历史或场景配置。
- 每次打开/刷新，由宿主系统后端重新领取一次性票据；首次提交创建新 OpenCode Session。打开页面本身不调用模型。
- 三名使用者可同时排查；同一使用者的不同页面拥有不同凭据和 Session。不同页面不能相互查看、改名、删除、中断或回复 Permission/Form。
- 当前用户由宿主已认证会话确定，映射到管理员配置的独立执行 owner。该入口不使用 `.workbench/user.json` 或用户名/手机号表单。
- 保留原报文页面的时间、Trace ID、msgSn、关键词、结果、追问、条件调整及原生非敏感权限/Form。PR #113 的机构报文页面不在本次变更中。

## 身份和执行边界

宿主后端必须从自己的可信登录态或网关身份提取稳定 subject，不能直接采用前端提交的 subject。本次不实现对方系统的 SSO。

| 层次 | 实现 |
|---|---|
| 宿主认证 | 后端专属 API Key + 已配置的 clientId/subject 映射；API Key 不发送给浏览器 |
| 页面接入 | 随机票据，60 秒过期，数据库事务保证只能兑换一次；票据通过指定来源的 postMessage 传入，不能放 URL |
| 本次会话 | 随机 Bearer 凭据 + CSRF，服务端只保存摘要，浏览器只保存在 iframe 内存；不使用第三方 Cookie |
| 原生会话归属 | 每个页面最多创建一个 Session；grant、owner、服务实例、native ID、DB binding 和 metadata 关联 |
| 执行 | 复用现有 ExecutionRegistry；每个主体需预配置独立 OpenCode 服务/OS 用户或容器、可写目录、临时目录和业务凭据 |
| 并发保护 | 不同 owner 可并发；同 owner 的提交用独立 MySQL 连接池中的 advisory lock 串行检查，其他页面有 active/queued 任务时返回 409，不盲目复用固定写入点 |
| 结束和过期 | 先撤销 grant，再只中断/清理它绑定的原生 Session；清理失败保留队列，每 15 秒重试，不能中断整个服务 |

配置中的 isolation 是部署声明，必须在内网真实验证 OS/容器 ACL。HTTP/浏览器脚本化测试不证明实际 OpenCode Shell/文件系统隔离已经生效。两人不得映射到同一个 owner、服务、授权头或可写目录。

票据/会话由 MySQL 持久化，服务端重启不清空票据的已消费状态。浏览器刷新不会读取旧 grant 或旧聊天缓存，宿主应重新领取票据。关闭浏览器不是可靠的取消通知；需要立即停止时点击“结束会话”，否则按过期策略清理。

配置变更在服务重启后加载；每次请求重新检查 subject/来源授权。移除映射后，已有凭据在下次请求即失效并进入清理，不继续依赖原 TTL。

业务 Skill 使用执行环境内已配置的业务授权。本入口不支持在普通聊天/Form 中收集密码、短信验证码或 Token；检测到敏感原生 Form 会拒绝返回。需要额外业务认证时应在目标业务系统完成，本轮不复用旧手机号短信模拟框架。

## 数据库迁移与启动

沿用 TypeScript + MySQL 8.4 + OpenCode 2.x；不新增 Agent Runtime 或 Python。

1. 在 `web-workbench` 执行 `npm ci`、`npm run typecheck`、`npm run web:typecheck`、`npm run web:build`。
2. 备份工作台专用数据库，使用现有 `npm run migrate` 和单独迁移账号应用 `003_embedded_message.sql`，新增 wh_embed_tickets / wh_embed_sessions。不改 001/002。运行账号需访问这两表、wh_owners 和 wh_chat_session_bindings；不要给运行账号 DDL 权限。
3. 配置 `HARNESS_PUBLIC_ORIGIN=https://workbench.example` 和原有 MySQL 连接参数，通过 TLS 反向代理对外提供服务，后端仍只监听 127.0.0.1。代理必须保留固定 Host 和浏览器 Origin，关闭 SSE 缓冲。
4. 使用 `HARNESS_EXECUTION_REGISTRY` 配置每个用户的隔离服务，参见 [V1.1 部署文档](v1-1-deployment-and-acceptance.md)。每个服务需预先加载真实的报文排查 Skill/Command/Subagent，工作目录映射遵循该文件的既有规则。设置每用户独立 TMP/缓存/凭据，不能复制共享 user.json 充当身份来源。
5. 在启动工作目录的 `.workbench/scenarios.json` 中明确配置 `message` 执行入口。嵌入模式拒绝 `builtin:chat` 和默认 `/models`，防止没有接入查询工具却执行普通聊天。示例（Skill 名称以实际安装为准）：

```json
{"version":1,"scenes":{"message":{"directory":"D:\\teamai","executor":{"type":"skill","name":"message-query"}}}}
```

6. 在受文件权限保护、不会入库的服务器配置文件中登记宿主：

```json
{
  "publicOrigin": "https://workbench.example",
  "sessionTTLSeconds": 7200,
  "clients": [{
    "id": "operations-portal",
    "secret": "REPLACE_WITH_A_RANDOM_SECRET_AT_LEAST_32_CHARACTERS",
    "parentOrigin": "https://portal.example",
    "principals": {
      "employee-001": "11111111-1111-4111-8111-111111111111",
      "employee-002": "22222222-2222-4222-8222-222222222222"
    }
  }]
}
```

owner UUID 必须与执行注册表对应，建议为嵌入使用者分配专属 owner/服务，不与其他工作台执行器共用凭据和写目录。首次成功领取票据时创建缺失的 wh_owners 记录，不要求手机号或用户资料表记录。

7. 设置 `HARNESS_EMBED_CONFIG` 指向该文件，并启动：

```powershell
$env:HARNESS_PUBLIC_ORIGIN = "https://workbench.example"
$env:HARNESS_EXECUTION_REGISTRY = "D:\workbench-config\execution-registry.json"
$env:HARNESS_EMBED_CONFIG = "D:\workbench-config\embed-clients.json"
node --experimental-strip-types apps/server/src/start.ts --multi-user --workspace "D:\teamai"
```

原 Windows `--local-only` 启动方式保持不变。不设置嵌入配置时不开放嵌入 API。配置有误、执行环境未登记时拒绝启动；没有真实报文执行入口时拒绝执行。

## 宿主系统对接

**宿主后端**提供自己的 `/message-investigation/ticket` 路由：先校验宿主现有身份，从服务端会话提取 subject，再调用以下接口。禁止把工作台 API Key 放进网页或把用户输入的 subject 直接转发。

```http
POST https://workbench.example/api/v1/embed/tickets
Authorization: Bearer <宿主后端专属 API Key>
Content-Type: application/json

{"clientId":"operations-portal","subject":"employee-001"}
```

返回 `{data:{ticket,expiresAt,parentOrigin,embedUrl},requestId}`。不发送 Origin 头；这是后端调用。示例网页（宿主自己的后端负责上面的认证转发）：

```html
<iframe id="investigation" title="报文排查"
  src="https://workbench.example/embed/message?parentOrigin=https%3A%2F%2Fportal.example"
  style="width:100%;height:850px;border:0"></iframe>
<script>
const targetOrigin = 'https://workbench.example';
const frame = document.getElementById('investigation');
window.addEventListener('message', async event => {
  if (event.origin !== targetOrigin || event.source !== frame.contentWindow) return;
  if (event.data?.type !== 'workbench.embed.ready' || typeof event.data.nonce !== 'string') return;
  const response = await fetch('/message-investigation/ticket', {
    method: 'POST', credentials: 'same-origin', cache: 'no-store'
    // 按宿主自身要求附加 CSRF；后端从已认证会话确定用户。
  });
  if (!response.ok) return;
  const result = await response.json();
  frame.contentWindow.postMessage({
    type: 'workbench.embed.launch', nonce: event.data.nonce, ticket: result.ticket
  }, targetOrigin);
});
</script>
```

iframe 会校验精确 parent origin、event.source、每次加载生成的 nonce，并通过自身同源请求兑换票据。CSP frame-ancestors 仅允许配置中的来源。接口不放宽原工作台的 Cookie/Origin/CSRF 策略，不开放任意 CORS。

## 接口清单

| 接口 | 使用方 / 限制 |
|---|---|
| GET /embed/message | 独立页面；CSP 来源白名单 |
| GET /api/v1/embed/config?parentOrigin=… | 页面初始化；无秘密 |
| POST /api/v1/embed/tickets | 仅宿主后端；Key + subject 映射；每 client 每分钟 120 次 |
| POST /api/v1/embed/exchange | 仅允许工作台 Origin；一次性票据兑换 |
| GET /api/v1/embed/session | 查询本次 Session ID/状态，不提供历史 |
| DELETE /api/v1/embed/session | 撤销本次凭据并清理本次原生会话 |
| GET /api/v1/embed/scene-config | 只读本次报文绑定，不提供配置编辑能力 |
| /api/v1/embed/chat/models、sessions、sessions/:id/** | 复用原生消息/权限/Form/中断/事件；仅报文场景，Bearer + CSRF + grant/DB/native 多重归属校验 |

历史、项目切换、文件附件、Session 重命名/删除接口被拒绝；结束使用专用 session 端点。Bearer 无法访问普通工作台 API，也不能通过 native ID 访问另一个页面，即使两个页面属于同一个用户。

## 验证与验收

- `npm run test:embed`：三用户并发、同用户多窗口归属、跨会话八类操作、票据原子兑换/重放/过期、目录/场景/附件/CSRF 拒绝、敏感 Form 拒绝、过期只清理本会话、SSE Bearer 分块解析。
- `npm run test:embed:ui`：真实 Chromium + iframe + 三页面，已有工作台 Cookie 不被使用；各自结果、刷新空白、无浏览器持久化和截图。OpenCode 使用脚本化测试客户端。
- `npm run test:embed:mysql`：**仅独立 harness_ci_test 库**，真实 MySQL 8.4 迁移、并发一次性兑换、Session 绑定、撤销、清理和跨实例 advisory lock。
- 原工作台回归、Windows 浏览器、Node 22/24/26 与 MySQL 集成继续运行，不能用新入口通过代替旧功能兼容。
- 内网需接入方后端、实际报文 Skill、私有模型、真实 OS/容器 ACL 做 A/B/C 并发验证。真实机构认证或短信不在本入口实现；不声称已完成。

回退：停止服务，取消 HARNESS_EMBED_CONFIG，回到此前启动包；保留 003 表与备份，不自动 DROP。原有工作台与用户数据不迁移、不重新归属。
