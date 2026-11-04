import type { FastifyInstance } from 'fastify';
import type { RowDataPacket } from 'mysql2/promise';
import type { DTO } from '../../../../packages/contracts/src/domain.ts';
import { query, utcNow, uuid, type DbRow } from '../../../../packages/storage/src/db.ts';
import { httpFail } from '../auth.ts';
import { getRefs, previewCommitted, repositoryIdentity, validateLocalRepository } from './git.ts';
import { idempotent, paramRequired, stringRequired, uuidRequired } from './ops.ts';

const iso=(text:unknown)=>new Date(String(text).replace(' ','T')+'Z').toISOString();
const mapRepo=(row:DbRow):DTO<'Repository'>=>({
  id:String(row.id),name:String(row.name),path:String(row.canonical_path),
  status:row.status as DTO<'Repository'>['status'],createdAt:iso(row.created_at),
});
export async function ownedRepository(owner:string,id:string):Promise<DTO<'Repository'>> {
  const rows=await query<DbRow>('SELECT * FROM wh_repositories WHERE id=? AND owner_id=? LIMIT 1',[id,owner]);
  if(!rows.length)httpFail(404,'NOT_FOUND');
  return mapRepo(rows[0]);
}
export function registerRepositories(app:FastifyInstance,getOwner:()=>Promise<string>):void{
  app.get('/api/v1/repositories',async(req,reply)=>{
    const owner=await getOwner();
    const rows=await query<DbRow>('SELECT * FROM wh_repositories WHERE owner_id=? ORDER BY created_at DESC,id DESC LIMIT 100',[owner]);
    return reply.send({data:{items:rows.map(mapRepo),nextCursor:null},requestId:req.id});
  });
  app.post('/api/v1/repositories',async(req,reply)=>{
    const body=req.body as Record<string,unknown>;
    const key=uuidRequired(body?.operationId);
    const name=stringRequired(body?.name,100);
    const path=validateLocalRepository(stringRequired(body?.path,4096));
    const owner=await getOwner();
    const value=await idempotent(owner,'REPOSITORY_REGISTER',owner,key,{name,path},async tx=>{
      const pathHash=repositoryIdentity(path);
      const [old]=await tx.execute<RowDataPacket[]>(
        'SELECT * FROM wh_repositories WHERE owner_id=? AND path_sha256=? LIMIT 1 FOR UPDATE',[owner,pathHash]);
      if(old.length)return{value:mapRepo(old[0] as DbRow)};
      const id=uuid(),now=utcNow();
      await tx.execute('INSERT INTO wh_repositories(id,owner_id,name,canonical_path,path_sha256,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)',
        [id,owner,name,path,pathHash,'READY',now,now]);
      return{value:{id,name,path,status:'READY' as const,createdAt:new Date().toISOString()}};
    });
    return reply.status(201).send({data:value,requestId:req.id});
  });
  app.get<{Params:{repositoryId:string}}>('/api/v1/repositories/:repositoryId/refs',async(req,reply)=>{
    const repo=await ownedRepository(await getOwner(),paramRequired(req.params.repositoryId));
    return reply.send({data:getRefs(repo),requestId:req.id});
  });
  app.post('/api/v1/review-previews',async(req,reply)=>{
    const body=req.body as Record<string,unknown>;
    const owner=await getOwner(),key=uuidRequired(body?.operationId);
    const repositoryId=paramRequired(body?.repositoryId);
    const baseRefId=stringRequired(body?.baseRefId,2048);
    const repo=await ownedRepository(owner,repositoryId);
    let preview:DTO<'Preview'>;
    try{preview=previewCommitted(repo,baseRefId);}
    catch(e){
      const message=e instanceof Error?e.message:'UNSUPPORTED_REPOSITORY';
      if(['BASE_REF_MISSING','NO_COMMON_ANCESTOR','NO_CHANGES','NO_APPLICABLE_FILES'].includes(message))httpFail(422,message);
      throw e;
    }
    const result=await idempotent(owner,'REVIEW_PREVIEW',repositoryId,key,{repositoryId,baseRefId},async tx=>{
      const id=uuid(),record={...preview,id},scope=record.scope;
      const mysqlExpire=new Date(record.expiresAt).toISOString().slice(0,23).replace('T',' ');
      await tx.execute(
        'INSERT INTO wh_review_previews(id,owner_id,repository_id,base_ref_id,base_ref,source_branch,head_oid,base_oid,merge_base_oid,object_format,summary,expires_at,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)',
        [id,owner,repositoryId,baseRefId,scope.baseRef,scope.sourceBranch,
          scope.headOID,scope.baseOID,scope.mergeBaseOID,scope.objectFormat,JSON.stringify(record),
          mysqlExpire,utcNow()]);
      return {value:record};
    });
    return reply.status(201).send({data:result,requestId:req.id});
  });
}
