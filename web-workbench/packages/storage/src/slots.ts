import type { Pool, PoolConnection, RowDataPacket, ResultSetHeader } from 'mysql2/promise';
import { withTransaction } from './db.ts';
import { appendEvent } from './events.ts';

export class LeaseError extends Error {
  readonly code: string;
  constructor(code: string) { super(code); this.code = code; }
}
export type Lease = {
  slotKey: 'local-default'; runId: string; taskId: string;
  outboxId: string; owner: string; fence: string;
};
type SlotRow = RowDataPacket & {
  run_id: string | null; lease_owner: string | null; fence: string;
  quarantined: number | boolean; valid: number | string | null;
};
async function lockedSlot(tx: PoolConnection): Promise<SlotRow> {
  const [rows] = await tx.execute<SlotRow[]>(
    "SELECT run_id,lease_owner,fence,quarantined,"+
    "(lease_until>UTC_TIMESTAMP(3)) AS valid FROM wh_executor_slots "+
    "WHERE slot_key='local-default' FOR UPDATE"
  );
  if(rows.length!==1)throw new LeaseError('EXECUTOR_SLOT_UNAVAILABLE');
  return rows[0];
}
export async function assertFencedLease(tx:PoolConnection,lease:Lease):Promise<void>{
  const slot=await lockedSlot(tx);
  if(slot.run_id!==lease.runId||slot.lease_owner!==lease.owner||
    String(slot.fence)!==lease.fence||Boolean(slot.quarantined)||
    Number(slot.valid)!==1)throw new LeaseError('LEASE_FENCE_LOST');
}
/** A stale run is NEVER automatically replaced. A reviewer must reconcile its native state first. */
export async function claimNext(pool:Pool,workerId:string):Promise<Lease|null>{
  if(!/^[a-zA-Z0-9._:-]{1,128}$/.test(workerId))throw new LeaseError('INVALID_WORKER_ID');
  return withTransaction(pool,async tx=>{
    const slot=await lockedSlot(tx);
    if(slot.run_id){
      if(Number(slot.valid)!==1&&!Boolean(slot.quarantined)){
        const [runs]=await tx.execute<RowDataPacket[]>(
          'SELECT task_id,status FROM wh_runs WHERE id=? FOR UPDATE',[slot.run_id]);
        if(runs.length===1 && !['SUCCEEDED','FAILED','STOPPED','RECOVERING'].includes(runs[0].status)){
          await tx.execute("UPDATE wh_runs SET status='RECOVERING',error_code='LEASE_EXPIRED',"+
            "dispatch_state=IF(dispatch_state='SENDING','UNKNOWN',dispatch_state),updated_at=UTC_TIMESTAMP(3) WHERE id=?",[slot.run_id]);
          await tx.execute("UPDATE wh_operations p JOIN wh_outbox o ON o.operation_id=p.id "+
            "SET p.status='UNKNOWN',p.error_code='LEASE_EXPIRED',p.updated_at=UTC_TIMESTAMP(3) WHERE o.run_id=? AND o.status='SENDING'",[slot.run_id]);
          await tx.execute("UPDATE wh_outbox SET status='UNKNOWN',updated_at=UTC_TIMESTAMP(3) WHERE run_id=? AND status='SENDING'",[slot.run_id]);
          await appendEvent(tx,{taskId:String(runs[0].task_id),runId:slot.run_id,type:'run.status',
            data:{status:'RECOVERING',previousStatus:runs[0].status,errorCode:'LEASE_EXPIRED'}});
        }
        await tx.execute("UPDATE wh_executor_slots SET quarantined=1,updated_at=UTC_TIMESTAMP(3) "+
          "WHERE slot_key='local-default'");
      }
      return null;
    }
    if(Boolean(slot.quarantined))return null;
    const [items]=await tx.execute<RowDataPacket[]>(
      "SELECT o.id,o.task_id,o.run_id FROM wh_outbox o "+
      "JOIN wh_runs r ON r.id=o.run_id AND r.task_id=o.task_id "+
      "WHERE o.action='DISPATCH' AND o.status='PENDING' AND "+
      "o.available_at<=UTC_TIMESTAMP(3) AND r.status='QUEUED' "+
      "ORDER BY o.created_at ASC,o.id ASC LIMIT 1 FOR UPDATE SKIP LOCKED");
    if(!items.length)return null;
    const item=items[0];
    const fence=(BigInt(String(slot.fence))+1n).toString();
    const lease:Lease={slotKey:'local-default',runId:String(item.run_id),
      taskId:String(item.task_id),outboxId:String(item.id),owner:workerId,fence};
    await tx.execute("UPDATE wh_executor_slots SET run_id=?,lease_owner=?,fence=?,"+
      "lease_until=DATE_ADD(UTC_TIMESTAMP(3), INTERVAL 30 SECOND),"+
      "quarantined=0,updated_at=UTC_TIMESTAMP(3) WHERE slot_key='local-default'",
      [lease.runId,workerId,fence]);
    const [run]=await tx.execute<ResultSetHeader>(
      "UPDATE wh_runs SET status='PREPARING',lease_fence=?,"+
      "updated_at=UTC_TIMESTAMP(3),started_at=UTC_TIMESTAMP(3) "+
      "WHERE id=? AND task_id=? AND status='QUEUED' AND dispatch_state='NOT_SENT'",
      [fence,lease.runId,lease.taskId]);
    if(run.affectedRows!==1)throw new LeaseError('RUN_CLAIM_CONFLICT');
    const [box]=await tx.execute<ResultSetHeader>(
      "UPDATE wh_outbox SET status='CLAIMED',lease_owner=?,lease_fence=?,"+
      "lease_until=DATE_ADD(UTC_TIMESTAMP(3), INTERVAL 30 SECOND),"+
      "attempts=attempts+1,updated_at=UTC_TIMESTAMP(3) "+
      "WHERE id=? AND status='PENDING'",
      [workerId,fence,lease.outboxId]);
    if(box.affectedRows!==1)throw new LeaseError('OUTBOX_CLAIM_CONFLICT');
    await appendEvent(tx,{taskId:lease.taskId,runId:lease.runId,
      type:'run.status',data:{status:'PREPARING',previousStatus:'QUEUED',errorCode:null}});
    return lease;
  });
}
/** Renew only when both lease identity and unexpired DB time are still valid. */
export async function renewLease(pool:Pool,lease:Lease):Promise<void>{
  await withTransaction(pool,async tx=>{
    await assertFencedLease(tx,lease);
    const [res]=await tx.execute<ResultSetHeader>(
      "UPDATE wh_executor_slots SET lease_until=DATE_ADD(UTC_TIMESTAMP(3), INTERVAL 30 SECOND),"+
      "updated_at=UTC_TIMESTAMP(3) WHERE slot_key=? AND run_id=? AND lease_owner=? AND fence=? "+
      "AND quarantined=0 AND lease_until>UTC_TIMESTAMP(3)",
      [lease.slotKey,lease.runId,lease.owner,lease.fence]);
    if(res.affectedRows!==1)throw new LeaseError('LEASE_RENEW_REJECTED');
    await tx.execute("UPDATE wh_outbox SET lease_until=DATE_ADD(UTC_TIMESTAMP(3), INTERVAL 30 SECOND),"+
      "updated_at=UTC_TIMESTAMP(3) WHERE id=? AND lease_owner=? AND lease_fence=?",
      [lease.outboxId,lease.owner,lease.fence]);
  });
}
/** Persist SENDING before any non-idempotent native model command. */
export async function beginNativeSend(pool:Pool,lease:Lease):Promise<void>{
  await withTransaction(pool,async tx=>{
    await assertFencedLease(tx,lease);
    const [run]=await tx.execute<ResultSetHeader>(
      "UPDATE wh_runs SET dispatch_state='SENDING',updated_at=UTC_TIMESTAMP(3) "+
      "WHERE id=? AND task_id=? AND lease_fence=? AND status='PREPARING' "+
      "AND dispatch_state='NOT_SENT'",
      [lease.runId,lease.taskId,lease.fence]);
    if(run.affectedRows!==1)throw new LeaseError('NATIVE_SEND_NOT_PERMITTED');
    const [box]=await tx.execute<ResultSetHeader>(
      "UPDATE wh_outbox SET status='SENDING',updated_at=UTC_TIMESTAMP(3) "+
      "WHERE id=? AND run_id=? AND lease_owner=? AND lease_fence=? AND status='CLAIMED'",
      [lease.outboxId,lease.runId,lease.owner,lease.fence]);
    if(box.affectedRows!==1)throw new LeaseError('OUTBOX_SEND_NOT_PERMITTED');
  });
}
/** Unknown native acceptance must be persisted and quarantined, never replayed. */
export async function quarantineUnknown(pool:Pool,lease:Lease,reason:string):Promise<void>{
  if(!/^[A-Z0-9_]{1,64}$/.test(reason))throw new LeaseError('INVALID_RECOVERY_REASON');
  await withTransaction(pool,async tx=>{
    await assertFencedLease(tx,lease);
    const [state]=await tx.execute<RowDataPacket[]>(
      'SELECT status FROM wh_runs WHERE id=? AND task_id=? FOR UPDATE',
      [lease.runId,lease.taskId]);
    if(!state.length)throw new LeaseError('RUN_NOT_FOUND');
    if(['SUCCEEDED','FAILED','STOPPED'].includes(String(state[0].status)))
      throw new LeaseError('TERMINAL_RUN_CANNOT_RECOVER');
    const previous=String(state[0].status);
    await tx.execute("UPDATE wh_operations p JOIN wh_outbox o ON p.id=o.operation_id SET p.status='UNKNOWN',"+
      "p.error_code=?,p.updated_at=UTC_TIMESTAMP(3) WHERE o.id=? AND o.status<>'SENT'",[reason,lease.outboxId]);
    await tx.execute("UPDATE wh_runs SET status='RECOVERING',dispatch_state=IF(dispatch_state='ACCEPTED','ACCEPTED','UNKNOWN'),"+
      "error_code=?,updated_at=UTC_TIMESTAMP(3) WHERE id=? AND lease_fence=?",
      [reason,lease.runId,lease.fence]);
    await tx.execute("UPDATE wh_outbox SET status='UNKNOWN',updated_at=UTC_TIMESTAMP(3) "+
      "WHERE id=? AND lease_fence=? AND status<>'SENT'",[lease.outboxId,lease.fence]);
    await tx.execute("UPDATE wh_executor_slots SET quarantined=1,updated_at=UTC_TIMESTAMP(3) "+
      "WHERE slot_key='local-default' AND fence=?",[lease.fence]);
    await appendEvent(tx,{taskId:lease.taskId,runId:lease.runId,type:'run.status',
      data:{status:'RECOVERING',previousStatus:previous as 'PREPARING'|'RUNNING'|'STOP_REQUESTED'|'RECOVERING',
        errorCode:reason}});
  });
}
export async function markNativeAccepted(pool:Pool,lease:Lease,nativeInputId:string):Promise<void>{
  if(!nativeInputId||nativeInputId.length>128)throw new LeaseError('NATIVE_INPUT_ID_INVALID');
  await withTransaction(pool,async tx=>{
    await assertFencedLease(tx,lease);
    const [run]=await tx.execute<ResultSetHeader>(
      "UPDATE wh_runs SET status='RUNNING',dispatch_state='ACCEPTED',native_input_id=?,"+
      "updated_at=UTC_TIMESTAMP(3) WHERE id=? AND task_id=? AND lease_fence=? "+
      "AND status='PREPARING' AND dispatch_state='SENDING'",
      [nativeInputId,lease.runId,lease.taskId,lease.fence]);
    if(run.affectedRows!==1)throw new LeaseError('ACCEPTANCE_NOT_ALLOWED');
    await tx.execute("UPDATE wh_outbox SET status='SENT',updated_at=UTC_TIMESTAMP(3) "+
      "WHERE id=? AND status='SENDING' AND lease_fence=?",
      [lease.outboxId,lease.fence]);
    await tx.execute("UPDATE wh_operations p JOIN wh_outbox o ON p.id=o.operation_id SET p.status='SUCCEEDED',"+
      "p.updated_at=UTC_TIMESTAMP(3) WHERE o.id=? AND o.status='SENT'",[lease.outboxId]);
    await appendEvent(tx,{taskId:lease.taskId,runId:lease.runId,
      type:'run.status',data:{status:'RUNNING',previousStatus:'PREPARING',errorCode:null}});
  });
}
