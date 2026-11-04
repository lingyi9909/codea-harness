import type { FastifyInstance } from 'fastify';
import type { RowDataPacket } from 'mysql2/promise';
import type { DTO } from '../../../../packages/contracts/src/domain.ts';
import { query, utcNow, uuid, sha256, futureTime, type DbRow } from '../../../../packages/storage/src/db.ts';
import { httpFail } from '../auth.ts';
import { confirmPreviewStillCurrent } from './git.ts';
import { ownedRepository } from './repositories.ts';
import { getOwnedTask, parsed } from './tasks.ts';
import { idempotent, operationResult, outbox, paramRequired, stringRequired, uuidRequired, textLimit } from './ops.ts';
import { dataDir } from '../config.ts';
import { join } from 'node:path';

const live=new Set(['QUEUED','PREPARING','RUNNING','WAITING_INPUT','WAITING_PERMISSION','COLLECTING','STOP_REQUESTED','RECOVERING']);
const final=new Set(['SUCCEEDED','FAILED','STOPPED']);
const nativeBlock=(ready:()=>boolean)=>{if(!ready())httpFail(503,'CAPABILITY_MISSING');};
async function latest(tx:{execute:<T extends RowDataPacket[]>(sql:string,p?:unknown[])=>Promise<[T,unknown]>},taskId:string){
  const [rows]=await tx.execute<RowDataPacket[]>('SELECT * FROM wh_runs WHERE task_id=? ORDER BY run_number DESC LIMIT 1',[taskId]);
  if(!rows.length)httpFail(409,'RUN_NOT_FOUND');
  return rows[0];
}
export function registerTaskWrites(app:FastifyInstance,getOwner:()=>Promise<string>,nativeReady:()=>boolean):void {
  app.post('/api/v1/tasks',async(req,reply)=>{
    nativeBlock(nativeReady);
    const body=req.body as Record<string,unknown>;
    const owner=await getOwner(),key=uuidRequired(body?.operationId),previewId=paramRequired(body?.previewId);
    const controller=body?.controller===undefined?'':textLimit(body.controller,128);
    if(controller&&!/^[A-Za-z_][A-Za-z0-9_]*$/.test(controller))httpFail(400,'INVALID_CONTROLLER');
    const notes=body?.notes===undefined?'':textLimit(body.notes,2000);
    const requestedTitle=body?.title===undefined?'':textLimit(body.title,160);
    const rows=await query<DbRow>('SELECT * FROM wh_review_previews WHERE id=? AND owner_id=?',[previewId,owner]);
    if(!rows.length)httpFail(404,'NOT_FOUND');
    const preview=rows[0];
    if(new Date(String(preview.expires_at).replace(' ','T')+'Z').getTime()<Date.now())httpFail(409,'PREVIEW_EXPIRED');
    const repo=await ownedRepository(owner,String(preview.repository_id));
    const scope:DTO<'Scope'>={
      headOID:String(preview.head_oid),baseOID:String(preview.base_oid),mergeBaseOID:String(preview.merge_base_oid),
      baseRef:String(preview.base_ref),sourceBranch:preview.source_branch===null?null:String(preview.source_branch),
      objectFormat:preview.object_format as DTO<'Scope'>['objectFormat'],
    };
    try{confirmPreviewStillCurrent(repo,scope,String(preview.base_ref_id));}
    catch{httpFail(409,'SCOPE_CHANGED');}
    const title=requestedTitle||repo.name+' · '+scope.headOID.slice(0,9);
    const normalized={previewId,title,controller,notes};
    const result=await idempotent(owner,'TASK_CREATE',previewId,key,normalized,async(tx,op)=>{
      const [used]=await tx.execute<RowDataPacket[]>('SELECT id FROM wh_tasks WHERE preview_id=? FOR UPDATE',[previewId]);
      if(used.length)httpFail(409,'PREVIEW_USED');
      const taskId=uuid(),runId=uuid(),snapId=uuid(),now=utcNow();
      await tx.execute('INSERT INTO wh_tasks(id,owner_id,repository_id,preview_id,title,controller,notes,next_run_number,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)',
        [taskId,owner,repo.id,previewId,title,controller||null,notes||null,2,now,now]);
      const manifest={scope,files:[],sourceRepositoryId:repo.id};
      await tx.execute(
        'INSERT INTO wh_scope_snapshots(id,task_id,head_oid,base_oid,merge_base_oid,base_ref,source_branch,object_format,manifest,manifest_sha256,storage_key,status,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)',
        [snapId,taskId,scope.headOID,scope.baseOID,scope.mergeBaseOID,scope.baseRef,scope.sourceBranch,
         scope.objectFormat,JSON.stringify(manifest),sha256(JSON.stringify(manifest)),join(dataDir,'tasks',taskId,'source'),
         'PENDING',now]);
      await tx.execute(
        'INSERT INTO wh_runs(id,task_id,run_number,kind,status,dispatch_state,asset_versions,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)',
        [runId,taskId,1,'REVIEW','QUEUED','NOT_SENT',JSON.stringify({}),now,now]);
      await outbox(tx,op,taskId,runId,'DISPATCH',{kind:'REVIEW'});
      return {value:operationResult(key,taskId,runId),taskId,runId,status:'ACCEPTED' as const};
    });
    return reply.status(202).send({data:result,requestId:req.id});
  });
  app.post<{Params:{runId:string}}>('/api/v1/runs/:runId/stop',async(req,reply)=>{
    const owner=await getOwner(),runId=paramRequired(req.params.runId);
    const body=req.body as Record<string,unknown>,key=uuidRequired(body?.operationId);
    const rows=await query<DbRow>('SELECT r.task_id,r.status FROM wh_runs r JOIN wh_tasks t ON t.id=r.task_id WHERE r.id=? AND t.owner_id=?',[runId,owner]);
    if(!rows.length)httpFail(404,'NOT_FOUND');
    const taskId=String(rows[0].task_id);
    const result=await idempotent(owner,'RUN_STOP',runId,key,{runId},async(tx,op)=>{
      const current=await latest(tx,taskId);
      if(current.id!==runId)httpFail(409,'RUN_NOT_CURRENT');
      if(final.has(String(current.status)))return{value:operationResult(key,taskId,runId,'SUCCEEDED'),taskId,runId,status:'SUCCEEDED' as const};
      if(current.status==='STOP_REQUESTED')httpFail(409,'STOP_IN_PROGRESS');
      if(!live.has(String(current.status)))httpFail(409,'EXECUTION_UNCERTAIN');
      const now=utcNow();
      await tx.execute('UPDATE wh_runs SET status=?,updated_at=? WHERE id=?',['STOP_REQUESTED',now,runId]);
      await outbox(tx,op,taskId,runId,'INTERRUPT',{runId});
      return{value:operationResult(key,taskId,runId),taskId,runId,status:'ACCEPTED' as const};
    });
    return reply.status(202).send({data:result,requestId:req.id});
  });
  async function startFollowup(owner:string,taskId:string,key:string,action:'DISCUSS'|'RECHECK'|'RETRY',
    payload:Record<string,unknown>,request:Record<string,unknown>):Promise<DTO<'Operation'>> {
    nativeBlock(nativeReady);
    const detail=await getOwnedTask(owner,taskId);
    if(detail.snapshotStatus!=='READY')httpFail(409,'SNAPSHOT_NOT_READY');
    if(live.has(detail.latestRun.status))httpFail(409,'TASK_BUSY');
    if(action==='RETRY'){
      if(!['FAILED','STOPPED'].includes(detail.latestRun.status))httpFail(409,'RETRY_NOT_ALLOWED');
    }else if(!detail.reports.length)httpFail(409,'NO_REPORT');
    // Only after a verified native inspect proves idle may a new round begin.
    // The worker will re-inspect before actual dispatch; uncertain gets RECOVERING.
    return idempotent(owner,action,taskId,key,request,async(tx,op)=>{
      const [tasks]=await tx.execute<RowDataPacket[]>('SELECT next_run_number FROM wh_tasks WHERE id=? AND owner_id=? FOR UPDATE',[taskId,owner]);
      if(!tasks.length)httpFail(404,'NOT_FOUND');
      const last=await latest(tx,taskId);
      if(live.has(String(last.status)))httpFail(409,'TASK_BUSY');
      const slot=await tx.execute<RowDataPacket[]>('SELECT run_id,quarantined FROM wh_executor_slots WHERE slot_key=? FOR UPDATE',['local-default']);
      if(slot[0][0]?.run_id||slot[0][0]?.quarantined)httpFail(409,'EXECUTION_UNCERTAIN');
      const number=Number(tasks[0].next_run_number), runId=uuid(),now=utcNow();
      const kind=action==='RETRY'?String(last.kind):action;
      await tx.execute(
        'INSERT INTO wh_runs(id,task_id,run_number,kind,status,dispatch_state,retry_of_run_id,asset_versions,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)',
        [runId,taskId,number,kind,'QUEUED','NOT_SENT',action==='RETRY'?String(last.id):null,JSON.stringify({}),now,now]);
      await tx.execute('UPDATE wh_tasks SET next_run_number=next_run_number+1,updated_at=? WHERE id=?',[now,taskId]);
      if(action==='DISCUSS'){
        const messageId=uuid();
        await tx.execute('INSERT INTO wh_messages(id,task_id,run_id,role,content,status,report_artifact_id,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)',
          [messageId,taskId,runId,'USER',String(payload.text),'DRAFT',payload.reportArtifactId??null,now,now]);
        payload.messageId=messageId;
      }
      await outbox(tx,op,taskId,runId,'DISPATCH',{kind,...payload});
      return {value:operationResult(key,taskId,runId),taskId,runId,status:'ACCEPTED' as const};
    });
  }
  app.post<{Params:{taskId:string}}>('/api/v1/tasks/:taskId/messages',async(req,reply)=>{
    const taskId=paramRequired(req.params.taskId),owner=await getOwner();
    const body=req.body as Record<string,unknown>,key=uuidRequired(body?.operationId);
    const text=stringRequired(body?.text,8000);
    const quote=body.quote===undefined?'':textLimit(body.quote,4000);
    const reportArtifactId=body.reportArtifactId===undefined?null:paramRequired(body.reportArtifactId);
    const result=await startFollowup(owner,taskId,key,'DISCUSS',
      {text,quote,reportArtifactId},{text,quote,reportArtifactId});
    return reply.status(202).send({data:result,requestId:req.id});
  });
  app.post<{Params:{taskId:string}}>('/api/v1/tasks/:taskId/rechecks',async(req,reply)=>{
    const taskId=paramRequired(req.params.taskId),owner=await getOwner();
    const key=uuidRequired((req.body as Record<string,unknown>)?.operationId);
    const result=await startFollowup(owner,taskId,key,'RECHECK',{}, {taskId});
    return reply.status(202).send({data:result,requestId:req.id});
  });
  app.post<{Params:{runId:string}}>('/api/v1/runs/:runId/retries',async(req,reply)=>{
    const runId=paramRequired(req.params.runId),owner=await getOwner();
    const key=uuidRequired((req.body as Record<string,unknown>)?.operationId);
    const rows=await query<DbRow>('SELECT r.task_id FROM wh_runs r JOIN wh_tasks t ON t.id=r.task_id WHERE r.id=? AND t.owner_id=?',[runId,owner]);
    if(!rows.length)httpFail(404,'NOT_FOUND');
    const taskId=String(rows[0].task_id);
    const result=await startFollowup(owner,taskId,key,'RETRY',{retryOfRunId:runId},{retryOfRunId:runId});
    return reply.status(202).send({data:result,requestId:req.id});
  });
  app.post<{Params:{interactionId:string}}>('/api/v1/interactions/:interactionId/answers',async(req,reply)=>{
    nativeBlock(nativeReady);
    const id=paramRequired(req.params.interactionId),owner=await getOwner();
    const body=req.body as Record<string,unknown>,key=uuidRequired(body?.operationId);
    const revision=body?.revision;
    if(!Number.isInteger(revision)||Number(revision)<1)httpFail(400,'INVALID_REVISION');
    const answer=body?.answer;
    if(!answer||typeof answer!=='object'||Array.isArray(answer))httpFail(400,'INVALID_ANSWER');
    const valid=answer as Record<string,unknown>;
    if(valid.kind==='PERMISSION') {
      if(!['ALLOW_ONCE','DENY'].includes(String(valid.decision)))httpFail(400,'INVALID_ANSWER');
    } else if(valid.kind==='FORM'){
      if(typeof valid.cancel!=='boolean'||!valid.values||typeof valid.values!=='object')httpFail(400,'INVALID_ANSWER');
    } else httpFail(400,'INVALID_ANSWER');
    const rows=await query<DbRow>(
      'SELECT i.* FROM wh_interactions i JOIN wh_tasks t ON t.id=i.task_id WHERE i.id=? AND t.owner_id=? LIMIT 1',[id,owner]);
    if(!rows.length)httpFail(404,'NOT_FOUND');
    const row=rows[0],taskId=String(row.task_id),runId=String(row.run_id);
    const result=await idempotent(owner,'INTERACTION_ANSWER',id,key,{revision,answer},async(tx,op)=>{
      const [locked]=await tx.execute<RowDataPacket[]>('SELECT * FROM wh_interactions WHERE id=? FOR UPDATE',[id]);
      if(!locked.length||locked[0].status!=='PENDING'||Number(locked[0].revision)!==revision)httpFail(409,'INTERACTION_STALE');
      if(String(locked[0].kind)!==valid.kind)httpFail(409,'INTERACTION_STALE');
      const current=await latest(tx,taskId);
      if(String(current.id)!==runId)httpFail(409,'INTERACTION_STALE');
      const now=utcNow();
      await tx.execute('UPDATE wh_interactions SET status=?,answer_json=?,updated_at=? WHERE id=?',
        ['ANSWERING',JSON.stringify(answer),now,id]);
      await outbox(tx,op,taskId,runId,'ANSWER',{interactionId:id,revision});
      return {value:operationResult(key,taskId,runId),taskId,runId,status:'ACCEPTED' as const};
    });
    return reply.status(202).send({data:result,requestId:req.id});
  });
}
