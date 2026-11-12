import {activity,activityPreview} from './activity.ts';
import {chatConnection} from './connection.ts';
import { mkdir,realpath,stat } from 'node:fs/promises';
import {once} from 'node:events';
import { createHash,randomUUID } from 'node:crypto';
import { delimiter,isAbsolute,relative,resolve,sep } from 'node:path';
import { pathToFileURL } from 'node:url';
import type {FormInfo,FormField} from '@opencode/client';
import {activeFormFields,matchesFormCondition,formDefaults,type ChatForm,type ChatFormField,type FormAnswers,type FormValue} from '../../../../../packages/contracts/src/chat.ts';
import type { FastifyInstance } from 'fastify';
import type { ChatMessage,ChatStatus,ChatModel,ChatFragment,ChatHistoryPage,ChatScenario } from '../../../../../packages/contracts/src/chat.ts';
import { AuthProblem,type LocalAuth } from '../../auth.ts';

const wrap=<T>(data:T)=>({data,requestId:randomUUID()});
const within=(root:string,target:string)=>{
  const part=relative(root,target);
  return part!=='..'&&!part.startsWith('..'+sep)&&!isAbsolute(part);
};
const revision=(p:{id:string;action:string;resources:string[];message?:string})=>
  createHash('sha256').update(JSON.stringify([p.id,p.action,p.resources,p.message])).digest('hex');
type Params={id:string};
const property=(maxLength:number)=>({type:'string',minLength:1,maxLength});
const body=(properties:Record<string,unknown>,required=Object.keys(properties))=>({
  body:{type:'object',additionalProperties:false,required,properties},
});

