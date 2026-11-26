# Code-Harness → 平台 B：机构报文排查 API V1

> 分支：`feature/workbench-institution-b-proxy-access`（PR #116，基于 PR #115）。只提供创建排查任务和查询任务最终结果两个接口。原有机构嵌入页、其他工作台场景和 OpenCode 能力保持不变。真实内网模型/数据库/网络尚须现场验收。

## 1. 适用场景与网络

```text
客户浏览器 → B 平台前端 → B 平台后端 → C 智能工作台 API → OpenCode → 机构排查 Skill
```

客户只需要访问 B；由 B 后端调用 C，不使用 iframe，不要求客户直接访问 C。

**按最新要求：不配置 API Key、Bearer Token，也不配置来源 IP 白名单。** C 不再验证请求是否来自 B：任何网络层能连接 C 服务端口的客户端都能够调用这两个 API。必须由企业网络/防火墙/网关限制 C 的可达范围。

## 2. API 契约（只有两条）

### 创建任务

```http
POST http://<C内网IP>:<端口>/api/v1/integrations/institution/investigations
Content-Type: application/json
X-Workbench-User-Id: customer-001

{
  "date": "2026-11-22",
  "msgSn": "000012345678",
  "requestId": "B-20261122-001"
}
```

- `date`：严格的 YYYY-MM-DD，按 Asia/Shanghai UTC+08:00 计算全天 [00:00:00, 次日 00:00:00)。
- `msgSn`：1–256 位字符串，保留前导 0；允许英文字母、数字、下划线、点、冒号、连字符。
- `requestId`：1–128 位调用方幂等号。同一用户+同一请求号+相同 date/msgSn 复用旧任务，不重复执行；同号不同参数返回 409。
- `X-Workbench-User-Id`：B 后端根据已登录用户生成的稳定业务标识。**C 不认证此标识**；仍以 SHA-256 摘要执行任务归属匹配。自新增管理员接入查询功能起，新任务还将保存调用方声明的原始用户标识，供 C 本机管理员查看；旧任务只有不可逆摘要，不能恢复原始 ID。可达 C 的调用方可以伪造该标识。

新任务 HTTP 202；相同幂等号重复提交 HTTP 200。响应例：

```json
{
  "data": {
    "taskId": "550e8400-e29b-41d4-a716-446655440000",
    "status": "QUEUED",
    "requestId": "B-20261122-001",
    "createdAt": "2026-11-22T04:00:00.000Z",
    "updatedAt": "2026-11-22T04:00:00.000Z",
    "result": null,
    "errorCode": null
  },
  "requestId": "response-trace-id"
}
```

### 查询任务状态与最终结果

```http
GET http://<C内网IP>:<端口>/api/v1/integrations/institution/investigations/{taskId}
X-Workbench-User-Id: customer-001
```

返回状态及完成后的结论；使用不同的业务用户标识查询他人任务返回 404，但标识本身不是安全凭证。

```json
{
  "data": {
    "taskId": "550e8400-e29b-41d4-a716-446655440000",
    "status": "SUCCEEDED",
    "requestId": "B-20261122-001",
    "createdAt": "...",
    "updatedAt": "...",
    "result": {"summary": "面向业务人员的最终排查结论（Markdown 文本）"},
    "errorCode": null
  },
  "requestId": "response-trace-id"
}
```

状态：`QUEUED`（排队）、`RUNNING`（执行中）、`SUCCEEDED`（已得到最终结论）、`FAILED`（确定失败）、`UNKNOWN`（重启、超时或传输异常使最终结果不确定，**不自动重跑模型任务**）。

失败可能有 HTTP 400（参数格式不合法）、404（找不到此业务用户任务）、409（幂等号冲突）。**不再因缺少密钥或不在应用 IP 白名单而产生 401/403。**

B 可间隔 2–5 秒轮询查询接口；创建任务立刻返回，不需要维持长连接。当前不提供 SSE、追问、取消、客户端自定义模型或 Skill 接口。

## 3. C 工作台部署

**不需要新的接口启用环境变量**。本地单服务工作台启动后自动注册这两个 API；不会在不支持的多用户/隔离执行模式下强行注册。

如 C 需要对 B 开放 LAN 监听，可沿用既有的工作台启动菜单选择局域网地址和端口；示意配置如下（具体 IP 请替换为 C 自己的 IP）：

