# 嵌入式报文排查阶段验证

基线：PR #112 / 1842914c8dfe105d7215756aa2a604207a36159b。
分支：feature/workbench-embedded-message-v1。原单机入口保留，PR #110/#113 未改动。

## 阶段一：服务端接入与归属

新增 003 迁移、宿主配置/主体映射、一次性票据、数据库摘要会话、报文专属 API、单页面 Session 绑定、原生服务实例/metadata 核验和过期清理。

`node --test tests/api/embedded-message.test.ts` 的三项用例本地通过：

1. 三用户并发 + 同一用户第二窗口；读取、改名、删除、消息、中断、Permission、Form、SSE 的跨 grant 请求均拒绝；关闭 A 不改变 B/C。
2. 过期 grant 不可使用，后台清理只停止其绑定会话。
3. 单次兑换/并发重放/过期、宿主后端 Key、Origin、CSRF、目录、场景、附件、历史、重复创建与敏感 Form 拒绝；服务端认证实例重建可恢复有效 grant；metadata 不匹配时禁止清理。

OpenCode 使用脚本化客户端；本阶段不以此证明真实 OS/容器或内网模型隔离。

## 阶段二：独立页面与 iframe

复用现有报文表单、结果、追问及权限/Form 展示；独立页无工作台导航、登录、手机号或历史。模型明确选定后才能提交，唯一模型可自动选定；票据/会话凭据均不写 URL/Cookie/浏览器存储。SSE 使用携带 Authorization 的 fetch 流，不在 EventSource URL 放凭据。

- `npm run test:embed`：3 项 API + 1 项分块 SSE，共 4/4 通过。
- `npm run test:embed:ui`：真实 Chromium iframe 三页面，1/1 通过。覆盖已有工作台 Cookie 但嵌入 API/SSE 仍不携带 Cookie、三个页面各自结果、刷新新页面空白、其他页面不变、浏览器存储无持久化。
- 测试截图见 [新页面](validation/embedded-message-fresh.png) / [结果页](validation/embedded-message-result.png)，来自真实 Chromium + 模拟 OpenCode。

## 阶段三：兼容、迁移与接入交付

- 前后端 TypeScript 检查通过，Vite 生产构建通过。
- 原有 API/capabilities/events/git/reports/workers、聊天缓存/渐进消息及迁移静态检查：77 项，76 通过、0 失败、1 项 Windows 平台检查在 Linux 跳过。
- 新增真实 MySQL 8.4 专项：原子票据兑换、会话绑定、撤销/清理、跨实例 advisory lock、独立连接池保持普通查询可用。由 CI 的独立 harness_ci_test 库执行，本地无 MySQL 服务，未虚构本地通过。
- CI 扩展原 V1.1 workflow 运行嵌入 API/SSE/MySQL/Chromium，并上传实际截图。

本地第一次使用系统 Chromium 时，执行环境拒绝其 Unix socket；随后使用 Playwright 自带 Headless Shell 成功执行浏览器测试，没有修改产品权限或削弱断言。

实际内网的宿主 SSO/后端、真实报文 Skill、私有模型、独立 OS/容器权限与真实业务系统仍由使用方联调。此 PR 优先交付嵌入页，不声明原 V1.1 的多人 Review 调度已完成；手机号和短信协同不再作为本入口依赖。

具体启动、API、前端宿主代码、账号映射、迁移和回退见 [接入文档](embedded-message-v1.md)。最新 CI 结果以 PR 中的 Actions Run 为准。

## 首次 CI 与补充校验

在 fc6bbda8820ce51f88a85f96b7b4b0adde909cd9 上，Run 44212965685 已通过回归、4 项嵌入专项、原身份 MySQL 专项，以及新增的真实 MySQL 8.4 票据/撤销/清理/跨实例锁测试。其浏览器安装步骤因 ubuntu-latest 升为 Ubuntu 26.04、Playwright 1.56.1 不支持该系统而失败；将本工作流固定为 Playwright 包内列出的 Ubuntu 24.04，保留原浏览器断言重新执行。

补充校验：管理员从配置移除 subject/更改宿主来源后，旧 grant 在下次请求即被拒绝并撤销，随后进入清理队列，不能仅等待原有效期结束。对应回归用例加入原票据测试。
