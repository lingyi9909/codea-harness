# 机构报文独立页：真实业务执行需要隔离式 OpenCode

## 为什么“阶段 1”后停止

本地单服务匿名模式会在 Session 创建时强制下发 `permissions = [{action:'*',resource:'*',effect:'deny'}]`。
模型可以生成“阶段 1”这类文字，但无法实际调用业务 Skill 所需的工具。继续发“继续”也不是可靠的执行方案。
真正的机构业务查询必须使用彼此隔离的 OpenCode 执行服务，不能为了让工具运行就把单服务通配 deny 直接取消。

## 独立执行模式（支持 SQLite，无需 MySQL）

1. 管理员先准备多个独立的 OpenCode `serve --service` 进程：每个进程使用独立 OS 用户或容器、服务 URL、认证凭据、状态目录与私有工作目录。每个私有目录内预装相同机构报文 Skill 和受控 DBHub MCP，数据库使用只读账号、必要的库表授权、查询超时和行数限制。
2. 准备 `HARNESS_EMBED_EXECUTION_REGISTRY` 指向的 JSON 数组，每项包含 `ownerId`（独立 UUID）、`serviceIdentity`、`url`、`authorization`、`isolation: 'os-user'` 或 `'container'`、`directories`（启动工作目录映射到该执行槽私有工作目录）。不同执行槽的 URL、凭据、目标目录不能相同或重叠。
3. 准备 `HARNESS_EMBED_ISOLATED_CONFIG` 指向的 JSON 对象，包含 `publicOrigin`、空 `clients` 和 `anonymous: {parentOrigins: [...], executionOwners: [上面各 UUID]}`；不设置 `singleService`。父站点必须使用明确合法 Origin，不能用通配符。
4. 设置 `HARNESS_SQLITE_PATH` 为共享工作目录外、与旧单服务模式不同的独立 SQLite 数据库路径。
5. 启动工作台，仍使用原来的 `启动工作台.cmd` 和相同工作目录参数；三个环境变量必须在启动该进程前设置。

示例 PowerShell 环境变量（路径需由管理员按真实部署修改）：

```powershell
$env:HARNESS_EMBED_EXECUTION_REGISTRY = 'D:\workbench-private\execution-registry.json'
$env:HARNESS_EMBED_ISOLATED_CONFIG = 'D:\workbench-private\embed-isolated.json'
$env:HARNESS_SQLITE_PATH = 'D:\workbench-private\isolated-embedded.sqlite'
.\启动工作台.cmd 'D:\teamai'
```

执行注册表示例（仅示意一个执行槽；实际必须为每个访问者配置独立环境）：

```json
{
  "ownerId": "11111111-1111-4111-8111-111111111111",
  "serviceIdentity": "institution-worker-1",
  "url": "http://127.0.0.1:53001",
  "authorization": "Bearer <管理员提供的真实凭据>",
  "isolation": "os-user",
  "directories": {"D:\\teamai": "D:\\isolated\\institution-worker-1"}
}
```

以上对象应放进 JSON 数组。`directories` 的左侧必须等于工作台实际启动目录。工作台只验证目录/URL/凭据隔离的配置结构；管理员必须保证实际 OS 用户/容器确实隔离。

匿名服务配置示例（同样需增加相应执行槽）：

```json
{
  "publicOrigin": "http://127.0.0.1:4176",
  "clients": [],
  "anonymous": {
    "parentOrigins": ["http://127.0.0.1:4176"],
    "executionOwners": ["11111111-1111-4111-8111-111111111111"]
  }
}
```

如需重复分配已完成的执行槽，必须提供可信 resetCommand 并在回收前清理私有运行状态；未配置 resetCommand 的槽安全退役，不会无清理直接复用。

## 验收

首次提交日期与 msgSn 后，核对真正的 OpenCode Tool/Skill 活动、阶段1→阶段2→最终结果、失败错误、只读证据以及 A/B 不同访客隔离。没有原生工具执行证据，不能标记“已完成机构排查”。模型是否按完整 SOP 连续运行，还依赖真实 Kimi 和内网 Skill，模拟测试不能代替真实验收。