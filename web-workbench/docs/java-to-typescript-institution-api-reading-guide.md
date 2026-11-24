# Java 研发阅读智能工作台 TypeScript 代码指南

> 面向熟悉 Java 17、Spring Boot、MyBatis/JDBC 的研发同学。阅读对象为 PR #116 的机构报文排查 B→C API、并发调度、任务管理。本文是**源码导读**，不是替代现有 API 或部署文档。
>
> **先看目录，再按调用链读代码**。不要一开始就逐行读 400 行 TSX：先理解数据在哪里、谁负责什么，最后再看 React 页面怎样绑定接口。

## 一、先建立 Java MVC 对照

| 本项目代码 | Java/Spring 中的直观对照 | 职责 |
| --- | --- | --- |
| `apps/server/src/institution-integration.ts` 的 `registerInstitutionIntegration()` | `@RestController` / `@RequestMapping` | 创建 HTTP 路由、校验请求、区分管理员与 B 接口 |
| 同文件的 `InstitutionIntegration` | `@Service` + `ExecutorService` | 创建任务、控制并发、调用 OpenCode、保存结果 |
| `apps/server/src/institution-task-store.ts` | `@Repository` / `Mapper` / JDBC | SQLite 建表、幂等、状态迁移、按用户统计 |
| `apps/web/src/api/client.ts` | `RestTemplate` / Feign Client + DTO | 浏览器封装 GET/PATCH 请求、携带登录 Cookie/CSRF |
| `apps/web/src/InstitutionTaskManagementPage.tsx` | MVC 页面 Controller + View | 登录态、筛选、自动刷新、任务列表和详情 |
| `apps/web/src/InstitutionTaskOperations.tsx` | 管理页面中的两个子组件 | 3/5/10/15 并发设置、按业务用户聚合钻取 |
| `tests/api/institution-integration.test.ts` | Spring MVC/Test + MockMvc | B 双接口和本机管理员访问边界 |
| `tests/api/institution-concurrency.test.ts` | 单元/集成测试 | 三路/五路并发、任务状态、聚合、CSRF、持久化 |

代码目录入口（从 `web-workbench/` 出发）：

```text
apps/
├── server/src/
│   ├── app.ts                         # 注册整个 Web 应用的路由
│   ├── institution-integration.ts     # Controller + Service
│   ├── institution-task-store.ts      # Repository（本次整理后拆出）
│   ├── auth.ts                        # 本地会话与 CSRF
│   ├── scene-config.ts                # Skill 与工作目录绑定
│   └── execution-registry.ts          # OpenCode 执行上下文
└── web/src/
    ├── InstitutionTaskManagementPage.tsx
    ├── InstitutionTaskOperations.tsx
    └── api/client.ts
```

## 二、走一遍完整请求（先理解功能，再学习语法）

```text
平台 B（后端）
   │ POST /api/v1/integrations/institution/investigations
   │ X-Workbench-User-Id: customer-001
   ▼
Fastify 路由（registerInstitutionIntegration）
   │ 检查 date / msgSn / requestId
   ▼
InstitutionIntegration.create()
   │
   ▼
InstitutionTaskStore.create()
   │ INSERT OR IGNORE + 唯一键 (client_id, user_hash, request_id)
   │ 相同请求号 + 相同参数 => 直接返回旧 taskId（幂等）
   ▼
InstitutionIntegration.pump()
   │ running < maxConcurrency ? take() : 留在 QUEUED
   ▼
InstitutionTaskStore.take()
   │ QUEUED → RUNNING，使用带状态条件的 UPDATE
   ▼
InstitutionIntegration.runTask()
   │ 查询目录绑定 Skill / 选择 Kimi 模型 / 合并工具权限
   │ 为每项任务新建 OpenCode Session
   │ 投递 Prompt，轮询原生完成消息
   ▼
InstitutionTaskStore.finish()
   │ SUCCEEDED + result.summary
   │ 或 FAILED / UNKNOWN + 错误信息
   ▼
平台 B：GET /api/v1/integrations/institution/investigations/{taskId}
   │ 返回单任务状态 + 最终结论
   ▼
C 本机管理员：接口任务管理 / 按业务用户聚合
```

这里有**两个不同的排队层次**，不要混淆：

- C 自己的 SQLite 队列：`QUEUED` / `RUNNING` 决定最多同时执行几个机构任务。
- OpenCode 请求里的 `delivery:'queue'`：表示该请求如何投递到**单个 OpenCode Session**，并不意味着所有 C 任务只能串行。

## 三、TypeScript 语法速查：从熟悉的 Java 概念类比

### 1. `type` 是编译期的类型，不是 Java 类

```ts
type TaskStatus = 'QUEUED' | 'RUNNING' | 'SUCCEEDED' | 'FAILED' | 'UNKNOWN';

type TaskRow = {
  id: string;
  status: TaskStatus;
  result_text: string | null;
};
```

