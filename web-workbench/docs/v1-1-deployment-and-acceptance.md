# V1.1 开发交付与 Windows 验收

## 当前结论：开发草稿，尚未达到 V1.1 全量验收

已实现身份/数据库会话、路由归属、浏览器命名空间、独立执行服务选择、受信任用户解析核心，以及业务验证安全端点/UI/模拟器。保留本地入口。**不能将本分支直接作为已验收的多人生产版本。**

尚未完成的实质能力：

1. 多人代码审查的执行器/槽位/调度适配；目前多人模式主动禁用 Review 的 native scheduler 和创建入口，避免复用共享执行器。现有单人 Review 链路保留。
2. 原生 OpenCode Skill/MCP 调用与服务端 UserContextResolver 之间的可验证传输绑定。已提供服务端解析核心和浏览器认证通路，未安装任何能够代表当前原生 Tool 调用者的桥接插件。不得将模型提交的 owner/phone/session 参数当作可信身份。
3. Skill 自动发起挑战、等待安全 UI 回复、继续查询的完整原生编排。POST 挑战端点和协调器可在模拟联调时调用，UI 会轮询等待中的挑战；不能宣称原生 Skill 已自动续跑。
4. 每用户服务的真实 OS 用户/容器/文件 ACL 隔离验证，真实 OpenCode 三用户执行与 Windows 浏览器切人验证。
5. 真实内网日志平台、短信与 Token 适配。当前仅显式 opt-in 的 mock-logs，验证码固定 123456；不发送真实短信。

## 文件、迁移与接口

参见 [文件级实施计划](v1-1-implementation-plan.md) 和 [阶段测试记录](v1-1-test-records.md)。新增 002 迁移创建 wh_user_profiles、wh_web_sessions、wh_chat_session_bindings；001 不变。未自动归属旧单机数据。

新增或调整的 API：

