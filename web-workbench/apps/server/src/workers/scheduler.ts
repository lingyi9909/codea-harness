import { join, resolve } from 'node:path';
import type { Pool, RowDataPacket, ResultSetHeader } from 'mysql2/promise';
import type { DTO, SessionBinding, Snapshot } from '../../../../packages/contracts/src/domain.ts';
import type { OpenCodeAdapter } from '../../../../packages/contracts/src/ports.ts';
import { withTransaction } from '../../../../packages/storage/src/db.ts';
import { appendEvent } from '../../../../packages/storage/src/events.ts';
import { assertFencedLease, beginNativeSend, claimNext, markNativeAccepted,
  quarantineUnknown, renewLease } from '../../../../packages/storage/src/slots.ts';
import type { Lease } from '../../../../packages/storage/src/slots.ts';
import { prepareSnapshot } from '../../../../packages/git-provider/src/snapshot.ts';
import { trustedRepository } from '../../../../packages/git-provider/src/repository.ts';
import { inspectFrozenSnapshot, originalRepositoryFingerprint } from
  '../../../../packages/opencode-adapter/src/git-guard.ts';
import { setTimeout as delay } from 'node:timers/promises';

export interface SchedulerConfig {
  pool: Pool;
  workerId: string;
  dataRoot: string;
  repositoryRoots: string[];
  nativeVersion: string; // must come from verified target build, not guessed
  adapter: OpenCodeAdapter;
}
function requireCapabilities(caps:DTO<'Capability'>[]):boolean{
  // The production adapter already fails closed; scheduler must not claim the DB
  // execution slot before checking the verified readiness gate.
  return caps.length>0 && caps.every(c=>!c.required||c.status==='SUPPORTED');
}
function safeDetail(e:unknown):string{
  // Never persist native response bodies, paths, credentials or model prompts.
  if(e instanceof Error&&e.message.startsWith('OPENCODE_CAPABILITY_BLOCKED'))return 'CAPABILITY_UNVERIFIED';
  return 'PREPARE_OR_DISPATCH_UNCERTAIN';
}
async function prepareAndPersist(config:SchedulerConfig,lease:Lease,capabilities:DTO<'Capability'>[]):Promise<{
  binding:SessionBinding; controller?:string;
}>{
  const [rows]=await config.pool.execute<RowDataPacket[]>(
    "SELECT t.id,t.controller,t.notes,repo.canonical_path,"+
    "s.id AS snapshot_id,s.head_oid,s.base_oid,s.merge_base_oid,s.base_ref,s.source_branch,s.object_format,s.status "+
    "FROM wh_tasks t JOIN wh_scope_snapshots s ON s.task_id=t.id "+
    "JOIN wh_repositories repo ON repo.id=t.repository_id "+
    "JOIN wh_runs r ON r.task_id=t.id "+
    "WHERE t.id=? AND r.id=?",
    [lease.taskId,lease.runId]);
  if(rows.length!==1||rows[0].status!=='PENDING')throw new Error('SNAPSHOT_NOT_PENDING');
  const row=rows[0],root=await trustedRepository(String(row.canonical_path),config.repositoryRoots);
  const scope:DTO<'Scope'>={
    headOID:String(row.head_oid),baseOID:String(row.base_oid),
    mergeBaseOID:String(row.merge_base_oid),baseRef:String(row.base_ref),
    sourceBranch:row.source_branch===null?null:String(row.source_branch),
    objectFormat:row.object_format,
  };
  const fingerprintBefore=originalRepositoryFingerprint(root);
  const built=await prepareSnapshot({
    taskId:lease.taskId,sourceRoot:root,dataRoot:config.dataRoot,scope,
  });
  const fingerprintAfter=originalRepositoryFingerprint(root);
  if(fingerprintBefore!==fingerprintAfter)throw new Error('SOURCE_MUTATED_DURING_SNAPSHOT');
  const snapshotPath=resolve(config.dataRoot,...built.snapshot.storageKey.split('/'));
  inspectFrozenSnapshot({sourceRoot:root,snapshotRoot:snapshotPath,
    expectedHeadOID:scope.headOID,expectedMergeBaseOID:scope.mergeBaseOID});
  await withTransaction(config.pool,async tx=>{
    await assertFencedLease(tx,lease);
    const [result]=await tx.execute<ResultSetHeader>(
      "UPDATE wh_scope_snapshots SET manifest=?,manifest_sha256=?,storage_key=?,"+
      "status='READY',ready_at=UTC_TIMESTAMP(3) "+
      "WHERE task_id=? AND status='PENDING' AND head_oid=? AND merge_base_oid=?",
      [JSON.stringify(built.manifest),built.snapshot.manifestHash,built.snapshot.storageKey,
        lease.taskId,scope.headOID,scope.mergeBaseOID]);
    if(result.affectedRows!==1)throw new Error('SNAPSHOT_READY_CONFLICT');
    const [run]=await tx.execute<RowDataPacket[]>(
      'SELECT status,dispatch_state FROM wh_runs WHERE id=? AND task_id=? FOR UPDATE',
      [lease.runId,lease.taskId]);
    if(run.length!==1||run[0].status!=='PREPARING'||run[0].dispatch_state!=='NOT_SENT')
      throw new Error('RUN_NOT_READY_FOR_SESSION');
    await appendEvent(tx,{taskId:lease.taskId,runId:lease.runId,type:'scope.ready',
      data:{snapshotId:String(row.snapshot_id),manifestHash:built.snapshot.manifestHash}});
  });
  // Adapter is explicitly bound to the absolute verified repository directory.
  const adapterSnapshot:Snapshot={...built.snapshot,storageKey:snapshotPath};
  const binding=await config.adapter.createSession(lease.taskId,adapterSnapshot);
  await withTransaction(config.pool,async tx=>{
    await assertFencedLease(tx,lease);
    const [run]=await tx.execute<RowDataPacket[]>(
      'SELECT status,dispatch_state FROM wh_runs WHERE id=? AND task_id=? FOR UPDATE',
      [lease.runId,lease.taskId]);
    if(run.length!==1||run[0].status!=='PREPARING'||run[0].dispatch_state!=='NOT_SENT')
      throw new Error('RUN_CHANGED_AFTER_SESSION_CREATE');
    await tx.execute(
      "INSERT INTO wh_session_bindings(task_id,service_identity,native_session_id,location_key,"+
      "native_version,capabilities,continuity,created_at,updated_at) "+
      "VALUES (?,?,?,?,?,?,'UNKNOWN',UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))",
      [lease.taskId,binding.serviceIdentity,binding.sessionId,binding.locationKey,
        config.nativeVersion,JSON.stringify(capabilities)]);
  });
  // T05 rejects notes until the pinned build proves instruction entries exist.
  if(row.notes)await config.adapter.putContext(binding,{scope,notes:String(row.notes)});
  return {binding,...(row.controller?{controller:String(row.controller)}:{})};
}
export async function dispatchNext(config:SchedulerConfig):Promise<string|null>{
  if(!config.nativeVersion.trim()||config.nativeVersion==='UNVERIFIED')
    return null;
  const capabilities=await config.adapter.probe();
  if(!requireCapabilities(capabilities))return null;
  const lease=await claimNext(config.pool,config.workerId);
  if(!lease)return null;
  const abort=new AbortController();
  let leaseLost=false;
  const heartbeat=(async()=>{
    while(!abort.signal.aborted){
      try{await delay(10000,undefined,{signal:abort.signal});}catch{break;}
      if(abort.signal.aborted)break;
      try{await renewLease(config.pool,lease);}
      catch{leaseLost=true;break;}
    }
  })();
  try{
    const prepared=await prepareAndPersist(config,lease,capabilities);
    if(leaseLost)throw new Error('LEASE_RENEWAL_LOST');
    await renewLease(config.pool,lease);
    await beginNativeSend(config.pool,lease);
    let acceptance:Awaited<ReturnType<OpenCodeAdapter['startReview']>>;
    try {
      acceptance=await config.adapter.startReview(prepared.binding,{
        runId:lease.runId,controller:prepared.controller,
      });
    }catch{
      acceptance={state:'UNKNOWN',reason:'NATIVE_TRANSPORT_UNCERTAIN'};
    }
    if(acceptance.state==='ACCEPTED'&&acceptance.inputId){
      await markNativeAccepted(config.pool,lease,acceptance.inputId);
    }else{
      await quarantineUnknown(config.pool,lease,'NATIVE_ACCEPTANCE_UNKNOWN');
    }
  }catch(e){
    // Do not clear an execution slot or reissue native commands on errors.
    try {await quarantineUnknown(config.pool,lease,safeDetail(e));}
    catch { /* On DB/lease loss, occupied slot must be reconciled on restart. */ }
  }finally{
    abort.abort();
    await heartbeat;
  }
  return lease.runId;
}