const finite=(value:unknown)=>typeof value==='number'&&Number.isFinite(value)?value:undefined;
function formField(field:FormField):ChatFormField{
  const common={key:field.key,title:field.title,description:field.description};
  if(field.type==='external'){
    let url:URL|undefined;
    try{url=new URL(field.url);}catch{}
    return {...common,type:field.type,url:url&&['http:','https:'].includes(url.protocol)&&!url.username&&!url.password?url.href:undefined};
  }
  const base={...common,required:field.required,hidden:field.hidden,when:field.when?.map(({key,op,value})=>({key,op,value}))};
  switch(field.type){
    case 'boolean':return {...base,type:field.type,default:field.default};
    case 'integer':case 'number':return {...base,type:field.type,default:finite(field.default),minimum:finite(field.minimum),maximum:finite(field.maximum)};
    case 'multiselect':return {...base,type:field.type,default:field.default,custom:field.custom,minItems:field.minItems,maxItems:field.maxItems,
      options:field.options.map(({value,label,description})=>({value,label,description}))};
    case 'string':return {...base,type:field.type,default:field.default,custom:field.custom,minLength:field.minLength,maxLength:field.maxLength,
      pattern:field.pattern,placeholder:field.placeholder,format:field.format,options:field.options?.map(({value,label,description})=>({value,label,description}))};
    default:throw new AuthProblem(502,'FORM_SERVICE_UNAVAILABLE');
  }
}
function nativeForm(native:FormInfo):ChatForm{
  const form={id:native.id,title:native.title,fields:native.fields.map(formField)};
  return {...form,revision:createHash('sha256').update(JSON.stringify(form)).digest('hex')};
}
// Resolve dependencies on server-held defaults before exposing fields to the browser.
function visibleForm(form:ChatForm):ChatForm{
  type Conditions=NonNullable<ChatFormField['when']>;
  const hidden=new Map<string,{defaultValue?:FormValue;when:Conditions}>();
  const excluded=new Set<string>(),fields:ChatFormField[]=[];
  for(const field of form.fields){
    const when:Conditions=[];
    const available=(field.when??[]).every(condition=>{
      if(excluded.has(condition.key))return false;
      const dependency=hidden.get(condition.key);
      if(!dependency){when.push(condition);return true;}
      if(!matchesFormCondition(dependency.defaultValue,condition))return false;
      for(const prerequisite of dependency.when)if(!when.includes(prerequisite))when.push(prerequisite);
      return true;
    });
    if(!available){excluded.add(field.key);continue;}
    if(field.hidden){
      hidden.set(field.key,{defaultValue:'default' in field?field.default:undefined,when});
      continue;
    }
    fields.push({...field,when:when.length?when:undefined});
  }
  return {...form,fields};
}
function validateValue(field:ChatFormField,value:FormValue){
  switch(field.type){
    case 'external':return value===true;
    case 'boolean':return typeof value==='boolean';
    case 'integer':case 'number':return typeof value==='number'&&Number.isFinite(value)&&(field.type!=='integer'||Number.isInteger(value))
      &&value>=(field.minimum??-Infinity)&&value<=(field.maximum??Infinity);
    case 'multiselect':return Array.isArray(value)&&value.length>=Math.max(field.minItems??0,field.required?1:0)&&value.length<=(field.maxItems??Infinity)
      &&value.every(v=>typeof v==='string'&&(field.custom||field.options.some(o=>o.value===v)));
    case 'string':return typeof value==='string'&&value.length>=Math.max(field.minLength??0,field.required?1:0)&&value.length<=(field.maxLength??Infinity)
      &&(!field.options?.length||field.custom||field.options.some(o=>o.value===value));
  }
}
function checkedAnswers(form:ChatForm,submitted:FormAnswers|undefined){
  if(!submitted)throw new AuthProblem(400,'FORM_ANSWER_INVALID');
  const defaults=formDefaults({...form,fields:form.fields.filter(f=>f.hidden)}),answers={...defaults,...submitted};
  const active=activeFormFields(form,answers);
  const editable=new Set(active.filter(f=>!f.hidden).map(f=>f.key));
  if(Object.keys(submitted).some(key=>!editable.has(key)))throw new AuthProblem(400,'FORM_ANSWER_INVALID');
  const result:FormAnswers=Object.create(null);
  for(const field of active){
    const value=Object.hasOwn(answers,field.key)?answers[field.key]:undefined;
    if(value===undefined){
      if(field.required||field.type==='external')throw new AuthProblem(400,'FORM_ANSWER_INVALID');
    }else{
      if(!validateValue(field,value))throw new AuthProblem(400,'FORM_ANSWER_INVALID');
      result[field.key]=value;
    }
  }
  return result;
}
function upstreamProblem(error:unknown){
  return error instanceof Error&&error.cause instanceof AuthProblem?error.cause:error;
}