| 方法/路径 | 行为 |
|---|---|
| POST /api/v1/auth/login | username、phone 识别账号；无短信 |
| POST /api/v1/auth/local-session | 复用既有登录页 name、phone；多人模式转向持久身份 |
| GET /api/v1/bootstrap | 当前 Cookie 的 owner、csrfToken、identityMode |
| DELETE /api/v1/auth/session | CSRF 校验、撤销当前 Session |
| /api/v1/chat/** | 每次验证真实用户；Session DB 绑定与原生 metadata 双重核验 |
| GET /api/v1/chat/sessions/:id/business-verification | 本会话模拟业务验证状态 |
| POST 同上 | 本会话发起模拟挑战；本阶段为联调端点，尚未供原生 Skill 调用 |
| POST 同上/:challenge/code | 专用安全验证码提交；不经过聊天或 Form |
| DELETE 同上/:challenge | 取消本会话挑战并失效本会话访问句柄 |

普通用户在多人模式中不能编辑场景配置。配置由部署人员在服务器维护，未以自填姓名授予管理员权限。

## Windows 本地单用户兼容

仍使用原有 `--local-only --workspace "D:\work\project"` 和已有启动包。该模式保留本地 profile、原有 Origin/CSRF、聊天与 Review 流程。不要把此入口直接发布到局域网。

## Windows 多人联调部署

以下仅用于受控联调；上述实质阻断解除后才能做最终验收。已有 OpenCode、Node、TeamAI、DBHub 无需重复安装。

1. 下载本 PR 分支，在 `web-workbench` 运行 `npm ci`、`npm run typecheck`、`npm run web:typecheck`、`npm run web:build`。
2. 备份现有**工作台专用** MySQL 8.4 数据库；配置 HARNESS_MYSQL_DATABASE、HARNESS_MYSQL_HOST/PORT。禁止指向业务库。
3. 使用单独迁移账号设置 HARNESS_MIGRATION_USER、HARNESS_MIGRATION_PASSWORD、HARNESS_MIGRATION_CONFIRM_DATABASE（精确等于库名），运行 `npm run migrate`。检查 wh_schema_migrations 中 001/002 的校验和与执行记录。DDL 部分失败需恢复备份，不可盲目重跑。
4. 配置运行账号 HARNESS_MYSQL_USER/PASSWORD，遵循既有最小 DML 授权；运行账号不授予 DDL。
5. 设置 HARNESS_PUBLIC_ORIGIN 为固定 HTTPS Origin，例如 `https://workbench.example`；反向代理使用此域名和有效证书。后端仍只监听 127.0.0.1，trustProxy=false；代理保留固定 Host 与浏览器 Origin，禁止外部直接访问后端。Cookie 为 HttpOnly+Secure+SameSite=Strict，写请求仍须 Origin 和 CSRF。
6. 启动 `node --experimental-strip-types apps/server/src/start.ts --multi-user --workspace "D:\work\shared-config"`。与 --local-only 互斥。没有执行映射时可登录，但所有聊天执行接口返回 EXECUTION_NOT_PROVISIONED。
7. 三人分别登录，管理员从 wh_user_profiles 查询稳定 owner_id。为每人配置独立 OS 用户或容器、OpenCode 服务、授权头、可写工作目录、临时目录及凭据。各执行环境仅挂载自己的写目录与获准只读定义；禁止挂载其他用户目录、主机根目录或共享业务凭据。
8. 创建受 OS ACL 保护的执行映射 JSON（包含秘密，不加入仓库），通过 HARNESS_EXECUTION_REGISTRY 指定路径，重启后读取。示例结构如下，实际绝对路径必须存在；源目录对应场景配置目录，目标目录由该用户隔离服务访问：

```json
[
  {
    "ownerId": "替换为 wh_user_profiles 的真实 UUID",
    "serviceIdentity": "opencode-user-a-v1",
    "url": "http://127.0.0.1:5101",
    "authorization": "替换为该服务专属授权头",
    "isolation": "os-user",
    "directories": {"D:\\work\\shared-config": "D:\\isolated\\user-a\\workspace"}
  }
]
```

`isolation` 配置只是部署声明，不能证明操作系统隔离已经生效。必须真实尝试跨目录读写并验证拒绝。源目录、场景目录的只读定义需由管理员按原有分发方式放入每个隔离环境；工作台不复制业务凭据或共享 user.json。

9. A/B/C 同时执行，A 中断、退出、模拟异常，确认 B/C 的 Session、临时文件和业务凭据不变。当前多人 Review 被禁用，必须先完成调度适配，才能验收“三人含 Review 并发”。
10. 只在模拟环境设置 HARNESS_ENABLE_MOCK_BUSINESS=1；模拟验证码 123456。用专用 POST 端点发起挑战，UI 出现安全输入框。验证正确/错误/超时/取消/重复提交；检查聊天、原生消息、日志、SSE、报告无验证码/Token。取消模拟开关后不注册这些业务端点。

## 自动化与人工验收

- `node --test tests/api/multi-user-auth.test.ts tests/api/multi-user-isolation.test.ts tests/api/execution-isolation.test.ts tests/api/multi-user-routes.test.ts tests/api/user-context.test.ts tests/api/business-verification.test.ts`
- 在独立 `harness_ci_test` 数据库配置完成后执行 `node --test tests/mysql/multi-user.test.ts`。它会运行迁移、真实并发唯一账号、会话恢复/撤销和绑定授权检查，并清理自身创建的用户。不得改为生产库名来执行。
- 新增 GitHub Workflow `workbench-multi-user.yml` 提供 Node 24 + MySQL 8.4 验证；首次 Run 44037192474 已通过真实 MySQL 8.4.13 测试，详见测试记录。最终提交仍以最新 Actions Run 为准。
- Windows 两浏览器/三用户、真实 OpenCode、文件 ACL 和内网短信仍需使用方实测。HTTP 注入和模拟器结果不能代替这些检查。

## 回退

停止多人服务，保留数据库备份和新增表；回到此前单机包与 --local-only。旧单机记录继续由旧身份访问，新用户记录不转给 local-workbench。清理迁移需另行备份与变更审批，不通过自动 DROP 或修改 001 回退。业务挑战和 Token 只在内存，重启后要求重新验证；不尝试恢复验证码或 Token。
