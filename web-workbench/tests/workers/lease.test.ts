import { describe,it } from 'node:test';
import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import type { Pool,PoolConnection,RowDataPacket } from 'mysql2/promise';
import { claimNext,renewLease,beginNativeSend,markNativeAccepted,quarantineUnknown,
  LeaseError } from '../../packages/storage/src/slots.ts';
import { requestStop } from '../../apps/server/src/modules/runs/stop.ts';

type RunState='QUEUED'|'PREPARING'|'RUNNING'|'STOP_REQUESTED'|'RECOVERING'|'STOPPED';
function fixture(opts:{occupied?:boolean;expired?:boolean;quarantined?:boolean}={}){
  const runId=randomUUID(),taskId=randomUUID(),outboxId=randomUUID(),ownerId=randomUUID();
  const slot={run_id:opts.occupied?runId:null as string|null,
    lease_owner:opts.occupied?'old-worker':null as string|null,
    fence:'5',quarantined:opts.quarantined?1:0,valid:opts.expired?0:1};
  let runStatus:RunState=opts.occupied?'RUNNING':'QUEUED';
  let dispatch='NOT_SENT',boxStatus=opts.occupied?'SENDING':'PENDING';
  let seq=2n,commitCount=0,rollbackCount=0,insertedOperations=0;
  const commands:string[]=[];
  const execute=async(sql:string,values:unknown[]=[])=>{
    commands.push(sql);
    if(sql.includes('FROM wh_executor_slots')&&sql.includes('FOR UPDATE')){
      return [[{...slot}],[]];
    }
    if(sql.includes('FROM wh_outbox o')&&sql.includes('SKIP LOCKED'))
      return [boxStatus==='PENDING'?[{id:outboxId,task_id:taskId,run_id:runId}]:[],[]];
    if(sql.includes('SELECT next_event_seq FROM wh_tasks'))
      return [[{next_event_seq:seq.toString()}],[]];
    if(sql.includes('SELECT status FROM wh_runs WHERE id='))
      return [[{status:runStatus}],[]];
    if(sql.includes('FROM wh_operations WHERE owner_id='))
      return [[],[]];
    if(sql.includes('JOIN wh_tasks t ON t.id=r.task_id'))
      return [[{id:runId,task_id:taskId,status:runStatus,dispatch_state:dispatch}],[]];
    if(sql.includes('UPDATE wh_executor_slots')){
      if(sql.includes('quarantined=1'))slot.quarantined=1;
      if(sql.includes('SET run_id=?')){
        slot.run_id=String(values[0]);slot.lease_owner=String(values[1]);
        slot.fence=String(values[2]);slot.valid=1;
      }
      return [{affectedRows:1},[]];
    }
    if(sql.includes('UPDATE wh_runs')){
      if(sql.includes("status='PREPARING'")){runStatus='PREPARING';return [{affectedRows:1},[]];}
      if(sql.includes("dispatch_state='SENDING'")){
        if(dispatch!=='NOT_SENT')return [{affectedRows:0},[]];
        dispatch='SENDING';return [{affectedRows:1},[]];
      }
      if(sql.includes("status='RUNNING'")){
        if(dispatch!=='SENDING')return [{affectedRows:0},[]];
        dispatch='ACCEPTED';runStatus='RUNNING';return [{affectedRows:1},[]];
      }
      if(sql.includes("status='RECOVERING'")){runStatus='RECOVERING';dispatch='UNKNOWN';}
      if(sql.includes("status='STOPPED'"))runStatus='STOPPED';
      if(sql.includes("status='STOP_REQUESTED'"))runStatus='STOP_REQUESTED';
      return [{affectedRows:1},[]];
    }
    if(sql.includes('UPDATE wh_outbox')){
      if(sql.includes("status='CLAIMED'"))boxStatus='CLAIMED';
      if(sql.includes("status='SENDING'"))boxStatus='SENDING';
      if(sql.includes("status='SENT'"))boxStatus='SENT';
      if(sql.includes("status='UNKNOWN'"))boxStatus='UNKNOWN';
      if(sql.includes("status='CANCELLED'")){boxStatus='CANCELLED';return [{affectedRows:1},[]];}
      return [{affectedRows:1},[]];
    }
    if(sql.includes('UPDATE wh_tasks SET next_event_seq=')){seq=BigInt(String(values[0]));return [{affectedRows:1},[]];}
    if(sql.includes('INSERT INTO wh_operations'))insertedOperations++;
    return [{affectedRows:1},[]];
  };
  const tx={execute,query:async()=>[[],[]],beginTransaction:async()=>{},
    commit:async()=>{commitCount++;},rollback:async()=>{rollbackCount++;},release:()=>{}} as unknown as PoolConnection;
  const pool={getConnection:async()=>tx,execute} as unknown as Pool;
  return {pool,slot,runId,taskId,outboxId,ownerId,commands,
    get runStatus(){return runStatus;},get dispatch(){return dispatch;},
    get boxStatus(){return boxStatus;},get commits(){return commitCount;},
    get operations(){return insertedOperations;},get rollbacks(){return rollbackCount;}};
}
describe('T06 leased outbox safety (deterministic SQL contract, not MySQL)',()=>{
  it('claims only one pending run, records fence and PREPARING, renews and accepts once',async()=>{
    const f=fixture(),lease=await claimNext(f.pool,'worker-1');
    assert.ok(lease);
    assert.equal(lease.fence,'6');assert.equal(f.slot.run_id,f.runId);
    assert.equal(f.runStatus,'PREPARING');
    assert.equal(await claimNext(f.pool,'worker-2'),null);
    await renewLease(f.pool,lease);
    await beginNativeSend(f.pool,lease);
    assert.equal(f.dispatch,'SENDING');
    await markNativeAccepted(f.pool,lease,'native-input-1');
    assert.equal(f.runStatus,'RUNNING');
    assert.equal(f.dispatch,'ACCEPTED');
    assert.equal(f.boxStatus,'SENT');
    await assert.rejects(()=>beginNativeSend(f.pool,lease),e=>e instanceof LeaseError);
    assert.ok(f.commands.some(s=>s.includes('FOR UPDATE SKIP LOCKED')));
    assert.ok(f.commands.some(s=>s.includes('DATE_ADD(UTC_TIMESTAMP(3), INTERVAL 30 SECOND)')));
  });
  it('never reclaims an occupied expired slot; it quarantines instead',async()=>{
    const f=fixture({occupied:true,expired:true});
    assert.equal(await claimNext(f.pool,'replacement'),null);
    assert.equal(f.slot.quarantined,1);
    assert.equal(f.slot.run_id,f.runId);
    assert.equal(f.slot.lease_owner,'old-worker');
  });
  it('rejects stale fence and does not send the native command',async()=>{
    const f=fixture(),lease=await claimNext(f.pool,'worker-1');
    assert.ok(lease);
    f.slot.fence='999';
    await assert.rejects(()=>beginNativeSend(f.pool,lease),
      e=>e instanceof LeaseError&&e.code==='LEASE_FENCE_LOST');
    assert.equal(f.dispatch,'NOT_SENT');
  });
  it('unknown native response quarantines outbox and run; does not restart',async()=>{
    const f=fixture(),lease=await claimNext(f.pool,'worker-1');
    assert.ok(lease);
    await beginNativeSend(f.pool,lease);
    await quarantineUnknown(f.pool,lease,'NATIVE_RESPONSE_UNKNOWN');
    assert.equal(f.runStatus,'RECOVERING');
    assert.equal(f.dispatch,'UNKNOWN');
    assert.equal(f.boxStatus,'UNKNOWN');
    assert.equal(f.slot.quarantined,1);
    assert.equal(await claimNext(f.pool,'replacement'),null);
  });
  it('queued stop cancels dispatch and commits a final stopped event without session',async()=>{
    const f=fixture();
    const result=await requestStop(f.pool,{
      ownerId:f.ownerId,runId:f.runId,operationId:randomUUID(),
    });
    assert.equal(result.status,'SUCCEEDED');
    assert.deepEqual(result.result,{queuedCancelled:true});
    assert.equal(f.runStatus,'STOPPED');
    assert.equal(f.boxStatus,'CANCELLED');
    assert.equal(f.operations,1);
    assert.ok(!f.commands.some(s=>s.includes("'INTERRUPT'")));
  });
  it('active stop persists STOP_REQUESTED and interrupt outbox but not STOPPED',async()=>{
    const f=fixture({occupied:true});
    const result=await requestStop(f.pool,{
      ownerId:f.ownerId,runId:f.runId,operationId:randomUUID(),
    });
    assert.equal(result.status,'ACCEPTED');
    assert.equal(f.runStatus,'STOP_REQUESTED');
    assert.ok(f.commands.some(s=>s.includes("'INTERRUPT'")));
    assert.notEqual(f.runStatus,'STOPPED');
  });
});
