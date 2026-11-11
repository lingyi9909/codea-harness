import type {RowDataPacket,ResultSetHeader} from 'mysql2/promise';
import type {SchedulerConfig} from './scheduler.ts';
import type {Lease} from '../../../../packages/storage/src/slots.ts';
import {withTransaction} from '../../../../packages/storage/src/db.ts';
import {appendEvent} from '../../../../packages/storage/src/events.ts';

export async function resumeExpired(config:SchedulerConfig){
  if(!config.adapter.reattach)return null;
  const query="SELECT r.id,r.task_id,r.native_input_id,s.fence,o.id AS outbox_id,"+
    "b.service_identity,b.native_session_id,b.location_key FROM wh_executor_slots s "+
    "JOIN wh_runs r ON r.id=s.run_id JOIN wh_session_bindings b ON b.task_id=r.task_id "+
    "JOIN wh_outbox o ON o.run_id=r.id AND o.action='DISPATCH' AND o.status='SENT' "+
    "WHERE s.slot_key='local-default' AND s.quarantined=1 AND s.lease_until<=UTC_TIMESTAMP(3) "+
    "AND r.status='RECOVERING' AND r.error_code='LEASE_EXPIRED' AND r.dispatch_state='ACCEPTED' "+
    "AND r.native_input_id IS NOT NULL AND b.native_version=? AND NOT EXISTS "+
    "(SELECT 1 FROM wh_outbox u WHERE u.run_id=r.id AND u.status IN ('SENDING','UNKNOWN')) "+
    "AND NOT EXISTS (SELECT 1 FROM wh_interactions i WHERE i.run_id=r.id AND i.status IN ('PENDING','ANSWERING','UNKNOWN'))";
  const [rows]=await config.pool.execute<RowDataPacket[]>(query,[config.nativeVersion]);
  if(rows.length!==1)return null;
  const row=rows[0],runId=String(row.id),taskId=String(row.task_id),inputId=String(row.native_input_id);
  const binding={taskId,sessionId:String(row.native_session_id),serviceIdentity:String(row.service_identity),locationKey:String(row.location_key)};
  if(!await config.adapter.reattach(binding,runId,inputId))return null;
  return withTransaction(config.pool,async tx=>{
    const [current]=await tx.execute<RowDataPacket[]>(query+' FOR UPDATE',[config.nativeVersion]);
    if(current.length!==1||current[0].id!==runId||String(current[0].fence)!==String(row.fence)||
      current[0].native_input_id!==inputId||current[0].native_session_id!==binding.sessionId||
      current[0].location_key!==binding.locationKey||current[0].service_identity!==binding.serviceIdentity)return null;
    const fence=(BigInt(row.fence)+1n).toString();
    const lease:Lease={slotKey:'local-default',runId,taskId,fence,owner:config.workerId,outboxId:String(row.outbox_id)};
    const [slot]=await tx.execute<ResultSetHeader>("UPDATE wh_executor_slots SET quarantined=0,fence=?,lease_owner=?,"+
      "lease_until=DATE_ADD(UTC_TIMESTAMP(3),INTERVAL 30 SECOND),updated_at=UTC_TIMESTAMP(3) WHERE slot_key=? AND fence=?",
      [fence,lease.owner,lease.slotKey,row.fence]);
    if(slot.affectedRows!==1)throw new Error('RECOVERY_FENCE_CHANGED');
    const [stop]=await tx.execute<RowDataPacket[]>("SELECT id FROM wh_outbox WHERE run_id=? AND action='INTERRUPT' LIMIT 1",[runId]);
    const status=stop.length?'STOP_REQUESTED':'RUNNING';
    await tx.execute('UPDATE wh_runs SET status=?,lease_fence=?,error_code=NULL,updated_at=UTC_TIMESTAMP(3) WHERE id=?',[status,fence,runId]);
    await appendEvent(tx,{taskId,runId,type:'run.status',data:{previousStatus:'RECOVERING',status,errorCode:null}});
    return {lease,binding,inputId};
  });
}
