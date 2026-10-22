# Codea Commit 1.0 验收记录

## 2026-10-22：目标版本 v2.0.27 核对补充

用户现已提供 TUI **v2.0.27**。未修公开同版本核验 **6 PASS / 5 FAIL**，已保留为对照；本轮完成实际 V2 原生补丁，新增安全回归 **17/17 PASS**、修复源码真实服务探针 **11/11 PASS**，未修改CLI中文拒绝和退出码验证通过。公司构建/Windows/TUI/TeamAI/私有模型正常提交仍 PENDING。详见 [2.0.27 专项核对与安装回退说明](../../codea-commit/compatibility/v2.0.27/README.md)。隔离的 `payload/` 已提供正式扩展名测试资源，仅对安全核验通过的现有宿主安装；不自动恢复业务入口。下文 1.2.27 补丁仅为历史参考，不适用于 2.x。

## 以下为先前 1.2.27 研究与原功能验收记录

状态：**原生修复补丁已开发验证 / Draft，公司适配与验收 PENDING；适配前暂停安装、分发、合并发布。** 2026-10-21 审核更正：原设计和实现遗漏模型前参数 Shell 执行边界。已在公开上游真实宿主复现；仅靠 Command/Skill 提示词无法根治。可发现资源已同时撤下，保留 `.txt` 评审草稿；这仅为停用措施。

必须先完成 [原生参数前置校验与 S01—S08 验收](codea-commit-command-security.md)，再恢复安装和 C01—C16。参考 `1.2.27` 的真实源码前置拦截已实现并测试，见 [原生补丁](../../codea-commit/host-patches/README.md)。公司 exact 版本无法访问，不宣称公司版本已修复或内网通过。原离线验收包已撤回；旧版不得继续安装。

## 基线与兼容性调查（Task 1）

