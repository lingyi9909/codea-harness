import { execFile as execFileCallback } from 'node:child_process';
import { createHash, randomUUID } from 'node:crypto';
import { promisify } from 'node:util';
import { mkdir, readdir, lstat, copyFile, realpath, writeFile, rename, rm, open } from 'node:fs/promises';
import { dirname, isAbsolute, join, relative, resolve, sep } from 'node:path';
import { git, GitFailure, sha256 } from './repository.ts';
import type { DTO, Snapshot } from '../../contracts/src/domain.ts';

const execFile = promisify(execFileCallback);
const OID = /^(?:[a-f0-9]{40}|[a-f0-9]{64})$/;
export const MAX_TREE_FILES=50000;
export const MAX_CHANGED_FILES=5000;
export const MAX_GIT_BYTES=5*1024*1024*1024;
export const MAX_MATERIALIZED_BYTES=1024*1024*1024;
export const MAX_BROWSE_BYTES=1024*1024;
export type Side='BASE'|'HEAD';
export type Change='ADDED'|'MODIFIED'|'DELETED'|'RENAMED'|'CONTEXT';
export type Entry={path:string;mode:string;oid:string;size:number};
export type ManifestFile={
  id:string;path:string;side:Side;change:Change;blobOID:string;
  sha256:string;lineCount:number;browseable:boolean;sizeBytes:number;
};
export type Manifest={version:1;taskId:string;scope:DTO<'Scope'>;files:ManifestFile[];
  excluded:{path:string;reason:string}[];storageKey:string;createdAt:string};
export type BuiltSnapshot={snapshot:Snapshot;manifest:Manifest};
const safeWithin=(root:string,p:string)=>{
  const r=relative(root,p);
  return r===''||(!r.startsWith('..'+sep)&&r!=='..'&&!isAbsolute(r));
};
const safePath=(p:string)=>{
  if(!p||p.length>4096||isAbsolute(p)||p.startsWith('/')||p.includes('\\')||
    /[\x00-\x1f\x7f<>:"|?*]/.test(p)||p.split('/').some(x=>
      !x||x==='.'||x==='..'||x.endsWith('.')||x.endsWith(' ')||
      /^(con|prn|aux|nul|com[1-9]|lpt[1-9])(\..*)?$/i.test(x)))
    throw new GitFailure('UNSAFE_TRACKED_PATH');
  return p;
};
const protectedFile=(path:string)=>/(^|\/)(\.git|\.opencode|\.teamai)(\/|$)/i.test(path)||
  /(^|\/)(\.env(?:\..*)?|\.npmrc|\.pypirc|id_(?:rsa|ed25519)|[^/]+\.(?:pem|key|p12|pfx))$/i.test(path);
const textFile=(p:string)=>/\.(?:java|xml|sql|yml|yaml|properties|json|md|txt|gradle|kt|kts)$/i.test(p)||
  /(^|\/)pom\.xml$/i.test(p);
