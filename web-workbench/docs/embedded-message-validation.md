# 嵌入式报文排查阶段验证

基线：PR #112 / 1842914c8dfe105d7215756aa2a604207a36159b。
分支：feature/workbench-embedded-message-v1。原单机入口保留，PR #110/#113 未改动。

## 阶段一：服务端接入与归属

新增 003 迁移、宿主配置/主体映射、一次性票据、数据库摘要会话、报文专属 API、单页面 Session 绑定、原生服务实例/metadata 核验和过期清理。

`node --test tests/api/embedded-message.test.ts` 的三项用例本地通过：

1. 三用户并发 + 同一用户第二窗口；读取、改名、删除、消息、中断、Permission、Form、SSE 的跨 grant 请求均拒绝；关闭 A 不改变 B/C。
2. 过期 grant 不可使用，后台清理只停止其绑定会话。
3. 单次兑换/并发重放/过期、宿主后端 Key、Origin、CSRF、目录、场景、附件、历史、重复创建与敏感 Form 拒绝；服务端认证实例重建可恢复有效 grant；metadata 不匹配时禁止清理。

OpenCode 使用脚本化客户端；本阶段不以此证明真实 OS/容器或内网模型隔离。

