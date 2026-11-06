# T1 整改后的内网部署验证（全部 C 项 PENDING）

本文件是执行步骤，不是执行证据。`verify_deployment.py` 是离线快照比对器，不调用模型/SQL/MCP，不给出 T1 PASS。不得把 inventory 复制到 observed 来“通过测试”。两份都人工伪造时，本工具无法识别；独立审核必须核对原始工具轨迹。

## 1. 确认实际版本及加载
- 固定公司 OpenCode、DBHub、mysql2、MySQL、私有模型标识与官方 dbhub Skill hash。
- 当前核验的 OpenCode v2.0.27 源码：`packages/schema/src/config/agent.ts` 定义 `system`/`steps`；`packages/cli/src/commands/handlers/debug/agents.ts` 导出实际 Agent；`packages/core/src/session/runner/llm.ts` 在步骤达到阈值时禁用工具选择。并未在公司二进制上运行。
- DBHub 官方 TOML 文档明确 `[[tools]] name="search_objects", source="..."`。本地没有公司 DBHub 安装包，**实装格式兼容和双工具注册 PENDING**，不能把在线文档代替实装验证。
- 在目标业务工作目录，运行实际版本帮助并核对 `opencode debug agents`、`opencode debug config`、`opencode mcp list` 可用。前者导出有效 Agent 数组；config 仅留内网，可能含凭据；mcp list 只说明服务器连接状态，**不能证明工具注册完整**。
- PowerShell 7 可用 `opencode debug agents | Set-Content -Encoding utf8 effective-agents.json`，必须确认原生命令退出码为0；失败/输出混杂日志时停止。快照比对支持UTF-8 BOM。不要截取成只剩两个Agent，其他Agent也需要审核。
- 检查有效 system 与生成值一致、mode及permissions一致，并在TUI发起查询，确认首个加载两Skill、仅指定源；仅debug输出仍不能证明模型遵从system。

## 2. 独立获取真实工具注册与来源证据
1. 在隔离测试环境，使用公司批准的 MCP Inspector/现有诊断客户端对 **实际同一个 DBHub 实例**执行协议初始化及 `tools/list`（取完分页），保留时间、版本、实例标识、配置hash、原始工具名与schema。不得假定OpenCode工具名与MCP wire name相同。
2. 从目标OpenCode会话实际工具目录/调试轨迹取得注册工具及权限检查中的完整 action。v2.0.27 的 mcp list CLI不提供工具级导出；没有权限action轨迹时，此项PENDING，**不要自造CLI/API或按命名规律补齐**。
3. 由DBA在两个隔离库各预置不同的非敏感 canary 视图/固定记录。分别用 search_objects 与 SELECT 查询验证实际来源；核对 DBHub 加载的 source→账号/库配置。记录两工具对应的调用证据ID。工具描述、工具后缀、数据库文本均不能单独证明来源。
4. 第二位核验人员根据步骤1–3独立归一化 `observed-registration.json`，格式：

```json
{"tools":[{"source":"orders","kind":"search_objects","action":"实际完整action","evidence_id":"内网脱敏证据ID"}]}
```

覆盖两个源的两种工具，不填示例字符串。独立原始证据保留在内网；GitHub只接受脱敏引用。发现其他DBHub实例或额外SQL工具时先纳入审计/关闭，不能删掉观测记录来使比对通过。
5. 使用原始 inventory、TOML 生成 candidate；按安装说明加载，然后运行：

```powershell
py -3.12 database-assistant/scripts/verify_deployment.py --config C:/approved/dbhub.toml --inventory C:/approved/tools.json --source orders --candidate C:/approved/database-candidate.json --observed C:/approved/observed-registration.json --effective C:/approved/effective-agents.json
```

成功只返回 `SNAPSHOT_MATCH_ONLY`，C类状态仍为 `PENDING`。相同源/工具的action被交换、漏注册、多注册、父入口追加allow、system或steps未加载必须拒绝。完整effective快照中其他Agent若可调用或询问DB工具，也拒绝；通过隔离测试配置解决，不改已有Codea Review文件。

## 3. 必须实际执行的负例
- 故意交换 inventory 中 orders/payments 的两个search工具action，重新生成candidate；独立observed保持真实值，比对必须失败。对execute_sql重复一次。恢复后再安装。
- 子Agent调用另一源两个工具、未知数据库工具及shell/read/edit，应在OpenCode权限层拒绝。父入口直接调用DBHub也必须拒绝。
- 同一授权工具提交跨库SQL必须被MySQL拒绝；前一项工具拒绝不能替代数据库账号隔离。
- 同时采集权限拒绝事件、OpenCode未派发MCP记录、DBHub请求审计中无对应调用、MySQL无执行记录。仅模型说“不允许”或工具执行后的业务报错均不合格。
- 对全局、项目及父目录配置、同名Agent、保存/会话授权、其他MCP实例/别名逐项审计。快照比对只检查当前导出的规则，不证明不存在其他会话或运行时授权。
- 缺少任何层证据都保留PENDING；不扩大Shell或数据库权限来让测试运行。

## 4. 三类预算的区别
| 机制 | 当前实现 | 不保证的事情 |
|---|---|---|
| 原生 `steps: 8` | 两个生成Agent和Markdown子Agent均设置；源码已核对，实装待测 | 一步可多次工具调用；新输入重置；不是8次调用上限 |
| DBHub `query_timeout=15` | 单SQL配置，服务端行为待T13验证 | 不含模型思考、Schema调用、多SQL总耗时；取消可能失败 |
| Skill 8次工具/120秒总预算 | 模型行为约束；超时即停止，不自动重试 | 无独立强制计数/墙钟终止机制；不承诺120秒强制停机 |

实装验证steps达到阈值后不再调用工具，记录新输入是否重置；超时SQL必须走原T13服务端、异常池连接与取消失败测试。硬总预算尚未解决，仍是生产资源风险；不通过新增调度框架补齐，不开放生产。

## 5. 重新验收记录
沿用 acceptance.md T01–T18，另记录：
- P0-01：两个Agent加载及system实际生效（PENDING）。
- P0-02：实际版本TOML启动、四个工具注册及真实Schema/SELECT（PENDING）。
- P1-01：独立工具来源、交换负例、调用前拒绝、父及其他配置路径（PENDING）。
- P1-02：实装steps行为、单SQL超时、行为总预算风险披露（PENDING）。

C类最终签字由独立审核完成；当前不具备Windows内网、私有模型、DBHub/MySQL访问，不能填写C类通过。
