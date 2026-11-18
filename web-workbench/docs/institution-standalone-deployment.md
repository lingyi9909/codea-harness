# 机构报文排查：单 OpenCode 服务简易部署

## 本机三步

1. 下载本分支、进入 web-workbench，沿用已有离线依赖，首次执行 npm run web:build。
2. 保持现有 OpenCode 服务正常，执行：

```powershell
.\启动机构报文排查.cmd "D:\teamai"
```

也可直接运行：

```powershell
node --experimental-strip-types apps/server/src/start.ts --embedded-only --workspace "D:\teamai"
```

3. 打开终端打印的地址，默认 http://127.0.0.1:4176/embed/institution-message。

不需要执行注册表、embed-clients.json、槽位 UUID、多个服务地址、多个工作目录、重置脚本或 MySQL。自动发现并复用现有 OpenCode 服务；找不到且登记文件不存在时只尝试启动一个原生服务。HARNESS_OPENCODE_URL / HARNESS_OPENCODE_AUTHORIZATION 仍可指定公司现有的一个服务，未设置时复用 service.json。

macOS 使用 sh 启动机构报文排查.command "/唯一工作目录"。原 --local-only 工作台保留原来的场景/工具能力。

## 对方只加 iframe

正式发布使用现有网关将固定 HTTPS 域名代理到 127.0.0.1:4176，保留 Host/Origin/Authorization 并关闭 SSE 缓冲。只需设置访问域名与允许嵌入的宿主 Origin：

```powershell
$env:HARNESS_PUBLIC_ORIGIN = "https://workbench.example"
$env:HARNESS_EMBED_PARENTS = "https://portal.example"
.\启动机构报文排查.cmd "D:\teamai"
```

多个获准 Origin 用逗号分隔，不带页面路径。对方添加：

```html
<iframe src="https://workbench.example/embed/institution-message"
  title="机构报文排查" style="width:100%;height:900px;border:0"></iframe>
```

无登录、用户身份接口、宿主后端票据或 postMessage；不依赖第三方 Cookie。

## 一个服务如何多人使用

一个原生服务管理不同 Session。每页独立随机凭据、SQLite 归属记录、原生 Session，所有 Session 使用同一个 --workspace。消息、结果、SSE、Form/Permission、中断和清理按页面归属校验。

A 结束只中断/删除 A 的 Session，不停止服务、不清理公共目录、不重置 B/C。多个页面可以同时提交；实际模型并行度由原生 OpenCode 和模型本身决定。默认允许 100 个未过期页面，可选 HARNESS_EMBED_MAX_PAGES，不是执行槽位。

## 真实查询与工具边界

机构入口沿用“专用接口完成机构认证/取证 → OpenCode 分析证据和追问”流程。真实接口仍需设置 HARNESS_INSTITUTION_ADAPTER_URL/AUTH，接收 day/from/to/msgSn/account/password/verificationCode，仅返回 {"evidence":"允许展示的日志证据"}。适配器负责目标系统认证、授权隔离及脱敏。未配置时明确拒绝，不会调用模型或伪造数据。真实内网联调尚未完成。

**单服务机构入口为证据分析模式，不是任意工具执行模式。** 每个原生 Session 强制 permissions: [{"action":"*","resource":"*","effect":"deny"}]；创建及后续操作核验回显，缺失/扩大权限即拒绝执行，权限确认不能绕过。模型不执行 Shell、文件、MCP、Skill/Command/Subagent 工具，避免通过同一个原生用户目录读取其他会话状态。不要用此独立入口运行需要这些工具的旧 Skill。

这不是关闭权限检查，而是收紧此入口的工具能力。原工作台其他场景继续用原启动方式；不要将 OpenCode 服务密钥交给访问者。真实引擎规则生效仍须内网验证，HTTP 桩不代替模型/工具验收。

## 自动保存与恢复

SQLite 首次启动按版本化迁移建库，无需账号或建表命令。默认文件位于当前账户私有 .smart-workbench/state/embedded.sqlite（Windows 优先 LOCALAPPDATA）。可选 HARNESS_SQLITE_PATH 指定本地绝对路径，不能放进公共工作目录、UNC 或符号链接。

正常重启工作台后，原页面凭内存凭据恢复绑定和安全证据，SSE 重连，不重发验证码或模型任务。刷新按需求新建会话。OpenCode 本身中断不保证任务续跑，不自动重放结果不明的任务。

备份：npm run embedded:storage -- --backup "D:\workbench-backup\embedded.sqlite"。运行时不要只复制主文件而遗漏 WAL；异常强杀留下的不确定 owner 锁仍需管理员核验后显式恢复，正常重启释放锁。

旧 MySQL/多槽位会话不会自动迁入新单服务身份范围。清理按服务身份过滤，不会操作其他服务的会话。原 MySQL Review/任务/报告模块未整体迁移。

## 验证

官方 SDK + 单一真实 HTTP 监听地址：三个 Session 同时活跃、同一目录、停止互不影响、越权拒绝，且没有工具/命令接口调用。真实 SQLite 与 CLI 测试不配置 MySQL、注册表或嵌入文件。浏览器三 iframe 页面验证单客户端、后端/SQLite 重开、追问及敏感字段隔离。Windows/Linux × Node 22/24/26 继续验证；真实机构接口、私有模型与原生规则留待内网验收。