```cmd
set "HARNESS_ENABLE_LAN=1"
set "HARNESS_HTTP_BIND=10.20.30.40"
set "HARNESS_HTTP_PORT=4176"
set "HARNESS_PUBLIC_ORIGIN=http://10.20.30.40:4176"
.\启动工作台.cmd "D:\teamai"
```

上述网络监听参数本来就属于工作台现有部署配置，不是 B 接口专属设置。如果采用启动菜单选择 LAN 地址/端口，按实际菜单启动即可。

可选：`HARNESS_INSTITUTION_API_MODEL_ID` 精确指定已启用模型；未指定时优先寻找 Kimi，或在只启用了一个模型时使用该模型。默认 `HARNESS_INSTITUTION_API_CLIENT_ID` 是 platform-b，只有未来接入场景需要另行分组时才考虑配置。

接口不读取 `HARNESS_INSTITUTION_API_KEY` 或 `HARNESS_INSTITUTION_API_ALLOWED_IPS`：即使旧启动脚本里存在这两个环境变量，也不会用它们验证请求。

机构场景必须绑定有效 Skill；API V1 不允许由 B 指定任意 Skill、工作目录、模型、SQL、Shell 或 Prompt。沿用当前机构场景权限和 C 启动时配置的 Shell 策略。

任务状态及最终结论保存在独立 SQLite，默认是运行账户目录的 `.smart-workbench/state/institution-api.sqlite`，也可用 `HARNESS_INSTITUTION_API_DB_PATH` 指向业务工作目录外的本地绝对路径；保存最近 7 天，不需要新建 MySQL 表。

## 4. B 平台接入示例

仅需 B 后端发 HTTP 请求，**不要把 C 的地址开放给 B 客户端浏览器**。

```bash
curl -X POST 'http://10.20.30.40:4176/api/v1/integrations/institution/investigations' \
  -H 'Content-Type: application/json' \
  -H 'X-Workbench-User-Id: customer-001' \
  -d '{"date":"2026-11-22","msgSn":"000012345678","requestId":"B-20261122-001"}'
```

得到 taskId 后：

```bash
curl 'http://10.20.30.40:4176/api/v1/integrations/institution/investigations/<taskId>' \
  -H 'X-Workbench-User-Id: customer-001'
```

B 请求 POST 网络超时，应使用原 `requestId` 和完全相同参数幂等重试，不要换号造成重复执行。B 自己负责客户登录、机构数据访问权限及审计。

## 5. 权限与风险边界

- API 层**没有密钥，没有 IP 白名单，没有用户身份认证**。任何能连接 C 服务端口的用户均可以创建机构排查任务、消耗模型/DBHub 查询资源。
- 有任务 ID 且能伪造 `X-Workbench-User-Id` 的可达调用方可能读取该用户的排查结论。虽然仍用摘要匹配任务归属，但新任务会额外保存原始用户标识用于本地管理员审计；这不代表已认证。
- **必须确认** C 网络端口不向公网、不可信内网或 B 客户终端暴露；若工作台的其他 LAN 页面面向较宽用户群，其端口所处的共享网络也可能让这些用户直接调用 API。推荐网络侧 ACL、企业网关或单独网络隔离。
- B 后端不得信任浏览器直接传来的业务用户 ID；应从自己已登录会话中生成标识、检查报文查询授权，并限制调用频率。
- 业务数据和任务结果保留 7 天；接口不返回 OpenCode Session ID、原始思考、工具调用轨迹或密钥。现在可以由 C 本机管理员设置最多同时执行 3、5、10 或 15 个独立 Session；超出上限排队。进程重启后未完成任务标记 UNKNOWN，不会自动重跑。

## 6. 联调验收

- [ ] B 后端无需密钥、无需 IP 白名单配置，就能调用 C 的两条接口。
- [ ] 不同 IP 的可达测试机均能得到 API 响应，证明应用层已取消白名单。
- [ ] B 真实已认证用户与任务对应；请求号幂等，错参数返回 409；不同业务标识查不到别的任务。
- [ ] 真实 OpenCode/机构 Skill 执行成功，最终只返回业务结论，机构 msgSn 保留前导零。
- [ ] 两位 B 客户并发不互串，重启时不重复执行；原机构嵌入页和其他场景无回归。
- [ ] C 的端口已由公司网络侧限定可达范围，B 客户端浏览器不能直接访问 C。

CI 的模拟 OpenCode 测试不能代替公司内网真实业务及网络验证。

## 7. C 本机管理员：接口任务管理页面（PR #116 增量）

