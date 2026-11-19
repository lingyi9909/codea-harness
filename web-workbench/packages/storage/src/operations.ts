import { createHash } from 'node:crypto';
import type { Pool, PoolConnection, RowDataPacket } from 'mysql2/promise';
import type { DTO } from '../../contracts/src/domain.ts';

export function digestOperation(action:string,body:Record<string,unknown>):string{
  // Callers provide a fixed field-order body: route target and action included.
  return createHash('sha256').update(JSON.stringify({action,body})).digest('hex');
}
function parseResult(raw:unknown):Record<string,unknown>|null{
  if(raw===null||raw===undefined)return null;
  if(typeof raw==='string'){
    const v=JSON.parse(raw) as unknown;
    return v&&typeof v==='object'&&!Array.isArray(v)?v as Record<string,unknown>:null;
  }
  return typeof raw==='object'&&!Array.isArray(raw)?raw as Record<string,unknown>:null;
}
export function toOperation(row:RowDataPacket):DTO<'Operation'>{
  const result=parseResult(row.result_json);
  return {operationId:String(row.operation_key),status:row.status,
    taskId:row.task_id===null?null:String(row.task_id),
    runId:row.run_id===null?null:String(row.run_id),
    errorCode:row.error_code===null?null:String(row.error_code),
    result};
}
export async function findOperation(tx:Pool|PoolConnection,ownerId:string,operationId:string):
  Promise<(DTO<'Operation'>&{action:string;requestSha256:string})|null>{
  const [rows]=await tx.execute<RowDataPacket[]>(
    'SELECT operation_key,status,task_id,run_id,error_code,result_json,action,request_sha256 '+
    'FROM wh_operations WHERE owner_id=? AND operation_key=?',
    [ownerId,operationId]);
  if(!rows.length)return null;
  return {...toOperation(rows[0]),action:String(rows[0].action),
    requestSha256:String(rows[0].request_sha256)};
}
