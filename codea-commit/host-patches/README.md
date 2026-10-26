# 现有 OpenCode 原生参数拦截补丁

这是对**现有宿主源码**的最小修正，不是新增 Codea Runtime、MCP、插件、Agent 或产品执行器。补丁已在公开上游参考版本开发、测试；公司定制版适配/构建与 Windows/内网验收仍 PENDING。不要直接替换公司二进制、套用到未知版本，或绕过现有发布流程。

## 适用基线与行为

- 仓库：`anomalyco/opencode`。
- 参考版本：`v1.2.27`，commit `4ee426ba549131c4903a71dfb6259200467aca81`。
- 补丁：`opencode-1.2.27-codea-commit-arguments.patch`，包含四处原生源码改动和配套回归测试。
- `/codea-commit` 非空原始参数在共享 `SessionPrompt.command` 开头拒绝，先于日志、Command/Skill/JSON 配置的模板读取、参数替换/默认追加、Shell 和模型。
- HTTP 返回 400 与中文用法；通过既有会话错误事件向 TUI/CLI 展示说明，CLI 返回非零退出码，事件流不可用时也直接根据原生 HTTP 错误显示中文提示。busy/retry 会话不被重置；空/仅空白沿用现有 Build，其他命令路径保持。

不改变 Reviewer 权限、Provider、默认模型、Git hooks 或签名。本补丁只处理精确命令 ID `codea-commit` 的无参数契约，不声称修复其他命令接受任意参数的宿主通用问题。

参考验证：源码专项 **13/13**、已安装测试入口的真实 HTTP/CLI **5/5**、typecheck PASS；上游全包 **1326 PASS / 8 SKIP / 0 FAIL**。这些不是公司/Windows验收。

补丁 SHA256：`1dfda6df7b49b71dc76f3597568b08c49435a882f792b36fc3eb5037b6be5c3e`。机器可读记录见 `VALIDATION.json`。

## 由宿主维护方适配

1. 核对公司 exact 版本、源码来源与构建基线。在**独立宿主源码分支**审核；禁止在业务仓库应用补丁。
2. 若基线恰为上述 commit，可先运行 `git apply --check <补丁绝对路径>`，再 `git apply <补丁绝对路径>`。必须先核对 `git rev-parse HEAD`，不要仅凭 apply 未冲突就认定版本兼容。
3. 若为公司 V2/其他基线，维护方应将同样的前置语义移植到该版本的公共分发入口，并覆盖 TUI、CLI/API、同名 JSON 与 Skill 自动命令；不强行应用此补丁，不新增包装执行器。
4. 按公司既有依赖锁定、构建、签名及发布流程构建**现有 OpenCode**。本包不提供替换二进制，不携带测试依赖。
5. 在隔离验收配置和临时项目运行下列真实入口测试。只有公司 S01—S08 通过并复审后才恢复本模块两份安装资源。

## 测试

上游原生包目录 `packages/opencode/`：

```text
bun test --timeout 30000 test/session/codea-commit.test.ts
bun run typecheck
bun test --timeout 90000
```

完整测试首次在旧工具注册用例上 30 秒超时；单独放宽超时复测通过，随后用 90 秒阈值重跑全套，结果见验收记录。参考测试准备曾遇到过滤安装与 frozen lock 不兼容；隔离环境用 `bun install --no-save --ignore-scripts --filter './packages/opencode'` 完成测试依赖准备，未修改上游 lock/manifest；这不替代公司构建依赖认证。

本 Codea 仓库的**已安装测试入口**验证（区别于旧停用检查）：

```powershell
$env:CODEA_TEST_OPENCODE_EXE = 'C:\approved-test-tools\patched-opencode.exe'
python .\tests\fixtures\codea-commit\test_native_argument_guard.py
Remove-Item Env:CODEA_TEST_OPENCODE_EXE
```

测试仅将两份草稿放入临时配置，实际加载 Command/Skill，发送普通参数、Shell、围栏、边界伪造等载荷，断言中文 400、不生成 sentinel、不进入模型；检查空白仍进入模型选择、CLI 显示中文并非零退出、其他命令不被改写。无模型凭据，故意使用不存在的模型，不验证实际中文 commit 语义。未设置显式二进制则 SKIP，不能计为通过。

`test_command_preprocessing.py` 是旧宿主漏洞/暂停分发对照；`test_native_argument_guard.py` 才是原生修复验收。二者目的不同，不能用“入口不被发现”冒充修复成功。
