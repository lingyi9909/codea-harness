import { randomUUID, createHmac, timingSafeEqual } from 'node:crypto';
import type { Pool, RowDataPacket } from 'mysql2/promise';
import type { DTO } from '../../../../../packages/contracts/src/domain.ts';
import { withTransaction } from '../../../../../packages/storage/src/db.ts';
import { digestOperation, findOperation } from '../../../../../packages/storage/src/operations.ts';
import { GitFailure, git, trustedRepository } from '../../../../../packages/git-provider/src/repository.ts';
import { AuthProblem } from '../../auth.ts';
import { checkUuid, findRepository } from '../repositories/service.ts';

export type TaskOptions={
  pool:Pool;roots:string[];cursorKey:string;notesSupported:boolean;
};
const statusValues=new Set(['QUEUED','PREPARING','RUNNING','WAITING_INPUT','WAITING_PERMISSION',
  'COLLECTING','STOP_REQUESTED','RECOVERING','SUCCEEDED','FAILED','STOPPED']);
const iso=(s:unknown)=>s===null?null:String(s).replace(' ','T')+'Z';
function requireText(value:unknown,name:string,max:number):string{
  if(typeof value!=='string'||Array.from(value).length>max)
    throw new AuthProblem(400,name+'_INVALID');
  return value;
}
function runDTO(row:RowDataPacket):DTO<'Run'>{
  return {id:String(row.id),taskId:String(row.task_id),number:Number(row.run_number),
    kind:row.kind,status:row.status,retryOfRunId:row.retry_of_run_id===null?null:String(row.retry_of_run_id),
    dispatchState:row.dispatch_state,errorCode:row.error_code===null?null:String(row.error_code),
    createdAt:iso(row.created_at)!,startedAt:iso(row.started_at),finishedAt:iso(row.finished_at)};
}
function taskSummary(r:RowDataPacket):DTO<'TaskSummary'>{
  return {id:String(r.id),repositoryId:String(r.repository_id),title:String(r.title),
    scenarioId:'code-review',status:r.latest_status,latestRunId:String(r.latest_run_id),
    headOID:String(r.head_oid),baseRef:String(r.base_ref),
    createdAt:iso(r.created_at)!,updatedAt:iso(r.updated_at)!};
}
const baseSql='FROM wh_tasks t JOIN wh_review_previews p ON p.id=t.preview_id '+
  'JOIN wh_runs r ON r.task_id=t.id AND r.run_number='+
  '(SELECT MAX(w.run_number) FROM wh_runs w WHERE w.task_id=t.id) ';

