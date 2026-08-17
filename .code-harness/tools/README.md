# 受控工具契约

Subagent 只能使用以下操作。具体实现必须拒绝 `harness.yaml` 范围之外的值。

---

## 只读工具

### `git_diff(baseRef, headRef?, includeWorkingTree?) -> DiffResult`
返回本次 Review 的完整 Change Set（变更文件及变更块）。只读。

**参数规则**：

- `baseRef`：必填。默认来自 `harness.yaml.review.baseRef`。这是评审基线，不能为空。
- `headRef`：可选。默认 `HEAD`。
- `includeWorkingTree`：可选。默认来自 `harness.yaml.review.includeWorkingTree`。

**比较语义（必须使用 merge-base）**：

```text
mergeBase = merge-base(baseRef, HEAD)
```

已提交变更范围：

```text
mergeBase → HEAD
```

等价于：

```text
baseRef...HEAD
```

**禁止**直接使用普通 `git diff`（只对工作区，看不到分支差异）作为 `harness review` 的完整变更来源。

**纳入工作区变化**：

当 `includeWorkingTree: true` 时，必须同时获取四部分：

```text
1. committed   —— merge-base(baseRef, HEAD) → HEAD
2. staged      —— git diff --cached
3. unstaged    —— git diff（工作区相对 index）
4. untracked   —— 未被 Git 追踪的文件（普通 git diff 看不到，必须主动获取文件列表）
```

`untracked` 文件必须主动枚举，并把新文件内容作为新增文件纳入 Review。

**同一文件合并去重**：

如果同一文件同时存在多个来源的修改（例如 `OrderService.java` 既有已 commit 修改，又有当前 unstaged 修改），不能作为两个独立文件重复 Review。应合并为一个统一 Change Set 条目：

```text
OrderService.java

sources:
- COMMITTED
- UNSTAGED
```

Reviewer 应看到该文件相对于 merge-base 的最终有效状态变化。

**返回 DiffResult**：

```json
{
  "currentBranch": "feature/order",
  "baseRef": "origin/master",
  "baseCommit": "abc111",
  "mergeBase": "abc000",
  "headRef": "HEAD",
  "headCommit": "abc999",
  "includeWorkingTree": true,
  "sources": {
    "committed": true,
    "staged": true,
    "unstaged": true,
    "untracked": true
  },
  "files": []
}
```

`sources` 表示本次实际纳入的变更来源；`files` 为去重后的统一变更文件列表。后续 Agent 据此清楚本次到底 Review 了什么。

### `git_refs() -> GitRefsResult`
只读。列出本地已有的 Git refs，供 Project Adapter 识别 `review.baseRef`。**不读取文件正文，不执行 `git fetch`、`git pull` 或联网更新远端状态**。

返回：

```json
{
  "currentBranch": "feature/order",
  "detachedHead": false,
  "localBranches": ["master", "develop"],
  "remoteBranches": ["origin/master", "origin/main", "origin/develop"],
  "originHead": "origin/master"
}
```

- `currentBranch`：当前分支名，Detached HEAD 时为 `null`。
- `detachedHead`：是否处于 Detached HEAD 状态。
- `localBranches`：本地分支列表。
- `remoteBranches`：远端跟踪分支列表（`refs/remotes/origin/*`）。
- `originHead`：`refs/remotes/origin/HEAD` 指向的目标（如 `origin/master`），无则为 `null`。

### `read_code(paths, lineRanges?) -> CodeBundle`
读取 source/test scope 允许的仓库文本文件。只读；glob 匹配不得逃出项目根目录。

### `read_test_report(runId) -> TestReportBundle`
仅读取配置的 `reportDir` 下的 Maven 进程输出和文件。

### `read_service_logs(runId, from, to) -> LogBundle`
读取采集的进程日志和配置的应用日志，限定请求的时间窗口。

### `list_project_tree(root, maxDepth?, includes?, excludes?) -> TreeResult`
读取目标项目目录结构。不读取文件正文。默认排除 `.git`、`target`、`node_modules`、日志目录和大型制品目录。

### `read_project_file(path) -> FileContent`
允许读取：`pom.xml`、Java 源码、测试源码、`application` 配置、Maven Wrapper 配置、项目已有 `AGENTS.md`。

**禁止读取**：密钥文件、`.env`、生产凭据、证书、私钥、用户明确禁止的路径。

**默认不读取**：`application-prod.*`、`bootstrap-prod.*` 及其他生产环境配置文件。

**敏感信息脱敏**：对于允许读取的配置文件，返回内容前必须对以下键的值进行脱敏处理：

```
password, passwd, secret, token, accessKey, secretKey, credential, privateKey
```

脱敏后示例：

```yaml
spring:
  datasource:
    username: test_user
    password: "***REDACTED***"
```

初始化只需要分析配置键、Profile 和路径，不需要密码原文。

---

## 进程管理工具

### `run_maven_test(testClass, runId) -> ProcessResult`
执行 `integrationTest.executable` 中配置的命令及替换 `${testClass}` 后的 args。强制超时；不经过 Shell 求值。

### `start_service(runId) -> ServiceHandle`
执行配置的 service executable 和 args，将 stdout/stderr 采集到 `runs/<runId>/service.log`，记录 PID 和时间戳，并评估就绪状态。

返回 `ServiceHandle`：

```json
{
  "rootPid": 1234,
  "startedAt": "2026-08-04T10:00:00Z",
  "processGroup": 1234
}
```

### `stop_service(runId, serviceHandle) -> StopResult`
停止 `serviceHandle.processGroup` 标识的进程树。只停止同一 run 中由 `start_service` 记录的进程。拒绝未知或不匹配的 handle。

---

## 写入工具

### `write_test(path, content, planId) -> WriteResult`
需要经人工审批的测试计划，以 `planId` 标识。路径必须匹配 `write.allowedTestPaths` 且不在 denied paths 中。

### `apply_approved_patch(fixPlanId, changes) -> PatchResult`
需要经人工审批的修复方案，以 `fixPlanId` 标识。每个变更路径必须匹配 `allowedProductionPaths`、避开 denied paths，且出现在审批通过的方案中。

### `write_harness_file(path, content) -> WriteResult`
默认只允许写入 `.code-harness/harness.yaml` 和 `.code-harness/project.md`。禁止写入 `src/main/**`、`src/test/**`、`pom.xml`、`application*.yml`、`application*.yaml`、`application*.properties`。

### `update_root_agents_entry(approved, content) -> WriteResult`
在目标项目根目录创建或更新 `AGENTS.md` 中的 Codea Harness 快捷入口。

规则：

1. 只有用户明确同意（`approved: true`）后才能调用。
2. 只能创建或更新 `<!-- CODEA-HARNESS:START -->` ... `<!-- CODEA-HARNESS:END -->` 标记区块。
3. 不得修改标记区块之外的任何内容。
4. 根目录无 `AGENTS.md` → 创建新文件，仅包含标记区块。
5. 根目录已有 `AGENTS.md` 但无标记区块 → 在文件末尾追加标记区块。
6. 已有标记区块 → 只替换区块内容，不重复追加。
7. 写入前必须展示将要增加/修改的完整内容。
8. 用户拒绝时不得调用。

固定标记区块内容：

```markdown
<!-- CODEA-HARNESS:START -->

## Codea Harness

执行以下 Harness 意图时，必须先读取：

- `.code-harness/AGENTS.md`
- `.code-harness/agents/orchestrator.md`
- `.code-harness/harness.yaml`
- `.code-harness/project.md`

不得绕过其中的审批门禁和安全约束。

<!-- CODEA-HARNESS:END -->
```
