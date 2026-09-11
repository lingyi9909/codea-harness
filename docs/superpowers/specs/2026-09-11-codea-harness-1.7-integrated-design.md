# Codea Harness 1.7 完整项目设计：Code Review、内网与 TeamAI 协同

日期：2026-09-11。状态：研发交接版；设计与执行计划已整理，功能尚未实现或验收。

执行计划：[1.7 完整执行计划](../plans/2026-09-11-codea-harness-1.7-integrated-plan.md)。两份文档必须从同一提交读取。

已核对核心 main：`b864bd19b9313bd882da89612e701d8bb3221e7d`；TeamAI 接入资源：`b3381b5c089acaf75eedf5ed94b3ae2031b00273`。这些是位置/行为核对点，不是已验收的 1.6.4.final 开发基线。

本设计完整纳入已确认的八项功能及最新维护边界，取代 2026-09-03、2026-09-09 旧1.7方案以及先前整合稿的范围与任务安排。本文已包含必要代码关系细节，不要求研发再拼接旧文档。长期日志/研发全流程协同稿仅作未来参考。

版本结果：修改代码后直接 Review，自动使用当前可确认调用关系，结合已绑定的业务要求给出有证据的问题；已有 TeamAI 目录不变，Git 管理总仓，Harness 安装升级与 TeamAI 资源同步各走原流程。

## 1. 目标及范围约束

研发正常执行 `harness review`，系统自动分析本次相关调用关系，补充必要业务代码，给出有依据的问题及分析缺口。无需先维护 Chain YAML、执行 refresh 或逐条确认调用关系。

以下约束原文同步到执行计划：

