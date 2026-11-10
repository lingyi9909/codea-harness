import { randomUUID } from 'node:crypto';
import type { Pool, RowDataPacket, ResultSetHeader } from 'mysql2/promise';
import type { DTO } from '../../../../../packages/contracts/src/domain.ts';
import { withTransaction } from '../../../../../packages/storage/src/db.ts';
import { appendEvent } from '../../../../../packages/storage/src/events.ts';
import { digestOperation, findOperation } from '../../../../../packages/storage/src/operations.ts';
import { AuthProblem } from '../../auth.ts';
import { checkUuid } from '../repositories/service.ts';

const active=new Set(['PREPARING','RUNNING','WAITING_INPUT','WAITING_PERMISSION','COLLECTING']);
export async function requestStop(pool:Pool,input:{ownerId:string;runId:string;operationId:string}):
  Promise<DTO<'Operation'>>{
  checkUuid(input.operationId);checkUuid(input.runId);
  const hash=digestOperation('RUN_STOP',{runId:input.runId});
  return withTransaction(pool,async tx=>{
    // Same lock order as dispatch and task creation.
    const [slots]=await tx.execute<RowDataPacket[]>(
      "SELECT run_id,quarantined FROM wh_executor_slots WHERE slot_key='local-default' FOR UPDATE");
    if(slots.length!==1)throw new AuthProblem(503,'EXECUTOR_SLOT_MISSING');
    const old=await findOperation(tx,input.ownerId,input.operationId);
    if(old){
      if(old.action!=='RUN_STOP'||old.requestSha256!==hash)throw new AuthProblem(409,'OPERATION_CONFLICT');
      const {action,requestSha256,...op}=old;
      return op;
    }
    const [runs]=await tx.execute<RowDataPacket[]>(
      "SELECT r.id,r.task_id,r.status,r.dispatch_state FROM wh_runs r "+
      "JOIN wh_tasks t ON t.id=r.task_id WHERE r.id=? AND t.owner_id=? FOR UPDATE",
      [input.runId,input.ownerId]);
    if(!runs.length)throw new AuthProblem(404,'NOT_FOUND');
    const r=runs[0],status=String(r.status),taskId=String(r.task_id);
    if(['SUCCEEDED','FAILED','STOPPED'].includes(status))
      throw new AuthProblem(409,'RUN_ALREADY_TERMINAL');
    if(status==='STOP_REQUESTED')throw new AuthProblem(409,'STOP_ALREADY_REQUESTED');
    if(status==='RECOVERING')throw new AuthProblem(409,'RUN_REQUIRES_RECONCILIATION');
    if(status!=='QUEUED'&&!active.has(status))throw new AuthProblem(409,'RUN_STATUS_UNKNOWN');
    const queued=status==='QUEUED';
    if(queued){
      // Any claimed or sending outbox violates queued cancellation safety.
      const [result]=await tx.execute<ResultSetHeader>(
        "UPDATE wh_outbox SET status='CANCELLED',updated_at=UTC_TIMESTAMP(3) "+
        "WHERE run_id=? AND action='DISPATCH' AND status='PENDING'",
        [input.runId]);
      if(result.affectedRows!==1)throw new AuthProblem(409,'QUEUE_CANCEL_RACE');
      await tx.execute("UPDATE wh_runs SET status='STOPPED',finished_at=UTC_TIMESTAMP(3),"+
        "updated_at=UTC_TIMESTAMP(3) WHERE id=? AND status='QUEUED'",[input.runId]);
    }else{
      if(String(slots[0].run_id)!==input.runId||Number(slots[0].quarantined)!==0)
        throw new AuthProblem(409,'EXECUTOR_RECONCILIATION_REQUIRED');
      await tx.execute("UPDATE wh_runs SET status='STOP_REQUESTED',"+
        "updated_at=UTC_TIMESTAMP(3) WHERE id=? AND status=?",[input.runId,status]);
    }
    const operationDbId=randomUUID(),result={queuedCancelled:queued};
    await tx.execute(
      "INSERT INTO wh_operations(id,owner_id,operation_key,action,request_sha256,task_id,run_id,"+
      "status,result_json,created_at,updated_at) VALUES (?,?,?,?,?,?,?,? ,?,"+
      "UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))",
      [operationDbId,input.ownerId,input.operationId,'RUN_STOP',hash,taskId,input.runId,
        queued?'SUCCEEDED':'ACCEPTED',JSON.stringify(result)]);
    if(!queued){
      await tx.execute(
        "INSERT INTO wh_outbox(id,operation_id,task_id,run_id,action,payload,status,"+
        "available_at,created_at,updated_at) VALUES (?,?,?,?,'INTERRUPT',?,'PENDING',"+
        "UTC_TIMESTAMP(3),UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))",
        [randomUUID(),operationDbId,taskId,input.runId,JSON.stringify({runId:input.runId})]);
    }
    await appendEvent(tx,{taskId,runId:input.runId,type:'run.status',
      data:{status:queued?'STOPPED':'STOP_REQUESTED',previousStatus:status,errorCode:null}});
    return{operationId:input.operationId,taskId,runId:input.runId,
      status:queued?'SUCCEEDED':'ACCEPTED',errorCode:null,result};
  });
}
