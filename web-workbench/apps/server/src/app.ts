import type {chatConnection} from './modules/chat/connection.ts';
import {PageVisitStore,validPageId} from './page-visits.ts';
import {SceneAccessStore} from './scene-access.ts';
import type {ChatBindingStore} from '../../../packages/storage/src/chat-bindings.ts';
import type {InstitutionQueryAdapter} from './institution-query.ts';
import {registerEmbedded} from './embed-routes.ts';
import type {EmbeddedAuth} from './embed-auth.ts';
import {MysqlIdentityStore} from '../../../packages/storage/src/identity.ts';
import {UserContextResolver} from './user-context.ts';
import {BusinessVerificationBridge,MockLogProvider} from './business-verification.ts';
import type {ExecutionRegistry} from './execution-registry.ts';
import {MysqlChatBindings} from '../../../packages/storage/src/chat-bindings.ts';
import { MultiUserAuth } from './multi-user-auth.ts';
import { parseProfile, profileKey, WorkspaceProfile } from './local-profile.ts';
import { registerChat, resolveLocalWorkspace } from './modules/chat/routes.ts';
import { SceneConfigStore, SCENE_CONFIG_PATH, SCENE_ACCESS_PATH } from './scene-config.ts';
import Fastify from 'fastify';
import serveStatic from '@fastify/static';
import { existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';
import { dispatchNext,type SchedulerConfig } from './workers/scheduler.ts';
import { randomUUID } from 'node:crypto';
import { delimiter, join } from 'node:path';
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
export async function createApp(opts:{nativeConnection?:ReturnType<typeof chatConnection>;localOnly?:boolean;workspace?:string;autoOpenCode?:boolean;pool?:Pool;auth:LocalAuth|MultiUserAuth;execution?:ExecutionRegistry;mockBusiness?:boolean;embedded?:EmbeddedAuth;institutionAdapter?:InstitutionQueryAdapter;resetEmbedSlot?:(owner:string)=>Promise<void>;embeddedBindings?:ChatBindingStore;embeddedStorageCheck?:()=>void;native?:Omit<SchedulerConfig,'pool'|'signal'>;pageVisits?:PageVisitStore}){
  const app=Fastify({logger:false,bodyLimit:32768,trustProxy:false});
  const capabilities=opts.native?await opts.native.adapter.probe():evaluateCapabilities().capabilities;
  const enabled=!(opts.auth instanceof MultiUserAuth)&&Boolean(opts.native&&opts.native.nativeVersion.trim()&&opts.native.nativeVersion!=='UNVERIFIED'&&
    evaluateCapabilities().capabilities.filter(c=>c.required).every(c=>
      capabilities.some(ob=>ob.name===c.name&&ob.status==='SUPPORTED')));
  const capability={ready:enabled,capabilities};
  const frontend=fileURLToPath(new URL('../../web/dist/',import.meta.url));
  if(existsSync(frontend))await app.register(serveStatic,{root:frontend,wildcard:false});
  app.get('/component-lab',async(req,reply)=>{
    if(sceneAccess&&opts.auth instanceof LocalAuth&&!opts.auth.isLocal(req)&&!sceneAccess.policy('component-lab').lan)
      throw new AuthProblem(403,'SCENE_LAN_DISABLED');
    if(!existsSync(frontend+'index.html'))throw new AuthProblem(503,'FRONTEND_BUILD_MISSING');
    return reply.header('Cache-Control','no-store').sendFile('index.html');
  });
  app.get('/config',async(req,reply)=>{
    if(opts.auth instanceof LocalAuth&&!opts.auth.isLocal(req))throw new AuthProblem(403,'CONFIG_LOCAL_ONLY');
    if(!existsSync(frontend+'index.html'))throw new AuthProblem(503,'FRONTEND_BUILD_MISSING');
    return reply.header('Cache-Control','no-store').header('Content-Security-Policy',"frame-ancestors 'none'").sendFile('index.html');
  });
  if(!(opts.auth instanceof MultiUserAuth) && opts.native && opts.pool && enabled){
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
  const makeBootstrap=async(csrf:string,owner:{id:string;name:string}):Promise<DTO<'Bootstrap'>&{identityMode:'local'|'multi-user'|'lan';businessVerification:boolean}>=>({
    owner,csrfToken:csrf,businessVerification:opts.auth instanceof MultiUserAuth&&opts.mockBusiness===true,identityMode:opts.auth instanceof MultiUserAuth?'multi-user':owner.id.startsWith('lan:')?'lan':'local',
    scenarios:[{id:'direct-chat',name:'直接对话',enabled:true,reason:null},{id:'log-inspection',name:'问题排查',enabled:true,reason:null},
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
    if(opts.embeddedStorageCheck){
      try{opts.embeddedStorageCheck();return result({status:'READY',storage:'sqlite'});}
      catch{return reply.status(503).send(result({status:'UNAVAILABLE',storage:'sqlite'}));}
    }
    let database:'READY'|'UNAVAILABLE'='READY';
    try{if(!opts.pool)throw new Error();await opts.pool.query('SELECT 1');}catch{database='UNAVAILABLE';}
    const health:DTO<'Health'>={
      status:database==='READY'&&capability.ready?'READY':database==='READY'?'DEGRADED':'UNAVAILABLE',
      database,opencode:capability.ready?'READY':'UNVERIFIED',build:'UNVERIFIED',
      capabilities:capability.capabilities,
    };
    return reply.status(health.status==='READY'?200:503).send(result(health));
  });
  const multi=opts.auth instanceof MultiUserAuth;
  if(multi&&!opts.pool)throw new Error('MULTI_USER_MYSQL_REQUIRED');
  app.addHook('onRequest',async(req,reply)=>{if(req.url.startsWith('/api/'))reply.header('Cache-Control','no-store');});
  app.post<{Body:{page:unknown;visitorId:unknown}}>('/api/v1/page-visits',{
    schema:{body:{type:'object',additionalProperties:false,required:['page','visitorId'],
      properties:{page:{type:'string',maxLength:48},visitorId:{type:'string',maxLength:36}}}},
  },async(req,reply)=>{
    const origin=req.headers.origin,host=req.headers.host;
    if(!host||origin!=='http://'+host&&origin!=='https://'+host)throw new AuthProblem(403,'VISIT_ORIGIN_INVALID');
    if(!validPageId(req.body.page)||typeof req.body.visitorId!=='string')throw new AuthProblem(400,'PAGE_INVALID');
    opts.pageVisits?.record(req.body.page,req.body.visitorId);
    return reply.send(result({recorded:true}));
  });
  const localSession=opts.localOnly===true&&!opts.pool;
  const workspace=opts.workspace?await resolveLocalWorkspace(opts.workspace):localSession?await resolveLocalWorkspace():undefined;
  const sceneConfig=workspace?new SceneConfigStore(workspace):undefined;
  const sceneAccess=workspace?new SceneAccessStore(workspace):undefined;
  await sceneConfig?.initialize();
  if(multi){
    const auth=opts.auth as MultiUserAuth;
    app.post('/api/v1/auth/login',async(req,reply)=>{
      const session=await auth.login(req,reply,req.body);
      return result(await makeBootstrap(session.csrf,{id:session.ownerId,name:session.profile!.name}));
    });
    // Existing login page keeps its two fields; server mode uses the same UI.
    app.post('/api/v1/auth/local-session',async(req,reply)=>{
      const body=req.body as {name?:unknown;phone?:unknown}|undefined;
      const session=await auth.login(req,reply,{username:body?.name,phone:body?.phone});
      return result(await makeBootstrap(session.csrf,{id:session.ownerId,name:session.profile!.name}));
    });
  }else if(workspace&&localSession){
    const store=new WorkspaceProfile(workspace);
    app.addHook('onRequest',async(req,reply)=>{
      const route=req.routeOptions.url;
      if(!route?.startsWith('/api/v1/')||route.startsWith('/api/v1/embed/')||route==='/api/v1/page-visits'||route==='/api/v1/health'||route==='/api/v1/auth/local-session'||
        (route==='/api/v1/auth/session'&&req.method==='DELETE'))return;
      let session;
      try{session=await opts.auth.session(req);}catch(e){
        if(!(e instanceof AuthProblem)||e.code!=='AUTH_REQUIRED')throw e;
      }
      if(!session?.profile||(session.scope==='lan'? !session.ownerId.startsWith('lan:'): !store.matches(session.profile.key)))throw new AuthProblem(401,'PROFILE_LOGIN_REQUIRED');
      reply.header('Cache-Control','no-store');
    });
    app.post('/api/v1/auth/local-session',async(req,reply)=>{
      const localAuth=opts.auth as LocalAuth;
      const fields=req.body as {name?:unknown;phone?:unknown;lanCode?:unknown}|undefined;
      const profile=parseProfile({name:fields?.name,phone:fields?.phone});
      const session=localAuth.isLan(req)
        ?localAuth.openLan(req,reply,{name:profile.name,key:profileKey(profile)},fields?.lanCode)
        :localAuth.openLocal(req,reply,{name:profile.name,key:profileKey(profile)},()=>store.save(profile));
      return reply.header('Cache-Control','no-store').send(result(await makeBootstrap(session.csrf,{id:session.ownerId,name:profile.name})));
    });
  }else app.post<{Body:{pairingCode:string}}>('/api/v1/auth/session',{schema:{
    body:{type:'object',additionalProperties:false,required:['pairingCode'],
      properties:{pairingCode:{type:'string',minLength:1,maxLength:256}}},
  }},async(req,reply)=>{
    const owner=opts.pool?await ensureLocalOwner(opts.pool):{id:'local-workbench',name:'Local Developer'};
    const session=(opts.auth as LocalAuth).pair(req,reply,req.body.pairingCode,owner.id);
    return reply.send(result(await makeBootstrap(session.csrf,owner)));
  });
  app.delete('/api/v1/auth/session',async(req,reply)=>{
    await opts.auth.logout(req,reply);return reply.status(204).send();
  });
  app.get('/api/v1/bootstrap',async(req,reply)=>{
    const session=await Promise.resolve().then(()=>opts.auth.session(req)).catch(error=>{if(multi&&error instanceof AuthProblem&&error.code==='AUTH_REQUIRED')throw new AuthProblem(401,'PROFILE_LOGIN_REQUIRED');throw error;});
    const owner=multi?{id:session.ownerId,name:session.profile!.name}:opts.pool?await ensureLocalOwner(opts.pool):{id:session.ownerId,name:session.profile?.name??'Local Developer'};
    if(owner.id!==session.ownerId)throw new AuthProblem(401,'OWNER_MISMATCH');
    return reply.send(result(await makeBootstrap(session.csrf,owner)));
  });
  if(sceneConfig){
    app.get('/api/v1/scene-runtime',async req=>{
      await opts.auth.session(req);
      return result({workspace,configPath:'',accessPath:'',...sceneConfig.read(),canEdit:false,username:null});
    });
  }
  if(sceneConfig){
    // Only a named local login can match the configured editor usernames.
    // Pairing-code sessions have no verified username and remain read-only.
    const editorUsername=(session:{profile?:{name:string}})=>session.profile?.name??null;
    app.get('/api/v1/scene-config',async(req,reply)=>{
      if(opts.auth instanceof LocalAuth&&!opts.auth.isLocal(req))throw new AuthProblem(403,'CONFIG_LOCAL_ONLY');
      const session=await opts.auth.session(req),username=editorUsername(session);
      return reply.header('Cache-Control','no-store').send(result({
        workspace,configPath:join(workspace!,SCENE_CONFIG_PATH),accessPath:join(workspace!,SCENE_ACCESS_PATH),
        ...sceneConfig.read(),canEdit:!multi&&sceneConfig.canEdit(username??undefined),
        username,
      }));
    });
    app.put<{Body:{config:unknown;revision:string}}>('/api/v1/scene-config',{
      schema:{body:{type:'object',additionalProperties:false,required:['config','revision'],properties:{
        config:{type:'object'},revision:{type:'string',minLength:64,maxLength:64}}}},
    },async(req,reply)=>{
      if(opts.auth instanceof LocalAuth&&!opts.auth.isLocal(req))throw new AuthProblem(403,'CONFIG_LOCAL_ONLY');
      const session=await opts.auth.mutation(req);
      if(multi||!sceneConfig.canEdit(editorUsername(session)??undefined))throw new AuthProblem(403,'SCENE_CONFIG_EDIT_FORBIDDEN');
      const saved=await sceneConfig.save(req.body.config,req.body.revision);
      return reply.header('Cache-Control','no-store').send(result({...saved,canEdit:true}));
    });
  }
  if(sceneConfig&&sceneAccess){
    const editor=async(req:import('fastify').FastifyRequest,write=false)=>{
      if(opts.auth instanceof LocalAuth&&!opts.auth.isLocal(req))throw new AuthProblem(403,'CONFIG_LOCAL_ONLY');
      const session=write?await opts.auth.mutation(req):await opts.auth.session(req);
      if(multi||!sceneConfig.canEdit(session.profile?.name))throw new AuthProblem(403,'SCENE_CONFIG_EDIT_FORBIDDEN');
      return session;
    };
    app.get('/api/v1/scene-access',async req=>{
      await editor(req);return result(sceneAccess.read());
    });
    app.put<{Body:{config:unknown;revision:string}}>('/api/v1/scene-access',{
      schema:{body:{type:'object',additionalProperties:false,required:['config','revision'],properties:{
        config:{type:'object'},revision:{type:'string',minLength:64,maxLength:64}}}},
    },async req=>{
      await editor(req,true);return result(sceneAccess.save(req.body.config,req.body.revision));
    });
    app.get('/api/v1/page-visits',async req=>{
      await editor(req);return result(opts.pageVisits?.summary()??{days:7,rows:[],daily:[]});
    });
  }
  if(multi)app.addHook('onRoute',route=>{
    if(!String(route.url).startsWith('/api/v1/chat/'))return;
    const handler=route.handler;
    route.handler=async function(req,reply){
      const session=await opts.auth.session(req);
      if(!opts.execution)throw new AuthProblem(503,'EXECUTION_NOT_PROVISIONED');
      return opts.execution.run(session.ownerId,async()=>handler.call(this,req,reply));
    };
  });
  let business: {resolver:UserContextResolver;bridge:BusinessVerificationBridge}|undefined;
  if(multi&&opts.mockBusiness){
    const identities=new MysqlIdentityStore(opts.pool!);
    const resolver=new UserContextResolver(owner=>identities.profile(owner));
    business={resolver,bridge:new BusinessVerificationBridge(resolver,new MockLogProvider())};
  }
  if(opts.embedded){
    if((!multi&&!localSession)||(!opts.pool&&!opts.embeddedBindings)||!opts.execution||!workspace||!sceneConfig)throw new Error('EMBED_REQUIRES_MULTI_USER_EXECUTION');
    for(const path of ['/embed/message','/embed/institution-message'])app.get(path,async(req,reply)=>{
      const id=path==='/embed/institution-message'?'institution-message':'message';
      if(sceneAccess&&opts.auth instanceof LocalAuth&&!opts.auth.isLocal(req)&&!sceneAccess.policy(id).lan)
        throw new AuthProblem(403,'SCENE_LAN_DISABLED');
      if(!opts.embedded!.origin(req))throw new AuthProblem(403,'ORIGIN_INVALID');
      if(!existsSync(frontend+'index.html'))throw new AuthProblem(503,'FRONTEND_BUILD_MISSING');
      reply.header('Content-Security-Policy',"frame-ancestors 'self' "+[...new Set([...opts.embedded!.config.clients.map(c=>c.parentOrigin),...(opts.embedded!.config.anonymous?.parentOrigins??[])])].join(' '));
      return reply.header('Referrer-Policy','no-referrer').header('Cache-Control','no-store').sendFile('index.html');
    });
    await registerEmbedded(app,{auth:opts.embedded,execution:opts.execution,workspace,sceneConfig,sceneAccess,pool:opts.pool,bindings:opts.embeddedBindings,institutionAdapter:opts.institutionAdapter,resetSlot:opts.resetEmbedSlot});
  }
  // Review task pages stay reachable on LAN; their APIs honour the per-page LAN setting.
  app.addHook('preHandler',async req=>{
    if(!sceneAccess||!req.url.startsWith('/api/v1/')||!opts.auth||!(opts.auth instanceof LocalAuth)||opts.auth.isLocal(req))return;
    if((req.url.startsWith('/api/v1/tasks')||req.url.startsWith('/api/v1/repositories')||
      req.url.startsWith('/api/v1/review-previews')||req.url.startsWith('/api/v1/runs'))&&
      !sceneAccess.policy('review').lan)throw new AuthProblem(403,'SCENE_LAN_DISABLED');
  });
  await registerChat(app,opts.auth,{nativeConnection:opts.nativeConnection,mayStart:opts.autoOpenCode,workspace,sceneConfig,sceneAccess,business,...(multi?{bindings:new MysqlChatBindings(opts.pool!),execution:opts.execution,multiUser:true}:{})});
  if(!opts.pool){
    for(const path of ['/api/v1/tasks','/api/v1/repositories'])app.get(path,async(req)=>{
      await opts.auth.session(req);return result({items:[],nextCursor:null});
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
