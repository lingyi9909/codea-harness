# 机构页直接执行、Kimi 默认模型与结果宽度

基线：47a9ecbfb6815844ef28fff4ad90a687201f5beb。

- 默认从可用模型列表匹配 Kimi（ID/名称、大小写不敏感、精确名优先）；保留真实 provider/variant。不覆盖等待请求期间的手动选择，无 Kimi 时沿用唯一可用模型或提示选择。
- 删除 InstitutionMessageInvestigation 的证据预取调用及“业务查询接口未配置”前置中止。首次把日期、当天起止时间、原始 msgSn 和毫秒边界 JSON 交给 scenes.institution-message 绑定的原生 Skill/Command/Subagent；追问沿用同一个会话。旧后端接口兼容保留，页面不再调用，不需要 HARNESS_INSTITUTION_ADAPTER_URL/AUTH。
- 修正独立页继承主工作台固定宽度的问题：窄参数栏，提交后收起，结果和追问铺满可用区域；样式仅限机构独立页。
- 不删除服务端归属校验，不更改单服务原生权限策略。直接提交不意味着 Shell/文件/MCP 等受限工具已放行，真实查询仍需内网验证；模型无法取证必须说明，不伪称已有查询证据。

本地 macOS / Node26：前后端类型检查、生产构建和真实进程启动通过；综合99项，98通过、0失败、1项Windows专项按平台跳过。新增纯函数测试覆盖Kimi名称/ID识别、精确匹配、provider/variant保留及空/无匹配列表。

浏览器回归同时在Windows离线流水线和Linux运行：配置两个模型默认选Kimi；不提供业务适配器，三个iframe只填日期/msgSn就调用绑定Skill；检查真实原生入参含Skill ID和时间条件、前导零不变、没有institution-query网络请求；重启恢复/追问不重复初始化；检查1920、1440、768、390宽度下结果占比及无横向溢出。最终CI及下载见PR #114。