**打开地址：** `http://127.0.0.1:4176/institution-tasks`（如修改启动端口，替换为实际端口）。从工作台本机侧栏或 `/config` 页顶部也可进入。

- 页面按创建时间倒序展示由 B API 发起的全部机构排查任务；包含任务 ID、请求号、来源系统、排查日期、提交时间、最后更新时间和执行状态；可以按状态筛选、按完整任务 ID 或 `requestId` 精确查询；每页可选择 20 / 50 / 100 条，并支持翻页和跳页。
- 状态有 `QUEUED`、`RUNNING`、`SUCCEEDED`、`FAILED`、`UNKNOWN`；每 5 秒自动刷新，也可手工刷新。运行中状态不会自动发起新的执行。
- 点击“详情”查看状态、错误代码，以及 **`SUCCEEDED` 后实际返回给 B 的完整 `result.summary` Markdown 结论**。失败或不确定状态不会编造结论。
- 完成的任务清理了原始 `msgSn`，因此详情会显示“已清理”，避免延长业务敏感报文的存储周期。任务库仅保留最近 7 天。

**权限与配置页管理权限保持一致：** 两个新只读管理接口 `GET /api/v1/institution-tasks` 和 `GET /api/v1/institution-tasks/{taskId}` 必须同时满足 C 服务器真实回环地址访问、本地已登录工作台会话，以及工作目录 `.workbench/config-access.json` 的 `editors` 名单校验；普通内网用户、未登录者和非管理员不得读取任务和最终结论。

这些 **管理接口不是平台 B 的对外接口**；B 仍只有第 2 节定义的 POST 创建任务和 GET 查询单笔任务两个接口，接口参数和响应保持不变。管理页不会提供重试、停止、删除、修改权限等危险操作。

提示：姓名登录和 `editors` 列表沿用当前工作台已有的管理机制，这不是企业 SSO 或强身份认证；在生产场景仍需配合本机访问控制及系统账号保护。

## 8. 系统管理目录与接口接入情况

工作台侧栏「**系统管理**」是独立可折叠菜单，**位于“最近对话”上方，不再把管理页面混在聊天历史之后**。仅在 C 服务器本机登录，并通过 `/api/v1/scene-access` 的配置管理员访问校验时显示，下属页面：

- 场景配置：`/config`
- 接口任务管理：`/institution-tasks`
- 接口接入情况：`/institution-tasks?view=access`
- 访问统计：`/analytics`
- 组件测试：`/component-lab`

「接口接入情况」通过管理专用 `GET /api/v1/institution-access` 返回：

- **当前接入模式：NONE（无密钥、无来源白名单、无平台身份验证）**；
- 当前服务端来源标签：默认 `platform-b`，可由 `HARNESS_INSTITUTION_API_CLIENT_ID` 配置；这个标签**不证明**请求真来自平台 B；
- 过去 7 天任务中观察到的调用方标识 `X-Workbench-User-Id`、该标识提交任务数、成功数及最近提交时间，最多 100 个不同标识。**不是授权名册**。
- 管理任务列表可按 `taskId`、`requestId` 或原始 `X-Workbench-User-Id` 精确搜索。

**数据迁移：** 旧 `institution-api.sqlite` 首次使用时自动增量添加 `external_user_id` 列，不删除已有任务或结论。旧任务仅能展示前 12 位不可逆用户摘要；新任务仅在本机管理员查询接口中显示原始业务用户 ID。数据库文件继续限制为当前服务进程私有，保留 7 天。

**重要：** 因为此前选择取消密钥和 IP 白名单，当前系统**没有可查询的“已授权平台列表”**，也没有平台级拒绝开关。若将来要真正按平台禁用访问，必须增加可信调用方身份验证或通过企业网络网关进行认证和准入；仅登记一个可由调用方自行填写的 `X-Workbench-User-Id` 不能起到授权作用。B 对外接口数量不变，以上接口仅供 C 本机配置管理员读取。

## 9. API 并发执行与按业务用户聚合（PR #116 二次增强）

管理入口：`http://127.0.0.1:4176/institution-tasks`

### 并发上限

