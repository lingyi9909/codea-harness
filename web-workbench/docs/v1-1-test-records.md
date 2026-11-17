# V1.1 阶段测试记录

日期：2026-11-17。基线 PR #111：90b54e88c0c85e4b41f8c0bb7ffc7dce4bd87097。
结论：开发草稿；尚未满足全量 V1.1 验收，不得直接生产发布。

## 阶段提交与验证

| 阶段 | 远程 SHA | 验证与边界 |
|---|---|---|
| T1 | ac735b468c3953cda5f4fba41c63b6b25f0cdea2 | 身份核心 2/2 通过；三人登录、稳定 ID、冲突、CSRF、撤销与过期。使用测试存储，实例重建不等于真实进程/MySQL 重启验收 |
| T2 | a1bc138d78ffe51ccd44b14e663840f28ed573cb | 归属守卫与浏览器命名空间 2/2 通过；错 owner/service、删除记录、metadata 不一致、A/B/C 切换缓存均覆盖 |
| T3 | aec6bb11e7277b76c4dd4420c2e6e437eb5634f0 | 执行上下文/配置检查 2/2，HTTP 三用户模拟注入 1/1 通过；真实 OS 隔离待验，多人 Review 调度未实现且禁用 |
| T4 | 98d8837eb4e4a5f500578cd9cd69bfd809e7eba9 | 上下文解析 1/1 通过；普通工具无明文手机号；真实 Skill/MCP 传输绑定仍缺失 |
| T5 | ab5e5dccfdf4453dc6472f0fa2707c106e5b727e | 模拟业务验证 2/2 通过；正确/错误/过期/取消/重放/并发取消、跨用户和输出秘密检查；真实 Skill 编排和短信待接入 |
| T6 | 本文所在提交 | 类型检查/构建通过；新增 10 项通过；现有回归 67 项通过、3 项未通过、1 项跳过；MySQL/Windows/真实 OpenCode 尚未验收 |

## 已执行命令

- `node node_modules/typescript/bin/tsc --noEmit --project tsconfig.base.json`：通过。
- `node node_modules/typescript/bin/tsc --noEmit --project apps/web/tsconfig.json`：通过。
- `node node_modules/vite/bin/vite.js build apps/web --config apps/web/vite.config.ts`：通过，515 模块构建。
- `npm run test:multi-user`：10/10 通过，日志见 [v1-1-multi-user-tests.txt](validation/v1-1-multi-user-tests.txt)。
- 现有 API/capabilities/events/git/reports/workers、聊天历史/渐进显示及迁移静态检查：67 通过、3 未通过、1 Windows 平台检查跳过，完整输出见 [v1-1-local-regression.txt](validation/v1-1-local-regression.txt)。

回归命令：

```sh
node --test tests/api/*.test.ts tests/capabilities/*.test.ts tests/events/*.test.ts tests/git/*.test.ts tests/reports/*.test.ts tests/workers/*.test.ts tests/ui/chat-history-storage.test.ts tests/ui/progressive-messages.test.ts tests/mysql/migration.test.ts
```

该次全套运行早于新增 multi-user-routes 和 user-context 两个文件；这两项后来在新增测试命令中通过，不把两个不同运行的统计拼成全套通过。

## 未通过/未执行

- tests/api/direct-chat.test.ts：Node 26.0.0 发生原生 InternalCallbackScope 断言退出，不能判定该文件通过。
- tests/api/scene-execution.test.ts、tests/capabilities/adapter.test.ts 中需开启测试服务的用例：监听 127.0.0.1 被执行沙箱以 EPERM 拒绝，未通过。
- 未放宽权限、删掉断言或改成成功来规避上述失败。
- 本机最初 npm ci 因 registry DNS 和缺少缓存失败；随后从现有缓存恢复依赖，才能完成类型检查、构建和本记录的测试。缓存恢复并非标准 npm ci 成功证据。
- 新增 `tests/mysql/multi-user.test.ts` 尚未在本机执行：没有可用的 MySQL 8.4 测试服务。它对专用 harness_ci_test 库执行真实迁移、并发唯一性、持久登录和绑定校验；由新增 CI 或测试人员运行。
- GitHub Workflow 配置已提交；CI 是否通过，以具体 Actions Run 为准。
- Windows 浏览器三用户、真实 OpenCode 任务/文件/凭据隔离、原生 Skill/MCP 身份传播和真实日志短信平台均未验收。

## 必须继续研发的阻断项

1. 多人 Review 的 owner 绑定执行器与调度槽位，替换当前多人模式的禁用门禁。
2. 原生 Skill/MCP 的受信任调用传输，并完成发挑战→等待专用 UI→续跑查询；当前后端解析器和安全 UI 不能替代完整链路。
3. Windows/真实 OpenCode 三人并发、进程/目录 ACL、浏览器实测，以及真实短信平台适配条件与内网验证。

部署及回退见 [v1-1-deployment-and-acceptance.md](v1-1-deployment-and-acceptance.md)。
