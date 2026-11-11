import {randomUUID} from 'node:crypto';
import type {Pool,RowDataPacket} from 'mysql2/promise';
import type {DTO} from '../../../../../packages/contracts/src/domain.ts';
import type {OpenCodeAdapter} from '../../../../../packages/contracts/src/ports.ts';
import {withTransaction} from '../../../../../packages/storage/src/db.ts';
import {appendEvent} from '../../../../../packages/storage/src/events.ts';
import {digestOperation,findOperation} from '../../../../../packages/storage/src/operations.ts';
import {AuthProblem} from '../../auth.ts';
import {checkUuid} from '../repositories/service.ts';

export async function retryRun(pool:Pool,ownerId:string,runId:string,operationId:string,adapter?:OpenCodeAdapter):Promise<DTO<'Operation'>>{
  checkUuid(runId);checkUuid(operationId);
  const hash=digestOperation('RUN_RETRY',{runId});
  function replay(old:NonNullable<Awaited<ReturnType<typeof findOperation>>>){
    if(old.action!=='RUN_RETRY'||old.requestSha256!==hash)throw new AuthProblem(409,'OPERATION_CONFLICT');
    const {action,requestSha256,...op}=old;return op;
  }
  const prior=await findOperation(pool,ownerId,operationId);
  if(prior)return replay(prior);
  const [rows]=await pool.execute<RowDataPacket[]>('SELECT r.task_id,r.status,b.service_identity,b.native_session_id,b.location_key '+
    'FROM wh_runs r JOIN wh_tasks t ON t.id=r.task_id LEFT JOIN wh_session_bindings b ON b.task_id=t.id '+
    'WHERE r.id=? AND t.owner_id=?',[runId,ownerId]);
  if(!rows.length)throw new AuthProblem(404,'NOT_FOUND');
  const base=rows[0],taskId=String(base.task_id);
  if(!['FAILED','STOPPED'].includes(base.status))throw new AuthProblem(409,'RUN_NOT_RETRYABLE');
  if(!adapter)throw new AuthProblem(409,'EXECUTION_UNCERTAIN');
  if(base.native_session_id){
    const inspection=await adapter.inspect({taskId,sessionId:base.native_session_id,locationKey:base.location_key,
      serviceIdentity:base.service_identity}).catch(()=>null);
    if(inspection?.active!==false||inspection.pendingInputs!==0||inspection.pendingInteractions!==0||
      inspection.externalMutation||inspection.acceptedInputIds.length)throw new AuthProblem(409,'EXECUTION_UNCERTAIN');
  }
  return withTransaction(pool,async tx=>{
    const [slots]=await tx.execute<RowDataPacket[]>("SELECT run_id,quarantined FROM wh_executor_slots WHERE slot_key='local-default' FOR UPDATE");
    const old=await findOperation(tx,ownerId,operationId);
    if(old)return replay(old);
    if(slots.length!==1||slots[0].quarantined||slots[0].run_id===runId)throw new AuthProblem(409,'EXECUTION_UNCERTAIN');
    const [latest]=await tx.execute<RowDataPacket[]>('SELECT r.*,t.next_run_number,s.status AS snapshot_status FROM wh_runs r JOIN wh_tasks t ON t.id=r.task_id '+
      'JOIN wh_scope_snapshots s ON s.task_id=t.id WHERE t.id=? AND t.owner_id=? ORDER BY r.run_number DESC LIMIT 1 FOR UPDATE',[taskId,ownerId]);
    const run=latest[0];
    if(!run||run.id!==runId||!['FAILED','STOPPED'].includes(run.status))throw new AuthProblem(409,'STALE_RETRY');
    if(!base.native_session_id && !(run.kind==='REVIEW'&&run.snapshot_status==='PENDING'&&run.dispatch_state==='NOT_SENT'))
      throw new AuthProblem(409,'EXECUTION_UNCERTAIN');
    const [count]=await tx.execute<RowDataPacket[]>("SELECT COUNT(*) AS n FROM wh_runs WHERE status='QUEUED'");
    if(Number(count[0].n)>=20)throw new AuthProblem(429,'QUEUE_FULL');
    const id=randomUUID(),op=randomUUID();
    await tx.execute("INSERT INTO wh_runs(id,task_id,run_number,kind,retry_of_run_id,asset_versions,created_at,updated_at) "+
      "SELECT ?,task_id,?,kind,id,asset_versions,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3) FROM wh_runs WHERE id=?",
      [id,run.next_run_number,runId]);
    await tx.execute('UPDATE wh_tasks SET next_run_number=next_run_number+1,updated_at=UTC_TIMESTAMP(3) WHERE id=?',[taskId]);
    const result={taskId,runId:id,retryOfRunId:runId};
    await tx.execute("INSERT INTO wh_operations(id,owner_id,operation_key,action,request_sha256,task_id,run_id,status,result_json,created_at,updated_at) "+
      "VALUES (?,?,?,'RUN_RETRY',?,?,?,'ACCEPTED',?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))",[op,ownerId,operationId,hash,taskId,id,JSON.stringify(result)]);
    await tx.execute("INSERT INTO wh_outbox(id,operation_id,task_id,run_id,action,payload,available_at,created_at,updated_at) "+
      "VALUES (?,?,?,?,'DISPATCH',?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))",[randomUUID(),op,taskId,id,JSON.stringify(result)]);
    await appendEvent(tx,{taskId,runId:id,type:'run.status',data:{status:'QUEUED',previousStatus:null,errorCode:null}});
    return {operationId,taskId,runId:id,status:'ACCEPTED',errorCode:null,result};
  });
}
