# 统一入口与共享场景配置验证

日期：2026-11-18。基线：6a360470c2ba04c06b696aa3de6ecd9afe9963cc。未更改 PR #110 或合并分支。

## 文件级变更

- start.ts / embedded-start.ts：本地启动组合原工作台与 SQLite 匿名入口，共用一个 OpenCode 连接工厂；旧机构启动方式兼容为相同服务。
- app.ts：同服务提供 /、/config、/embed/institution-message；配置管理保留本机 Host/Origin、会话、CSRF、编辑名单与配置 revision 校验。
- scene-config.ts / contracts/chat.ts：同一 scenarios.json 增加 institution-message 独立键，默认补齐、不覆盖已有配置，无数据库结构变更。
- embed-routes.ts / modules/chat/routes.ts：移除单服务绕过场景绑定的分支，复用原 Skill 目录解析与原生 prompt/command/agent 分发；保留归属及原生权限回显校验。旧机构 message-inspection 会话兼容恢复。
- StandaloneConfigPage.tsx / main.tsx：配置编辑器改为独立文档，复用原 API、登录组件和样式；工作台不再导入/挂载编辑器或提供配置菜单。
- SceneConfigPage.tsx / InstitutionMessageInvestigation.tsx：机构独立配置行和场景标识，普通报文配置保持独立；凭据继续仅交安全接口。
- Windows 启动脚本、部署说明与离线打包流程同步统一入口，解压后验证三个页面地址。

## 本地执行结果

macOS，Node v26.0.0。

- npm run typecheck：通过。
- npm run web:typecheck：通过。
- npm run web:build：通过。
- npm run smoke:local：通过，真实进程提供首页及构建 JS，无 MySQL/模型调用。
- API/capabilities/events/git/reports/workers/SQLite/静态迁移及浏览器缓存等综合测试：97 项，96 通过、0 失败、1 项 Windows 专项按平台跳过。
- 其中新统一服务测试使用官方 SDK 对本地真实 HTTP 协议桩：三匿名访客分别调用绑定的原生 Skill；主 API 与机构 API 解析同一 Skill ID；配置保存/磁盘一致、普通报文不被覆盖、revision 冲突、匿名写配置拒绝、跨会话中断拒绝、A 结束不影响 B/C、旧存量机构会话追问兼容均通过。

综合命令：

```sh
node --experimental-strip-types --test tests/api/*.test.ts tests/capabilities/*.test.ts tests/events/*.test.ts tests/git/*.test.ts tests/reports/*.test.ts tests/workers/*.test.ts tests/sqlite/*.test.ts tests/ui/chat-history-storage.test.ts tests/ui/progressive-messages.test.ts tests/ui/user-storage-auth.test.ts tests/mysql/migration.test.ts
```

远程 CI 增加独立配置浏览器测试，验证工作台无配置菜单、直接访问 /config、权限/保存/刷新、四场景配置共存及机构匿名页面执行。Windows 离线打包在删开发依赖和解压后重复 SQLite 测试，调用真实 CMD 检查三个地址与静态资源；最终结果和包下载地址记录在 PR #114。

## 验证边界

以上是本地文件、真实 HTTP 与受控 OpenCode 协议桩，不是私有模型或真实业务接口联调。单服务 Session 通配 deny 及权限回显校验保持不变；绑定的 Skill 虽已通过原生 skills 附件传递，Shell/文件/MCP 等工具能力仍受现有权限策略限制。机构凭据接口仍待真实内网接入；无适配器时页面明确拒绝，不会把密码、验证码交给模型。原 MySQL Review/任务模块没有迁移至 SQLite。
