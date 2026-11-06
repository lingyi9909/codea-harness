# Database Assistant V1.0 Implementation Plan

Goal: 原生 OpenCode V2 TUI 单一 Subagent，复用 DBHub 和 TeamAI。
Architecture: Agent/Skill 按现有目录分发；Python 3.11 标准库离线生成候选权限与检查配置，不运行 SQL、不引入调度框架。
Spec: 用户提供的 Database Assistant V1.0 原任务书及强制补充基线（2026-11-06）。

## 约束及实施顺序
- T1 安全未实测通过，不进入 T2；本次交付 T1 实现及后续验收用例，不能宣称 V1 完成。
- `mode: subagent`、V2 `permissions` 数组、默认 deny；不开放 shell、edit、任意 read。
- DBHub 只读账号、数据源隔离、100 行/15 秒初值，实际版本和工具名由内网导出。
- 不改 Codea Review、Provider、默认主 Agent；新增独立受限委派入口。
- 不保存凭据；A/B/C 分开。独立审核由用户另行安排。

## Task 1 配置生成与安全静态检查
Files: database-assistant/scripts/prepare.py, tests/test_prepare.py, config/。
Interface: prepare(config: dict, inventory: dict, source: str) -> dict，生成原生 V2 agents 配置；不从命名规则推导 MCP 权限动作。
- [x] 写失败测试：来源不存在/重复、工具未注册/重复、通配符/内置 action 注入、只读关闭、无超时、无行数上限、明文 DSN、多个来源隔离。
- [x] 实现严格输入校验、单来源配置生成、父 Agent 默认拒绝工具，子 Agent 仅允许所选来源。
- [x] 运行测试并保留输出。A 类不能证明实际权限执行。

## Task 2 原生 Agent/Skill 与分发文档
Files: agents/database-assistant.md, skills/database-assistant/SKILL.md, references/, README.md。
- [x] 复用官方 dbhub Skill；普通查询流程、停止条件、证据模板。
- [x] 权限由准备脚本产生候选配置，安装前核对实际生效权限及父会话绕过。
- [x] 内网安装不覆盖原有文件，明确回滚及版本清单。

## Task 3 验收材料
Files: evidence/report.md, tests/acceptance.md。
- [x] T01–T18 和 T14 扩展全部列出；不虚报 SQL、模型或数据库权限测试。
- [x] 隔离库超时、服务端取消失败、连接池恢复验证步骤。
- [x] 本地回归、GitHub commit/PR。T2–T4 等待 T1 强制安全门槛。

## Review focus
1. 新工具/数据源误放行：未知默认 deny。
2. 工具名称碰撞内置 action：仅显式 MCP action，拒绝内置名称。
3. 配置伪装为只读：严格 bool/int 和 source-tool 对应检查。
4. 父 Agent 绕过：专用父入口默认 deny；现有 Build 不宣称受保护。
5. 安全证据混淆：候选配置不代表部署通过，真实运行单独待验收。

执行结果：T1静态实现与A类验证已完成；真实安全门槛未通过，T2–T4停止在待验收状态。没有声称阶段完成。
