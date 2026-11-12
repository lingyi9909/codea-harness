import {activity,activityPreview} from './activity.ts';
import {registerAccess} from './access.ts';
import {chatConnection} from './connection.ts';
import { mkdir,realpath,stat } from 'node:fs/promises';
import {once} from 'node:events';
import { createHash,randomUUID } from 'node:crypto';
import { delimiter,isAbsolute,relative,resolve,sep } from 'node:path';
import { pathToFileURL } from 'node:url';
import type { FastifyInstance } from 'fastify';
import type { ChatMessage,ChatStatus,ChatModel,ChatFragment } from '../../../../../packages/contracts/src/chat.ts';
import { AuthProblem,type LocalAuth } from '../../auth.ts';

const wrap=<T>(data:T)=>({data,requestId:randomUUID()});
const within=(root:string,target:string)=>{
  const part=relative(root,target);
  return part!=='..'&&!part.startsWith('..'+sep)&&!isAbsolute(part);
};
const revision=(p:{id:string;action:string;resources:string[]})=>
  createHash('sha256').update(JSON.stringify([p.id,p.action,p.resources])).digest('hex');
type Params={id:string};
const property=(maxLength:number)=>({type:'string',minLength:1,maxLength});
const body=(properties:Record<string,unknown>,required=Object.keys(properties))=>({
  body:{type:'object',additionalProperties:false,required,properties},
});

