/**
 * 场景执行配置
 *
 * 职责：维护每个业务子场景的工作目录及 Skill、Agent 或命令绑定。
 * Java 研发阅读提示：类比 Java 配置中心；目录与执行能力必须在实际调用前核验。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
import { closeSync, constants, existsSync, fstatSync, fsyncSync, lstatSync, mkdirSync, openSync, readFileSync, realpathSync, renameSync, statSync, unlinkSync, writeFileSync } from 'node:fs';
import { createHash, randomUUID } from 'node:crypto';
import { isAbsolute, join, relative, resolve } from 'node:path';
import { AuthProblem } from './auth.ts';

/** Configuration lives in the *startup* workspace; per-scene directories are separate. */
export const SCENE_KEYS=['logs','message','institution-message','data'] as const;
export type SceneKey=typeof SCENE_KEYS[number];
export type Executor={type:'skill'|'subagent'|'command'|'builtin';name:string};
export type Binding={directory:string;executor:Executor};
export type SceneConfiguration={version:1;scenes:Partial<Record<SceneKey,Binding>>};
export const SCENE_CONFIG_PATH='.workbench/scenarios.json';
export const SCENE_ACCESS_PATH='.workbench/config-access.json';
const EMPTY:SceneConfiguration={version:1,scenes:{}};
// Defaults do not install any external Skill. The public brainstorming skill
// must already be loaded by OpenCode. /models opens the model selector, not
// the custom-command execution endpoint.
export function defaultSceneConfiguration(root:string):SceneConfiguration{
  return {version:1,scenes:{
    logs:{directory:root,executor:{type:'skill',name:'brainstorming'}},
    message:{directory:root,executor:{type:'command',name:'models'}},
    'institution-message':{directory:root,executor:{type:'builtin',name:'chat'}},
    data:{directory:root,executor:{type:'builtin',name:'chat'}},
  }};
}
// Current workbench owner. This is a UI-level allowance; local profile names are not verified.
const DEFAULT_EDITORS=['zhangzhanhui'];
const sha=(value:string)=>createHash('sha256').update(value).digest('hex');
const plain=(value:unknown):value is Record<string,unknown>=>Boolean(value)&&typeof value==='object'&&!Array.isArray(value);
const invalid=(code='SCENE_CONFIG_INVALID')=>new AuthProblem(400,code);

