# 智能工作台：页面统计、局域网协作、统一权限、组件测试（PR #115）

## 单一工作台入口
继续使用 Windows 离线包中的 `启动工作台.cmd "D:\teamai"`。按启动菜单选「局域网访问」，从网卡列表选本机私有 IP。服务默认仍为 127.0.0.1，本机配置页面访问地址是 `http://127.0.0.1:4176/config`。

启用 LAN 后，启动终端会输出一条**随机生成的同事访问码**（或者使用事先设置的 `HARNESS_LAN_ACCESS_CODE`）。把电脑局域网 IP、端口和访问码发给获准的同事。每个人使用自己的姓名、手机号与共享访问码登录，获得独立的页面会话及原生 OpenCode Session 归属。该访问码不做手机号短信验证，也不等于企业单点登录。服务重启后，未固定的随机访问码会重新生成。**不要把公网端口开放、不要随意分享访问码，建议用企业 HTTPS 反向代理和身份认证。**

可以直接打开：
- 工作台主页：`http://<本机局域网IP>:4176/`
- 机构报文排查：`http://<本机局域网IP>:4176/embed/institution-message`
- 组件测试：`http://<本机局域网IP>:4176/component-lab`
- **独立访问统计页**：`http://<本机局域网IP>:4176/analytics`（需要工作台登录）

公管问题排查、普通报文排查、公管数据分析和代码审查仍由主工作台侧栏进入。主工作台和配置页共用原始启动工作目录，不新增第二套 OpenCode、MySQL 或服务端口。

**配置页只通过 localhost 访问**：服务端对 `/config`、`/api/v1/scene-config`、`/api/v1/scene-access`、`/api/v1/page-visits` 做环回 IP + Host 双重校验；配置修改继续要求原有管理员名单与 CSRF。远程访问页面不展示配置入口，也不能用伪造浏览器标签页绕过后端校验。

## 访问统计

工作台侧栏点击「访问统计」，或直接打开 **`/analytics` 独立页面**，查看最近 7 天或 30 天的 PV、UV、按天趋势、最近访问时间。页面不再嵌在 `/config` 中；远程同事登录后也能访问，管理员可在 `/config` 中关闭该页面的 LAN 访问。由浏览器页面加载/导航产生统计请求，双重挂载和两秒内快速重复访问会合并；访问日志最多保留 90 天。数据库默认保存于系统账号私有的 `.smart-workbench/state/page-visits.sqlite`，独立于业务工作目录，升级及进程重启可继续统计。

统计只持久化页面编码、访问时间、随机页面运行实例标识的 SHA-256 摘要，不保存 IP、姓名、手机号、报文编号或执行内容。访客标识仅存内存，尤其不向匿名 iframe 的 LocalStorage/SessionStorage 写入追踪值。UV 按当前页面运行实例去重，刷新页面通常会重新计为访客，不等同于企业实名认证人数。

## 页面权限统一配置

在 `/config` 下的「各页面权限管理」选择场景，再配置细粒度权限。文件继续保存在工作目录 `.workbench/scene-access.json`，不覆盖已有 `.workbench/scenarios.json`。权限文件升级为 **version 2**，旧版 version 1 会在读取时兼容迁移，首次保存后写入新结构。

- 局域网访问：每个场景独立开关，统计页 `analytics` 也支持配置；配置页 `/config` 始终本机专用
- **命令与 Agent**：Shell、Code Mode (`execute`)、Skill、Subagent
- **文件与目录**：read、write、edit、glob、grep、list、external_directory
- **联网工具**：webfetch、websearch（已安装对应工具时才实际生效）
- **DBHub MCP**：dbhub_search_objects、dbhub_execute_sql
- **其他 MCP / 工具**：可输入实际 `action` 名称、`resource` 匹配资源，以及允许／询问／拒绝；每个场景最多 16 条

每项原生工具可选「继承默认 / 允许 / 每次询问 / 拒绝」。对应 OpenCode 原生权限规则以 `action + resource + effect` 下发，只有安装且受 OpenCode 识别的 action 才会实际起作用；并非自动安装或配置新工具。代码审查仍是独立 Review 调度器，因此这里仅控制其页面 LAN 权限，暂不声称控制其独立执行器的 Shell 规则。

权限在**服务端**为 OpenCode 新 Session 生效，原生会话的权限回显仍要核验。若旧 Session 配置已不一致，会拒绝后续使用并要求创建新 Session；不能靠前端 UI 按钮隐藏替代权限控制。原生 Shell 的允许权限可以绕过独立的文件写入禁止开关，因而这些选项不是操作系统沙箱。其他名称的 MCP 权限仍遵循 OpenCode 配置，不代表任意 MCP 已被封禁。

机构匿名页在原有基础权限与 `HARNESS_INSTITUTION_ALLOW_SHELL` 启动选项之上叠加统一配置；命令行开关和配置页的「允许」均会放宽匿名机构 Shell，务必配合可验证身份、最小 Windows 账号权限及 LAN 白名单使用。现有独立 Shell/DBHub 业务功能不做替换。

## 组件测试页

`/component-lab` 仅用于 UI 交互验证，不会向业务系统查询或向模型发送提示词。包括原生日期控件、秒级时间输入、一天的时区 / 夏令时边界预览、模拟模型选择器、按钮、弹窗、基础表单以及验证提示。日志问题排查和机构报文排查实际使用同一套 `TimeFields.tsx` 公共日期组件。

## 验收建议

先用本机 `/config` 保存 Shell 和 LAN 场景策略，再由至少两个同事分别从局域网 IP 访问，确认互相无法查看/中断对方的 Session。打开 `/analytics` 验证统计与 7/30 日切换，确认远程 `/config`、配置读写接口仍返回 403；验证组件测试页日期与秒级时间输入，机构页日期和旧业务排查功能仍可用；确认 PV/UV 在刷新、切换场景、重启后稳定并且不会因 React StrictMode 重复统计。

真实内网权限效果、团队网络防火墙、HTTPS 网关和私有模型 / DBHub 仍须在企业内网验证，自动化 CI 不能替代真实数据与工具调用测试。