Java 的直观类比是 `enum TaskStatus` + `TaskRow DTO`。区别：`type` 在编译后消失，**不会自动创建运行时对象，也不会替你验证数据库或 HTTP JSON**。

### 2. `?:`、`| null` 与 `??`

```ts
function query(userHash?: string) {
  return userHash ?? '未指定';
}
```

- `userHash?: string`：可以省略此参数，值可能是 `undefined`。
- `string | null`：字段可能显式为 `null`。
- `value ?? fallback`：仅当左侧为 `null` 或 `undefined` 才使用默认值；不同于 `||`，不会把 `''` 或 `0` 视为缺省。

### 3. `as const` 和字面量联合类型

```ts
const options = [3, 5, 10, 15] as const;
type Concurrency = typeof options[number]; // 3 | 5 | 10 | 15
```

这可以类比为只允许四个枚举值。后端仍通过 `validConcurrency()` 和 Fastify schema 进行**运行时校验**，不能只依赖 TS 编译器。

### 4. `Promise<T>`、`async`、`await`

```ts
async function run(): Promise<string> {
  const answer = await callRemoteModel();
  return answer;
}
```

类比 Java 的 `CompletableFuture<String>`，但 Node.js 的 `await` **不会为每个任务创建一个新线程**。它挂起当前异步函数，把事件循环留给其他请求/任务。模型查询、DBHub 查询主要属于 I/O 等待；所以多个任务可以有多个 Promise 和 Session 同时在途。

### 5. `Promise.all()` 与 Java `allOf()`

```ts
const [active, inbox, messages, permissions] = await Promise.all([
  client.session.active(),
  client.session.inbox.list({sessionID}),
  client.message.list({sessionID}),
  client.permission.list({sessionID}),
]);
```

`[a,b,c]` 是数组解构：按位置取出四个 Promise 的结果。可以理解成 Java 多项异步 RPC 全部完成后聚合结果。

### 6. `?.`、`map()`、`filter()`、`find()`

```ts
const available = models.data.filter(m => m.enabled);
const kimi = available.find(m => m.id === 'kimi');
const id = kimi?.id;
```

`filter` / `map` / `find` 近似 Java Stream API；`?.` 是可选链，避免空值时访问属性抛异常，但最后得到的值可能是 `undefined`。

### 7. `import type`、`export` 与依赖

```ts
import {InstitutionTaskStore} from './institution-task-store.ts';
import type {TaskRow} from './institution-task-store.ts';
export {InstitutionTaskStore} from './institution-task-store.ts';
```

- `import type`：只导入编译期类型，运行时不加载该类型对象。
- `export`：模块公开可导出的函数、类或类型，概念上类似 Java 的 `public`。
- 保留从 `institution-integration.ts` 转发 `InstitutionTaskStore`，是为了兼容此前已有测试的导入路径，属于**阅读性重构不破坏公共接口**。

## 四、数据库 Repository 应重点读哪些方法

### 1. `constructor()` —— 建表和旧数据兼容

先看 `CREATE TABLE`、`ALTER TABLE` 和 `UPDATE ... status='UNKNOWN'` 三块。

特别注意：进程崩溃后，模型或 DBHub 可能已经执行过操作，因此不能简单重置成 `QUEUED` 再执行一次。这里保留了原设计：重启时将未完成任务标为 `UNKNOWN`，由管理员核验。

### 2. `create()` —— 为什么说它是幂等的

唯一约束为 `(client_id,user_hash,request_id)`。同一个业务用户重复提交同一个 requestId：

- 如果 `date,msgSn` 的哈希与原任务一致：返回老 taskId，不创建新任务。
- 如果参数不一致：报 `REQUEST_ID_PAYLOAD_CONFLICT`。
- `X-Workbench-User-Id` 是业务用户标签，不是可信认证；其摘要只用于任务匹配与分组。

### 3. `take()` —— 谁负责并发领取

```sql
SELECT * FROM institution_api_tasks
 WHERE status='QUEUED'
 ORDER BY created_at,id
 LIMIT 1;

UPDATE institution_api_tasks
   SET status='RUNNING',updated_at=?
 WHERE id=? AND status='QUEUED';
```

这里的 `WHERE status='QUEUED'` 与 `changes===1` 相当于 Java 开发里常见的**带状态条件更新**。它是单 C 进程调度的一部分，**不代表已实现跨多个 C 实例共享队列**。

### 4. `finish()` —— 结果与状态保护

只有 RUNNING 的任务允许写入 SUCCEEDED / FAILED / UNKNOWN。最终结论只给成功任务返回，内部 Session ID 和工具日志不会出现在 B 接口。

### 5. `groupsAdmin()` —— 用户聚合

按 `client_id,user_hash` 分组，使用 SQL 的 `COUNT()`、`SUM(CASE WHEN ...)` 统计运行中/排队中/成功/失败/未知，再由前端绘制成表格。

