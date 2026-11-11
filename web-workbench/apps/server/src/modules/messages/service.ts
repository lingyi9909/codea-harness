import { createHmac, randomUUID, timingSafeEqual } from 'node:crypto';
import type { Pool, RowDataPacket } from 'mysql2/promise';
import type { DTO } from '../../../../../packages/contracts/src/domain.ts';
import { withTransaction } from '../../../../../packages/storage/src/db.ts';
import { digestOperation, findOperation } from '../../../../../packages/storage/src/operations.ts';
import { appendEvent } from '../../../../../packages/storage/src/events.ts';
import { AuthProblem } from '../../auth.ts';
import { checkUuid } from '../repositories/service.ts';

type ContinuationKind='DISCUSS'|'RECHECK';
type Operation=DTO<'Operation'>;
const iso=(v:unknown)=>String(v).replace(' ','T')+'Z';
function requireText(value:unknown,max:number,name:string):string{
  if(typeof value!=='string'||!value.trim()||Array.from(value).length>max)
    throw new AuthProblem(400,name+'_INVALID');
  return value.trim();
}
function rawJson(value:unknown):string{
  if(typeof value==='string'){
    try{JSON.parse(value);return value;}catch{throw new AuthProblem(503,'STORED_JSON_INVALID');}
  }
  if(value&&typeof value==='object')return JSON.stringify(value);
  throw new AuthProblem(503,'STORED_JSON_INVALID');
}
export async function createContinuation(input:{
  ownerId:string;taskId:string;operationId:string;kind:ContinuationKind;
  text?:string;quote?:string;reportArtifactId?:string;
},opts:{pool:Pool}):Promise<Operation>{
  checkUuid(input.taskId);checkUuid(input.operationId);
  if(input.kind!=='DISCUSS'&&input.kind!=='RECHECK')
    throw new AuthProblem(400,'CONTINUATION_KIND_INVALID');
  const text=input.kind==='DISCUSS'?requireText(input.text,8000,'DISCUSSION_TEXT'):'';
  const quote=input.kind==='DISCUSS'&&input.quote!==undefined?
    requireText(input.quote,4000,'QUOTE'):'';
  if(input.kind==='RECHECK'&&(input.text!==undefined||input.quote!==undefined||
    input.reportArtifactId!==undefined))throw new AuthProblem(400,'RECHECK_SCOPE_IMMUTABLE');
  if(input.reportArtifactId)checkUuid(input.reportArtifactId);
  const action=input.kind==='DISCUSS'?'TASK_DISCUSS':'TASK_RECHECK';
  const requestHash=digestOperation(action,{taskId:input.taskId,text,quote,
    reportArtifactId:input.reportArtifactId??null});
  return withTransaction(opts.pool,async tx=>{
    // Same serialization order as Task creation/stop/Answer.
    const [slots]=await tx.execute<RowDataPacket[]>(
      "SELECT run_id,quarantined FROM wh_executor_slots WHERE slot_key='local-default' FOR UPDATE");
    if(slots.length!==1)throw new AuthProblem(503,'EXECUTOR_SLOT_MISSING');
    const old=await findOperation(tx,input.ownerId,input.operationId);
    if(old){
      if(old.action!==action||old.requestSha256!==requestHash)
        throw new AuthProblem(409,'OPERATION_CONFLICT');
      const {action:_action,requestSha256:_sha,...op}=old;
      return op;
    }
    const [tasks]=await tx.execute<RowDataPacket[]>(
      "SELECT t.id,t.next_run_number,s.status AS snapshot_status FROM wh_tasks t "+
      "JOIN wh_scope_snapshots s ON s.task_id=t.id "+
      "WHERE t.id=? AND t.owner_id=? FOR UPDATE",
      [input.taskId,input.ownerId]);
    if(tasks.length!==1)throw new AuthProblem(404,'NOT_FOUND');
    if(tasks[0].snapshot_status!=='READY')throw new AuthProblem(409,'SNAPSHOT_NOT_READY');
    const [runs]=await tx.execute<RowDataPacket[]>(
      'SELECT id,kind,status,run_number,asset_versions FROM wh_runs WHERE task_id=? '+
      'ORDER BY run_number DESC LIMIT 1 FOR UPDATE',[input.taskId]);
    if(runs.length!==1||runs[0].status!=='SUCCEEDED')
      throw new AuthProblem(409,'LATEST_RUN_NOT_SUCCESSFUL');
    if(String(slots[0].run_id)===String(runs[0].id)||
      Number(slots[0].quarantined)!==0)
      throw new AuthProblem(409,'PREVIOUS_RUN_NOT_SETTLED');
    const [binding]=await tx.execute<RowDataPacket[]>(
      'SELECT task_id,continuity FROM wh_session_bindings WHERE task_id=?',[input.taskId]);
    if(binding.length!==1)throw new AuthProblem(409,'TRUSTED_SESSION_MISSING');
    if(input.reportArtifactId){
      const [artifacts]=await tx.execute<RowDataPacket[]>(
        'SELECT id FROM wh_artifacts WHERE id=? AND task_id=?',
        [input.reportArtifactId,input.taskId]);
      if(artifacts.length!==1)throw new AuthProblem(404,'REPORT_NOT_FOUND');
    }
    if(input.kind==='RECHECK'){
      // Never replace a previous report. New Review/RECHECK must get its own version.
      const [artifacts]=await tx.execute<RowDataPacket[]>(
        'SELECT id FROM wh_artifacts WHERE task_id=? ORDER BY version DESC LIMIT 1',
        [input.taskId]);
      if(!artifacts.length)throw new AuthProblem(409,'NO_PRIOR_REVIEW_REPORT');
    }
    const [queued]=await tx.execute<RowDataPacket[]>(
      "SELECT COUNT(*) AS total FROM wh_runs WHERE status='QUEUED'");
    if(Number(queued[0]?.total??0)>=20)throw new AuthProblem(429,'QUEUE_FULL');
    const number=Number(tasks[0].next_run_number);
    if(!Number.isSafeInteger(number)||number<2||number>=4294967295)
      throw new AuthProblem(503,'RUN_COUNTER_INVALID');
    const runId=randomUUID(),messageId=input.kind==='DISCUSS'?randomUUID():null;
    const opDbId=randomUUID();
    await tx.execute(
      "INSERT INTO wh_runs(id,task_id,run_number,kind,status,dispatch_state,asset_versions,"+
      "created_at,updated_at) VALUES (?,?,?,?,'QUEUED','NOT_SENT',?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))",
      [runId,input.taskId,number,input.kind,rawJson(runs[0].asset_versions)]);
    await tx.execute(
      'UPDATE wh_tasks SET next_run_number=?,updated_at=UTC_TIMESTAMP(3) WHERE id=? AND next_run_number=?',
      [number+1,input.taskId,number]);
    if(messageId){
      // The report quote is data, never an instruction entry or a changed Git scope.
      const userContent=JSON.stringify({text,quote,reportArtifactId:input.reportArtifactId??null});
      await tx.execute(
        "INSERT INTO wh_messages(id,task_id,run_id,role,content,status,report_artifact_id,created_at,updated_at) "+
        "VALUES (?,?,?,'USER',?,'COMPLETE',?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))",
        [messageId,input.taskId,runId,userContent,input.reportArtifactId??null]);
    }
    const operationResult={taskId:input.taskId,runId,
      ...(messageId?{messageId}:{}),kind:input.kind};
    await tx.execute(
      "INSERT INTO wh_operations(id,owner_id,operation_key,action,request_sha256,task_id,run_id,"+
      "status,result_json,created_at,updated_at) VALUES (?,?,?,?,?,?,?,'ACCEPTED',?,"+
      "UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))",
      [opDbId,input.ownerId,input.operationId,action,requestHash,input.taskId,runId,
        JSON.stringify(operationResult)]);
    const payload={kind:input.kind,taskId:input.taskId,runId,
      ...(messageId?{messageId,text,quote,reportArtifactId:input.reportArtifactId??null}:{})};
    await tx.execute(
      "INSERT INTO wh_outbox(id,operation_id,task_id,run_id,action,payload,status,available_at,"+
      "created_at,updated_at) VALUES (?,?,?,?,'DISPATCH',?,'PENDING',UTC_TIMESTAMP(3),"+
      "UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))",
      [randomUUID(),opDbId,input.taskId,runId,JSON.stringify(payload)]);
    await appendEvent(tx,{taskId:input.taskId,runId,type:'run.status',
      data:{status:'QUEUED',previousStatus:null,errorCode:null}});
    if(messageId)await appendEvent(tx,{taskId:input.taskId,runId,type:'message.completed',
      data:{messageId,status:'COMPLETE'}});
    return {operationId:input.operationId,status:'ACCEPTED',
      taskId:input.taskId,runId,errorCode:null,result:operationResult};
  });
}
function signCursor(secret:string,taskId:string,ownerId:string,date:string,id:string):string{
  const body=Buffer.from(JSON.stringify({taskId,ownerId,date,id})).toString('base64url');
  return body+'.'+createHmac('sha256',secret).update(body).digest('base64url');
}
function parseCursor(secret:string,cursor:string,ownerId:string,taskId:string){
  if(!secret||secret.length<32||cursor.length>2048)throw new AuthProblem(400,'MESSAGE_CURSOR_INVALID');
  const [body,mac,...rest]=cursor.split('.');
  if(!body||!mac||rest.length)throw new AuthProblem(400,'MESSAGE_CURSOR_INVALID');
  const expect=createHmac('sha256',secret).update(body).digest();
  const received=Buffer.from(mac,'base64url');
  if(expect.length!==received.length||!timingSafeEqual(expect,received))
    throw new AuthProblem(400,'MESSAGE_CURSOR_INVALID');
  let value:unknown;
  try{value=JSON.parse(Buffer.from(body,'base64url').toString('utf8'));}
  catch{throw new AuthProblem(400,'MESSAGE_CURSOR_INVALID');}
  if(!value||typeof value!=='object')throw new AuthProblem(400,'MESSAGE_CURSOR_INVALID');
  const obj=value as Record<string,unknown>;
  if(obj.ownerId!==ownerId||obj.taskId!==taskId||
    typeof obj.date!=='string'||typeof obj.id!=='string')
    throw new AuthProblem(400,'MESSAGE_CURSOR_INVALID');
  return {date:obj.date,id:obj.id};
}
export async function listTaskMessages(input:{
  pool:Pool;ownerId:string;taskId:string;limit:number;cursor?:string;cursorKey:string;
}):Promise<DTO<'MessagePage'>>{
  checkUuid(input.taskId);
  if(!Number.isInteger(input.limit)||input.limit<1||input.limit>100)
    throw new AuthProblem(400,'LIMIT_INVALID');
  if(input.cursorKey.length<32)throw new AuthProblem(503,'CURSOR_KEY_NOT_CONFIGURED');
  const [owned]=await input.pool.execute<RowDataPacket[]>(
    'SELECT id FROM wh_tasks WHERE id=? AND owner_id=?',
    [input.taskId,input.ownerId]);
  if(owned.length!==1)throw new AuthProblem(404,'NOT_FOUND');
  const cursor=input.cursor?
    parseCursor(input.cursorKey,input.cursor,input.ownerId,input.taskId):null;
  let sql='SELECT id,run_id,role,content,status,report_artifact_id,created_at,updated_at '+
    'FROM wh_messages WHERE task_id=? ';
  const params:(string|number)[]=[input.taskId];
  if(cursor){
    sql+='AND (created_at<? OR (created_at=? AND id<?)) ';
    params.push(cursor.date,cursor.date,cursor.id);
  }
  sql+='ORDER BY created_at DESC,id DESC LIMIT ?';
  params.push(input.limit+1);
  const [rows]=await input.pool.execute<RowDataPacket[]>(sql,params);
  const page=rows.slice(0,input.limit);
  const last=page.at(-1);
  const items:DTO<'Message'>[]=page.map(r=>{
    let text=String(r.content);
    if(r.role==='USER'){
      try{
        const value=JSON.parse(text) as unknown;
        if(value&&typeof value==='object'&&'text' in value)
          text=String((value as {text:unknown}).text);
      }catch{}
    }
    return{id:String(r.id),runId:String(r.run_id),role:r.role,
      text,status:r.status,reportArtifactId:r.report_artifact_id===null?null:String(r.report_artifact_id),
      createdAt:iso(r.created_at),updatedAt:iso(r.updated_at)};
  });
  return {items,nextCursor:rows.length>input.limit&&last?
    signCursor(input.cursorKey,input.taskId,input.ownerId,String(last.created_at),String(last.id)):null};
}
