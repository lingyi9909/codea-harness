/**
 * 机构 B→C API 与调度
 *
 * 职责：封装创建任务、并发领取、独立 OpenCode Session 和管理员接口。
 * Java 研发阅读提示：主线为 create → pump → runTask；数据库逻辑在 institution-task-store.ts。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
/**
 * 机构报文排查 API：HTTP 路由（Controller）+ 执行调度（Service）。
 *
 * 建议 Java 同学阅读顺序：
 *   1. registerInstitutionIntegration() —— Fastify 路由，类似 @RestController
 *   2. InstitutionIntegration.create()/pump() —— 任务入队与并发调度
 *   3. InstitutionIntegration.runTask() —— 一个任务调用 OpenCode 的生命周期
 *   4. institution-task-store.ts —— SQLite 仓储，类似 Repository/DAO
 *
 * 本文件没有 B 平台身份认证或 IP 白名单：X-Workbench-User-Id 只是 B 自报的标识。
 * 网络准入必须通过企业内网/网关实现，不能把这个请求头当成登录凭证。
 *
 * 每个任务都创建独立 OpenCode Session，对外只返回 taskId、状态和最终结论，
 * 不暴露 Session ID、内部推理、Skill 工具轨迹或数据库访问细节。
 */
import {createHash,randomUUID} from 'node:crypto';
import {homedir} from 'node:os';
import {isAbsolute,join} from 'node:path';
import {setTimeout as delay} from 'node:timers/promises';
import type {FastifyInstance,FastifyRequest} from 'fastify';
import {AuthProblem,type LocalAuth} from './auth.ts';
import {WorkspaceProfile} from './local-profile.ts';
import {type ExecutionRegistry} from './execution-registry.ts';
import {type SceneConfigStore} from './scene-config.ts';
import {type SceneAccessStore,applyToolPolicy} from './scene-access.ts';
import type {chatConnection} from './modules/chat/connection.ts';
import {InstitutionTaskStore,validConcurrency} from './institution-task-store.ts';
import type {InstitutionConcurrency,TaskRow,TaskStatus} from './institution-task-store.ts';

// 兼容旧代码的 import 路径，避免本轮阅读性重构影响现有测试或调用方。
export {InstitutionTaskStore,INSTITUTION_CONCURRENCY_OPTIONS} from './institution-task-store.ts';
export type {InstitutionConcurrency} from './institution-task-store.ts';

const API_ROOT='/api/v1/integrations/institution/investigations';
const TIMEOUT_MS=10*60*1000;
const MAX_CONSECUTIVE_POLL_ERRORS=5;
const FALLBACK_POLL_MS=6500;
const NO_EVENT_POLL_MS=1200;
// Event names are fixed; tool arguments, output, reasoning and SQL are NEVER persisted.
const DIAGNOSTIC_NATIVE_EVENTS:Record<string,{label:string;level:'INFO'|'WARN'|'ERROR';wake:boolean}>={
  'session.execution.started':{label:'NATIVE_EXECUTION_STARTED',level:'INFO',wake:false},
  'session.execution.succeeded':{label:'NATIVE_EXECUTION_SUCCEEDED',level:'INFO',wake:true},
  'session.execution.failed':{label:'NATIVE_EXECUTION_FAILED',level:'ERROR',wake:true},
  'session.execution.interrupted':{label:'NATIVE_EXECUTION_INTERRUPTED',level:'WARN',wake:true},
  'session.tool.called':{label:'NATIVE_TOOL_CALLED',level:'INFO',wake:false},
  'session.tool.success':{label:'NATIVE_TOOL_SUCCEEDED',level:'INFO',wake:false},
  'session.tool.failed':{label:'NATIVE_TOOL_FAILED',level:'WARN',wake:true},
  'permission.asked':{label:'NATIVE_PERMISSION_ASKED',level:'WARN',wake:true},
};
/** Only retry read-only native status calls. NEVER retry session.prompt(). */
export function transientInstitutionPollError(error:unknown){
  if(error instanceof AuthProblem){
    // The SDK wraps native HTTP 4xx as an AuthProblem(502). They are definite
    // request/authentication errors, not transient network transport failures.
    if(/^OPENCODE_HTTP_4\d\d$/.test(error.code))return false;
    return [502,503,504].includes(error.status);
  }
  if(error instanceof Error)return error.name==='AbortError'||error.name==='TimeoutError'||
    /fetch failed|network|ECONNRESET|ECONNREFUSED|ETIMEDOUT|socket hang up/i.test(error.message);
  return false;
}
const DIGEST=(s:string)=>createHash('sha256').update(s).digest('hex');
/** C 服务启动时读取的配置。对应 Java 中的 ConfigurationProperties/配置 DTO。 */
export type InstitutionIntegrationConfig={
  clientId:string;
  databasePath:string;
};
/**
 * 构造函数依赖（TypeScript 的依赖注入方式）：统一从 createApp() 传入。
 * 这里的 type/接口仅用于编译期类型检查，不会生成 Java 式的运行时接口对象。
 */