export async function createTask(input:{
  ownerId:string;operationId:string;previewId:string;title?:string;controller?:string;notes?:string;
},opts:TaskOptions):Promise<DTO<'Operation'>>{
  checkUuid(input.operationId);checkUuid(input.previewId);
  const title=input.title===undefined?'':requireText(input.title,'TITLE',160).trim();
  const controller=input.controller===undefined?'':requireText(input.controller,'CONTROLLER',128).trim();
  const notes=input.notes===undefined?'':requireText(input.notes,'NOTES',2000).trim();
  if(controller&&!/^[A-Za-z_][A-Za-z_0-9]*$/.test(controller))
    throw new AuthProblem(400,'INVALID_CONTROLLER');
  if(notes&&!opts.notesSupported)throw new AuthProblem(422,'NOTES_UNSUPPORTED');
  const operationHash=digestOperation('TASK_CREATE',{
    previewId:input.previewId,title,controller,notes,
  });
  // First look up the original operation: a retry must not pin a fresh source OID.
  const earlier=await findOperation(opts.pool,input.ownerId,input.operationId);
  if(earlier){
    if(earlier.action!=='TASK_CREATE'||earlier.requestSha256!==operationHash)
      throw new AuthProblem(409,'OPERATION_CONFLICT');
    const {action,requestSha256,...operation}=earlier;
    return operation;
  }
  const [candidate]=await opts.pool.execute<RowDataPacket[]>(
    'SELECT p.*, r.canonical_path,r.status AS repository_status '+
    'FROM wh_review_previews p JOIN wh_repositories r ON r.id=p.repository_id '+
    'WHERE p.owner_id=? AND p.id=? AND r.owner_id=?',
    [input.ownerId,input.previewId,input.ownerId]);
  if(!candidate.length)throw new AuthProblem(404,'NOT_FOUND');
  const preview=candidate[0];
  if(preview.repository_status!=='READY')throw new AuthProblem(409,'REPOSITORY_UNAVAILABLE');
  // Do not hold database locks while reading Git. A concurrent source move is
  // rechecked by the immutable snapshot worker, which refuses a mismatched OID.
  try{
    const root=await trustedRepository(String(preview.canonical_path),opts.roots);
    const [head,base,merge]=await Promise.all([
      git(root,['rev-parse','--verify','HEAD']),
      git(root,['rev-parse','--verify',String(preview.base_ref)+'^{commit}']),
      git(root,['merge-base',String(preview.base_oid),String(preview.head_oid)]),
    ]);
    if(head!==preview.head_oid||base!==preview.base_oid||merge!==preview.merge_base_oid)
      throw new AuthProblem(409,'SCOPE_CHANGED');
  }catch(e){
    if(e instanceof AuthProblem)throw e;
    if(e instanceof GitFailure)throw new AuthProblem(409,'SCOPE_CHANGED');
    throw e;
  }
  return withTransaction(opts.pool,async tx=>{
    // Serialize create vs worker queue claims using the singleton executor slot.
    const [slots]=await tx.execute<RowDataPacket[]>(
      "SELECT slot_key FROM wh_executor_slots WHERE slot_key='local-default' FOR UPDATE");
    if(!slots.length)throw new AuthProblem(503,'EXECUTOR_SLOT_MISSING');
    const duplicate=await findOperation(tx,input.ownerId,input.operationId);
    if(duplicate){
      if(duplicate.action!=='TASK_CREATE'||duplicate.requestSha256!==operationHash)
        throw new AuthProblem(409,'OPERATION_CONFLICT');
      const {action,requestSha256,...operation}=duplicate;
      return operation;
    }
    const [rows]=await tx.execute<RowDataPacket[]>(
      'SELECT p.*,r.name AS repository_name,r.status AS repository_status,'+
      '(p.expires_at>UTC_TIMESTAMP(3)) AS not_expired '+
      'FROM wh_review_previews p JOIN wh_repositories r ON r.id=p.repository_id '+
      'WHERE p.id=? AND p.owner_id=? AND r.owner_id=? FOR UPDATE',
      [input.previewId,input.ownerId,input.ownerId]);
    if(!rows.length)throw new AuthProblem(404,'NOT_FOUND');
    const p=rows[0];
    if(p.repository_status!=='READY')throw new AuthProblem(409,'REPOSITORY_UNAVAILABLE');
    if(Number(p.not_expired)!==1)throw new AuthProblem(409,'PREVIEW_EXPIRED');
    const [taken]=await tx.execute<RowDataPacket[]>(
      'SELECT id FROM wh_tasks WHERE preview_id=? LIMIT 1 FOR UPDATE',[input.previewId]);
    if(taken.length)throw new AuthProblem(409,'PREVIEW_USED');
    const [queued]=await tx.execute<RowDataPacket[]>(
      "SELECT COUNT(*) AS total FROM wh_runs WHERE status='QUEUED'");
    if(Number(queued[0]?.total??0)>=20)throw new AuthProblem(429,'QUEUE_FULL');
    const taskId=randomUUID(),runId=randomUUID(),operationDbId=randomUUID();
    const actualTitle=title||String(p.repository_name).slice(0,110)+' · '+String(p.head_oid).slice(0,10);
    await tx.execute(
      "INSERT INTO wh_tasks(id,owner_id,repository_id,preview_id,scenario_id,title,controller,notes,"+
      "next_event_seq,next_run_number,created_at,updated_at) VALUES (?,?,?,?,'code-review',?,?,?,2,2,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))",
      [taskId,input.ownerId,p.repository_id,input.previewId,actualTitle,controller||null,notes||null]);
    const relativeSnapshot=['tasks',taskId,'repo'].join('/');
    await tx.execute(
      "INSERT INTO wh_scope_snapshots(id,task_id,head_oid,base_oid,merge_base_oid,base_ref,source_branch,"+
      "object_format,manifest,manifest_sha256,storage_key,status,created_at) "+
      "VALUES (?,?,?,?,?,?,?,?,?,?,?,'PENDING',UTC_TIMESTAMP(3))",
      [randomUUID(),taskId,p.head_oid,p.base_oid,p.merge_base_oid,p.base_ref,p.source_branch,
        p.object_format,JSON.stringify({version:1,pending:true}),'0'.repeat(64),relativeSnapshot]);
    await tx.execute(
      "INSERT INTO wh_runs(id,task_id,run_number,kind,status,dispatch_state,native_input_id,"+
      "asset_versions,created_at,updated_at) VALUES (?,?,1,'REVIEW','QUEUED','NOT_SENT',?,?,"+
      "UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))",
      [runId,taskId,randomUUID(),JSON.stringify({pending:true})]);
    await tx.execute(
      "INSERT INTO wh_operations(id,owner_id,operation_key,action,request_sha256,task_id,run_id,"+
      "status,result_json,created_at,updated_at) VALUES (?,?,?,?,?,?,?,'ACCEPTED',?,"+
      "UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))",
      [operationDbId,input.ownerId,input.operationId,'TASK_CREATE',operationHash,taskId,runId,
        JSON.stringify({taskId,runId})]);
    await tx.execute(
      "INSERT INTO wh_outbox(id,operation_id,task_id,run_id,action,payload,status,available_at,"+
      "created_at,updated_at) VALUES (?,?,?,?,'DISPATCH',?,'PENDING',UTC_TIMESTAMP(3),"+
      "UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))",
      [randomUUID(),operationDbId,taskId,runId,JSON.stringify({taskId,runId})]);
    await tx.execute(
      "INSERT INTO wh_events(task_id,seq,run_id,event_type,payload,created_at) "+
      "VALUES (?,1,?,'task.created',?,UTC_TIMESTAMP(3))",
      [taskId,runId,JSON.stringify({taskId})]);
    return {operationId:input.operationId,status:'ACCEPTED',
      taskId,runId,errorCode:null,result:{taskId,runId}};
  });
}
export async function getOperation(pool:Pool,ownerId:string,operationId:string):Promise<DTO<'Operation'>>{
  checkUuid(operationId);
  const op=await findOperation(pool,ownerId,operationId);
  if(!op)throw new AuthProblem(404,'NOT_FOUND');
  const {action,requestSha256,...operation}=op;
  return operation;
}
export async function getTask(pool:Pool,ownerId:string,taskId:string):Promise<DTO<'TaskDetail'>>{
  checkUuid(taskId);
  const [rows]=await pool.execute<RowDataPacket[]>(
    'SELECT t.*,p.head_oid,p.base_oid,p.merge_base_oid,p.base_ref,p.source_branch,p.object_format,'+
    's.status AS snapshot_status,(t.next_event_seq-1) AS last_seq '+
    'FROM wh_tasks t JOIN wh_review_previews p ON p.id=t.preview_id '+
    'LEFT JOIN wh_scope_snapshots s ON s.task_id=t.id WHERE t.owner_id=? AND t.id=?',
    [ownerId,taskId]);
  if(!rows.length)throw new AuthProblem(404,'NOT_FOUND');
  const t=rows[0];
  const [runs]=await pool.execute<RowDataPacket[]>(
    'SELECT * FROM wh_runs WHERE task_id=? ORDER BY run_number DESC LIMIT 1',[taskId]);
  if(!runs.length)throw new AuthProblem(503,'TASK_WITHOUT_RUN');
  const latestRun=runDTO(runs[0]);
  const [artifacts]=await pool.execute<RowDataPacket[]>(
    'SELECT id,run_id,version,kind,sha256,size_bytes,coverage,created_at '+
    'FROM wh_artifacts WHERE task_id=? ORDER BY version ASC',[taskId]);
  const [bindings]=await pool.execute<RowDataPacket[]>(
    'SELECT continuity FROM wh_session_bindings WHERE task_id=?',[taskId]);
  const summary=taskSummary({...t,latest_status:latestRun.status,latest_run_id:latestRun.id} as RowDataPacket);
  return {task:summary,scope:{headOID:String(t.head_oid),baseOID:String(t.base_oid),
    mergeBaseOID:String(t.merge_base_oid),baseRef:String(t.base_ref),
    sourceBranch:t.source_branch===null?null:String(t.source_branch),
    objectFormat:t.object_format},snapshotStatus:t.snapshot_status??'PENDING',
    latestRun,reports:artifacts.map(r=>({id:String(r.id),runId:String(r.run_id),version:Number(r.version),
      kind:'REVIEW_MARKDOWN',sha256:String(r.sha256),sizeBytes:String(r.size_bytes),
      coverage:r.coverage,createdAt:iso(r.created_at)!})),
    lastSeq:String(t.last_seq),continuity:bindings.length?bindings[0].continuity:'UNKNOWN',
    controller:t.controller===null?null:String(t.controller),
    notes:t.notes===null?null:String(t.notes)};
}
function cursorFor(secret:string,ownerId:string,status:string,repo:string,date:string,id:string):string{
  const data=Buffer.from(JSON.stringify({v:1,ownerId,status,repo,date,id})).toString('base64url');
  const mac=createHmac('sha256',secret).update(data).digest('base64url');
  return data+'.'+mac;
}
function decodeCursor(secret:string,token:string,ownerId:string,status:string,repo:string){
  if(secret.length<32)throw new AuthProblem(503,'CURSOR_KEY_NOT_CONFIGURED');
  const parts=token.split('.');
  if(parts.length!==2||parts[0].length>2048)throw new AuthProblem(400,'CURSOR_INVALID');
  const computed=createHmac('sha256',secret).update(parts[0]).digest();
  let supplied:Buffer;
  try{supplied=Buffer.from(parts[1],'base64url');}catch{throw new AuthProblem(400,'CURSOR_INVALID');}
  if(supplied.length!==computed.length||!timingSafeEqual(supplied,computed))
    throw new AuthProblem(400,'CURSOR_INVALID');
  let parsed:unknown;
  try{parsed=JSON.parse(Buffer.from(parts[0],'base64url').toString('utf8')) as unknown;}
  catch{throw new AuthProblem(400,'CURSOR_INVALID');}
  if(!parsed||typeof parsed!=='object')throw new AuthProblem(400,'CURSOR_INVALID');
  const v=parsed as Record<string,unknown>;
  if(v.v!==1||v.ownerId!==ownerId||v.status!==status||v.repo!==repo||
    typeof v.date!=='string'||typeof v.id!=='string')throw new AuthProblem(400,'CURSOR_INVALID');
  return {date:v.date,id:v.id};
}
export async function listTasks(input:{ownerId:string;limit:number;cursor?:string;
  repositoryId?:string;status?:string;},opts:TaskOptions):Promise<DTO<'TaskPage'>>{
  const {ownerId,limit}=input;
  if(!Number.isInteger(limit)||limit<1||limit>100)throw new AuthProblem(400,'LIMIT_INVALID');
  if(input.status&&!statusValues.has(input.status))throw new AuthProblem(400,'INVALID_STATUS');
  if(input.repositoryId)checkUuid(input.repositoryId);
  const status=input.status??'',repo=input.repositoryId??'';
  if(opts.cursorKey.length<32)throw new AuthProblem(503,'CURSOR_KEY_NOT_CONFIGURED');
  const cursor=input.cursor?decodeCursor(opts.cursorKey,input.cursor,ownerId,status,repo):null;
  let sql='SELECT t.id,t.repository_id,t.title,t.created_at,t.updated_at,p.head_oid,p.base_ref,'+
    'r.status AS latest_status,r.id AS latest_run_id '+baseSql+'WHERE t.owner_id=? ';
  const params:(string|number)[]=[ownerId];
  if(repo){sql+='AND t.repository_id=? ';params.push(repo);}
  if(status){sql+='AND r.status=? ';params.push(status);}
  if(cursor){sql+='AND (t.created_at<? OR (t.created_at=? AND t.id<?)) ';
    params.push(cursor.date,cursor.date,cursor.id);}
  sql+='ORDER BY t.created_at DESC,t.id DESC LIMIT ?';
  params.push(String(limit+1));
  const [rows]=await opts.pool.execute<RowDataPacket[]>(sql,params);
  const page=rows.slice(0,limit);
  const last=page.at(-1);
  return {items:page.map(taskSummary),nextCursor:rows.length>limit&&last?
    cursorFor(opts.cursorKey,ownerId,status,repo,String(last.created_at),String(last.id)):null};
}
export async function listRuns(pool:Pool,ownerId:string,taskId:string,limit:number):Promise<DTO<'RunPage'>>{
  checkUuid(taskId);
  if(!Number.isInteger(limit)||limit<1||limit>100)throw new AuthProblem(400,'LIMIT_INVALID');
  const [owned]=await pool.execute<RowDataPacket[]>(
    'SELECT id FROM wh_tasks WHERE owner_id=? AND id=?',[ownerId,taskId]);
  if(!owned.length)throw new AuthProblem(404,'NOT_FOUND');
  const [runs]=await pool.execute<RowDataPacket[]>(
    'SELECT * FROM wh_runs WHERE task_id=? ORDER BY run_number DESC LIMIT ?',
    [taskId,String(limit)]);
  return{items:runs.map(runDTO),nextCursor:null};
}
