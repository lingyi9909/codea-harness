/**
 * Server-to-server institution investigation API for trusted platform B.
 *
 * API routes are available for the local single-service workbench by default.
 * The B-to-C endpoints do not perform application authentication or IP allowlisting;
 * network access MUST be controlled outside this application.
 *
 * One OpenCode session per task; never expose native session IDs, directories,
 * tools, transcripts, reasoning or secrets to the external caller.
 */
import {DatabaseSync} from 'node:sqlite';
import {createHash,randomUUID} from 'node:crypto';
import {existsSync,lstatSync,mkdirSync,chmodSync} from 'node:fs';
import {homedir} from 'node:os';
import {dirname,isAbsolute,join,relative,sep} from 'node:path';
import {setTimeout as delay} from 'node:timers/promises';
import type {FastifyInstance,FastifyRequest} from 'fastify';
import {AuthProblem,type LocalAuth} from './auth.ts';
import {WorkspaceProfile} from './local-profile.ts';
import {type ExecutionRegistry} from './execution-registry.ts';
import {type SceneConfigStore} from './scene-config.ts';
import {type SceneAccessStore,applyToolPolicy} from './scene-access.ts';
import type {chatConnection} from './modules/chat/connection.ts';

const API_ROOT='/api/v1/integrations/institution/investigations';
const RETENTION_MS=7*24*60*60*1000;
const TIMEOUT_MS=10*60*1000;
const DIGEST=(s:string)=>createHash('sha256').update(s).digest('hex');
type TaskStatus='QUEUED'|'RUNNING'|'SUCCEEDED'|'FAILED'|'UNKNOWN';
type TaskRow={
  id:string;client_id:string;user_hash:string;request_id:string;payload_hash:string;
  day:string;msg_sn:string;external_user_id:string|null;status:TaskStatus;session_id:string|null;
  result_text:string|null;error_code:string|null;created_at:number;updated_at:number;
};
export type InstitutionIntegrationConfig={
  clientId:string;databasePath:string;
};
export type InstitutionIntegrationDeps={
  workspace:string;sceneConfig:SceneConfigStore;sceneAccess:SceneAccessStore;auth:LocalAuth;
  execution:ExecutionRegistry;nativeConnection:ReturnType<typeof chatConnection>;
};
const validId=(id:string)=>/^[A-Za-z0-9_.:-]{1,128}$/.test(id);
const taskIdPattern=/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const ADMIN_ROOT='/api/v1/institution-tasks';
const ADMIN_STATUSES=['QUEUED','RUNNING','SUCCEEDED','FAILED','UNKNOWN'] as const;
export const INSTITUTION_CONCURRENCY_OPTIONS=[3,5,10,15] as const;
export type InstitutionConcurrency=typeof INSTITUTION_CONCURRENCY_OPTIONS[number];
const validConcurrency=(n:unknown):n is InstitutionConcurrency=>
  typeof n==='number'&&INSTITUTION_CONCURRENCY_OPTIONS.includes(n as InstitutionConcurrency);
const status=(row:TaskRow)=>({
  taskId:row.id,status:row.status,requestId:row.request_id,
  createdAt:new Date(row.created_at).toISOString(),updatedAt:new Date(row.updated_at).toISOString(),
  result:row.status==='SUCCEEDED'&&row.result_text?{summary:row.result_text}:null,
  errorCode:row.status==='FAILED'||row.status==='UNKNOWN'?row.error_code:null,
});
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

