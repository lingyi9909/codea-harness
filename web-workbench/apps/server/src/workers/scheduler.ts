import { collectNativeEvents } from './event-collector.ts';
import { join, resolve } from 'node:path';
import { lstat, realpath, rm } from 'node:fs/promises';
import type { Pool, RowDataPacket, ResultSetHeader } from 'mysql2/promise';
import type { DTO, SessionBinding, Snapshot } from '../../../../packages/contracts/src/domain.ts';
import type { OpenCodeAdapter } from '../../../../packages/contracts/src/ports.ts';
import { withTransaction } from '../../../../packages/storage/src/db.ts';
import { appendEvent } from '../../../../packages/storage/src/events.ts';
import { assertFencedLease, beginNativeSend, claimNext, markNativeAccepted,
  quarantineUnknown, renewLease } from '../../../../packages/storage/src/slots.ts';
import type { Lease } from '../../../../packages/storage/src/slots.ts';
import { prepareSnapshot } from '../../../../packages/git-provider/src/snapshot.ts';
import { verifiedManifest } from '../../../../packages/git-provider/src/files.ts';
import { git, trustedRepository } from '../../../../packages/git-provider/src/repository.ts';
import { inspectFrozenSnapshot, originalRepositoryFingerprint } from
  '../../../../packages/opencode-adapter/src/git-guard.ts';
import { REPORT_RELATIVE_PATH } from '../../../../packages/opencode-adapter/src/mapping.ts';
import { setTimeout as delay } from 'node:timers/promises';

export interface SchedulerConfig{
  signal?:AbortSignal;pool:Pool;workerId:string;dataRoot:string;repositoryRoots:string[];
  nativeVersion:string;adapter:OpenCodeAdapter;
}
type Discussion={messageId:string;text:string;quote?:string;reportArtifactId?:string};
type Prepared={binding:SessionBinding;kind:'REVIEW'|'RECHECK'|'DISCUSS';
  controller?:string;discussion?:Discussion};
const requiredCapabilities=(caps:DTO<'Capability'>[])=>caps.length>0&&
  caps.every(c=>!c.required||c.status==='SUPPORTED');
const safeError=(e:unknown)=>e instanceof Error&&e.message.startsWith('OPENCODE_CAPABILITY_BLOCKED')?
  'CAPABILITY_UNVERIFIED':'PREPARE_OR_DISPATCH_UNCERTAIN';

