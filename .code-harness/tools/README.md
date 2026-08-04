# 受控工具契约

Subagent 只能使用以下操作。具体实现必须拒绝 `harness.yaml` 范围之外的值。

## `git_diff(baseRef?, headRef?) -> DiffResult`
返回变更文件及变更块。只读。

## `read_code(paths, lineRanges?) -> CodeBundle`
读取 source/test scope 允许的仓库文本文件。只读；glob 匹配不得逃出项目根目录。

## `write_test(path, content, planId) -> WriteResult`
需要经人工审批的测试计划，以 `planId` 标识。路径必须匹配 `write.allowedTestPaths` 且不在 denied paths 中。

## `run_maven_test(testClass, runId) -> ProcessResult`
执行 `integrationTest.executable` 中配置的命令及替换 `${testClass}` 后的 args。强制超时；不经过 Shell 求值。

## `start_service(runId) -> ServiceHandle`
执行配置的 service executable 和 args，将 stdout/stderr 采集到 `runs/<runId>/service.log`，记录 PID 和时间戳，并评估就绪状态。

返回 `ServiceHandle`：
```json
{
  "rootPid": 1234,
  "startedAt": "2026-08-04T10:00:00Z",
  "processGroup": 1234
}
```

## `stop_service(runId, serviceHandle) -> StopResult`
停止 `serviceHandle.processGroup` 标识的进程树。只停止同一 run 中由 `start_service` 记录的进程。拒绝未知或不匹配的 handle。

## `read_test_report(runId) -> TestReportBundle`
仅读取配置的 `reportDir` 下的 Maven 进程输出和文件。

## `read_service_logs(runId, from, to) -> LogBundle`
读取采集的进程日志和配置的应用日志，限定请求的时间窗口。

## `apply_approved_patch(fixPlanId, changes) -> PatchResult`
需要经人工审批的修复方案，以 `fixPlanId` 标识。每个变更路径必须匹配 `allowedProductionPaths`、避开 denied paths，且出现在审批通过的方案中。

## `list_project_tree(root, maxDepth?, includes?, excludes?) -> TreeResult`
读取目标项目目录结构。不读取文件正文。默认排除 `.git`、`target`、`node_modules`、日志目录和大型制品目录。

## `read_project_file(path) -> FileContent`
允许读取：`pom.xml`、Java 源码、测试源码、`application` 配置、Maven Wrapper 配置、项目已有 `AGENTS.md`。禁止读取：密钥文件、`.env`、生产凭据、证书、私钥、用户明确禁止的路径。

## `write_harness_file(path, content) -> WriteResult`
默认只允许写入 `.code-harness/harness.yaml` 和 `.code-harness/project.md`。禁止写入 `src/main/**`、`src/test/**`、`pom.xml`、`application*.yml`、`application*.yaml`、`application*.properties`。修改目标项目根目录 `AGENTS.md` 不得通过该工具自动完成，必须使用单独的用户确认流程。
