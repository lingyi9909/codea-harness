import type { FastifyInstance } from 'fastify';
import type { DTO } from '../../../../packages/contracts/src/domain.ts';
import { query, type DbRow } from '../../../../packages/storage/src/db.ts';
import { httpFail } from '../auth.ts';
import { lookupOperation, paramRequired } from './ops.ts';

export const parsed=<T>(value:unknown):T=>{
  return (typeof value==='string'?JSON.parse(value) as unknown:value) as T;
};
export const iso=(value:unknown):string=>
  new Date(String(value).replace(' ','T')+'Z').toISOString();
function run(row:DbRow):DTO<'Run'> {
  return {
    id:String(row.id),taskId:String(row.task_id),number:Number(row.run_number),
    kind:row.kind as DTO<'Run'>['kind'],status:row.status as DTO<'Run'>['status'],
    retryOfRunId:row.retry_of_run_id===null?null:String(row.retry_of_run_id),
    dispatchState:row.dispatch_state as DTO<'Run'>['dispatchState'],
    errorCode:row.error_code===null?null:String(row.error_code),
    createdAt:iso(row.created_at),
    startedAt:row.started_at===null?null:iso(row.started_at),
    finishedAt:row.finished_at===null?null:iso(row.finished_at),
  };
}
function taskSummary(row:DbRow):DTO<'TaskSummary'>{
  return {
    id:String(row.id),repositoryId:String(row.repository_id),title:String(row.title),
    scenarioId:'code-review',status:row.run_status as DTO<'Run'>['status'],
    latestRunId:String(row.latest_run_id),headOID:String(row.head_oid),baseRef:String(row.base_ref),
    createdAt:iso(row.created_at),updatedAt:iso(row.updated_at),
  };
}
export function artifactSummary(row:DbRow):DTO<'ArtifactSummary'> {
  return {id:String(row.id),runId:String(row.run_id),version:Number(row.version),
    kind:'REVIEW_MARKDOWN',sha256:String(row.sha256),sizeBytes:String(row.size_bytes),
    coverage:row.coverage as DTO<'ArtifactSummary'>['coverage'],createdAt:iso(row.created_at)};
}
export async function getOwnedTask(owner:string,id:string):Promise<DTO<'TaskDetail'>>{
  const tasks=await query<DbRow>(
    'SELECT t.*,p.head_oid,p.base_oid,p.merge_base_oid,p.base_ref,p.source_branch,p.object_format, s.status AS snapshot_status, '+
    's.manifest_sha256,s.storage_key,r.id AS latest_run_id,r.status AS run_status '+
    'FROM wh_tasks t JOIN wh_review_previews p ON p.id=t.preview_id '+
    'JOIN wh_scope_snapshots s ON s.task_id=t.id '+
    'JOIN wh_runs r ON r.task_id=t.id AND r.run_number=(SELECT MAX(x.run_number) FROM wh_runs x WHERE x.task_id=t.id) '+
    'WHERE t.id=? AND t.owner_id=? LIMIT 1',[id,owner]);
  if(!tasks.length)httpFail(404,'NOT_FOUND');
  const row=tasks[0];
  const runs=await query<DbRow>('SELECT * FROM wh_runs WHERE id=? AND task_id=?',[String(row.latest_run_id),id]);
  const reports=await query<DbRow>('SELECT * FROM wh_artifacts WHERE task_id=? ORDER BY version ASC',[id]);
  return {
    task:taskSummary(row),
    scope:{headOID:String(row.head_oid),baseOID:String(row.base_oid),
      mergeBaseOID:String(row.merge_base_oid),baseRef:String(row.base_ref),
      sourceBranch:row.source_branch===null?null:String(row.source_branch),
      objectFormat:row.object_format as DTO<'Scope'>['objectFormat']},
    snapshotStatus:row.snapshot_status as DTO<'TaskDetail'>['snapshotStatus'],
    latestRun:run(runs[0]),reports:reports.map(artifactSummary),
    lastSeq:String(BigInt(String(row.next_event_seq))-1n),
    continuity:'UNKNOWN',
    controller:row.controller===null?null:String(row.controller),
    notes:row.notes===null?null:String(row.notes),
  };
}
export function registerTaskReads(app:FastifyInstance,getOwner:()=>Promise<string>):void{
  app.get<{Params:{operationId:string}}>('/api/v1/operations/:operationId',async(req,reply)=>{
    const owner=await getOwner();
    const value=await lookupOperation(owner,paramRequired(req.params.operationId));
    if(!value)httpFail(404,'NOT_FOUND');
    return reply.send({data:value,requestId:req.id});
  });
  app.get('/api/v1/tasks',async(req,reply)=>{
    const owner=await getOwner();
    const rows=await query<DbRow>(
      'SELECT t.*,p.head_oid,p.base_ref,r.id AS latest_run_id,r.status AS run_status '+
      'FROM wh_tasks t JOIN wh_review_previews p ON p.id=t.preview_id JOIN wh_runs r ON r.task_id=t.id '+
      'AND r.run_number=(SELECT MAX(x.run_number) FROM wh_runs x WHERE x.task_id=t.id) '+
      'WHERE t.owner_id=? ORDER BY t.created_at DESC,t.id DESC LIMIT 100',[owner]);
    return reply.send({data:{items:rows.map(taskSummary),nextCursor:null},requestId:req.id});
  });
  app.get<{Params:{taskId:string}}>('/api/v1/tasks/:taskId',async(req,reply)=>{
    return reply.send({data:await getOwnedTask(await getOwner(),paramRequired(req.params.taskId)),requestId:req.id});
  });
  app.get<{Params:{taskId:string}}>('/api/v1/tasks/:taskId/runs',async(req,reply)=>{
    const taskId=paramRequired(req.params.taskId);
    await getOwnedTask(await getOwner(),taskId);
    const rows=await query<DbRow>('SELECT * FROM wh_runs WHERE task_id=? ORDER BY run_number DESC LIMIT 100',[taskId]);
    return reply.send({data:{items:rows.map(run),nextCursor:null},requestId:req.id});
  });
  app.get<{Params:{taskId:string}}>('/api/v1/tasks/:taskId/messages',async(req,reply)=>{
    const taskId=paramRequired(req.params.taskId);
    await getOwnedTask(await getOwner(),taskId);
    const rows=await query<DbRow>(
      'SELECT * FROM wh_messages WHERE task_id=? ORDER BY created_at DESC,id DESC LIMIT 100',[taskId]);
    const items:DTO<'Message'>[]=rows.map(row=>({
      id:String(row.id),runId:String(row.run_id),
      role:row.role as DTO<'Message'>['role'],status:row.status as DTO<'Message'>['status'],
      text:String(row.content),reportArtifactId:row.report_artifact_id===null?null:String(row.report_artifact_id),
      createdAt:iso(row.created_at),updatedAt:iso(row.updated_at),
    }));
    return reply.send({data:{items,nextCursor:null},requestId:req.id});
  });
  app.get<{Params:{taskId:string}}>('/api/v1/tasks/:taskId/interactions',async(req,reply)=>{
    const taskId=paramRequired(req.params.taskId);
    await getOwnedTask(await getOwner(),taskId);
    const rows=await query<DbRow>(
      "SELECT i.schema_json FROM wh_interactions i JOIN wh_runs r ON r.id=i.run_id WHERE i.task_id=? AND i.status IN ('PENDING','ANSWERING','UNKNOWN') AND r.status IN ('RUNNING','WAITING_INPUT','WAITING_PERMISSION','RECOVERING') ORDER BY i.created_at ASC",
      [taskId]);
    const items=rows.map(row=>parsed<DTO<'Interaction'>>(row.schema_json));
    return reply.send({data:{items},requestId:req.id});
  });
}
