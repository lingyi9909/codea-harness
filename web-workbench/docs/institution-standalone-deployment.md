# 一个服务：工作台、机构报文页与配置页

## 启动一次

Windows 离线包解压后进入 web-workbench，沿用现有 Node/OpenCode 环境执行：

```powershell
.\启动工作台.cmd "D:\teamai"
```

离线包已带运行依赖和构建产物，无需 npm ci / web:build。源码开发需先安装锁定依赖并执行 npm run web:build。macOS 使用 `./启动工作台.command "/唯一业务工作目录"`。

| 用途 | 默认地址 |
| --- | --- |
| 工作台 | http://127.0.0.1:4176/ |
| 独立机构报文排查 | http://127.0.0.1:4176/embed/institution-message |
| 独立场景配置 | http://127.0.0.1:4176/config |

三个地址由同一进程、同一端口提供。start-chat.cmd、旧“启动机构报文排查”脚本与 --embedded-only 兼容转到同一服务，无需分别启动。工作台不再挂载配置组件或侧栏入口。主工作台与配置页沿用原本机登录和编辑权限；机构页无需登录，每次打开独立会话。

OpenCode 连接发现、缓存和首次启动复用同一工厂。默认复用 service.json；HARNESS_OPENCODE_URL / HARNESS_OPENCODE_AUTHORIZATION 可指定现有的一个服务。无需执行槽位注册表、多个工作目录或 MySQL 来运行机构页。旧 MySQL Review/任务模块没有迁移到 SQLite，仍按原部署方式使用。

## 同一配置文件绑定 Skill

唯一文件在**启动工作目录**下：`D:\teamai\.workbench\scenarios.json`，不是源码目录。配置项为 logs、message、institution-message、data。

本机打开 `/config`，使用原有资料登记会话，编辑权限仍由 `.workbench/config-access.json` 的 editors 数组控制，首次目录默认 zhangzhanhui。用户名登记不是远程管理员认证；配置写接口继续限于本机有权会话，不对匿名 iframe 开放。

在“机构报文排查”行选择 Skill，填写 OpenCode 已加载的 Skill 名称并保存。以下仅展示机构项，应保留其他 scenes：

```json
{
  "version": 1,
  "scenes": {
    "institution-message": {
      "directory": "D:\\teamai",
      "executor": {"type": "skill", "name": "institution-log-query"}
    }
  }
}
```

Skill 可继续由 TeamAI 分发；name 必须匹配 OpenCode 在该目录下 skill.list 的名称，不带斜杠或路径。不存在或重名明确拒绝，不降级。Command / Subagent 使用相同 executor 结构及原生分发逻辑。普通报文用 message，机构报文用 institution-message，互不覆盖；不自动继承旧 message 绑定。

首次启动只补齐缺失项，不覆盖已有配置。机构项默认 builtin/chat，需要 Skill 时修改该行。单服务机构项的 directory 必须与唯一启动目录一致。页面保存与手工编辑修改同一文件，新请求读取最新绑定；已有会话目录变更要求新建，避免静默切换。升级前的 message-inspection 机构 Session 仍兼容恢复和追问。

## iframe 接入

```powershell
$env:HARNESS_PUBLIC_ORIGIN = "https://workbench.example"
$env:HARNESS_EMBED_PARENTS = "https://portal.example"
.\启动工作台.cmd "D:\teamai"
```

现有网关将 HTTPS 域名代理到 127.0.0.1:4176，保留 Host/Origin/Authorization，关闭 SSE 缓冲。对方只加：

```html
<iframe src="https://workbench.example/embed/institution-message"
  title="机构报文排查" style="width:100%;height:900px;border:0"></iframe>
```

允许的宿主 Origin 用逗号分隔。机构页使用内存 Bearer 凭据，不依赖第三方 Cookie。配置页设置 frame-ancestors 'none'，配置管理保持本机地址访问限制。

## 同事通过本机局域网访问机构页（显式开启）

**正式使用推荐内网 HTTPS 网关 + 身份认证/访问控制**：网关代理到原来的 `127.0.0.1:4176`，保留 Host、Origin、Authorization，关闭 SSE 缓冲；仍然一个启动命令。机构页可能处理敏感银行业务数据，不能把内网等同于可信身份。

如果只是受控内网**测试**，需要同事直接通过 Windows 电脑 IP 访问，可显式启用 LAN HTTP。此模式无传输加密，**不得用于生产银行数据或未认证的高权限 Shell 执行**。先运行 `ipconfig` 查询本机私有 IPv4（以下 `10.20.30.40` 为示例），在同一个 CMD 窗口运行：