export type InstitutionIntegrationDeps={
  workspace:string;
  sceneConfig:SceneConfigStore;
  sceneAccess:SceneAccessStore;
  auth:LocalAuth;
  execution:ExecutionRegistry;
  nativeConnection:ReturnType<typeof chatConnection>;
};
const validId=(id:string)=>/^[A-Za-z0-9_.:-]{1,128}$/.test(id);
const taskIdPattern=/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const ADMIN_ROOT='/api/v1/institution-tasks';
const ADMIN_STATUSES=['QUEUED','RUNNING','SUCCEEDED','FAILED','UNKNOWN'] as const;
/** 将数据库实体转为 B API 的安全 DTO：刻意过滤 nativeSessionId、msgSn 和内部日志。 */
const status=(row:TaskRow)=>({
  taskId:row.id,status:row.status,requestId:row.request_id,
  createdAt:new Date(row.created_at).toISOString(),updatedAt:new Date(row.updated_at).toISOString(),
  result:row.status==='SUCCEEDED'&&row.result_text?{summary:row.result_text}:null,
  errorCode:row.status==='FAILED'||row.status==='UNKNOWN'?row.error_code:null,
});
/**
 * 读取 C 服务器部署配置。
 * 类比 Spring Boot 的 @ConfigurationProperties；此处仍沿用 process.env，
 * 不在重构期间改变环境变量、API 密钥策略或工作台的启动行为。
 */
export function institutionIntegrationConfig(env:NodeJS.ProcessEnv=process.env):InstitutionIntegrationConfig{
  // The two APIs are available by default in local single-service mode.
  // Neither a token nor an IP allowlist is required or consulted here.
  const clientId=env.HARNESS_INSTITUTION_API_CLIENT_ID??'platform-b';
  if(!validId(clientId))throw new Error('INSTITUTION_API_CLIENT_CONFIG_INVALID');
  const databasePath=env.HARNESS_INSTITUTION_API_DB_PATH??
    join(env.LOCALAPPDATA??homedir(),'.smart-workbench','state','institution-api.sqlite');
  if(!isAbsolute(databasePath)||databasePath.startsWith('\\\\')||databasePath.startsWith('//'))
    throw new Error('INSTITUTION_API_DB_PATH_INVALID');
  return {clientId,databasePath};
}

/**
 * 把业务日期转成上海时区的 [当天 00:00:00, 次日 00:00:00)。
 * 类比 Java 的 LocalDate -> ZonedDateTime -> Instant。
 * 右开区间避免跨天边界的精度和重复查询问题。
 */
export function institutionDayRange(day:string){
  if(!/^\d{4}-\d{2}-\d{2}$/.test(day))throw new AuthProblem(400,'INVALID_INSTITUTION_DATE');
  const from=Date.parse(day+'T00:00:00+08:00');
  if(!Number.isFinite(from)||new Date(from+8*3600000).toISOString().slice(0,10)!==day)
    throw new AuthProblem(400,'INVALID_INSTITUTION_DATE');
  return {from,to:from+86400000};
}
/**
 * 生成发给 OpenCode 的完整任务说明（不是返回给 B 的结果）。
 * 为什么包含 JSON 和毫秒时间戳：防止模型猜测日期范围，保留 msgSn 前导零。
 */
