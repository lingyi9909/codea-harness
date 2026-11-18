# 机构报文排查：匿名 iframe 接入与重启恢复

## 本轮需求基线与改造计划

仅提供独立页面，接入方只加 iframe；不要求工作台账号、手机号、宿主身份或票据接口。每次打开或刷新创建新会话；已有页面在工作台后端重启后凭原凭据重新连接，不重放认证或模型请求。机构业务认证继续由机构系统完成。

| 文件 | 改造 |
|---|---|
| apps/server/src/embed-auth.ts | 同源匿名创建、随机能力凭据、CSRF、持久归属检查 |
| packages/storage/migrations/004_anonymous_embed.sql | 匿名模式、执行槽位回收标记、认证查询状态/安全证据；保留 001～003 原文 |
| packages/storage/src/embed.ts | 原子槽位租用，跨实例锁、查询领取/结果持久化与不确定状态拒绝 |
| apps/server/src/embed-routes.ts | 匿名创建、机构安全查询、单会话归属与受信任重置后回收 |
| apps/server/src/institution-query.ts | 机构接口适配契约；凭据专用 POST，不经普通聊天/原生 Form |
| apps/web/src/AnonymousInstitutionPage.tsx | 无父页面脚本的独立入口；内存凭据、重连、结束/过期 |
| apps/web/src/InstitutionMessageInvestigation.tsx | 沿用 PR #113 日期/msgSn/账号/密码/验证码样式；仅安全证据进入分析 |
| apps/web/src/api/client.ts、activity-source.ts | 不用 Cookie，503 SSE 重连，结束后禁止降级到普通工作台 API |
| tests/api、tests/mysql、tests/ui | 三人并发、越权、重启、槽位回收、凭据隔离和真实 iframe 测试 |

## 接入方只需要这段 HTML

```html
<iframe src="https://workbench.example/embed/institution-message"
  title="机构报文排查" style="width:100%;height:900px;border:0"></iframe>
```

也可以直接打开同一 URL。无 postMessage 握手、宿主后端 Key、用户映射、跨站 Cookie。部署人员通过 CSP 精确登记允许嵌入的域名，不开放任意来源。

## 我方部署

下载 `feature/workbench-embedded-message-v1`。已有 OpenCode、Node、TeamAI、DBHub 不需要重新安装。进入 web-workbench，按原离线依赖流程准备后执行 `npm ci`、`npm run web:build`。备份工作台专用 MySQL，使用单独迁移账号运行 `npm run migrate`，检查 004 记录；不能用业务库。

HARNESS_EMBED_CONFIG 文件示例：

```json
{
  "publicOrigin": "https://workbench.example",
  "sessionTTLSeconds": 7200,
  "clients": [],
  "anonymous": {
    "parentOrigins": ["https://portal.example"],
    "executionOwners": [
      "11111111-1111-4111-8111-111111111111",
      "22222222-2222-4222-8222-222222222222",
      "33333333-3333-4333-8333-333333333333"
    ],
    "resetCommand": ["node", "D:\\workbench-config\\reset-slot.mjs"]
  }
}
```

executionOwners 是后台空闲槽位编号，不是真人账号。执行注册表仍为每个槽位预置独立 OpenCode 服务、OS 用户/容器、工作目录、TMP/缓存和服务鉴权头，格式见原 [部署文档](v1-1-deployment-and-acceptance.md)。首次租用自动补齐 wh_owners，不要求 wh_user_profiles 或任何手机号。配置 3 个槽位允许 3 个页面同时在线，容量用完明确返回 503，不挤占其他页面；刷新创建新会话，旧页按到期策略回收，因此要为刷新/遗弃页预留容量。

resetCommand 是**管理员维护的幂等环境重置程序**，不是模型命令。后端只在该页撤销、确认对应原生会话中断并移除后执行，并追加 `--owner <UUID> --directory <映射目录> --service <实例标识>`；不经过 shell、不传业务密码、输出不记录，60 秒超时。程序必须在隔离环境内清理访客可写文件、临时文件、业务授权缓存及原生残留状态，恢复批准的只读定义，成功后才退出 0。需要停止服务时仅操作该槽位的服务，不能停止其他槽位。

**本轮不提供假定公司 OpenCode 安装布局的通用清目录脚本。** 真实 resetCommand 和 OS ACL 必须按内网实际路径验收。缺少重置程序或重置失败时，槽位安全停用，不会自动交给下一位；失败会重试。管理员也可确认环境已彻底清理后，更新 serviceIdentity 重新部署该槽位。仅修改标识不能替代实际清理。租用和清理共用 MySQL advisory lock，防止两个 API 实例重复重置后误删新访客文件。

启动（继续使用已有 MySQL 运行账号环境变量）：

