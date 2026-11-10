import type { FastifyInstance } from 'fastify';
import type { Pool, RowDataPacket } from 'mysql2/promise';
import { randomUUID } from 'node:crypto';
import type { DTO, Snapshot } from '../../../../../packages/contracts/src/domain.ts';
import { GitFailure } from '../../../../../packages/git-provider/src/repository.ts';
import { listSnapshotFiles, readSnapshotLines } from '../../../../../packages/git-provider/src/files.ts';
import { AuthProblem } from '../../auth.ts';
import type { LocalAuth } from '../../auth.ts';

export async function loadTaskSnapshot(pool:Pool,ownerId:string,taskId:string):Promise<Snapshot>{
  if(!/^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$/.test(taskId))
    throw new AuthProblem(400,'INVALID_TASK_ID');
  const [rows]=await pool.execute<RowDataPacket[]>(
    'SELECT s.task_id,s.head_oid,s.base_oid,s.merge_base_oid,s.base_ref,s.source_branch,'+
    's.object_format,s.storage_key,s.manifest_sha256,s.status '+
    'FROM wh_tasks t LEFT JOIN wh_scope_snapshots s ON s.task_id=t.id '+
    'WHERE t.owner_id=? AND t.id=?',[ownerId,taskId]);
  if(!rows.length)throw new AuthProblem(404,'NOT_FOUND');
  const r=rows[0];
  if(r.status!=='READY')throw new AuthProblem(409,'SNAPSHOT_NOT_READY');
  const scope:DTO<'Scope'>={headOID:String(r.head_oid),baseOID:String(r.base_oid),
    mergeBaseOID:String(r.merge_base_oid),baseRef:String(r.base_ref),
    sourceBranch:r.source_branch===null?null:String(r.source_branch),
    objectFormat:r.object_format};
  return{taskId,scope,storageKey:String(r.storage_key),manifestHash:String(r.manifest_sha256)};
}
const wrap=<T>(data:T)=>({data,requestId:randomUUID()});
export function registerTaskFileRoutes(app:FastifyInstance,opts:{
  pool:Pool;auth:LocalAuth;dataRoot:string;
}):void{
  app.get<{Params:{taskId:string}}>('/api/v1/tasks/:taskId/files',async(req,reply)=>{
    const {ownerId}=opts.auth.session(req);
    const snap=await loadTaskSnapshot(opts.pool,ownerId,req.params.taskId);
    try{return reply.send(wrap(await listSnapshotFiles(opts.dataRoot,snap)));}
    catch(e){if(e instanceof GitFailure)throw new AuthProblem(409,'SNAPSHOT_INTEGRITY_FAILED');throw e;}
  });
  app.get<{Params:{taskId:string;fileId:string};Querystring:{startLine?:string;endLine?:string}}>(
    '/api/v1/tasks/:taskId/files/:fileId',async(req,reply)=>{
      const {ownerId}=opts.auth.session(req);
      const snap=await loadTaskSnapshot(opts.pool,ownerId,req.params.taskId);
      const start=req.query.startLine===undefined?1:Number(req.query.startLine);
      const end=req.query.endLine===undefined?Math.min(start+199,500000):Number(req.query.endLine);
      if(!Number.isSafeInteger(start)||!Number.isSafeInteger(end)||
        start<1||end<start||end-start+1>500)throw new AuthProblem(400,'INVALID_LINE_RANGE');
      try{return reply.send(wrap(await readSnapshotLines(opts.dataRoot,snap,req.params.fileId,start,end)));}
      catch(e){if(e instanceof GitFailure){
        if(e.code==='FILE_NOT_BROWSABLE'||e.code==='INVALID_FILE_ID')
          throw new AuthProblem(404,'NOT_FOUND');
        throw new AuthProblem(409,'SNAPSHOT_INTEGRITY_FAILED');
      }throw e;}
    });
}
