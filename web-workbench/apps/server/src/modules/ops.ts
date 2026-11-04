import type { PoolConnection, RowDataPacket } from 'mysql2/promise';
import { sha256, utcNow, uuid, withTransaction, query, type DbRow } from '../../../../packages/storage/src/db.ts';
import { httpFail } from '../auth.ts';
import type { DTO } from '../../../../packages/contracts/src/domain.ts';

const UUID=/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
export function uuidRequired(value:unknown):string {
  if(typeof value!=='string'||!UUID.test(value))httpFail(400,'INVALID_OPERATION_ID');
  return value;
}
export function stringRequired(value:unknown,max=2048):string{
  if(typeof value!=='string'||!value.trim()||value.length>max)httpFail(400,'INVALID_REQUEST');
  return value;
}
export function paramRequired(value:unknown):string{
  if(typeof value!=='string'||!UUID.test(value))httpFail(400,'INVALID_RESOURCE_ID');
  return value;
}
export function textLimit(value:unknown,max=8000):string {
  if(typeof value!=='string'||value.length>max)httpFail(400,'INVALID_REQUEST');
  return value;
}
function asJson(value:unknown):Record<string,unknown>|null {
  if(value===null||value===undefined)return null;
  const parsed=typeof value==='string'?JSON.parse(value) as unknown:value;
  return parsed&&typeof parsed==='object'&&!Array.isArray(parsed)?parsed as Record<string,unknown>:null;
}
function operationFromRow(row:DbRow):DTO<'Operation'> {
  return {operationId:String(row.operation_key),status:row.status as DTO<'Operation'>['status'],
    taskId:row.task_id===null?null:String(row.task_id),
    runId:row.run_id===null?null:String(row.run_id),
    errorCode:row.error_code===null?null:String(row.error_code),
    result:asJson(row.result_json)};
}
export async function lookupOperation(owner:string,key:string):Promise<DTO<'Operation'>|null>{
  const rows=await query<DbRow>('SELECT * FROM wh_operations WHERE owner_id=? AND operation_key=? LIMIT 1',[owner,key]);
  return rows.length?operationFromRow(rows[0]):null;
}
type WorkResult<T>={value:T;taskId?:string;runId?:string;status?:DTO<'Operation'>['status'];result?:Record<string,unknown>};
export async function idempotent<T>(owner:string,action:string,target:string,key:string,normalized:unknown,
  work:(tx:PoolConnection,operationRowId:string)=>Promise<WorkResult<T>>):Promise<T> {
  uuidRequired(key);
  const digest=sha256(JSON.stringify({action,target,normalized}));
  return withTransaction(async tx=>{
    const [old]=await tx.execute<RowDataPacket[]>('SELECT * FROM wh_operations WHERE owner_id=? AND operation_key=? FOR UPDATE',
      [owner,key]);
    if(old.length) {
      if(old[0].request_sha256!==digest||old[0].action!==action)httpFail(409,'OPERATION_CONFLICT');
      const saved=asJson(old[0].result_json);
      if(!saved || !Object.hasOwn(saved,'__value'))httpFail(409,'OPERATION_PENDING');
      return saved.__value as T;
    }
    const op=uuid(),now=utcNow();
    await tx.execute(
      'INSERT INTO wh_operations(id,owner_id,operation_key,action,request_sha256,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)',
      [op,owner,key,action,digest,'PROCESSING',now,now]);
    const done=await work(tx,op);
    await tx.execute('UPDATE wh_operations SET task_id=?,run_id=?,status=?,result_json=?,updated_at=? WHERE id=?',
      [done.taskId??null,done.runId??null,done.status??'SUCCEEDED',
      JSON.stringify({__value:done.value,...(done.result??{})}),utcNow(),op]);
    return done.value;
  });
}
export function operationResult(opId:string,taskId:string|null,runId:string|null,status:DTO<'Operation'>['status']='ACCEPTED'):DTO<'Operation'>{
  return {operationId:opId,status,taskId,runId,errorCode:null,result:null};
}
export async function outbox(tx:PoolConnection,operationRowId:string,taskId:string,runId:string,action:'DISPATCH'|'ANSWER'|'INTERRUPT',payload:unknown):Promise<void>{
  const now=utcNow();
  await tx.execute(
    'INSERT INTO wh_outbox(id,operation_id,task_id,run_id,action,payload,status,available_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?, ?,?,?)',
    [uuid(),operationRowId,taskId,runId,action,JSON.stringify(payload),'PENDING',now,now,now]);
}
