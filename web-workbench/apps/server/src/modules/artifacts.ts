import type { FastifyInstance } from 'fastify';
import { readFile, realpath, lstat } from 'node:fs/promises';
import { execFileSync } from 'node:child_process';
import { resolve, isAbsolute, relative } from 'node:path';
import { query, sha256, type DbRow } from '../../../../packages/storage/src/db.ts';
import { getOwnedTask, parsed, artifactSummary } from './tasks.ts';
import { paramRequired } from './ops.ts';
import { dataDir } from '../config.ts';
import { httpFail } from '../auth.ts';
import type { DTO } from '../../../../packages/contracts/src/domain.ts';

const oid=/^(?:[a-f0-9]{40}|[a-f0-9]{64})$/;
function within(root:string,path:string):boolean{
  const rel=relative(root,path);return rel===''||(!rel.startsWith('..')&&!isAbsolute(rel));
}
async function ownedArtifact(owner:string,id:string):Promise<{row:DbRow;scope:DTO<'Scope'>;body:Buffer}>{
  const rows=await query<DbRow>(
    'SELECT a.*,t.owner_id FROM wh_artifacts a JOIN wh_tasks t ON t.id=a.task_id WHERE a.id=? AND t.owner_id=?',[id,owner]);
  if(!rows.length)httpFail(404,'NOT_FOUND');
  const row=rows[0];
  const root=await realpath(dataDir);
  const file=await realpath(resolve(String(row.storage_key)));
  if(!within(root,file))httpFail(403,'ARTIFACT_PATH_INVALID');
  const info=await lstat(file);
  if(!info.isFile()||info.size>5_000_000)httpFail(422,'ARTIFACT_FILE_INVALID');
  const body=await readFile(file);
  if(sha256(body)!==row.sha256)httpFail(409,'ARTIFACT_HASH_MISMATCH');
  const task=await getOwnedTask(owner,String(row.task_id));
  return {row,scope:task.scope,body};
}
export function registerArtifacts(app:FastifyInstance,getOwner:()=>Promise<string>):void{
  app.get<{Params:{artifactId:string}}>('/api/v1/artifacts/:artifactId',async(req,reply)=>{
    const {row,scope,body}=await ownedArtifact(await getOwner(),paramRequired(req.params.artifactId));
    const data:DTO<'Artifact'>={meta:artifactSummary(row),taskId:String(row.task_id),scope,
      markdown:body.toString('utf8'),validation:parsed<Record<string,unknown>>(row.validation)};
    return reply.send({data,requestId:req.id});
  });
  app.get<{Params:{artifactId:string}}>('/api/v1/artifacts/:artifactId/download',async(req,reply)=>{
    const id=paramRequired(req.params.artifactId);
    const {body}=await ownedArtifact(await getOwner(),id);
    return reply.header('Content-Type','text/markdown; charset=utf-8')
      .header('Content-Disposition','attachment; filename="code-harness-review-'+id+'.md"')
      .header('Cache-Control','no-store').send(body);
  });
  app.get<{Params:{taskId:string}}>('/api/v1/tasks/:taskId/files',async(req,reply)=>{
    const taskId=paramRequired(req.params.taskId),owner=await getOwner();
    await getOwnedTask(owner,taskId);
    const rows=await query<DbRow>(
      'SELECT manifest,status FROM wh_scope_snapshots WHERE task_id=?',[taskId]);
    if(!rows.length||rows[0].status!=='READY')httpFail(409,'SNAPSHOT_NOT_READY');
    const manifest=parsed<{files:DTO<'File'>[];excluded?:DTO<'Exclusion'>[]}>(rows[0].manifest);
    const data:DTO<'FileList'>={items:manifest.files??[],excluded:manifest.excluded??[]};
    return reply.send({data,requestId:req.id});
  });
  app.get<{Params:{taskId:string;fileId:string};Querystring:{startLine?:string;endLine?:string}}>(
    '/api/v1/tasks/:taskId/files/:fileId',async(req,reply)=>{
      const owner=await getOwner(),taskId=paramRequired(req.params.taskId);
      await getOwnedTask(owner,taskId);
      const fileId=String(req.params.fileId);
      if(!/^[a-f0-9]{64}$/.test(fileId))httpFail(400,'FILE_ID_INVALID');
      const rows=await query<DbRow>(
        'SELECT manifest,status,storage_key FROM wh_scope_snapshots WHERE task_id=?',[taskId]);
      if(!rows.length||rows[0].status!=='READY')httpFail(409,'SNAPSHOT_NOT_READY');
      const manifest=parsed<{files:DTO<'File'>[]}>(rows[0].manifest);
      const file=manifest.files.find(x=>x.id===fileId);
      if(!file)httpFail(404,'NOT_FOUND');
      if(!oid.test(file.blobOID))httpFail(409,'FILE_BLOB_INVALID');
      const start=Number(req.query.startLine??1),end=Number(req.query.endLine??(start+199));
      if(!Number.isSafeInteger(start)||!Number.isSafeInteger(end)||start<1||end<start||end-start>499)
        httpFail(400,'FILE_RANGE_INVALID');
      const snapshot=await realpath(String(rows[0].storage_key)),root=await realpath(dataDir);
      if(!within(root,snapshot))httpFail(403,'SNAPSHOT_PATH_INVALID');
      let buffer:Buffer;
      try{
        buffer=execFileSync('git',['-C',snapshot,'cat-file','blob',file.blobOID],
          {timeout:15000,maxBuffer:4*1024*1024,env:{...process.env,GIT_TERMINAL_PROMPT:'0',GIT_NO_LAZY_FETCH:'1'}});
      }catch{httpFail(409,'FILE_BLOB_UNAVAILABLE');}
      if(buffer.includes(0)||buffer.length>3_000_000)httpFail(422,'UNSUPPORTED_FILE_TYPE');
      const all=buffer.toString('utf8').split(/\r?\n/);
      const lines=all.slice(start-1,end);
      const data:DTO<'FileLines'>={file,startLine:start,endLine:Math.min(end,all.length),lines,hasMore:end<all.length};
      return reply.send({data,requestId:req.id});
    });
}