- 页面提供 **3 / 5 / 10 / 15** 四档，默认 **3**；当前设置持久化在同一个 `institution-api.sqlite` 的 `institution_api_settings` 表，C 服务重启后保留。
- 每个 B API 任务继续创建独立 OpenCode Session，任务领取由单串行调度改为限流并行调度。一个任务释放执行名额后，队列自动补位；SQLite 条件领取和 `requestId` 幂等语义不变。
- C 本机管理员专用：
  - `GET /api/v1/institution-concurrency`：查询 `maxConcurrency`、`running`、`queued`。
  - `PATCH /api/v1/institution-concurrency`，body `{"maxConcurrency":5}`：修改生效上限。必须有真实本机访问、已登录管理会话、`config-access.json` editors 权限、有效 CSRF 和匹配 Origin。
- 增大并发数时立即领取后续排队任务；降低并发数时不终止已执行的任务，等待运行中数量降到新上限以下后再补位。
- 页面每 5 秒刷新运行中和排队中的任务数量。**15 并发仅代表本系统最多同时提交 15 个 Session，并不保证私有模型、DBHub 或底层业务系统能够真正支持 15 路同步处理**；建议内网逐档压测、校验响应时延/限流/DB 资源。
- 本轮仍是**单 C 进程**调度，勿让两个独立进程共享该 SQLite 库。重启时已领取任务标记为 UNKNOWN，不自动重做任何模型、SQL 或业务动作。

### 业务用户聚合

在「接口任务管理」切换「**按业务用户聚合**」视图，按调用方自报 `X-Workbench-User-Id` 分组，支持对业务用户 ID 精确搜索和分页。每个用户独立显示 **总任务数 / 运行中 / 排队中 / 成功 / 失败 / 未知**。选中用户可展开全部任务，进一步查看对应任务详情和最终 `result.summary` Markdown 结论。

- 聚合使用 SQLite 存储的 `client_id + user_hash` 作为主分组键，避免相似 ID 或同名不同来源的任务误聚合；新请求将展示原始用户 ID。
- 升级前的历史任务保留了不可逆摘要，仍然归入正确的哈希分组，但不能还原原始用户 ID。
- 管理接口 `GET /api/v1/institution-tasks/users?q=customer-001&offset=0` 返回用户分组和计数（每页最多 50 个）。`GET /api/v1/institution-tasks?userHash=<64-hex>` 返回该分组的任务；二者均与配置编辑使用同一组本机管理员鉴权条件。
- B 的创建任务 POST 和单笔任务 GET 两条对外接口 **保持原样**；无新密钥或 IP 白名单，用户 ID 依旧是调用方主动提供的信息，不能当作平台认证。

## 10. 任务执行诊断与状态轮询优化（PR #117）

本轮只覆盖机构 B API 任务；B 原有创建与查询接口不变，不向 B 返回诊断日志、Session ID 或原生工具信息。

- 本机管理员进入 `/institution-tasks`，点击任务行的「诊断」（或「详情」下方的诊断区）查看按任务 ID 保存的阶段时间线，日志**最新在前**。
- 管理员只读接口：`GET /api/v1/institution-tasks/{taskId}/logs?offset=0&limit=20`，`limit` 允许 20/50/100；分页有 `total` / `offset` / `limit` / `items`。
- 日志事件写在同一个 `institution-api.sqlite` 的 `institution_api_task_events` 表，随关联任务的 7 天保留期级联清理。仅存日期、阶段枚举、级别、受控错误代码和原生事件去重 ID，不记录报文内容、msgSn、SQL、Prompt、凭证、工具输入、输出及模型推理。
- 任务执行阶段记录连接、Skill/模型检查、Session 创建、Prompt 受理、原生工具调用状态、状态读取重试、最终结果接收及任务终态。服务关闭/重启时对未完成任务补记异常阶段，不自动重投 Prompt。
- 同一 C 进程的机构任务复用**一条** OpenCode 原生事件流，以 Session ID 将事件匹配回任务。终态与权限事件可以唤醒状态核验；若事件流缺失/断线，正常原生客户端约每 6.5 秒只读轮询一次。测试桩不提供事件接口时继续使用原 1.2 秒轮询作兼容。最多等待 10 分钟、不确定结果标记 UNKNOWN 的安全语义不变。
- 仅 C 真实回环地址、有效登录会话及 `.workbench/config-access.json` 的 editors 可以读取日志；LAN 普通用户及 B 对外接口不能读取。

**验收要求：** 检查两个并发用户日志不串、原生事件订阅丢失时仍能最终完成、真实私有 OpenCode 的事件类型与读取行为、高并发下轮询量与延迟；确认 B 原接口完全不变。旧任务不会补录当时没有采集到的日志。
