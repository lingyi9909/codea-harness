import { randomUUID } from 'node:crypto';
import type { Pool, RowDataPacket, PoolConnection } from 'mysql2/promise';
import type { DTO } from '../../../../../packages/contracts/src/domain.ts';
import { withTransaction } from '../../../../../packages/storage/src/db.ts';
import { GitFailure, sha256, trustedRepository } from '../../../../../packages/git-provider/src/repository.ts';
import { previewCommittedChanges } from '../../../../../packages/git-provider/src/preview.ts';
import { AuthProblem } from '../../auth.ts';
import { checkUuid, findRepository } from './service.ts';

type PreviewOpts={pool:Pool;roots:string[];refKey:string};
const iso=(s:string)=>s.replace(' ','T')+'Z';
function json<T>(value:unknown):T{
  if(typeof value==='string')return JSON.parse(value) as T;
  if(value&&typeof value==='object')return value as T;
  throw new AuthProblem(503,'INVALID_STORED_JSON');
}
function mapPreview(row:RowDataPacket):DTO<'Preview'>{
  const summary=json<{changedFiles:number;applicableFiles:number;dirtyCounts:DTO<'DirtyCounts'>;
    exclusions:DTO<'Exclusion'>[];notice:string;}>(row.summary);
  return {id:String(row.id),repositoryId:String(row.repository_id),
    scope:{headOID:String(row.head_oid),baseOID:String(row.base_oid),
      mergeBaseOID:String(row.merge_base_oid),baseRef:String(row.base_ref),
      sourceBranch:row.source_branch===null?null:String(row.source_branch),
      objectFormat:row.object_format},
    changedFiles:summary.changedFiles,applicableFiles:summary.applicableFiles,
    dirtyCounts:summary.dirtyCounts,exclusions:summary.exclusions,
    expiresAt:iso(String(row.expires_at)),notice:summary.notice};
}
async function savedPreview(tx:Pool|PoolConnection,ownerId:string,id:string):Promise<DTO<'Preview'>>{
  const [rows]=await tx.execute<RowDataPacket[]>(
    'SELECT * FROM wh_review_previews WHERE owner_id=? AND id=?',[ownerId,id]);
  if(!rows.length)throw new AuthProblem(503,'OPERATION_RECONCILIATION_REQUIRED');
  return mapPreview(rows[0]);
}
async function previous(tx:Pool|PoolConnection,ownerId:string,key:string,hash:string):
  Promise<DTO<'Preview'>|null>{
  const [rows]=await tx.execute<RowDataPacket[]>(
    'SELECT action,request_sha256,result_json FROM wh_operations WHERE owner_id=? AND operation_key=?',
    [ownerId,key]);
  if(!rows.length)return null;
  if(rows[0].action!=='REVIEW_PREVIEW'||rows[0].request_sha256!==hash)
    throw new AuthProblem(409,'OPERATION_CONFLICT');
  const result=json<{previewId:string}>(rows[0].result_json);
  return savedPreview(tx,ownerId,result.previewId);
}
export async function createPreview(input:{
  ownerId:string;operationId:string;repositoryId:string;baseRefId:string;
},opts:PreviewOpts):Promise<DTO<'Preview'>>{
  checkUuid(input.operationId);checkUuid(input.repositoryId);
  if(!/^[0-9a-f]{64}$/.test(input.baseRefId))throw new AuthProblem(400,'INVALID_BASE_REF');
  if(opts.refKey.length<32)throw new AuthProblem(503,'REF_SIGNING_KEY_REQUIRED');
  const hash=sha256(JSON.stringify({action:'REVIEW_PREVIEW',
    repositoryId:input.repositoryId,baseRefId:input.baseRefId}));
  // Idempotency is checked BEFORE Git so a replay cannot silently change its source OIDs.
  const cached=await previous(opts.pool,input.ownerId,input.operationId,hash);
  if(cached)return cached;
  const repository=await findRepository(opts.pool,input.ownerId,input.repositoryId);
  if(repository.status!=='READY')throw new AuthProblem(409,'REPOSITORY_UNAVAILABLE');
  let proof:Awaited<ReturnType<typeof previewCommittedChanges>>;
  try {
    const root=await trustedRepository(repository.path,opts.roots);
    proof=await previewCommittedChanges(root,repository.id,opts.refKey,input.baseRefId);
  }catch(e){
    if(e instanceof GitFailure){
      const code=e.code;
      throw new AuthProblem(
        code==='NO_CHANGES'||code==='NO_APPLICABLE_FILES'||code==='BASE_REF_MISSING'||
        code==='NO_COMMON_ANCESTOR'?422:409,code);
    }
    throw e;
  }
  return withTransaction(opts.pool,async tx=>{
    await tx.execute('SELECT id FROM wh_owners WHERE id=? FOR UPDATE',[input.ownerId]);
    const existing=await previous(tx,input.ownerId,input.operationId,hash);
    if(existing)return existing;
    const id=randomUUID(),scope=proof.scope;
    const summary=JSON.stringify({
      changedFiles:proof.changedFiles,applicableFiles:proof.applicableFiles,
      dirtyCounts:proof.dirtyCounts,exclusions:proof.exclusions,notice:proof.notice});
    await tx.execute(
      `INSERT INTO wh_review_previews(
        id,owner_id,repository_id,base_ref_id,base_ref,source_branch,head_oid,base_oid,
        merge_base_oid,object_format,summary,expires_at,created_at
      ) VALUES (?,?,?,?,?,?,?,?,?,?,?,DATE_ADD(UTC_TIMESTAMP(3), INTERVAL 300 SECOND),UTC_TIMESTAMP(3))`,
      [id,input.ownerId,repository.id,input.baseRefId,scope.baseRef,scope.sourceBranch,
        scope.headOID,scope.baseOID,scope.mergeBaseOID,scope.objectFormat,summary]);
    await tx.execute(
      `INSERT INTO wh_operations(
        id,owner_id,operation_key,action,request_sha256,status,result_json,created_at,updated_at
      ) VALUES (?,?,?,?,?,'SUCCEEDED',?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))`,
      [randomUUID(),input.ownerId,input.operationId,'REVIEW_PREVIEW',hash,JSON.stringify({previewId:id})]);
    return savedPreview(tx,input.ownerId,id);
  });
}