const requireOid=(v:string)=>{if(!OID.test(v))throw new GitFailure('INVALID_PINNED_OID');};
async function rawGit(cwd:string,args:string[],maxBuffer=MAX_BROWSE_BYTES+4096):Promise<Buffer>{
  const {stdout}=await execFile('git',['-C',cwd,...args],{
    encoding:'buffer',timeout:30000,windowsHide:true,shell:false,maxBuffer,
    env:{...process.env,GIT_TERMINAL_PROMPT:'0',GIT_NO_LAZY_FETCH:'1',
      GIT_OPTIONAL_LOCKS:'0',GIT_CONFIG_GLOBAL:process.platform==='win32'?'NUL':'/dev/null'},
  });
  return stdout as Buffer;
}
export async function getTree(cwd:string,oid:string):Promise<Map<string,Entry>>{
  requireOid(oid);
  const raw=await rawGit(cwd,['ls-tree','-rz','-l','--full-tree',oid],64*1024*1024);
  const files=new Map<string,Entry>(),names=new Set<string>();
  for(const rec of raw.subarray(0,raw.length-1).toString('binary').split('\0')){
    if(!rec)continue;
    const buf=Buffer.from(rec,'binary');
    const tab=buf.indexOf(9);
    if(tab<0)throw new GitFailure('INVALID_GIT_TREE');
    const prefix=buf.subarray(0,tab).toString('ascii');
    const parts=prefix.match(/^(100644|100755|120000|160000) (blob|commit) ([a-f0-9]{40}|[a-f0-9]{64}) +([0-9]+|-)$/);
    if(!parts||parts[2]!=='blob'||(parts[1]!=='100644'&&parts[1]!=='100755'))
      throw new GitFailure('UNSUPPORTED_TRACKED_ENTRY');
    const rawName=buf.subarray(tab+1);
    const path=rawName.toString('utf8');
    if(!Buffer.from(path,'utf8').equals(rawName))throw new GitFailure('INVALID_UTF8_GIT_PATH');
    safePath(path);
    const lower=path.toLowerCase();
    if(names.has(lower))throw new GitFailure('CASE_COLLIDING_TRACKED_FILES');
    names.add(lower);
    const size=Number(parts[4]);
    if(!Number.isSafeInteger(size)||size<0)throw new GitFailure('INVALID_BLOB_SIZE');
    files.set(path,{path,mode:parts[1],oid:parts[3],size});
    if(files.size>MAX_TREE_FILES)throw new GitFailure('BUDGET_EXCEEDED_TREE');
  }
  return files;
}
async function copyObjects(sourceRoot:string,dest:string):Promise<void>{
  const srcGit=join(sourceRoot,'.git');
  if(!(await lstat(srcGit)).isDirectory())throw new GitFailure('EXTERNAL_GIT_DIR_NOT_SUPPORTED');
  const from=join(srcGit,'objects'),to=join(dest,'.git','objects');
  const canonical=await realpath(from);
  let total=0;
  async function walk(source:string,target:string):Promise<void>{
    if(!safeWithin(canonical,await realpath(source)))throw new GitFailure('OBJECT_STORE_ESCAPE');
    await mkdir(target,{recursive:true});
    for(const entry of await readdir(source,{withFileTypes:true})){
      if(entry.name==='alternates')throw new GitFailure('ALTERNATE_OBJECT_STORE_NOT_ALLOWED');
      const src=join(source,entry.name),dst=join(target,entry.name),st=await lstat(src);
      if(st.isSymbolicLink())throw new GitFailure('SYMLINKED_OBJECT_STORE');
      if(st.isDirectory()){await walk(src,dst);continue;}
      if(!st.isFile())throw new GitFailure('UNSUPPORTED_OBJECT_STORE_ENTRY');
      total+=st.size;
      if(total>MAX_GIT_BYTES)throw new GitFailure('BUDGET_EXCEEDED_OBJECTS');
      await copyFile(src,dst); // intentionally never hardlink original objects
    }
  }
  await walk(from,to);
}
const linesIn=(bytes:Buffer)=>bytes.length===0?0:bytes.toString('utf8').split(/\r?\n/).length-
  (bytes.toString('utf8').endsWith('\n')?1:0);
const fileId=(taskId:string,side:Side,path:string,oid:string)=>
  createHash('sha256').update(taskId+'\0'+side+'\0').update(Buffer.from(path,'utf8')).update('\0'+oid).digest('hex');

