import Fastify from 'fastify';
import { randomUUID } from 'node:crypto';
import { delimiter } from 'node:path';
import { registerGitRoutes } from './modules/repositories/routes.ts';
import { registerTaskFileRoutes } from './modules/tasks/files.ts';
import { registerTaskRoutes } from './modules/tasks/routes.ts';
import { registerEventRoutes } from './modules/events/routes.ts';
import { registerRunStopRoute } from './modules/runs/routes.ts';
import type { Pool } from 'mysql2/promise';
import { LocalAuth, AuthProblem, ensureLocalOwner } from './auth.ts';
import { evaluateCapabilities } from '../../../packages/opencode-adapter/src/probe.ts';
import type { DTO } from '../../../packages/contracts/src/domain.ts';

const result=<T>(data:T)=>({data,requestId:randomUUID()});
export async function createApp(opts:{pool:Pool;auth:LocalAuth}){
  const app=Fastify({logger:false,bodyLimit:32768,trustProxy:false});
  const capability=evaluateCapabilities();
  const makeBootstrap=async(csrf:string,owner:{id:string;name:string}):Promise<DTO<'Bootstrap'>>=>({
    owner,csrfToken:csrf,
    scenarios:[{id:'code-review',name:'Codea Review',enabled:capability.ready,
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
    try{await opts.pool.query('SELECT 1');}catch{database='UNAVAILABLE';}
    const health:DTO<'Health'>={
      status:database==='READY'&&capability.ready?'READY':database==='READY'?'DEGRADED':'UNAVAILABLE',
      database,opencode:capability.ready?'READY':'UNVERIFIED',build:'UNVERIFIED',
      capabilities:capability.capabilities,
    };
    return reply.status(health.status==='READY'?200:503).send(result(health));
  });
  app.post<{Body:{pairingCode:string}}>('/api/v1/auth/session',{schema:{
    body:{type:'object',additionalProperties:false,required:['pairingCode'],
      properties:{pairingCode:{type:'string',minLength:1,maxLength:256}}},
  }},async(req,reply)=>{
    const owner=await ensureLocalOwner(opts.pool);
    const session=opts.auth.pair(req,reply,req.body.pairingCode,owner.id);
    return reply.send(result(await makeBootstrap(session.csrf,owner)));
  });
  app.delete('/api/v1/auth/session',async(req,reply)=>{
    opts.auth.logout(req,reply);return reply.status(204).send();
  });
  app.get('/api/v1/bootstrap',async(req,reply)=>{
    const session=opts.auth.session(req);
    const owner=await ensureLocalOwner(opts.pool);
    if(owner.id!==session.ownerId)throw new AuthProblem(401,'OWNER_MISMATCH');
    return reply.send(result(await makeBootstrap(session.csrf,owner)));
  });
  registerGitRoutes(app,{pool:opts.pool,auth:opts.auth,
    roots:(process.env.HARNESS_REPOSITORY_ROOTS??'').split(delimiter).filter(Boolean),
    refKey:process.env.HARNESS_REF_KEY??''});
  registerTaskRoutes(app,{pool:opts.pool,auth:opts.auth,
    roots:(process.env.HARNESS_REPOSITORY_ROOTS??'').split(delimiter).filter(Boolean),
    cursorKey:process.env.HARNESS_CURSOR_KEY??'',notesSupported:false});
  registerEventRoutes(app,{pool:opts.pool,auth:opts.auth});
  registerRunStopRoute(app,{pool:opts.pool,auth:opts.auth});
  registerTaskFileRoutes(app,{pool:opts.pool,auth:opts.auth,
    dataRoot:process.env.HARNESS_DATA_DIR??'.workbench-local'});
  // Native execution and artifacts remain unavailable until T06-T10.
  return app;
}
