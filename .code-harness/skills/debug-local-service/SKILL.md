---
name: debug-local-service
description: 启动本地服务，记录进程树（ServiceHandle），验证就绪状态，为调试时间窗口采集日志。
version: 1
agent: runtime-debugger
tools:
  - start_service
  - stop_service
  - read_service_logs
output_schema: null
---

# 调试本地服务

## 目标

启动配置的服务进程，记录 `ServiceHandle`（进程树），采集 stdout/stderr，验证就绪状态，记录调试时间窗口，在人工请求后采集日志。

## 适用场景

- 用户说 `harness debug-service` 或请求启动服务进行人工调试
- Orchestrator 将服务调试意图路由给 Runtime Debugger
- 开发者想在本地启动服务，并由 Harness 采集日志供后续分析

## 不适用场景

- 用户想运行自动化集成测试——应使用 `run-integration-tests`
- 目标端口上已有服务在运行

## 输入

- `runId`：唯一运行标识
- `harness.yaml` 配置：`service.executable`、`service.args`、`service.startupTimeoutSeconds`、`service.readiness`、`service.logFile`
- `stopService.mode`（必须为 `processTree`）

## 允许使用的工具

- `start_service`——启动配置的进程
- `stop_service`——仅停止 ServiceHandle 中记录的进程树
- `read_service_logs`——读取采集的 stdout/stderr 和应用日志

## 前置条件

- `harness.yaml` 中 `service.executable` 和 `service.args` 配置有效
- 配置的端口未被占用
- `stopService.mode` 为 `processTree`

## 执行步骤

1. **启动服务**：调用 `start_service(runId)`。执行配置的 `service.executable` 和 `service.args`。不经过 Shell 求值。将 stdout/stderr 采集到 `runs/<runId>/service.log`。
2. **记录 ServiceHandle**：获取返回的 `ServiceHandle`：
   ```json
   {
     "rootPid": 1234,
     "startedAt": "2026-08-04T10:00:00Z",
     "processGroup": 1234
   }
   ```
   保存此 handle——`stop_service` 需要它，且不得用于其他 run。
3. **验证就绪**：在采集的 stdout/stderr 中轮询配置的 `readiness.pattern`（如 `Started Application`）。最多等待 `startupTimeoutSeconds`。如果未找到匹配模式，分类为 `SERVICE_START_ERROR`。
4. **记录调试窗口**：记录 `windowStart`（确认就绪的时间）。窗口保持开放，直到用户发出完成信号或调用 `stop_service`。
5. **等待人工请求**：开发者或前端手动触发请求。Harness V1 **不发送**自动化 HTTP 请求。
6. **采集日志**：用户发出完成信号后（或出错时），调用 `read_service_logs(runId, windowStart, now)` 采集：
   - 从 `runs/<runId>/service.log` 采集的 stdout/stderr
   - 应用日志文件（如有配置 `service.logFile`）同一时间窗口内的内容
7. **停止服务**：调用 `stop_service(runId, serviceHandle)`。停止 `serviceHandle.processGroup` 标识的进程树。绝不停止其他进程。

## 输出

- `ServiceHandle`（rootPid、startedAt、processGroup）
- 就绪状态（ready 或 timeout）
- 调试窗口的日志包
- 如果启动失败：分类为 `SERVICE_START_ERROR` 的诊断结果

## 停止条件

- 启动超时 → 分类为 `SERVICE_START_ERROR`，尝试停止进程树，输出诊断结果
- 端口已被占用 → 报告后停止
- 用户请求停止 → 通过 `stop_service` 停止进程树

## 禁止行为

- 不得停止未记录在当前 ServiceHandle 中的任何进程
- 不得发送自动化 HTTP 请求
- 不得直接使用 `kill` 或系统命令——只能使用 `stop_service`
- 不得在服务已运行时再次启动
- 不得直接执行 Shell 命令——只能使用受控工具

## 示例

```
输入：runId=debug-20260804-001
命令：./mvnw spring-boot:run
ServiceHandle：{ rootPid: 5678, startedAt: "2026-08-04T10:05:00Z", processGroup: 5678 }
就绪状态：10:05:23 确认就绪（匹配模式 "Started Application"）
窗口：10:05:23 → 10:15:00（用户发出停止信号）
采集日志：service.log（235 行）、application.log（窗口内 89 行）
停止：processGroup=5678 的进程树已成功停止
```
