import type { Pool, RowDataPacket } from 'mysql2/promise';
import { randomUUID } from 'node:crypto';
import type { DTO } from '../../../../../packages/contracts/src/domain.ts';
import { withTransaction } from '../../../../../packages/storage/src/db.ts';
import { GitFailure, sha256, trustedRepository } from '../../../../../packages/git-provider/src/repository.ts';
import { AuthProblem } from '../../auth.ts';

export type RepositoryOptions={pool:Pool;roots:string[]};
const date=(value:string)=>value.replace(' ','T')+'Z';
const id=/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
export function checkUuid(value:unknown):string{
  if(typeof value!=='string'||!id.test(value))throw new AuthProblem(400,'INVALID_UUID');
  return value;
}
function asRepository(r:RowDataPacket):DTO<'Repository'>{
  return {id:String(r.id),name:String(r.name),path:String(r.canonical_path),
    status:r.status,createdAt:date(String(r.created_at))};
}
export async function findRepository(pool:Pool,ownerId:string,repoId:string):Promise<DTO<'Repository'>>{
  const [rows]=await pool.execute<RowDataPacket[]>(
    'SELECT id,name,canonical_path,status,created_at FROM wh_repositories WHERE owner_id=? AND id=?',
    [ownerId,repoId]);
  if(!rows.length)throw new AuthProblem(404,'NOT_FOUND');
  return asRepository(rows[0]);
}
export async function listRepositories(pool:Pool,ownerId:string,limit:number):Promise<DTO<'Repository'>[]>{
  const [rows]=await pool.execute<RowDataPacket[]>(
    'SELECT id,name,canonical_path,status,created_at FROM wh_repositories WHERE owner_id=? ORDER BY created_at DESC,id DESC LIMIT ?',
    [ownerId,limit]);
  return rows.map(asRepository);
}
export async function registerRepository(input:{
  ownerId:string;operationId:string;name:string;path:string;
},opts:RepositoryOptions):Promise<DTO<'Repository'>>{
  checkUuid(input.operationId);
  if(!input.name?.trim()||input.name.length>100||!input.path?.trim()||input.path.length>4096)
    throw new AuthProblem(400,'INVALID_REPOSITORY');
  let canonical:string;
  try{canonical=await trustedRepository(input.path,opts.roots);}
  catch(e){
    if(e instanceof GitFailure)throw new AuthProblem(e.code==='ROOT_NOT_ALLOWED'?403:422,e.code);
    throw new AuthProblem(422,'GIT_REPOSITORY_UNAVAILABLE');
  }
  const name=input.name.trim(),pathHash=sha256(canonical),requestHash=sha256(JSON.stringify({
    action:'REPOSITORY_REGISTER',name,path:canonical,
  }));
  return withTransaction(opts.pool,async tx=>{
    // Serialize local owner's operations; native dispatch never runs within a DB tx.
    await tx.execute('SELECT id FROM wh_owners WHERE id=? FOR UPDATE',[input.ownerId]);
    const [operations]=await tx.execute<RowDataPacket[]>(
      'SELECT action,request_sha256,result_json FROM wh_operations WHERE owner_id=? AND operation_key=? FOR UPDATE',
      [input.ownerId,input.operationId]);
    if(operations.length){
      if(operations[0].action!=='REPOSITORY_REGISTER'||operations[0].request_sha256!==requestHash)
        throw new AuthProblem(409,'OPERATION_CONFLICT');
      const result=typeof operations[0].result_json==='string'?
        JSON.parse(operations[0].result_json):operations[0].result_json;
      const [saved]=await tx.execute<RowDataPacket[]>(
        'SELECT id,name,canonical_path,status,created_at FROM wh_repositories WHERE owner_id=? AND id=?',
        [input.ownerId,result.repositoryId]);
      if(!saved.length)throw new AuthProblem(503,'OPERATION_RECONCILIATION_REQUIRED');
      return asRepository(saved[0]);
    }
    let [rows]=await tx.execute<RowDataPacket[]>(
      'SELECT id,name,canonical_path,status,created_at FROM wh_repositories WHERE owner_id=? AND path_sha256=? FOR UPDATE',
      [input.ownerId,pathHash]);
    let repository:DTO<'Repository'>;
    if(rows.length){repository=asRepository(rows[0]);}
    else {
      const repoId=randomUUID();
      await tx.execute(
        'INSERT INTO wh_repositories(id,owner_id,name,canonical_path,path_sha256,status,created_at,updated_at) VALUES (?,?,?,?,?,\'READY\',UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))',
        [repoId,input.ownerId,name,canonical,pathHash]);
      [rows]=await tx.execute<RowDataPacket[]>(
        'SELECT id,name,canonical_path,status,created_at FROM wh_repositories WHERE owner_id=? AND id=?',
        [input.ownerId,repoId]);
      repository=asRepository(rows[0]);
    }
    await tx.execute(
      'INSERT INTO wh_operations(id,owner_id,operation_key,action,request_sha256,status,result_json,created_at,updated_at) VALUES (?,?,?,?,?,\'SUCCEEDED\',?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))',
      [randomUUID(),input.ownerId,input.operationId,'REPOSITORY_REGISTER',requestHash,
        JSON.stringify({repositoryId:repository.id})]);
    return repository;
  });
}
