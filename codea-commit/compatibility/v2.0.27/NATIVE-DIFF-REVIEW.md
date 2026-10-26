# 2026-10-26 原生 Diff 与简洁确认：验证记录

本轮用户决定复用 OpenCode 原生 Diff，撤销附件面板与新增界面宿主开发。此次仅修改现有 Command/Skill/模板、对应候选和记录。实际公司 TUI/Windows/私有模型/TeamAI 均 PENDING，不提供伪造截图或将工具加载称为界面演示。

## 真实运行

- `python3 tests/fixtures/codea-commit/test_fixtures.py`：21/21 PASS。
- `python3 -m unittest discover -s tests -v`：40/40 PASS。
- `python3 tests/fixtures/codea-review-t1/test_git_scope.py`：4/4 PASS。
- 最终三文件资源在修复后公开 `opencode v2.0.27` 源码CLI服务隔离探针：12/12 PASS，摘要与SOURCE一致。包含11项API边界和1项临时模板落盘检查，不调用模型，不执行产品commit。中间版探针也通过，但因后续规则修正没有计作最终资源通过；一次重跑因报告路径已存在被工具拒绝，随后用新路径完整重跑。
- 新临时C02真实Git对照：7/7 PASS。只读status不改index原始字节；真实cached与捕获tree对象Diff相等；外部add后live tree变化，而原快照Diff不变；空树按当前对象格式取得并可用于根提交Diff。原始数据见NATIVE-DIFF-SNAPSHOT.json。这些检查验证Git方法，不代表模型严格执行了指令。
- 既有宿主补丁SHA256未变；Review/模型/权限/锁文件未改。

## 独立只读提示推演（不是产品模型验收）

RED基线：旧规则“读实际 Diff…列出明确路径，只暂存该组”只要求模型读取，允许用户尚未看到就add。旧模板冗长，但没有实际要求不存在的增强面板，不能把它误记成旧缺陷。

GREEN：明确用户完整看到候选真实变更后才决定范围及add；真实index最终Diff完整后再显示简洁确认。范围已明确不重复询问；部分暂存不重新add；大Diff截断继续补齐、无法补齐则不确认；说明修改/保存后再确认；取消/关闭结束本轮；基线变化旧确认失效；hook失败/超时查实际结果且不重试；重复确认不再创建提交。

第一轮独立审阅发现并修复：最终读取Diff=A后绑定tree=B的快照缺口（P1），以及status默认可刷新index与只读声明冲突（P2）。改为同一对不可变HEAD/空树与index tree读取全部分段/统计，发确认前及提交前复核当前基线；只读调用使用--no-optional-locks。

第二轮发现并修复第2节动态cached与第3节不可变快照的表述冲突；根提交明确空输入git mktree。最终独立只读复审：未发现剩余P1/P2，三个payload/candidates逐字一致。审阅者未修改文件或执行Git写操作。

## 原生能力与安装边界

固定参考源：v2.0.27 / 1f4d07a4a519185955ae8ab2ea3607805509e386。原生Diff有branch/committed/working/turn，没有index模式；工作区/会话视图只能辅助浏览，不冒充暂存内容。最终真实index快照差异经现有工具/通道展示，不新增面板、index Viewer或消息编辑器，不承诺Skill自动打开或注入这些界面。

三个产品文件、两个安装目标不变：Command文件及完整Skill目录（包含templates/commit-preview.md）。既有模型前参数拦截安全安装门禁保留，先通过公司等价能力核验；本轮没有新增宿主UI源码修改。P01—P15仍PENDING。历史原生Core/Server测试和消息清理/CRLF证据保留原日期，未重跑未变化的大套件。
