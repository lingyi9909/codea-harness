# Code-Harness → 平台 B：机构报文排查 API V1

> 分支：\`feature/workbench-institution-b-proxy-access\`；基于 PR #115。仅增加两个 **B 服务端 → C 服务端**接口；保留 C 上的机构嵌入页、其他场景、既有 OpenCode 调度和权限。**未经过企业内网真实模型与网络验收，不可直接视为生产可用。**

## 1. 网络架构

\`\`\`text
B 客户的浏览器 ─HTTPS→ B 业务页面 ─ B 自己的服务端 ─内网 HTTP/TLS→ C Code-Harness
                                                           └→ 现有 institution-message Skill → OpenCode/DBHub/TeamAI
\`\`\`

客户**不访问 C**，平台 B 不需要 iframe。B 自己开发页面，B 后端使用可信凭据调用 C；不得将 B→C 密钥发给浏览器，也不能让浏览器直接调用这两个 API。

## 2. 接口契约

### 接口一：创建排查任务

\`POST http://<C 内网 IP>:<端口>/api/v1/integrations/institution/investigations\`

Headers（**两条接口都必须提供**）：

\`\`\`http
Authorization: Bearer <C 为平台 B 分配的服务端 API 密钥>
X-Workbench-User-Id: <B 已认证且稳定不变的业务用户 ID>
Content-Type: application/json
\`\`\`

Body：

\`\`\`json
{
  "date": "2026-11-22",
  "msgSn": "000012345678",
  "requestId": "B-20261122-10001"
}
\`\`\`

- \`date\`：严格的 \`YYYY-MM-DD\` 日历日期，按 **Asia/Shanghai (UTC+08:00)** 解释，查询 \`[00:00:00, 次日 00:00:00)\`；不以浏览器所在时区解释。
- \`msgSn\`：原始字符串，保留前导 0，1–256 位，允许英文字母、数字、\`_\`、\`.\`、\`:\`、\`-\`，不允许换行或注入额外 Prompt。
- \`requestId\`：调用方生成的幂等号，1–128 位，允许英文字母、数字、\`_\`、\`.\`、\`:\`、\`-\`。同一客户用户、同一请求号和相同日期/msgSn **返回原任务，不重新执行**；重用同号却修改日期或 msgSn 返回 409。
- \`X-Workbench-User-Id\`：只能由 B 的**可信后端**根据其登录态生成，不接受浏览器自行指定；仅存 SHA-256 派生值，不存明文。

新建任务返回 HTTP 202：

\`\`\`json
{
  "data": {
    "taskId": "550e8400-e29b-41d4-a716-446655440000",
    "status": "QUEUED",
    "requestId": "B-20261122-10001",
    "createdAt": "2026-11-22T04:00:00.000Z",
    "updatedAt": "2026-11-22T04:00:00.000Z",
    "result": null,
    "errorCode": null
  },
  "requestId": "response-trace-id"
}
\`\`\`

重复提交同一请求时返回 HTTP 200 和已有 \`taskId\`。创建接口**不等待模型执行完成**，避免 B HTTP 网关超时。

### 接口二：查询任务状态及最终结果

\`GET http://<C 内网 IP>:<端口>/api/v1/integrations/institution/investigations/{taskId}\`

仍须带同一套 \`Authorization\`、\`X-Workbench-User-Id\` 请求头；不属于该用户的任务返回 404，不返回其他用户数据。

处理中示例：

\`\`\`json
{
  "data": {
    "taskId": "550e8400-e29b-41d4-a716-446655440000",
    "status": "RUNNING",
    "requestId": "B-20261122-10001",
    "createdAt": "...",
    "updatedAt": "...",
    "result": null,
    "errorCode": null
  },
  "requestId": "..."
}
\`\`\`

成功示例：

\`\`\`json
{
  "data": {
    "taskId": "550e8400-e29b-41d4-a716-446655440000",
    "status": "SUCCEEDED",
    "requestId": "B-20261122-10001",
    "createdAt": "...",
    "updatedAt": "...",
    "result": {
      "summary": "基于真实查询证据形成的最终排查结论（Markdown 文本）"
    },
    "errorCode": null
  },
  "requestId": "..."
}
\`\`\`

状态取值：

| 状态 | 含义 | B 端处理 |
| --- | --- | --- |
| \`QUEUED\` | 已受理，待执行 | 间隔 2–5 秒轮询 |
| \`RUNNING\` | OpenCode 正在执行 | 间隔 2–5 秒轮询 |
| \`SUCCEEDED\` | 收到可验证的已完成最终答复 | 展示 \`result.summary\` |
| \`FAILED\` | 启动/配置/模型/权限等**执行前确定失败** | 展示 \`errorCode\`，联系维护人员 |
| \`UNKNOWN\` | 提交已受理但网络中断、重启等原因使最终执行状态不确定 | **不要自动重试同一次模型调用**；联系管理员核对原任务后由用户另发新请求 |

只返回最终用户可见文本，**不提供内部 reasoning/思考、SQL 原始工具轨迹、工作目录、OpenCode Session ID、原生权限和凭据**。

### HTTP 错误

- 400：日期、msgSn、requestId、用户标识不合法。
- 401：API 密钥错误或来源 IP 不在白名单。
- 404：任务不存在或不属于当前 B 用户。
- 409：相同幂等号用于不同参数。
- 503：服务不可用；B 不应盲目换号重复提交，请先使用原请求号重试查询或幂等创建。

目前没有 \`GET .../events\`、追问、停止或 B 远程设置工具权限的接口。

## 3. C 服务启动与配置

C 的 Windows 工作目录、模型、OpenCode 服务、Skill、DBHub/TeamAI 均沿用原有部署。C 本地 \`/config\` 的 \`institution-message\` 场景应绑定真实存在的机构 Skill（也支持现有配置的 message Skill 兼容回退）；API V1 **只接受 Skill 绑定**，不接受内置聊天、Command 或 Subagent 作为该 API 的自动执行入口，避免外部调用方触发任意命令。

在 C 主机上配置**环境变量**（以下仅为示意，不要在代码仓库保存密钥）：

\`\`\`cmd
set "HARNESS_ENABLE_LAN=1"
set "HARNESS_HTTP_BIND=10.20.30.40"
set "HARNESS_HTTP_PORT=4176"
set "HARNESS_PUBLIC_ORIGIN=http://10.20.30.40:4176"
set "HARNESS_INSTITUTION_API_CLIENT_ID=platform-b"
set "HARNESS_INSTITUTION_API_KEY=<至少32字符的随机高强度密钥>"
set "HARNESS_INSTITUTION_API_ALLOWED_IPS=10.20.30.51,10.20.30.52"
set "HARNESS_INSTITUTION_API_MODEL_ID=kimi"
.\启动工作台.cmd "D:\teamai"
\`\`\`

启动交互菜单请选择 **3「沿用已有环境配置」**，否则普通 LAN 菜单可能覆盖有关 Origin/IP 的设置。Shell 优先选择默认受限模式。B 的两台代理/服务端 IP 需替换为实际出口 IP。实际模型 ID 不一定为 \`kimi\`；如果省略 \`HARNESS_INSTITUTION_API_MODEL_ID\`，将优先选择实际模型目录中的 Kimi，只有一个启用模型时可使用它；多个非 Kimi 模型时明确拒绝，绝不凭空生成模型 ID。

建议为 B→C 使用企业内网 HTTPS/mTLS 网关；如采用内部 HTTP，确保是受控可信网段。C 防火墙**只允许 B 服务端指定 IP**连接监听端口。不要在 Windows 离线包、前端 JS、iframe URL、Git 仓库中存储密钥。若不设置 \`HARNESS_INSTITUTION_API_KEY\`，两条 API **不注册**，原有功能照旧。

**只支持当前 Windows/Mac 单服务（\`--local-only\`）共享执行模式**。若 C 使用隔离执行池或独立 MySQL 多用户模式，启动时会拒绝启用 B API，避免错误复用会话身份或执行权限。

数据库：API 任务状态单独使用 SQLite，默认位于运行系统账号下的 \`.smart-workbench/state/institution-api.sqlite\`；或由 \`HARNESS_INSTITUTION_API_DB_PATH\` 指定工作目录**外部**的本地绝对路径。保留最近 7 天任务记录和最终结论，不新增 MySQL 表。注意数据库备份时保持 SQLite WAL 一致性；不可将保存结果的目录共享给业务用户。

## 4. 执行保障与边界

- 每个任务创建独立 OpenCode Session，沿用 \`institution-message\` 场景的工具权限及 C 启动时的 Shell 约束；B **不能**指定 Skill、模型、工作目录、Session 或工具权限。
- 一次只执行一个 B API 任务，后续任务排队，不抢占现有嵌入页会话。但它们仍共用底层 OpenCode 服务/OS 账户，生产应核实容量和资源隔离，不把 Session 隔离当成操作系统沙箱。
- 接收原生最终完成消息，并核对会话当前无活动执行、无待处理输入后才返回 \`SUCCEEDED\`。若 Skill 请求人工确认权限，当前 API 不支持远程回答，任务不会假装执行成功。
- 运行中服务重启或断电时，任务变为 \`UNKNOWN\`，不自动再次调用模型。这是安全的保守恢复策略，**不是断点续跑**。
- 单任务最多等待 10 分钟；中途工具调用失败、超时或模型链路断开时，应排查 \`errorCode\`，而不是假定“没有进行数据库查询”。
- API 不接受文件上传、自由 Prompt、客户端指定 SQL、脚本或任意资源路径。B 必须在其后端校验当前业务用户是否有权查询该日期和 msgSn，并承担业务审计责任。

## 5. B 后端接入流程

\`\`\`text
1. B 校验客户登录态和“机构报文排查”业务权限。
2. B 后端生成稳定 requestId、带上用户身份与服务密钥，请求 C POST。
3. B 保存 taskId（关联自己的用户与业务请求）。
4. B 轮询 C GET（2–5s），直到 SUCCEEDED、FAILED 或 UNKNOWN。
5. B 在自己的页面展示最终 Markdown 结论（渲染时进行 HTML 转义/安全过滤）。
\`\`\`

B 遇到 POST 超时不要立即换新的 requestId；使用**原 requestId**和完全相同参数幂等重试，避免重复执行。

## 6. 验收清单

- [ ] 客户仅访问 B，B 服务端可调用 C，两条接口从客户浏览器直接访问均失败。
- [ ] 未配置 B 密钥时，API 路由不存在；错误密钥/非白名单来源返回 401。
- [ ] 同用户同 requestId 返回同 taskId，参数变更返回 409，跨用户读取返回 404。
- [ ] 日期按 Asia/Shanghai 全天换算，msgSn 保留前导零，机构 Skill 在配置工作目录中真实生效。
- [ ] 正常任务确实在 C OpenCode 执行，最终结果来自真实模型和业务证据，不是模拟脚本。
- [ ] 同时发起两个 B 请求，Session 不互串，后续排队。
- [ ] 断电/进程重启后任务标记 UNKNOWN，没有重复调用模型。
- [ ] C 原有机构嵌入页、公管问题排查、报文排查、代码审查和配置/统计/组件页行为无回归。
- [ ] 已配置企业内网认证、IP 白名单、HTTPS/mTLS、数据保留与日志审计策略。

本仓库自动化测试使用**脚本化模拟 OpenCode Peer**，只能验证本项目代码契约；真实私有模型、B 网络、业务 SQL 权限和进程重启必须由公司内网进行最终验收。
