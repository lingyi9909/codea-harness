/**
 * 场景权限策略
 *
 * 职责：保存/核验不同研发场景的工具使用权限。
 * Java 研发阅读提示：权限校验在 C 后端完成；配置编辑权限和工具执行权限需分开理解。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
import {closeSync,constants,existsSync,fstatSync,fsyncSync,lstatSync,mkdirSync,openSync,readFileSync,renameSync,unlinkSync,writeFileSync} from 'node:fs';
import {createHash,randomUUID} from 'node:crypto';
import {join} from 'node:path';
import {AuthProblem} from './auth.ts';
import {
  accessToolActions,
  type SceneAccessConfig,type SceneAccessPolicy,type AccessScene,
  type AccessToolAction,type AccessToolChoice,type CustomToolRule,
} from '../../../packages/contracts/src/chat.ts';
export type {SceneAccessConfig,SceneAccessPolicy,AccessScene,AccessToolAction,AccessToolChoice,CustomToolRule} from '../../../packages/contracts/src/chat.ts';

export const ACCESS_SCENES=['home','review','logs','message','institution-message','data','component-lab','analytics'] as const;
/** These pages contain administrative or testing information and are never exposed on LAN. */
export const LOOPBACK_ONLY_SCENES=['component-lab','analytics'] as const;
export function isLoopbackOnlyScene(scene:AccessScene):boolean {
  return (LOOPBACK_ONLY_SCENES as readonly string[]).includes(scene);
}
export type NativeRule={action:string;resource:string;effect:'allow'|'deny'|'ask'};
export const ACCESS_FILE='.workbench/scene-access.json';
const sha=(s:string)=>createHash('sha256').update(s).digest('hex');
const invalid=()=>new AuthProblem(400,'SCENE_ACCESS_INVALID');
const choices=['inherit','allow','ask','deny'] as const;
const actions=new Set<string>(accessToolActions);
const scenes=new Set<string>(ACCESS_SCENES);
const object=(v:unknown):v is Record<string,unknown>=>Boolean(v)&&typeof v==='object'&&!Array.isArray(v);

