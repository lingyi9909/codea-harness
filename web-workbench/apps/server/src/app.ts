import { parseProfile, profileKey, WorkspaceProfile } from './local-profile.ts';
import { registerChat, resolveLocalWorkspace } from './modules/chat/routes.ts';
import Fastify from 'fastify';
import serveStatic from '@fastify/static';
import { existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';
import { dispatchNext,type SchedulerConfig } from './workers/scheduler.ts';
import { randomUUID } from 'node:crypto';
import { delimiter } from 'node:path';
import { registerGitRoutes } from './modules/repositories/routes.ts';
import { registerTaskFileRoutes } from './modules/tasks/files.ts';
import { registerTaskRoutes } from './modules/tasks/routes.ts';
import { registerEventRoutes } from './modules/events/routes.ts';
import { registerRunStopRoute } from './modules/runs/routes.ts';
import { registerInteractionRoutes } from './modules/interactions/routes.ts';
import { registerMessageRoutes } from './modules/messages/routes.ts';
import { registerArtifactRoutes } from './modules/artifacts/routes.ts';
import type { Pool } from 'mysql2/promise';
import { LocalAuth, AuthProblem, ensureLocalOwner } from './auth.ts';
import { evaluateCapabilities } from '../../../packages/opencode-adapter/src/probe.ts';
import type { DTO } from '../../../packages/contracts/src/domain.ts';

const result=<T>(data:T)=>({data,requestId:randomUUID()});
export async function createApp(opts:{localOnly?:boolean;workspace?:string;autoOpenCode?:boolean;pool?:Pool;auth:LocalAuth;native?:Omit<SchedulerConfig,'pool'|'signal'>}){
  const app=Fastify({logger:false,bodyLimit:32768,trustProxy:false});
  const capabilities=opts.native?await opts.native.adapter.probe():evaluateCapabilities().capabilities;
  const enabled=Boolean(opts.native&&opts.native.nativeVersion.trim()&&opts.native.nativeVersion!=='UNVERIFIED'&&
    evaluateCapabilities().capabilities.filter(c=>c.required).every(c=>
      capabilities.some(ob=>ob.name===c.name&&ob.status==='SUPPORTED')));
  const capability={ready:enabled,capabilities};
  const frontend=fileURLToPath(new URL('../../web/dist/',import.meta.url));
  if(existsSync(frontend))await app.register(serveStatic,{root:frontend,wildcard:false});
  if(opts.native && opts.pool && enabled){
    const stop=new AbortController();
    const config={...opts.native,pool:opts.pool,signal:stop.signal};
    let worker:Promise<void>|undefined;
    app.addHook('onReady',async()=>{
      worker=(async()=>{
        while(!stop.signal.aborted){
          try{await dispatchNext(config);}catch{app.log.error('Native scheduler iteration failed');}
          await delay(500,undefined,{signal:stop.signal}).catch(()=>{});
        }
      })();
    });
    app.addHook('onClose',async()=>{stop.abort();await worker;});
  }
  const makeBootstrap=async(csrf:string,owner:{id:string;name:string}):Promise<DTO<'Bootstrap'>>=>({
    owner,csrfToken:csrf,
    scenarios:[{id:'direct-chat',name:'直接对话',enabled:true,reason:null},{id:'log-inspection',name:'日志排查',enabled:true,reason:null},
      {id:'data-analysis',name:'数据分析',enabled:true,reason:null},{id:'code-review',name:'Codea Review',enabled:capability.ready,
      reason:capability.ready?null:'OpenCode native capabilities require target-environment validation'}],
    capabilities:capability.capabilities,
    limits:{maxQueued:20,maxActiveRuns:1,notesMaxLength:2000,messageMaxLength:8000,previewTTLSeconds:300},
  });
  app.setErrorHandler((e,req,reply)=>{
    const problem=e instanceof AuthProblem?e:null;
    const status=problem?.status??(typeof e.statusCode==='number'&&e.statusCode<500?e.statusCode:503);
    const code=problem?.code??(status===503?'SERVICE_UNAVAILABLE':'INVALID_REQUEST');
    reply.status(status).send({error:{code,message:code,retryable:status>=500,details:{}},requestId:randomUUID()});
  });
  app.get('/api/v1/health',async(req,reply)=>{
    let database:'READY'|'UNAVAILABLE'='READY';
    try{if(!opts.pool)throw new Error();await opts.pool.query('SELECT 1');}catch{database='UNAVAILABLE';}
    const health:DTO<'Health'>={
      status:database==='READY'&&capability.ready?'READY':database==='READY'?'DEGRADED':'UNAVAILABLE',
      database,opencode:capability.ready?'READY':'UNVERIFIED',build:'UNVERIFIED',
      capabilities:capability.capabilities,
    };
    return reply.status(health.status==='READY'?200:503).send(result(health));
  });
  const localSession=opts.localOnly===true&&!opts.pool;
  const workspace=localSession?await resolveLocalWorkspace(opts.workspace):undefined;
  if(workspace){
    const store=new WorkspaceProfile(workspace);
    app.addHook('onRequest',async(req,reply)=>{
      const route=req.routeOptions.url;
      if(!route?.startsWith('/api/v1/')||route==='/api/v1/health'||route==='/api/v1/auth/local-session'||
        (route==='/api/v1/auth/session'&&req.method==='DELETE'))return;
      let session;
      try{session=opts.auth.session(req);}catch(e){
        if(!(e instanceof AuthProblem)||e.code!=='AUTH_REQUIRED')throw e;
      }
      if(!session?.profile||!store.matches(session.profile.key))throw new AuthProblem(401,'PROFILE_LOGIN_REQUIRED');
      reply.header('Cache-Control','no-store');
    });
    app.post('/api/v1/auth/local-session',async(req,reply)=>{
      opts.auth.requireLocalOrigin(req);
      const profile=parseProfile(req.body);
      const session=opts.auth.openLocal(req,reply,{name:profile.name,key:profileKey(profile)},()=>store.save(profile));
      return reply.header('Cache-Control','no-store').send(result(await makeBootstrap(session.csrf,{id:session.ownerId,name:profile.name})));
    });
  }else app.post<{Body:{pairingCode:string}}>('/api/v1/auth/session',{schema:{
    body:{type:'object',additionalProperties:false,required:['pairingCode'],
      properties:{pairingCode:{type:'string',minLength:1,maxLength:256}}},
  }},async(req,reply)=>{
    const owner=opts.pool?await ensureLocalOwner(opts.pool):{id:'local-workbench',name:'Local Developer'};
    const session=opts.auth.pair(req,reply,req.body.pairingCode,owner.id);
    return reply.send(result(await makeBootstrap(session.csrf,owner)));
  });
  app.delete('/api/v1/auth/session',async(req,reply)=>{
    opts.auth.logout(req,reply);return reply.status(204).send();
  });
  app.get('/api/v1/bootstrap',async(req,reply)=>{
    const session=opts.auth.session(req);
    const owner=opts.pool?await ensureLocalOwner(opts.pool):{id:'local-workbench',name:session.profile?.name??'Local Developer'};
    if(owner.id!==session.ownerId)throw new AuthProblem(401,'OWNER_MISMATCH');
    return reply.send(result(await makeBootstrap(session.csrf,owner)));
  });
  await registerChat(app,opts.auth,{mayStart:opts.autoOpenCode,workspace});
  if(!opts.pool){
    for(const path of ['/api/v1/tasks','/api/v1/repositories'])app.get(path,async(req)=>{
      opts.auth.session(req);return result({items:[],nextCursor:null});
    });
    return app;
  }
  registerGitRoutes(app,{pool:opts.pool,auth:opts.auth,
    roots:(process.env.HARNESS_REPOSITORY_ROOTS??'').split(delimiter).filter(Boolean),
    refKey:process.env.HARNESS_REF_KEY??''});
  registerTaskRoutes(app,{pool:opts.pool,auth:opts.auth,
    roots:(process.env.HARNESS_REPOSITORY_ROOTS??'').split(delimiter).filter(Boolean),
    cursorKey:process.env.HARNESS_CURSOR_KEY??'',ready:enabled,notesSupported:enabled&&capabilities.some(c=>c.name==='instruction.entry'&&c.status==='SUPPORTED')});
  registerEventRoutes(app,{pool:opts.pool,auth:opts.auth});
  registerRunStopRoute(app,{pool:opts.pool,auth:opts.auth,adapter:enabled?opts.native?.adapter:undefined});
  registerInteractionRoutes(app,{pool:opts.pool,auth:opts.auth});
  registerMessageRoutes(app,{pool:opts.pool,auth:opts.auth,
    cursorKey:process.env.HARNESS_CURSOR_KEY??''});
  registerArtifactRoutes(app,{pool:opts.pool,auth:opts.auth,
    dataRoot:process.env.HARNESS_DATA_DIR??'.workbench-local'});
  registerTaskFileRoutes(app,{pool:opts.pool,auth:opts.auth,
    dataRoot:process.env.HARNESS_DATA_DIR??'.workbench-local'});
  return app;
}
