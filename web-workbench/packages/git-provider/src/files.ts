import { createHash } from 'node:crypto';
import { execFile as execFileCallback } from 'node:child_process';
import { promisify } from 'node:util';
import { readFile, realpath, stat } from 'node:fs/promises';
import { isAbsolute, join, relative, resolve, sep } from 'node:path';
import type { DTO, Snapshot } from '../../contracts/src/domain.ts';
import { GitFailure, sha256 } from './repository.ts';
import type { Manifest, ManifestFile } from './snapshot.ts';
import { MAX_BROWSE_BYTES } from './snapshot.ts';

const execFile=promisify(execFileCallback);
const within=(root:string,child:string)=>{
  const r=relative(root,child);
  return r===''||(!r.startsWith('..'+sep)&&r!=='..'&&!isAbsolute(r));
};
export async function verifiedManifest(dataRoot:string,snapshot:Snapshot):Promise<{
  root:string;manifest:Manifest;
}>{
  if(!/^[a-f0-9-]{36}$/.test(snapshot.taskId) ||
    !/^[a-f0-9]{64}$/.test(snapshot.manifestHash) ||
    snapshot.storageKey!==['tasks',snapshot.taskId,'repo'].join('/'))
    throw new GitFailure('SNAPSHOT_REFERENCE_INVALID');
  const data=await realpath(resolve(dataRoot));
  const root=await realpath(join(data,...snapshot.storageKey.split('/')));
  if(!within(data,root))throw new GitFailure('SNAPSHOT_STORAGE_ESCAPE');
  const manifestPath=join(data,'tasks',snapshot.taskId,'manifest.json');
  if(!within(data,await realpath(manifestPath)))throw new GitFailure('MANIFEST_STORAGE_ESCAPE');
  const bytes=await readFile(manifestPath);
  if(bytes.length>32*1024*1024)throw new GitFailure('MANIFEST_TOO_LARGE');
  if(sha256(bytes.toString('utf8'))!==snapshot.manifestHash)
    throw new GitFailure('MANIFEST_HASH_MISMATCH');
  let manifest:Manifest;
  try{manifest=JSON.parse(bytes.toString('utf8')) as Manifest;}
  catch{throw new GitFailure('MANIFEST_INVALID_JSON');}
  if(manifest.version!==1||manifest.taskId!==snapshot.taskId||
    manifest.storageKey!==snapshot.storageKey||
    JSON.stringify(manifest.scope)!==JSON.stringify(snapshot.scope)||
    !Array.isArray(manifest.files)||!Array.isArray(manifest.excluded))
    throw new GitFailure('MANIFEST_SCOPE_MISMATCH');
  const dotGit=await realpath(join(root,'.git'));
  if(!within(root,dotGit))throw new GitFailure('SNAPSHOT_GIT_DIR_ESCAPE');
  return{root,manifest};
}
export async function listSnapshotFiles(dataRoot:string,snapshot:Snapshot):Promise<DTO<'FileList'>>{
  const {manifest}=await verifiedManifest(dataRoot,snapshot);
  return {items:manifest.files.filter(e=>e.browseable).map(toFile),excluded:manifest.excluded};
}
function toFile(f:ManifestFile):DTO<'File'>{
  return{id:f.id,path:f.path,side:f.side,blobOID:f.blobOID,change:f.change,
    lineCount:f.lineCount};
}
export async function readSnapshotLines(dataRoot:string,snapshot:Snapshot,fileId:string,
  startLine=1,endLine=200):Promise<DTO<'FileLines'>>{
  if(!/^[a-f0-9]{64}$/.test(fileId))throw new GitFailure('INVALID_FILE_ID');
  if(!Number.isSafeInteger(startLine)||!Number.isSafeInteger(endLine)||
    startLine<1||endLine<startLine||endLine-startLine+1>500)
    throw new GitFailure('INVALID_LINE_RANGE');
  const {root,manifest}=await verifiedManifest(dataRoot,snapshot);
  const file=manifest.files.find(f=>f.id===fileId&&f.browseable);
  if(!file)throw new GitFailure('FILE_NOT_BROWSABLE');
  if(file.sizeBytes>MAX_BROWSE_BYTES)throw new GitFailure('FILE_TOO_LARGE');
  const {stdout}=await execFile('git',['-C',root,'cat-file','blob',file.blobOID],{
    encoding:'buffer',shell:false,windowsHide:true,timeout:15000,
    maxBuffer:MAX_BROWSE_BYTES+4096,
    env:{...process.env,GIT_TERMINAL_PROMPT:'0',GIT_NO_LAZY_FETCH:'1',
      GIT_CONFIG_GLOBAL:process.platform==='win32'?'NUL':'/dev/null'},
  });
  const contents=stdout as Buffer;
  if(contents.length!==file.sizeBytes||createHash('sha256').update(contents).digest('hex')!==file.sha256)
    throw new GitFailure('SNAPSHOT_BLOB_HASH_MISMATCH');
  if(contents.includes(0))
    throw new GitFailure('BINARY_FILE_NOT_BROWSABLE');
  const content=contents.toString('utf8');
  if(!Buffer.from(content,'utf8').equals(contents))throw new GitFailure('INVALID_UTF8_FILE');
  const all=content.length===0?[]:content.split(/\r?\n/);
  if(all.length&&all[all.length-1]===''&&content.endsWith('\n'))all.pop();
  if(all.length!==file.lineCount)throw new GitFailure('LINE_COUNT_MISMATCH');
  const actualEnd=Math.min(endLine,all.length);
  return {file:toFile(file),startLine,endLine:actualEnd,
    lines:all.slice(startLine-1,actualEnd),hasMore:actualEnd<all.length};
}