export function defaultAccess():SceneAccessConfig{
  return {version:2,scenes:Object.fromEntries(ACCESS_SCENES.map(key=>[
    key,{lan:!isLoopbackOnlyScene(key),tools:Object.fromEntries(accessToolActions.map(action=>[action,'inherit'])) as SceneAccessPolicy['tools'],custom:[]},
  ])) as unknown as SceneAccessConfig['scenes']};
}
export function accessForChat(scenario:string):AccessScene{
  return scenario==='log-inspection'?'logs':scenario==='message-inspection'?'message':
    scenario==='institution-message-inspection'?'institution-message':
    scenario==='data-analysis'?'data':'home';
}
export function applyToolPolicy(base:NativeRule[]|undefined,policy:SceneAccessPolicy):NativeRule[]|undefined{
  const rules:NativeRule[]=[...(base??[])];
  for(const action of accessToolActions){
    const effect=policy.tools[action];
    if(effect!=='inherit')rules.push({action,resource:'*',effect});
  }
  for(const rule of policy.custom)rules.push({...rule});
  return rules.length?rules:undefined;
}
/** Retain the original v1 config and its revision; migrate in memory until first save. */
function migrateV1(value:Record<string,unknown>):SceneAccessConfig{
  if(value.version!==1||!object(value.scenes)||Object.keys(value).some(k=>!['version','scenes'].includes(k)))throw invalid();
  const old=value.scenes;
  const expected=new Set<string>(ACCESS_SCENES.filter(k=>k!=='analytics'));
  if(Object.keys(old).length!==expected.size||Object.keys(old).some(k=>!expected.has(k)))throw invalid();
  const config=defaultAccess();
  for(const id of expected){
    const row=old[id];
    if(!object(row)||Object.keys(row).length!==4||Object.keys(row).some(k=>!['lan','shell','fileWrite','dbhub'].includes(k))||
      typeof row.lan!=='boolean'||!['inherit','allow','deny'].includes(String(row.shell))||
      !['inherit','allow','deny'].includes(String(row.fileWrite))||
      !['inherit','deny'].includes(String(row.dbhub)))throw invalid();
    const value=config.scenes[id as AccessScene];
    value.lan=isLoopbackOnlyScene(id as AccessScene)?false:row.lan;
    value.tools.shell=row.shell as AccessToolChoice;
    value.tools.write=row.fileWrite as AccessToolChoice;
    value.tools.edit=row.fileWrite as AccessToolChoice;
    if(row.dbhub==='deny'){
      value.tools.dbhub_search_objects='deny';
      value.tools.dbhub_execute_sql='deny';
    }
  }
  return config;
}
function validRule(raw:unknown):CustomToolRule{
  if(!object(raw)||Object.keys(raw).length!==3||Object.keys(raw).some(k=>!['action','resource','effect'].includes(k)))throw invalid();
  if(typeof raw.action!=='string'||!/^[a-z][a-z0-9_.:-]{0,79}$/.test(raw.action)||
    typeof raw.resource!=='string'||raw.resource.length<1||raw.resource.length>512||
    /[\x00-\x1f\x7f]/.test(raw.resource)||!['allow','ask','deny'].includes(String(raw.effect)))throw invalid();
  return {action:raw.action,resource:raw.resource,effect:raw.effect as CustomToolRule['effect']};
}
export function validateAccess(value:unknown):SceneAccessConfig{
  if(!object(value))throw invalid();
  if(value.version===1)return migrateV1(value);
  if(value.version!==2||!object(value.scenes)||Object.keys(value).length!==2||Object.keys(value).some(k=>!['version','scenes'].includes(k)))throw invalid();
  const entries=value.scenes;
  if(Object.keys(entries).length!==ACCESS_SCENES.length||Object.keys(entries).some(k=>!scenes.has(k)))throw invalid();
  const config=defaultAccess();
  for(const key of ACCESS_SCENES){
    const row=entries[key];
    if(!object(row)||Object.keys(row).length!==3||Object.keys(row).some(k=>!['lan','tools','custom'].includes(k))||
       typeof row.lan!=='boolean'||!object(row.tools)||!Array.isArray(row.custom))throw invalid();
    if(Object.keys(row.tools).length!==accessToolActions.length||Object.keys(row.tools).some(k=>!actions.has(k))||
      Object.values(row.tools).some(v=>!choices.includes(v as AccessToolChoice)))throw invalid();
    if(row.custom.length>16)throw invalid();
    const rules=row.custom.map(validRule);
    const unique=new Set(rules.map(r=>JSON.stringify([r.action,r.resource])));
    if(unique.size!==rules.length)throw invalid();
    // Upgrade older saved policies permissively, but never allow LAN for utility/admin pages.
    config.scenes[key]={lan:isLoopbackOnlyScene(key)?false:row.lan,tools:{...row.tools} as SceneAccessPolicy['tools'],custom:rules};
  }
  return config;
}
export class SceneAccessStore{
  readonly file:string;
  private readonly folder:string;
  constructor(workspace:string){
    this.folder=join(workspace,'.workbench');
    this.file=join(this.folder,'scene-access.json');
  }
  private directory(){
    const info=lstatSync(this.folder,{throwIfNoEntry:false});
    if(!info)mkdirSync(this.folder,{mode:0o700,recursive:true});
    const checked=lstatSync(this.folder);
    if(!checked.isDirectory()||checked.isSymbolicLink())throw invalid();
  }
  private raw(){
    this.directory();
    const info=lstatSync(this.file,{throwIfNoEntry:false});
    if(!info)return null;
    if(!info.isFile()||info.isSymbolicLink()||info.size>65536)throw invalid();
    const fd=openSync(this.file,constants.O_RDONLY|(constants.O_NOFOLLOW??0));
    try{if(!fstatSync(fd).isFile())throw invalid();return readFileSync(fd,'utf8');}
    finally{closeSync(fd);}
  }
  read():{config:SceneAccessConfig;revision:string}{
    const raw=this.raw();
    if(raw===null)return {config:defaultAccess(),revision:sha('')};
    return {config:validateAccess(JSON.parse(raw) as unknown),revision:sha(raw)};
  }
  policy(scene:AccessScene){return this.read().config.scenes[scene];}
  save(value:unknown,revision:string){
    const config=validateAccess(value);
    if(config.version!==2||!object(value)||(value as Record<string,unknown>).version!==2)throw invalid();
    if(!/^[a-f0-9]{64}$/.test(revision)||this.read().revision!==revision)throw new AuthProblem(409,'SCENE_ACCESS_STALE');
    const serialized=JSON.stringify(config,null,2)+'\n';
    if(serialized.length>65536)throw invalid();
    this.directory();
    const tmp=join(this.folder,'.access-'+randomUUID()+'.tmp');
    try{
      const fd=openSync(tmp,'wx',0o600);
      try{writeFileSync(fd,serialized,'utf8');fsyncSync(fd);}finally{closeSync(fd);}
      if(this.read().revision!==revision)throw new AuthProblem(409,'SCENE_ACCESS_STALE');
      renameSync(tmp,this.file);
      return this.read();
    }finally{if(existsSync(tmp))unlinkSync(tmp);}
  }
}
