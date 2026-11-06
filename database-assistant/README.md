# Database Assistant V1.0

一个OpenCode 2.x原生`database-assistant` Subagent，一个专属Skill，复用已有DBHub MCP/官方Skill和TeamAI CLI。支持普通查询、统计分析、多步业务排查与按需知识检索；全部执行规则在Skill正文，references提供展开说明和用例。

**当前交付供V1.0代码审核与整体集成测试。公司Windows私有模型、DBHub/MySQL/TeamAI实测均PENDING，PR保持Draft、不合并。** 按最新开发安排，T2–T4规则一并交付，不再以T1内网测试作为开发阶段门槛；这不代表真实安全验收已经通过。

## 文件及依赖

- [原生Agent](agents/database-assistant.md)：mode/steps/permissions及system正文。
- [专属Skill](skills/database-assistant/SKILL.md)：查询、统计、排查、知识、追问与安全的完整运行规则。
- [DBHub TOML](config/dbhub.example.toml)：两个隔离源和每源两个工具的参考，不是已接通配置。
- references：[查询](skills/database-assistant/references/query-workflow.md)、[统计](skills/database-assistant/references/analysis-workflow.md)、[排查](skills/database-assistant/references/troubleshooting-workflow.md)、[TeamAI](skills/database-assistant/references/knowledge-policy.md)、[安全](skills/database-assistant/references/sql-safety.md)、[输出](skills/database-assistant/references/report-template.md)、[验收](skills/database-assistant/references/manual-verification.md)。
- [研发验证与已知限制](VALIDATION.md)。

运行依赖仅为现有OpenCode、模型、DBHub、MySQL及可选TeamAI CLI；不用Python、新Node.js脚本、配置生成器、安装器、测试平台、独立Runtime、新MCP或额外primary Agent。知识服务不可用时，已明确口径的数据库查询仍可运行。

## Windows离线安装