export function institutionPrompt(day:string,msgSn:string){
  const {from,to}=institutionDayRange(day);
  return [
    '机构报文排查：使用本场景绑定的 Skill，按以下条件查询并分析报文。',
    '这是一项完整的排查任务：连续定位业务、调用工具核验、分析证据并输出最终结论。',
    '不要停在某个中间阶段并要求继续。严禁编造工具查询证据。',
    '仅输出给业务人员的最终结论：事实、异常原因、证据及限制；不输出内部思考。',
    '不能继续执行、缺少必要输入或权限不足时，明确说明阻断原因，不要假装完成。',
    '业务时区：Asia/Shanghai (UTC+08:00)',
    '查询日期：'+day,
    '开始时间：'+day+' 00:00:00',
    '结束时间：'+day+' 23:59:59',
    '查询条件 JSON（含 from 不含 to，毫秒时间戳；msgSn 必须作为原始字符串处理）：',
    JSON.stringify({day,from,to,msgSn}),
  ].join('\n');
}
/**
 * 任务执行服务（类比 Java 的 @Service + 有界线程池 ExecutorService）。
 *
 * 区别：Node.js 单线程事件循环通过 async/await 并发等待 I/O，
 * 不是每个任务分配一个 Java Thread。
 *
 * 核心不变量：
 * - active.size <= 当前最大并发数（缩容期间可暂时超限，直到运行任务自然结束）
 * - 一个任务 -> 一个 Promise -> 一个 OpenCode Session
 * - 只在 SQLite take() 成功后执行；任务结束才释放名额
 * - 只有确认可安全失败时记 FAILED；调用是否已送达不确定时记 UNKNOWN
 */
