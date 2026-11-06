# T1 内网验收交付计划

基线：778baec494c65c4d3013fe3b6c4e4ba53ff642b7，代码审核PASS；阶段C类验收PENDING。
只增加测试与部署材料，不扩展Agent功能。当前没有Windows、MySQL客户端/服务、DBHub或公司私有模型访问。

1. 建立非覆盖的隔离测试工作目录安装脚本；复用prepare输出与官方Skill，不修改原用户配置。
2. 提供固定命名的合成双库SQL、最小授权参考和逐例真值；不含凭据、不自动建账号/清理旧库。
3. 提供仅验收用MySQL客户端脚本：固定用例、先canary检查、失败即停止、拒绝必须匹配权限错误码、服务端超时仅观测不判PASS。
4. 提供DBHub Inspector逐例调用和TUI任务卡；保留OpenCode/DBHub/MySQL三层证据，不替代私有模型操作。
5. 生成C类PENDING记录模板、Windows安装升级回滚说明、离线zip及SHA256；测试脚本自检与既有回归。
6. 通过GitHub连接器提交当前分支、更新Draft PR，不合并。

本轮不能完成的项：公司实际版本、有效配置、实际注册清单、DB权限/超时和TUI模型证据。均明确PENDING。
