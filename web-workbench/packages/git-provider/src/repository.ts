import { execFile } from 'node:child_process';
import { createHmac, createHash } from 'node:crypto';
import { promisify } from 'node:util';
import { realpath, lstat } from 'node:fs/promises';
import { isAbsolute, relative, resolve } from 'node:path';
const run=promisify(execFile);
export class GitFailure extends Error { readonly code:string; constructor(code:string) { super(code); this.code=code; } }
const OID=/^(?:[0-9a-f]{40}|[0-9a-f]{64})$/;
export async function git(root:string,args:string[]):Promise<string>{
  if (args.some(a=>a.includes('\0'))) throw new GitFailure('INVALID_GIT_ARGUMENT');
  try {
    const r=await run('git',['-C',root,...args],{shell:false,windowsHide:true,
      timeout:15000,maxBuffer:16000000,encoding:'utf8',
      env:{...process.env,GIT_TERMINAL_PROMPT:'0',GIT_NO_LAZY_FETCH:'1',GIT_OPTIONAL_LOCKS:'0'}});
    return r.stdout.trimEnd();
  } catch { throw new GitFailure('GIT_READ_FAILED'); }
}
const inside=(root:string,p:string)=>{
  const rel=relative(root,p);
  return rel===''||(!rel.startsWith('..')&&!isAbsolute(rel));
};
export async function trustedRepository(path:string,roots:string[]):Promise<string>{
  if(!isAbsolute(path)||!roots.length)throw new GitFailure('ROOT_NOT_ALLOWED');
  const root=await realpath(resolve(path));
  const accepted=await Promise.all(roots.map(r=>realpath(resolve(r))));
  if(!accepted.some(r=>inside(r,root)))throw new GitFailure('ROOT_NOT_ALLOWED');
  if((await lstat(resolve(path))).isSymbolicLink())throw new GitFailure('ROOT_SYMLINK_NOT_ALLOWED');
  if(await git(root,['rev-parse','--show-prefix'])!=='')throw new GitFailure('GIT_ROOT_REQUIRED');
  const common=await git(root,['rev-parse','--git-common-dir']);
  if(common!=='.git'&&common!=='./.git')throw new GitFailure('SHARED_WORKTREE_NOT_ALLOWED');
  return root;
}
export interface Ref { id:string; fullName:string; oid:string; kind:'LOCAL_BRANCH'|'REMOTE_TRACKING' }
export interface Refs {items:Ref[];headOID:string;branch:string|null;objectFormat:'sha1'|'sha256';observedAt:string}
export function opaqueRef(secret:string,repositoryId:string,fullName:string,oid:string):string{
  return createHmac('sha256',secret).update(repositoryId+'\0'+fullName+'\0'+oid).digest('hex');
}
export function sha256(s:string):string{return createHash('sha256').update(s).digest('hex')}
export async function getRefs(root:string,repoId:string,secret:string):Promise<Refs>{
  const [raw,headOID,objectFormat]=await Promise.all([
    git(root,['for-each-ref','--format=%(refname) %(objectname) %(objecttype)','refs/heads','refs/remotes']),
    git(root,['rev-parse','--verify','HEAD']),
    git(root,['rev-parse','--show-object-format']),
  ]);
  if(!OID.test(headOID)||(objectFormat!=='sha1'&&objectFormat!=='sha256'))
    throw new GitFailure('GIT_OID_INVALID');
  const items:Ref[]=[];
  for (const line of raw.split(/\r?\n/).filter(Boolean)){
    const match=line.match(/^(refs\/(heads|remotes)\/\S+) ([0-9a-f]{40}|[0-9a-f]{64}) commit$/);
    if(!match)continue;
    items.push({id:opaqueRef(secret,repoId,match[1],match[3]),fullName:match[1],oid:match[3],
      kind:match[2]==='heads'?'LOCAL_BRANCH':'REMOTE_TRACKING'});
  }
  let branch:string|null=null;
  try {branch=await git(root,['symbolic-ref','--quiet','--short','HEAD']);}catch{}
  return {items,headOID,branch,objectFormat,observedAt:new Date().toISOString()};
}
