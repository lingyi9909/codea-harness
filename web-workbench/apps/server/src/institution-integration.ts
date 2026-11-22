/**
 * Server-to-server institution investigation API for trusted platform B.
 *
 * This intentionally does not reuse browser/anonymous embed credentials:
 * the integration is enabled only by explicit environment configuration,
 * bound to an explicit caller IP allowlist, without application-layer API keys.
 *
 * One OpenCode session per task; never expose native session IDs, directories,
 * tools, transcripts, reasoning or secrets to the external caller.
 */
import {DatabaseSync} from 'node:sqlite';
import {createHash,randomUUID} from 'node:crypto';
import {existsSync,lstatSync,mkdirSync,chmodSync} from 'node:fs';
import {homedir} from 'node:os';
import {isIP} from 'node:net';
import {dirname,isAbsolute,join,relative,sep} from 'node:path';
import {setTimeout as delay} from 'node:timers/promises';
import type {FastifyInstance,FastifyRequest} from 'fastify';
import {AuthProblem} from './auth.ts';
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
  day:string;msg_sn:string;status:TaskStatus;session_id:string|null;
  result_text:string|null;error_code:string|null;created_at:number;updated_at:number;
};
export type InstitutionIntegrationConfig={
  clientId:string;allowedIps:ReadonlySet<string>;databasePath:string;
};
export type InstitutionIntegrationDeps={
  workspace:string;sceneConfig:SceneConfigStore;sceneAccess:SceneAccessStore;
  execution:ExecutionRegistry;nativeConnection:ReturnType<typeof chatConnection>;
};
const validId=(id:string)=>/^[A-Za-z0-9_.:-]{1,128}$/.test(id);
const status=(row:TaskRow)=>({
  taskId:row.id,status:row.status,requestId:row.request_id,
  createdAt:new Date(row.created_at).toISOString(),updatedAt:new Date(row.updated_at).toISOString(),
  result:row.status==='SUCCEEDED'&&row.result_text?{summary:row.result_text}:null,
  errorCode:row.status==='FAILED'||row.status==='UNKNOWN'?row.error_code:null,
});
export function institutionIntegrationConfig(env:NodeJS.ProcessEnv=process.env):InstitutionIntegrationConfig|undefined{
  // An explicit source IP allowlist enables the API. No key or bearer token is required.
  // Without the list, do not register these routes at all.
  if(env.HARNESS_INSTITUTION_API_ALLOWED_IPS===undefined)return undefined;
  const clientId=env.HARNESS_INSTITUTION_API_CLIENT_ID??'platform-b';
  const ips=(env.HARNESS_INSTITUTION_API_ALLOWED_IPS??'').split(',').map(s=>s.trim()).filter(Boolean);
  if(!validId(clientId)||ips.length===0||
     ips.some(ip=>isIP(ip)===0||ip==='0.0.0.0'||ip==='::')||
     new Set(ips).size!==ips.length)
    throw new Error('INSTITUTION_API_AUTH_CONFIG_INVALID');
  const databasePath=env.HARNESS_INSTITUTION_API_DB_PATH??
    join(env.LOCALAPPDATA??homedir(),'.smart-workbench','state','institution-api.sqlite');
  if(!isAbsolute(databasePath)||databasePath.startsWith('\\\\')||databasePath.startsWith('//'))
    throw new Error('INSTITUTION_API_DB_PATH_INVALID');
  return {clientId,allowedIps:new Set(ips),databasePath};
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
      "status TEXT NOT NULL,session_id TEXT,result_text TEXT,error_code TEXT,"+
      "created_at INTEGER NOT NULL,updated_at INTEGER NOT NULL,"+
      "UNIQUE(client_id,user_hash,request_id));"+
      "CREATE INDEX IF NOT EXISTS ix_institution_api_task_status ON institution_api_tasks(status,created_at);");
    // Crashes or service restarts must not silently replay model/tool actions.
    // The native session might still have run, so never assert FAILED/SUCCEEDED.
    this.db.prepare("UPDATE institution_api_tasks SET status='UNKNOWN',error_code='PROCESS_RESTARTED',updated_at=?"+
      " WHERE status IN ('QUEUED','RUNNING')").run(Date.now());
    this.prune();
  }
  private prune(){
    this.db.prepare('DELETE FROM institution_api_tasks WHERE created_at<?').run(Date.now()-RETENTION_MS);
  }
  create(clientId:string,userHash:string,requestId:string,day:string,msgSn:string){
    const payloadHash=DIGEST(JSON.stringify([day,msgSn]));
    const id=randomUUID(),now=Date.now();
    this.prune();
    const change=this.db.prepare(
      "INSERT OR IGNORE INTO institution_api_tasks "+
      "(id,client_id,user_hash,request_id,payload_hash,day,msg_sn,status,created_at,updated_at) "+
      "VALUES (?,?,?,?,?,?,?,'QUEUED',?,?)"
    ).run(id,clientId,userHash,requestId,payloadHash,day,msgSn,now,now);
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
  private busy=false;
  private closed=false;
  private readonly stop=new AbortController();
  private readonly cfg:InstitutionIntegrationConfig;
  private readonly deps:InstitutionIntegrationDeps;
  private readonly tasks:InstitutionTaskStore;
  constructor(cfg:InstitutionIntegrationConfig,deps:InstitutionIntegrationDeps,tasks:InstitutionTaskStore){
    this.cfg=cfg;this.deps=deps;this.tasks=tasks;
    if(!deps.execution.singleService())throw new Error('INSTITUTION_API_REQUIRES_SINGLE_SERVICE_RUNTIME');
  }
  authenticate(req:FastifyRequest){
    // Fastify trustProxy=false: only the real socket peer IP is trusted.
    // X-Forwarded-For/Origin/Host are not used for access decisions.
    const ip=req.ip.startsWith('::ffff:')?req.ip.slice(7):req.ip;
    if(!this.cfg.allowedIps.has(ip))throw new AuthProblem(403,'INTEGRATION_SOURCE_FORBIDDEN');
    const subject=req.headers['x-workbench-user-id'];
    if(typeof subject!=='string'||!validId(subject))throw new AuthProblem(400,'INTEGRATION_USER_REQUIRED');
    return DIGEST(JSON.stringify([this.cfg.clientId,subject]));
  }
  create(userHash:string,requestId:string,day:string,msgSn:string){
    const created=this.tasks.create(this.cfg.clientId,userHash,requestId,day,msgSn);
    if(created.created)this.kick();
    return {data:status(created.row),created:created.created};
  }
  get(userHash:string,id:string){
    const row=this.tasks.get(this.cfg.clientId,userHash,id);
    if(!row)throw new AuthProblem(404,'INVESTIGATION_NOT_FOUND');
    return status(row);
  }
  private kick(){
    if(this.busy||this.closed)return;
    this.busy=true;
    void this.drain().finally(()=>{
      this.busy=false;
      if(!this.closed&&this.tasks.hasQueued())this.kick();
    });
  }
  private async drain(){
    while(!this.closed){
      const task=this.tasks.take();
      if(!task)break;
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
  }
  close(){
    this.closed=true;this.stop.abort();
    this.tasks.stopUnfinished();
    this.tasks.close();
  }
}

export function registerInstitutionIntegration(app:FastifyInstance,
  deps:InstitutionIntegrationDeps,cfg:InstitutionIntegrationConfig){
  const tasks=new InstitutionTaskStore(cfg.databasePath,deps.workspace);
  const manager=new InstitutionIntegration(cfg,deps,tasks);
  app.addHook('onClose',()=>manager.close());
  app.post<{Body:{date:string;msgSn:string;requestId:string}}>(API_ROOT,{
    schema:{body:{type:'object',additionalProperties:false,required:['date','msgSn','requestId'],properties:{
      date:{type:'string',minLength:10,maxLength:10},msgSn:{type:'string',minLength:1,maxLength:256},
      requestId:{type:'string',minLength:1,maxLength:128},
    }}},
  },async(req,reply)=>{
    const user=manager.authenticate(req);
    const {date,msgSn,requestId}=req.body;
    institutionDayRange(date);
    if(typeof msgSn!=='string'||!/^[A-Za-z0-9_.:-]{1,256}$/.test(msgSn)||
      typeof requestId!=='string'||!validId(requestId))
      throw new AuthProblem(400,'INSTITUTION_PARAMETERS_INVALID');
    const result=manager.create(user,requestId,date,msgSn);
    return reply.code(result.created?202:200).header('Cache-Control','no-store')
      .send({data:result.data,requestId:randomUUID()});
  });
  app.get<{Params:{id:string}}>(API_ROOT+'/:id',async(req,reply)=>{
    const user=manager.authenticate(req);
    const id=req.params.id;
    if(!/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(id))
      throw new AuthProblem(404,'INVESTIGATION_NOT_FOUND');
    return reply.header('Cache-Control','no-store')
      .send({data:manager.get(user,id),requestId:randomUUID()});
  });
}
