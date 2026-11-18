import {mkdtemp,mkdir,realpath,writeFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {randomUUID} from 'node:crypto';
import type {Pool} from 'mysql2/promise';
import {createApp} from '../../apps/server/src/app.ts';
import {MultiUserAuth} from '../../apps/server/src/multi-user-auth.ts';
import {EmbeddedAuth,type EmbedConfig} from '../../apps/server/src/embed-auth.ts';
import {ExecutionRegistry,type ExecutionRecord} from '../../apps/server/src/execution-registry.ts';
import type {EmbedStore,EmbedGrant,EmbedTicket} from '../../packages/storage/src/embed.ts';
export class MemoryEmbedStore implements EmbedStore {
  tickets=new Map<string,EmbedTicket>();grants=new Map<string,EmbedGrant>();revoked=new Set<string>();clean=new Set<string>();
  now=Date.now;
  async ticket(ticket:EmbedTicket){this.tickets.set(ticket.digest,ticket);}
  async exchange(digest:string,parentOrigin:string,g:Pick<EmbedGrant,'id'|'digest'|'expires'>){
    const t=this.tickets.get(digest);if(!t||t.parentOrigin!==parentOrigin||t.expires<=this.now())return undefined;
    this.tickets.delete(digest);const grant:EmbedGrant={...g,ownerId:t.ownerId,parentOrigin,state:'NEW'};this.grants.set(g.digest,grant);return {...grant};
  }
  async find(digest:string){const g=this.grants.get(digest);return g&&!this.revoked.has(g.id)&&g.expires>this.now()?{...g}:undefined;}
  private byId(id:string){return [...this.grants.values()].find(g=>g.id===id);}
  async reserve(id:string){const g=this.byId(id);if(!g||g.state!=='NEW'||this.revoked.has(id))return false;g.state='CREATING';return true;}
  async bind(id:string,nativeId:string,serviceIdentity:string){const g=this.byId(id);if(!g||g.state!=='CREATING'||this.revoked.has(id))throw Error('bind');g.nativeId=nativeId;g.serviceIdentity=serviceIdentity;g.state='BOUND';}
  async fail(id:string){const g=this.byId(id);if(g?.state==='CREATING')g.state='FAILED';}
  async revoke(id:string){this.revoked.add(id);}
  async cleanupCandidates(){return [...this.grants.values()].filter(g=>(g.expires<=this.now()||this.revoked.has(g.id))&&!this.clean.has(g.id));}
  async cleaned(id:string){this.clean.add(id);}
  async otherSessions(ownerId:string,grantId:string){return [...this.grants.values()].filter(g=>g.ownerId===ownerId&&g.id!==grantId&&g.state==='BOUND'&&!this.revoked.has(g.id)&&g.expires>this.now()).map(g=>g.nativeId!);}
  private readonly locked=new Set<string>();
  async serialize<T>(ownerId:string,fn:()=>Promise<T>){if(this.locked.has(ownerId))throw Error('EMBED_EXECUTOR_BUSY');this.locked.add(ownerId);try{return await fn();}finally{this.locked.delete(ownerId);}}
}
export async function embedFixture(origin='https://workbench.example',parentOrigin='https://host.example',hold=true){
  const base=await realpath(await mkdtemp(join(tmpdir(),'embed-fixture-'))),workspace=join(base,'shared');await mkdir(workspace);
  await mkdir(join(workspace,'.workbench'));
  await writeFile(join(workspace,'.workbench','scenarios.json'),JSON.stringify({version:1,scenes:{message:{directory:workspace,executor:{type:'skill',name:'message-query'}}}}));
  const owners=[randomUUID(),randomUUID(),randomUUID()];
  const records:ExecutionRecord[]=await Promise.all(owners.map(async(ownerId,i)=>{const dir=join(base,'user-'+i);await mkdir(dir);return {ownerId,serviceIdentity:'service-'+i,url:'http://127.0.0.1:'+String(6000+i),authorization:'private-service-'+i,isolation:'os-user',directories:{[workspace]:dir}};}));
  const config:EmbedConfig={publicOrigin:origin,clients:[{id:'host',secret:'host-backend-secret-that-is-not-in-the-browser',parentOrigin,principals:{alice:owners[0],bob:owners[1],carol:owners[2]}}]};
  const store=new MemoryEmbedStore(),embedded=new EmbeddedAuth(config,store);
  const bindings=new Map<string,{owner:string;scenario:string;directory:string}>();
  const pool={execute:async(sql:string,args:unknown[])=>{
    if(sql.startsWith('INSERT INTO wh_chat_session_bindings')){bindings.set(args[0]+':'+args[1],{owner:String(args[2]),scenario:String(args[3]),directory:String(args[4])});return [{affectedRows:1}];}
    if(sql.includes('SELECT scenario,directory FROM wh_chat_session_bindings')){const b=bindings.get(args[0]+':'+args[1]);return [b&&b.owner===args[2]?[b]:[]];}
    if(sql.startsWith('UPDATE wh_chat_session_bindings'))bindings.delete(args[0]+':'+args[1]);
    return [{affectedRows:1}];
  }} as unknown as Pool;
  const nativeSessions=new Map<string,any>(),messages=new Map<string,any[]>(),forms=new Map<string,any[]>(),active:Record<string,boolean>={},calls:string[]=[];
  let serial=0;
  const execution=new ExecutionRegistry(records,async(url)=>{
    const index=records.findIndex(r=>r.url===url),prefix='user-'+index;
    return {
      skill:{list:async({location}:any)=>({location,data:[{id:'message-skill',name:'message-query'}]})},
      model:{list:async()=>({data:[{id:'private',providerID:'company',name:'内网测试模型',enabled:true}]})},
      message:{list:async({sessionID}:any)=>({data:messages.get(sessionID)??[]})},permission:{list:async()=>[]},
      event:{subscribe:async function*({signal}:any){await new Promise<void>(resolve=>signal.addEventListener('abort',()=>resolve(),{once:true}));}},
      session:{
        create:async(input:any)=>{const session={id:prefix+'-'+(++serial),...input,time:{updated:Date.now()}};nativeSessions.set(session.id,session);return session;},
        get:async({sessionID}:any)=>nativeSessions.get(sessionID),list:async()=>({data:[...nativeSessions.values()]}),active:async()=>Object.fromEntries(Object.entries(active).filter(([id])=>id.startsWith(prefix+'-'))),
        inbox:{list:async()=>[]},form:{list:async({sessionID}:any)=>forms.get(sessionID)??[]},instructions:{entry:{put:async()=>{}}},
        switchModel:async({sessionID,model}:any)=>{nativeSessions.get(sessionID).model=model;},
        prompt:async({sessionID,text}:any)=>{
          calls.push(sessionID);active[sessionID]=hold;const now=Date.now();
          messages.set(sessionID,[{id:'input-'+serial,type:'user',text,time:{created:now}},{id:'answer-'+serial,type:'assistant',content:[{type:'text',text:'仅本会话结果：'+sessionID}],finish:'stop',time:{created:now,completed:now+1}}]);return {id:sessionID};
        },
        interrupt:async({sessionID}:any)=>{active[sessionID]=false;return {interrupted:true};},
        remove:async({sessionID}:any)=>{nativeSessions.delete(sessionID);messages.delete(sessionID);},
      },
    } as never;
  });
  const userAuth=new MultiUserAuth({identify:async()=>{throw Error('no login');},issue:async()=>{},find:async()=>undefined,revoke:async()=>{}},'https://workbench.example');
  const app=await createApp({auth:userAuth,pool,execution,embedded,workspace});
  const headers=(token?:string,csrf?:string)=>({host:new URL(origin).host,origin,...(token?{authorization:'Bearer '+token}:{}),...(csrf?{'x-csrf-token':csrf}:{})});
  const ticket=async(subject:string)=>{
    const r=await app.inject({method:'POST',url:'/api/v1/embed/tickets',headers:{host:new URL(origin).host,authorization:'Bearer '+config.clients[0].secret},payload:{clientId:'host',subject}});
    if(r.statusCode!==200)throw Error(r.body);return r.json().data;
  };
  const visit=async(subject:string)=>{
    const t=await ticket(subject);const r=await app.inject({method:'POST',url:'/api/v1/embed/exchange',headers:headers(),payload:{ticket:t.ticket,parentOrigin}});
    if(r.statusCode!==200)throw Error(r.body);return r.json().data as {token:string;csrfToken:string;expiresAt:number};
  };
  return {app,store,config,owners,workspace,records,execution,embedded,nativeSessions,active,calls,messages,forms,headers,ticket,visit,close:async()=>{await app.close();await rm(base,{recursive:true,force:true});}};
}
