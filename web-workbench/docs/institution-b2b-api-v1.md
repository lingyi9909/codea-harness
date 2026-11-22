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
- `X-Workbench-User-Id`：B 后端根据已登录用户生成的稳定业务标识。**C 不认证此标识，只在数据库中保存其摘要用于任务查询隔离；可达 C 的调用方可以伪造该标识。**

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
- 有任务 ID 且能伪造 `X-Workbench-User-Id` 的可达调用方可能读取该用户的排查结论。摘要化保存标识只降低静态存储明文，不意味着已认证。
- **必须确认** C 网络端口不向公网、不可信内网或 B 客户终端暴露；若工作台的其他 LAN 页面面向较宽用户群，其端口所处的共享网络也可能让这些用户直接调用 API。推荐网络侧 ACL、企业网关或单独网络隔离。
- B 后端不得信任浏览器直接传来的业务用户 ID；应从自己已登录会话中生成标识、检查报文查询授权，并限制调用频率。
- 业务数据和任务结果保留 7 天；接口不返回 OpenCode Session ID、原始思考、工具调用轨迹或密钥。并发任务排队，进程重启后未完成任务标记 UNKNOWN，不会自动重跑。

## 6. 联调验收

- [ ] B 后端无需密钥、无需 IP 白名单配置，就能调用 C 的两条接口。
- [ ] 不同 IP 的可达测试机均能得到 API 响应，证明应用层已取消白名单。
- [ ] B 真实已认证用户与任务对应；请求号幂等，错参数返回 409；不同业务标识查不到别的任务。
- [ ] 真实 OpenCode/机构 Skill 执行成功，最终只返回业务结论，机构 msgSn 保留前导零。
- [ ] 两位 B 客户并发不互串，重启时不重复执行；原机构嵌入页和其他场景无回归。
- [ ] C 的端口已由公司网络侧限定可达范围，B 客户端浏览器不能直接访问 C。

CI 的模拟 OpenCode 测试不能代替公司内网真实业务及网络验证。
