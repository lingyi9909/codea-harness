# Database Assistant V1.0 — T1 开发预览

版本 `1.0.0-t1`；来源 `lingyi9909/codea-harness` 的 `feat/database-assistant-v1` 分支。
**T1 尚未验收通过，不能接生产。T2–T4 未实施。** 已交付原生 Agent/Skill、离线配置生成器和 A 类测试；缺少真实 DBHub/MySQL/私有模型执行证据。

## 分发和安装
沿用 Codea Review 的 agents/、skills/ 目录和 TeamAI 原生 push/pull 分发方式。同步整个 Skill 目录，记录分发版本和文件 SHA256。不修改 Codea Review，不添加 LLM 调度器/数据库 MCP，不改 OpenCode 核心代码。

1. 管理员记录实际 Windows、OpenCode 2.x、DBHub、mysql2、MySQL 版本及 DBHub lockfile；禁止以 latest 作为内网部署版本。Python 3.11+ 仅用于离线准备和测试，不是 Agent 执行依赖。
2. DBA 在隔离库配置两个不同的专用只读账号，只授予各自批准的脱敏视图 SELECT；不得具有全局 SELECT、FILE、EXECUTE、写权限、跨库角色或可间接访问敏感数据的视图权限。使用 SHOW GRANTS 及实际拒绝测试双重验证。凭据经公司安全方式注入 DBHub 环境，严禁放入仓库或 Agent 可读目录。
3. 参考 config/dbhub.example.toml，在实际版本验证配置被加载。`${ORDERS_DSN}` 等必须在启动前由管理员确认存在；DBHub 未设置的变量可能保留原样。不同 source 使用不同账号，清理同一服务器上其他高权限 MCP 实例。
4. 管理员导出实际 DBHub 工具和 **OpenCode 实际权限 action 名称**，检查其源映射，按 config/inventory.example.json 结构录入。示例故意为空，不能生成可用配置。每条包含 `source`、`kind`（search_objects 或 execute_sql）、`action`（实际注册的完整动作名）。不得按后缀猜工具名。该清单是管理员信任输入，不接收模型、知识库或数据库文本自动修改。
5. 在准备机运行（PowerShell 示例）：

```powershell
py -3.12 database-assistant/scripts/prepare.py --config database-assistant/config/dbhub.example.toml --inventory C:/approved/tools.json --source orders --out C:/approved/database-candidate.json
```

生成器只输出候选 `agents` 配置，不连接数据库、不执行 SQL、不修改现有设置。默认上限 100 行、15 秒，连接 60 秒以内；减小可行，提高需修改策略并重测。当前严格仅支持示例 TOML 子集，其他字段拒绝，避免默默忽略。

6. 在隔离 OpenCode 配置目录安装 `skills/database-assistant/` 和已批准的官方 `dbhub` Skill；将候选 JSON 的两个 Agent 合并至该环境的原生 V2 `agents` 配置。源 Markdown 是默认禁用数据库的分发模板；**不要同时安装同名 Markdown 与 JSON Agent**。检查项目级/全局同名覆盖与保存的授权，重启后核对实际生效配置。
7. TUI 切换 `database-entry` 主 Agent，再要求“使用 database-assistant 查询 orders 测试库订单 10001”。该入口只允许委派，子 Agent 只允许选定源的两个工具及两个 Skill。不要从不受限 Build 启动并声称父路径安全；原有 Build/Review 不被本补丁修改，现有高权限路径必须由管理员关闭或隔离。
8. 完成 tests/acceptance.md 中 T1 强制拒绝和服务端超时验证，保留脱敏轨迹，经独立审核后再进入 T2。

官方 dbhub Skill 若需要额外 read 权限或实际运行产生其他权限请求，先检查所需路径和版本；本包不默认开放任意 read。加载失败停止。

## 安全边界及已知缺口
- 生成器验证静态约束，不验证管理员清单真实性、MySQL GRANT、实际 OpenCode 合并/权限执行，也不是 SQL 防火墙。
- 单一部署只允许一个源。切换 orders/payments 必须重新准备、重载并验证授权；不实现同一子会话动态授权。不可将两套 allow 简单合并。
- SQL 副作用/资源攻击阻断依赖实际 DBHub 及数据库能力，尚无本地服务端证据；任务总调用数/总时长仅行为约束，尚无硬限制。因此安全验收状态为证据不足，禁止生产。
- T2 固定参数 TeamAI CLI 适配未实施；不会为了 recall 开放 Shell。T3 聚合及异常多轮流程未实施。T4 提供验收步骤，不宣称完成攻击测试。
- 不重新复制官方 DBHub Skill；记录内网安装的来源/版本/hash，确认与实际 DBHub 相容。

## 测试与回滚

```powershell
py -3.12 database-assistant/tests/test_prepare.py
py -3.12 tests/test_codea_review_2_0_hardening.py
py -3.12 tests/fixtures/codea-review-t1/test_git_scope.py
```

安装前备份实际配置及同名文件；不覆盖安装。回滚仅移除新增两个 Agent 配置、database-assistant Skill 并恢复备份。停止专用 DBHub 进程、由 DBA 撤销专用账号访问；保留脱敏审计证据。不要删除其他 Skill 或修改业务库。

## 核验参考
- [DBHub TOML](https://dbhub.ai/config/toml)：sources 超时，tools readonly/max_rows，环境插值。
- [Issue 384](https://github.com/bytebase/dbhub/issues/384)、[PR 386](https://github.com/bytebase/dbhub/pull/386)：客户端超时、服务端取消及连接池问题。后续文档仍说明取消可能失败，不能只确认 PR 合入就放行。
- 本地已获取的 OpenCode v2.0.27 `packages/core/src/tool/mcp.ts`：权限动作是实际注册的 server/tool 映射；本生成器不自行拼接它。公司版本仍须实测。