export async function registerChat(app:FastifyInstance,auth:LocalAuth,options:{mayStart?:boolean;localOnly:boolean;workspace?:string}){
  const client=chatConnection(options.mayStart??false);
  const configured=(process.env.HARNESS_CHAT_ROOTS??process.env.HARNESS_REPOSITORY_ROOTS??'').split(delimiter).filter(Boolean);
  const defaultDirectory=resolve('chat-files');
  let workspace:string|undefined;
  if(options.localOnly){
    const selected=options.workspace??configured[0]??defaultDirectory;
    if(options.workspace===undefined&&!configured.length)await mkdir(defaultDirectory,{recursive:true});
    workspace=await realpath(selected).catch(()=>undefined);
    if(!workspace||!(await stat(workspace)).isDirectory())
      throw new Error('Workspace directory does not exist or is not a directory: '+selected);
  }
  const roots=workspace?[workspace]:configured.length?configured:[defaultDirectory];
  async function directory(input:string){
    if(!isAbsolute(input))throw new AuthProblem(400,'ABSOLUTE_DIRECTORY_REQUIRED');
    const path=await realpath(input).catch(()=>null);
    if(!path||!(await stat(path)).isDirectory())throw new AuthProblem(400,'DIRECTORY_NOT_FOUND');
    if(workspace){
      if(relative(workspace,path)!=='')throw new AuthProblem(403,'DIRECTORY_OUTSIDE_CHAT_ROOTS');
      return workspace;
    }
    if(!configured.length&&options.mayStart)return path;
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
  async function pendingForms(sessionID:string){
    try{
      const native=await (await client()).session.form.list({sessionID});
      return native.filter(form=>form.sessionID===sessionID).map(nativeForm);
    }catch(error){
      const problem=upstreamProblem(error);
      if(problem instanceof AuthProblem&&['OPENCODE_HTTP_404','OPENCODE_HTTP_405','OPENCODE_HTTP_501'].includes(problem.code))return [];
      if(problem instanceof AuthProblem)throw problem;
      throw new AuthProblem(502,'FORM_SERVICE_UNAVAILABLE');
    }
  }
  app.get('/api/v1/chat/settings',async(req)=>{
    auth.session(req);return wrap({configured:true,directories:roots});
  });
  async function project(input?:string){
    if(!workspace&&!configured.length&&(!input||input===roots[0]))await mkdir(roots[0],{recursive:true});
    return directory(input??roots[0]);
  }
  async function models(dir:string):Promise<ChatModel[]>{
    const result=await (await client()).model.list({location:{directory:dir}});
    return result.data.filter(m=>m.enabled).map(({id,providerID,name,variants,limit})=>({id,providerID,name,
      ...(variants?.length?{variants:variants.map(v=>v.id)}:{}),...(limit?.context?{context:limit.context}:{})}));
  }
  async function logQuerySkill(dir:string){
    const native=await client();
    const catalog=await native.skill.list({location:{directory:dir}}).catch(()=>{
      throw new AuthProblem(502,'LOG_QUERY_CATALOG_UNAVAILABLE');
    });
    if(!Array.isArray(catalog?.data)||typeof catalog.location?.directory!=='string'||
      relative(dir,catalog.location.directory)!=='')throw new AuthProblem(502,'LOG_QUERY_CATALOG_UNAVAILABLE');
    const matches=catalog.data.filter(skill=>skill.name==='log-query');
    if(!matches.length)throw new AuthProblem(409,'LOG_QUERY_SKILL_MISSING');
    if(matches.length>1)throw new AuthProblem(409,'LOG_QUERY_SKILL_AMBIGUOUS');
    if(!matches[0].id)throw new AuthProblem(502,'LOG_QUERY_CATALOG_UNAVAILABLE');
    return {id:matches[0].id};
  }
  type ProjectInput={Body:{directory?:string}};
  if(!workspace)app.post<ProjectInput>('/api/v1/chat/project',{schema:body({directory:property(4096)})},async req=>{
    auth.mutation(req);return wrap({directory:await project(req.body.directory)});
  });
  app.post<{Body:{directory?:string;scenario:ChatScenario;cursor?:string}}>('/api/v1/chat/history',{schema:body({
    directory:property(4096),scenario:{enum:['log-inspection','direct-chat']},cursor:property(4096),
  },['scenario'])},async(req,reply)=>{
    const {ownerId}=auth.mutation(req);
    const {directory:input,scenario,cursor}=req.body;
    const dir=await project(input);
    const page=await (await client()).session.list({directory:dir,order:'desc',limit:50,cursor});
    const items:ChatHistoryPage['items']=[];
    for(const session of page.data){
      if(session.parentID||session.metadata?.harnessOwner!==ownerId||session.metadata.harnessScenario!==scenario)continue;
      const location=session.location?.directory;
      if(!location||!isAbsolute(location)||relative(dir,location)!==''||!session.id)continue;
      items.push({id:session.id,title:session.title??'',updated:session.time.updated});
    }
    return reply.header('Cache-Control','no-store').send(wrap({items,next:page.cursor?.next??undefined}));
  });
  app.post<ProjectInput>('/api/v1/chat/models',async req=>{
    auth.mutation(req);
    const input=req.body?.directory;
    if(input!==undefined&&(typeof input!=='string'||input.length>4096))throw new AuthProblem(400,'INVALID_DIRECTORY');
    return wrap(await models(await project(input)));
  });
  type Selection=Pick<ChatModel,'providerID'|'id'|'variant'>;
  const modelFields={providerID:property(256),id:property(512),variant:property(256)};
  async function validateModel(dir:string,model:Selection){
    const match=(await models(dir)).find(m=>m.providerID===model.providerID&&m.id===model.id);
    if(!match||(model.variant!==undefined&&!match.variants?.includes(model.variant)))
      throw new AuthProblem(409,'CHAT_MODEL_UNAVAILABLE');
  }
  app.post<{Body:{directory?:string;model?:Selection;title?:string;scenario?:'log-inspection'|'direct-chat'}}>('/api/v1/chat/sessions',{schema:body({
    directory:property(4096),title:property(120),scenario:{enum:['log-inspection','direct-chat']},model:{type:'object',additionalProperties:false,required:['providerID','id'],
      properties:modelFields}},[])},async(req,reply)=>{
    const {ownerId}=auth.mutation(req);
    const dir=await project(req.body?.directory),model=req.body?.model;
    if(model)await validateModel(dir,model);
    if(req.body?.scenario==='log-inspection')await logQuerySkill(dir);
    const title=req.body?.title?.trim()||(req.body?.scenario==='log-inspection'?'Code Harness · 日志排查':'Code Harness · 直接对话');
    const session=await (await client()).session.create({location:{directory:dir},...(model?{model}:{}),title,
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
    return reply.code(201).send(wrap({id:session.id,directory:dir,title:session.title||title}));
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
        const eventSession=event.type==='form.created'?event.data.form.sessionID:
          'sessionID' in event.data?event.data.sessionID:undefined;
        if(eventSession!==sessionID)continue;
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
          case 'form.created':case 'form.cancelled':case 'form.replied':
          case 'permission.asked':case 'permission.replied':
            frame='event: requests\ndata: {}';
            break;
          case 'session.tool.progress':{
            const meta=event.data.metadata;
            const label=[meta.title,meta.message,meta.status].find((v):v is string=>typeof v==='string'&&v.length>0);
            if(!label||!event.data.assistantMessageID)continue;
            frame='data: '+JSON.stringify({id:event.id,message:event.data.assistantMessageID,at:event.created,text:activityPreview(label,300)});
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
    const [page,permissions,running,inbox,forms]=await Promise.all([(await client()).message.list({sessionID,order:'desc',limit:100}),
      (await client()).permission.list({sessionID}),(await client()).session.active(),(await client()).session.inbox.list({sessionID}),pendingForms(sessionID)]);
    const messages:ChatMessage[]=[];
    for(const m of page.data.slice().reverse()){
      if(m.type==='user')messages.push({id:m.id,role:'user',text:m.text,files:m.files?.map(f=>f.name??(f.source.type==='uri'?f.source.uri:'附件'))??[],tools:[]});
      const steps=activity(m,m.id===req.query.message?req.query.part:undefined);
      if(m.type!=='assistant'&&steps.length)messages.push({id:m.id,role:'assistant',text:'',files:[],tools:[],steps});
      if(m.type==='assistant'&&(steps.length||m.snapshot?.files?.length))messages.push({id:m.id,role:'assistant',
        toolCallTurn:m.finish==='tool-calls',complete:Boolean(m.time?.completed||m.finish||m.error),steps,text:m.content.flatMap(p=>p.type==='text'?[p.text]:[]).join('\n')||m.error?.message||'',
        tools:m.content.flatMap(p=>p.type==='tool'?[{name:p.name,status:p.state.status}]:[]),
        files:m.snapshot?.files??[]});
    }
    for(const item of inbox)if(item.type==='user'&&!messages.some(m=>m.id===item.id))
      messages.push({id:item.id,role:'user',queued:true,text:item.payload.text,files:item.payload.files?.map(f=>f.name??(f.source.type==='uri'?f.source.uri:'附件'))??[],tools:[]});
    const result:ChatStatus={id:sessionID,title:session.title,forms:forms.map(visibleForm),directory:session.location.directory,messages,
      ...(session.model?{model:{id:session.model.id,providerID:session.model.providerID,variant:session.model.variant}}:{}),
      active:Boolean(running[sessionID])||inbox.length>0,outcome:session.outcome,
      permissions:permissions.filter(p=>p.sessionID===sessionID).map(p=>({id:p.id,action:p.action,resources:p.resources,message:p.message,revision:revision(p)}))};
    return wrap(result);
  });
  app.post<{Params:Params;Body:Selection}>('/api/v1/chat/sessions/:id/model',{
    schema:body(modelFields,['providerID','id'])},async(req)=>{
    const {ownerId}=auth.mutation(req),sessionID=req.params.id;
    const session=await owned(sessionID,ownerId),model=req.body;
    await validateModel(session.location.directory,model);
    await (await client()).session.switchModel({sessionID,model});
    return wrap({model});
  });
  app.post<{Params:Params;Body:{text:string;files?:string[];scenario?:'log-inspection'|'direct-chat'}}>('/api/v1/chat/sessions/:id/messages',{
    bodyLimit:128*1024,schema:body({text:property(16000),scenario:{enum:['direct-chat','log-inspection']},files:{type:'array',maxItems:8,uniqueItems:true,items:property(1024)}},['text'])},async(req,reply)=>{
    const {ownerId}=auth.mutation(req),sessionID=req.params.id;
    const session=await owned(sessionID,ownerId);
    const scenario=session.metadata?.harnessScenario;
    if(req.body.scenario&&req.body.scenario!==scenario)throw new AuthProblem(409,'CHAT_SCENARIO_MISMATCH');
    const text=req.body.text.trim();
    if(!text)throw new AuthProblem(400,'EMPTY_MESSAGE');
    const root=await directory(session.location.directory);
    const files=await Promise.all((req.body.files??[]).map(async name=>{
      if(isAbsolute(name))throw new AuthProblem(400,'RELATIVE_FILE_REQUIRED');
      const target=await realpath(resolve(root,name)).catch(()=>null);
      if(!target||!within(root,target)||!(await stat(target)).isFile())throw new AuthProblem(400,'FILE_OUTSIDE_WORKSPACE_OR_MISSING');
      return {uri:pathToFileURL(target).href,name};
    }));
    const skills=scenario==='log-inspection'?[await logQuerySkill(root)]:undefined;
    const accepted=await (await client()).session.prompt({sessionID,text,files,skills,delivery:'queue'});
    return reply.code(202).send(wrap({id:accepted.id}));
  });
  app.post<{Params:Params;Body:{requestId:string;revision:string;decision:'cancel'|'reply';answer?:FormAnswers}}>('/api/v1/chat/sessions/:id/form',{
    schema:body({requestId:property(128),revision:property(64),decision:{enum:['cancel','reply']},answer:{type:'object',additionalProperties:true}},['requestId','revision','decision'])
  },async req=>{
    const {ownerId}=auth.mutation(req),sessionID=req.params.id;
    await owned(sessionID,ownerId);
    const form=(await pendingForms(sessionID)).find(f=>f.id===req.body.requestId);
    if(!form||form.revision!==req.body.revision)throw new AuthProblem(409,'FORM_CHANGED');
    const api=(await client()).session.form,identity={sessionID,formID:form.id};
    try{
      if(req.body.decision==='cancel')await api.cancel(identity);
      else await api.reply({...identity,answer:checkedAnswers(form,req.body.answer)});
    }catch(error){
      const problem=upstreamProblem(error);
      if(problem instanceof AuthProblem){
        if(['OPENCODE_HTTP_404','OPENCODE_HTTP_409'].includes(problem.code))throw new AuthProblem(409,'FORM_CHANGED');
        if(problem.code==='OPENCODE_HTTP_400')throw new AuthProblem(400,'FORM_ANSWER_INVALID');
        throw problem;
      }
      throw new AuthProblem(502,'FORM_SERVICE_UNAVAILABLE');
    }
    return wrap({submitted:true});
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
