import { randomUUID } from 'node:crypto';
import type { Pool, RowDataPacket } from 'mysql2/promise';
import type { DTO } from '../../../../../packages/contracts/src/domain.ts';
import { withTransaction } from '../../../../../packages/storage/src/db.ts';
import { digestOperation, findOperation } from '../../../../../packages/storage/src/operations.ts';
import { AuthProblem } from '../../auth.ts';
import { checkUuid } from '../repositories/service.ts';

type Answer=DTO<'AnswerRequest'>['answer'];
type Field=DTO<'FormField'>;
const isObject=(x:unknown):x is Record<string,unknown> =>
  !!x&&typeof x==='object'&&!Array.isArray(x);
const parseJson=(x:unknown):Record<string,unknown>=>{
  if(typeof x==='string'){
    try{x=JSON.parse(x) as unknown;}catch{throw new AuthProblem(503,'INTERACTION_SCHEMA_CORRUPT');}
  }
  if(!isObject(x))throw new AuthProblem(503,'INTERACTION_SCHEMA_CORRUPT');
  return x;
};
const asFormFields=(obj:Record<string,unknown>):Field[]=>{
  if(!Array.isArray(obj.fields)||!obj.fields.every(f=>isObject(f) &&
    typeof f.id==='string' && typeof f.label==='string' &&
    ['TEXT','SINGLE_SELECT','MULTI_SELECT','BOOLEAN'].includes(String(f.type)) &&
    typeof f.required==='boolean' && Array.isArray(f.options) &&
    f.options.every((v:unknown)=>typeof v==='string') &&
    typeof f.maxLength==='number' && Number.isInteger(f.maxLength)&&f.maxLength>=0 && f.maxLength<=8000))
    throw new AuthProblem(503,'UNSUPPORTED_NATIVE_FORM_SCHEMA');
  const fields=obj.fields as Field[];
  if(new Set(fields.map(f=>f.id)).size!==fields.length||
    fields.length>50||fields.some(f=>!f.id||f.id.length>128))
    throw new AuthProblem(503,'UNSUPPORTED_NATIVE_FORM_SCHEMA');
  return fields;
};
export function validateAnswer(kind:string,schemaRaw:unknown,answer:unknown):Answer{
  if(!isObject(answer))throw new AuthProblem(400,'INVALID_ANSWER');
  const schema=parseJson(schemaRaw);
  if(kind==='PERMISSION'){
    if(Object.keys(answer).sort().join(',')!=='decision,kind' ||
      answer.kind!=='PERMISSION'||!['ALLOW_ONCE','DENY'].includes(String(answer.decision)))
      throw new AuthProblem(400,'INVALID_PERMISSION_ANSWER');
    const allowed=schema.allowedDecisions;
    if(!Array.isArray(allowed)||!allowed.includes(answer.decision)||
      allowed.some(v=>v!=='ALLOW_ONCE'&&v!=='DENY'))
      throw new AuthProblem(409,'PERMISSION_DECISION_NOT_ALLOWED');
    return{kind:'PERMISSION',decision:answer.decision as 'ALLOW_ONCE'|'DENY'};
  }
  if(kind!=='FORM'||answer.kind!=='FORM'||typeof answer.cancel!=='boolean'||
    !isObject(answer.values)||Object.keys(answer).sort().join(',')!=='cancel,kind,values')
    throw new AuthProblem(400,'INVALID_FORM_ANSWER');
  const fields=asFormFields(schema);
  const values=answer.values;
  if(answer.cancel) {
    if(Object.keys(values).length)throw new AuthProblem(400,'CANCEL_FORM_VALUES_FORBIDDEN');
    return {kind:'FORM',cancel:true,values:{}};
  }
  const expected=new Map(fields.map(f=>[f.id,f]));
  if(Object.keys(values).some(k=>!expected.has(k)))throw new AuthProblem(400,'UNKNOWN_FORM_FIELD');
  for(const field of fields){
    const value=values[field.id];
    if(value===undefined){
      if(field.required)throw new AuthProblem(400,'REQUIRED_FORM_FIELD_MISSING');
      continue;
    }
    if(field.type==='TEXT'){
      if(typeof value!=='string'||Array.from(value).length>field.maxLength||
        (field.required&&!value.trim()))throw new AuthProblem(400,'INVALID_FORM_TEXT');
    }else if(field.type==='BOOLEAN'){
      if(typeof value!=='boolean')throw new AuthProblem(400,'INVALID_FORM_BOOLEAN');
    }else if(field.type==='SINGLE_SELECT'){
      if(typeof value!=='string'||!field.options.includes(value))
        throw new AuthProblem(400,'INVALID_FORM_OPTION');
    }else if(!Array.isArray(value)||value.length>field.options.length||
      !value.every(v=>typeof v==='string'&&field.options.includes(v))||
      new Set(value).size!==value.length||(field.required&&value.length===0))
      throw new AuthProblem(400,'INVALID_FORM_MULTI_SELECT');
  }
  return {kind:'FORM',cancel:false,
    values:values as Record<string,string|boolean|string[]>};
}
function mappedInteraction(row:RowDataPacket):DTO<'Interaction'>{
  const schema=parseJson(row.schema_json);
  const common={id:String(row.id),taskId:String(row.task_id),runId:String(row.run_id),
    revision:Number(row.revision),status:row.status,expiresAt:String(row.expires_at).replace(' ','T')+'Z'};
  if(row.kind==='PERMISSION'){
    const allowed=schema.allowedDecisions;
    if(typeof schema.tool!=='string'||typeof schema.requestText!=='string'||
      !Array.isArray(allowed)||!allowed.every(x=>x==='ALLOW_ONCE'||x==='DENY'))
      throw new AuthProblem(503,'UNSUPPORTED_NATIVE_PERMISSION_SCHEMA');
    return {...common,kind:'PERMISSION',tool:schema.tool,requestText:schema.requestText,
      allowedDecisions:allowed as Array<'ALLOW_ONCE'|'DENY'>};
  }
  if(row.kind==='FORM'){
    if(typeof schema.title!=='string')throw new AuthProblem(503,'UNSUPPORTED_NATIVE_FORM_SCHEMA');
    return {...common,kind:'FORM',title:schema.title,fields:asFormFields(schema)};
  }
  throw new AuthProblem(503,'UNSUPPORTED_INTERACTION_KIND');
}
export async function listPendingInteractions(pool:Pool,ownerId:string,taskId:string):Promise<DTO<'Interaction'>[]>{
  checkUuid(taskId);
  const [owned]=await pool.execute<RowDataPacket[]>(
    'SELECT id FROM wh_tasks WHERE id=? AND owner_id=?',[taskId,ownerId]);
  if(owned.length!==1)throw new AuthProblem(404,'NOT_FOUND');
  const [rows]=await pool.execute<RowDataPacket[]>(
    "SELECT i.* FROM wh_interactions i JOIN wh_runs r ON r.id=i.run_id "+
    "WHERE i.task_id=? AND r.run_number=(SELECT MAX(x.run_number) FROM wh_runs x WHERE x.task_id=?) "+
    "AND i.status IN ('PENDING','ANSWERING','UNKNOWN') ORDER BY i.created_at ASC,i.id ASC",
    [taskId,taskId]);
  return rows.map(mappedInteraction);
}
export async function answerInteraction(input:{
  pool:Pool;ownerId:string;interactionId:string;operationId:string;revision:number;answer:unknown;
}):Promise<DTO<'Operation'>>{
  checkUuid(input.interactionId);checkUuid(input.operationId);
  if(!Number.isInteger(input.revision)||input.revision<1)throw new AuthProblem(400,'REVISION_INVALID');
  // Do not accept arbitrary schemas from the client: only persisted trusted native request data.
  const requestHash=digestOperation('INTERACTION_ANSWER',{
    interactionId:input.interactionId,revision:input.revision,answer:input.answer,
  });
  return withTransaction(input.pool,async tx=>{
    const [slot]=await tx.execute<RowDataPacket[]>(
      "SELECT run_id,quarantined,(lease_until>UTC_TIMESTAMP(3)) AS valid FROM wh_executor_slots "+
      "WHERE slot_key='local-default' FOR UPDATE");
    if(slot.length!==1)throw new AuthProblem(503,'EXECUTOR_SLOT_MISSING');
    const old=await findOperation(tx,input.ownerId,input.operationId);
    if(old){
      if(old.action!=='INTERACTION_ANSWER'||old.requestSha256!==requestHash)
        throw new AuthProblem(409,'OPERATION_CONFLICT');
      const {action,requestSha256,...original}=old;
      return original;
    }
    const [rows]=await tx.execute<RowDataPacket[]>(
      "SELECT i.*,r.status AS run_status,r.run_number,t.owner_id,"+
      "(i.expires_at>UTC_TIMESTAMP(3)) AS not_expired "+
      "FROM wh_interactions i JOIN wh_tasks t ON t.id=i.task_id "+
      "JOIN wh_runs r ON r.id=i.run_id AND r.task_id=i.task_id "+
      "WHERE i.id=? AND t.owner_id=? FOR UPDATE",
      [input.interactionId,input.ownerId]);
    if(rows.length!==1)throw new AuthProblem(404,'NOT_FOUND');
    const item=rows[0];
    if(item.status!=='PENDING'||Number(item.revision)!==input.revision)
      throw new AuthProblem(409,'INTERACTION_NOT_CURRENT');
    if(Number(item.not_expired)!==1)throw new AuthProblem(409,'INTERACTION_EXPIRED');
    if(!['WAITING_INPUT','WAITING_PERMISSION'].includes(item.run_status)||
      String(slot[0].run_id)!==String(item.run_id)||
      Number(slot[0].quarantined)!==0||Number(slot[0].valid)!==1)
      throw new AuthProblem(409,'EXECUTION_NOT_ANSWERABLE');
    const [latest]=await tx.execute<RowDataPacket[]>(
      'SELECT id FROM wh_runs WHERE task_id=? ORDER BY run_number DESC LIMIT 1',
      [item.task_id]);
    if(latest.length!==1||latest[0].id!==item.run_id)
      throw new AuthProblem(409,'STALE_INTERACTION_RUN');
    const validated=validateAnswer(String(item.kind),item.schema_json,input.answer);
    const operationDbId=randomUUID(),taskId=String(item.task_id),runId=String(item.run_id);
    await tx.execute(
      "UPDATE wh_interactions SET status='ANSWERING',answer_json=?,updated_at=UTC_TIMESTAMP(3) "+
      "WHERE id=? AND revision=? AND status='PENDING'",
      [JSON.stringify(validated),input.interactionId,input.revision]);
    await tx.execute(
      "INSERT INTO wh_operations(id,owner_id,operation_key,action,request_sha256,task_id,run_id,"+
      "status,result_json,created_at,updated_at) VALUES (?,?,?,?,?,?,?,'ACCEPTED',?,"+
      "UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))",
      [operationDbId,input.ownerId,input.operationId,'INTERACTION_ANSWER',requestHash,
        taskId,runId,JSON.stringify({interactionId:input.interactionId,revision:input.revision})]);
    await tx.execute(
      "INSERT INTO wh_outbox(id,operation_id,task_id,run_id,action,payload,status,available_at,"+
      "created_at,updated_at) VALUES (?,?,?,?,'ANSWER',?,'PENDING',UTC_TIMESTAMP(3),"+
      "UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))",
      [randomUUID(),operationDbId,taskId,runId,
        JSON.stringify({interactionId:input.interactionId,revision:input.revision,
          nativeRequestId:String(item.native_request_id),answer:validated})]);
    return {operationId:input.operationId,status:'ACCEPTED',taskId,runId,
      errorCode:null,result:{interactionId:input.interactionId,revision:input.revision}};
  });
}
