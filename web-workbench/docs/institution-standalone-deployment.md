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

## 执行与凭据边界

**本轮恢复原生执行器绑定和分发，未放宽单服务工具权限。** Skill 从目录解析 ID 后通过 session.prompt 的 skills 字段传入；Command 调用 session.command；Subagent 通过 agents/@mention 传入。单服务 Session 仍强制 permissions 通配 deny，创建及后续请求均核验回显，权限回复不能扩大。Skill/命令需要 Shell、文件、MCP 或子任务时仍受此策略限制。绑定成功不等于任意业务工具已获准执行，协议桩不代表真实引擎工具验收。

机构页只填写日期和 msgSn，不再收集账号、密码或验证码。日期仍转为当天起止边界，精度到秒，API 的 from/to 单位为毫秒，to 为次日零点（不含）。后端查询接口仅接收 day/from/to/msgSn；旧的账号、密码、验证码字段会被拒绝。查询继续通过 HARNESS_INSTITUTION_ADAPTER_URL / HARNESS_INSTITUTION_ADAPTER_AUTH 配置的接口取证，接口负责自身所需认证并仅返回可展示的 evidence。未配置接口明确拒绝，本轮未更换查询执行路径，也没有真实内网联调。升级后重新打开机构页建立新会话；之前使用账号参与指纹的查询缓存不自动迁移。

## 持久化与验收

SQLite 首次按版本化迁移建库，默认当前账户私有 `.smart-workbench/state/embedded.sqlite`（Windows 优先 LOCALAPPDATA）。HARNESS_SQLITE_PATH 可指定公共工作目录外的本地绝对路径。正常重启保留原页面归属及证据，SSE 重连不重放模型任务；刷新按约定新建会话。

每个机构页面有独立凭据与原生 Session，A 结束只中断/删除 A，不停止 OpenCode、不清公共目录、不影响 B/C。默认100个未过期页面，可用 HARNESS_EMBED_MAX_PAGES 调整。备份使用 `npm run embedded:storage -- --backup "D:\workbench-backup\embedded.sqlite"`，不要只复制运行中的主文件而遗漏 WAL。旧多槽位/MySQL 数据不自动迁入。本轮只新增配置键，不改变数据库结构。

验收：一次启动访问三个地址；工作台无配置菜单；独立配置页保存/刷新一致；机构与普通报文绑定独立；三人并发原生 Skill 调用和跨会话操作拒绝；重启恢复；匿名凭据不能写配置；表单和查询请求仅有日期范围与 msgSn。真实 Skill 工具权限、业务接口及私有模型仍需内网验证。