```cmd
set "HARNESS_ENABLE_LAN=1"
set "HARNESS_HTTP_BIND=0.0.0.0"
set "HARNESS_PUBLIC_ORIGIN=http://10.20.30.40:4176"
set "HARNESS_EMBED_PARENTS=http://10.20.30.40:4176"
.\启动工作台.cmd "D:\teamai"
```

同事通过 `http://10.20.30.40:4176/embed/institution-message` 访问。如果需嵌入其他内网站点，将精确宿主 Origin 加入 `HARNESS_EMBED_PARENTS`。HTTPS 宿主页通常阻止 HTTP iframe 混合内容，建议改用 HTTPS 网关。

Windows 防火墙只对允许的同事 IP 开放 TCP 4176，禁止公网映射端口。以下为管理员 PowerShell 示例（替换实际 IP）：

```powershell
New-NetFirewallRule -DisplayName "机构报文工作台测试" -Direction Inbound -Action Allow -Protocol TCP -LocalPort 4176 -RemoteAddress 10.20.30.51,10.20.30.52 -Profile Domain
```

LAN 监听默认关闭。启用时同时要求 `HARNESS_ENABLE_LAN=1`，`HARNESS_HTTP_BIND` 为 `0.0.0.0` 或私有 IPv4，`HARNESS_PUBLIC_ORIGIN` 是端口一致的私有 HTTP IPv4 地址；公网 IP、缺少开关或地址不匹配拒绝启动。主工作台和 `/config` 继续执行本机登录校验，**同事直连只用于机构嵌入页，不开放本机管理员工作台**。

## 执行与凭据边界

**本轮恢复原生执行器绑定和分发，未放宽单服务工具权限。** Skill 从目录解析 ID 后通过 session.prompt 的 skills 字段传入；Command 调用 session.command；Subagent 通过 agents/@mention 传入。单服务 Session 仍强制 permissions 通配 deny，创建及后续请求均核验回显，权限回复不能扩大。Skill/命令需要 Shell、文件、MCP 或子任务时仍受此策略限制。绑定成功不等于任意业务工具已获准执行，协议桩不代表真实引擎工具验收。

机构页只填写日期和 msgSn，不收集账号、密码或验证码。页面直接把日历日、当天秒级起止时间、原始 msgSn 和毫秒时间戳范围（to 不含）交给所绑定的 Skill/Command/Subagent。首次提交包含条件，追问只发送问题、沿用同一 Session。**不再调用 /api/v1/embed/institution-query，也不需要配置 HARNESS_INSTITUTION_ADAPTER_URL/AUTH。** 旧适配接口仅保留后端兼容和独立测试，不再是页面前置步骤。

默认从当前可用模型目录选择名称或 ID 含 Kimi 的模型（优先精确名 kimi）；保留真实 provider/variant，可手动切换。无 Kimi 且只有一个模型时自动使用唯一可用项，多个其他模型时提示选择，不虚构模型 ID。模型列表获取失败会提示。

独立页覆盖主工作台固定宽度，开始排查后收起参数区，结果及追问框使用可用宽度；窄屏自动单列。样式只作用机构独立页。

直接发送条件并不放宽前述原生工具权限，依赖 Shell/文件/MCP 的 Skill 仍受通配 deny 限制。真实业务查询和私有模型需内网验收，模型无法取证时必须说明缺口，不得把条件当作已查询证据。

## 持久化与验收

SQLite 首次按版本化迁移建库，默认当前账户私有 `.smart-workbench/state/embedded.sqlite`（Windows 优先 LOCALAPPDATA）。HARNESS_SQLITE_PATH 可指定公共工作目录外的本地绝对路径。正常重启保留原页面归属及证据，SSE 重连不重放模型任务；刷新按约定新建会话。

每个机构页面有独立凭据与原生 Session，A 结束只中断/删除 A，不停止 OpenCode、不清公共目录、不影响 B/C。默认100个未过期页面，可用 HARNESS_EMBED_MAX_PAGES 调整。备份使用 `npm run embedded:storage -- --backup "D:\workbench-backup\embedded.sqlite"`，不要只复制运行中的主文件而遗漏 WAL。旧多槽位/MySQL 数据不自动迁入。本轮只新增配置键，不改变数据库结构。

验收：一次启动访问三个地址；工作台无配置菜单；独立配置页保存/刷新一致；机构与普通报文绑定独立；三人并发原生 Skill 调用和跨会话操作拒绝；重启恢复；匿名凭据不能写配置；表单和查询请求仅有日期范围与 msgSn。真实 Skill 工具权限、业务接口及私有模型仍需内网验证。
