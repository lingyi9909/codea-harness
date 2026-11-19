import { createHash, randomUUID } from 'node:crypto';
import { link, lstat, mkdir, open, readFile, realpath, unlink } from 'node:fs/promises';
import { join, resolve, relative, isAbsolute, sep } from 'node:path';
import type { Pool, ResultSetHeader, RowDataPacket } from 'mysql2/promise';
import type { DTO, Snapshot } from '../../../../packages/contracts/src/domain.ts';
import { withTransaction } from '../../../../packages/storage/src/db.ts';
import { appendEvent } from '../../../../packages/storage/src/events.ts';
import { assertFencedLease } from '../../../../packages/storage/src/slots.ts';
import type { Lease } from '../../../../packages/storage/src/slots.ts';
import { verifiedManifest } from '../../../../packages/git-provider/src/files.ts';
import { verifyWrittenReport } from '../../../../packages/opencode-adapter/src/native.ts';
import { REPORT_RELATIVE_PATH } from '../../../../packages/opencode-adapter/src/mapping.ts';
import { MAX_REPORT_BYTES } from '../modules/artifacts/service.ts';
import { AuthProblem } from '../auth.ts';

const sha=(buf:Buffer)=>createHash('sha256').update(buf).digest('hex');
const iso=(s:unknown)=>String(s).replace(' ','T')+'Z';
export type NativeWriteEvidence={
  /** Verified internal native event, never supplied through a browser API. */
  taskId:string;runId:string;nativeSessionId:string;serviceIdentity:string;
  nativeWriteMessageId:string;path:string;toolOutcome:'SUCCEEDED';
  /** Native state confirmed after input and form reconciliation. */
  modelTerminated:true;pendingInputs:0;pendingInteractions:0;externalMutation:false;
  cursor?:string;coverage:'FULL'|'PARTIAL';
};
export interface ReportCollectorConfig{pool:Pool;dataRoot:string}
export async function collectReport(config:ReportCollectorConfig,lease:Lease,
  evidence:NativeWriteEvidence):Promise<DTO<'ArtifactSummary'>>{
  if(evidence.taskId!==lease.taskId||evidence.runId!==lease.runId||
    evidence.path!==REPORT_RELATIVE_PATH||evidence.toolOutcome!=='SUCCEEDED'||
    evidence.modelTerminated!==true||evidence.pendingInputs!==0||
    evidence.pendingInteractions!==0||evidence.externalMutation!==false||
    !['FULL','PARTIAL'].includes(evidence.coverage)||
    !evidence.nativeWriteMessageId||evidence.nativeWriteMessageId.length>128)
    throw new AuthProblem(409,'NATIVE_REPORT_EVIDENCE_REQUIRED');
  const [rows]=await config.pool.execute<RowDataPacket[]>(
    "SELECT r.id AS run_id,r.status AS run_status,r.kind,r.started_at,"+
    "s.head_oid,s.base_oid,s.merge_base_oid,s.base_ref,s.source_branch,s.object_format,"+
    "s.storage_key,s.manifest_sha256,s.status AS snapshot_status,"+
    "b.native_session_id,b.service_identity FROM wh_runs r "+
    "JOIN wh_scope_snapshots s ON s.task_id=r.task_id "+
    "JOIN wh_session_bindings b ON b.task_id=r.task_id "+
    "WHERE r.id=? AND r.task_id=?",
    [lease.runId,lease.taskId]);
  if(rows.length!==1)throw new AuthProblem(404,'RUN_NOT_FOUND');
  const row=rows[0];
  if(row.run_status!=='COLLECTING'||!['REVIEW','RECHECK'].includes(row.kind)||
    row.snapshot_status!=='READY'||!row.started_at||
    row.native_session_id!==evidence.nativeSessionId||
    row.service_identity!==evidence.serviceIdentity)
    throw new AuthProblem(409,'RUN_NOT_COLLECTABLE');
  const scope:DTO<'Scope'>={
    headOID:String(row.head_oid),baseOID:String(row.base_oid),
    mergeBaseOID:String(row.merge_base_oid),baseRef:String(row.base_ref),
    sourceBranch:row.source_branch===null?null:String(row.source_branch),
    objectFormat:row.object_format,
  };
  const snapshot:Snapshot={taskId:lease.taskId,scope,storageKey:String(row.storage_key),
    manifestHash:String(row.manifest_sha256)};
  const {root:snapshotRoot}=await verifiedManifest(config.dataRoot,snapshot);
  const startedAt=Date.parse(iso(row.started_at));
  if(!Number.isFinite(startedAt))throw new AuthProblem(409,'RUN_START_INVALID');
  const checked=await verifyWrittenReport(snapshotRoot,startedAt);
  if(checked.path!==REPORT_RELATIVE_PATH||checked.bytes>MAX_REPORT_BYTES)
    throw new AuthProblem(409,'REPORT_TOO_LARGE_OR_WRONG_PATH');
  const reportAbs=join(snapshotRoot,...REPORT_RELATIVE_PATH.split('/'));
  const candidate=await readFile(reportAbs);
  if(candidate.length!==checked.bytes||sha(candidate)!==checked.sha256||
    candidate.includes(0)||!Buffer.from(candidate.toString('utf8'),'utf8').equals(candidate))
    throw new AuthProblem(409,'REPORT_CHANGED_OR_INVALID_ENCODING');
  const dataRoot=await realpath(resolve(config.dataRoot));
  const archiveDir=join(dataRoot,'artifacts',lease.taskId);
  const rel=relative(dataRoot,archiveDir);
  if(rel==='..'||rel.startsWith('..'+sep)||isAbsolute(rel))
    throw new AuthProblem(409,'ARCHIVE_STORAGE_ESCAPE');
  await mkdir(archiveDir,{recursive:true});
  const canonicalDir=await realpath(archiveDir);
  if(canonicalDir!==archiveDir)
    throw new AuthProblem(409,'ARCHIVE_DIRECTORY_REDIRECTED');
  const dest=join(canonicalDir,lease.runId+'.md');
  const key=['artifacts',lease.taskId,lease.runId+'.md'].join('/');
  const digest=sha(candidate);
  let alreadyExists=false;
  try{await lstat(dest);alreadyExists=true;}
  catch(e){
    if(!(e instanceof Error&&'code' in e&&e.code==='ENOENT'))throw e;
  }
  if(alreadyExists){
    const info=await lstat(dest),bytes=await readFile(dest);
    if(!info.isFile()||info.isSymbolicLink()||sha(bytes)!==digest)
      throw new AuthProblem(409,'ARCHIVED_REPORT_CONFLICT');
  }else{
    const temp=join(canonicalDir,lease.runId+'.'+randomUUID()+'.partial');
    const file=await open(temp,'wx',0o600);
    try{
      try{await file.writeFile(candidate);await file.sync();}finally{await file.close();}
      await withTransaction(config.pool,async tx=>{
        await assertFencedLease(tx,lease);
        try{await link(temp,dest);}
        catch(error){
          if(!(error instanceof Error&&'code' in error&&error.code==='EEXIST'))throw error;
          const info=await lstat(dest);
          if(!info.isFile()||info.isSymbolicLink()||sha(await readFile(dest))!==digest)
            throw new AuthProblem(409,'ARCHIVED_REPORT_CONFLICT');
        }
      });
    }finally{await unlink(temp);}
  }
  const finalBytes=await readFile(dest);
  if(sha(finalBytes)!==digest||finalBytes.length!==candidate.length)
    throw new AuthProblem(409,'ARCHIVE_INTEGRITY_FAILED');
  const validation={
    templateVersion:'v1',chapters:checked.sections,
    nativeWriteMessageId:evidence.nativeWriteMessageId,
    scopeHash:snapshot.manifestHash,coverage:evidence.coverage,
  };
  return withTransaction(config.pool,async tx=>{
    await assertFencedLease(tx,lease);
    const [existing]=await tx.execute<RowDataPacket[]>(
      "SELECT * FROM wh_artifacts WHERE run_id=? AND kind='REVIEW_MARKDOWN' FOR UPDATE",
      [lease.runId]);
    if(existing.length){
      if(existing[0].sha256!==digest||existing[0].storage_key!==key)
        throw new AuthProblem(409,'ARTIFACT_RECONCILIATION_REQUIRED');
      return {id:String(existing[0].id),runId:lease.runId,version:Number(existing[0].version),
        kind:'REVIEW_MARKDOWN',sha256:digest,sizeBytes:String(existing[0].size_bytes),
        coverage:existing[0].coverage,createdAt:iso(existing[0].created_at)};
    }
    const [run]=await tx.execute<RowDataPacket[]>(
      'SELECT status FROM wh_runs WHERE id=? AND task_id=? FOR UPDATE',
      [lease.runId,lease.taskId]);
    if(run.length!==1||run[0].status!=='COLLECTING')
      throw new AuthProblem(409,'RUN_CHANGED_DURING_COLLECTION');
    const [prior]=await tx.execute<RowDataPacket[]>(
      'SELECT COALESCE(MAX(version),0) AS version FROM wh_artifacts WHERE task_id=?',
      [lease.taskId]);
    const version=Number(prior[0]?.version??0)+1;
    if(!Number.isSafeInteger(version)||version<1)throw new AuthProblem(503,'ARTIFACT_VERSION_INVALID');
    const artifactId=randomUUID();
    await tx.execute(
      "INSERT INTO wh_artifacts(id,task_id,run_id,version,kind,storage_key,sha256,"+
      "size_bytes,coverage,validation,native_write_message_id,created_at) "+
      "VALUES (?,?,?,?,'REVIEW_MARKDOWN',?,?,?,?,?,?,UTC_TIMESTAMP(3))",
      [artifactId,lease.taskId,lease.runId,version,key,digest,
        String(candidate.length),evidence.coverage,JSON.stringify(validation),
        evidence.nativeWriteMessageId]);
    const [updated]=await tx.execute<ResultSetHeader>(
      "UPDATE wh_runs SET status='SUCCEEDED',finished_at=UTC_TIMESTAMP(3),"+
      "updated_at=UTC_TIMESTAMP(3) WHERE id=? AND status='COLLECTING' AND lease_fence=?",
      [lease.runId,lease.fence]);
    if(updated.affectedRows!==1)throw new AuthProblem(409,'RUN_COMPLETION_CONFLICT');
    await appendEvent(tx,{taskId:lease.taskId,runId:lease.runId,type:'artifact.ready',
      data:{artifactId,version,coverage:evidence.coverage}});
    await appendEvent(tx,{taskId:lease.taskId,runId:lease.runId,type:'run.status',
      data:{status:'SUCCEEDED',previousStatus:'COLLECTING',errorCode:null}});
    if(evidence.cursor){
      if(!/^(0|[1-9][0-9]{0,63})$/.test(evidence.cursor))throw new Error('NATIVE_CURSOR_INVALID');
      await tx.execute("UPDATE wh_session_bindings SET native_log_seq=?,continuity=IF(continuity='GAP','GAP','COMPLETE'),"+
        "updated_at=UTC_TIMESTAMP(3) WHERE task_id=?",[evidence.cursor,lease.taskId]);
    }
    await tx.execute(
      "UPDATE wh_executor_slots SET run_id=NULL,lease_owner=NULL,lease_until=NULL,"+
      "quarantined=0,updated_at=UTC_TIMESTAMP(3) "+
      "WHERE slot_key='local-default' AND run_id=? AND lease_owner=? AND fence=?",
      [lease.runId,lease.owner,lease.fence]);
    return {id:artifactId,runId:lease.runId,version,kind:'REVIEW_MARKDOWN',
      sha256:digest,sizeBytes:String(candidate.length),coverage:evidence.coverage,
      createdAt:new Date().toISOString()};
  });
}
