# 机构报文排查：一个工作目录 + SQLite 文件部署

## 当前交付边界

业务工作目录始终只有一个，例如 D:\teamai。所有机构排查页面的原生 Session.location.directory 都是这个目录。Skill、Command、项目文件和场景配置不复制到每人或每个槽位的另一套工作目录。

机构嵌入入口新增 `--embedded-only` 启动方式，默认使用 Node 自带的 SQLite 文件，不需要安装 MySQL、不需要账号和手动建表。普通 JSON 文件不能直接代替事务及跨进程互斥，未采用 JSON 聊天状态拼接方案。原本地单人入口与旧 MySQL 完整工作台继续兼容；本轮没有把 Review、任务/报告等所有旧 MySQL 模块迁移到 SQLite。

多人仍有不同的 Session、访问凭据、结果、权限回复和执行状态。共享的是只读业务目录，不共享访客临时文件和业务授权。完整工具型原生 Agent 仍使用隔离的执行服务/OS 账户承载各自私有运行状态；本次没有改成所有人共用一套可写进程状态或一套业务 Token。

## 1. 下载与构建

分支 feature/workbench-embedded-message-v1。进入 web-workbench，沿用现有 Windows 离线依赖准备方式，运行：

```powershell
npm ci
npm run web:build
```

已有 OpenCode、Node、TeamAI、DBHub 不重复安装。SQLite 使用 `node:sqlite`，没有增加 sqlite3/better-sqlite3 原生 npm 安装步骤；兼容测试覆盖 Node 22/24/26。较早 Node 22/24 可能显示实验性模块提示，不等于启动失败。

## 2. 唯一工作目录与执行配置

唯一工作目录 D:\teamai。其 .workbench/scenarios.json 的 message 场景指向 D:\teamai，并绑定实际报文 Skill/Command/Subagent；原 /models 不是查询执行入口。已有文件只改 message 项，保留其他场景：

```json
{"version":1,"scenes":{"message":{"directory":"D:\\teamai","executor":{"type":"skill","name":"message-query"}}}}
```

message-query 替换成内网实际名称。机构业务 Skill 不在本仓库，本轮不能假称已经修改或验证内网 Skill 的文件写入行为。

在仅工作台后端账户可读的配置目录保存 execution-registry.json，示例：

```json
[
  {"ownerId":"11111111-1111-4111-8111-111111111111","serviceIdentity":"institution-a-shared-v1","url":"http://127.0.0.1:5101","authorization":"替换为A原生服务完整Authorization头","isolation":"os-user","workspaceAccess":"shared-readonly"},
  {"ownerId":"22222222-2222-4222-8222-222222222222","serviceIdentity":"institution-b-shared-v1","url":"http://127.0.0.1:5102","authorization":"替换为B原生服务完整Authorization头","isolation":"os-user","workspaceAccess":"shared-readonly"},
  {"ownerId":"33333333-3333-4333-8333-333333333333","serviceIdentity":"institution-c-shared-v1","url":"http://127.0.0.1:5103","authorization":"替换为C原生服务完整Authorization头","isolation":"os-user","workspaceAccess":"shared-readonly"}
]
```

不再配置 D:\slot-a\workspace、D:\slot-b\workspace 等映射目录。shared-readonly 模式会自动将唯一启动目录映射到它本身；显式填写 directories 时也必须是同一个路径，不能增加第二个业务目录、子路径或其他写入目标。独立服务/鉴权/OS 用户检查仍保留，不通过删除隔离检查来接受重复配置。

**两层只读边界必须成立：**

1. 工作台在共享模式的原生 Session.create 中设置 V2 permissions：`[{"action":"edit","resource":"*","effect":"deny"}]`。创建响应和后续读取必须回显相同规则；缺失/改变时拒绝执行。网页不能通过“允许一次”覆盖 edit 禁止。
2. Windows/容器同时把 D:\teamai 对各执行账户设为只读。原生 Shell、MCP 或脚本不会天然受文件编辑工具规则约束，必须由实际 OS ACL/只读挂载阻止写入。工作台维护账户可以保留配置写入权；不能仅增加 RX 而保留其他组继承的修改权限。

共享目录只能放所有访问者都获准读取的定义和业务文件。会话日志、临时结果、账号/密码/验证码/Token、数据库和服务配置不得放在该目录；不以 .workbench/user.json 识别访问者。内网 Skill 如果固定写 result.json/token.json，需要改为私有临时目录或接口返回；本模式会拒绝共享目录编辑，不会悄悄恢复写权限。

## 3. 嵌入配置

保存 embed-clients.json：

```json
{
  "publicOrigin":"https://workbench.example",
  "clients":[],
  "sessionTTLSeconds":7200,
  "anonymous":{
    "parentOrigins":["https://portal.example"],
    "executionOwners":["11111111-1111-4111-8111-111111111111","22222222-2222-4222-8222-222222222222","33333333-3333-4333-8333-333333333333"],
    "resetPrivateCommand":["node","D:\\workbench-config\\reset-native-profile.mjs"]
  }
}
```

executionOwners 是并发槽位编号，不是真人账号。三个槽位允许三个页面同时占用；刷新创建新会话，旧页按 TTL 回收，需预留容量。SQLite 并发与持久化不等于无限执行容量。

