# V1.1 阶段测试记录

基线：PR #111 / 90b54e88c0c85e4b41f8c0bb7ffc7dce4bd87097。

## T1：身份与存储核心

命令：`node --experimental-strip-types --test tests/api/multi-user-auth.test.ts`。
结果：2/2 通过。三人并发登录、稳定用户 ID、共享存储的认证实例重建、同号异名冲突、CSRF、退出隔离、可信 Origin、Cookie 和过期测试。
环境：macOS / Node 26.0.0；测试使用 MemoryIdentityStore，不代表 MySQL 8.4 迁移与真实进程重启通过。

限制：npm ci --offline 因 undici-types-6.21.0.tgz 未缓存失败；完整类型检查、Fastify 路由集成和原有回归尚未执行。新增 002 尚未在 MySQL 8.4 执行。不得据此开放多人生产环境。
