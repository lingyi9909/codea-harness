import Fastify, { type FastifyInstance } from 'fastify';
import { createReadStream, existsSync, statSync } from 'node:fs';
import { resolve, extname, relative, isAbsolute } from 'node:path';
import { fileURLToPath } from 'node:url';
import { databaseReady, ensureLocalOwner } from '../../../packages/storage/src/db.ts';
import { evaluateCapabilities, type Observation } from '../../../packages/opencode-adapter/src/probe.ts';
import { csrfFor, registerAuth } from './auth.ts';
import { registerRepositories } from './modules/repositories.ts';
import { registerTaskReads } from './modules/tasks.ts';
import { registerTaskWrites } from './modules/actions.ts';
import { registerEvents } from './modules/events.ts';
import { registerArtifacts } from './modules/artifacts.ts';
import type { DTO } from '../../../packages/contracts/src/domain.ts';

export type ServerOptions={
  /** Strictly real verified company Windows evidence; no mock data accepted. */
  nativeObservations?:Partial<Record<Parameters<typeof evaluateCapabilities>[0] extends infer O?
    keyof NonNullable<O>:never,Observation>>;
};
export function createApp(options:ServerOptions={}):FastifyInstance {
  const app=Fastify({logger:false,bodyLimit:32768,disableRequestLogging:true});
  const summary=evaluateCapabilities(options.nativeObservations);
  const nativeReady=()=>summary.ready;
  const owner=async()=> (await ensureLocalOwner()).id;
  registerAuth(app);
  app.setErrorHandler((error,req,reply)=>{
    const native=error as Error&{statusCode?:number;code?:string};
    const status=native.statusCode&&native.statusCode>=400&&native.statusCode<=599?
      native.statusCode:500;
    const code=status===500?'INTERNAL_ERROR':native.code??'INVALID_REQUEST';
    reply.status(status).send({
      error:{code,message:status===500?'本地服务发生内部错误，请查看脱敏诊断日志。':code,
        retryable:[429,503].includes(status),details:{}},requestId:req.id,
    });
  });
  app.get('/api/v1/health',async(req,reply)=>{
    const database=await databaseReady();
    const health:DTO<'Health'>={
      status:database&&summary.ready?'READY':database?'DEGRADED':'UNAVAILABLE',
      database:database?'READY':'UNAVAILABLE',
      opencode:summary.ready?'READY':'UNVERIFIED',
      build:'T01-PENDING',capabilities:summary.capabilities,
    };
    return reply.status(database?200:503).send({data:health,requestId:req.id});
  });
  app.get('/api/v1/bootstrap',async(req,reply)=>{
    const local=await ensureLocalOwner();
    const enabled=summary.ready&&await databaseReady();
    const data:DTO<'Bootstrap'>={
      owner:{id:local.id,name:local.name},csrfToken:csrfFor(req),
      scenarios:[{id:'code-review',name:'Codea Review 2.0',
        enabled,reason:enabled?null:'尚未完成当前公司构建的 OpenCode 原生能力验证，真实 Review 已安全禁用。'}],
      capabilities:summary.capabilities,
      limits:{maxQueued:20,maxActiveRuns:1,notesMaxLength:2000,
        messageMaxLength:8000,previewTTLSeconds:300},
    };
    return reply.send({data,requestId:req.id});
  });
  registerRepositories(app,owner);
  registerTaskReads(app,owner);
  registerTaskWrites(app,owner,nativeReady);
  registerEvents(app,owner);
  registerArtifacts(app,owner);
  // The built Vite UI is served from the same loopback origin in production.
  const dist=fileURLToPath(new URL('../../web/dist/',import.meta.url));
  const mime:Record<string,string>={'.html':'text/html; charset=utf-8',
    '.js':'text/javascript; charset=utf-8','.css':'text/css; charset=utf-8',
    '.svg':'image/svg+xml','.png':'image/png','.jpg':'image/jpeg',
    '.ico':'image/x-icon','.woff2':'font/woff2'};
  async function serve(requested:string,reply:import('fastify').FastifyReply){
    const file=resolve(dist,requested);
    const rel=relative(dist,file);
    if(rel.startsWith('..')||isAbsolute(rel))return reply.status(403).send();
    if(!existsSync(file)||!statSync(file).isFile())return reply.status(404).send();
    reply.header('Cache-Control',requested==='index.html'?'no-store':'public, max-age=3600');
    return reply.type(mime[extname(file)]??'application/octet-stream').send(createReadStream(file));
  }
  app.get('/',async(_,reply)=>serve('index.html',reply));
  app.get<{Params:{'*':string}}>('/static/*',async(req,reply)=>serve(req.params['*'],reply));
  app.get<{Params:{'*':string}}>('/assets/*',async(req,reply)=>serve('assets/'+req.params['*'],reply));
  return app;
}