export class InstitutionIntegration {
  /** 活跃任务的 Promise 集合，相当于 Java 线程池里正在执行的 Future。 */
  private readonly active=new Set<Promise<void>>();
  /** One shared native event subscription for all institution tasks (not one per task). */
  private eventStream?:Promise<void>;
  private readonly nativeTasks=new Map<string,string>();
  private readonly nativeWakeups=new Map<string,()=>void>();
  private readonly recentNativeTerminal=new Map<string,number>();
  private closed=false;
  /** AbortController ≈ Java 的取消信号：用于停止轮询等待，不直接撤销已执行的 SQL。 */
  private readonly stop=new AbortController();
  private readonly cfg:InstitutionIntegrationConfig;
  private readonly deps:InstitutionIntegrationDeps;
  private readonly tasks:InstitutionTaskStore;
  constructor(cfg:InstitutionIntegrationConfig,deps:InstitutionIntegrationDeps,tasks:InstitutionTaskStore){
    this.cfg=cfg;this.deps=deps;this.tasks=tasks;
    if(!deps.execution.singleService())throw new Error('INSTITUTION_API_REQUIRES_SINGLE_SERVICE_RUNTIME');
  }
  /**
   * 读取业务用户标识并生成稳定摘要，用于同一用户的任务隔离和幂等。
   * 这里没有校验用户真实身份，因此不应该理解成 Spring Security 的认证。
   */
  ownerFor(req:FastifyRequest){
    // User IDs are asserted by the caller, NOT verified by C. Only use the API
    // behind network-level access control and a B backend enforcing identity.
    const subject=req.headers['x-workbench-user-id'];
    if(typeof subject!=='string'||!validId(subject))throw new AuthProblem(400,'INTEGRATION_USER_REQUIRED');
    return DIGEST(JSON.stringify([this.cfg.clientId,subject]));
  }
  /**
   * API 创建任务：Repository.create() 先落库，再通知 pump() 消费。
   * 幂等重放不会重复创建 Session；调用方拿到 taskId 后即可异步轮询。
   */
  create(userHash:string,requestId:string,day:string,msgSn:string,externalUserId:string){
    const created=this.tasks.create(this.cfg.clientId,userHash,requestId,day,msgSn,externalUserId);
    if(created.created)this.pump();
    return {data:status(created.row),created:created.created};
  }
  /** B 平台按 taskId 查询本人任务（仅返回状态与最终结论）。 */
  get(userHash:string,id:string){
    const row=this.tasks.get(this.cfg.clientId,userHash,id);
    if(!row)throw new AuthProblem(404,'INVESTIGATION_NOT_FOUND');
    return status(row);
  }
  listAdmin(statusFilter:TaskStatus|undefined,query:string|undefined,offset:number,userHash?:string,limit=50){
    return this.tasks.listAdmin(statusFilter,query,offset,userHash,limit);
  }
  groupsAdmin(query:string|undefined,offset:number,limit=50){return this.tasks.groupsAdmin(query,offset,limit);}
  concurrencyStatus(){return {maxConcurrency:this.tasks.concurrency(),...this.tasks.queueCounts()};}
  /** 管理员修改并发档位；升容立即补位，缩容不取消已运行任务。 */
  setConcurrency(n:InstitutionConcurrency){
    this.tasks.setConcurrency(n);
    // Increase capacity immediately; reductions do not terminate running sessions.
    this.pump();
    return this.concurrencyStatus();
  }
  detailAdmin(id:string){return this.tasks.detailAdmin(id);}
  usageAdmin(){return {...this.tasks.usageAdmin(),currentPlatformLabel:this.cfg.clientId};}
  eventsAdmin(id:string,offset:number,limit=20){return this.tasks.eventsAdmin(id,offset,limit);}
  /** Diagnostics are best effort, never block or re-run the native task. */
  private note(taskId:string,eventType:string,level:'INFO'|'WARN'|'ERROR'='INFO',detail?:string,nativeId?:string){
    try{this.tasks.appendEvent(taskId,eventType,level,detail,nativeId);}
    catch{/* SQLite diagnostic logging must not change model execution semantics. */}
  }
  /**
   * Subscribe once to the OpenCode event feed and demultiplex by native session ID.
   * Events wake completion checks; a slower polling fallback covers stream gaps.
   * The feed never publishes raw prompts, private reasoning or tool contents.
   */
  private enableNativeEvents(client:Awaited<ReturnType<ReturnType<typeof chatConnection>>>){
    if(!client.event||typeof client.event.subscribe!=='function')return false;
    if(!this.eventStream){
      const running=this.consumeNativeEvents(client);
      this.eventStream=running.finally(()=>{this.eventStream=undefined;});
    }
    return true;
  }
  private async consumeNativeEvents(client:Awaited<ReturnType<ReturnType<typeof chatConnection>>>){
    while(!this.closed){
      try{
        for await(const event of client.event.subscribe({signal:this.stop.signal,onActivity:()=>{}})){
          if(this.closed)break;
          const data=event.data as unknown as Record<string,unknown>;
          const sessionId=typeof data.sessionID==='string'?data.sessionID:undefined;
          if(!sessionId)continue;
          const taskId=this.nativeTasks.get(sessionId);
          if(!taskId)continue;
          const kind=DIAGNOSTIC_NATIVE_EVENTS[event.type];
          if(!kind)continue;
          // Tool names are fixed identifiers; never copy args, SQL or model output.
          const toolName=event.type.startsWith('session.tool.')&&typeof data.name==='string'&&
            /^[A-Za-z0-9_.:-]{1,80}$/.test(data.name)?data.name:undefined;
          this.note(taskId,kind.label,kind.level,toolName,event.id);
          if(event.type==='session.execution.succeeded'||event.type==='session.execution.failed'||
             event.type==='session.execution.interrupted')
            this.recentNativeTerminal.set(sessionId,Date.now());
          if(kind.wake)this.nativeWakeups.get(sessionId)?.();
        }
      }catch{/* Subscription may disconnect: polling continues; reconnect this reader only. */}
      if(this.closed)break;
      for(const taskId of new Set(this.nativeTasks.values()))
        this.note(taskId,'NATIVE_EVENT_STREAM_RECONNECT','WARN');
      await delay(2000,undefined,{signal:this.stop.signal}).catch(()=>{});
    }
  }
  private async awaitNativeProgress(sessionId:string,usingEvents:boolean){
    if(!usingEvents){await delay(NO_EVENT_POLL_MS,undefined,{signal:this.stop.signal});return;}
    const controller=new AbortController();
    const wake=()=>controller.abort();
    this.nativeWakeups.set(sessionId,wake);
    try{
      // A terminal native event can precede the final persisted message by a
      // fraction of a second; briefly retry more frequently only in that window.
      const terminal=this.recentNativeTerminal.get(sessionId);
      const waitMs=terminal&&Date.now()-terminal<5000?1200:FALLBACK_POLL_MS;
      await delay(waitMs,undefined,{
        signal:AbortSignal.any([controller.signal,this.stop.signal]),
      });
    }catch(error){
      if(!controller.signal.aborted&&!this.stop.signal.aborted)throw error;
    }finally{
      if(this.nativeWakeups.get(sessionId)===wake)this.nativeWakeups.delete(sessionId);
    }
  }
  /**
   * 补足空闲执行名额：相当于线程池调度器。
   *
   * take() 是同步 SQLite 操作；每次只把一个 QUEUED 改成 RUNNING。
   * .catch() 给异常兜底，.finally() 回收名额并调度下一个任务。
   * 注意 JS 的 Promise 不等于“新建线程”：模型/DBHub 调用属于异步 I/O。
   */
  private pump(){
    if(this.closed)return;
    // Claims are synchronous and conditional in SQLite; each task has its own
    // Promise, execution context and native OpenCode Session.
    while(!this.closed&&this.active.size<this.tasks.concurrency()){
      const task=this.tasks.take();
      if(!task)break;
      // job 的 Promise 会一直跟踪此任务，直到数据库状态落为终态。
      let job:Promise<void>;
      job=this.runTask(task).catch(error=>{
        if(!this.closed)this.tasks.finish(task.id,'UNKNOWN',null,
          error instanceof AuthProblem?error.code:'INSTITUTION_WORKER_UNEXPECTED');
      }).finally(()=>{
        this.active.delete(job);
        if(!this.closed)this.pump();
      });
      this.active.add(job);
    }
  }
  /**
   * 单任务全生命周期：选 Skill/模型 -> 创建 Session -> 投递 Prompt ->
   * 等待模型执行 -> 读取最终消息 -> SQLite 落库。
   *
   * Java 对照：像 CompletableFuture 中执行远程 RPC，await 只暂停当前
   * async 函数，不阻塞事件循环，其余任务仍能继续访问 OpenCode。
   */
  private async runTask(task:TaskRow){
    if(this.closed)return;

    // accepted=true 代表请求可能已被原生服务受理：
    // 哪怕后续网络抖动，也不能确定它完全没执行。
    let accepted=false;
    let definiteFailure=false;
    let nativeId:string|undefined;
    let usingEvents=false;
    this.note(task.id,'NATIVE_CONNECTING');

    try{
      const result=await this.deps.execution.run(task.id,async()=>{
        // execution.run 设置每个任务自己的执行上下文（类似 Java ThreadLocal）。
        const client=await this.deps.nativeConnection();
        this.note(task.id,'NATIVE_CONNECTED');

        // 1. 从工作台配置确定 Skill/工作目录，拒绝使用调用者自定义的执行目标。
        const binding=await this.deps.sceneConfig.binding('institution-message');
        if(binding.executor.type!=='skill'){
          throw new AuthProblem(409,'INSTITUTION_SKILL_NOT_CONFIGURED');
        }
        const catalog=await client.skill.list({location:{directory:binding.directory}});
        if(catalog.location?.directory!==binding.directory||!Array.isArray(catalog.data)){
          throw new AuthProblem(502,'INSTITUTION_SKILL_CATALOG_UNAVAILABLE');
        }
        const matches=catalog.data.filter(skill=>skill.name===binding.executor.name&&Boolean(skill.id));
        if(matches.length!==1)throw new AuthProblem(409,'INSTITUTION_SKILL_NOT_FOUND');
        this.note(task.id,'SKILL_READY');

        // 2. 在可用模型中选择 Kimi（也允许管理员通过环境变量指定）。
        const models=await client.model.list({location:{directory:binding.directory}});
        const enabled=models.data.filter(m=>m.enabled);
        const preferred=process.env.HARNESS_INSTITUTION_API_MODEL_ID?.trim();
        const model=preferred?enabled.find(m=>m.id===preferred):
          enabled.find(m=>m.id.toLowerCase()==='kimi'||m.name.toLowerCase()==='kimi')??
          enabled.find(m=>(m.id+' '+m.name).toLowerCase().includes('kimi'))??
          (enabled.length===1?enabled[0]:undefined);
        if(!model)throw new AuthProblem(409,'INSTITUTION_MODEL_NOT_CONFIGURED');
        this.note(task.id,'MODEL_READY');

        // 3. 权限规则在 C 侧决定。每个任务都有独立的 OpenCode Session。
        const permissions=applyToolPolicy(
          this.deps.execution.permissions(),
          this.deps.sceneAccess.policy('institution-message'),
        );
        const session=await client.session.create({
          location:{directory:binding.directory},
          title:'机构报文排查 API',
          model:{providerID:model.providerID,id:model.id},
          metadata:{
            harnessScenario:'institution-message-inspection',
            harnessOwner:task.id,
            harnessIntegration:'platform-b',
          },
          ...(permissions?{permissions}:{}),
        });

        // 校验原生服务没有悄悄修改 Session 的目录或权限。
        if(!session?.id||session.location?.directory!==binding.directory||
          (permissions&&JSON.stringify(session.permissions)!==JSON.stringify(permissions))){
          if(session?.id)await client.session.remove({sessionID:session.id}).catch(()=>{});
          throw new AuthProblem(409,'INSTITUTION_SESSION_MISMATCH');
        }
        if(this.closed){
          await client.session.remove({sessionID:session.id}).catch(()=>{});
          throw new Error('SERVER_STOPPED');
        }
        this.tasks.session(task.id,session.id);
        nativeId=session.id;
        this.nativeTasks.set(session.id,task.id);
        usingEvents=this.enableNativeEvents(client);
        this.note(task.id,'NATIVE_SESSION_CREATED');

        // 4. 先标记「可能已受理」，再投递 Prompt；不确定的失败必须标 UNKNOWN。
        // delivery:'queue' 是 OpenCode Session 内的投递方式，
        // 和 C 层 SQLite 的 QUEUED 任务队列不是同一个概念。
        accepted=true;
        try{
          await client.session.prompt({
            sessionID:session.id,
            text:institutionPrompt(task.day,task.msg_sn),
            skills:[{id:matches[0].id}],
            delivery:'queue',
          });
        }catch(error){
          // An explicit HTTP 4xx from native service is a definite rejection,
          // unlike a transport timeout where Prompt may have arrived.
          if(error instanceof AuthProblem&&
            /^OPENCODE_HTTP_(?:400|401|403|404|409|422)$/.test(error.code))
            definiteFailure=true;
          throw error;
        }

        this.note(task.id,'PROMPT_ACCEPTED');
        // 5. Prefer native events and use low-frequency status polling as a safety net.
        const end=Date.now()+TIMEOUT_MS;
        const readSnapshot=()=>Promise.all([
          client.session.active(),
          client.session.inbox.list({sessionID:session.id}),
          client.message.list({sessionID:session.id,order:'desc',limit:100}),
          client.permission.list({sessionID:session.id}),
        ]);
        let consecutivePollErrors=0;
        while(!this.closed&&Date.now()<end){
          // Polling four read-only native APIs in parallel is particularly
          // vulnerable to temporary 25-second HTTP timeouts under load.
          // A failed *read* does not mean the running model session failed.
          let snapshot:Awaited<ReturnType<typeof readSnapshot>>;
          try{
            snapshot=await readSnapshot();
            if(consecutivePollErrors>0)this.note(task.id,'STATUS_READ_RECOVERED');
            consecutivePollErrors=0;
          }catch(error){
            if(!transientInstitutionPollError(error))throw error;
            consecutivePollErrors++;
            this.note(task.id,'STATUS_READ_RETRY','WARN',String(consecutivePollErrors));
            if(consecutivePollErrors>MAX_CONSECUTIVE_POLL_ERRORS||Date.now()>=end)
              throw new AuthProblem(503,'INSTITUTION_POLL_UNAVAILABLE');
            // Do not re-send Prompt: the native model may still be running.
            await delay(Math.min(consecutivePollErrors,3)*1200,undefined,{signal:this.stop.signal});
            continue;
          }
          const [active,inbox,messages,permissionsPending]=snapshot;
          if(permissionsPending.some(p=>p.sessionID===session.id)){
            this.note(task.id,'PERMISSION_REQUIRED','ERROR');
            await client.session.interrupt({sessionID:session.id});
            definiteFailure=true;
            throw new AuthProblem(409,'INSTITUTION_PERMISSION_REQUIRED');
          }

          // 只收最终完成的 assistant 文本，过滤工具调用过程和中间消息。
          // m is Extract<...> 是 TS 类型守卫，不是 Java 风格的运行时强制转换。
          const final=messages.data.find((m):m is Extract<(typeof messages.data)[number],{type:'assistant'}>=>
            m.type==='assistant'&&Boolean(m.time?.completed)&&
            (Boolean(m.error)||(Boolean(m.finish)&&m.finish!=='tool-calls')));
          // A completed native assistant error is definitive even if the
          // active-session indicator has not caught up yet.
          if(final?.error||final?.finish==='error'){
            definiteFailure=true;
            throw new AuthProblem(502,'INSTITUTION_NATIVE_EXECUTION_FAILED');
          }
          if(final&&!active[session.id]&&inbox.length===0){
            const answer=final.content
              .flatMap(part=>part.type==='text'?[part.text]:[])
              .join('\n')
              .trim();
            if(!answer){definiteFailure=true;throw new AuthProblem(502,'INSTITUTION_EMPTY_RESULT');}
            this.note(task.id,'FINAL_RESULT_RECEIVED');
            return answer.slice(0,65536);
          }
          await this.awaitNativeProgress(session.id,usingEvents);
        }
        if(!this.closed)await client.session.interrupt({sessionID:session.id}).catch(()=>{});
        this.note(task.id,'EXECUTION_TIMEOUT','WARN');
        throw new AuthProblem(504,'INSTITUTION_EXECUTION_TIMEOUT');
      });

      // 6. 仅在 C 仍运行时写入最终结论；已关闭的服务统一进入 UNKNOWN。
      if(!this.closed)this.tasks.finish(task.id,'SUCCEEDED',result,null);
    }catch(error){
      if(!this.closed){
        const code=error instanceof AuthProblem?error.code:'INSTITUTION_EXECUTION_UNAVAILABLE';
        // 在 Prompt 投递后出现未知网络错误，不能断言任务没执行 -> UNKNOWN。
        this.tasks.finish(task.id,accepted&&!definiteFailure?'UNKNOWN':'FAILED',null,code);
      }
    }finally{
      if(nativeId){
        this.nativeTasks.delete(nativeId);
        this.nativeWakeups.get(nativeId)?.();
        this.nativeWakeups.delete(nativeId);
        this.recentNativeTerminal.delete(nativeId);
      }
    }
  }
  /** 关闭 C：停止继续领取任务，向轮询发送取消信号，并标记未完成状态。 */
  close(){
    this.closed=true;this.stop.abort();
    this.nativeTasks.clear();
    this.nativeWakeups.clear();
    this.recentNativeTerminal.clear();
    this.tasks.stopUnfinished();
    this.tasks.close();
  }
}