export function registerChat(app:FastifyInstance,auth:LocalAuth,mayStart=false){
  const log=registerAccess(app,auth);
  const client=chatConnection(log,mayStart);
  const configured=(process.env.HARNESS_CHAT_ROOTS??process.env.HARNESS_REPOSITORY_ROOTS??'').split(delimiter).filter(Boolean);
  const roots=configured.length?configured:[resolve('chat-files')];
  async function directory(input:string){
    if(!isAbsolute(input))throw new AuthProblem(400,'ABSOLUTE_DIRECTORY_REQUIRED');
    const path=await realpath(input).catch(()=>null);
    if(!path||!(await stat(path)).isDirectory())throw new AuthProblem(400,'DIRECTORY_NOT_FOUND');
    if(!configured.length&&mayStart)return path; // Paired local-only workbench with no administrator-defined roots.
    for(const root of roots){
      const canonical=await realpath(root).catch(()=>null);
      if(canonical&&within(canonical,path))return path;
    }
    throw new AuthProblem(403,'DIRECTORY_OUTSIDE_CHAT_ROOTS');
  }
  async function owned(id:string,owner:string){
    if(!/^[\w-]{1,128}$/.test(id))throw new AuthProblem(400,'INVALID_SESSION');
    const session=await (await client()).session.get({sessionID:id});
    if(session.id!==id||(session.metadata?.harnessScenario!=='direct-chat'&&session.metadata?.harnessScenario!=='log-inspection')||session.metadata.harnessOwner!==owner)
      throw new AuthProblem(404,'CHAT_NOT_FOUND');
    await directory(session.location.directory);
    return session;
  }
  app.get('/api/v1/chat/settings',async(req)=>{
    auth.session(req);return wrap({configured:true,directories:roots});
  });
  async function project(input?:string){
    if(!configured.length&&(!input||input===roots[0]))await mkdir(roots[0],{recursive:true});
    return directory(input??roots[0]);
  }
  async function models(dir:string):Promise<ChatModel[]>{
    const result=await (await client()).model.list({location:{directory:dir}});
    return result.data.filter(m=>m.enabled).map(({id,providerID,name})=>({id,providerID,name}));
  }
  type ProjectInput={Body:{directory?:string}};
  app.post<ProjectInput>('/api/v1/chat/project',{schema:body({directory:property(4096)})},async req=>{
    auth.mutation(req);return wrap({directory:await project(req.body.directory)});
  });
  app.post<ProjectInput>('/api/v1/chat/models',async req=>{
    auth.mutation(req);
    const input=req.body?.directory;
    if(input!==undefined&&(typeof input!=='string'||input.length>4096))throw new AuthProblem(400,'INVALID_DIRECTORY');
    return wrap(await models(await project(input)));
  });
  type Selection={providerID:string;id:string};
  app.post<{Body:{directory?:string;model?:Selection;scenario?:'log-inspection'|'direct-chat'}}>('/api/v1/chat/sessions',{schema:body({
    directory:property(4096),scenario:{enum:['log-inspection','direct-chat']},model:{type:'object',additionalProperties:false,required:['providerID','id'],
      properties:{providerID:property(256),id:property(512)}}},[])},async(req,reply)=>{
    const {ownerId}=auth.mutation(req);
    const dir=await project(req.body?.directory),model=req.body?.model;
    if(model&&!(await models(dir)).some(m=>m.providerID===model.providerID&&m.id===model.id))
      throw new AuthProblem(409,'CHAT_MODEL_UNAVAILABLE');
    const session=await (await client()).session.create({location:{directory:dir},...(model?{model}:{}),title:req.body?.scenario==='log-inspection'?'Code Harness · 日志排查':'Code Harness · 直接对话',
      metadata:{harnessScenario:req.body?.scenario??'direct-chat',harnessOwner:ownerId}}).catch((error:unknown)=>{
        if(error instanceof Error&&error.cause instanceof AuthProblem)throw error.cause;
        if(error instanceof Error&&'reason' in error&&error.reason==='Transport'){
          const timedOut=error.cause instanceof Error&&error.cause.name==='TimeoutError';
          throw new AuthProblem(timedOut?504:502,timedOut?'OPENCODE_TIMEOUT':'OPENCODE_CONNECTION_FAILED');
        }
        throw new AuthProblem(502,'OPENCODE_SESSION_RESPONSE');
      });
    if(!session?.id||typeof session.location?.directory!=='string')throw new AuthProblem(502,'OPENCODE_SESSION_RESPONSE');
    if(resolve(session.location.directory)!==dir)throw new AuthProblem(409,'SESSION_DIRECTORY_CHANGED');
    return reply.code(201).send(wrap({id:session.id,directory:dir}));
  });
  const streams=new Set<AbortController>();
  app.addHook('preClose',async()=>{for(const stop of streams)stop.abort();});
  app.get<{Params:Params}>('/api/v1/chat/sessions/:id/activity',async(req,reply)=>{
    const session=auth.session(req),sessionID=req.params.id;
    await owned(sessionID,session.ownerId);
    const native=await client();
    const stop=new AbortController(),raw=reply.raw;
    reply.hijack();
    raw.writeHead(200,{'content-type':'text/event-stream','cache-control':'no-store','x-accel-buffering':'no'});
    raw.write('retry: 5000\n\n');
    streams.add(stop);raw.once('close',()=>stop.abort());
    const expiry=setTimeout(()=>stop.abort(),Math.max(1,session.expires-Date.now()));
    const heartbeat=setInterval(()=>{if(!raw.write(': heartbeat\n\n'))stop.abort();},15000);
    try{
      let ready=false;
      for await(const event of native.event.subscribe({signal:stop.signal})){
        if(stop.signal.aborted)break;
        if(!ready){raw.write('event: ready\ndata: {}\n\n');ready=true;}
        if(!('sessionID' in event.data)||event.data.sessionID!==sessionID)continue;
        if(auth.session(req).csrf!==session.csrf)break;
        let frame:string;
        switch(event.type){
          case 'session.text.started':case 'session.text.delta':case 'session.text.ended':
          case 'session.reasoning.started':case 'session.reasoning.delta':case 'session.reasoning.ended':{
            const data=event.data;
            const fragment:ChatFragment={id:event.id,message:data.assistantMessageID,ordinal:data.ordinal,
              kind:event.type.startsWith('session.text.')?'text':'thinking',at:event.created,
              phase:'delta' in data?'delta':'text' in data?'ended':'started',
              text:'delta' in data?data.delta:'text' in data?data.text:''};
            frame='event: fragment\ndata: '+JSON.stringify(fragment);
            break;
          }
          case 'session.tool.progress':{
            const meta=event.data.metadata;
            const label=[meta.title,meta.message,meta.status].find((v):v is string=>typeof v==='string'&&v.length>0);
            frame='data: '+JSON.stringify({id:event.id,at:event.created,text:label?activityPreview(label,300):'工具报告了新的进度'});
            break;
          }
          default:continue;
        }
        if(!raw.write(frame+'\n\n'))await once(raw,'drain',{signal:stop.signal});
      }
    }catch{/* The message snapshot remains available if native event streaming is unavailable. */}
    finally{streams.delete(stop);stop.abort();clearInterval(heartbeat);clearTimeout(expiry);if(!raw.destroyed)raw.end();}
  });
  app.get<{Params:Params;Querystring:{message?:string;part?:number}}>('/api/v1/chat/sessions/:id',{
    schema:{querystring:{type:'object',additionalProperties:false,properties:{
      message:property(160),part:{type:'integer',minimum:0}}}}
  },async(req,reply)=>{
    reply.header('Cache-Control','no-store');
    const {ownerId}=auth.session(req),sessionID=req.params.id;
    const session=await owned(sessionID,ownerId);
    const [page,permissions,running,inbox]=await Promise.all([(await client()).message.list({sessionID,order:'desc',limit:100}),
      (await client()).permission.list({sessionID}),(await client()).session.active(),(await client()).session.inbox.list({sessionID})]);
    const messages:ChatMessage[]=[];
    for(const m of page.data.slice().reverse()){
      if(m.type==='user')messages.push({id:m.id,role:'user',text:m.text,files:m.files?.map(f=>f.name??(f.source.type==='uri'?f.source.uri:'附件'))??[],tools:[]});
      const steps=activity(m,m.id===req.query.message?req.query.part:undefined);
      if(m.type!=='assistant'&&steps.length)messages.push({id:m.id,role:'assistant',text:'',files:[],tools:[],steps});
      if(m.type==='assistant'&&(steps.length||m.snapshot?.files?.length))messages.push({id:m.id,role:'assistant',
        complete:Boolean(m.time?.completed||m.finish||m.error),steps,text:m.content.flatMap(p=>p.type==='text'?[p.text]:[]).join('\n')||m.error?.message||'',
        tools:m.content.flatMap(p=>p.type==='tool'?[{name:p.name,status:p.state.status}]:[]),
        files:m.snapshot?.files??[]});
    }
    for(const item of inbox)if(item.type==='user'&&!messages.some(m=>m.id===item.id))
      messages.push({id:item.id,role:'user',text:item.payload.text,files:item.payload.files?.map(f=>f.name??(f.source.type==='uri'?f.source.uri:'附件'))??[],tools:[]});
    const result:ChatStatus={id:sessionID,directory:session.location.directory,messages,
      ...(session.model?{model:{id:session.model.id,providerID:session.model.providerID}}:{}),
      active:Boolean(running[sessionID])||inbox.length>0,outcome:session.outcome,
      permissions:permissions.filter(p=>p.sessionID===sessionID).map(p=>({id:p.id,action:p.action,resources:p.resources,revision:revision(p)}))};
    return wrap(result);
  });
  app.post<{Params:Params;Body:{text:string;files?:string[]}}>('/api/v1/chat/sessions/:id/messages',{
    bodyLimit:128*1024,schema:body({text:property(16000),files:{type:'array',maxItems:8,uniqueItems:true,items:property(1024)}},['text'])},async(req,reply)=>{
    const {ownerId}=auth.mutation(req),sessionID=req.params.id;
    const session=await owned(sessionID,ownerId);
    const text=req.body.text.trim();
    if(!text)throw new AuthProblem(400,'EMPTY_MESSAGE');
    const root=await directory(session.location.directory);
    const files=await Promise.all((req.body.files??[]).map(async name=>{
      if(isAbsolute(name))throw new AuthProblem(400,'RELATIVE_FILE_REQUIRED');
      const target=await realpath(resolve(root,name)).catch(()=>null);
      if(!target||!within(root,target)||!(await stat(target)).isFile())throw new AuthProblem(400,'FILE_OUTSIDE_WORKSPACE_OR_MISSING');
      return {uri:pathToFileURL(target).href,name};
    }));
    const accepted=await (await client()).session.prompt({sessionID,text,files,delivery:'queue'});
    return reply.code(202).send(wrap({id:accepted.id}));
  });
  app.post<{Params:Params;Body:{requestId:string;decision:'once'|'reject';revision:string}}>('/api/v1/chat/sessions/:id/permission',{
    schema:body({requestId:property(128),decision:{enum:['once','reject']},revision:property(64)})},async(req)=>{
    const {ownerId}=auth.mutation(req),sessionID=req.params.id;
    await owned(sessionID,ownerId);
    const pending=await (await client()).permission.list({sessionID});
    const request=pending.find(p=>p.sessionID===sessionID&&p.id===req.body.requestId);
    if(!request||revision(request)!==req.body.revision)throw new AuthProblem(409,'PERMISSION_CHANGED');
    await (await client()).permission.reply({sessionID,requestID:request.id,decision:req.body.decision});
    return wrap({submitted:true});
  });
  app.post<{Params:Params}>('/api/v1/chat/sessions/:id/interrupt',async(req)=>{
    const {ownerId}=auth.mutation(req),sessionID=req.params.id;
    await owned(sessionID,ownerId);return wrap(await (await client()).session.interrupt({sessionID}));
  });
}
