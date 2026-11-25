/**
 * 机构任务 SQLite 仓储
 *
 * 职责：保存幂等请求、任务状态、并发档位和用户聚合。
 * Java 研发阅读提示：重点关注 take/finish 与 UNKNOWN：执行结果不确定时不得自动重放。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
/**
 * 机构 API 的 SQLite 持久层。请先读这里，再读 institution-integration.ts。
 *
 * 阅读路线：表结构 -> create/get 幂等 -> take/finish 状态机 ->
 *           concurrency 设置 -> groupsAdmin 用户聚合。
 *
 * Java 对照：
 * - type/接口：DTO 与实体类型声明（仅编译期）
 * - class：Repository 实现类
 * - DatabaseSync.prepare().get()/all()/run()：类似 JDBC PreparedStatement
 * - foo?:string：可选参数（相当于 @Nullable + 默认未传）
 */
import {DatabaseSync} from 'node:sqlite';
import {createHash,randomUUID} from 'node:crypto';
import {existsSync,lstatSync,mkdirSync,chmodSync,realpathSync} from 'node:fs';
import {dirname,isAbsolute,relative,sep} from 'node:path';
import {AuthProblem} from './auth.ts';

const RETENTION_MS=7*24*60*60*1000;
const DIGEST=(s:string)=>createHash('sha256').update(s).digest('hex');

/** 字符串字面量联合类型：类似 Java enum，但仅在 TS 编译时检查。 */
export type TaskStatus='QUEUED'|'RUNNING'|'SUCCEEDED'|'FAILED'|'UNKNOWN';

/** 数据库实体：snake_case 字段与 institution_api_tasks 的列名一一对应。 */
export type TaskRow={
  id:string;client_id:string;user_hash:string;request_id:string;payload_hash:string;
  day:string;msg_sn:string;external_user_id:string|null;status:TaskStatus;session_id:string|null;
  result_text:string|null;error_code:string|null;created_at:number;updated_at:number;
};

/** as const 保留精确数值，InstitutionConcurrency 就变成 3|5|10|15。 */
export const INSTITUTION_CONCURRENCY_OPTIONS=[3,5,10,15] as const;
export type InstitutionConcurrency=typeof INSTITUTION_CONCURRENCY_OPTIONS[number];

/** 类型守卫：既做运行时校验，也告诉 TS 编译器 n 是允许的档位。 */
export const validConcurrency=(n:unknown):n is InstitutionConcurrency=>
  typeof n==='number'&&INSTITUTION_CONCURRENCY_OPTIONS.includes(n as InstitutionConcurrency);

/**
 * SQLite 任务仓储（类比 Java 的 Repository/DAO）。
 *
 * 这里只处理数据持久化、幂等、状态迁移与统计，不负责调用 OpenCode。
 * 上层 InstitutionIntegration 相当于 Java 的 Service，HTTP 路由相当于 Controller。
 *
 * 注意：node:sqlite 的 DatabaseSync 是同步接口，SQL 完成后才继续执行下一条 JS。
 */
