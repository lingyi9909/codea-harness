# Codea Commit 参数执行阻塞与宿主验收门槛

状态：**OPEN / BLOCKED**。已撤下本模块可自动发现的 Command 和 Skill；暂时停用不等于根治。不合并、不发布、不提供可安装包。原始设计和实现都遗漏了模型之前的信任边界；不将该问题仅归责于实现。

## 原生数据流与已复现结果

固定公开上游：npm `opencode-ai@1.2.27` / macOS arm64，独立 XDG 配置/数据/缓存/状态目录，HOME 保持原值，不读取公司配置；模型设为不存在的 `codea-test-invalid/model`，无凭据，原生工具权限设为 deny。进程只监听本机，测试完成后终止。安全探针仅在新临时目录写入固定 sentinel 文件。

本次测试二进制 SHA256：`3036292cd25a74682d08545ae12de2cf1df864fea812934bfee1ec759f301083`（macOS arm64，不能作为 Windows/公司二进制值）。

对应源码：

- [session/prompt.ts](https://github.com/anomalyco/opencode/blob/v1.2.27/packages/opencode/src/session/prompt.ts#L1755-L1803)，blob `743537f598717633eb7d803cf6de5b1456d50dfe`：原始参数 → 占位符替换或默认追加 → `ConfigMarkdown.shell` → raw Shell 执行 → 模型解析/原生工具/模型提示词。
- [config/markdown.ts](https://github.com/anomalyco/opencode/blob/v1.2.27/packages/opencode/src/config/markdown.ts#L7-L18)：Shell 扫描不以代码围栏作为安全边界。
- [command/index.ts](https://github.com/anomalyco/opencode/blob/v1.2.27/packages/opencode/src/command/index.ts#L126-L139)：Skill 能自动注册成可调用命令；不能只删除同名 Command。
- [当前上游 Command 文档](https://dev.opencode.ai/v2/docs/commands/)：描述参数展开、fallback 追加及模型前 Shell 执行；不是公司定制版证据。

| 探针/阶段 | 实际结果 | 结论 |
|---|---|---|
| 原 Codea Command + 无害 Shell 参数 | sentinel 写出，再返回 ProviderModelNotFoundError | 复现审核阻塞，模型校验太晚 |
| 仅删除参数占位符 | fallback 追加后仍写出 sentinel | 不是修复 |
| 参数放代码围栏 | 仍写出 sentinel | 不是修复 |
| 保留 Skill 自动命令 | 仍写出 sentinel | 只撤 Command 不足 |
| 同时撤下可发现的 Command/Skill 后 | 命令列表无入口，调用不写 sentinel | 仅证明暂停分发/隔离；功能仍 BLOCKED |

55 项原本地 Git/fixture 回归不覆盖这里，不能拿它们证明 Shell 安全。这里使用真实原生宿主，不是模拟参数解析器，但未验证正常提交功能、公司 exact 版本或 Windows/TeamAI。

## 可重复测试（测试用途，不安装到业务环境）

测试脚本不会下载/安装宿主；测试者显式传入已获准的测试二进制绝对路径：

```powershell
$env:CODEA_TEST_OPENCODE_EXE = 'C:\approved-test-tools\opencode.exe'
python .\tests\fixtures\codea-commit\test_command_preprocessing.py
Remove-Item Env:CODEA_TEST_OPENCODE_EXE
```

POSIX 同样设置 `CODEA_TEST_OPENCODE_EXE` 后运行该 Python 文件。未设置则明确跳过，不能记为通过。脚本创建临时项目，复制本模块中实际存在的可发现资源，另生成 raw/fallback/fence/Skill 四个**已知不安全负对照**。负对照预期写临时标记，证明探针有效；Codea 本体要求不写标记且不被发现。当前“绿”代表入口已撤下，不代表可以安装使用。

这份探针用于复现已知 `1.2.27` 行为和核验撤下状态。已修正的公司宿主可能使负对照不再复现，应记录实际差异，不反向修改宿主以让负对照变绿。公司解除阻塞验收按下表独立执行，不能用暂停分发的测试替代功能修复验收。原生宿主二进制仅在测试目录使用，不属于产品依赖或交付物。

## 公司宿主最小适配要求（待维护方核实）

校验必须发生在所有可到达的 TUI、CLI/API Command 共用入口、任何模板替换/默认追加/Shell 扫描之前。建议的语义边界如下，**这是要求示意，不是可安装补丁**；真实位置必须依据公司 exact 版源码确认：

```text
收到原始 command 与 arguments
  如果 command 精确等于 codea-commit 且 arguments.trim() 非空：
    原生返回“当前版本仅支持 /codea-commit”并停止
  否则按原生流程继续
```

不得把拦截放进模型、Skill、后置插件钩子、Shell 权限或提交前 hook。不得通过超大位置占位符丢掉参数，或允许非空参数静默触发提交。此模块仍只允许 Command + Skill，不新增 Runtime/MCP/插件/独立 Agent，也不改 Reviewer 权限、模型或既有 Review 流程。

## 解除阻塞验收（必须先于正常 C01—C16）

所有试验使用独立测试用户/配置和临时仓库；参数载荷只能写临时标记，不含业务操作或凭据。以下均由公司执行，当前 PENDING：

| 编号 | 必须观察到的结果 | 状态/证据 |
|---|---|---|
| S01 | Windows、OpenCode exact 版本、来源/二进制 SHA、实际 Shell 与原生适配 diff/发行证据齐全 | PENDING |
| S02 | 空参数正常路由既有 Build，第一个模型工具为原生 skill(codea-commit)，无预先 Shell | PENDING |
| S03 | 仅空白按设计处理；普通非空/选项/引号/多行参数在原生展开前返回用法 | PENDING |
| S04 | 无害 Shell 载荷、代码围栏/边界伪造都不写 sentinel，不调用模型/Skill/Git | PENDING |
| S05 | TUI 与 CLI/API 各可到达路径均经过同一个前置校验，不存在旁路 | PENDING |
| S06 | 工具 deny 与原有组织权限下仍在原生前处理拒绝；不新增 allow，不借权限弹窗代替拒绝 | PENDING |
| S07 | 同名 JSON/项目配置/Skill 自动注册路径全部覆盖，查明实际来源，不可只测一个 md 文件 | PENDING |
| S08 | 原 Codea Review、Provider、模型与其他命令行为回归保持 | PENDING |

提供原始参数进入点、拒绝点、未触发 Shell/模型的轨迹与 sentinel 缺失证据。仅“模型回答拒绝”不算通过。S01—S08 完成并复审后才恢复草稿为可发现文件、重新生成两资源 hash 与安装包；再进行既有 C01—C16、三次重复、十组真实脱敏说明和 TeamAI 分发验收。