- 开发起点：`58426663c69adc9bbd426d6a540767274ff9cf9a`（main）；已重读该基线的设计、任务书与现有 Review 资源。设计参考的 `8ba491e` 并非本次开发起点。
- 本地：macOS / zsh；Git `2.39.5 (Apple Git-154)`；Python `3.9.6`。公司 OpenCode/TeamAI 与配置仍不可访问；审核后在隔离测试目录新增上游 `opencode-ai@1.2.27`，仅复现安全边界，不是产品依赖。
- 仓库事实：现有 Review README 记录 OpenCode V2 与 TeamAI push/pull 整个 Skill 目录；历史 Runtime CI 固定的 `opencode-ai@1.18.25` **不是公司 exact 版本证据**。
- 上游公开 [Command](https://opencode.ai/docs/commands/) 与 [Skill](https://opencode.ai/docs/skills/) 文档核对日期：2026-10-21；支持目录、agent 与原生 skill 概念。公司兼容性仍以以下真实记录为准，不新增适配 Runtime。
- 负责人：本地实现/fixture 为本 PR 执行代理；内网执行由用户后续安排，具体姓名 PENDING。未指定的人名不代填。

| 目标环境/能力 | 实际值/证据 | 状态 | 内网记录人 |
|---|---|---|---|
| Windows 版本/build | 待填写 | PENDING | 待用户指定 |
| OpenCode exact 版本与来源 | 待填写，附版本输出 | PENDING | 待用户指定 |
| Git for Windows exact 版本 | 待填写 | PENDING | 待用户指定 |
| 工具实际 Shell 与中文编码 | 待填写，区别终端与工具 Shell | PENDING | 待用户指定 |
| 公司私有模型完整标识 | 待填写，不附凭据 | PENDING | 待用户指定 |
| TeamAI exact 版本/同步命令与资源范围 | 待填写 | PENDING | 待用户指定 |
| 现有 Build ID 与权限 | 源 Command 为 build；实际 ID 待确认 | PENDING | 待用户指定 |
| 原生 skill 可用/加载来源 | 附工具返回的实际路径和 SHA | PENDING | 待用户指定 |
| Command 切 Build 后会话任务上下文 | C03 与 C04 对照；缺上下文必须询问 | PENDING | 待用户指定 |
| 只读 Git 原生权限框/拒绝行为 | 记录原策略、提示次数与工具结果 | PENDING | 待用户指定 |
| 明确 literal 路径 add 的原生权限 | 同上，不新增 allow | PENDING | 待用户指定 |
| commit、hooks、签名权限 | 同上，区分签名失败与工具拒绝 | PENDING | 待用户指定 |
| 系统临时 UTF-8 无 BOM 消息文件 | 路径、编码、写入权限、清理记录 | PENDING | 待用户指定 |
| TeamAI Skill 目录同步、Command 分发 | 分别记录实际路径/SHA；离线复制备选待真机验证 | PENDING | 待用户指定 |

若 actual exact 版缺少 agent 路由或原生 Skill，应暂停内网试用，附具体差异与最小资源适配建议；不能把缺能力当作加 Runtime/权限框架的理由。

## 已撤下草稿 SHA256（不安装）

以下值仅标识已撤下草稿原始字节，不表示允许安装。宿主修正验收通过、恢复产品文件后必须重新记录来源与 hash。

| 资源 | SHA256 |
|---|---|
| `codea-commit/blocked/codea-commit.md.txt` | `2d7c11a51741c54d8215c6917b35d9682e487cf2ba7ebf71d9eb3768a908f34b` |
| `codea-commit/blocked/SKILL.md.txt` | `079885453fc017075736566ece6e8543ecc9435bfea3013f54c5cf5968d2066b` |

## 本地证据（不是产品真机 PASS）

| 检查 | 执行结果 | 证据与限制 |
|---|---|---|
| 基线 Review 回归 | PASS，40/40 | `python3 -m unittest discover -s tests -v`，开发前执行 |
| 既有 Review Git fixture | PASS，4/4 | `python3 tests/fixtures/codea-review-t1/test_git_scope.py` |
| Commit fixture 初始缺失 | 8 个 missing setup.py 错误 | 验收工具未实现的基线，不是模型 RED |
| C02 正确/错误普通 Git 对照 | PASS | 未提交时缺 HEAD 报错；只提交 index 通过；错误 add 后明确指出 shared.txt 和 other.txt；真实 blob 对照 A=1/B=0，工作区 B=1 |
| Commit fixture 最终自检 | PASS，macOS 11/11（约 17 秒；Windows 执行时两项 POSIX 专属检查会跳过） | `python3 tests/fixtures/codea-commit/test_fixtures.py`；每次创建新临时仓库 |
| 历史文件复制/升级/回退/卸载自检（已被安全阻塞取代） | 历史 PASS，4 个步骤；不再用于安装 | 独立临时配置目录，仅文件级结果，不证明 OpenCode 实际加载 |
| 最终全套 Python 回归及 Review 差异 | PASS，40/40；Review fixture 4/4 | 精确记录命令与结果；原生 Review 入口真机仍 PENDING |
| Command/Skill 正常提交真实模型运行 | PENDING | 无公司宿主；新增安全探针使用实际上游宿主但故意让模型不可用，不是功能验收 |
| 原生前处理安全复现（原入口） | FAIL，已复现漏洞 | 上游 1.2.27；原入口写 sentinel 后才报模型不存在；非空参数拒绝缺失 |
| 原生暂停分发检查（两资源已撤下） | PASS，3/3（含四个不安全负对照） | `test_command_preprocessing.py`，仅证明入口撤下，功能依然 BLOCKED |
| 公司原生参数校验 S01—S08 | PENDING / 发布阻塞 | 完成前不恢复可发现文件，不进行业务试用 |
| Windows fixture 自检 | PENDING | 本地 macOS 结果不能替代 Windows |

### C02 合成普通 Git 实测收据

2026-10-21，本地独立临时仓库；以下并非 OpenCode 产品运行，也非真实团队代码。两个独立对照有相同基线父提交 `b750e2aa2df1547d22cb251af595e8731fc98266`。

| 对照 | 实际 commit SHA | oracle 结果 | 实际残留 |
|---|---|---|---|
| 只提交 index | `c5eca9896ca11f4143bc965356cec20654dfe9eb` | PASS；提交树 A=1/B=0 | ` M shared.txt`（B=1），`?? other.txt` |
| 故意 add shared.txt 与 other.txt | `45c22a505d32d0234268faf375abfcf0c9f48a47` | 预期 FAIL：commit tree/index 的 shared.txt 与 other.txt 均不符合 | 干净，反而证明仅检查剩余文件数会漏错 |

两次消息均为人工对照 `fix(fixture): 仅提交 A 变更`；错误对照的消息与实际内容不符，不能因消息看似正确就宣称通过。验证器还新增保护快照测试：更改 hooksPath 时旧验证器错误地 PASS，新测试先失败，再加入 local config/hooks 对比后通过。独立评审发现可执行位和目录符号链接漏报；两个新负对照先复现 FAIL，再补齐工作区模式/链接快照，不跟随链接访问目标。

上一轮独立评审检查了五个 Review Focus，当时未发现产品 Critical/Important；该结论未覆盖模型前参数处理，已被本轮安全阻塞推翻。上述 oracle 漏报按验收影响升级处理，两项回归均先失败再通过。评审未执行 OpenCode/Windows/TeamAI/私有模型，也没有把 Codex 应用规则当作产品通过。

文件级生命周期自检使用临时目录复制真实两份资源并比对 SHA，模拟备份/替换/恢复/仅卸载本模块，额外 Review/Agent/Provider 哨兵 bytes 保持。未执行 PowerShell 或 OpenCode，不能回填下方真机流程为 PASS。

## 2026-10-22 原生修复执行记录

参考基线 `anomalyco/opencode` commit `4ee426ba549131c4903a71dfb6259200467aca81`（tag `v1.2.27`）；Bun `1.3.10`，macOS arm64。仅在独立源码目录开发，未写入公司或用户已安装宿主，不携带新 Runtime/构建依赖。

| 检查 | 实际结果 | 证据/边界 |
|---|---|---|
| 原生源码修复前安全回归 | 3 PASS / 6 FAIL | 四类模板入口实际写出 sentinel，HTTP/早于 registry 断言失败；证明缺少前置拦截 |
| 既有 UI 错误事件补充回归 | 先失败再通过 | 拦截后还须通过既有事件给出中文提示，保留 busy/retry 状态 |
| 原生源码针对性回归 | 13/13 PASS | 真实 SessionPrompt、Server 和 CLI 子进程，不模拟解析器 |
| CLI 事件通道不可用 | 先失败再通过 | 真实 CLI + 原生 HTTP；只屏蔽事件通道，确保仍输出中文并非零退出 |
| 原生 typecheck | PASS | `bun run typecheck` |
| 原生完整包回归 | 1326 PASS / 8 SKIP / 0 FAIL | 首次既有 tool.registry singular 用例 30s 超时；单独 90s 复测 3/3 PASS，再全套重跑 |
| 未修发行二进制对照 | FAIL（预期） | `test_native_argument_guard.py` 在原始 1.2.27 上无法原生拒绝；不是停用测试 |
| 已安装草稿的修后源码服务/CLI | 5/5 PASS | 非空中文 400、不写 sentinel、不进模型；空白进入 Build/模型选择；CLI 可见提示并非零退出；其他命令保持 |
| CLI busy/retry 边界 | 2/2 PASS（已包含在 13 项专项中） | 真实服务+子进程；目标目录明确，非零及时退出、原任务状态不变。评审曾推断监听挂起，真实回归未复现；全局入口 finally 的 process.exit 已负责退出，未添加多余生产改动 |
| 补丁 apply/reverse 与内容对照 | PASS | 在固定基线导出的干净临时树应用，逐文件 bytes 与测试源码一致 |
| 公司 OpenCode / Windows / 私有模型 / TeamAI | PENDING | 参考源码测试不可替代公司适配与验收 |

当前产品仍只定义 Command+Skill；新增的是对**现有宿主**的源码维护补丁与测试资料，不是另一个运行时。只有宿主团队应用/适配并完成 S01—S08 后才恢复两份安装资源，随后执行原有正常提交和说明质量验收。

## 功能矩阵（每项产品验收均待内网填写）

**先解除 S01—S08 参数安全阻塞。以下功能场景目前不执行安装/业务试用，均 PENDING。** 所有场景只用新建临时仓库。数据构造、正确/错误对照及手动扰动步骤见 [fixture README](../../tests/fixtures/codea-commit/README.md)。本地普通 Git 对照不回填此表为 PASS。

| ID | 观察要点/必需结果 | 状态 | 执行人 / 环境 / 工具轨迹 / 实际 SHA / 残留状态 |
|---|---|---|---|
| C01 | 单一 staged 一次新提交，准确中文，无例行二次确认 | PENDING | 待填写 |
| C02 | commit 仅含 A staged hunk；B 与 other.txt 保持，不描述 B | PENDING | 待填写 |
| C03 | 明确会话任务，仅 add 指定新增/修改/删除路径 | PENDING | 待填写 |
| C04 | 无可靠上下文先问范围，HEAD/index 保持 | PENDING | 待填写 |
| C05 | staged 无关需求给拆分建议，保持 index，单次最多一提交 | PENDING | 待填写 |
| C06 | 干净，不空提交、不报故障 | PENDING | 待填写 |
| C07 | 新仓库根提交，无虚构 HEAD，提交树正确 | PENDING | 待填写 |
| C08 | 中文/空格/引号/美元/反引号/前导短横线/literal 路径与消息保真；异常路径明确停止 | PENDING | 按 Windows 合法文件名和消息、POSIX 扩展分别记录 |
| C09 | hook/身份/签名/index.lock 失败不绕过、不删锁、不改配置 | PENDING | 四个子项分别填写 |
| C10 | hook 改消息/改 index/HEAD 已变或返回丢失：真实 SHA/消息，异常标需复核，无回滚 | PENDING | 各子项分别填写；post-hook exit 1 不等于 commit 失败 |
| C11 | detached、merge/rebase/cherry-pick/revert/冲突停止，原状态保持 | PENDING | 各子项分别填写 |
| C12 | Diff 后他进程改 HEAD/分支/index，停止重新分析，不沿用旧说明 | PENDING | 保留扰动前后及恢复模型后的三组状态 |
| C13 | 非 Git、缺 Command/Skill、工具 deny；非空/注入参数必须在宿主展开前停止 | PENDING / 前置阻塞已复现 | 先完成 S01—S08，不以模型拒绝作为证据 |
| C14 | 正常隐藏配置可选；临时报告不自动加入，已暂存时先问 | PENDING | 待填写 |
| C15 | 截断补读、binary/LFS/gitlink 不编造、linked worktree 正确定位 | PENDING | 各子项分别填写，合成 binary 不代替 LFS/gitlink |
| C16 | 再调用不重复/空提交，剩余内容重新判断 | PENDING | C01 干净与 C02 有残留分别填写 |

## 稳定性重复与权限提示

每次新 fixture 目录，C01—C03 各三次；“业务确认”与组织原生工具权限框分开统计。若范围明确仍反复确认、边界扩大或失败误报，修复后重新验收，不抹掉失败记录。

| 场景 | 第 1 次 | 第 2 次 | 第 3 次 |
|---|---|---|---|
| C01 | PENDING | PENDING | PENDING |
| C02 | PENDING | PENDING | PENDING |
| C03 | PENDING | PENDING | PENDING |

每格记录：执行人、环境表版本、真实 commit SHA、原生权限框次数/内容、业务确认次数、剩余状态与工具轨迹位置；不得记录凭据或公司真实源码。

## 10 组真实脱敏说明质量评审

由团队选取真实改动并脱敏，不能用合成 fixture、关键词命中或示例消息代替。逐项核对 type/scope、行为变化、必要原因有无依据、是否仅描述 commit、有无虚构工单/功能/测试。评审输入/结论只保留脱敏摘要与内网证据位置。

| 编号 | 改动类别（团队选取） | type/scope | 行为/原因准确 | 仅 commit / 无编造 | 评审人 / 证据 / 状态 |
|---|---|---|---|---|---|
| Q01 | Java | PENDING | PENDING | PENDING | 待指定 |
| Q02 | Java | PENDING | PENDING | PENDING | 待指定 |
| Q03 | SQL | PENDING | PENDING | PENDING | 待指定 |
| Q04 | SQL | PENDING | PENDING | PENDING | 待指定 |
| Q05 | 配置 | PENDING | PENDING | PENDING | 待指定 |
| Q06 | 配置 | PENDING | PENDING | PENDING | 待指定 |
| Q07 | 文档 | PENDING | PENDING | PENDING | 待指定 |
| Q08 | 文档 | PENDING | PENDING | PENDING | 待指定 |
| Q09 | 测试 | PENDING | PENDING | PENDING | 待指定 |
| Q10 | 测试 | PENDING | PENDING | PENDING | 待指定 |

## 安装、分发及回归验收

| 流程 | 必须留存的证据 | 状态 |
|---|---|---|
| 首次安装→刷新/重启→调用 | 两份源/目标 hash、Build ID、首个 skill 工具及实际路径 | PENDING |
| 只同步 Skill / 旧 Command 新 Skill | 识别入口缺失/版本不配，不误报新版已加载 | PENDING |
| 项目/JSON/兼容目录同名覆盖 | 查明实际来源，停止误装，不猜优先级 | PENDING |
| 备份→升级→回退 | 两资源恢复原 hash；备份位于扫描目录外 | PENDING |
| 卸载与 TeamAI 再同步 | 仅本模块移除；原有资源/业务文件保持 | PENDING |
| /codea-review 真机回归 | 原入口、Reviewer 权限、模型、Review 流程不变 | PENDING |
| PR 差异检查 | 本模块停用/草稿、fixture/测试、安全验收与设计/任务书更正、根 README 链接；codea-review、.code-harness、.github 无差异 | PASS（停用后的静态差异检查） |

每次执行记录建议：场景/重复号；人员与时间；环境表版本；资源 SHA；调用输入；首个工具/加载来源；权限框与业务确认；原 HEAD/branch/index tree；实际 commit SHA、父提交、完整消息和 tree；剩余状态；oracle 输出；人工结论与 PENDING 原因。实际记录留内网，PR 仅写脱敏结论。

发布前必须处理四类禁止问题：无关代码混入、部分暂存扩大、编造功能/验证、失败后擅改历史。关键内网验收未完成保持 Draft；本 PR 不自动合并，也不自动向公司业务仓库推送。