共享目录模式**禁止旧 resetCommand**，防止通用“清目录”程序误删唯一业务目录。新的 resetPrivateCommand 只接收 `--owner UUID --service 实例标识`，不接收共享目录参数；只能重置该执行服务的私有原生会话/临时文件/业务授权状态，不能删除或停掉其他实例，不得清理 D:\teamai。这个幂等私有状态重置程序仍需按公司原生服务布局实现并验收。缺失/失败时不回收槽位；不能用空脚本退出 0 冒充隔离完成。

## 4. 启动：不安装数据库

从 web-workbench 执行：

```powershell
$env:HARNESS_EXECUTION_REGISTRY = "D:\workbench-config\execution-registry.json"
$env:HARNESS_EMBED_CONFIG = "D:\workbench-config\embed-clients.json"
# 可选；不设置时使用当前工作台账户的私有用户目录。
$env:HARNESS_SQLITE_PATH = "D:\workbench-data\embedded.sqlite"
node --experimental-strip-types apps/server/src/start.ts --embedded-only --workspace "D:\teamai"
```

无需 HARNESS_MYSQL_*，无需 npm run migrate。SQLite 使用独立版本化迁移 packages/storage/migrations/sqlite/001_embedded.sql，首次启动事务建表并记录 SHA256；后续核验校验和，不修改原 MySQL 001～004。

数据库、WAL/SHM 与备份属于后端私有运行数据，不是第二个 OpenCode 工作目录。默认路径位于当前账户的 .smart-workbench/state 下（Windows 优先 LOCALAPPDATA），也可指定本地磁盘绝对路径；程序拒绝把数据库放进共享业务目录，拒绝数据库符号链接与 UNC 路径。Windows ACL 仍需确认执行账户无法读取该私有数据目录。SQLite 本轮支持一台服务器、本地磁盘；不把同一文件放到共享盘供多台服务器写入。

--embedded-only 只提供机构嵌入业务所需 API，不注册工作台登录/Review/旧任务接口；访问 / 自动转到机构页面。原 --local-only 和 --multi-user 启动方式保留，不与新参数混用。

正式部署仍通过固定 HTTPS 域名代理到 127.0.0.1:4176，保留 Host/Origin/Authorization，关闭 SSE 缓冲。仅本机试用时 publicOrigin 可设为 http://127.0.0.1:4176，parentOrigins 对应实际测试宿主；正式 iframe 地址按 HTTPS 页面要求部署。

## 5. 对方接入与机构查询

对方只需：

```html
<iframe src="https://workbench.example/embed/institution-message"
  title="机构报文排查" style="width:100%;height:900px;border:0"></iframe>
```

不接工作台登录、不提供用户身份接口、不接票据或 postMessage。

真实业务查询仍需配置 HARNESS_INSTITUTION_ADAPTER_URL/AUTH。专用适配接口接收日期边界、msgSn、账号、密码、验证码，完成机构系统认证和查询，仅返回 `{"evidence":"允许展示的日志证据"}`；凭据和业务 Token 不经模型、普通聊天或存储。未配置时明确拒绝，不伪造结果。接口契约沿用 [匿名机构交互说明](anonymous-institution-embed.md)。真实机构系统联调尚未完成。

## 6. 恢复、备份与回退

正常重启：原页面内存中的随机凭据继续访问 SQLite 中的授权、绑定和查询状态，SSE 重连，不自动重新认证或调用模型。刷新仍创建新会话；不从浏览器缓存恢复前一个访客。OpenCode 自身中断不保证任务续跑。

SQLite 的写事务只覆盖本地 SQL，不跨网络/模型调用持有写事务；不同执行者使用持久 owner 锁互斥。强制杀掉 API 时，外部重置子进程可能仍在运行，因此不自动抢占死进程锁。相关槽位进入待恢复状态，其余无锁会话继续可用；管理员确认旧 OpenCode 操作及重置程序已停止后，用本机工具恢复，不能自动重放不确定任务：

```powershell
npm run embedded:storage -- --list-locks
npm run embedded:storage -- --recover-lock "槽位UUID" --nonce "列表中的nonce" --confirm-stopped
```

正常运行中可以通过 SQLite backup API 生成一致性备份，目标必须是不存在的绝对路径：

```powershell
npm run embedded:storage -- --backup "D:\workbench-backup\embedded-20261118.sqlite"
```

不要运行中只复制主文件而遗漏 WAL。停止服务后可复制整个私有 state 目录。恢复时停止服务，用完整备份替换数据库文件，处理对应旧 WAL/SHM；保留原目录副本。备份仍须受后端账户 ACL 保护。

旧 MySQL 会话/数据**没有自动导入 SQLite**。首次切换应先结束旧页面、清理私有执行状态并更新 serviceIdentity，再使用新入口；已有旧会话不能假称无感迁移。回退停止新入口、保留 SQLite 备份，恢复旧启动参数和原数据库；不删除业务目录，不修改历史迁移。

## 自动化与内网验收

- test:sqlite：真实文件迁移/校验和、单次票据、持久绑定/查询、备份、真实多进程并发、崩溃锁隔离和独立 HTTP 恢复。
- shared-workspace.test.ts：三会话同一路径、权限缺失/被改拒绝、编辑权限回复不能放行、重置不接收共享目录、共享文件不被删；Unix 非 root 的真实只读文件测试不冒充 Windows ACL 验收。
- test:anonymous:ui：真实 Chromium、宿主仅 iframe、SQLite 关闭重开、三页面共用一个工作目录、凭据隔离、停止和重新打开。
- 新增无 MySQL 服务的 Windows/Linux + Node 22/24/26 工作流。真实模型、内网 Skill、Windows 执行账户读写/跨账户访问、私有状态重置及机构接口仍需内网验收。