```powershell
$env:HARNESS_PUBLIC_ORIGIN = "https://workbench.example"
$env:HARNESS_EXECUTION_REGISTRY = "D:\workbench-config\execution-registry.json"
$env:HARNESS_EMBED_CONFIG = "D:\workbench-config\embed-clients.json"
node --experimental-strip-types apps/server/src/start.ts --multi-user --workspace "D:\teamai"
```

--multi-user 是现有执行隔离启动模式，嵌入入口不显示登录，也不调用 auth/login。原工作台/本地单人入口保留。反向代理 TLS、固定 Host/Origin 和 SSE 禁用缓冲沿用原配置。启动目录中的 message 场景需绑定真实报文 Skill/Command/Subagent，不接受默认 /models 或 builtin:chat。

## 机构系统接口：明确未替代真实联调

PR #113 只有页面，尚未接入机构认证/查询。此轮提供服务器专用适配接口：

```powershell
$env:HARNESS_INSTITUTION_ADAPTER_URL = "https://internal-adapter.example/investigate"
$env:HARNESS_INSTITUTION_ADAPTER_AUTH = "Bearer <仅服务端使用的适配接口密钥>"
```

后端对该固定地址 POST：

```json
{"day":"2026-11-18","from":1794931200000,"to":1795017600000,"msgSn":"000123","account":"...","password":"...","verificationCode":"001234"}
```

from 为起点含、to 为次日零点不含，页面以本地自然日转换，精确到秒。HTTP 头 X-Workbench-Grant 为本次服务器生成的会话标识。适配服务负责目标业务系统认证、验证码核验和只读查询，Token 仅留在其会话范围并在完成后销毁。只返回 `{"evidence":"允许展示给用户及模型的日志证据"}`，不得返回密码、验证码、Token、Set-Cookie 或其他字段；接口失败内容不透传。

凭据只经专用 POST 送至适配服务，不进入普通聊天、模型、数据库或工作台日志。证据严格限长，拒绝明显的秘密回显；可信适配器仍必须履行脱敏契约，通用字符串检查不能证明任意日志内容绝无秘密。证据与查询状态持久化，用于服务重启后避免再次消费验证码，结束清理时删除该临时证据。

每页一次机构认证查询，成功后可以追问；新条件需重新打开页面。并发重复请求只允许一个领取执行；同条件已成功请求返回已保存证据。RUNNING/FAILED、不同条件或调用结果不确定时，不自动重发认证或模型任务。未配置适配器显示“接口尚未配置”，不会假装查到数据或调用模型。

本轮改变为“确定性安全认证/取证 → 原生 OpenCode 分析和追问”。不声称实现模型自行持有凭据、原生 Skill 自动认证续跑，或真实机构系统联调通过。

## 接口与恢复边界

| 接口 | 行为 |
|---|---|
| GET /embed/institution-message | 独立机构页，CSP frame-ancestors 白名单 |
| POST /api/v1/embed/anonymous | 同源创建页面会话；无宿主身份，随机凭据仅留内存 |
| GET/DELETE /api/v1/embed/session | 查询/结束本页；数据库持久授权及原生 Session 绑定 |
| POST /api/v1/embed/institution-query | Bearer+CSRF 专用认证查询，结果持久化，不收前端 owner |
| /api/v1/embed/chat/** | 原有归属校验、SSE/追问/停止/非敏感 Permission/Form；不开放历史/附件/其他场景 |

只重启工作台后端且原页面仍开着：自动重连原会话、继续查看结果/追问，不重新创建 Session、不重新调用模型或验证。刷新/重新打开：按需求创建全新会话，不从浏览器存储恢复。OpenCode 本身或业务适配服务也重启时，不能保证正在执行的任务继续；展示原生实际状态/不可用错误，不自动补发。

## 验证与回退

- `npm run test:anonymous`：三人并发、八类越权、API 重建恢复、独立停止、容量/回收、证据缓存、秘密回显和不确定状态。
- `npm run test:anonymous:mysql`：独立 harness_ci_test 库，真实 MySQL 8.4 并发租用/跨实例锁、查询领取和重建存储恢复。
- `npm run test:anonymous:ui`：真实浏览器，宿主仅 iframe，三页面、敏感字段不进其他请求/模型/存储、API 真正关闭后重启、追问及重新打开；原生执行和业务适配器使用测试桩。
- 真实内网三用户、真实报文入口、机构认证及 resetCommand/OS ACL 由使用方验收。测试桩不等于真实平台联调。

回退到 d4715a18：停止服务，取消 anonymous 配置及新增适配环境变量，使用旧启动包；保留 004 和备份，不修改历史迁移、不自动 DROP。旧宿主票据模式仍在原路径保留兼容，当前需求不依赖它。
