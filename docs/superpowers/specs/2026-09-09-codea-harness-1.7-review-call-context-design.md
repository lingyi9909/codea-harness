# Codea Harness 1.7 — 调用链自动更新与业务上下文增强 Review

> 2026-09-11：本文已由 [1.7完整项目设计](2026-09-11-codea-harness-1.7-integrated-design.md) 及其执行计划取代。保留作历史参考；正式研发从新文档开始，旧“仅六项/不含TeamAI”范围不再适用。

日期：2026-09-09
状态：根据本次确认范围编写的实施设计；本文提交只交付设计和计划，不表示功能已实现或已验收。
代码基线：`2467e13c493c3a997d2c719955437bff8b9c3cd4`（1.6.4）。
执行计划：[1.7 执行计划](../plans/2026-09-09-codea-harness-1.7-review-call-context-plan.md)。

本文取代 [2026-09-03 的旧 1.7 草案](2026-09-03-codea-harness-1.7-project-harness-knowledge-context-design.md) 的版本范围。旧稿中的 Project Harness、TeamAI、Business Context Snapshot 不进入本版。

## 1. 目标及范围约束

研发正常执行 `harness review`，系统自动分析本次相关调用关系，补充必要业务代码，给出有依据的问题及分析缺口。无需先维护 Chain YAML、执行 refresh 或逐条确认调用关系。

以下约束原文同步到执行计划：