1. 在可联网环境取得本PR确定Commit的这些原生文件；通过公司已有离线渠道或现有TeamAI分发规范传入内网，不新建分发系统。内网沿用已批准的OpenCode、DBHub及官方dbhub Skill、TeamAI CLI和私有模型，不由Agent下载或安装依赖。记录各实际版本、Skill来源/版本与文件摘要。
2. 在隔离测试目录/数据库操作，只用合成数据。由DBA创建每源独立只读账号，仅对批准表/列或脱敏视图授SELECT，检查角色、间接视图权限，不授写、FILE、EXECUTE或其他库权限。敏感原列不能先读出再遮盖。
3. 在已有DBHub实例手工合并[TOML参考](config/dbhub.example.toml)，不要覆盖其他源或再建MCP服务。核实实装DBHub版本支持的格式；query_timeout/connection_timeout位于sources，readonly/max_rows位于execute_sql。DSN由现有启动环境注入，确认变量设置，不将密码写进文件或仓库。
4. 用现有MCP诊断工具查看完整tools/list，同时核对OpenCode实际注册的**完整权限action**。两者命名可能不同，不能猜后缀。每源必须同时注册search_objects与execute_sql；利用真实Schema和合成标记记录确认来源。交换工具/源的负例必须能发现。
5. 编辑[Agent](agents/database-assistant.md)：将“部署授权数据源：未配置。”改为唯一批准source ID；将两条注释工具allow替换成该源实测的完整action。保留deny-all和两个Skill allow，不增加工具通配符。模板未配置前不能调用数据库。官方Skill名称若不是dbhub，按实际已安装名称同步修改Skill allow和加载指令，不凭名称假装加载成功。
6. TeamAI使用已确认的`teamai recall "帮我查询数据库"`位置参数语法，以及同语法的三个固定主题。管理员沿用已有CLI安装、登录和访问域配置，核实可信可执行文件、PATH/别名及Windows命令解析。命令与Skill映射必须逐字匹配，不能用`recall *`或插入用户内容。四个主题可在安装时成对替换为批准的业务主题；需重新测试。尚未验证时可删除四条shell allow，普通查询仍可用，自动知识增强记录PENDING。见[TeamAI权限说明](skills/database-assistant/references/knowledge-policy.md)。
7. 备份同名文件后，将Agent复制到`%USERPROFILE%\.config\opencode\agents\database-assistant.md`；将整个专属Skill目录复制到`%USERPROFILE%\.config\opencode\skills\database-assistant\`。复用官方Skill既有位置，不复制第二份。若公司配置了自定义目录，使用实装OpenCode实际加载路径并记录，不盲目复制两份。
8. 检查用户/项目/父目录`.opencode`及JSON同名覆盖。Markdown正文由V2加载为system，不另建同名JSON Agent，不使用旧prompt/permission/bash/task配置。重启TUI，确认system、mode:subagent、steps:8、permissions以及两个Skill实际加载。JSON/YAML语法通过不能代替这一步。

## 现有主Agent安全委派

仅限制子Agent不能保证父Agent安全。在数据库专用测试目录原生配置中，为**已经存在且实际使用的主Agent**合并仅委派权限。例如仅当现有ID确实为build时：

```json
{
  "agents": {
    "build": {
      "permissions": [
        { "action": "*", "resource": "*", "effect": "deny" },
        { "action": "subagent", "resource": "database-assistant", "effect": "allow" }
      ]
    }
  }
}
```

这只收紧该工作目录中现有主Agent，不创建primary、不替换system/model、不修改Codea Review；正常编码请退出专用目录或恢复配置。子Agent使用独立权限，父Agent不应直接调用DBHub、TeamAI Shell或其他子Agent。公司实装版本必须验证这个边界，并检查保存授权、全局/项目覆盖、其他可切换Agent及其他MCP实例是否存在绕过路径。

跨委派传递[紧凑上下文](skills/database-assistant/references/troubleshooting-workflow.md)，不假定新子会话拥有旧聊天。摘要不赋予数据源权限，每次重新委派都核对当前部署。切换source由管理员重新授权单源部署，不简单叠加多个来源allow。

## 使用示例

在专用目录启动TUI，通过已收紧权限的现有主Agent委派。将orders替换为批准源，业务名称/指标和时间按真实情况明确。

- “使用database-assistant查询orders源订单10001的编号、状态、金额与币种。”
- “查询用户7在UTC 2026-01-01这一天的待支付订单，编号升序，每页20条。”追问：“下一页。”
- “查看订单表字段和索引，核实订单与支付流水的关联。”
- “按币种统计指定区间的订单数、已知金额总额与平均值，说明NULL如何处理。”
- “按业务时区分日、分状态统计最近七个完整自然日的订单数；状态口径不清时先检索知识。”
- “订单10002有成功支付流水但订单未更新，按只读数据逐步排查，区分事实、原因候选与待验证项。”
- “解释这条SELECT并在核实Schema与安全后执行。”

有返回才给结论；固定知识主题未命中就说明缺失，必要时澄清。完整查询及统计SQL合成示例见references，不能直接当公司业务Schema。

## 安全与预算

MySQL最小权限、DBHub readonly=true、精确源工具授权共同落实安全，模型口头拒绝不是底层阻断证据。示例默认100行、15秒单SQL超时；max_rows不限制扫描成本。steps:8限制模型步骤；Skill的8次DBHub/2次recall/120秒是行为预算，尚无本模块提供的硬任务总时长/总调用/并发限制。SQL超时不自动重试，不在未观测数据库时声称服务端终止。

TeamAI固定Shell资源是原生权限约束，不是独立OS沙箱；实装Windows解析需负例实测。MySQL服务端取消、异常池连接处置、后续查询恢复和取消失败报警都须取证，不能只看客户端错误。全部手工验收见[矩阵](skills/database-assistant/references/manual-verification.md)，无C类证据统一PENDING。

## 升级、回滚及卸载

升级：备份现有Agent、专属Skill、本次调整的主Agent权限及DBHub配置，记录旧Commit。手工替换模块文件，再合并已批准的source/action、固定recall主题与Skill映射；凭据继续沿用现有注入。重启并复验加载、普通查询、权限负例及超时，不以文件复制成功代替升级验收。

回滚：恢复上述备份及原Commit文件，重启并复核；若回滚版本含已知安全缺陷，停止数据库接入。旧生成器/额外database-entry主入口及同名JSON配置应由管理员清理重复加载，不恢复旧辅助平台。

卸载：备份后仅移除本模块Agent和专属Skill；从专用目录配置中还原本次增添的主Agent委派规则。DBHub与官方Skill、TeamAI若共享则保留；专用源/账号是否撤销由管理员确认，不自动删除其他业务配置或数据库。没有常驻新服务需要清理。