- 只新增 Code Review 八项能力及其内网/TeamAI 协同；不扩展 Test、Debug、Fix、Verify 或 API Doc 产品能力。
- TeamAI 原有目录、配置和管理方式保持不变，仅增加 Harness 接入资源；不迁移现有知识。
- Git 管理总仓文件，TeamAI 命令同步其资源，Harness 沿用原正式安装/升级流程；三者不能相互替代。
- 不新增持久代码索引、跨 run 解析缓存、图数据库、图查询、后台监听或 LSP/JDT。
- 不新增快照或独立认证体系；保留既有 ChangeSet、certification、freshness、Reviewer Host 和八阶段 Runtime 权威。
- 不整套集成 OpenCodeReview/Code Review Graph，不引入向量库、Embedding 或新的知识管理平台。
- 每次 Review 自动派生本次调用链；正常 Review 不写 .code-harness/chains/**，不要求人工刷新。
- 内网完全无法访问外网；默认零新增第三方依赖，缺包逐项申请，不自动下载或通过 vendor 绕过审批。
- Review 内不执行 Git/TeamAI 同步或自动升级；上下文不扩大 FULL/TARGETED、代码读写范围或用户授权。
- 日志平台、登录验证码、连接器/MCP、故障诊断、修复增强及经验自动生成不进入本版。
- 不另建发布保障、升级回滚或评测平台；必要的版本适配和验证复用现有机制。
- 正式开发从验收通过的 1.6.4.final 建立独立分支，本次交付只包含文档，不宣称功能已实现。

实现技术沿用 Go 1.23.10 基线、随包 ast-grep、Git、Go 标准库及现有已批准模块。本文不要求升级上述工具；实际公司批准的具体制品仍以现有安装包为准。

### 八项功能

| 编号 | 功能 | 用户可见结果 |
|---|---|---|
| F1 | Java / Spring 调用解析增强 | 能区分实现、继承、重载及注入条件，不随便选择目标 |
| F2 | 调用链自动更新 | 修改代码后直接 Review，自动使用当前可验证关系 |
| F3 | MyBatis 关联 Review | 联合检查 Mapper 方法、XML 语句、参数及公共 SQL |
| F4 | Dubbo 边界识别 | 识别远程契约；有获准源码则继续，无源码则说明边界 |
| F5 | 按需补充 Review 上下文 | 沿当前变更和规则寻找必要上下游、资源及旧代码证据 |
| F6 | 问题准确性与报告优化 | 仅列需处理问题，说明依据；未完成不能表述为通过 |
| F7 | 业务知识接入 Review | 明确绑定正式规则和参考资料，判断适用性、缺失、冲突和变化 |
| F8 | 内网和 TeamAI 协同 | 原目录不变、轻量入口、完整正式包、原安装升级和同步方式 |

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

业务知识读取在 REVIEW_PLANNING 中、Unit 之后与最终 dispatch 之前执行，详见第13/16节。两次代码上下文步骤使用同一个分析组件；“阶段”仅决定 Runtime 如何取种子，不是两个独立系统。它们不会改变现有 ChangeSet 的生成或保存方式。

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

callsite 还保留起止列，行列采用一基位置，列按Unicode码点计、结束列为右开边界；由同一转换器将AST字节位置转换为该格式。同一行出现两次相同调用不能因缺少列位置而合并成一个调用点。

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

具体沿用当前路由：plain `harness review` 由 Runtime ReviewOptions 选择 AUTO_FULL/AUTO_SINGLE/USER_SELECTION；显式 Controller/Controller.method 直接进入对应 TARGETED，并保留机器要求分支的防遗漏验证，不因为有多个分支就追加链选择菜单。Service 等下游目标出现多上游时仍按现有规则选择范围。

## 10. 按需上下文、性能与覆盖

DISCOVERY 种子由本次实际变更派生；RULES 种子由已选 ReviewUnit 及已分发规则派生。从 caller/callee、实现、SQL 关联补充必要上下文，向上查找也依赖有界文本候选和 AST 确认，不承诺无索引情况下能低成本获得全仓完整反向影响。

默认新增上下文预算（工程初值，不是性能承诺）：

- 下游方法展开 6 跳，上游 3 跳；
- 每 run 最多额外解析 80 个上下文文件；
- 每 run 最多 400 个候选位置进入语义确认；
- DISCOVERY 与 RULES 各最多 40 个额外文件、200 个候选位置、1 MiB 补充源码、15 秒探索耗时，合计上限对应上述 80/400 和 2 MiB/30 秒；
- 每个候选搜索进程最多 5 秒；上述是上下文探索预算，既有认证的源码复验开销单独计算，不声称整个 Review 只需 30 秒。

预算不包括既有 required changed files 的必读责任，不能用“上下文到限”跳过变更文件。代码探索预算由 Runtime 固定，不增加可由 Agent 调整的预算配置；唯一新增可选项目配置为第13节的知识绑定 context.yaml。同一进程内可复用文件读取及 AST 结果；两个独立命令进程不得把落盘解析结果变成跨进程/跨 run 缓存。若分阶段重复解析开销明显，后续合并同进程调用，不引入持久索引。

搜索只能使用参数数组调用已有 Git/ast-grep 执行封装，pathspec 与 pattern 分开。不得 shell 拼接、下载依赖或启动任意 MCP。不能让仅覆盖 tracked 文件的搜索静默漏掉本次 untracked 文件，后者取自既有 ChangeSet。

区分两类不完整：

1. 既有 ChangeSet / EntryPoint / required-file coverage 或认证失败：保持原有硬停止，不进入 Finding Review。
2. 已认证变更范围内的某项规则缺少额外业务上下文：仅该 rule dispatch 标 BLOCKED；其他 READY 规则可以执行，最终报告为“评审未完成”，不报整体通过。

CONDITIONAL / AMBIGUOUS / UNRESOLVED 是关系状态；READY / BLOCKED 是具体规则的就绪状态，不能混用。未分发的规则不计为 BLOCKED。缺少远程源码并不必然阻断所有本地规则。

## 11. Finding 及报告

保留现有 Proposal → Verify → Certify → Runtime renderer 流程；不新增独立认证平台。

扩展 evidence kind 为 CONTEXT_RELATION，并添加 relationId、workspace、sourceSide（BASE/CURRENT）等必要字段；业务依据另用第13节的 BUSINESS_RULE；Agent 只引用 Runtime 返回的 ID。认证时重新验证当前规则所属 ReviewUnit、关系来源及实际源码，不能只检查 ID 在 JSON 中出现。

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
- 旧 run 不被重新认证；新 run 使用 1.7 上下文和已同步的 strict schema。旧运行记录按原协议读取；新run要求匹配的新Runtime/Host，不承诺新旧组件混装互通。

## 13. Review 业务知识设计

### 13.1 知识来源与维护

业务知识由业务/项目负责人确认；调用关系由 Runtime 每次从源码自动派生。公共规范、组件资料和已确认经验沿用内网 TeamAI 现有位置；项目专属正式规则跟随业务代码版本，团队资料只引用其权威位置。已有资料不迁移、不复制第二份可独立编辑正文。

TeamAI 负责分发，Harness 只消费已经准备在本机的明确来源。没有 TeamAI 或未启用 Recall 时仍能读取已准备资料；本版不启用索引、图、向量检索或自动经验生成。历史经验只能作为参考，不能替代当前源码依据。

### 13.2 唯一新增的可选项目绑定

拟新增 `.code-harness/context.yaml`，属于 Project State，不属于 Framework Managed，不在正式包中预置公司实例。解析复用现有 `yaml.v3` 和 JSON Schema，不增加依赖。

以下是虚构项目的格式示例，**对应能力待 1.7 实现**，不能直接用于旧版：

```yaml
version: 1
projectId: order-service
teamRoot: D:/team/total-repo
sources:
  - id: order-state-rule
    root: PROJECT
    path: docs/business/order-state.md
    kind: RULES
    required: true
  - id: shared-idempotency
    root: TEAM
    path: docs/components/idempotency.md
    kind: REFERENCE
    required: false
```

确定规则：

- 顶层只允许 `version/projectId/teamRoot/sources`；source 只允许 `id/root/path/kind/required`。拒绝未知字段、重复 YAML key、重复 sourceId、非 UTF-8 内容和多 YAML 文档。
- `version=1`；projectId/sourceId 使用 `[A-Za-z0-9][A-Za-z0-9._-]{0,127}`。root 为 PROJECT 或 TEAM；kind 为 RULES、REFERENCE、EXPERIENCE；required 是显式布尔值。
- 本版RULES来源必须设置required=true，正式规则不能因资料缺失被悄悄降为可选；REFERENCE/EXPERIENCE按实际需要声明必需性。ruleId沿用sourceId的字符约束，避免BUSINESS规则键分隔符歧义。
- 每个 source 指向一个普通 Markdown 文件，不接受目录、glob、URL、脚本或压缩包。配置最多 100 条来源，读取内容仍受本节预算限制。
- PROJECT 固定是当前业务 Git 工作区根。TEAM 是用户已有本地资料目录；teamRoot 允许本地绝对路径或相对业务根的路径，拒绝 URL、UNC/设备路径和网络挂载目标。没有 TEAM 来源时可省略 teamRoot，有 TEAM 来源则必须明确指定。
- teamRoot 可以是内网总仓的本机检出目录，也可以是 TeamAI 实际 docs 同步目录。source.path 始终相对选中的根；后者通常不再包含原仓库的 `docs/` 前缀。不猜 TeamAI 私有缓存路径，不新建同步命令。
- `path` 规范化后必须留在对应根内，拒绝绝对路径、`..`、Windows ADS、符号链接/重解析点越界、非普通文件。既有 projectpath/workspace 的安全封装优先复用；读取前后检查文件身份，变化时不使用不一致内容。
- `projectId` 与项目专属正式规则的 projectId 必须相同；公共规则可用 `*`。这验证输入一致性，不宣称能证明人工填写的项目归属真实正确。
- 配置仅由项目维护人按需编辑。本次 Review 不自动创建、补全或改写配置；同一份配置的 source 清单与 teamRoot 变更会使依赖旧绑定的结论失效。

配置缺失时保持旧项目技术 Review；报告明确“未配置业务规则检查”。配置存在但不合法时拒绝读取相关知识，标记业务检查未完成；不因配置损坏改读整个磁盘。非法命令请求、run 身份或既有代码认证失败仍走硬停止。

### 13.3 正式规则格式与适用性

RULES 文件一文件一条规则，Markdown 顶部使用 YAML frontmatter，正文保留原业务描述。REFERENCE/EXPERIENCE 可以继续使用普通 Markdown，不强制改造已有资料。

```markdown
---
ruleId: ORDER-STATE-001
projectId: order-service
status: ACTIVE
version: "1"
owner: order-team
source: requirements/order-state.md
approvalRef: reviews/order-state-approved.md
appliesTo:
  paths:
    - src/main/java/example/order/
  entryPoints: []
supersedes: []
exceptions: []
---
只有待确认状态的订单允许确认；已经取消的订单不得再次确认。
```

示例仅解释格式，不代表公司的实际要求。frontmatter 必填 `ruleId/projectId/status/version/owner/source/approvalRef/appliesTo/supersedes/exceptions`；status 为 DRAFT、ACTIVE、RETIRED。`appliesTo.paths` 是业务仓相对文件路径或以 `/` 结尾的目录前缀，不使用自定义 glob；`entryPoints` 使用 `ownerFqcn#method(parameterTypes)` 的精确身份。两组条件取并集，命中已选 ReviewUnit 文件或已确认入口则适用；至少提供一个选择条件；只有显式 `paths: ["./"]` 表示全项目，两组空列表无效。source 和 approvalRef 为人工可追溯来源字符串，运行时不访问其中的网址或执行其内容。

正式规则的 ID 和版本必须稳定；owner/source/approvalRef 非空且正文有业务要求。ACTIVE 是状态声明，不是审批证明；项目负责人通过已有评审流程维护规则及批准来源。不能自动把草稿或 Agent 刚写入的 ACTIVE 当成业务批准。

规则选择由 Runtime 根据已认证 scope/unit 计算，不能只由模型“觉得相关”或 Recall 命中决定。跨模块上下游规则只有适用条件实际匹配本次范围时才加入正式业务检查，不自动扩大 Finding Anchor。

- 明确不适用的规则标 NOT_APPLICABLE，不形成 BLOCKED。
- 已声明必需但文件缺失、元数据错误或无法判断适用性的规则，保守阻断当前选定范围的业务规则检查；不能因为没读到适用条件而跳过。
- 不同文件使用同一 ruleId 时拒绝作为生效规则，不静默选择最新文件。替代关系只能由明确的 `supersedes` 与既有批准来源说明，不能按文件时间猜测。
- Runtime 检查重复 ID、项目不匹配及版本一致性；supersedes仅用于追溯，不自动隐藏另一条ACTIVE规则，旧规则由负责人按原流程标为RETIRED。自然语言规则矛盾由 Reviewer 标记相关检查 INCOMPLETE，不能声称机器能穷尽检测语义矛盾。
- 同一次变更修改代码与规则时，报告单列规则变更及批准依据。工作区新规则不能自动豁免旧要求；生效性不明确则业务项未完成，不能让作者通过改规则自证代码正确。

### 13.4 运行接口、阶段与读取预算

新增一个内部命令，不增加用户产品命令：

```text
codea-dcep-tools.exe review knowledge --input .code-harness/runs/<runId>/requests/review-knowledge.json
```

request 严格只有 `{"runId":"review-example"}`。命令层从当前根的 context.yaml 和同 run 已认证分析/selection/ReviewUnit 取得输入；Agent 不传资料根、来源清单、路径、内容、哈希、必需性或 READY 状态。

它在 REVIEW_PLANNING 内、ReviewUnit 生成后、最终 RuleDispatch 生成前执行。Runtime 为每个适用业务规则构造 `BUSINESS:<sourceId>:<ruleId>` 的 rule key，并与 unitId 配对加入既有 dispatch；此命名空间不能与内置规则冲突。未启用知识、无适用规则和技术规则零分发分别保留其真实语义。

业务规则以本次临时规则集接入：Kind=AGENT，RuleVersion=1表示业务规则适配协议版本，业务文档的version字符串原样保留在知识元数据中；默认严重级别medium，实际问题仍按影响与证据判定。RequiredEvidence包含BUSINESS_RULE及源码证据。1.7有效RuleCatalog摘要绑定内置catalog与knowledgeSha256，证据认证按同一临时规则集验证；不能让BUSINESS规则因不在静态spring-v1.yaml中被全部拒绝，也不能把公司规则写回该静态文件。

每 run 知识正文最多读取 20 个文件、合计 256 KiB；EXPERIENCE 最多 5 个。先读取所有 RULES，按规范化来源顺序处理，再处理 required 参考资料，最后按声明顺序读取 optional 参考和经验。可提前判断“不适用”只能基于已经合法读取的规则元数据。

超限不能截断正式规则后仍宣称已完整读取；未读的必需资料对应检查 BLOCKED，其他 READY 检查可继续。知识预算独立于代码上下文的 80 文件/400 候选/2 MiB/30 秒预算，均不豁免既有必读变更文件。

运行中保存 `analysis/review-knowledge.json`：runId、绑定摘要、sourceId/ruleId、相对路径、内容 SHA256、适用 unit、状态和原因。**不保存源文档全文，不维护历史文档副本、索引或可恢复快照。** 当次读取到的内容及已验证位置只通过工具结果或既有只读文件工具提供给独立 Reviewer；前者可原样转交，Main Agent 不概括成新的权威规则。Host 无权读取时标记缺口，不扩大工具权限。

知识内容中的命令、登录文字和“跳过校验”等提示视为资料，不成为执行指令。仅允许声明的本地资料，引用不触发网络读取、平台登录或其他操作。

### 13.5 认证、变化检测与报告

新增 evidence kind `BUSINESS_RULE`，必填 `sourceId/ruleId/sourceSha256`；业务 Finding 同时需要该规则引用、本次源码依据、适用 unit 和已核验变更关联。知识只能当证据，不能当源码 workspace 或正式 Anchor。

knowledge manifest 的规范序列化摘要绑定到最终 RuleDispatch 和既有 CertifiedSet/certificate；认证与本次首次报告发布前重新读取配置和实际被依赖的资料，比较摘要、适用性、源码证据与 dispatch 归属。绑定被删改、source 已变化、ruleId 被替换或额外字段被篡改时，该次上下文失效并停止正式结论发布，重新发起 Review；不能在原 run 内替换依据后沿用旧 Reviewer 结果。

被依赖的资料包括所有进入 Reviewer 上下文的文档，以及用于计算规则适用性/分发的规则文件；不能只检查最终 Finding 引用的少数文件，否则“没有产生问题”的检查会失去变化校验。预算外未读取的来源按原缺口状态处理，不伪装成已复验。

历史报告保持原有历史产物语义，不因之后知识库正常更新而重写旧报告，也不能将历史结论直接用于当前代码。当前 run 的知识元数据不是新认证平台；复用已有摘要/证书和 freshness 机制。

### 13.6 READY 与完成检查的区别

READY 仅表示材料足够。1.7 的 FINDINGS 提交在保留 `finding-proposals.json` 顶层数组的同时，增加独立 `requests/review-checks.json`，逐项列出当前分发且 READY 的 `(reviewUnitId, ruleId)`：

```json
[
  {
    "reviewUnitId": "unit-example",
    "ruleId": "BUSINESS:order-state-rule:ORDER-STATE-001",
    "status": "COMPLETED",
    "reason": "",
    "sourceIds": ["order-state-rule"]
  }
]
```

只允许 COMPLETED/INCOMPLETE；INCOMPLETE 必须给具体 reason。Runtime 自己形成的 BLOCKED 项不需要 Reviewer 伪造完成记录。对所有技术/业务 READY 项都必须恰好一条，缺失或重复不能当完成；不允许加入未分发 unit/rule，资料 ID 不能越过已读取集合。

沿用 `codea-reviewer-submit` 的 kind=findings，仅增加 `checks` JSON 字符串参数。该工具独占写入两个请求文件，最后写入现有 finding authority receipt 的 v2 形式，绑定两个内容摘要。Host 会话认证必须校验同一独立 Reviewer 工具调用中的 proposal 与 checks 均匹配。部分写入或 v2 receipt 缺失不能认证，不要求新建多文件事务框架。

只有受支持的 1.7 run 接受 v2；旧 run 沿用原 v1 读取与证据规范。选择契约版本基于 Runtime 记录的可信运行版本，不能由 Agent 传版本降级。新增 `reviewChecksSha256` 与 `knowledgeSha256` 纳入既有证书，不创建第二张证书。CHANGE_ANALYSIS 仍保留原 submission 语义。

这只能核验“受信 Reviewer 对哪些检查作了完成声明及其来源”，不证明语义判断必然正确；实际质量仍靠反证和真实样例验证。Reviewer 失败/超时仍按原 Host 合同硬停止，不允许用 INCOMPLETE 把 Host 失败转换成可发布报告。

## 14. Git、TeamAI 与 Harness 的协同边界

本节是已确认的维护方式：**TeamAI 原有文件结构和管理方式不变，只新增 Harness 接入内容。** 不创建第二套公司知识目录、不迁移旧资料、不让 TeamAI 充当 Runtime 安装器。

| 对象 | 权威来源与维护方式 | 本版新增 |
|---|---|---|
| 总仓所有文件 | 内网 Git 管理版本、审查变更 | 可附带完整 Harness 正式包 |
| TeamAI 已有资源 | 原 skills/rules/docs 及既有项目/角色配置 | 一个轻量入口、一份必要协作约定、接入说明 |
| Harness Runtime/核心 Skill/Reviewer | 核心仓库统一维护、正式包整体交付 | Review 增强；原安装升级机制延续 |
| 公司/项目资料 | 原有 TeamAI/业务仓库位置 | 按需补齐适用规则与本地来源绑定 |
| 本次运行和项目状态 | 业务项目现有 .code-harness | 新增可选 context.yaml 和本次知识引用记录 |

```text
内网总仓库/
├── teamai.yaml                 # 保留原配置
├── skills/                     # 保留原结构，仅新增轻量入口
├── rules/                      # 保留原结构，仅新增必要协作约定
├── docs/                       # 保留原知识位置，仅新增接入说明
├── …                           # 其他既有 TeamAI 资源保持
└── code-harness/               # 普通 Git 管理的完整正式包存放目录示例
    └── <批准版本的完整包>/
```

`code-harness/` 的名称和位置不是 TeamAI 协议；已有内网制品目录即可使用，不强制新建目录。完整包保持内部布局，不把其中 `.code-harness/.opencode/install.ps1` 拆到总仓多个位置。

当前核对的 1.6.4 正式 install 包包含 `.code-harness/`、`install.ps1` 及项目级 `.opencode/agents/reviewer.md`、`.opencode/commands/harness-review-reviewer.md`、`.opencode/tools/codea-reviewer-submit.ts`。三个 Host 文件随 Harness 管理，不由 TeamAI 再生成一份 Reviewer。源码目录缺运行程序，不能替代正式包。

操作语义固定：

- Git 更新总仓文件，不等于业务项目 Harness 已升级。
- `teamai init/pull` 按原流程部署 TeamAI 资源；知识维护/贡献沿用现有 Git 或 TeamAI 工作流，不新增自定义命令。
- Harness 首次安装仍走正式安装器；已有项目仍走正式 upgrade 包与入口。1.7 仅补齐新文件清单和必要兼容处理，不重建安装升级机制。
- Review 运行中不主动执行 Git/TeamAI 同步，不自动升级或下载。已有会话开始同步按内网设置运行；运行中发现依据变化按本次 freshness 处理。

### 文件冲突与版本

`skills/codea-harness-review/` 与核心 `.code-harness/skills/**` 分别维护，入口只路由到安装版本，不复制核心审查提示。TeamAI 不维护核心 Agent/提交工具；共享 OpenCode 配置遵守已有按条目管理方式，不整文件覆盖。

核心 Skill、Runtime、Host 三件资源必须来自同一受支持正式包。TeamAI 入口优先读取项目已安装 bootstrap/合同与版本信息；缺能力则说明，不能把 1.7 配置强塞给旧版。入口通常不跟随每个补丁版本修改。

## 15. 内外网边界和依赖

外网只维护通用核心与 `teamai-harness` 通用接入资源。实际公司知识、业务代码、项目配置、内部地址、凭据和运行报告留在内网；不自动回传、不提交到公共模板仓。

同一内网总仓可以管理知识和正式包，不为此建立核心分叉或补丁目录。通用问题在外网用不含公司资料的最小复现修复，产生新正式版本后按原审批流程导入。

内网完全不可访问公网是固定条件。默认零新增第三方依赖；现有 Git、OpenCode、TeamAI、PowerShell、Go 和正式包也须使用公司已批准版本。缺包或依赖逐项申请；不得通过打包/vendor、公共镜像、自动下载绕过申请。工具能启动不代表其完整依赖已获批准。

查询日志、平台登录、验证码、连接器、MCP、经验自动贡献、故障修复不进入本版。已有其他功能保持原样，不预建未来功能目录或接口空壳。TeamAI 目录分组不等于 Git 读权限隔离，敏感项目资料继续留在已有受限位置。

## 16. 状态、兼容与失败处理

八个 Runtime 阶段名称与权威保持：REVIEW_BEGIN、SNAPSHOT、CHANGE_ANALYSIS、CERTIFICATION、REVIEW_PLANNING、REVIEW_EXECUTION、FINDING_CERTIFICATION、REPORT。上下文读取是阶段内子操作，不新增 Agent 可控制的 advance/complete 接口。

| 子操作 | 允许的现有阶段 | 前提 |
|---|---|---|
| context DISCOVERY | CHANGE_ANALYSIS | 同 run Runtime ChangeSet 已生成；不替代 Reviewer |
| knowledge | REVIEW_PLANNING | analysis/selection/ReviewUnit 已验证，dispatch 尚未发布 |
| context RULES | REVIEW_PLANNING | 含知识绑定的 units/dispatch 已验证；计算上下文 READY/BLOCKED |
| 关系与知识复验 | CERTIFICATION 或 FINDING_CERTIFICATION，按消费阶段 | Runtime 只复验被使用的上下文 |
| 结论发布前 freshness | REPORT | 已认证 set、Host 与知识身份一致 |

规划顺序固定为：Unit → knowledge → 发布一次不可变dispatch → context RULES → Runtime推进REVIEW_PLANNING。dispatch表达应检查哪些规则，READY/BLOCKED保存在上下文结果中，不反写dispatch。1.7的review dispatch写入后不立即推进；context RULES在校验同run知识、dispatch及上下文齐备后才按内部既有接口推进。旧run保留旧推进边界。不存在Agent可调用的通用finalize/advance命令，失败后不能通过重复context跳过阶段。T5/T7须共同验证这一顺序。

阶段 SUCCEEDED 表示该操作按协议产出了结果，不等于代码审核通过；合法的部分检查结果可以完成 REPORT 阶段，但报告结论仍必须为未完成。用户报告不展示逐项通过清单。

| 情况 | 处理 |
|---|---|
| 未配置知识 | 技术 Review 按原范围执行，声明未启用业务检查 |
| 0-change / LIST | 保留原产品语义；不调用 FINDINGS 或额外生成业务结论 |
| 可选资料缺失且无实际依赖 | 记录限制，不自动阻断全部 Review |
| 必需资料缺失、超预算、规则冲突 | 相关业务项 BLOCKED/INCOMPLETE，其余 READY 可执行，整体未完成 |
| 声明范围内代码未读、认证失败 | 沿用硬停止，不能继续 FINDINGS |
| Reviewer 无法启动/完成或会话认证失败 | 既有 reviewer-unavailable 失败上报与 HARD STOP，禁止主 Agent 接管 |
| Reviewer checks 漏项、伪造或跨 run | 不签发完整结论；无效 Host 提交按原失败合同停止 |
| 已使用知识/绑定在认证前后变化 | 本次上下文失效，停止发布，重新 Review |
| 所有应检项完成、没有正式 Finding | 一句未发现问题，保留真实范围 |

新 run 由可信 Runtime 版本决定新契约要求；旧 run 与旧证书保留原序列化验证，不添加零值字段后重新计算旧摘要。不要把旧记录重新认证为 1.7。兼容的含义是旧资料可按历史规则阅读，不承诺混装新旧 Runtime/Host 能互相调用。

安装升级适配仅覆盖必要事项：登记新 Framework 文件；context.yaml 登记为 Project State；1.6.4→1.7 更新受管 Host 资源；本地不同内容冲突保持保护；保留 harness.yaml/project.md/database.yaml/chains/runs 及已有知识。当前 Host 升级触发条件仅跨入 1.6.4，不能假设它自动处理 1.6.4→1.7 新 Host 协议，需在原升级机制中补版本适配。

## 17. 验收矩阵与完成定义

| 编号 | 必须覆盖的场景 | 对应任务 |
|---|---|---|
| A1 | Java 同名/重载/继承/遮蔽/递归、并列调用不串联 | T1、T2、T5 |
| A2 | Spring 明确注入、条件缺失、第二实现加入、不执行容器 | T2 |
| A3 | Mapper/XML/参数/include/循环/动态 SQL 正反例 | T3 |
| A4 | Dubbo group/version/import、Provider 缺失和未批准依赖 | T4 |
| A5 | STALE 自动临时链、保存 YAML/备注字节不变、实际选择仍保留 | T6 |
| A6 | FULL/TARGETED/0-change/LIST、预算、untracked、依赖只作证据 | T5、T6、T8 |
| A7 | 知识无配置、显式绑定、路径越界、元数据、适用性、草稿/重复/冲突 | T1、T7 |
| A8 | 必需资料缺失、同改规则代码、知识/绑定变化、规则与代码双重证据 | T7、T8 |
| A9 | READY 不等于完成；Host v2 两份提交、缺 checks、伪造 receipt、未完成不报通过 | T8 |
| A10 | TeamAI 布局不迁移、同步不写核心、入口不接管 Reviewer、知识唯一来源 | T9 |
| A11 | 原安装/升级流程完整包、Host 同版本、新 Project State 保留 | T9、T10 |
| A12 | 实际 Windows/禁公网/批准模型、零自动下载和无依赖增量 | T10 |

代码模型验收至少 12 个独立案例：6 个有明确依据的缺陷、6 个正确反例。另加 6 个知识案例：规则违反、符合规则、必需规则缺失、冲突、变更导致引用失效、草稿不能豁免。输入与答案分离，不注入期望 proposal，不把固定 proposal 的既有 24-case 合约测试当模型能力验收。

对可判定案例要求预期高价值缺陷有有效依据、正确反例无错误正式 Finding；缺口案例要求准确未完成，不冒充通过。失败逐例给实际输出、归因和复测；18 个案例不能支撑统计性的全业务精确率承诺。代码质量比较固定基线/模型/规则/预算，记录误报、漏报、耗时和人工介入；不另建评测平台。

功能完成须同时具备：T1–T10 的必要测试、相关现有 Workflow、真实 Reviewer/Windows/内网验收和可用正式包。缺实际环境的项标记“未执行”，不能用 mock 代替。编写本文及执行计划不代表上述功能已完成。

## 18. 研发交付与文档归属

本设计和配套执行计划的唯一维护源是 `lingyi9909/codea-harness`。研发从同一提交读取两份文件；内网按已批准流程导入对应版本的离线副本，保持原相对目录，不独立修改成另一套设计。`teamai-harness` 仅保留接入资源及指向当前设计的说明，不要求 TeamAI 将完整研发文档作为日常知识加载。

正式开发以验收通过的 1.6.4.final 为基线，独立分支实现。本文核对的 main `b864bd19b9313bd882da89612e701d8bb3221e7d` 仅证明当前文件/接口位置，不能代替最终验收记录。T1 首先核对最终 SHA 和适用 Gate，再把文档要求映射到最终树。

参考来源：[TeamAI 官方资源布局](https://github.com/Tencent/teamai-cli/blob/main/README.md)、[核心 Reviewer 合同](https://github.com/lingyi9909/codea-harness/blob/b864bd19b9313bd882da89612e701d8bb3221e7d/.code-harness/contracts/reviewer-host-contract.md)、[核心安装说明](https://github.com/lingyi9909/codea-harness/blob/b864bd19b9313bd882da89612e701d8bb3221e7d/README.md)。这些外网链接用于来源追溯；内网执行不访问它们，也不下载任何资源。