export class InstitutionTaskStore {
  private readonly db:DatabaseSync;
  private readonly lease=JSON.stringify({pid:process.pid,token:randomUUID()});
  /**
   * 首次打开时自动建表/兼容旧库；已有 RUNNING/QUEUED 统一标记 UNKNOWN。
   * 类比 Java 应用启动时执行数据库迁移，绝不自动重试可能已触发的业务动作。
   *
   * @param path SQLite 文件路径，必须位于项目工作目录之外
   * @param workspace OpenCode 工作目录，用于校验数据文件不落入代码仓库
   */
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
    // WAL 支持读写并行；busy_timeout 允许在 SQLite 文件短暂占用时等待。
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
    // This is deliberately a single-writer queue. Different ports/workspaces
    // previously opened one default SQLite file and rewrote each other's
    // RUNNING tasks to UNKNOWN during initialization or shutdown.
    try{this.claimExclusive(workspace);}catch(error){this.db.close();throw error;}
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
  /**
   * Claim the database atomically BEFORE any in-flight task status migration.
   * Two service processes may never take() from the same institution queue.
   * The workspace fingerprint prevents accidental cross-project sharing,
   * including after a prior process releases its lease.
   * A dead PID is a stale lease left by a crash and may be reclaimed safely;
   * it does NOT imply prior tasks completed (constructor marks them UNKNOWN).
   */
  private claimExclusive(workspace?:string){
    const normalized=workspace?realpathSync.native(workspace):undefined;
    const fingerprint=normalized?DIGEST(process.platform==='win32'?normalized.toLowerCase():normalized):undefined;
    this.db.exec('BEGIN IMMEDIATE');
    let committed=false;
    try{
      const owner=this.db.prepare("SELECT value FROM institution_api_settings WHERE key='workspace_fingerprint'")
        .get() as {value:string}|undefined;
      if(owner&&fingerprint&&owner.value!==fingerprint)
        throw new AuthProblem(409,'INSTITUTION_API_DB_WORKSPACE_CONFLICT');
      const holder=this.db.prepare("SELECT value FROM institution_api_settings WHERE key='active_instance'")
        .get() as {value:string}|undefined;
      if(holder){
        let alive=true;
        try{
          const info=JSON.parse(holder.value) as {pid:number;token:string};
          if(!Number.isSafeInteger(info.pid)||info.pid<=0||typeof info.token!=='string')
            throw new Error('INVALID_LEASE');
          try{process.kill(info.pid,0);}
          catch(error){
            if((error as NodeJS.ErrnoException).code==='ESRCH')alive=false;
          }
        }catch{/* Invalid owner metadata must fail closed; do not reset live tasks. */}
        if(alive)throw new AuthProblem(409,'INSTITUTION_API_DB_ALREADY_IN_USE');
      }
      if(fingerprint&&!owner){
        this.db.prepare("INSERT INTO institution_api_settings(key,value) VALUES('workspace_fingerprint',?)")
          .run(fingerprint);
      }
      this.db.prepare("INSERT INTO institution_api_settings(key,value) VALUES('active_instance',?) "+
        "ON CONFLICT(key) DO UPDATE SET value=excluded.value").run(this.lease);
      this.db.exec('COMMIT');committed=true;
    }finally{if(!committed)this.db.exec('ROLLBACK');}
  }
  /** 按创建时间清理超过 7 天的数据，避免保存敏感业务信息过久。 */
  private prune(){
    this.db.prepare('DELETE FROM institution_api_tasks WHERE created_at<?').run(Date.now()-RETENTION_MS);
  }
  /**
   * 创建任务（Java 类比：幂等 insertIfAbsent）。
   *
   * 唯一键 client_id + user_hash + request_id：
   *  - 同一个业务用户重复提交相同请求 -> 返回原任务，不重复跑模型
   *  - requestId 相同但日期/msgSn 不同 -> 返回 409，避免请求号被错误复用
   */
  create(clientId:string,userHash:string,requestId:string,day:string,msgSn:string,externalUserId?:string){
    const payloadHash=DIGEST(JSON.stringify([day,msgSn]));
    const id=randomUUID(),now=Date.now();
    this.prune();
    // INSERT OR IGNORE + 唯一约束：避免两个调用方提交相同请求号产生重复任务。
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
  /**
   * 给 B 平台查询自己的任务，必须同时匹配业务用户哈希（WHERE 条件）。
   * 注意：用户 ID 是 B 自报的，哈希隔离 ≠ 身份认证。
   */
  get(clientId:string,userHash:string,id:string):TaskRow|undefined{
    this.prune();
    return this.db.prepare('SELECT * FROM institution_api_tasks WHERE id=? AND client_id=? AND user_hash=?')
      .get(id,clientId,userHash) as TaskRow|undefined;
  }
  /** 读取管理员设置的最大并发数。数据库值只允许 3/5/10/15。 */
  concurrency():InstitutionConcurrency{
    const row=this.db.prepare("SELECT value FROM institution_api_settings WHERE key='max_concurrency'")
      .get() as {value:string}|undefined;
    const value=Number(row?.value??'3');
    if(!validConcurrency(value))throw new Error('INSTITUTION_CONCURRENCY_CONFIG_INVALID');
    return value;
  }
  /** 修改最大并发数（持久化）；不负责中断现有运行任务。 */
  setConcurrency(n:InstitutionConcurrency){
    if(!validConcurrency(n))throw new AuthProblem(400,'INSTITUTION_CONCURRENCY_INVALID');
    this.db.prepare("UPDATE institution_api_settings SET value=? WHERE key='max_concurrency'").run(String(n));
    return this.concurrency();
  }
  /** 用于后台仪表盘的运行中/排队中数量。 */
  queueCounts(){
    const rows=this.db.prepare("SELECT status,COUNT(*) AS n FROM institution_api_tasks WHERE status IN ('QUEUED','RUNNING') GROUP BY status")
      .all() as Array<{status:string;n:number}>;
    return {running:rows.find(r=>r.status==='RUNNING')?.n??0,
      queued:rows.find(r=>r.status==='QUEUED')?.n??0};
  }
  /**
   * 按业务用户聚合（类比 SQL 的 GROUP BY）。
   * 由服务端管理员路由调用；user_hash 是不对外开放给普通 B 调用方的分组键。
   * 原始用户标识是后续版本才保存的，历史记录可能为 NULL。
   */
  groupsAdmin(query:string|undefined,offset:number,limit=50){
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
      ' GROUP BY client_id,user_hash ORDER BY last_seen DESC LIMIT ? OFFSET ?'
    ).all(...params,limit,offset) as Array<{client_id:string;user_hash:string;external_user_id:string|null;total:number;
      queued:number;running:number;succeeded:number;failed:number;unknown:number;last_seen:number}>;
    return {total:count.n,limit,offset,items:rows.map(row=>({
      clientId:row.client_id,userHash:row.user_hash,externalUserId:row.external_user_id,
      total:row.total,queued:row.queued,running:row.running,succeeded:row.succeeded,
      failed:row.failed,unknown:row.unknown,lastSeen:new Date(row.last_seen).toISOString(),
    }))};
  }
  /**
   * 管理后台任务列表：参数采用绑定值，不拼接用户输入到 SQL 语句。
   * 通过 status、requestId、业务用户 ID 或 userHash 组合过滤。
   */
  listAdmin(statusFilter:TaskStatus|undefined,query:string|undefined,offset:number,userHash?:string,limit=50){
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
      where+' ORDER BY created_at DESC,id DESC LIMIT ? OFFSET ?')
      .all(...params,limit,offset) as Array<Pick<TaskRow,
      'id'|'client_id'|'user_hash'|'external_user_id'|'request_id'|'day'|'msg_sn'|'status'|'error_code'|'created_at'|'updated_at'>&{has_result:number}>;
    return {total,limit,offset,items:rows.map(row=>({
      taskId:row.id,clientId:row.client_id,userHash:row.user_hash.slice(0,12),
      externalUserId:row.external_user_id,
      requestId:row.request_id,day:row.day,msgSn:row.msg_sn||null,status:row.status,
      errorCode:row.error_code,hasResult:row.has_result===1,
      createdAt:new Date(row.created_at).toISOString(),updatedAt:new Date(row.updated_at).toISOString(),
    }))};
  }
  /** 仅管理员可读取完整排查结论；列表接口故意不查询 result_text。 */
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
  /**
   * 接口接入情况是使用观测统计，不是授权表。
   * client_id 是服务器配置标签；X-Workbench-User-Id 由请求方主动声明。
   */
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
  /** 查询是否还有待执行任务，可用于运维监测。 */
  hasQueued(){
    return Boolean(
      this.db.prepare("SELECT 1 FROM institution_api_tasks WHERE status='QUEUED' LIMIT 1").get(),
    );
  }
  /**
   * 从队列领取最早任务：QUEUED -> RUNNING。
   * Java 类比：带状态条件的 CAS 更新；只有 UPDATE changes===1 的调用者领取成功。
   * 当前设计要求单 C 进程，不能据此推断可多进程共享执行队列。
   */
  take():TaskRow|undefined{
    const row=this.db.prepare("SELECT * FROM institution_api_tasks WHERE status='QUEUED' ORDER BY created_at,id LIMIT 1").get() as TaskRow|undefined;
    if(!row)return undefined;
    const changed=this.db.prepare("UPDATE institution_api_tasks SET status='RUNNING',updated_at=? WHERE id=? AND status='QUEUED'")
      .run(Date.now(),row.id);
    return Number(changed.changes)===1?{...row,status:'RUNNING'}:undefined;
  }
  /** 保存该任务所对应的 OpenCode Session ID（只用于 C 内部跟踪）。 */
  session(id:string,nativeId:string){
    this.db.prepare("UPDATE institution_api_tasks SET session_id=?,updated_at=? WHERE id=? AND status='RUNNING'")
      .run(nativeId,Date.now(),id);
  }
  /**
   * 完成任务并写入最终结果。
   * WHERE status='RUNNING' 防止覆盖已经终止的任务；同时清空 msg_sn 以缩短敏感数据保留。
   */
  finish(id:string,next:'SUCCEEDED'|'FAILED'|'UNKNOWN',result:string|null,error:string|null){
    this.db.prepare("UPDATE institution_api_tasks SET status=?,result_text=?,error_code=?,msg_sn='',updated_at=?"+
      " WHERE id=? AND status='RUNNING'").run(next,result,error,Date.now(),id);
  }
  /**
   * 服务关闭时标记未完成任务 UNKNOWN（不是 FAILED）。
   * 因为任务可能已经送达模型或 DBHub，不能安全地自动重放。
   */
  stopUnfinished(){
    this.db.prepare("UPDATE institution_api_tasks SET status='UNKNOWN',error_code='SERVER_STOPPED',updated_at=?"+
      " WHERE status IN ('QUEUED','RUNNING')").run(Date.now());
  }
  /** 释放 SQLite 文件句柄。 */
  close(){
    try{this.db.prepare("DELETE FROM institution_api_settings WHERE key='active_instance' AND value=?")
      .run(this.lease);}finally{this.db.close();}
  }
}