export class InstitutionTaskStore {
  private readonly db:DatabaseSync;
  constructor(path:string,workspace?:string){
    if(!isAbsolute(path)||path.startsWith('\\\\')||path.startsWith('//'))
      throw new Error('INSTITUTION_API_DB_PATH_INVALID');
    if(workspace){
      const r=relative(workspace,path);
      if(r===''||r!=='..'&&!r.startsWith('..'+sep)&&!isAbsolute(r))
        throw new Error('INSTITUTION_API_DB_MUST_BE_OUTSIDE_WORKSPACE');
    }
    mkdirSync(dirname(path),{recursive:true,mode:0o700});
    if(existsSync(path)&&(!lstatSync(path).isFile()||lstatSync(path).isSymbolicLink()))
      throw new Error('INSTITUTION_API_DB_PATH_INVALID');
    this.db=new DatabaseSync(path);
    chmodSync(path,0o600);
    this.db.exec("PRAGMA journal_mode=WAL;PRAGMA busy_timeout=3000;"+
      "CREATE TABLE IF NOT EXISTS institution_api_tasks ("+
      "id TEXT PRIMARY KEY,client_id TEXT NOT NULL,user_hash TEXT NOT NULL,"+
      "request_id TEXT NOT NULL,payload_hash TEXT NOT NULL,day TEXT NOT NULL,msg_sn TEXT NOT NULL,"+
      "status TEXT NOT NULL,session_id TEXT,result_text TEXT,error_code TEXT,external_user_id TEXT,"+
      "created_at INTEGER NOT NULL,updated_at INTEGER NOT NULL,"+
      "UNIQUE(client_id,user_hash,request_id));"+
      "CREATE INDEX IF NOT EXISTS ix_institution_api_task_status ON institution_api_tasks(status,created_at);"+
      "CREATE TABLE IF NOT EXISTS institution_api_settings(key TEXT PRIMARY KEY,value TEXT NOT NULL);"+
      "INSERT OR IGNORE INTO institution_api_settings(key,value) VALUES ('max_concurrency','3');");
    // Preserve all prior tasks. Their hashed user IDs are irreversible.
    const columns=this.db.prepare("PRAGMA table_info(institution_api_tasks)").all() as Array<{name:string}>;
    if(!columns.some(c=>c.name==='external_user_id'))
      this.db.exec("ALTER TABLE institution_api_tasks ADD COLUMN external_user_id TEXT");
    // Crashes or service restarts must not silently replay model/tool actions.
    // The native session might still have run, so never assert FAILED/SUCCEEDED.
    this.db.prepare("UPDATE institution_api_tasks SET status='UNKNOWN',error_code='PROCESS_RESTARTED',updated_at=?"+
      " WHERE status IN ('QUEUED','RUNNING')").run(Date.now());
    this.prune();
  }
  private prune(){
    this.db.prepare('DELETE FROM institution_api_tasks WHERE created_at<?').run(Date.now()-RETENTION_MS);
  }
  create(clientId:string,userHash:string,requestId:string,day:string,msgSn:string,externalUserId?:string){
    const payloadHash=DIGEST(JSON.stringify([day,msgSn]));
    const id=randomUUID(),now=Date.now();
    this.prune();
    const change=this.db.prepare(
      "INSERT OR IGNORE INTO institution_api_tasks "+
      "(id,client_id,user_hash,request_id,payload_hash,day,msg_sn,external_user_id,status,created_at,updated_at) "+
      "VALUES (?,?,?,?,?,?,?,?,'QUEUED',?,?)"
    ).run(id,clientId,userHash,requestId,payloadHash,day,msgSn,externalUserId??null,now,now);
    const row=this.db.prepare('SELECT * FROM institution_api_tasks WHERE client_id=? AND user_hash=? AND request_id=?')
      .get(clientId,userHash,requestId) as TaskRow|undefined;
    if(!row)throw new Error('INSTITUTION_API_TASK_NOT_SAVED');
    if(row.payload_hash!==payloadHash)throw new AuthProblem(409,'REQUEST_ID_PAYLOAD_CONFLICT');
    return {row,created:Number(change.changes)===1};
  }
  get(clientId:string,userHash:string,id:string):TaskRow|undefined{
    this.prune();
    return this.db.prepare('SELECT * FROM institution_api_tasks WHERE id=? AND client_id=? AND user_hash=?')
      .get(id,clientId,userHash) as TaskRow|undefined;
  }
  concurrency():InstitutionConcurrency{
    const row=this.db.prepare("SELECT value FROM institution_api_settings WHERE key='max_concurrency'")
      .get() as {value:string}|undefined;
    const value=Number(row?.value??'3');
    if(!validConcurrency(value))throw new Error('INSTITUTION_CONCURRENCY_CONFIG_INVALID');
    return value;
  }
  setConcurrency(n:InstitutionConcurrency){
    if(!validConcurrency(n))throw new AuthProblem(400,'INSTITUTION_CONCURRENCY_INVALID');
    this.db.prepare("UPDATE institution_api_settings SET value=? WHERE key='max_concurrency'").run(String(n));
    return this.concurrency();
  }
  queueCounts(){
    const rows=this.db.prepare("SELECT status,COUNT(*) AS n FROM institution_api_tasks WHERE status IN ('QUEUED','RUNNING') GROUP BY status")
      .all() as Array<{status:string;n:number}>;
    return {running:rows.find(r=>r.status==='RUNNING')?.n??0,
      queued:rows.find(r=>r.status==='QUEUED')?.n??0};
  }
  /** Only config administrators may obtain opaque per-user group identifiers. */
  groupsAdmin(query:string|undefined,offset:number){
    this.prune();
    const where=query?' WHERE external_user_id=?':'',params=query?[query]:[];
    const count=this.db.prepare('SELECT COUNT(*) AS n FROM (SELECT 1 FROM institution_api_tasks'+where+
      ' GROUP BY client_id,user_hash)').get(...params) as {n:number};
    const rows=this.db.prepare(
      'SELECT client_id,user_hash,MAX(external_user_id) AS external_user_id,COUNT(*) AS total,'+
      "SUM(CASE WHEN status='QUEUED' THEN 1 ELSE 0 END) AS queued,"+
      "SUM(CASE WHEN status='RUNNING' THEN 1 ELSE 0 END) AS running,"+
      "SUM(CASE WHEN status='SUCCEEDED' THEN 1 ELSE 0 END) AS succeeded,"+
      "SUM(CASE WHEN status='FAILED' THEN 1 ELSE 0 END) AS failed,"+
      "SUM(CASE WHEN status='UNKNOWN' THEN 1 ELSE 0 END) AS unknown,"+
      'MAX(created_at) AS last_seen FROM institution_api_tasks'+where+
      ' GROUP BY client_id,user_hash ORDER BY last_seen DESC LIMIT 50 OFFSET ?'
    ).all(...params,offset) as Array<{client_id:string;user_hash:string;external_user_id:string|null;total:number;
      queued:number;running:number;succeeded:number;failed:number;unknown:number;last_seen:number}>;
    return {total:count.n,limit:50,offset,items:rows.map(row=>({
      clientId:row.client_id,userHash:row.user_hash,externalUserId:row.external_user_id,
      total:row.total,queued:row.queued,running:row.running,succeeded:row.succeeded,
      failed:row.failed,unknown:row.unknown,lastSeen:new Date(row.last_seen).toISOString(),
    }))};
  }
  /** B integration tasks across users; only the separately authorized local admin routes call this. */
  listAdmin(statusFilter:TaskStatus|undefined,query:string|undefined,offset:number,userHash?:string){
    this.prune();
    const clauses:string[]=[],params:string[]=[];
    if(statusFilter){clauses.push('status=?');params.push(statusFilter);}
    if(query){clauses.push('(id=? OR request_id=? OR external_user_id=?)');params.push(query,query,query);}
    if(userHash){clauses.push('user_hash=?');params.push(userHash);}
    const where=clauses.length?' WHERE '+clauses.join(' AND '):'';
    const total=(this.db.prepare('SELECT COUNT(*) AS n FROM institution_api_tasks'+where)
      .get(...params) as {n:number}).n;
    const rows=this.db.prepare('SELECT id,client_id,user_hash,external_user_id,request_id,day,msg_sn,status,error_code,'+
      'created_at,updated_at,(result_text IS NOT NULL) AS has_result FROM institution_api_tasks'+
      where+' ORDER BY created_at DESC,id DESC LIMIT 50 OFFSET ?')
      .all(...params,offset) as Array<Pick<TaskRow,
      'id'|'client_id'|'user_hash'|'external_user_id'|'request_id'|'day'|'msg_sn'|'status'|'error_code'|'created_at'|'updated_at'>&{has_result:number}>;
    return {total,limit:50,offset,items:rows.map(row=>({
      taskId:row.id,clientId:row.client_id,userHash:row.user_hash.slice(0,12),
      externalUserId:row.external_user_id,
      requestId:row.request_id,day:row.day,msgSn:row.msg_sn||null,status:row.status,
      errorCode:row.error_code,hasResult:row.has_result===1,
      createdAt:new Date(row.created_at).toISOString(),updatedAt:new Date(row.updated_at).toISOString(),
    }))};
  }
  detailAdmin(id:string){
    this.prune();
    const row=this.db.prepare('SELECT * FROM institution_api_tasks WHERE id=?').get(id) as TaskRow|undefined;
    if(!row)throw new AuthProblem(404,'INVESTIGATION_NOT_FOUND');
    return {
      taskId:row.id,clientId:row.client_id,userHash:row.user_hash.slice(0,12),
      externalUserId:row.external_user_id,
      requestId:row.request_id,day:row.day,msgSn:row.msg_sn||null,status:row.status,
      errorCode:row.error_code,
      createdAt:new Date(row.created_at).toISOString(),updatedAt:new Date(row.updated_at).toISOString(),
      result:row.status==='SUCCEEDED'&&row.result_text?{summary:row.result_text}:null,
    };
  }
  /** Observed caller assertions, NOT an authenticated or approved-platform list. */
  usageAdmin(){
    this.prune();
    const rows=this.db.prepare(
      "SELECT client_id,MAX(external_user_id) AS external_user_id,user_hash,COUNT(*) AS task_count,"+
      "MIN(created_at) AS first_seen,MAX(created_at) AS last_seen,"+
      "SUM(CASE WHEN status='SUCCEEDED' THEN 1 ELSE 0 END) AS succeeded "+
      "FROM institution_api_tasks GROUP BY client_id,user_hash "+
      "ORDER BY last_seen DESC LIMIT 100"
    ).all() as Array<{client_id:string;external_user_id:string|null;user_hash:string;
      task_count:number;first_seen:number;last_seen:number;succeeded:number}>;
    return {authMode:'NONE',platformIdentityVerified:false,platformAllowlistEnabled:false,
      retentionDays:7,users:rows.map(row=>({
        clientId:row.client_id,externalUserId:row.external_user_id,
        userHash:row.user_hash.slice(0,12),taskCount:row.task_count,
        successCount:row.succeeded,firstSeen:new Date(row.first_seen).toISOString(),
        lastSeen:new Date(row.last_seen).toISOString(),
      }))};
  }
  hasQueued(){return Boolean(this.db.prepare("SELECT 1 FROM institution_api_tasks WHERE status='QUEUED' LIMIT 1").get());}
  take():TaskRow|undefined{
    const row=this.db.prepare("SELECT * FROM institution_api_tasks WHERE status='QUEUED' ORDER BY created_at,id LIMIT 1").get() as TaskRow|undefined;
    if(!row)return undefined;
    const changed=this.db.prepare("UPDATE institution_api_tasks SET status='RUNNING',updated_at=? WHERE id=? AND status='QUEUED'")
      .run(Date.now(),row.id);
    return Number(changed.changes)===1?{...row,status:'RUNNING'}:undefined;
  }
  session(id:string,nativeId:string){
    this.db.prepare("UPDATE institution_api_tasks SET session_id=?,updated_at=? WHERE id=? AND status='RUNNING'")
      .run(nativeId,Date.now(),id);
  }
  finish(id:string,next:'SUCCEEDED'|'FAILED'|'UNKNOWN',result:string|null,error:string|null){
    this.db.prepare("UPDATE institution_api_tasks SET status=?,result_text=?,error_code=?,msg_sn='',updated_at=?"+
      " WHERE id=? AND status='RUNNING'").run(next,result,error,Date.now(),id);
  }
  stopUnfinished(){
    this.db.prepare("UPDATE institution_api_tasks SET status='UNKNOWN',error_code='SERVER_STOPPED',updated_at=?"+
      " WHERE status IN ('QUEUED','RUNNING')").run(Date.now());
  }
  close(){this.db.close();}
}