历史记录可能只有不可逆用户哈希，无法恢复原始 `customer-001`。

## 五、理解 3 / 5 / 10 / 15 并发：优先读 `pump()`

```ts
while (!this.closed && this.active.size < this.tasks.concurrency()) {
  const task = this.tasks.take();
  if (!task) break;

  let job: Promise<void>;
  job = this.runTask(task)
    .catch(/* 未预期错误兜底 */)
    .finally(() => {
      this.active.delete(job);
      if (!this.closed) this.pump();
    });

  this.active.add(job);
}
```

这个调度器相当于**有界执行池**：

1. `active` 是正在运行的 Promise 集合，类似保存 `Future` 的集合。
2. `concurrency()` 从 SQLite 设置表读取最大并发档位。
3. 只要有名额，就在循环中继续 `take()` 下一项任务。
4. 任务结束后，在 `.finally()` 中释放名额，再尝试补位。
5. 从 5 降到 3 时不打断已有的 5 个 Session；等运行中任务自然减少到低于 3，才启动新任务。
6. 每个 `runTask()` 的工具权限和 OpenCode Session 独立，但**内网模型、DBHub 和业务数据库是否支持 15 路并发，需要真实压测**。

**不要把这个 Promise 池误认为支持多服务器部署**：本设计是单个 C 服务进程内部调度；多个进程共享同一个 SQLite 文件不是现阶段的交付能力。

## 六、React 页面怎么读

页面文件分开读，比阅读一整段 JSX 容易：

1. `InstitutionTaskManagementPage.tsx`：
   - `mode`：加载、登录、权限不足、正常展示等页面状态。
   - `filter / appliedQuery / offset`：后端列表查询参数。
   - `refresh`：每隔 5 秒增加一次，触发重新读取。
   - `selectedId / detail`：选中的 taskId 和该任务的最终结果。
2. `InstitutionTaskOperations.tsx`：
   - `InstitutionConcurrencyControl`：读取当前上限、下拉选择、点击保存后 PATCH。
   - `InstitutionUserGroupsView`：三层数据：用户组 → 该用户任务 → 单个任务结果。
3. `api/client.ts`：仅负责把页面操作转成 HTTP，不负责授权，也不会直接调用模型。

```tsx
const [refresh, setRefresh] = useState(0);

useEffect(() => {
  // refresh 改变时执行这里；这是查询，不是提交任务。
  api.institutionConcurrency().then(setStatus);
}, [refresh]);
```

注意 `useState` 更新会触发组件重新渲染；`useEffect` 可以在依赖变化时执行异步查询。页面隐藏时暂停定时拉取是为了降低无意义的请求数；后台任务仍然继续执行。

### 为什么不能只在前端隐藏管理员按钮？

这是很重要的安全边界：**React 的任何隐藏/禁用都不等于授权**。C 服务每个管理员接口仍检查：

- 是否从 C 服务器本机真实回环地址访问；
- 是否有工作台有效本机会话；
- 登录用户名是否在 `.workbench/config-access.json` 的 `editors` 中；
- 修改并发设置时，还必须通过 CSRF 检查。

B 的 POST/GET 两个公开业务接口与管理员查询接口必须分开理解。

## 七、建议阅读和练习顺序

**第 1 遍：只看主干。** 看 `registerInstitutionIntegration()`、`create()`、`pump()`、`runTask()`、`finish()` 的调用链。先不用读 React。

**第 2 遍：看数据状态。** 画出 QUEUED → RUNNING → SUCCEEDED/FAILED/UNKNOWN，回答“为什么不能自动重试 UNKNOWN”。

**第 3 遍：看并发。** 解释 `active.size` 为什么限制正在运行的任务数，`Promise.all()` 与 `Promise<void>` 各做什么，缩容为何不立即中止原任务。

**第 4 遍：看页面。** 从用户聚合按钮的 `onClick` 追踪到 `useState`、`useEffect`、`api.institutionTasks()`、后端 `groupsAdmin()`。

**第 5 遍：运行自动化测试。** 在已有的 Node 和依赖环境下：

```bash
cd web-workbench
npm run typecheck
npm run web:typecheck
npm run test:institution-api
npm run web:build
```

新手练习建议：先修改一个**不影响业务的显示文案或辅助注释**，观察 TS 编译和浏览器页面变化；然后在现有测试里增加“同用户幂等提交”“不同用户聚合隔离”断言。

## 八、这次代码整理遵循的边界

- 保留原有 B 平台双接口路径、请求头、响应体与状态含义。
- 不更改模型选择、Skill 绑定、DBHub 权限、并发档位和管理权限机制。
- SQLite 文件和原有业务任务继续使用相同表和字段；只将原代码放进更清晰的 Repository 文件。
- 不改其他场景（公管排查、报文排查、代码审查、数据分析）。
- 阅读文档仅作教学；**实际接口契约以** [机构 B2C API V1 文档](institution-b2b-api-v1.md) **为准**。