async function loadRun(config:SchedulerConfig,lease:Lease){
  const [rows]=await config.pool.execute<RowDataPacket[]>(
    "SELECT t.id,t.controller,t.notes,repo.canonical_path,r.kind,r.retry_of_run_id,r.status AS run_status,"+
    "s.id AS snapshot_id,s.head_oid,s.base_oid,s.merge_base_oid,s.base_ref,s.source_branch,"+
    "s.object_format,s.status AS snapshot_status,s.storage_key,s.manifest_sha256 "+
    "FROM wh_tasks t JOIN wh_scope_snapshots s ON s.task_id=t.id "+
    "JOIN wh_repositories repo ON repo.id=t.repository_id "+
    "JOIN wh_runs r ON r.task_id=t.id "+
    "WHERE t.id=? AND r.id=?",
    [lease.taskId,lease.runId]);
  if(rows.length!==1||rows[0].run_status!=='PREPARING')
    throw new Error('RUN_NOT_PREPARING');
  const row=rows[0];
  if(!['REVIEW','DISCUSS','RECHECK'].includes(row.kind))
    throw new Error('UNSUPPORTED_RUN_KIND');
  const scope:DTO<'Scope'>={
    headOID:String(row.head_oid),baseOID:String(row.base_oid),
    mergeBaseOID:String(row.merge_base_oid),baseRef:String(row.base_ref),
    sourceBranch:row.source_branch===null?null:String(row.source_branch),
    objectFormat:row.object_format,
  };
  return{row,scope,kind:row.kind as Prepared['kind']};
}
async function initialReview(config:SchedulerConfig,lease:Lease,
  capabilities:DTO<'Capability'>[],row:RowDataPacket,scope:DTO<'Scope'>):Promise<Prepared>{
  if(row.snapshot_status!=='PENDING')throw new Error('SNAPSHOT_NOT_PENDING');
  const root=await trustedRepository(String(row.canonical_path),config.repositoryRoots);
  const before=originalRepositoryFingerprint(root);
  const built=await prepareSnapshot({
    taskId:lease.taskId,sourceRoot:root,dataRoot:config.dataRoot,scope,
  });
  const after=originalRepositoryFingerprint(root);
  if(before!==after)throw new Error('SOURCE_MUTATED_DURING_SNAPSHOT');
  const snapshotPath=resolve(config.dataRoot,...built.snapshot.storageKey.split('/'));
  inspectFrozenSnapshot({sourceRoot:root,snapshotRoot:snapshotPath,
    expectedHeadOID:scope.headOID,expectedMergeBaseOID:scope.mergeBaseOID});
  await withTransaction(config.pool,async tx=>{
    await assertFencedLease(tx,lease);
    const [res]=await tx.execute<ResultSetHeader>(
      "UPDATE wh_scope_snapshots SET manifest=?,manifest_sha256=?,storage_key=?,"+
      "status='READY',ready_at=UTC_TIMESTAMP(3) "+
      "WHERE task_id=? AND status='PENDING' AND head_oid=? AND merge_base_oid=?",
      [JSON.stringify(built.manifest),built.snapshot.manifestHash,built.snapshot.storageKey,
        lease.taskId,scope.headOID,scope.mergeBaseOID]);
    if(res.affectedRows!==1)throw new Error('SNAPSHOT_READY_CONFLICT');
    const [run]=await tx.execute<RowDataPacket[]>(
      'SELECT status,dispatch_state FROM wh_runs WHERE id=? AND task_id=? FOR UPDATE',
      [lease.runId,lease.taskId]);
    if(run.length!==1||run[0].status!=='PREPARING'||run[0].dispatch_state!=='NOT_SENT')
      throw new Error('RUN_NOT_READY_FOR_SESSION');
    await appendEvent(tx,{taskId:lease.taskId,runId:lease.runId,type:'scope.ready',
      data:{snapshotId:String(row.snapshot_id),manifestHash:built.snapshot.manifestHash}});
  });
  const snapshot:Snapshot={...built.snapshot,storageKey:snapshotPath};
  const binding=await config.adapter.createSession(lease.taskId,snapshot);
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
  if(row.notes)await config.adapter.putContext(binding,{scope,notes:String(row.notes)});
  return{binding,kind:'REVIEW',...(row.controller?{controller:String(row.controller)}:{})};
}
async function continuation(config:SchedulerConfig,lease:Lease,
  row:RowDataPacket,scope:DTO<'Scope'>,kind:'DISCUSS'|'RECHECK'):Promise<Prepared>{
  if(row.snapshot_status!=='READY')throw new Error('CONTINUATION_SNAPSHOT_NOT_READY');
  const snapshot:Snapshot={taskId:lease.taskId,scope,
    storageKey:String(row.storage_key),manifestHash:String(row.manifest_sha256)};
  const {root}=await verifiedManifest(config.dataRoot,snapshot);
  // Check committed tree identity even if the old model left an expected report file.
  const [head,base,tracked,untracked]=await Promise.all([
    git(root,['rev-parse','--verify','HEAD']),
    git(root,['rev-parse','--verify','refs/heads/harness-review-base']),
    git(root,['diff','--name-only','HEAD','--']),
    git(root,['ls-files','--others','--exclude-standard']),
  ]);
  if(head!==scope.headOID||base!==scope.mergeBaseOID||tracked.trim()||
    untracked.split(/\r?\n/).filter(Boolean).some(path=>path!==REPORT_RELATIVE_PATH))
    throw new Error('CONTINUATION_SNAPSHOT_MUTATED');
  const [rows]=await config.pool.execute<RowDataPacket[]>(
    'SELECT task_id,service_identity,native_session_id,location_key,native_version '+
    'FROM wh_session_bindings WHERE task_id=?',[lease.taskId]);
  if(rows.length!==1||rows[0].native_version!==config.nativeVersion)
    throw new Error('NATIVE_SESSION_UNVERIFIED');
  const native=rows[0];
  const location=await realpath(String(native.location_key));
  if(location!==root)throw new Error('NATIVE_SESSION_LOCATION_CHANGED');
  const binding:SessionBinding={taskId:lease.taskId,
    serviceIdentity:String(native.service_identity),
    sessionId:String(native.native_session_id),locationKey:location};
  // An in-memory binding lost on restart is NOT silently recreated; only a
  // verified T13 native reattachment protocol may allow it.
  const state=await config.adapter.inspect(binding);
  if(state.active!==false||state.pendingInputs!==0||state.pendingInteractions!==0||
    state.externalMutation||state.acceptedInputIds.length>0)
    throw new Error('NATIVE_SESSION_NOT_PROVEN_IDLE');
  let discussion:Discussion|undefined;
  if(kind==='DISCUSS'){
    const [messages]=await config.pool.execute<RowDataPacket[]>(
      "WITH RECURSIVE ancestry AS (SELECT id,retry_of_run_id FROM wh_runs WHERE task_id=? AND id=? "+
      "UNION ALL SELECT r.id,r.retry_of_run_id FROM wh_runs r JOIN ancestry a ON r.id=a.retry_of_run_id "+
      "WHERE r.task_id=? AND r.kind='DISCUSS') SELECT m.id,m.content FROM wh_messages m JOIN ancestry a ON a.id=m.run_id "+
      "WHERE m.task_id=? AND m.role='USER' AND m.status='COMPLETE'",
      [lease.taskId,lease.runId,lease.taskId,lease.taskId]);
    if(messages.length!==1)throw new Error('DISCUSSION_MESSAGE_NOT_UNIQUE');
    let payload:unknown;
    try{payload=JSON.parse(String(messages[0].content)) as unknown;}
    catch{throw new Error('DISCUSSION_PAYLOAD_INVALID');}
    if(!payload||typeof payload!=='object')throw new Error('DISCUSSION_PAYLOAD_INVALID');
    const value=payload as Record<string,unknown>;
    if(typeof value.text!=='string'||!value.text.trim()||value.text.length>8000||
      (value.quote!==undefined&&typeof value.quote!=='string'))
      throw new Error('DISCUSSION_PAYLOAD_INVALID');
    discussion={messageId:String(messages[0].id),text:value.text,
      ...(value.quote?{quote:value.quote}:{}),
      ...(typeof value.reportArtifactId==='string'?{reportArtifactId:value.reportArtifactId}:{})};
  }else{
    // The previous report is already archived; an old working-tree report
    // must never become the current recheck's new write evidence.
    const path=join(root,...REPORT_RELATIVE_PATH.split('/'));
    try{
      const info=await lstat(path);
      if(!info.isFile()||info.isSymbolicLink())throw new Error('STALE_REPORT_UNSAFE');
      await rm(path);
    }catch(e){
      if(!(e instanceof Error&&'code' in e&&e.code==='ENOENT'))throw e;
    }
  }
  await withTransaction(config.pool,async tx=>{await assertFencedLease(tx,lease);});
  return{binding,kind,...(row.controller?{controller:String(row.controller)}:{}),
    ...(discussion?{discussion}:{})};
}
async function prepare(config:SchedulerConfig,lease:Lease,
  capabilities:DTO<'Capability'>[]):Promise<Prepared>{
  const {row,scope,kind}=await loadRun(config,lease);
  if(kind==='REVIEW' && row.retry_of_run_id && row.snapshot_status==='READY')
    return continuation(config,lease,row,scope,'RECHECK');
  return kind==='REVIEW'?
    initialReview(config,lease,capabilities,row,scope):
    continuation(config,lease,row,scope,kind);
}
/** Pure selection: a continuation MUST NOT silently become a fresh REVIEW. */
export async function invokePreparedNative(adapter:OpenCodeAdapter,prepared:Prepared,
  runId:string):Promise<Awaited<ReturnType<OpenCodeAdapter['startReview']>>>{
  if(prepared.kind==='DISCUSS'){
    if(!prepared.discussion)throw new Error('DISCUSSION_INPUT_MISSING');
    return adapter.discuss(prepared.binding,{runId,...prepared.discussion});
  }
  return adapter.startReview(prepared.binding,{
    runId,...(prepared.controller?{controller:prepared.controller}:{}),
    ...(prepared.kind==='RECHECK'?{recheck:true}:{}),
  });
}
export async function dispatchNext(config:SchedulerConfig):Promise<string|null>{
  if(!config.nativeVersion.trim()||config.nativeVersion==='UNVERIFIED')return null;
  const capabilities=await config.adapter.probe();
  if(!requiredCapabilities(capabilities))return null;
  config.signal?.throwIfAborted();
  const lease=await claimNext(config.pool,config.workerId);
  if(!lease)return null;
  const abort=new AbortController();
  let leaseLost=false;
  const heartbeat=(async()=>{
    while(!abort.signal.aborted){
      try{await delay(10000,undefined,{signal:abort.signal});}catch{break;}
      if(abort.signal.aborted)break;
      try{await renewLease(config.pool,lease);}catch{leaseLost=true;abort.abort();break;}
    }
  })();
  try{
    const prepared=await prepare(config,lease,capabilities);
    config.signal?.throwIfAborted();
    if(leaseLost)throw new Error('LEASE_RENEWAL_LOST');
    await renewLease(config.pool,lease);
    await beginNativeSend(config.pool,lease);
    let acceptance:Awaited<ReturnType<OpenCodeAdapter['startReview']>>;
    try{acceptance=await invokePreparedNative(config.adapter,prepared,lease.runId);}
    catch{acceptance={state:'UNKNOWN',reason:'NATIVE_TRANSPORT_OR_MAPPING_UNCERTAIN'};}
    if(acceptance.state==='ACCEPTED'&&acceptance.inputId){
      await markNativeAccepted(config.pool,lease,acceptance.inputId);
      await collectNativeEvents({...config,lease,binding:prepared.binding,inputId:acceptance.inputId,
        durable:capabilities.some(cap=>cap.name==='session.log'&&cap.status==='SUPPORTED')},config.signal?AbortSignal.any([abort.signal,config.signal]):abort.signal);
    }else
      await quarantineUnknown(config.pool,lease,'NATIVE_ACCEPTANCE_UNKNOWN');
  }catch(e){
    // Any possible native execution or stale lease is reconciled, never replayed.
    try{await quarantineUnknown(config.pool,lease,safeError(e));}
    catch{/* Lease/DB loss keeps occupied slot for explicit recovery. */}
  }finally{
    abort.abort();await heartbeat;
  }
  return lease.runId;
}
