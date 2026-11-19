# T01 Windows 内网实机运行手册（待执行）

**只对公司的授权、隔离测试 Git 仓库操作。** 必须先在联网机获取并核验固定版本 npm tarballs 与完整 package-lock，制作离线 node_modules/缓存包。目标 Windows 禁止 npm 自动联网；本仓库没有自动安装器。

## 0. 准备

1. Windows x64，Node 22 LTS、Git for Windows、公司已有 OpenCode 2.x 服务/客户端与私有 Provider；无需 WSL、Docker、旧 Go Runtime。
2. `npm ci --offline --ignore-scripts --no-audit --no-fund` 仅在已有可信离线 npm 缓存时执行；若依赖缺失直接 FAIL，不打开公网获取。
3. 确认目标端口仅监听 127.0.0.1/::1，且原生服务身份与 TUI 公司构建一致；认证令牌只通过进程环境变量 `CODE_HARNESS_NATIVE_TOKEN` 提供。
4. 在原业务仓库之外，以**本地已有提交**生成独立 Git 测试副本。预先固定 base/head/merge-base，创建并验证本地 `harness-review-base` 指向冻结 merge-base；副本不能有 staged/unstaged/untracked 业务代码。
5. 基于公司构建的**真实 API 文档/抓包证据**制作不含密钥的 local JSON manifest（见下方结构）。不要自行猜测 Session、permission 或 form 路径。

```json
{
  "build": "<company build exact version>",
  "serviceIdentity": "<verified service identity>",
  "baseUrl": "http://127.0.0.1:<actual port>",
  "authHeader": "Authorization",
  "paths": {
    "identity": "/api/<verified native identity route>",
    "createSession": "/api/<verified native session create route>",
    "command": "/api/<verified native session command route with {sessionId}>"
  },
  "responseFields": {
    "build": "<verified JSON field>",
    "identity": "<verified JSON field>",
    "sessionId": "<verified JSON field>",
    "sessionLocation": "<verified JSON field>"
  },
  "requestFields": {
    "sessionLocation": "<verified JSON field>",
    "commandName": "<verified JSON field>",
    "commandText": "<verified JSON field>"
  }
}
```

> 以上尖括号全部是占位符，不是可以直接运行的公司 API；identity 的确切形态和认证必须先验证。manifest 放 `.local.json`，不要提交至 Git。

## 1. 编译与单测

在 `web-workbench/` 下执行：

```powershell
npm run gen:api
npm run typecheck
npm test
```

确认 DTO 生成无差异。随后执行 `scripts/windows/diagnose.ps1`，**脚本返回非零是能力未完成的正常阻断，不是通过**。

## 2. 真实 Session / Command

```powershell
$env:CODE_HARNESS_NATIVE_TOKEN = "<read securely, never commit>"
npm run probe -- --manifest "C:\\secure\\company.native.local.json"
npm run probe -- --manifest "C:\\secure\\company.native.local.json" --source "D:\\test\\source-repo" --snapshot "D:\\test\\task-copy" --expected-head "<frozen-head-oid>" --expected-merge-base "<frozen-merge-base-oid>" --run-review
```

`--run-review` 明确发起**一次**原生 command；脚本绝不自动重发、自动回复权限或宣称已完成。它返回非零并列出 UNKNOWN，这是防止 HTTP 2xx 冒充成功的保护。记录真实 Session ID（仅在受控内网证据）、确认 Agent/Command/Skill/模板来源 SHA256 与首个工具动作为原生 Skill 加载；确认私有模型使用的实际 alias/provider。

## 3. 原生权限、消息和中断

1. 通过公司已核对的原生 permission/form API 读取待处理请求，核对当前 Session、Run、nativeRequestId、工具文本；分别在独立测试轮次执行一次 ALLOW_ONCE 和 DENY。留下请求和执行行为证据，不修改全局权限。
2. 观察真实消息、事件、原生状态；测试同一 Session 的 prompt 追问，再在另一个执行中的输入上发送 interrupt，并检查实际状态与待处理请求是否清零。
3. 如果 instruction entry 缺失，禁用 notes；持久日志不可重放时标为 GAP。不要用额外 prompt 代替 instruction entry。

## 4. 报告和隔离

只在原生命令真正完成、且报告本轮实际产生之后，使用 `--verify-report --source "D:\\test\\source-repo" --before-fingerprint <首轮JSON的sourceBeforeFingerprint> --snapshot "D:\\test\\task-copy" --run-started-at <本轮毫秒时间戳> --manifest ...` 校验五章文件和 SHA256。校验脚本**不会**因存在旧报告就通过；报告校验必须附执行开始时间与上下文中的 native Session ID。

记录原仓库 HEAD、index 哈希和 `git status --porcelain=v1 -z` 前后结果，以及目标环境阻断外网的证据。避免将私有项目源码、完整命令输出、认证信息上传到公网 GitHub。

## 5. 审核

把逐项实测记录填入 `docs/capability-matrix.md`，包括 build、客户端锁定版本、API shape、请求/响应脱敏 trace ID、资源 digest、测试时间与结论。由独立审核方复核并给出 PASS / CONDITIONAL PASS / FAIL；全部 required 证据满足之前不允许开始依赖原生执行的 T02–T14 工作，也不宣称 Web 内网正式可用。

> 安全校验补充：真实 Review 之前运行只读 Git preflight，验证副本 Git common-dir 在副本内、仓库工作区干净、HEAD/本地 `harness-review-base`/merge-base OID 与指定值一致，拒绝 partial/promisor 仓库。保留首轮 JSON 的 `sourceBeforeFingerprint`，在原生命令实际终态后再比较原仓库指纹；未对账不能把 repository.immutable 标记为 SUPPORTED。
