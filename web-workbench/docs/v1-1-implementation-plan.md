# V1.1 文件级实施计划

基线：PR #111，90b54e88c0c85e4b41f8c0bb7ffc7dce4bd87097；依赖 PR #110。不得改写这两个分支。

## 阶段与文件

| 阶段 | 文件（相对 web-workbench） | 交付及检查 |
|---|---|---|
| T1 | `packages/storage/migrations/002_multi_user.sql`、`packages/storage/src/identity.ts`、`apps/server/src/{auth,multi-user-auth,app,start}.ts` | 版本迁移、稳定手机号账号、持久化摘要会话、CSRF、三人登录与重启/撤销测试 |
| T2 | `packages/storage/src/chat-bindings.ts`、chat/routes.ts、各资源 routes.ts、web/src/{app,chatHistory}.tsx/ts、api/client.ts | DB 与原生 metadata 双重授权；所有资源链路 owner 检查；切人销毁组件和用户命名空间缓存；SSE 撤销 |
| T3 | chat/connection.ts、server/config.ts、workers/scheduler.ts、storage/slots.ts | 显式 server 模式；固定 HTTPS Origin/Host；每用户隔离服务与目录；无法证明执行隔离时拒绝执行，禁止共享服务降级 |
| T4 | server/user-context.ts、受控工具桥接 | 服务端绑定调用者和 Session；普通调用无完整手机号；可信业务适配器按需解析 |
| T5 | server/business-verification.ts、业务路由、web/BusinessVerification.tsx、模拟日志适配器 | 专用验证码通路；owner+session+target 授权；过期、错误、取消、重放、并发及秘密泄漏测试 |
| T6 | tests/{api,mysql,ui}、docs/v1-1-acceptance.md | 旧版回归；三用户并发；Windows 部署、迁移、恢复和真实内网验收步骤 |

## 数据库迁移

保留 001 校验和，使用现有 runMigrations 管理新增 002。复用 wh_owners，新增 wh_user_profiles（手机号唯一、状态）、wh_web_sessions（token/csrf 摘要、过期与撤销）、wh_chat_session_bindings（服务实例+原生 Session 唯一、owner、场景、目录、删除状态）。所有 owner 外键与既有 CHAR(36) ascii_bin 保持一致。业务验证码和业务 Token 不入库、不进入普通事件。

升级前备份专用工作台数据库，使用单独迁移账号；MySQL DDL 不具备事务回滚，部分失败应恢复备份并排查后重试。应用回退保留新增表，使用旧单机入口，禁止将新用户数据映射给 local-single-user；只有经过批准的管理员映射才能迁移旧历史。

## 接口清单

| 接口 | 身份及行为 |
|---|---|
| POST /api/v1/auth/login | username+phone 必填，无短信；同号异名冲突；创建持久化 Cookie |
| GET /api/v1/bootstrap | Cookie 恢复本人 ID、显示名、CSRF |
| DELETE /api/v1/auth/session | 当前 Cookie+CSRF；撤销当前登录及订阅 |
| /api/v1/chat/**、tasks/runs/interactions/artifacts/events | Cookie、写操作 CSRF、逐资源 owner 检查 |
| GET /api/v1/chat/sessions/:id/business-verification | 查询本会话业务验证状态，秘密不返回 |
| POST /api/v1/chat/sessions/:id/business-verification/:challenge/code | 专用 UI 提交；验证码仅送业务验证适配器 |
| DELETE /api/v1/chat/sessions/:id/business-verification/:challenge | 取消挑战，后续提交拒绝 |
| 受控工具 checkAccess/sendChallenge/query | 调用上下文由桥接验证，模型不能提供 owner/phone 或读取 Token |

各阶段单独提交测试记录。Mock 通过不等于 MySQL 8.4、真实 OpenCode 或内网短信联调通过；未执行检查必须明确待验收。