/**
 * 将机构 API 路由注册到 Fastify（类似 Spring 的 @RestController）。
 *
 * 注意这里有两组完全不同的路由：
 *  - B 平台：POST 创建 / GET 查单笔，不提供应用层密钥或 IP 白名单
 *  - C 本机管理员：只读统计、聚合、配置并发；需本机会话+editors 权限
 */
export function registerInstitutionIntegration(app:FastifyInstance,
  deps:InstitutionIntegrationDeps,cfg:InstitutionIntegrationConfig){
  // ??= 是「若左边为 null/undefined 才赋值」，类似 Java 的懒加载单例。
  // 只有真的访问机构 API 才打开 SQLite，避免无关页面启动时创建数据文件。
  let manager:InstitutionIntegration|undefined;
  const getManager=()=>manager??=new InstitutionIntegration(cfg,deps,
    new InstitutionTaskStore(cfg.databasePath,deps.workspace));
  app.addHook('onClose',()=>manager?.close());
  // 管理接口共用 /config 编辑权限：真实回环地址 + 登录会话 + editors 名单。
  // PATCH 还要求 CSRF（防止从其他网站伪造管理员的更改请求）。
  // B 自报的 X-Workbench-User-Id 绝不能用于判定管理员身份。
  const localProfile=new WorkspaceProfile(deps.workspace);
  const authorizeAdmin=(req:FastifyRequest,mutating=false)=>{
    if(!deps.auth.isLocal(req)||!deps.auth.origin(req))
      throw new AuthProblem(403,'CONFIG_LOCAL_ONLY');
    const session=mutating?deps.auth.mutation(req):deps.auth.session(req);
    if(session.scope==='lan'||!session.profile||!localProfile.matches(session.profile.key))
      throw new AuthProblem(401,'PROFILE_LOGIN_REQUIRED');
    if(!deps.sceneConfig.canEdit(session.profile.name))
      throw new AuthProblem(403,'SCENE_CONFIG_EDIT_FORBIDDEN');
  };
  // -------- C 本机管理员接口（不可让 B 或普通 LAN 用户直接访问）--------
  app.get('/api/v1/institution-access',async(req,reply)=>{
    authorizeAdmin(req);
    return reply.header('Cache-Control','no-store')
      .send({data:getManager().usageAdmin(),requestId:randomUUID()});
  });
  app.get('/api/v1/institution-concurrency',async(req,reply)=>{
    authorizeAdmin(req);
    return reply.header('Cache-Control','no-store')
      .send({data:getManager().concurrencyStatus(),requestId:randomUUID()});
  });
  // Fastify 的 <{Body:...}> 相当于声明 Request DTO，schema 是运行时 JSON 校验。
  app.patch<{Body:{maxConcurrency:number}}>('/api/v1/institution-concurrency',{
    schema:{body:{type:'object',required:['maxConcurrency'],additionalProperties:false,
      properties:{maxConcurrency:{type:'integer',enum:[3,5,10,15]}}}},
  },async(req,reply)=>{
    authorizeAdmin(req,true);
    if(!validConcurrency(req.body.maxConcurrency))
      throw new AuthProblem(400,'INSTITUTION_CONCURRENCY_INVALID');
    return reply.header('Cache-Control','no-store')
      .send({data:getManager().setConcurrency(req.body.maxConcurrency),requestId:randomUUID()});
  });
  const parseAdminOffset=(value:string)=>{
    if(!/^[0-9]{1,7}$/.test(String(value))||Number(value)>1000000)
      throw new AuthProblem(400,'INSTITUTION_TASK_FILTER_INVALID');
    return Number(value);
  };
  const parseAdminLimit=(value:string)=>{
    if(!['20','50','100'].includes(value))throw new AuthProblem(400,'INSTITUTION_TASK_FILTER_INVALID');
    return Number(value);
  };
  app.get<{Querystring:{q?:string;offset?:string;limit?:string}}>(ADMIN_ROOT+'/users',async(req,reply)=>{
    authorizeAdmin(req);
    const {q='',offset='0',limit='50'}=req.query;
    if(q&&!validId(q))throw new AuthProblem(400,'INSTITUTION_TASK_FILTER_INVALID');
    return reply.header('Cache-Control','no-store')
      .send({data:getManager().groupsAdmin(q||undefined,parseAdminOffset(offset),parseAdminLimit(limit)),requestId:randomUUID()});
  });
  app.get<{Querystring:{status?:string;q?:string;offset?:string;limit?:string;userHash?:string}}>(ADMIN_ROOT,async(req,reply)=>{
    authorizeAdmin(req);
    const {status:state='ALL',q='',offset='0',limit='50',userHash}=req.query;
    if(state!=='ALL'&&!ADMIN_STATUSES.includes(state as TaskStatus))
      throw new AuthProblem(400,'INSTITUTION_TASK_FILTER_INVALID');
    if(q&&!validId(q)||userHash&&!/^[a-f0-9]{64}$/.test(userHash))
      throw new AuthProblem(400,'INSTITUTION_TASK_FILTER_INVALID');
    return reply.header('Cache-Control','no-store')
      .send({data:getManager().listAdmin(state==='ALL'?undefined:state as TaskStatus,q||undefined,
        parseAdminOffset(offset),userHash,parseAdminLimit(limit)),requestId:randomUUID()});
  });
  // Logs are admin-only, paginated and never exposed by platform B's GET.
  app.get<{Params:{id:string};Querystring:{offset?:string;limit?:string}}>(ADMIN_ROOT+'/:id/logs',async(req,reply)=>{
    authorizeAdmin(req);
    if(!taskIdPattern.test(req.params.id))throw new AuthProblem(404,'INVESTIGATION_NOT_FOUND');
    const {offset='0',limit='20'}=req.query;
    return reply.header('Cache-Control','no-store')
      .send({data:getManager().eventsAdmin(req.params.id,parseAdminOffset(offset),parseAdminLimit(limit)),requestId:randomUUID()});
  });
  app.get<{Params:{id:string}}>(ADMIN_ROOT+'/:id',async(req,reply)=>{
    authorizeAdmin(req);
    if(!taskIdPattern.test(req.params.id))throw new AuthProblem(404,'INVESTIGATION_NOT_FOUND');
    return reply.header('Cache-Control','no-store')
      .send({data:getManager().detailAdmin(req.params.id),requestId:randomUUID()});
  });
  // -------- B 平台的外部接口：保持原有 POST/GET 协议和返回结构 --------
  app.post<{Body:{date:string;msgSn:string;requestId:string}}>(API_ROOT,{
    schema:{body:{type:'object',additionalProperties:false,required:['date','msgSn','requestId'],properties:{
      date:{type:'string',minLength:10,maxLength:10},msgSn:{type:'string',minLength:1,maxLength:256},
      requestId:{type:'string',minLength:1,maxLength:128},
    }}},
  },async(req,reply)=>{
    const service=getManager();
    const user=service.ownerFor(req);
    const {date,msgSn,requestId}=req.body;
    institutionDayRange(date);
    if(typeof msgSn!=='string'||!/^[A-Za-z0-9_.:-]{1,256}$/.test(msgSn)||
      typeof requestId!=='string'||!validId(requestId))
      throw new AuthProblem(400,'INSTITUTION_PARAMETERS_INVALID');
    const result=service.create(user,requestId,date,msgSn,req.headers['x-workbench-user-id'] as string);
    return reply.code(result.created?202:200).header('Cache-Control','no-store')
      .send({data:result.data,requestId:randomUUID()});
  });
  app.get<{Params:{id:string}}>(API_ROOT+'/:id',async(req,reply)=>{
    const service=getManager();
    const user=service.ownerFor(req);
    const id=req.params.id;
    if(!taskIdPattern.test(id))
      throw new AuthProblem(404,'INVESTIGATION_NOT_FOUND');
    return reply.header('Cache-Control','no-store')
      .send({data:service.get(user,id),requestId:randomUUID()});
  });
}
