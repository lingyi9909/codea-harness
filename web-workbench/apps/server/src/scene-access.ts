import {closeSync,constants,existsSync,fstatSync,fsyncSync,lstatSync,mkdirSync,openSync,readFileSync,renameSync,unlinkSync,writeFileSync} from 'node:fs';
import {createHash,randomUUID} from 'node:crypto';
import {join} from 'node:path';
import {AuthProblem} from './auth.ts';
import type {SceneAccessConfig,SceneAccessPolicy,AccessScene,AccessToolChoice} from '../../../packages/contracts/src/chat.ts';
export type {SceneAccessConfig,SceneAccessPolicy,AccessScene,AccessToolChoice} from '../../../packages/contracts/src/chat.ts';

export const ACCESS_SCENES=['home','review','logs','message','institution-message','data','component-lab'] as const;
type ToolChoice=AccessToolChoice;
type AccessPolicy=SceneAccessPolicy;
type AccessConfig=SceneAccessConfig;
export type NativeRule={action:string;resource:string;effect:'allow'|'deny'|'ask'};
export const ACCESS_FILE='.workbench/scene-access.json';
const sha=(s:string)=>createHash('sha256').update(s).digest('hex');
const invalid=()=>new AuthProblem(400,'SCENE_ACCESS_INVALID');
export function defaultAccess():AccessConfig{
  return {version:1,scenes:Object.fromEntries(ACCESS_SCENES.map(key=>[
    key,{lan:true,shell:'inherit',fileWrite:'inherit',dbhub:'inherit'},
  ])) as AccessConfig['scenes']};
}
export function accessForChat(scenario:string):AccessScene{
  return scenario==='log-inspection'?'logs':scenario==='message-inspection'?'message':
    scenario==='institution-message-inspection'?'institution-message':
    scenario==='data-analysis'?'data':'home';
}
export function applyToolPolicy(base:NativeRule[]|undefined,policy:AccessPolicy):NativeRule[]|undefined{
  const rules:NativeRule[]=[...(base??[])];
  if(policy.shell!=='inherit')rules.push({action:'shell',resource:'*',effect:policy.shell});
  if(policy.fileWrite!=='inherit')
    for(const action of ['edit','write'])rules.push({action,resource:'*',effect:policy.fileWrite});
  if(policy.dbhub==='deny')
    for(const action of ['dbhub_search_objects','dbhub_execute_sql'])
      rules.push({action,resource:'*',effect:'deny'});
  return rules.length?rules:undefined;
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
    if(!info.isFile()||info.isSymbolicLink()||info.size>32768)throw invalid();
    const fd=openSync(this.file,constants.O_RDONLY|(constants.O_NOFOLLOW??0));
    try{if(!fstatSync(fd).isFile())throw invalid();return readFileSync(fd,'utf8');}
    finally{closeSync(fd);}
  }
  private validate(value:unknown):AccessConfig{
    if(!value||typeof value!=='object'||Array.isArray(value))throw invalid();
    const v=value as Record<string,unknown>;
    if(v.version!==1||!v.scenes||typeof v.scenes!=='object'||Array.isArray(v.scenes)||
      Object.keys(v).some(k=>!['version','scenes'].includes(k)))throw invalid();
    const scenes=v.scenes as Record<string,unknown>;
    if(Object.keys(scenes).length!==ACCESS_SCENES.length||Object.keys(scenes).some(k=>!ACCESS_SCENES.includes(k as AccessScene)))throw invalid();
    const config=defaultAccess();
    for(const key of ACCESS_SCENES){
      const p=scenes[key];
      if(!p||typeof p!=='object'||Array.isArray(p))throw invalid();
      const row=p as Record<string,unknown>;
      if(Object.keys(row).length!==4||typeof row.lan!=='boolean'||!['inherit','allow','deny'].includes(String(row.shell))||
        !['inherit','allow','deny'].includes(String(row.fileWrite))||!['inherit','deny'].includes(String(row.dbhub))||
        Object.keys(row).some(k=>!['lan','shell','fileWrite','dbhub'].includes(k)))throw invalid();
      config.scenes[key]={lan:row.lan,shell:row.shell as ToolChoice,fileWrite:row.fileWrite as ToolChoice,dbhub:row.dbhub as 'inherit'|'deny'};
    }
    return config;
  }
  read():{config:AccessConfig;revision:string}{
    const raw=this.raw();
    if(raw===null)return {config:defaultAccess(),revision:sha('')};
    return {config:this.validate(JSON.parse(raw) as unknown),revision:sha(raw)};
  }
  policy(scene:AccessScene){return this.read().config.scenes[scene];}
  save(value:unknown,revision:string){
    const config=this.validate(value);
    if(!/^[a-f0-9]{64}$/.test(revision)||this.read().revision!==revision)throw new AuthProblem(409,'SCENE_ACCESS_STALE');
    this.directory();
    const tmp=join(this.folder,'.access-'+randomUUID()+'.tmp');
    try{
      const fd=openSync(tmp,'wx',0o600);
      try{writeFileSync(fd,JSON.stringify(config,null,2)+'\n','utf8');fsyncSync(fd);}finally{closeSync(fd);}
      if(this.read().revision!==revision)throw new AuthProblem(409,'SCENE_ACCESS_STALE');
      renameSync(tmp,this.file);
      return this.read();
    }finally{if(existsSync(tmp))unlinkSync(tmp);}
  }
}
