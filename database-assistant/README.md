# Database Assistant T1

一个 OpenCode 2.x 原生 `database-assistant` Subagent，配一个专属 Skill；复用现有 DBHub MCP 和官方 `dbhub` Skill。
T1只做普通只读查询与Schema探索。T2再接TeamAI，T3再做统计/多轮排查，T4做综合验证。
**当前仍待内网实测，PR保持Draft；不代表T1已通过。**

## 文件与运行依赖

```text
database-assistant/
  agents/database-assistant.md
  skills/database-assistant/SKILL.md
  skills/database-assistant/references/manual-verification.md
  config/dbhub.example.toml
  README.md
```

运行只使用现有OpenCode、私有模型、DBHub和MySQL。没有配置生成器、安装器、权限比对器、测试平台或打包程序；不依赖Python或新增Node.js脚本，不新增primary Agent/MCP服务。

## 直接安装到现有 OpenCode

1. 记录实际OpenCode、DBHub/mysql2、MySQL及官方dbhub Skill版本。先在公司Windows隔离测试环境操作，使用专用只读账号和合成数据。
2. 在现有DBHub配置中按 [TOML示例](config/dbhub.example.toml) 核对所需source及两种工具。不要覆盖已有配置或再建MCP服务。`query_timeout`/`connection_timeout`在sources，`readonly`/`max_rows`在execute_sql工具。示例DSN从启动环境注入；确认变量已设置，凭据不得进入Agent/Skill或仓库。
3. 使用现有MCP诊断工具取得完整tools/list，并核对OpenCode实际注册的**权限action名称**与来源。两者名称可能不同；不要仅按命名后缀猜测。通过真实Schema及该源测试记录确认映射。
4. 手工编辑 `agents/database-assistant.md`：将“部署授权数据源：未配置。”改为批准的唯一source ID；在permissions中取消两条示例的注释，并将action替换为该源实际search_objects和execute_sql动作名。保留deny-all及两个Skill allow，不加通配符工具权限。未完成这一步时模板不会放行数据库工具。
5. 备份同名旧文件后，将该Agent复制到 `%USERPROFILE%\.config\opencode\agents\database-assistant.md`，将整个专属Skill目录复制到 `%USERPROFILE%\.config\opencode\skills\database-assistant\`。已安装的官方dbhub Skill沿用原位置，不复制第二份。也可用现有TeamAI分发渠道同步这些原生文件，T1不调用TeamAI知识检索。
6. 检查项目及父目录`.opencode`、用户级同名文件和JSON配置，排除同名覆盖。Markdown正文自动作为V2的system，不另建同名JSON Agent，也不使用旧prompt字段。重启OpenCode，核对实际加载来源、system、mode:subagent、permissions及steps:8。

## 由现有主 Agent 安全委派

使用现有主Agent显式委派，不安装额外主Agent。子Agent权限独立，限制子Agent本身不能保证父Agent安全。
在数据库专用测试目录的原生OpenCode配置里，为**已经存在、实际使用的主Agent**设置仅委派权限。例如仅当现有ID确实为build时：

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

这是现有主Agent的会话工作目录权限收紧，不创建新primary，不替换其system/model。手工合并到该测试目录已有配置；备份后操作，不全局覆盖现有开发权限，不改Codea Review文件。此目录中的该主Agent将不能直接执行Shell、读写文件或调用DBHub，只能委派；需要正常编码时退出该目录/恢复原配置。

OpenCode V2的自定义子Agent使用自己的权限，故可在父Agent拒绝直接DB工具时，仅由子Agent精确调用批准的两种工具。必须在公司实装版本验证这种行为；检查保存的授权、其他可切换Agent、项目/全局规则及其他MCP实例，关闭未经授权的绕过路径。若组织配置不允许这种安全委派，停止接入并说明具体冲突，不放宽权限或重建执行框架。

## 使用

在上述目录启动TUI，选择已收紧权限的现有主Agent，输入：
- “使用database-assistant查询orders测试源中订单10001的编号、状态和金额。”
- “使用database-assistant查看orders测试源中订单表的字段和索引。”
- “使用database-assistant查询用户7在指定日期区间内支付失败的订单，最多20条。”

将orders替换为部署批准的source ID。观察父委派、两个Skill加载、真实Schema查询、只读SQL及中文结果。没有实际工具结果不视为成功。切换数据源需重新核验授权并手工调整该Subagent部署，不合并多个来源的allow来代替授权确认。

## 安全与手工验收

- 每源使用独立MySQL只读账号，只授批准的表/脱敏视图SELECT；核对角色和间接视图权限。不得有写入、FILE、EXECUTE或跨库权限。
- DBHub显式readonly=true；默认100行、15秒单SQL超时。结果行数不限制扫描成本。
- 原生steps限制模型步骤，不是总调用次数或墙钟时限；Skill的8次/120秒是行为预算。硬任务总预算仍未实现，不能宣称生产资源风险已解决。
- 对照 [手工验证清单](skills/database-assistant/references/manual-verification.md)，用现有TUI、DBHub诊断工具及MySQL客户端验证只读、跨源隔离和服务端超时。模型说拒绝不等于工具/数据库已阻断。

没有内网C类证据的项目全部PENDING。精简前的自动化日志和验收包不再属于当前交付，不能证明当前方案可用；历史留在Git提交记录。

## 升级与回滚

升级前备份Agent、专属Skill及本次改动的原生权限/DBHub配置，记下版本或提交。手工替换本模块文件，保留并重新核对实际source/action和凭据注入方式，重启后复验。
回滚时恢复这些备份并重启，不删除其他Skill、不改Codea Review、不自动修改数据库。旧版生成的同名JSON Agent或额外数据库主入口应从实际配置中手工移除，避免与本版Markdown重复加载；旧辅助脚本和离线验收包停止使用。