/** Shanghai business dates: calendar day is [00:00:00, next 00:00:00) UTC+08:00. */
export function institutionDayRange(day:string){
  if(!/^\d{4}-\d{2}-\d{2}$/.test(day))throw new AuthProblem(400,'INVALID_INSTITUTION_DATE');
  const from=Date.parse(day+'T00:00:00+08:00');
  if(!Number.isFinite(from)||new Date(from+8*3600000).toISOString().slice(0,10)!==day)
    throw new AuthProblem(400,'INVALID_INSTITUTION_DATE');
  return {from,to:from+86400000};
}
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
export class InstitutionIntegration {
  private readonly active=new Set<Promise<void>>();
  private closed=false;
  private readonly stop=new AbortController();
  private readonly cfg:InstitutionIntegrationConfig;
  private readonly deps:InstitutionIntegrationDeps;
  private readonly tasks:InstitutionTaskStore;
  constructor(cfg:InstitutionIntegrationConfig,deps:InstitutionIntegrationDeps,tasks:InstitutionTaskStore){
    this.cfg=cfg;this.deps=deps;this.tasks=tasks;
    if(!deps.execution.singleService())throw new Error('INSTITUTION_API_REQUIRES_SINGLE_SERVICE_RUNTIME');
  }
  ownerFor(req:FastifyRequest){
    // User IDs are asserted by the caller, NOT verified by C. Only use the API
    // behind network-level access control and a B backend enforcing identity.
    const subject=req.headers['x-workbench-user-id'];
    if(typeof subject!=='string'||!validId(subject))throw new AuthProblem(400,'INTEGRATION_USER_REQUIRED');
    return DIGEST(JSON.stringify([this.cfg.clientId,subject]));
  }
  create(userHash:string,requestId:string,day:string,msgSn:string,externalUserId:string){
    const created=this.tasks.create(this.cfg.clientId,userHash,requestId,day,msgSn,externalUserId);
    if(created.created)this.pump();
    return {data:status(created.row),created:created.created};
  }
  get(userHash:string,id:string){
    const row=this.tasks.get(this.cfg.clientId,userHash,id);
    if(!row)throw new AuthProblem(404,'INVESTIGATION_NOT_FOUND');
    return status(row);
  }
  listAdmin(statusFilter:TaskStatus|undefined,query:string|undefined,offset:number,userHash?:string){
    return this.tasks.listAdmin(statusFilter,query,offset,userHash);
  }
  groupsAdmin(query:string|undefined,offset:number){return this.tasks.groupsAdmin(query,offset);}
  concurrencyStatus(){return {maxConcurrency:this.tasks.concurrency(),...this.tasks.queueCounts()};}
  setConcurrency(n:InstitutionConcurrency){
    this.tasks.setConcurrency(n);
    // Increase capacity immediately; reductions do not terminate running sessions.
    this.pump();
    return this.concurrencyStatus();
  }
  detailAdmin(id:string){return this.tasks.detailAdmin(id);}
  usageAdmin(){return {...this.tasks.usageAdmin(),currentPlatformLabel:this.cfg.clientId};}
  private pump(){
    if(this.closed)return;
    // Claims are synchronous and conditional in SQLite; each task has its own
    // Promise, execution context and native OpenCode Session.
    while(!this.closed&&this.active.size<this.tasks.concurrency()){
      const task=this.tasks.take();
      if(!task)break;
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
  private async runTask(task:TaskRow){
    if(this.closed)return;
    let accepted=false;
      let definiteFailure=false;
      try{
        const result=await this.deps.execution.run(task.id,async()=>{
          const client=await this.deps.nativeConnection();
          const binding=await this.deps.sceneConfig.binding('institution-message');
          if(binding.executor.type!=='skill')throw new AuthProblem(409,'INSTITUTION_SKILL_NOT_CONFIGURED');
          const catalog=await client.skill.list({location:{directory:binding.directory}});
          if(catalog.location?.directory!==binding.directory||!Array.isArray(catalog.data))
            throw new AuthProblem(502,'INSTITUTION_SKILL_CATALOG_UNAVAILABLE');
          const matches=catalog.data.filter(skill=>skill.name===binding.executor.name&&Boolean(skill.id));
          if(matches.length!==1)throw new AuthProblem(409,'INSTITUTION_SKILL_NOT_FOUND');
          const models=await client.model.list({location:{directory:binding.directory}});
          const enabled=models.data.filter(m=>m.enabled);
          const preferred=process.env.HARNESS_INSTITUTION_API_MODEL_ID?.trim();
          const model=preferred?enabled.find(m=>m.id===preferred):
            enabled.find(m=>m.id.toLowerCase()==='kimi'||m.name.toLowerCase()==='kimi')??
            enabled.find(m=>(m.id+' '+m.name).toLowerCase().includes('kimi'))??
            (enabled.length===1?enabled[0]:undefined);
          if(!model)throw new AuthProblem(409,'INSTITUTION_MODEL_NOT_CONFIGURED');
          const permissions=applyToolPolicy(this.deps.execution.permissions(),
            this.deps.sceneAccess.policy('institution-message'));
          const session=await client.session.create({
            location:{directory:binding.directory},title:'机构报文排查 API',
            model:{providerID:model.providerID,id:model.id},
            metadata:{harnessScenario:'institution-message-inspection',harnessOwner:task.id,harnessIntegration:'platform-b'},
            ...(permissions?{permissions}:{})
          });
          if(!session?.id||session.location?.directory!==binding.directory||
            (permissions&&JSON.stringify(session.permissions)!==JSON.stringify(permissions))){
            if(session?.id)await client.session.remove({sessionID:session.id}).catch(()=>{});
            throw new AuthProblem(409,'INSTITUTION_SESSION_MISMATCH');
          }
          if(this.closed){await client.session.remove({sessionID:session.id}).catch(()=>{});throw new Error('SERVER_STOPPED');}
          this.tasks.session(task.id,session.id);
          // A transport error during submission is ambiguous: the server might have
          // accepted the command even if the response was lost.
          accepted=true;
          await client.session.prompt({sessionID:session.id,text:institutionPrompt(task.day,task.msg_sn),
            skills:[{id:matches[0].id}],delivery:'queue'});
          const end=Date.now()+TIMEOUT_MS;
          while(!this.closed&&Date.now()<end){
            const [active,inbox,messages,permissionsPending]=await Promise.all([
              client.session.active(),client.session.inbox.list({sessionID:session.id}),
              client.message.list({sessionID:session.id,order:'desc',limit:100}),
              client.permission.list({sessionID:session.id}),
            ]);
            if(permissionsPending.some(p=>p.sessionID===session.id)){
              await client.session.interrupt({sessionID:session.id});
              definiteFailure=true;
              throw new AuthProblem(409,'INSTITUTION_PERMISSION_REQUIRED');
            }
            const final=messages.data.find((m):m is Extract<(typeof messages.data)[number],{type:'assistant'}>=>
              m.type==='assistant'&&Boolean(m.time?.completed)&&Boolean(m.finish)&&m.finish!=='tool-calls');
            if(final&&!active[session.id]&&inbox.length===0){
              if(final.error||final.finish==='error')throw new AuthProblem(502,'INSTITUTION_NATIVE_EXECUTION_FAILED');
              const answer=final.content.flatMap(part=>part.type==='text'?[part.text]:[]).join('\n').trim();
              if(!answer)throw new AuthProblem(502,'INSTITUTION_EMPTY_RESULT');
              return answer.slice(0,65536);
            }
            await delay(1200,undefined,{signal:this.stop.signal});
          }
          if(!this.closed)await client.session.interrupt({sessionID:session.id}).catch(()=>{});
          throw new AuthProblem(504,'INSTITUTION_EXECUTION_TIMEOUT');
        });
        if(!this.closed)this.tasks.finish(task.id,'SUCCEEDED',result,null);
      }catch(error){
        if(!this.closed){
          const code=error instanceof AuthProblem?error.code:'INSTITUTION_EXECUTION_UNAVAILABLE';
          // Delivery/network failure after prompt dispatch can be ambiguous;
          // only definite pre-dispatch or confirmed-interrupted failures are FAILED.
          this.tasks.finish(task.id,accepted&&!definiteFailure?'UNKNOWN':'FAILED',null,code);
        }
      }
  }
  close(){
    this.closed=true;this.stop.abort();
    this.tasks.stopUnfinished();
    this.tasks.close();
  }
}

export function registerInstitutionIntegration(app:FastifyInstance,
  deps:InstitutionIntegrationDeps,cfg:InstitutionIntegrationConfig){
  // Lazy initialization avoids creating a sensitive task DB during mere app
  // startup, including unrelated workbench tests that never call this API.
  let manager:InstitutionIntegration|undefined;
  const getManager=()=>manager??=new InstitutionIntegration(cfg,deps,
    new InstitutionTaskStore(cfg.databasePath,deps.workspace));
  app.addHook('onClose',()=>manager?.close());
  // Same management boundary as /config editing: actual local socket, active
  // workspace login, and exact editor username in config-access.json.
  // The B-supplied X-Workbench-User-Id is not an administrator credential.
  const localProfile=new WorkspaceProfile(deps.workspace);
  const authorizeAdmin=(req:FastifyRequest)=>{
    if(!deps.auth.isLocal(req)||!deps.auth.origin(req))
      throw new AuthProblem(403,'CONFIG_LOCAL_ONLY');
    const session=deps.auth.session(req);
    if(session.scope==='lan'||!session.profile||!localProfile.matches(session.profile.key))
      throw new AuthProblem(401,'PROFILE_LOGIN_REQUIRED');
    if(!deps.sceneConfig.canEdit(session.profile.name))
      throw new AuthProblem(403,'SCENE_CONFIG_EDIT_FORBIDDEN');
  };
  app.get('/api/v1/institution-access',async(req,reply)=>{
    authorizeAdmin(req);
    return reply.header('Cache-Control','no-store')
      .send({data:getManager().usageAdmin(),requestId:randomUUID()});
  });
  app.get<{Querystring:{status?:string;q?:string;offset?:string}}>(ADMIN_ROOT,async(req,reply)=>{
    authorizeAdmin(req);
    const {status:state='ALL',q='',offset='0'}=req.query;
    if(state!=='ALL'&&!ADMIN_STATUSES.includes(state as TaskStatus))
      throw new AuthProblem(400,'INSTITUTION_TASK_FILTER_INVALID');
    if(q&&(!validId(q)||q.length>128)||!/^\d{1,5}$/.test(String(offset))||Number(offset)>10000)
      throw new AuthProblem(400,'INSTITUTION_TASK_FILTER_INVALID');
    return reply.header('Cache-Control','no-store')
      .send({data:getManager().listAdmin(state==='ALL'?undefined:state as TaskStatus,q||undefined,Number(offset)),requestId:randomUUID()});
  });
  app.get<{Params:{id:string}}>(ADMIN_ROOT+'/:id',async(req,reply)=>{
    authorizeAdmin(req);
    if(!taskIdPattern.test(req.params.id))throw new AuthProblem(404,'INVESTIGATION_NOT_FOUND');
    return reply.header('Cache-Control','no-store')
      .send({data:getManager().detailAdmin(req.params.id),requestId:randomUUID()});
  });
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
