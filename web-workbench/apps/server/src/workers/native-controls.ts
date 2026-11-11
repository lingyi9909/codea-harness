import { isDeepStrictEqual } from 'node:util';
import type { Pool, ResultSetHeader, RowDataPacket } from 'mysql2/promise';
import type { DTO, SessionBinding } from '../../../../packages/contracts/src/domain.ts';
import type { OpenCodeAdapter } from '../../../../packages/contracts/src/ports.ts';
import { withTransaction } from '../../../../packages/storage/src/db.ts';
import { appendEvent } from '../../../../packages/storage/src/events.ts';
import { assertFencedLease, type Lease } from '../../../../packages/storage/src/slots.ts';
import { validateAnswer } from '../modules/interactions/service.ts';

type Context = {pool:Pool; lease:Lease; binding:SessionBinding; adapter:OpenCodeAdapter};
type Control = { id:string; operationId:string } & (
  { action:'INTERRUPT' } | { action:'ANSWER'; interactionId:string; answer:DTO<'AnswerRequest'>['answer'] }
);

async function claim(c:Context):Promise<Control|null>{
  return withTransaction(c.pool,async tx=>{
    await assertFencedLease(tx,c.lease);
    const [rows]=await tx.execute<RowDataPacket[]>(
      "SELECT o.id,o.operation_id,o.payload,o.action,r.status FROM wh_outbox o JOIN wh_runs r ON r.id=o.run_id "+
      "WHERE o.run_id=? AND o.task_id=? AND o.action IN ('ANSWER','INTERRUPT') AND o.status='PENDING' "+
      "AND o.available_at<=UTC_TIMESTAMP(3) ORDER BY (o.action='INTERRUPT') DESC,o.created_at,o.id LIMIT 1 FOR UPDATE",
      [c.lease.runId,c.lease.taskId]);
    if(!rows.length)return null;
    const row=rows[0],common={id:String(row.id),operationId:String(row.operation_id)};
    let control:Control;
    if(row.action==='INTERRUPT'){
      if(row.status!=='STOP_REQUESTED')throw new Error('INTERRUPT_STATE_CHANGED');
      control={...common,action:'INTERRUPT'};
    }else{
      const payload=typeof row.payload==='string'?JSON.parse(row.payload):row.payload;
      const [items]=await tx.execute<RowDataPacket[]>(
        "SELECT kind,schema_json,answer_json FROM wh_interactions WHERE id=? AND task_id=? AND run_id=? "+
        "AND revision=? AND native_request_id=? AND status='ANSWERING' AND expires_at>UTC_TIMESTAMP(3) FOR UPDATE",
        [payload.interactionId,c.lease.taskId,c.lease.runId,payload.revision,payload.nativeRequestId]);
      if(!items.length||!['WAITING_PERMISSION','WAITING_INPUT'].includes(row.status)){
        await tx.execute("UPDATE wh_outbox SET status='CANCELLED',updated_at=UTC_TIMESTAMP(3) WHERE id=?",[row.id]);
        await tx.execute("UPDATE wh_operations SET status='FAILED',error_code='INTERACTION_STALE',updated_at=UTC_TIMESTAMP(3) WHERE id=?",[row.operation_id]);
        return null;
      }
      const item=items[0];
      const answer=validateAnswer(item.kind,item.schema_json,typeof item.answer_json==='string'?JSON.parse(item.answer_json):item.answer_json);
      control={...common,action:'ANSWER',interactionId:payload.interactionId,answer};
    }
    await tx.execute("UPDATE wh_outbox SET status='SENDING',lease_owner=?,lease_fence=?,attempts=attempts+1,"+
      "updated_at=UTC_TIMESTAMP(3) WHERE id=?",[c.lease.owner,c.lease.fence,control.id]);
    return control;
  });
}

export async function dispatchControl(c:Context){
  const control=await claim(c);
  if(!control)return;
  let status:'SENT'|'FAILED'|'UNKNOWN'='UNKNOWN';
  try{
    if(control.action==='ANSWER'){
      const result=await c.adapter.answer(c.binding,control.interactionId,control.answer);
      if(result.state==='ACCEPTED')status='SENT';
      if(result.state==='REJECTED')status='FAILED';
    }else{
      const result=await c.adapter.interrupt(c.binding);
      if(result.responseReceived && typeof result.interrupted==='boolean')
        status=result.interrupted?'SENT':'FAILED';
    }
  }catch{/* Delivery is uncertain. Leave this item ineligible for automatic replay. */}
  await withTransaction(c.pool,async tx=>{
    await assertFencedLease(tx,c.lease);
    const [result]=await tx.execute<ResultSetHeader>("UPDATE wh_outbox SET status=?,updated_at=UTC_TIMESTAMP(3) "+
      "WHERE id=? AND status='SENDING' AND lease_owner=? AND lease_fence=?",[status,control.id,c.lease.owner,c.lease.fence]);
    if(result.affectedRows!==1)throw new Error('CONTROL_FENCE_CHANGED');
    await tx.execute("UPDATE wh_operations SET status=?,updated_at=UTC_TIMESTAMP(3) WHERE id=?",
      [status==='SENT'?'PROCESSING':status==='UNKNOWN'?'UNKNOWN':'FAILED',control.operationId]);
    if(control.action==='ANSWER' && status!=='SENT')
      await tx.execute("UPDATE wh_interactions SET status='UNKNOWN',updated_at=UTC_TIMESTAMP(3) WHERE id=? AND status='ANSWERING'",
        [control.interactionId]);
  });
}

