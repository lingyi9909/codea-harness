import { createHash } from 'node:crypto';
import { lstat, readFile, realpath } from 'node:fs/promises';
import { isAbsolute, join, relative, resolve, sep } from 'node:path';
import type { Pool, RowDataPacket } from 'mysql2/promise';
import type { DTO } from '../../../../../packages/contracts/src/domain.ts';
import { AuthProblem } from '../../auth.ts';
import { checkUuid } from '../repositories/service.ts';

export const MAX_REPORT_BYTES=2*1024*1024;
const sha=(bytes:Buffer)=>createHash('sha256').update(bytes).digest('hex');
const inside=(root:string,p:string)=>{
  const path=relative(root,p);
  return path!==''&&path!=='..'&&!path.startsWith('..'+sep)&&!isAbsolute(path);
};
const iso=(s:unknown)=>String(s).replace(' ','T')+'Z';
export type TrustedReport={raw:Buffer;artifact:DTO<'Artifact'>};
function objectOrError(value:unknown):Record<string,unknown>{
  if(typeof value==='string'){
    try{value=JSON.parse(value) as unknown;}catch{throw new AuthProblem(503,'ARTIFACT_VALIDATION_CORRUPT');}
  }
  if(!value||typeof value!=='object'||Array.isArray(value))
    throw new AuthProblem(503,'ARTIFACT_VALIDATION_CORRUPT');
  return value as Record<string,unknown>;
}
export async function readOwnedArtifact(input:{
  pool:Pool;ownerId:string;artifactId:string;dataRoot:string;
}):Promise<TrustedReport>{
  checkUuid(input.artifactId);
  const [rows]=await input.pool.execute<RowDataPacket[]>(
    'SELECT a.*,s.status AS snapshot_status,s.head_oid,s.base_oid,s.merge_base_oid,'+
    's.base_ref,s.source_branch,s.object_format FROM wh_artifacts a '+
    'JOIN wh_tasks t ON t.id=a.task_id '+
    'JOIN wh_scope_snapshots s ON s.task_id=t.id '+
    'WHERE a.id=? AND t.owner_id=?',
    [input.artifactId,input.ownerId]);
  if(rows.length!==1)throw new AuthProblem(404,'NOT_FOUND');
  const r=rows[0];
  if(r.snapshot_status!=='READY'||r.kind!=='REVIEW_MARKDOWN'||
    !['FULL','PARTIAL'].includes(r.coverage))
    throw new AuthProblem(409,'ARTIFACT_NOT_TRUSTED');
  const taskId=checkUuid(r.task_id),runId=checkUuid(r.run_id);
  const expected=['artifacts',taskId,runId+'.md'].join('/');
  if(String(r.storage_key)!==expected||
    !/^[a-f0-9]{64}$/.test(String(r.sha256)))
    throw new AuthProblem(409,'ARTIFACT_STORAGE_INVALID');
  const root=await realpath(resolve(input.dataRoot));
  const absolute=join(root,...expected.split('/'));
  let canonical:string;
  try{canonical=await realpath(absolute);}
  catch{throw new AuthProblem(409,'ARTIFACT_MISSING');}
  if(!inside(root,canonical))throw new AuthProblem(409,'ARTIFACT_STORAGE_ESCAPE');
  const stat=await lstat(absolute);
  if(!stat.isFile()||stat.isSymbolicLink()||stat.size<1||stat.size>MAX_REPORT_BYTES)
    throw new AuthProblem(409,'ARTIFACT_INVALID_FILE');
  const raw=await readFile(canonical);
  if(raw.length!==Number(r.size_bytes)||sha(raw)!==String(r.sha256))
    throw new AuthProblem(409,'ARTIFACT_HASH_MISMATCH');
  const markdown=raw.toString('utf8');
  if(!Buffer.from(markdown,'utf8').equals(raw)||raw.includes(0))
    throw new AuthProblem(409,'ARTIFACT_ENCODING_INVALID');
  const scope:DTO<'Scope'>={
    headOID:String(r.head_oid),baseOID:String(r.base_oid),mergeBaseOID:String(r.merge_base_oid),
    baseRef:String(r.base_ref),
    sourceBranch:r.source_branch===null?null:String(r.source_branch),objectFormat:r.object_format,
  };
  const meta:DTO<'ArtifactSummary'>={id:String(r.id),runId,version:Number(r.version),
    kind:'REVIEW_MARKDOWN',sha256:String(r.sha256),sizeBytes:String(r.size_bytes),
    coverage:r.coverage,createdAt:iso(r.created_at)};
  return {raw,artifact:{meta,taskId,scope,markdown,validation:objectOrError(r.validation)}};
}
