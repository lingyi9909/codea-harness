import { OpenCode } from '@opencode/client';
import { realpath,stat } from 'node:fs/promises';
import { createHash,randomUUID } from 'node:crypto';
import { delimiter,isAbsolute,relative,resolve,sep } from 'node:path';
import { pathToFileURL } from 'node:url';
import type { FastifyInstance } from 'fastify';
import type { ChatMessage,ChatStatus } from '../../../../../packages/contracts/src/chat.ts';
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

export function registerChat(app:FastifyInstance,auth:LocalAuth){
  const endpoint=process.env.HARNESS_OPENCODE_URL;
  const roots=(process.env.HARNESS_CHAT_ROOTS??process.env.HARNESS_REPOSITORY_ROOTS??'').split(delimiter).filter(Boolean);
  const token=process.env.HARNESS_OPENCODE_AUTHORIZATION;
  if(endpoint){
    const url=new URL(endpoint);
    if(url.username||url.password||url.search||url.hash||
      !(url.protocol==='https:'||(url.protocol==='http:'&&['localhost','127.0.0.1','[::1]'].includes(url.hostname))))
      throw new Error('INVALID_OPENCODE_URL');
  }
  const native=endpoint?OpenCode.make({baseUrl:endpoint,headers:token?{authorization:token}:{},
    fetch:async(input,init)=>{
      const response=await fetch(input,{...init,redirect:'manual',signal:AbortSignal.timeout(25000)});
      if(response.ok)return response;
      await response.body?.cancel();
      throw new AuthProblem(502,'OPENCODE_HTTP_'+response.status);
    }}):null;
  const client=()=>{if(!native)throw new AuthProblem(503,'OPENCODE_CHAT_NOT_CONFIGURED');return native;};
  async function directory(input:string){
    if(!isAbsolute(input))throw new AuthProblem(400,'ABSOLUTE_DIRECTORY_REQUIRED');
    const path=await realpath(input).catch(()=>null);
    if(!path||!(await stat(path)).isDirectory())throw new AuthProblem(400,'DIRECTORY_NOT_FOUND');
    for(const root of roots){
      const canonical=await realpath(root).catch(()=>null);
      if(canonical&&within(canonical,path))return path;
    }
    throw new AuthProblem(403,'DIRECTORY_OUTSIDE_CHAT_ROOTS');
  }
  async function owned(id:string,owner:string){
    if(!/^[\w-]{1,128}$/.test(id))throw new AuthProblem(400,'INVALID_SESSION');
    const session=await client().session.get({sessionID:id});
    if(session.id!==id||session.metadata?.harnessScenario!=='direct-chat'||session.metadata.harnessOwner!==owner)
      throw new AuthProblem(404,'CHAT_NOT_FOUND');
    await directory(session.location.directory);
    return session;
  }
  app.get('/api/v1/chat/settings',async(req)=>{
    auth.session(req);return wrap({configured:Boolean(native&&roots.length),directories:roots});
  });
  app.post<{Body:{directory:string}}>('/api/v1/chat/sessions',{schema:body({directory:property(4096)})},async(req,reply)=>{
    const {ownerId}=auth.mutation(req);
    const dir=await directory(req.body.directory);
    if(endpoint){
      const target=new URL(endpoint),host=new URL('http://'+req.headers.host);
      if(target.protocol==='http:'&&['localhost','127.0.0.1','[::1]'].includes(target.hostname)&&target.port===host.port)
        throw new AuthProblem(400,'OPENCODE_WORKBENCH_URL');
    }
    const session=await client().session.create({location:{directory:dir},title:'Code Harness · 直接对话',
      metadata:{harnessScenario:'direct-chat',harnessOwner:ownerId}}).catch((error:unknown)=>{
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
  app.get<{Params:Params}>('/api/v1/chat/sessions/:id',async(req)=>{
    const {ownerId}=auth.session(req),sessionID=req.params.id;
    const session=await owned(sessionID,ownerId);
    const [page,permissions,running,inbox]=await Promise.all([client().message.list({sessionID,order:'desc',limit:100}),
      client().permission.list({sessionID}),client().session.active(),client().session.inbox.list({sessionID})]);
    const messages:ChatMessage[]=[];
    for(const m of page.data.slice().reverse()){
      if(m.type==='user')messages.push({id:m.id,role:'user',text:m.text,files:m.files?.map(f=>f.name??(f.source.type==='uri'?f.source.uri:'附件'))??[],tools:[]});
      if(m.type==='assistant')messages.push({id:m.id,role:'assistant',
        text:m.content.flatMap(p=>p.type==='text'?[p.text]:[]).join('\n')||m.error?.message||'',
        tools:m.content.flatMap(p=>p.type==='tool'?[{name:p.name,status:p.state.status}]:[]),
        files:m.snapshot?.files??[]});
    }
    for(const item of inbox)if(item.type==='user'&&!messages.some(m=>m.id===item.id))
      messages.push({id:item.id,role:'user',text:item.payload.text,files:item.payload.files?.map(f=>f.name??(f.source.type==='uri'?f.source.uri:'附件'))??[],tools:[]});
    const result:ChatStatus={id:sessionID,directory:session.location.directory,messages,
      active:Boolean(running[sessionID])||inbox.length>0,outcome:session.outcome,
      permissions:permissions.filter(p=>p.sessionID===sessionID).map(p=>({id:p.id,action:p.action,resources:p.resources,revision:revision(p)}))};
    return wrap(result);
  });
  app.post<{Params:Params;Body:{text:string;files?:string[]}}>('/api/v1/chat/sessions/:id/messages',{
    schema:body({text:property(16000),files:{type:'array',maxItems:8,uniqueItems:true,items:property(1024)}},['text'])},async(req,reply)=>{
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
    const accepted=await client().session.prompt({sessionID,text,files,delivery:'queue'});
    return reply.code(202).send(wrap({id:accepted.id}));
  });
  app.post<{Params:Params;Body:{requestId:string;decision:'once'|'reject';revision:string}}>('/api/v1/chat/sessions/:id/permission',{
    schema:body({requestId:property(128),decision:{enum:['once','reject']},revision:property(64)})},async(req)=>{
    const {ownerId}=auth.mutation(req),sessionID=req.params.id;
    await owned(sessionID,ownerId);
    const pending=await client().permission.list({sessionID});
    const request=pending.find(p=>p.sessionID===sessionID&&p.id===req.body.requestId);
    if(!request||revision(request)!==req.body.revision)throw new AuthProblem(409,'PERMISSION_CHANGED');
    await client().permission.reply({sessionID,requestID:request.id,decision:req.body.decision});
    return wrap({submitted:true});
  });
  app.post<{Params:Params}>('/api/v1/chat/sessions/:id/interrupt',async(req)=>{
    const {ownerId}=auth.mutation(req),sessionID=req.params.id;
    await owned(sessionID,ownerId);return wrap(await client().session.interrupt({sessionID}));
  });
}
