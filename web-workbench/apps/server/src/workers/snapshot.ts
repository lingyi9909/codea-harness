import { execFileSync } from 'node:child_process';
import { createHash, randomUUID } from 'node:crypto';
import { mkdtemp, mkdir, writeFile, rename, rm, realpath } from 'node:fs/promises';
import { basename, dirname, isAbsolute, join, relative, resolve } from 'node:path';
import type { DTO } from '../../../../packages/contracts/src/domain.ts';
import { inspectFrozenSnapshot, originalRepositoryFingerprint } from '../../../../packages/opencode-adapter/src/git-guard.ts';
import { sha256 } from '../../../../packages/storage/src/db.ts';
import { previewFileList, validateLocalRepository } from '../modules/git.ts';

const OID=/^(?:[0-9a-f]{40}|[0-9a-f]{64})$/;
const env={
  ...process.env,
  GIT_TERMINAL_PROMPT:'0',GIT_OPTIONAL_LOCKS:'0',GIT_NO_LAZY_FETCH:'1',
  GIT_LFS_SKIP_SMUDGE:'1',GIT_ALLOW_PROTOCOL:'file',GIT_CONFIG_NOSYSTEM:'1',
};
function git(args:string[],limit=16*1024*1024):Buffer{
  return execFileSync('git',args,{env,timeout:60000,maxBuffer:limit,stdio:['ignore','pipe','ignore']});
}
function text(args:string[]):string{return git(args).toString('utf8').trim();}
function safePath(path:string):boolean{
  if(!path||path.startsWith('/')||path.includes('\\')||path.includes('\0'))return false;
  const parts=path.split('/');
  if(parts.some(p=>!p||p==='.'||p==='..'||p.startsWith('.opencode')||p==='.teamai'||p==='.agents'||/^(AGENTS|CLAUDE|OPENCODE)\.md$/i.test(p)||
    /[\x00-\x1f<>:"|?*]/.test(p)||/[. ]$/.test(p)||/^(CON|PRN|AUX|NUL|COM[1-9]|LPT[1-9])(\..*)?$/i.test(p)))
    return false;
  return true;
}
function inside(root:string,path:string):boolean{
  const rel=relative(root,path);
  return rel!==''&&!rel.startsWith('..')&&!isAbsolute(rel);
}
function countLines(content:Buffer):number{
  if(!content.length)return 0;
  let count=1;
  for(const byte of content)if(byte===10)count++;
  if(content[content.length-1]===10)count--;
  return count;
}
export type FrozenTaskSnapshot={
  storageKey:string;
  manifest: {
    schema:'frozen-scope-v1';scope:DTO<'Scope'>;files:DTO<'File'>[];
    excluded:DTO<'Exclusion'>[];sourceFingerprint:string;
  };
  manifestSha256:string;
};
export async function makeFrozenSnapshot(input:{
  taskId:string;repositoryPath:string;taskDirectory:string;scope:DTO<'Scope'>;
}):Promise<FrozenTaskSnapshot>{
  const {taskId,scope}=input;
  if(!OID.test(scope.headOID)||!OID.test(scope.mergeBaseOID)||!OID.test(scope.baseOID))
    throw new Error('SNAPSHOT_OID_INVALID');
  const source=validateLocalRepository(input.repositoryPath);
  const before=originalRepositoryFingerprint(source);
  const actualHead=text(['-C',source,'rev-parse','HEAD']);
  if(actualHead!==scope.headOID)throw new Error('SNAPSHOT_SOURCE_HEAD_CHANGED');
  const parent=resolve(input.taskDirectory);
  await mkdir(parent,{recursive:true,mode:0o700});
  const temporary=await mkdtemp(join(parent,'.snapshot-'));
  const finalPath=join(parent,'source');
  // mkdtemp created temporary; clone requires destination not to exist.
  await rm(temporary,{recursive:true,force:true});
  let committed=false;
  try{
    // Local clone copies object storage. No HTTP, no fetch/pull, no checkout/smudge filters.
    git(['clone','--no-checkout','--no-hardlinks','--',source,temporary],32*1024*1024);
    git(['-C',temporary,'remote','remove','origin']);
    git(['-C',temporary,'config','core.autocrlf','false']);
    git(['-C',temporary,'config','core.fsmonitor','false']);
    git(['-C',temporary,'config','core.hooksPath','__disabled_hooks__']);
    const verifiedHead=text(['-C',temporary,'cat-file','-t',scope.headOID]);
    const verifiedBase=text(['-C',temporary,'cat-file','-t',scope.mergeBaseOID]);
    if(verifiedHead!=='commit'||verifiedBase!=='commit')throw new Error('SNAPSHOT_OBJECT_NOT_COMMIT');
    git(['-C',temporary,'update-ref','refs/heads/harness-target',scope.headOID]);
    git(['-C',temporary,'update-ref','refs/heads/harness-review-base',scope.mergeBaseOID]);
    git(['-C',temporary,'symbolic-ref','HEAD','refs/heads/harness-target']);
    git(['-C',temporary,'read-tree','--reset',scope.headOID]);
    const raw=git(['-C',temporary,'ls-tree','-rz',scope.headOID],32*1024*1024);
    const paths=new Set<string>();
    let cumulative=0;
    const files:Map<string,{oid:string;content:Buffer}>=new Map();
    for(const line of raw.toString('utf8').split('\0').filter(Boolean)){
      const tab=line.indexOf('\t');
      if(tab<0)throw new Error('SNAPSHOT_TREE_ENTRY_INVALID');
      const [mode,type,oid]=line.slice(0,tab).split(' ');
      const path=line.slice(tab+1);
      if(!safePath(path)||paths.has(path.toLowerCase()))throw new Error('SNAPSHOT_PATH_UNSAFE_OR_COLLISION');
      paths.add(path.toLowerCase());
      // Explicitly reject symlinks, submodules and other special modes.
      if(!['100644','100755'].includes(mode)||type!=='blob'||!OID.test(oid))
        throw new Error('SNAPSHOT_UNSUPPORTED_TREE_MODE');
      const content=git(['-C',temporary,'cat-file','blob',oid],8*1024*1024);
      if(content.length>6_000_000||cumulative+content.length>250_000_000)
        throw new Error('SNAPSHOT_SIZE_QUOTA_EXCEEDED');
      cumulative+=content.length;
      files.set(path,{oid,content});
      const target=resolve(temporary,path);
      if(!inside(temporary,target))throw new Error('SNAPSHOT_PATH_ESCAPE');
      await mkdir(dirname(target),{recursive:true});
      await writeFile(target,content,{flag:'wx',mode:0o600});
    }
    const actualBase=text(['-C',temporary,'merge-base',scope.mergeBaseOID,scope.headOID]);
    if(actualBase!==scope.mergeBaseOID)throw new Error('SNAPSHOT_MERGE_BASE_CHANGED');
    const relevant=previewFileList(temporary,scope.mergeBaseOID,scope.headOID);
    const manifestFiles:DTO<'File'>[]=relevant.flatMap(path=>{
      const entry=files.get(path);
      if(!entry)return[];
      return [{
        id:sha256(JSON.stringify({taskId,path,oid:entry.oid,side:'HEAD'})),
        path,change:'MODIFIED' as const,blobOID:entry.oid,
        side:'HEAD' as const,lineCount:countLines(entry.content),
      }];
    });
    const manifest:FrozenTaskSnapshot['manifest']={
      schema:'frozen-scope-v1',scope,files:manifestFiles,excluded:[],
      sourceFingerprint:before,
    };
    if(originalRepositoryFingerprint(source)!==before)throw new Error('SOURCE_CHANGED_DURING_SNAPSHOT');
    await rename(temporary,finalPath);
    committed=true;
    inspectFrozenSnapshot({
      sourceRoot:source,snapshotRoot:finalPath,
      expectedHeadOID:scope.headOID,expectedMergeBaseOID:scope.mergeBaseOID,
    });
    return {storageKey:await realpath(finalPath),manifest,manifestSha256:sha256(JSON.stringify(manifest))};
  }finally{
    if(!committed)await rm(temporary,{recursive:true,force:true}).catch(()=>{});
  }
}
