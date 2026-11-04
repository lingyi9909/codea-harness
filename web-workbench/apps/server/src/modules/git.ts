import { createHmac, randomBytes } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { realpathSync, existsSync, readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { dirname, isAbsolute, relative, resolve, join } from 'node:path';
import type { DTO } from '../../../../packages/contracts/src/domain.ts';
import { allowedRoots, dataDir } from '../config.ts';
import { sha256 } from '../../../../packages/storage/src/db.ts';

type Repository = DTO<'Repository'>;
type Refs = DTO<'Refs'>;
type Scope = DTO<'Scope'>;
type Preview = DTO<'Preview'>;
const MAX_OUTPUT=32*1024*1024;
const HASH=/^(?:[0-9a-f]{40}|[0-9a-f]{64})$/;
const CTRL=/^[A-Za-z_][A-Za-z0-9_]*$/;
function git(root:string,args:string[],nul=false):string|Buffer{
  const out=execFileSync('git',['-C',root,...args],{encoding:nul?undefined:'utf8',
    timeout:15000,maxBuffer:MAX_OUTPUT,stdio:['ignore','pipe','ignore'],
    env:{...process.env,GIT_TERMINAL_PROMPT:'0',GIT_OPTIONAL_LOCKS:'0',GIT_NO_LAZY_FETCH:'1'}});
  return nul?out as Buffer:(out as string).trim();
}
const cleanPath=(root:string,path:string):boolean=>{
  const rel=relative(root,path);
  return rel===''||(!rel.startsWith('..')&&!isAbsolute(rel));
};
function canonical(value:string):string{return realpathSync(resolve(value));}
function secret():Buffer{
  const path=join(dataDir,'ref-signing.key');
  mkdirSync(dirname(path),{recursive:true,mode:0o700});
  if(!existsSync(path)){
    try{writeFileSync(path,randomBytes(32),{flag:'wx',mode:0o600});}
    catch(e){if(!existsSync(path))throw e;}
  }
  const bytes=readFileSync(path);if(bytes.length!==32)throw new Error('REF_SECRET_INVALID');
  return bytes;
}
function refId(repositoryId:string,name:string,oid:string):string {
  return createHmac('sha256',secret()).update(JSON.stringify({repositoryId,name,oid})).digest('hex');
}
export function validateLocalRepository(path:string):string {
  if(!path||!isAbsolute(path))throw new Error('REPOSITORY_ABSOLUTE_PATH_REQUIRED');
  const candidate=canonical(path);
  if(!allowedRoots.length||!allowedRoots.some(r=>{
    try{return cleanPath(canonical(r),candidate)}catch{return false}
  }))throw new Error('ROOT_NOT_ALLOWED');
  const top=git(candidate,['rev-parse','--show-toplevel']) as string;
  if(!cleanPath(candidate,canonical(top))||!cleanPath(canonical(top),candidate))
    throw new Error('REPOSITORY_NOT_GIT_ROOT');
  if(git(candidate,['rev-parse','--is-inside-work-tree'])!=='true')
    throw new Error('UNSUPPORTED_REPOSITORY');
  // Reject partial/promisor clone; reading missing objects might trigger an implicit fetch.
  for(const flags of [['config','--get','extensions.partialclone'],['config','--name-only','--get-regexp','promisor']]){
    try{git(candidate,flags);throw new Error('PROMISOR_REPOSITORY_NOT_ALLOWED');}
    catch(e){if(e instanceof Error&&e.message==='PROMISOR_REPOSITORY_NOT_ALLOWED')throw e;
      const status=(e as {status?:number}).status;
      if(status!==1)throw new Error('GIT_CONFIG_QUERY_FAILED');
    }
  }
  return candidate;
}
export function repositoryIdentity(path:string){return sha256(process.platform==='win32'?path.toLowerCase():path);}
export function getRefs(repository:Repository):Refs {
  const root=validateLocalRepository(repository.path);
  const headOID=git(root,['rev-parse','--verify','HEAD']) as string;
  if(!HASH.test(headOID))throw new Error('HEAD_OID_INVALID');
  const raw=git(root,['for-each-ref','--format=%(refname)%09%(objectname)','refs/heads','refs/remotes']) as string;
  const items:Refs['items']=[];
  for(const line of raw.split(/\r?\n/)){
    if(!line)continue;
    const [name,oid]=line.split('\t');
    if(!name||!oid||!HASH.test(oid)||name.endsWith('/HEAD'))continue;
    const kind=name.startsWith('refs/remotes/')?'REMOTE_TRACKING':'LOCAL_BRANCH';
    items.push({id:refId(repository.id,name,oid),fullName:name,oid,kind});
  }
  return {
    items,headOID,branch:(git(root,['symbolic-ref','--quiet','--short','HEAD']) as string)||null,
    objectFormat:headOID.length===64?'sha256':'sha1',observedAt:new Date().toISOString(),
  };
}
function splitNul(data:Buffer):string[]{
  return data.toString('utf8').split('\0').filter(Boolean);
}
function applicable(path:string):boolean{
  if(path.split('/').some(p=>p.startsWith('.')))return false;
  if(/^(?:target|build|dist|node_modules)\//i.test(path)||path.startsWith('temp/'))return false;
  return /(?:\.java|\.xml|\.sql|\.ya?ml|\.properties|\.gradle|pom\.xml)$/i.test(path);
}
export function previewCommitted(repository:Repository,baseRefId:string):Preview {
  const refs=getRefs(repository);
  const ref=refs.items.find(x=>x.id===baseRefId);
  if(!ref)throw new Error('BASE_REF_MISSING');
  const root=validateLocalRepository(repository.path);
  const mergeBaseOID=git(root,['merge-base',ref.oid,refs.headOID]) as string;
  if(!HASH.test(mergeBaseOID))throw new Error('NO_COMMON_ANCESTOR');
  const pathspec=['--','.',' :(exclude,glob)**/.*/**'.trim(),' :(exclude,glob)temp/**'.trim()];
  const changed=splitNul(git(root,['diff','--no-ext-diff','--no-textconv','--name-only','-z',mergeBaseOID,refs.headOID,...pathspec],true) as Buffer);
  if(!changed.length)throw new Error('NO_CHANGES');
  const valid=changed.filter(applicable);
  if(!valid.length)throw new Error('NO_APPLICABLE_FILES');
  const staged=splitNul(git(root,['diff','--no-ext-diff','--no-textconv','--name-only','-z','--cached','HEAD','--'],true) as Buffer);
  const unstaged=splitNul(git(root,['diff','--no-ext-diff','--no-textconv','--name-only','-z','--'],true) as Buffer);
  const untracked=splitNul(git(root,['ls-files','--others','--exclude-standard','-z','--'],true) as Buffer);
  const scope:Scope={headOID:refs.headOID,baseOID:ref.oid,mergeBaseOID,
    baseRef:ref.fullName,sourceBranch:refs.branch,objectFormat:refs.objectFormat};
  return {
    id:'',repositoryId:repository.id,scope,changedFiles:changed.length,applicableFiles:valid.length,
    dirtyCounts:{staged:staged.length,unstaged:unstaged.length,untracked:untracked.length},
    exclusions:changed.filter(x=>!applicable(x)).map(path=>({path,reason:'非业务文件或排除目录'})),
    expiresAt:new Date(Date.now()+300000).toISOString(),
    notice:'只读取固定 HEAD 与 merge-base 之间的已提交差异；未提交文件不纳入本次 Review。',
  };
}
export function confirmPreviewStillCurrent(repository:Repository,scope:Scope,baseRefId:string):void{
  const current=getRefs(repository);
  const same=current.items.find(x=>x.id===baseRefId);
  if(current.headOID!==scope.headOID||!same||same.oid!==scope.baseOID)
    throw new Error('SCOPE_CHANGED');
  const merge=git(repository.path,['merge-base',same.oid,current.headOID]) as string;
  if(merge!==scope.mergeBaseOID)throw new Error('SCOPE_CHANGED');
}
export function previewFileList(root:string,base:string,head:string):string[]{
  if(!HASH.test(base)||!HASH.test(head))throw new Error('INVALID_SNAPSHOT_OID');
  return splitNul(git(root,['diff','--no-ext-diff','--no-textconv','--name-only','-z',base,head,'--','.'],true) as Buffer).filter(applicable);
}