export async function syncInteractions(c:Context){
  const pending=await c.adapter.listInteractions(c.binding);
  const {taskId,runId}=c.lease;
  await withTransaction(c.pool,async tx=>{
    await assertFencedLease(tx,c.lease);
    const [runs]=await tx.execute<RowDataPacket[]>('SELECT status FROM wh_runs WHERE id=? AND task_id=? FOR UPDATE',[runId,taskId]);
    const previous=runs[0]?.status;
    if(!['RUNNING','WAITING_INPUT','WAITING_PERMISSION','STOP_REQUESTED'].includes(previous))
      throw new Error('INTERACTION_SYNC_STATE_CHANGED');
    const [rows]=await tx.execute<RowDataPacket[]>('SELECT id,kind,revision,status,native_request_id,schema_json FROM wh_interactions WHERE run_id=? FOR UPDATE',[runId]);
    for(const i of pending){
      if(i.taskId!==taskId||i.runId!==runId||i.status!=='PENDING'||!i.nativeRequestId)
        throw new Error('INTERACTION_SCOPE_CHANGED');
      const schema=i.kind==='FORM'?{title:i.title,fields:i.fields}:
        {tool:i.tool,requestText:i.requestText,allowedDecisions:i.allowedDecisions};
      const row=rows.find(r=>r.id===i.id);
      if(row){
        const stored=typeof row.schema_json==='string'?JSON.parse(row.schema_json):row.schema_json;
        if(row.kind!==i.kind||row.revision!==i.revision||row.native_request_id!==i.nativeRequestId||
            !isDeepStrictEqual(stored,schema)||!['PENDING','ANSWERING','UNKNOWN'].includes(row.status))
          throw new Error('NATIVE_REQUEST_CHANGED');
        continue;
      }
      const expiry=new Date(i.expiresAt);
      if(!Number.isFinite(expiry.getTime())) throw new Error('INVALID_NATIVE_EXPIRY');
      await tx.execute("INSERT INTO wh_interactions(id,task_id,run_id,native_request_id,kind,revision,status,schema_json,"+
        "created_at,updated_at,expires_at) VALUES (?,?,?,?,?,?,'PENDING',?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3),"+
        "LEAST(?,DATE_ADD(UTC_TIMESTAMP(3), INTERVAL 15 MINUTE)))",
        [i.id,taskId,runId,i.nativeRequestId,i.kind,i.revision,JSON.stringify(schema),expiry]);
      await appendEvent(tx,{taskId,runId,type:'interaction.pending',data:{interactionId:i.id,revision:i.revision,status:'PENDING'}});
    }
    for(const row of rows){
      if(!['PENDING','ANSWERING','UNKNOWN'].includes(row.status)||pending.some(i=>i.id===row.id))continue;
      const [boxes]=await tx.execute<RowDataPacket[]>(
        "SELECT operation_id,status FROM wh_outbox WHERE run_id=? AND action='ANSWER' AND "+
        "JSON_UNQUOTE(JSON_EXTRACT(payload,'$.interactionId'))=? ORDER BY created_at DESC LIMIT 1",[runId,row.id]);
      const answered=boxes[0]?.status==='SENT';
      if(!answered && row.status!=='PENDING')throw new Error('NATIVE_ANSWER_UNCONFIRMED');
      const status=answered?'ANSWERED':'CANCELLED';
      await tx.execute('UPDATE wh_interactions SET status=?,resolved_at=UTC_TIMESTAMP(3),updated_at=UTC_TIMESTAMP(3) WHERE id=?',[status,row.id]);
      if(answered) await tx.execute("UPDATE wh_operations SET status='SUCCEEDED',updated_at=UTC_TIMESTAMP(3) WHERE id=?",[boxes[0].operation_id]);
      await appendEvent(tx,{taskId,runId,type:'interaction.resolved',data:{interactionId:row.id,revision:row.revision,status}});
    }
    const next=pending.some(i=>i.kind==='PERMISSION')?'WAITING_PERMISSION':pending.length?'WAITING_INPUT':'RUNNING';
    if(previous!=='STOP_REQUESTED' && previous!==next){
      await tx.execute('UPDATE wh_runs SET status=?,updated_at=UTC_TIMESTAMP(3) WHERE id=?',[next,runId]);
      await appendEvent(tx,{taskId,runId,type:'run.status',data:{previousStatus:previous,status:next,errorCode:null}});
    }
  });
}
