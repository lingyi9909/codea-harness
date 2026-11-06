# V1.0研发检查与验收状态

基线：480085e888adfc880e93fb21ff1c98661173be2f。检查日期：2026-11-06。范围：仅database-assistant的原生Agent、Skill、TOML和Markdown说明；不引入脚本/平台，不改Codea Review。最终提交见PR #72最新提交。

## A类检查

- 原生字段与权限配置：YAML经Ruby Psych解析，检查mode:subagent、steps:8、permissions数组、默认拒绝、两个Skill和四条固定Shell资源；没有旧prompt/permission/bash/task字段或动作。Skill正文与固定命令一致。此项为静态检查，不证明Windows真实权限生效。
- DBHub TOML经Python 3.12 tomllib解析：两个唯一source、每源各一个execute_sql/search_objects、映射完整、readonly=true、max_rows=100、query_timeout=15、connection_timeout=10。未接入实际DBHub注册。
- 示例SQL：从查询/统计reference提取10条SQL，在内存SQLite合成夹具核对主键、多条件、下一页、空结果、分币种/NULL、EXISTS去重、先聚合后JOIN、零分母和异常筛选。10/10符合文档预期。仅验证可移植关系运算示例；SQLite不是MySQL，不证明DECIMAL、时区、权限、超时或Agent生成SQL正确。
- 文件范围、相对链接、单Agent/单Skill及无运行脚本检查通过。已知格式的凭据特征扫描未发现匹配，提交内容人工复核仅有环境变量占位及合成数据；不是对未知秘密的绝对保证。
- 既有回归：`python3.12 tests/test_codea_review_2_0_hardening.py`：40项通过；`python3.12 tests/fixtures/codea-review-t1/test_git_scope.py`：4项通过。它们验证已有Codea Review，不是数据库链路测试。

研发环境：macOS、本地Python 3.12及Ruby系统解析器；这些只是检查所用工具，不是交付运行依赖。未安装PyYAML，改用已有Ruby YAML解析；没有新增依赖或仓库测试脚本。原T1已删除的26项脚本测试不再作为当前交付证据。

## 原生实现核对与自查

对照已取得的OpenCode v2.0.27源码：`packages/schema/src/config/agent.ts`声明system/mode/steps/permissions；`packages/core/src/config/plugin/agent.ts`将Markdown正文载入system；`packages/core/src/permission.ts`按最后匹配规则决定；`packages/core/src/tool/plugin/shell.ts`对解析命令资源进行权限校验。配置先拒绝后精确允许。不能据此代替公司实装版本兼容性验证。

自查覆盖：授权源不从摘要继承；父Agent仅委派；知识与结果不可信；金额/NULL/一对多/时区口径；无来源知识降级；SQL超时不重试；预算耗尽输出部分证据。所有必需规则放入Skill正文，references不要求额外文件读取权限。此为研发自查，不冒充用户安排的独立审核。

## P1/P2收敛复核（仅文档与原生配置）

- P1：运行Skill与Agent正文明确单source、同源授权表可JOIN、跨DBHub sources不能自动调查/生成跨源SQL；查询、统计、排查示例标明两表同源前提，跨源问题只给本源事实、证据缺口及人工独立授权处理路径。
- P2：四条默认TeamAI主题只是有限覆盖；管理员可静态替换/增加逐条批准的有限固定命令，Agent白名单与Skill映射需同步，保留原有精确权限与降级策略。
- 本轮没有增加Python/TypeScript或其它执行脚本、Agent、MCP、Runtime、安装器或自动化验收平台。原有A类记录描述此前已完成的检查；本轮文档修订**未重新执行**此前SQLite、Ruby/Python、Codea Review回归检查，不能将旧检查结果冒充本轮复测或公司内网验收。

## B/C类状态与剩余限制

| 验证 | 状态 | 原因/所需证据 |
|---|---|---|
| B公开模型完整工具链 | PENDING | 未运行公开模型连接DBHub/TeamAI的端到端测试 |
| C Windows私有模型加载/委派/查询/统计/排查 | PENDING | 无公司内网访问，按22项手工矩阵整体测试 |
| C 实际注册工具、父子权限、跨库/源、敏感字段 | PENDING | 需要OpenCode派发、DBHub和MySQL三层真实证据 |
| C TeamAI CLI检索与固定命令限制 | PENDING | 用户确认语法，未验证四主题命中效果、来源字段、Windows扫描及可信PATH |
| C 服务端SQL取消、池恢复及取消失败 | PENDING | 需要数据库线程与连接池观测，客户端错误不够 |

未解决的硬能力：没有本模块提供的任务总墙钟/调用总数/并发强制器，steps仅模型步骤；行为预算不是硬隔离。固定CLI主题限制检索灵活性，未命中时需澄清；不得为便利而开放动态Shell。未配置source/action的模板默认不能查库，管理员必须填写实测值。

检查没有提供公司内网安装成功或生产安全的结论。整体V1运行效果及安全仍PENDING；遇真实写入、越权、泄露或无法确认服务端终止应判FAIL。PR保持Draft，提交独立审核后由用户统一集成验收，不自行合并。
