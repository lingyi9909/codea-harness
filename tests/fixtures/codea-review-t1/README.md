# T1 Java Git Diff Fixture（无外网依赖）

这是一个用于 Java Review **功能冒烟**的小样本，不是 T2 的 F01～F08 准确率基准。

目录含 `before/` 与 `after/` 两个版本。运行 `setup.ps1` 时在指定新目录创建独立 Git 仓库：`master` 先提交 before，再切 `feature/t1-risk-review` 提交 after。所有身份配置仅写入临时测试仓库的本地 Git 配置。

## Windows PowerShell 示例

```powershell
# 在本仓库根目录运行，目标目录不存在或为空
& .\tests\fixtures\codea-review-t1\setup.ps1 -Destination "$env:TEMP\codea-t1-中文 空格"
Set-Location "$env:TEMP\codea-t1-中文 空格"
git status --short
git diff master...HEAD --stat
# 然后启动公司允许的 OpenCode 2.x TUI，输入 /codea-review
```

预期差异：`OrderService.charge` 移除了按 `requestId` 防止重复调用的进程内保护；Mapper XML 的 `UPDATE` 去掉了 `tenant_id` 限定。`OrderController` 不变，但会调用受影响的 Service。请检查模型是否能把 Finding 关联到**真实变更点**及 Controller 入口。该样本没有模拟多实例全局幂等，不能宣称旧实现具有生产级幂等保障。

若模型没有发现其中任何风险，仍应输出如实范围和未解析项；T1 记录现象，T2 再专项提高召回率。

`expected-report-example.md` 仅说明报告应有哪些字段，并不是实际模型测试证据。不要把示例内容当作 OpenCode TUI 日志。