export class SceneConfigStore {
  readonly root:string;
  readonly file:string;
  readonly accessFile:string;
  private readonly folder:string;
  constructor(root:string){
    this.root=realpathSync.native(root);
    this.folder=join(this.root,'.workbench');
    this.file=join(this.folder,'scenarios.json');
    this.accessFile=join(this.folder,'config-access.json');
  }
  /** Initialize safe config files on first startup with the explicitly selected test editor.
   * The login profile is intentionally created only after a successful login.
   * Existing files are never overwritten. */
  async initialize(){
    this.directory(true);
    const defaults=[
      {path:this.file,text:JSON.stringify(defaultSceneConfiguration(this.root),null,2)+'\n'},
      {path:this.accessFile,text:JSON.stringify({version:1,editors:DEFAULT_EDITORS},null,2)+'\n'},
    ];
    for(const file of defaults){
      // Refuse existing symlinks or non-regular files. File creation is exclusive.
      if(this.readFile(file.path)!==null)continue;
      let fd:number|undefined;
      try{
        fd=openSync(file.path,'wx',0o600);
        writeFileSync(fd,file.text,'utf8');
        fsyncSync(fd);
      }catch(e){
        if((e as NodeJS.ErrnoException).code!=='EEXIST')throw e;
      }finally{if(fd!==undefined)closeSync(fd);}
      this.readFile(file.path);
    }
    const current=this.read(); // Invalid preexisting configuration blocks startup.
    const defaultConfig=defaultSceneConfiguration(this.root);
    const missing=SCENE_KEYS.filter(key=>!current.config.scenes[key]);
    if(missing.length){
      const scenes={...current.config.scenes};
      for(const key of missing)scenes[key]=defaultConfig.scenes[key];
      await this.save({version:1,scenes},current.revision);
    }
  }
  /** Never follow symlinks to config or the editor ACL. */
  private directory(create=false){
    if(relative(this.root,realpathSync.native(this.root))!=='')throw invalid();
    let info=lstatSync(this.folder,{throwIfNoEntry:false});
    if(!info&&create){mkdirSync(this.folder,{mode:0o700});info=lstatSync(this.folder);}
    if(!info)return false;
    if(!info.isDirectory()||info.isSymbolicLink()||realpathSync.native(this.folder)!==this.folder)throw invalid();
    return true;
  }
  private readFile(path:string){
    if(!this.directory())return null;
    const check=lstatSync(path,{throwIfNoEntry:false});
    if(!check)return null;
    if(!check.isFile()||check.isSymbolicLink()||check.size>16384||realpathSync.native(path)!==path)throw invalid();
    const fd=openSync(path,constants.O_RDONLY|(constants.O_NOFOLLOW??0));
    try{
      const info=fstatSync(fd);
      if(!info.isFile()||info.size>16384)throw invalid();
      return readFileSync(fd,'utf8');
    }finally{closeSync(fd);}
  }
  private validate(raw:unknown):SceneConfiguration{
    if(!plain(raw)||raw.version!==1||!plain(raw.scenes)||Object.keys(raw).some(key=>!['version','scenes'].includes(key)))throw invalid();
    const scenes:SceneConfiguration['scenes']={};
    for(const [key,value] of Object.entries(raw.scenes)){
      if(!SCENE_KEYS.includes(key as SceneKey)||!plain(value)||Object.keys(value).some(k=>!['directory','executor'].includes(k)))throw invalid();
      if(typeof value.directory!=='string'||!isAbsolute(value.directory)||value.directory.length>4096||!plain(value.executor))throw invalid();
      const {type,name}=value.executor;
      if(!['skill','subagent','command','builtin'].includes(String(type))||typeof name!=='string'||
        !/^[a-zA-Z][a-zA-Z0-9._-]{0,127}$/.test(name)||Object.keys(value.executor).some(k=>!['type','name'].includes(k))||
        (type==='builtin'&&name!=='chat'))throw invalid();
      scenes[key as SceneKey]={directory:value.directory,executor:{type:type as Executor['type'],name}};
    }
    return {version:1,scenes};
  }
  read():{config:SceneConfiguration;revision:string}{
    const raw=this.readFile(this.file);
    if(raw===null)return {config:{...EMPTY,scenes:{}},revision:sha('')};
    try{return {config:this.validate(JSON.parse(raw)),revision:sha(raw)};}
    catch(e){if(e instanceof AuthProblem)throw e;throw invalid();}
  }
  /** Fail closed. This file is managed outside the web editor to avoid self-grant. */
  canEdit(username?:string):boolean{
    if(!username)return false;
    let raw:string|null;
    try{raw=this.readFile(this.accessFile);}catch{return false;}
    if(!raw)return false;
    try{
      const acl=JSON.parse(raw) as unknown;
      return plain(acl)&&acl.version===1&&Array.isArray(acl.editors)&&
        acl.editors.length<=100&&acl.editors.every((item:unknown)=>
          typeof item==='string'&&item===item.trim()&&item.length>0&&
          [...item].length<=80&&!/\p{Cc}/u.test(item)&&item!=='*')&&
        acl.editors.includes(username);
    }catch{return false;}
  }
  async binding(key:SceneKey):Promise<Binding>{
    const scenes=this.read().config.scenes;
    let row=scenes[key];
    if(!row)throw new AuthProblem(409,'SCENE_NOT_CONFIGURED');
    // The embedded institution UI is another entry into the existing message
    // investigation workflow. If no dedicated institution Skill was selected,
    // reuse the already configured message Skill in the same workspace.
    // Keep a dedicated institution binding authoritative when configured.
    if(key==='institution-message'&&row.executor.type==='builtin'&&row.executor.name==='chat'){
      const message=scenes.message;
      if(message?.executor.type==='skill')row={...message,executor:{...message.executor}};
    }
    let path:string;
    try{path=realpathSync.native(row.directory);if(!statSync(path).isDirectory())throw new Error('NOT_DIRECTORY');}
    catch{throw new AuthProblem(409,'SCENE_DIRECTORY_INVALID');}
    return {...row,directory:path};
  }
  async save(value:unknown,revision:string):Promise<{config:SceneConfiguration;revision:string}>{
    const config=this.validate(value);
    if(!/^[a-f0-9]{64}$/.test(revision))throw invalid('SCENE_REVISION_INVALID');
    const scenes:SceneConfiguration['scenes']={};
    for(const [key,row] of Object.entries(config.scenes)){
      if(!row)continue;
      let path:string;
      try{path=realpathSync.native(row.directory);if(!statSync(path).isDirectory())throw new Error('NOT_DIRECTORY');}
      catch{throw invalid('SCENE_DIRECTORY_INVALID');}
      scenes[key as SceneKey]={...row,directory:path};
    }
    const fresh:SceneConfiguration={version:1,scenes};
    if(this.read().revision!==revision)throw new AuthProblem(409,'SCENE_CONFIG_STALE');
    this.directory(true);
    if(this.read().revision!==revision)throw new AuthProblem(409,'SCENE_CONFIG_STALE');
    let temporary:string|undefined;
    try{
      temporary=join(this.folder,'.scenes-'+randomUUID()+'.tmp');
      const fd=openSync(temporary,'wx',0o600);
      try{writeFileSync(fd,JSON.stringify(fresh,null,2)+'\n','utf8');fsyncSync(fd);}finally{closeSync(fd);}
      // Re-check parent and target before replacing; never follow a symlink.
      if(this.read().revision!==revision)throw new AuthProblem(409,'SCENE_CONFIG_STALE');
      this.directory();
      renameSync(temporary,this.file);
      temporary=undefined;
      return this.read();
    }finally{if(temporary&&existsSync(temporary))unlinkSync(temporary);}
  }
}
