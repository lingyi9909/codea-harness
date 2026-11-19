# Code Harness T01｜公司 OpenCode 能力兼容矩阵

> 状态：**PENDING / M0 BLOCKED**。本表只是已确认的验证项目，不是实机验收结果。
> 2026-11-03：未连接公司 Windows / OpenCode / 私有模型；不得由公共 v2 协议、单测或 HTTP 2xx 推导支持。

| 能力 | 必需 | 现状 | Windows 实测证据 |
| --- | :---: | --- | --- |
| service.identity / service.authentication | 是 | UNKNOWN | 公司 OpenCode 实际 build、服务身份、位置、认证失败与成功日志 |
| session.create / session.location | 是 | UNKNOWN | 独立 snapshot 目录的真实 Session ID、location，确认原仓库不变 |
| agent.trusted / command.trusted / skill.trusted / template.trusted | 是 | UNKNOWN | 原生加载来源、文件 digest、首个 skill 动作和五章模板引用 |
| model.private | 是 | UNKNOWN | 私有 Provider/model alias 实际执行轨迹、断公网验证 |
| command.execute | 是 | UNKNOWN | `codea-review --base harness-review-base` 真实任务从提交到终态 |
| permission.native | 是 | UNKNOWN | 同 Session/Run/Request 原生请求、一回允许和一次拒绝的效果 |
| messages.read / events.read / session.state | 是 | UNKNOWN | 原生消息、事件游标/快照、执行终态一致性 |
| report.persist | 是 | UNKNOWN | 任务副本实际写入、起止时间、五章内容、SHA256 与回读 |
| discussion.same_session | 是 | UNKNOWN | 同 Session ID 追问、消息历史与范围不变 |
| session.interrupt | 是 | UNKNOWN | 原生中断请求、活动态及待处理请求对账 |
| network.offline / repository.immutable | 是 | UNKNOWN | 无隐式公网/下载、原 Git HEAD/index/worktree 前后对比 |
| forms.native | 是（若有） | UNKNOWN | 原生表单列举、回复或取消；确认构建是否使用表单 |
| instruction.entry | 否 | UNKNOWN | 结构化上下文写入且不启动额外模型；不支持则禁用 notes |
| session.log | 否 | UNKNOWN | 原生持久游标/重放边界；不支持则事件 continuity=GAP |

**强制规则：** SUPPORTED / UNSUPPORTED 必须关联可复核的 Windows 真实调用证据；无证据为 UNKNOWN。任何 required=UNKNOWN/UNSUPPORTED 均不得进入 T02 依赖原生执行的开发。可选 instruction entry 缺失时禁止 notes；持久日志缺失时按 GAP 退化，但必须由消息、状态及待处理请求对账补足。

### 当前实际确认

- 设计基线：PR #67 / `b92549ad43718f0e0579cbcb43437d16f0e3c9c7`。
- 新开发分支：`feature/code-harness-web-workbench`。
- 已提供：契约类型、能力门禁、原生 HTTP loopback 探测入口、报告文件校验、单元测试源文件。
- 未完成：公司 build/SDK 锁定、权限与表单 API 的逐项映射、Windows OpenCode 全链路、离线 npm ci 包和实际测试证据。均保持 PENDING。
- 发现潜在契约差异：既有 Codea Review Skill 默认扫描 staged/unstaged/untracked，正式 Web 必须通过独立、干净、固定 OID 任务副本实机证明仅审查 COMMITTED。不能只检查 Command 接受 HTTP 204。

### 验收文件约束

提交：脱敏原生请求/响应结构及时间戳、哈希后的 Agent/Command/Skill/模板来源、真实报告 SHA256、原仓库前后 HEAD/index/status 的对比、网络阻断观察、`npm run typecheck` / `npm test` 终端输出、Node/Windows/Git/OpenCode 构建。不得提交 Token、Cookie、数据库 DSN、内部源代码正文、敏感路径或完整私有模型回复。

- 2026-11-03：已创建 [T01 Draft PR #68](https://github.com/lingyi9909/codea-harness/pull/68)；GitHub hosted Windows 静态依赖安装/DTO生成/typecheck/单测已有成功记录。**这不是公司 Windows 实测**，仍保留所有 REQUIRED 原生能力 UNKNOWN。