export async function prepareSnapshot(input:{
  taskId:string;sourceRoot:string;dataRoot:string;scope:DTO<'Scope'>;
}):Promise<BuiltSnapshot>{
  const {taskId,scope}=input;
  if(!/^[a-f0-9-]{36}$/.test(taskId))throw new GitFailure('TASK_ID_INVALID');
  for(const oid of [scope.headOID,scope.baseOID,scope.mergeBaseOID])requireOid(oid);
  const source=await realpath(resolve(input.sourceRoot));
  const data=resolve(input.dataRoot);
  await mkdir(data,{recursive:true});
  const dataReal=await realpath(data);
  if(safeWithin(source,dataReal)||safeWithin(dataReal,source))
    throw new GitFailure('SNAPSHOT_SOURCE_OVERLAP');
  const actualHead=await git(source,['rev-parse','--verify','HEAD']);
  const actualBase=await git(source,['rev-parse','--verify',scope.baseRef+'^{commit}']);
  const actualMerge=await git(source,['merge-base',scope.baseOID,scope.headOID]);
  if(actualHead!==scope.headOID||actualBase!==scope.baseOID||
    actualMerge!==scope.mergeBaseOID)throw new GitFailure('SCOPE_CHANGED');
  const [headTree,baseTree]=await Promise.all([
    getTree(source,scope.headOID),getTree(source,scope.mergeBaseOID),
  ]);
  const changed:string[]=[];
  for(const path of new Set([...headTree.keys(),...baseTree.keys()]))
    if(headTree.get(path)?.oid!==baseTree.get(path)?.oid||headTree.get(path)?.mode!==baseTree.get(path)?.mode)changed.push(path);
  if(changed.length>MAX_CHANGED_FILES)throw new GitFailure('BUDGET_EXCEEDED_CHANGED');
  if(changed.length===0)throw new GitFailure('NO_CHANGES');
  const taskDir=join(dataReal,'tasks',taskId);
  const partial=join(taskDir,'repo.partial'),final=join(taskDir,'repo');
  await mkdir(taskDir,{recursive:true});
  // No reuse of unknown partial state. Another worker must explicitly reconcile first.
  try{await lstat(partial);throw new GitFailure('SNAPSHOT_PARTIAL_EXISTS');}
  catch(e){if(!(e instanceof Error&&'code' in e&&e.code==='ENOENT'))throw e;}
  try{await lstat(final);throw new GitFailure('SNAPSHOT_ALREADY_EXISTS');}
  catch(e){if(!(e instanceof Error&&'code' in e&&e.code==='ENOENT'))throw e;}
  await mkdir(partial);
  let committed=false;
  try{
    await rawGit(partial,['init','-q','--object-format='+scope.objectFormat],1024);
    await copyObjects(source,partial);
    for(const [key,value] of Object.entries({
      'core.autocrlf':'false','core.safecrlf':'false','core.fsmonitor':'false',
      'core.sparseCheckout':'false','core.hooksPath':'.git/disabled-hooks',
      'core.filemode':'false',
    })) await rawGit(partial,['config','--local',key,value],1024);
    await rawGit(partial,['update-ref','refs/heads/harness-review-base',scope.mergeBaseOID],1024);
    await rawGit(partial,['update-ref','refs/heads/harness-review-head',scope.headOID],1024);
    await rawGit(partial,['symbolic-ref','HEAD','refs/heads/harness-review-head'],1024);
    await rawGit(partial,['read-tree',scope.headOID],1024);
    let materialized=0;
    for(const e of headTree.values()){
      materialized+=e.size;
      if(materialized>MAX_MATERIALIZED_BYTES)throw new GitFailure('BUDGET_EXCEEDED_MATERIALIZED');
      const target=join(partial,...e.path.split('/'));
      if(!safeWithin(partial,target))throw new GitFailure('SNAPSHOT_PATH_ESCAPE');
      await mkdir(dirname(target),{recursive:true});
      const bytes=await rawGit(partial,['cat-file','blob',e.oid],e.size+4096);
      if(bytes.length!==e.size)throw new GitFailure('BLOB_SIZE_MISMATCH');
      await writeFile(target,bytes,{flag:'wx',mode:e.mode==='100755'?0o755:0o644});
    }
    const status=await git(partial,['status','--porcelain=v1','--untracked-files=all']);
    if(status)throw new GitFailure('SNAPSHOT_NOT_CLEAN');
    const files:ManifestFile[]=[],excluded:{path:string;reason:string}[]=[];
    for(const path of changed.sort()){
      for(const side of ['BASE','HEAD'] as Side[]){
        const entry=(side==='HEAD'?headTree:baseTree).get(path);
        if(!entry)continue;
        let browseable=!protectedFile(path)&&textFile(path)&&entry.size<=MAX_BROWSE_BYTES;
        // Each BASE/HEAD view is explicitly recorded as non-browsable when filtered.
        let hash='',lineCount=0;
        if(browseable){
          const bytes=await rawGit(partial,['cat-file','blob',entry.oid],MAX_BROWSE_BYTES+4096);
          if(bytes.includes(0)||!Buffer.from(bytes.toString('utf8'),'utf8').equals(bytes))
            browseable=false;
          else{
            hash=createHash('sha256').update(bytes).digest('hex');
            lineCount=linesIn(bytes);
          }
        }
        if(!browseable)excluded.push({path,reason:protectedFile(path)?'PROTECTED_FILE':
          entry.size>MAX_BROWSE_BYTES?'FILE_TOO_LARGE':'NOT_BROWSABLE_TEXT'});
        files.push({id:fileId(taskId,side,path,entry.oid),path,side,
          change:!baseTree.has(path)?'ADDED':!headTree.has(path)?'DELETED':'MODIFIED',
          blobOID:entry.oid,sha256:hash,lineCount,browseable,sizeBytes:entry.size});
      }
    }
    // Manifest outside the Git working tree: the checked-out snapshot remains clean.
    const storageKey=['tasks',taskId,'repo'].join('/');
    const manifest:Manifest={version:1,taskId,scope,files,excluded,
      storageKey,createdAt:new Date().toISOString()};
    const encoded=JSON.stringify(manifest);
    const manifestHash=sha256(encoded);
    const handle=await open(join(taskDir,'manifest.partial'),'wx');
    try{await handle.writeFile(encoded,'utf8');await handle.sync();}finally{await handle.close();}
    await rename(partial,final);
    await rename(join(taskDir,'manifest.partial'),join(taskDir,'manifest.json'));
    committed=true;
    return{snapshot:{taskId,scope,storageKey,manifestHash},manifest};
  }finally{
    if(!committed)await rm(partial,{recursive:true,force:true}).catch(()=>{});
  }
}