- 只建设 Code Review 的六项功能，不扩展 Test、Debug、Fix、API Doc 产品能力。
- 不新增本地持久索引、跨运行解析缓存、图数据库、图查询、建图命令或后台监听。
- 不新增快照、SnapshotReader、业务上下文快照、双图 overlay 或新的快照认证体系；现有 1.6.4 ChangeSet / certification / freshness 机制保持原状。
- 不整套集成 OpenCodeReview、Code Review Graph、JDT LS、JDT Core、JavaParser、Embedding、向量数据库或知识库。
- 调用关系在本次 Review 自动分析和更新；人工只可选补充业务说明及缺失材料。
- 正常 Review 不写 .code-harness/chains/**；自动更新的是本次运行的链路结果。
- 公司内网不能访问外网；默认不增加第三方依赖，不自动安装包、下载模型资源或 git fetch/pull。
- 只读上下文不得自动扩大 Finding Scope、Write Scope 或用户已经选定的 FULL / TARGETED 范围。
- 不另建评测平台、发布系统或升级回滚专项；必要测试随功能实施，复用现有交付流程。

实现技术沿用 Go 1.23.10 基线、随包 ast-grep、Git、Go 标准库及现有已批准模块。本文不要求升级上述工具；实际公司批准的具体制品仍以现有安装包为准。

### 六项功能

| 编号 | 功能 | 用户可见结果 |
|---|---|---|
| F1 | Java / Spring 调用解析增强 | 能区分实现、继承、重载及注入条件，不随便选择目标 |
| F2 | 调用链自动更新 | 修改代码后直接 Review，自动使用当前可验证关系 |
| F3 | MyBatis 关联 Review | 联合检查 Mapper 方法、XML 语句、参数及公共 SQL |
| F4 | Dubbo 边界识别 | 识别远程契约；有获准源码则继续，无源码则说明边界 |
| F5 | 按需补充 Review 上下文 | 沿当前变更和规则寻找必要上下游、资源及旧代码证据 |
| F6 | 问题准确性与报告优化 | 仅列需处理问题，说明依据；未完成不能表述为通过 |

## 2. 当前实现及需要调整的位置

| 位置 | 当前行为 | 1.7 改动 |
|---|---|---|
| `internal/nav/project_calls_163.go` | 将 this/super/无 receiver 归到当前 owner，字段类型简化 | 新增精确方法引用及有限语义解析，兼容旧导航入口 |
| `internal/chain/project_discover_163.go` | 已有实现发现、Mapper XML 关联 | 复用共享解析，不再维护第二套实现选择 |
| `internal/reviewscope/chain_context.go` | STALE 默认进入 STALE_REQUIRES_DECISION | 仅 Review 自动重发现临时链，持久保存继续走既有明确授权 |
| `internal/reviewunit/build.go` | 构造确定性 ReviewUnit、文件和链路范围 | 将只读补充上下文和正式文件范围分开 |
| `internal/finding/evidence.go` | 检查证据存在及变更关联；dependency path 被拒绝 | 增加受控上下文引用，校验实际关系，保留锚点范围限制 |
| `internal/report/review.go` | PARTIAL 对应 MANUAL_ACTION_REQUIRED；正式报告加载 certified findings | 保留机器结果语义，简化用户显示并输出研发转述 |

上述路径均相对 `.code-harness/tools-runtime/`。本文指定的是需改行为，非宣称现有全部场景存在缺陷。

## 3. 设计选择

选择“现有导航增强 + 每次 Review 按需分析”。本地索引/图方案和外部完整 Review 引擎均已移出本版。

本版不承诺静态分析还原所有运行时执行路径。Java 静态目标、Spring Bean 选择、Dubbo 契约、MyBatis 语句关系分别表达，不能把“找到接口”当成“找到实际执行实例”。

自动维护按以下生命周期执行：

```text
review begin
  → 既有 ChangeSet 与分析入口
  → 按变更自动解析代码关系（DISCOVERY）
  → 既有 analysis certify
  → 既有 review options / select / units / dispatch
  → 按已分发规则补充只读上下文（RULES）
  → Finding Proposal
  → 既有 certify-findings 加强证据检查
  → review.md：问题 / 未发现问题 / 未完成
```

两次上下文步骤使用同一个分析组件；“阶段”仅决定 Runtime 如何取种子，不是两个独立系统。它们不会改变现有 ChangeSet 的生成或保存方式。

## 4. 模块及内部调用

新增 `internal/reviewcontext` 作为普通按需分析组件，负责串联 Java/Spring、MyBatis、Dubbo 及预算。它只依赖低层 `nav`、`workspace` 和标准库，不反向导入 `analysis`、`reviewunit`、`finding`，避免包循环。

新增一个内部 Runtime 子命令，非用户产品命令：

```text
codea-dcep-tools.exe review context --input .code-harness/runs/<runId>/requests/<name>.json
```

request 只允许：

```json
{"runId":"review-example","phase":"DISCOVERY"}
```

`phase` 仅可为 DISCOVERY 或 RULES，禁止 Agent 传入任意根目录、文件白名单、预算、关系事实和 Git identity。

- DISCOVERY：Runtime 从同 run 既有 ChangeSet 派生变更文件、符号及本地 base 内容；未跟踪文件继续使用既有 ChangeSet 语义。Agent 使用结果撰写 semantic proposal。
- RULES：Runtime 从同 run 已认证 ChangeAnalysis、ReviewUnit 和 RuleDispatch 派生需求。禁止借此扩大 selection。
- 所有路径只能来自 current source roots 和现有 `workspaceDependencies` 中机器验证通过的源码依赖。不存在“扫描任意相邻仓库”的回退。
- 新阶段仅插入 Review 编排；其他产品入口不自动启用。

本次结果保存为 `analysis/review-call-context.json` 和 `analysis/review-rule-context.json`。它们是运行记录，不是可复用索引或快照，不是独立 Authority。下个 run 不加载这些文件作代码事实。正式消费者不能信任文件名或 Agent 填写的 relationId，必须用现有认证入口重新验证相关源码关系。

## 5. 关系数据及确定性

使用普通结构化记录，不提供图查询语言或通用遍历 API。

方法身份包含 workspace、仓库相对 path、ownerFqcn、method、parameterTypes；方法显示名继续可以是 `OrderService.approve`，显示名不得充当唯一身份。代码位置包含 BASE/CURRENT 和起止行。BASE 只用于变更前证据，不作为当前调用关系。

每条关系包含：

| 字段 | 约束 |
|---|---|
| id | Runtime 根据关系类型、精确来源/目标及 callsite 稳定生成 |
| kind | JAVA_CALL / SPRING_BINDING / MYBATIS_STATEMENT / SQL_INCLUDE / DUBBO_CONTRACT |
| from、targets | 精确引用；目标可以为空或多个，禁止按列表第一个补目标 |
| resolution | EXACT / CONDITIONAL / AMBIGUOUS / UNRESOLVED |
| evidence | 实际源码位置；跨 workspace 必须保留 workspace identity |
| reason | 稳定机器原因；非 EXACT 必填 |
| assumptions | 配置、框架注册范围等前提；不能因已有一个候选就假设扫描完整 |

EXACT 只表示该类关系在已验证源码范围和明确条件下成立，不表示运行时路径实际执行过。只有 EXACT 关系进入已确认连续链路；其余保留为限制或候选，不强制用户人工选择实现。

同一个 caller 里的 `risk.check()` 与 `repository.save()` 分别记录为两条调用；不得生成 risk.check → repository.save。递归用访问集合停止，并记录 RECURSION_BOUNDARY，不宣称发现无限完整链。

### 与现有身份的兼容

新导航模型内部使用完整签名；既有 `symbolid.Ref` 的 workspace/path/symbol 不全仓迁移。可无歧义投影的关系继续填入现有 ChangeAnalysis 和 ChainRefs；同一路径的重载需靠新 relation 引用区分，不能用旧裸符号伪装为唯一目标。

本版不迁移 Chain YAML v1。无法安全投影的精确关系留在本次上下文，不能强行进入旧 Chain；对应依赖该关系的规则通过新上下文引用取证。

## 6. Java / Spring 首批支持边界

### Java

- 包名、显式 import、当前包及嵌套类型身份。
- 字段、方法参数、局部变量作用域；局部变量/参数优先于同名字段。
- this 与无 receiver 的普通实例调用；super 定位直接父类型并沿继承查找实际声明。
- 方法重载先以参数类型签名保留候选；仅在支持的实参类型规则唯一匹配时给 EXACT。
- 不实现完整 Java 编译器类型推断；泛型推断、方法引用、反射、复杂 lambda、链式表达式解析不足时给 UNRESOLVED。
- AST 是语法结构的依据；文本搜索仅定位候选，不以 regex 命中认证调用。

### Spring

- 字段、显式构造器、setter 的注入点；单构造器无 @Autowired 的常见形式。
- @Autowired、javax/jakarta @Resource(name)、@Qualifier、@Primary。
- @Service / @Component / @Repository 及返回类型明确的 @Bean；Bean 名称只在已确认注册候选集合中解释。
- 候选选择先按可赋值类型与可判断的泛型条件过滤，再按注入点限定、Resource 语义和 Primary 等规则解析，不能制定脱离框架版本的简单“注解优先级”。
- 显式源码扫描范围和公司配置可确认的候选才允许唯一判定；XML 容器配置、复杂 @Import、FactoryBean、未解析 Profile/Conditional 或 Bean 覆盖使候选集合不完整时给 CONDITIONAL/UNRESOLVED。
- Lombok 自动构造器先识别该模式并报告 GENERATED_CONSTRUCTOR_UNSUPPORTED；若字段本身有可验证注入注解，可独立解析该字段，不宣称支持 Lombok 生成代码。
- 不启动 Spring 容器，不执行仓库构建脚本或用户代码以“验证” Bean。

## 7. MyBatis

复用现有 Mapper/XML 查找，新增标准库 `encoding/xml` 解析。XML 解析不得解析外部实体或联网拉取 DTD。

优先支持 namespace + statement id、@Param、显式 resultType/resultMap、公共 sql/include，包含跨 namespace 的静态 refid。include 循环给 SQL_INCLUDE_CYCLE，未解析 refid 给 SQL_INCLUDE_UNRESOLVED。

只对可解析静态片段使用已有 SQL 解析能力；动态表名、动态分支、Provider SQL、缺失 databaseId 配置不得认证成单一确定 SQL。不访问数据库。

把证据提供给现有规则：

- WHERE/隔离条件变化：读取变更前后相关语句。
- 参数与返回契约：对照 Mapper 与语句映射。
- 动态拼接：必须有可控输入证据才能判断风险。
- 公共片段：按需搜索引用位置，范围截断时记录限制。

## 8. Dubbo

识别 @DubboReference / @DubboService，以及能从明确 import 判断的历史 Dubbo 注解；不把其他框架同名 @Reference / @Service 误认为 Dubbo。

服务身份包含 interface FQCN、方法签名、group、version 的原始值与已解析值。字面量及现有本地配置可解析属性可以使用；缺失配置、通配符或默认行为无法按当前框架确定时不得用空字符串强行匹配。

首批 Provider 来源仅限同仓及现有 VERIFIED workspaceDependencies。若 Provider 仓库不能通过既有 dependency identity 校验，则报告 PROVIDER_SOURCE_UNAVAILABLE；本版不新增跨仓目录、服务注册中心读取或新配置体系。

源码契约匹配只认证 DUBBO_CONTRACT，不认证实际运行时路由。多个 Provider、泛化调用、mock/stub、动态路由等保留边界。Repository 后续调用仍属于实际调用它的方法，不接到远程方法尾部伪造串行链。

## 9. 自动 Chain 生命周期

正常 Review 使用 AUTO_TEMPORARY 策略，策略来自 Runtime 产品入口，不接受 Agent 通过 JSON 自行覆盖。

| 情况 | 行为 |
|---|---|
| 无保存链 | 用本次分析自动生成临时链 |
| 保存链与当前关系一致 | 可以显示其名称和来源，但关系重新验证 |
| 保存链过期、入口仍有效 | 自动生成当前临时链，记录已变化，不要求 refresh 确认 |
| 保存链入口已删除 | 记录删除，排除当前链；旧内容仅供删除变更分析 |
| 保存 YAML 损坏 | 不信任其代码事实；能从当前代码恢复则继续临时链，显示一次提示 |
| 当前关系解析不完整 | 保留可确认前缀及限制；不得通过人工选一个目标消除机器歧义 |

不调用 seal-persist/persist，不覆盖人工备注。显式 `harness chain discover/refresh/edit` 保存项目状态仍沿用既有授权流程；这与 Review 自动临时更新是不同操作。

自动更新不取消 1.6.3 的多业务链 USER_SELECTION，也不伪造用户选择。因为旧 Chain 过期而弹出的维护确认取消；真正的评审范围选择保留。

## 10. 按需上下文、性能与覆盖

DISCOVERY 种子由本次实际变更派生；RULES 种子由已选 ReviewUnit 及已分发规则派生。从 caller/callee、实现、SQL 关联补充必要上下文，向上查找也依赖有界文本候选和 AST 确认，不承诺无索引情况下能低成本获得全仓完整反向影响。

默认新增上下文预算（工程初值，不是性能承诺）：

- 下游方法展开 6 跳，上游 3 跳；
- 每 run 最多额外解析 80 个上下文文件；
- 每 run 最多 400 个候选位置进入语义确认；
- DISCOVERY 与 RULES 各最多 40 个额外文件、200 个候选位置、1 MiB 补充源码、15 秒探索耗时，合计上限对应上述 80/400 和 2 MiB/30 秒；
- 每个候选搜索进程最多 5 秒；上述是上下文探索预算，既有认证的源码复验开销单独计算，不声称整个 Review 只需 30 秒。

预算不包括既有 required changed files 的必读责任，不能用“上下文到限”跳过变更文件。预算由 Runtime 固定，1.7 不新增 harness 配置项。同一进程内可复用文件读取及 AST 结果；两个独立命令进程不得把落盘解析结果变成跨进程/跨 run 缓存。若分阶段重复解析开销明显，后续合并同进程调用，不引入持久索引。

搜索只能使用参数数组调用已有 Git/ast-grep 执行封装，pathspec 与 pattern 分开。不得 shell 拼接、下载依赖或启动任意 MCP。不能让仅覆盖 tracked 文件的搜索静默漏掉本次 untracked 文件，后者取自既有 ChangeSet。

区分两类不完整：

1. 既有 ChangeSet / EntryPoint / required-file coverage 或认证失败：保持原有硬停止，不进入 Finding Review。
2. 已认证变更范围内的某项规则缺少额外业务上下文：仅该 rule dispatch 标 BLOCKED；其他 READY 规则可以执行，最终报告为“评审未完成”，不报整体通过。

CONDITIONAL / AMBIGUOUS / UNRESOLVED 是关系状态；READY / BLOCKED 是具体规则的就绪状态，不能混用。未分发的规则不计为 BLOCKED。缺少远程源码并不必然阻断所有本地规则。

## 11. Finding 及报告

保留现有 Proposal → Verify → Certify → Runtime renderer 流程；不新增独立认证平台。

扩展 evidence kind 为 CONTEXT_RELATION，并添加 relationId、workspace、sourceSide（BASE/CURRENT）等必要字段；Agent 只引用 Runtime 返回的 ID。认证时重新验证当前规则所属 ReviewUnit、关系来源及实际源码，不能只检查 ID 在 JSON 中出现。

外部上下文证据只允许已验证 workspace；正式 Anchor 仍在本次允许的 current repository 范围内。原有依赖路径拒绝行为对 Anchor/普通路径证据继续有效，不能直接删除这一限制。

旧 CHAIN/SYMBOL evidence 继续兼容，但依赖事务/跨文件关系的正式规则需要相应关系依据。introducedByChange 表示已核验的变更关联，不标记为“缺陷已证明”。用户报告只展示源码依据与模型判断，不将 confidence 显示为已校准正确率。

READY 只代表上下文具备，不代表模型已经完成检查。宿主工具/模型超时和中止继续通过既有 RuntimeErrors/PARTIAL 结果上报；未完成不能因为 proposals 为空推导为通过。新增上下文限制由 Runtime 汇总进 CertifiedSet 的可选 reviewContext 字段（status、blockedChecks），它受现有 CertifiedSet/certificate 的完整性校验保护，不增加新证书或快照。1.7 新 run 必须写该字段，旧 run 读取保留 legacy 行为。

高风险候选在同一宿主 Reviewer 中做一次有界反证检查，检查实际输入来源、是否已有保护、变更前是否已存在、是否误判框架语义；不新增独立模型客户端、多 Agent 平台或每次必调第二模型。仍无足够证据的候选不进入正式 finding，必需检查未完成则保留 BLOCKED。

机器结果沿用当前 report Result 枚举；存在 BLOCKED 时 Runtime 推导 MANUAL_ACTION_REQUIRED，不允许 Agent 传 PASS 掩盖。若保留已认证的可用 finding，也必须加载 CertifiedSet 后渲染，禁止沿 PARTIAL 原始请求分支直接显示 Agent findings。

用户默认报告：

- 有问题：按严重性列问题、位置、影响、依据、修改建议。
- 没发现问题且 required coverage 和规则检查均完成：一句“本次评审未发现需要处理的问题”。
- 未完成：列缺失源码、歧义或预算等具体原因，并说明已完成范围；不是代码缺陷。
- 末尾提供一段可复制给研发的内容，只包含待处理问题和必要缺口。
- 不逐项罗列通过项；技术产物/hash/完整导航过程留在 run 记录。

## 12. 兼容性及落地文件

新增接口和文件清单见执行计划。变更既有 strict schema 时，同步 Go model、Runtime decode、认证、renderer 和 Agent 使用说明；不得仅改 prompt。

关键保持项：

- 既有 ChangeSet/certification/freshness 与 1.6.4 exact-file 批量扫描不改变。
- Chain YAML v1 不迁移；项目人工文件不因 Review 被写入。
- FULL/TARGETED、USER_SELECTION、0-change 正式报告路径保持。
- 原有 Test/Debug/Fix/API Doc 流程不启用新 Review 策略。
- 无新依赖审批项是默认交付目标；做不到必须说明具体缺口并缩到已定义的 unsupported 状态，不能偷偷引入解析框架。
- 旧 run 不被重新认证；新 run 使用 1.7 上下文和已同步的 strict schema。未升级客户端通过既有协议继续工作，不向它们强塞新字段。

## 13. 必要验收（属于功能测试）

调用关系样例：super、重载、跨包同名、字段遮蔽、Qualifier、Primary、构造器、条件缺失、第二实现新增、方法删除、并列调用、递归。

MyBatis/Dubbo 样例：参数不匹配、合法动态 SQL、include 引用及循环、Provider 源码缺失、group/version 不匹配、同名其他框架注解。

Review 样例：旧链过期无需确认继续、项目 YAML 原字节不变、依赖只能作为上下文、selected scope 不扩张、预算到限、篡改关系 ID、BLOCKED 不报通过、0-change。

真实模型验收：至少 12 个有明确预期的样例（6 个缺陷、6 个正确反例），不注入 proposals 或期望答案，复用现有宿主与公司内网模型；固定模型/规则/预算，保存实际输出并人工判定。样本小，不据此宣称统计性 precision/recall。原 24-case 继续作为合约回归。

Windows 验收：使用既有安装方式，在公司 Windows 和禁公网环境运行正常 Review，确认无运行时下载。缺少实际 Windows/模型条件时记录“未执行”，不能用 mock 或 Linux 检查代替声称通过。

开发时只运行与改动相关的回归；最后执行既有 Go test/vet 及受影响的宿主协议检查。不新增全套发布认证专项，不要求为本版建设新的升级/回滚体系。
