# T1 内网验收包交付记录 — 2026-11-06

当前任务：停止功能扩展，向内网测试人员交付可复现验收包。
代码整改审核：用户转述独立审核PASS，基线778baec494c65c4d3013fe3b6c4e4ba53ff642b7。
**T1阶段：PENDING；无C类证据，不允许合并或进入T2。**

## 本轮交付与实际执行
- 非覆盖式隔离OpenCode工作目录准备脚本；不改全局配置或既有Codea Review。
- 双库合成SQL、最小授权参考、固定用例及预期行值。
- 直接MySQL账号验收脚本：先canary、严格拒绝码、意外成功即停、避免记录异常敏感返回。
- 复用官方Inspector的PowerShell固定用例脚本；不新增MCP/SQL执行框架或Agent Runtime。
- 独立DBA服务端超时观测、TUI任务卡、C类PENDING模板、安装升级回滚手册。
- 显式文件白名单的离线ZIP构建器及SHA256清单；不打包凭据、运行日志、第三方二进制。

A类：34项模块测试、40项Codea Review回归、4项Git范围回归通过，共78项。新增脚本的客户端调用使用单元测试替身验证失败停止逻辑，**不是MySQL或C类实测**。
B类：未执行。C类：INSTALL、T01/02/03/09/10/11/12/13/15/16/18全部PENDING。
当前本机无mysql/docker/pwsh，也无公司内网地址、账号和模型访问。SQL fixture、PowerShell包装脚本、Windows原生加载、实际DBHub注册和权限、数据库超时/池恢复均未执行。

内网人员按acceptance/WINDOWS-RUNBOOK.md执行并填写results.template.json。配置/工具清单/GRANT/原始模型及三层安全轨迹只在真实执行后产生；包内不会伪造这些材料。
硬总任务预算仍未实现，原生steps不等于总调用/墙钟上限。取消失败与连接池状态需实装故障注入及日志，不以脚本空采样或客户端超时替代。

---
以下为上一轮整改记录（历史阶段判定，不替代本轮代码审核PASS）：

# T1 独立审核整改 r1 测试报告 — 2026-11-06

审核/整改基线：66746d0a618030498d4affe5ccdcdf56d35a4c1d。
环境 A：macOS，本机 Python 3.12；不含数据库、DBHub 服务、模型调用。

| 验证 | 结果 | 证据 |
|---|---|---|
| 配置生成/拒绝/CLI不覆盖/错误不回显输入 | 26通过 | automated.txt |
| 既有Codea Review静态与参数回归 | 40通过 | review-regression.txt |
| 既有Git范围临时仓库测试 | 4通过 | git-scope-regression.txt |
| B 外部模型验证 | NOT RUN | 无模型轨迹 |
| C Windows内网私有模型 | 待内网验证 | 无内网连接 |

本轮先添加P0测试，复现旧TOML校验拒绝search_objects及缺少system；修复后18项通过。P1比对器先由7项测试复现缺失，再实现并补CLI端到端检查；最终26项模块测试通过。
这70项A类检查不代表T01–T18业务场景通过。未运行旧Harness Runtime全量Go测试；本补丁不改Runtime和既有文件。

## 阶段判定
T1：**FAIL（验收证据不足，开发预览）**。专用账号、实际工具权限拒绝、MySQL写入/跨库/敏感字段阻断、服务端超时取消和连接池状态均未实测，不能接生产。
T2/T3：未进入。T4：仅准备验收矩阵和操作步骤，安全攻击测试和真实环境验收未运行。

## 交付内容
- 默认拒绝数据库工具的原生Subagent与完整T1 Skill。
- 依据管理员实际清单生成单数据源精确权限及受限父入口的离线脚本。
- 两源只读DBHub TOML示例、空工具清单（默认无法生成可用配置）。
- 安装、分发、回滚、已知问题和T01–T18及扩展T14验收材料。

## 必须补齐的内网证据
1. 实装版本/依赖lockfile、配置hash、实际注册工具与生效权限。
2. 专用只读账号GRANT/角色及写入、跨库、FILE/EXECUTE、敏感原字段实际拒绝记录。
3. 父入口/其他Agent/MCP实例绕过检查与隔离结果。
4. 超时响应、独立服务端观察、连接丢弃、后续查询成功以及取消失败故障注入。
5. 双Skill真实加载、Windows TUI私有模型普通查询轨迹及结果准确性。
6. T1独立审核结论；通过后才能实施T2–T4。

不提交凭据、真实业务数据或原始内网日志。本报告不得作为最终PASS证明。


## 本轮问题处理
| 项目 | 代码/文档处理 | 真实环境状态 |
|---|---|---|
| P0-01 | 父子Agent均改用system，保留mode/permissions，新增steps:8；测试无旧字段 | 加载与system生效PENDING |
| P0-02 | 每源显式execute_sql/search_objects；双工具、重复、错误映射及未知字段校验 | 实装DBHub格式/注册/调用PENDING |
| P1-01 | 独立观测映射与完整有效Agent快照比对；两种工具交换、父路径追加权限等负例 | 原始注册/权限拒绝/其他配置与实例审计PENDING |
| P1-02 | 核对v2.0.27源码steps；保留并区分调用次数、墙钟时间行为预算及单SQL超时 | 原生步骤实装行为PENDING；硬总预算未实现 |

实际生成配置见 generated.synthetic.json，输入 inventory.synthetic.json；它们明确是A类合成示例，**不是实际工具注册清单**。真实清单和工具调用证据尚不存在，不以人工声明替代。

本轮范围仅本模块代码、测试、示例与验收文档；未变更Codea Review。回归40+4项再次通过，未运行旧Runtime全量Go测试。
新增比对器仅读文件，不联网、不执行命令/SQL、不修改权限；不认证证据真伪，不枚举未导出的其他会话，不能代替原生执行层。生产安全门槛仍未通过。

格式参考：[DBHub TOML工具配置](https://dbhub.ai/config/toml)。实际DBHub安装版本未获取，实装确认仍PENDING。
OpenCode源码依据：v2.0.27 packages/schema/src/config/agent.ts、services/www/src/docs/content/agents.mdx、packages/core/src/session/runner/llm.ts；本次未在Windows私有模型上执行。

重新提交独立审核，PR保持Draft、不合并、不进入T2。两项P0代码已修复不等于T1 PASS。
