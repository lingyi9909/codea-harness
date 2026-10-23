# 提交预览与确认：内网验收步骤

本表针对 2026-10-23 更新的 Command/Skill。前置条件：目标现有宿主先通过参数前置校验探针与 S01—S08；旧补丁/版本门禁不因确认交互改变。真实公司 Build、模型、权限不修改。含模板与消息清理修订，全部场景当前 **PENDING**；本地 Git 对照与提示词模拟不能替代本表。

每项使用新的独立 fixture 仓库，manifest/消息/轨迹留仓库外，不交给模型。C01—C03 至少各三次。记录完整 TUI 轨迹和实际 Git 结果；不能只截成功文案。真实敏感源码/配置不上传，PR 仅附脱敏结论和内网记录位置。

| ID | 操作与预期 | 内网结果 |
|---|---|---|
| P01 | C01 输入无参数命令；先概览、清单、每文件真实暂存 Diff、完整说明，完整展示前无最终确认。停在等待：HEAD/index/worktree 不变。 | PENDING |
| P02 | C02 部分暂存；只预览 A，明确 B 与 other.txt 未纳入。选择确认后恰好一个提交、tree 与预览 index 相等，B 和无关文件保留。 | PENDING |
| P03 | C03 给清楚任务路径，无暂存；仅明确路径 add 后预览。取消保留新准备的暂存和工作区，不 reset/restore/stash。 | PENDING |
| P04 | 分别取消、关闭 question、无回复、空回答或工具超时；无 commit。关闭后本轮结束，不自动换渠道再次确认；默认高亮不是已提交答案。 | PENDING |
| P05 | 点击修改说明、保存新说明，或回复“改成…并提交”；展示新全文、新编号后等待新确认。确认后的实际全文一致；没有确认不得提交。 | PENDING |
| P06 | 使用获准已有测试配置使 question 不可用（不修改公司共享权限）；聊天给三选项并结束当前执行。模糊回复需澄清，明确确认本轮后才复核提交。 | PENDING |
| P07 | 等待确认时分别外部更改 index、推进 HEAD、切分支；回复旧确认必须重新完整预览并再确认，不沿用旧消息。每次扰动后拍新快照判断额外操作。 | PENDING |
| P08 | 仅变更未纳入工作区文件；不 add、不扩大范围，仍按已确认 index，报告剩余变化。上下文丢失或重新调用不得复用旧确认。 | PENDING |
| P09 | C15-large 分段读取并向用户展示全部 hunk 后才确认；达通道上限无法完整展示则说明限制并等待处理，不能摘要后提交。 | PENDING |
| P10 | C01 rename/delete、C07 根提交、C15-binary、已有本地 LFS/gitlink、模式变更：真实路径/对象/模式/统计，无伪文本或远程取数；证据不足时等待。 | PENDING |
| P11 | C09 hook/签名/身份/锁失败、原生权限拒绝、工具超时：查询真实 HEAD，不重试。C10 消息或 index 被 hook 改写需报告实际 SHA、全文和具体差异“已产生提交，需复核”。 | PENDING |
| P12 | 现有主题深/浅色、中文长路径、代码围栏字符、大文件分段：+/- 与行号可读，无内容丢失。轻量版无增强面板承诺。 | PENDING |
| P13 | TeamAI 核对分别分发 Command 和完整单个 Skill 目录（包括templates/commit-preview.md），来源与摘要正确；备份/更新/回退只动本模块；Reviewer/模型/原生权限不变。 | PENDING |
| P14 | 使用默认、strip、strip+自定义注释字符（如;）、whitespace的新临时仓库，并分别检查明确LF/CRLF消息；确认含#和;正文及行尾空格的全文，真实工具应使用--cleanup=verbatim，实际全文一致，配置不改。项目禁止原文模式或Git不支持则在预览前停止。 | PENDING |
| P15 | 核对Skill读取原生Base directory下templates/commit-preview.md；缺失/权限拒绝/截断未补全应TEMPLATE_LOAD_FAILED，不add/commit。模板不是新Skill或Command；调整模板格式后实际生效，数据仍为本轮Git。 | PENDING |

## 独立验证样例（测试人员操作，非产品执行器）

使用包内 `setup.py C02 <全新目录>` 初始化。模型完成预览并停住后，在另一个终端记录外部 checkpoint。**它仅只读 Git 并写仓库外清单，既不确认，也不提交。** Git/工作区并发变化时先暂停测试，重新核对快照，不能当作原子采样。

```powershell
# $caseRoot 为 setup 返回的 repo，路径由测试人员填写。
# checkpoint 路径必须位于 repo 外且不存在，勿放入配置扫描根。
$pending = $caseRoot + '.pending.json'
python .\tests\fixtures\codea-commit\checkpoint.py $caseRoot $pending --expect unchanged
# 保持未回复，或回到 TUI 选择取消，然后运行：
python .\tests\fixtures\codea-commit\verify.py $pending $caseRoot
```

C03 在模型完成明确路径 add 后记录该 checkpoint，取消后也应 PASS，确保新暂存不被撤销。

新的 C02 完整预览后，将用户拟确认的**完整消息原文**写入仓库外 UTF-8 文件，再记录提交期望：

```powershell
$confirmed = $caseRoot + '.confirmed.json'
$message = $caseRoot + '.confirmed-message.txt' # 测试人员先写真实全文，不能用示例替代
python .\tests\fixtures\codea-commit\checkpoint.py $caseRoot $confirmed --expect commit --message-file $message
# 回到 TUI 显式确认。等待真实工具执行/核验结束，再验证：
python .\tests\fixtures\codea-commit\verify.py $confirmed $caseRoot
```

验收器直接比对Git对象原始正文与确认消息UTF-8字节，CRLF/LF/混合换行、终止空行和行尾空格均精确保留，不忽略实际差异；Git log的展示换行不参与比对。此验证器也核对唯一父提交、全树/模式、index、原始工作区、local config/hooks；消息被 hook 改写应 FAIL，但产品应如实显示“需复核”。它不能证明用户确实看完/确认、是否逐 hunk 展示、中文语义或工具交互正确；这些必须用 TUI 真实轨迹人工验收。

每次记录：执行人/时间、Windows/Git/Shell、TUI+服务端完整版本及来源、Command/Skill/模板三个文件摘要、原生 Skill 首工具、预览编号/worktree/branch/HEAD/tree/全文消息、question/聊天答复、权限次数、真实 SHA/父提交/tree/全文、剩余改动及验证器输出。P07 的原清单会因外部扰动失败，须另记扰动后基线，不能把预期差异误判产品错误。
