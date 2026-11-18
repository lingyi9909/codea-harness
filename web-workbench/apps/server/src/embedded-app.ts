import Fastify from 'fastify';
import serveStatic from '@fastify/static';
import {existsSync} from 'node:fs';
import {fileURLToPath} from 'node:url';
import {AuthProblem} from './auth.ts';
import type {EmbeddedAuth} from './embed-auth.ts';
import type {ExecutionRegistry} from './execution-registry.ts';
import type {ChatBindingStore} from '../../../packages/storage/src/chat-bindings.ts';
import {resolveLocalWorkspace} from './modules/chat/routes.ts';
import {SceneConfigStore} from './scene-config.ts';
import {registerEmbedded} from './embed-routes.ts';
import type {InstitutionQueryAdapter} from './institution-query.ts';
export async function createEmbeddedApp(opts:{auth:EmbeddedAuth;execution:ExecutionRegistry;workspace:string;bindings:ChatBindingStore;ensureOwner:(id:string,subject:string)=>Promise<void>;storageCheck:()=>void;institutionAdapter?:InstitutionQueryAdapter;resetSlot?:(owner:string)=>Promise<void>}){
  const app=Fastify({logger:false,bodyLimit:32768,trustProxy:false});
  try{
    const workspace=await resolveLocalWorkspace(opts.workspace),sceneConfig=new SceneConfigStore(workspace);
    await sceneConfig.initialize();
    const frontend=fileURLToPath(new URL('../../web/dist/',import.meta.url));
    if(existsSync(frontend))await app.register(serveStatic,{root:frontend,wildcard:false,index:false});
    app.setErrorHandler((e,_req,reply)=>{
      const p=e instanceof AuthProblem?e:null,status=p?.status??(typeof e.statusCode==='number'&&e.statusCode<500?e.statusCode:503),code=p?.code??(status>=500?'SERVICE_UNAVAILABLE':'INVALID_REQUEST');
      reply.status(status).send({error:{code,message:code,retryable:status>=500,details:{}}});
    });
    app.addHook('onRequest',async(_req,reply)=>{reply.header('Cache-Control','no-store');});
    app.get('/api/v1/health',async(_req,reply)=>{try{opts.storageCheck();return {data:{status:'READY',storage:'sqlite'}};}catch{return reply.status(503).send({data:{status:'UNAVAILABLE',storage:'sqlite'}});}});
    app.get('/',async(_req,reply)=>reply.redirect('/embed/institution-message'));
    for(const path of ['/embed/institution-message','/embed/message'])app.get(path,async(req,reply)=>{
      if(!opts.auth.origin(req))throw new AuthProblem(403,'ORIGIN_INVALID');
      if(!existsSync(frontend+'index.html'))throw new AuthProblem(503,'FRONTEND_BUILD_MISSING');
      const origins=[...opts.auth.config.clients.map(c=>c.parentOrigin),...(opts.auth.config.anonymous?.parentOrigins??[])];
      return reply.header('Content-Security-Policy',"frame-ancestors 'self' "+[...new Set(origins)].join(' ')).header('Referrer-Policy','no-referrer').sendFile('index.html');
    });
    await registerEmbedded(app,{auth:opts.auth,execution:opts.execution,workspace,sceneConfig,bindings:opts.bindings,ensureOwner:opts.ensureOwner,institutionAdapter:opts.institutionAdapter,resetSlot:opts.resetSlot});
    return app;
  }catch(e){await app.close().catch(()=>{});throw e;}
}
