# 平台 B 嵌入机构报文排查：客户无法直连 C 服务时的接入方案

> 开发基线：\`feature/workbench-analytics-lan-permissions-component-lab\` / PR #115。此文是 B/C 联调部署方案，**不改变**原有机构报文排查、OpenCode Session、Skill、MCP、Shell 权限或其他业务页面的实现。正式启用前须经网络、网关和业务安全验收。

## 1. 为什么现有 iframe 不通

客户浏览器能访问 B，B 服务端能访问 C，但客户浏览器不能访问 C。若 B 页面写 \`<iframe src="http://C-IP:4176/embed/institution-message">\`，iframe 内容、脚本和接口都是**客户浏览器主动访问 C**，B→C 网络可达并不能弥补客户→C 不通。CORS、修改 iframe 样式或在 B 页面里抓取一次 HTML 均不能解决整条浏览器请求链。

采用 B 服务端反向代理，保持浏览器只访问 B 可达的 HTTPS 地址：

\`\`\`text
客户浏览器 ──HTTPS──> 平台 B 页面（portal.b.example.com）
      │
      └──iframe HTTPS──> B 的机构专用网关（assistant.b.example.com）
                                │  认证、请求转发、流式透传
                                └──仅 B→C 的内部网络──> C:4176（Code-Harness）
                                                       └── OpenCode/Skill/模型
\`\`\`

iframe 指向 \`https://assistant.b.example.com/embed/institution-message\`，不再出现 C 地址。网页 JS 模块、CSS 资源、\`/api/v1/embed/...\`、\`/api/v1/page-visits\`、流式活动接口全部通过该 **B 域名** 转发。推荐 **独立子域名+根路径代理**：当前 Vite 构建产物使用 \`/assets/...\`，前端使用以 \`/api/v1/...\` 开头的绝对路径。直接挂载到 \`/workbench/...\` 子路径会导致资源/接口前缀不匹配，不能只替换 iframe 的 src。

## 2. C 服务配置（当前代码即可支持）

假设 C 的可绑定地址为 \`10.20.30.40:4176\`，B 父页面的浏览器 Origin 为 \`https://portal.b.example.com\`。

Windows CMD，在**本次启动**所用的同一个终端中设置：

\`\`\`cmd
set "HARNESS_ENABLE_LAN=1"
set "HARNESS_HTTP_BIND=10.20.30.40"
set "HARNESS_HTTP_PORT=4176"
set "HARNESS_PUBLIC_ORIGIN=http://10.20.30.40:4176"
set "HARNESS_EMBED_PARENTS=https://portal.b.example.com"
.\启动工作台.cmd "D:\teamai"
\`\`\`

启动菜单访问方式选 **3「沿用已有环境配置」**；Shell 仍建议选 **受限**。选菜单中的普通 LAN 模式会覆盖 \`HARNESS_EMBED_PARENTS\`，所以有独立 B 父域名时必须使用高级模式或非交互环境变量。开放给 B 的网卡可以是特定私有 IPv4，也可以是显式启用后的 \`0.0.0.0\`，但 C 的防火墙**仅允许 B 代理服务器 IP 访问服务端口**，不得开放到客户/公网。C 到 OpenCode 仍保持原配置。工作目录 \`.workbench/scene-access.json\` 中 \`institution-message.lan\` 必须是 \`true\`，其默认就是允许。

此配置中的 \`HARNESS_PUBLIC_ORIGIN\` **仍是 C 的内网 Origin**，因为当前 C 代码在 \`EmbeddedAuth.origin()\` 中严格比对 \`Host\` 和 \`Origin\`。它不是让客户访问的外部地址；客户外部地址由 B 网关提供。B 父页面的 Origin 则通过 \`HARNESS_EMBED_PARENTS\` 加入 iframe \`frame-ancestors\` 允许列表。请使用父页面的真实 **scheme+host+port**，不是 iframe 的子域名，也不要用 \`*\`。

## 3. B 代理的必要行为

B 平台为客户提供一个可达的 HTTPS 域名，例如 \`assistant.b.example.com\`，转发到 \`http://10.20.30.40:4176\`，并满足：

1. **先认证再代理**：B 端核验用户已登录且有“机构报文排查”权限；所有 HTML、资源、API、流式连接都纳入认证，不能只保护 iframe HTML。建议复用 B 现有网关授权或 \`auth_request\`；SSO Cookie 域、SameSite 策略及子域名登录态应由 B 验证。**C 现有匿名 iframe 凭据不代表 B 用户身份验证**。
2. **只转发最小路由**：\`GET /embed/institution-message\`，\`GET /assets/...\`，\`POST /api/v1/embed/anonymous\`，\`GET/DELETE /api/v1/embed/session\`，\`GET /api/v1/embed/scene-config\`，\`/api/v1/embed/chat/...\`（含 activity 流），可选 \`POST /api/v1/page-visits\`。其他路径如 \`/config\`、\`/analytics\`、\`/api/v1/scene-access\`、\`/api/v1/embed/tickets\` 不应代理。
3. **C 端兼容头**：当前代码不信任任意 \`X-Forwarded-*\` 头，不能直接把客户对外的 \`Host: assistant.b.example.com\` 与 \`Origin: https://assistant.b.example.com\` 透传到 C。**在已经认证、可信、且 C 防火墙只允许 B 到达的专用反向代理层**，把上游请求的 \`Host\` 设为 \`10.20.30.40:4176\`，\`Origin\` 设为 \`http://10.20.30.40:4176\`，令现有 C 内部校验通过。不要对未认证的通用代理或开放的网络路径使用这种头改写。B 网关应自己保留真实浏览器 Origin 以供审计。
4. **流式输出必须透传**：关闭响应缓冲、缓存与压缩，不聚合长连接内容，延长读取超时；保留 \`Authorization: Bearer ...\`、\`X-CSRF-Token\` 和请求体；禁止改变 JSON、活动流、状态码，透传用户中断操作。
5. **浏览器安全头**：B 父页面 CSP \`frame-src\` 要允许 \`https://assistant.b.example.com\`；C 已根据 \`HARNESS_EMBED_PARENTS\` 给嵌入页生成 CSP \`frame-ancestors\`。网关不能额外给嵌入页加 \`X-Frame-Options: DENY\` 或不匹配的 \`SAMEORIGIN\`。不要把 iframe 扩展为可访问管理页面的通用代理。

### B 侧 Nginx 配置骨架

以下是**需由 B 团队落地的配置示意**，不包含 B 的真实认证接口、证书和域名；必须将这些占位值替换并通过企业网关安全审核。建议把共同代理指令放在 \`/etc/nginx/snippets/harness-c-upstream.conf\`：

\`\`\`nginx
# /etc/nginx/snippets/harness-c-upstream.conf
proxy_pass http://10.20.30.40:4176;
proxy_http_version 1.1;
proxy_set_header Host 10.20.30.40:4176;
proxy_set_header Origin http://10.20.30.40:4176;
proxy_set_header X-Forwarded-Proto https;
proxy_set_header X-Forwarded-Host $host;
proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
proxy_set_header Authorization $http_authorization;
proxy_set_header X-CSRF-Token $http_x_csrf_token;
proxy_set_header Connection "";
proxy_buffering off;
proxy_request_buffering off;
proxy_cache off;
proxy_read_timeout 3600s;
proxy_send_timeout 3600s;
gzip off;
\`\`\`

\`\`\`nginx
server {
    listen 443 ssl;
    server_name assistant.b.example.com;
    ssl_certificate     /REPLACE/with/cert.pem;
    ssl_certificate_key /REPLACE/with/key.pem;

    # 严禁无认证部署。B 团队接入其实际 SSO/业务权限鉴权。
    auth_request /__b_auth;
    location = /__b_auth {
        internal;
        auth_request off;
        proxy_pass http://127.0.0.1:9000/REPLACE_WITH_B_AUTH_CHECK;
        proxy_pass_request_body off;
        proxy_set_header Content-Length "";
        proxy_set_header Cookie $http_cookie;
        proxy_set_header X-Original-URI $request_uri;
    }

    location = /embed/institution-message {
        limit_except GET { deny all; }
        include /etc/nginx/snippets/harness-c-upstream.conf;
    }
    location ^~ /assets/ {
        limit_except GET { deny all; }
        include /etc/nginx/snippets/harness-c-upstream.conf;
    }
    location = /api/v1/embed/anonymous {
        limit_except POST { deny all; }
        include /etc/nginx/snippets/harness-c-upstream.conf;
    }
    location = /api/v1/embed/session {
        include /etc/nginx/snippets/harness-c-upstream.conf;
    }
    location = /api/v1/embed/scene-config {
        limit_except GET { deny all; }
        include /etc/nginx/snippets/harness-c-upstream.conf;
    }
    location ^~ /api/v1/embed/chat/ {
        include /etc/nginx/snippets/harness-c-upstream.conf;
    }
    location = /api/v1/page-visits {
        limit_except POST { deny all; }
        include /etc/nginx/snippets/harness-c-upstream.conf;
    }
    location / { return 404; }
}
\`\`\`

**注意**：上述 \`auth_request\` 示例是假设 B 能向本地授权服务验证登录态，是否能在独立子域名获取 B 的会话 Cookie 由 B 的 SSO 系统决定；如不支持，必须由 B 实现受信任的会话/身份桥接，不能删除鉴权。Nginx \`limit_except\` 对部分方法可能返回 403；需要精确 405 可在 B 网关实现路由方法检查。脚本、页面和 API 如新增路径，必须更新代理白名单。生产建议 B↔C 也使用企业内部 TLS/mTLS（可以在 C 前置内网 TLS 网关）。

### B 页面的 iframe

\`\`\`html
<iframe
  src="https://assistant.b.example.com/embed/institution-message"
  title="机构报文排查"
  style="width:100%;height:900px;border:0">
</iframe>
\`\`\`

此时客户**只**请求 \`portal.b.example.com\` 和 \`assistant.b.example.com\`；不会直连 C。客户对 B 的访问授权、流量限额、防滥用与审计由 B 负责；C 仍负责自己的匿名会话容量、Session 归属和工具权限。

## 4. 为什么不推荐这些做法

- **单纯 iframe 改 URL / 配置 CORS**：无法解决客户到 C 的网络不可达。
- **B 服务端先抓取 C 的 HTML 然后嵌入**：HTML 可能显示，但 JS/CSS、API、流式活动仍从客户浏览器请求；如果 B 没有整套代理，依然失败。
- **在 B 同域名 \`/workbench/\` 下直接套一层 location**：当前前端的绝对路径 \`/assets\`、\`/api/v1\` 会跳出前缀，容易和 B 自己的路由冲突；若必须同域子路径，需要另开发前端 base path/资源和 API 前缀适配、重新构建和回归。
- **让 C 对所有客户公网开放**：违背现有网络隔离目的，特别是机构问题排查涉及业务日志及 Shell/Skill/MCP 能力。
- **只代理 HTML、不代理活动流**：排查会表现为“开始了但没有过程”“反复中断”“一次性返回结果”，不能满足现有实时执行页面体验。

## 5. 双端联调验收清单

- [ ] 客户浏览器无法直接连接 \`10.20.30.40:4176\`，仍能打开 B 的 iframe 页面。
- [ ] 浏览器网络面板所有 HTML、JS、CSS、API、流式请求域名均为 B 可达域名，绝无直连 C。
- [ ] B 未登录或无机构排查权限者无法打开 iframe，也不能通过 API 跳过页面鉴权。
- [ ] Nginx 到 C 的连通、\`Host\`/\`Origin\` 改写、代理防火墙白名单均通过。
- [ ] 实际父域名通过 CSP \`frame-ancestors\` 校验，B 父页面 \`frame-src\` 允许 iframe。
- [ ] 同时打开至少两位 B 用户的独立 iframe，Session 不串用、不能中断对方。
- [ ] 日期 + msgSn 排查，模型默认/手选，实时输出、授权交互、追问和停止流程均正常。
- [ ] SSE/长连接不会被代理缓冲、中途截断或超时错误关闭。
- [ ] 通过 B 域名访问 \`/config\`、\`/component-lab\`、\`/analytics\`、配置/权限 API 等均无法成功。
- [ ] 原 C 本机页面、公管问题排查、普通报文排查、独立机构排查及原有 LAN 配置不回归。
- [ ] 端口关闭、C 服务重启、B 到 C 网络中断、匿名容量达到限制时给出可理解的错误。

## 6. 方案与实现边界

这条路径**基于 PR #115 的现有能力即可部署验证**，无需重写 React 页面或 OpenCode 执行器。下一步需要 B 团队提供：可面向客户的 HTTPS 代理入口、B 的鉴权方案、B→C 私网源 IP、父页面准确 Origin，以及网关对流式转发的能力。C 团队提供：实际绑定 IP/端口、机构场景 LAN 策略、父域名白名单和防火墙策略。

此文不表示 B 的网关已经配置或真实内网已经验收；若 B 无权配置独立子域名，只能使用路径前缀，则需要另行设计、实施并测试 C 的可配置 base path，而不是简单更换 iframe src。
