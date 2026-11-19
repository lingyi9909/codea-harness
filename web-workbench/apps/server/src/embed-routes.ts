import {execFile} from 'node:child_process';
import {promisify} from 'node:util';
import type {InstitutionQueryAdapter} from './institution-query.ts';
import {institutionEvidence} from './institution-query.ts';
import {randomUUID} from 'node:crypto';
import type {FastifyInstance} from 'fastify';
import type {Pool} from 'mysql2/promise';
import {AuthProblem} from './auth.ts';
import {EmbeddedAuth} from './embed-auth.ts';
import type {ExecutionRegistry} from './execution-registry.ts';
import type {SceneConfigStore} from './scene-config.ts';
import {registerChat} from './modules/chat/routes.ts';
import {MysqlChatBindings} from '../../../packages/storage/src/chat-bindings.ts';
import type {ChatBindingStore} from '../../../packages/storage/src/chat-bindings.ts';
const wrap=<T>(data:T)=>({data,requestId:randomUUID()});
export async function registerEmbedded(app:FastifyInstance,opts:{auth:EmbeddedAuth;execution:ExecutionRegistry;workspace:string;sceneConfig:SceneConfigStore;pool?:Pool;bindings?:ChatBindingStore;ensureOwner?:(id:string,subject:string)=>Promise<void>;institutionAdapter?:InstitutionQueryAdapter;resetSlot?:(owner:string)=>Promise<void>}){
  const {auth,execution,workspace,sceneConfig,pool}=opts;
  if(!opts.bindings&&!pool)throw new Error('EMBED_BINDING_STORE_REQUIRED');
  async function cleanup(g:{id:string;ownerId:string;nativeId?:string;serviceIdentity?:string;accessMode?:string}){
    return auth.store.serialize(g.ownerId,async()=>{
    await auth.store.revoke(g.id);
    // Creation may have completed since logout/sweep took its snapshot. Read the
    // persisted binding again under the same lock as native Session creation.
    const current=(await auth.store.cleanupCandidates()).find(candidate=>candidate.id===g.id);
    if(!current)return;g=current;
    if(g.nativeId)await execution.run(g.ownerId,async()=>{
      if(g.serviceIdentity!==execution.serviceIdentity())throw new Error('EMBED_CLEANUP_SERVICE_CHANGED');
      const client=await execution.client();
      const native=await client.session.get({sessionID:g.nativeId!}).catch((error:unknown)=>{
        const cause=error instanceof Error&&error.cause instanceof AuthProblem?error.cause:error;
        if(cause instanceof AuthProblem&&cause.code==='OPENCODE_HTTP_404')return undefined;throw error;
      });
      if(!native)return;
      if(native.metadata?.harnessOwner!==g.ownerId||native.metadata?.harnessEmbedGrant!==g.id)throw new Error('EMBED_CLEANUP_OWNER_MISMATCH');
      await client.session.interrupt({sessionID:g.nativeId!});
      const active=await client.session.active();
      if(active[g.nativeId!])throw new Error('EMBED_CLEANUP_STILL_ACTIVE');
      await client.session.remove({sessionID:g.nativeId!});
    });
    await auth.store.cleaned(g.id);
    if(g.accessMode==='ANONYMOUS'&&execution.singleService()){await auth.store.recycled(g.id);return;}
    if(g.accessMode==='ANONYMOUS'){
      // Never lend an unreset execution environment to another anonymous visitor.
      if(opts.resetSlot)await opts.resetSlot(g.ownerId);
      else if(await execution.run(g.ownerId,async()=>execution.sharedWorkspace())){
        const command=auth.config.anonymous?.resetPrivateCommand;
        if(!command)return; // Never recycle an unreset private native profile.
        await execution.run(g.ownerId,async()=>{
          if(g.serviceIdentity!==execution.serviceIdentity())throw new Error('EMBED_RESET_SERVICE_CHANGED');
          const [file,...args]=command;
          await promisify(execFile)(file,[...args,'--owner',g.ownerId,'--service',g.serviceIdentity!],{timeout:60000,maxBuffer:4096,windowsHide:true});
        });
      }
      else if(auth.config.anonymous?.resetCommand){
        await execution.run(g.ownerId,async()=>{
          if(g.serviceIdentity!==execution.serviceIdentity())throw new Error('EMBED_RESET_SERVICE_CHANGED');
          const [command,...args]=auth.config.anonymous!.resetCommand!;
          await promisify(execFile)(command,[...args,'--owner',g.ownerId,'--directory',execution.directory((await binding()).directory),'--service',g.serviceIdentity!],{timeout:60000,maxBuffer:4096,windowsHide:true});
        });
      }else return; // Retire the slot until an operator reprovisions its generation.
      await auth.store.recycled(g.id);
    }
    });
  }
  let sweeping:Promise<void>|undefined;
  const sweep=()=>{
    sweeping??=(async()=>{
      try{for(const grant of await auth.store.cleanupCandidates())await cleanup(grant).catch(()=>{});}catch{/* Retry safely after storage/service recovery. */}
    })().finally(()=>{sweeping=undefined;});
    return sweeping;
  };
  const timer=setInterval(()=>void sweep(),15000);timer.unref();
  app.addHook('onClose',async()=>{clearInterval(timer);await sweeping;});
  for(const c of auth.config.clients)for(const owner of Object.values(c.principals))if(!execution.has(owner))throw new Error('EMBED_EXECUTION_NOT_PROVISIONED');
  for(const owner of auth.config.anonymous?.executionOwners??[]){
    if(!execution.has(owner))throw new Error('EMBED_EXECUTION_NOT_PROVISIONED');
    await execution.run(owner,async()=>{
      if(execution.sharedWorkspace()){
        if(auth.config.anonymous?.resetCommand)throw new Error('SHARED_WORKSPACE_RESET_COMMAND_FORBIDDEN');
        if((await sceneConfig.binding('message')).directory!==workspace)throw new Error('SHARED_WORKSPACE_SCENE_MUST_MATCH_STARTUP');
      }
    });
  }
  app.post('/api/v1/embed/anonymous',async(req,reply)=>{
    const slots=await Promise.all((auth.config.anonymous?.executionOwners??[]).map(ownerId=>execution.run(ownerId,async()=>({ownerId,serviceIdentity:execution.serviceIdentity()}))));
    return reply.header('Cache-Control','no-store').send(wrap(await auth.anonymous(req,slots)));
  });
  app.post('/api/v1/embed/institution-query',async(req,reply)=>{
    await auth.mutation(req);const grant=await auth.grant(req);
    if(grant.accessMode!=='ANONYMOUS')throw new AuthProblem(403,'EMBED_ACTION_FORBIDDEN');
    return reply.header('Cache-Control','no-store').send(wrap(await institutionEvidence(opts.institutionAdapter,auth.store,grant.id,req.body)));
  });
  const binding=async(institution=false)=>{
    const value=await sceneConfig.binding(institution?'institution-message':'message');
    // Directory comes from the administrator-maintained scene configuration,
    // not from the embedding browser. Reuse existing message Skill location
    // without a duplicate per-page directory setting.
    if(execution.singleService())execution.trustSceneDirectory(value.directory);
    if(!institution&&(value.executor.type==='builtin'||value.executor.type==='command'&&value.executor.name==='models'))throw new AuthProblem(503,'EMBED_EXECUTOR_NOT_CONFIGURED');
    return value;
  };
  app.get<{Querystring:{parentOrigin?:string}}>('/api/v1/embed/config',async(req,reply)=>{
    if(!auth.origin(req))throw new AuthProblem(403,'ORIGIN_INVALID');
    const parentOrigin=req.query.parentOrigin;
    if(!auth.config.clients.some(c=>c.parentOrigin===parentOrigin))throw new AuthProblem(403,'EMBED_PARENT_FORBIDDEN');
    return reply.header('Cache-Control','no-store').send(wrap({parentOrigin,version:1}));
  });
  app.post('/api/v1/embed/tickets',async(req,reply)=>reply.header('Cache-Control','no-store').send(wrap(await auth.issue(req,req.body,async(id,subject)=>{
    // Operator-provisioned owner mapping; never trust a browser-supplied owner ID.
    if(opts.ensureOwner)return opts.ensureOwner(id,'embed:'+subject);
    if(!pool)throw new Error('EMBED_OWNER_STORE_REQUIRED');
    await pool.execute("INSERT INTO wh_owners(id,subject,display_name,created_at,updated_at) VALUES (?,?,'Embedded user',UTC_TIMESTAMP(3),UTC_TIMESTAMP(3)) ON DUPLICATE KEY UPDATE id=id",[id,'embed:'+subject]);
  }))));
  app.post('/api/v1/embed/exchange',async(req,reply)=>reply.header('Cache-Control','no-store').send(wrap(await auth.exchange(req,req.body))));
  app.get('/api/v1/embed/session',async req=>{const g=await auth.grant(req);return wrap({sessionId:g.nativeId??null,state:g.state,expiresAt:g.expires});});
  app.delete('/api/v1/embed/session',async(req,reply)=>{
    const g=await auth.grant(req);await auth.logout(req,reply);
    // Revocation takes effect before native cleanup; other sessions remain active.
    await cleanup(g).catch(()=>{}); // Persisted revocation keeps failed cleanup in the retry queue.
    return reply.status(204).send();
  });
  await app.register(async embedded=>{
    embedded.addHook('onRoute',route=>{
      if(!String(route.url).startsWith('/api/v1/embed/chat/'))return;
      const handler=route.handler;
      route.handler=async function(req,reply){
        const grant=await auth.authorize(req);
        return execution.run(grant.ownerId,async()=>{
          const institution=grant.accessMode==='ANONYMOUS',value=await binding(institution);
          if(req.method==='POST'&&['/api/v1/embed/chat/models','/api/v1/embed/chat/sessions'].includes(req.url.split('?')[0])){
            const input=(req.body??{}) as {directory?:string;scenario?:string};
            if(input.directory!==undefined&&input.directory!==value.directory&&input.directory!==execution.directory(value.directory))throw new AuthProblem(403,'EMBED_DIRECTORY_FORBIDDEN');
            if(req.url.split('?')[0].endsWith('/sessions')&&input.scenario!=='message-inspection'&&!(institution&&input.scenario==='institution-message-inspection'))throw new AuthProblem(403,'EMBED_SCENARIO_FORBIDDEN');
            input.directory=value.directory;req.body=input;
          }
          // Older institution clients used the generic message scenario. Normalize
          // the alias server-side so the stored binding is always institution-specific.
          if(institution&&req.body&&typeof req.body==='object'&&'scenario' in req.body&&req.body.scenario==='message-inspection')req.body.scenario='institution-message-inspection';
          try{
            if(grant.accessMode==='ANONYMOUS'&&req.method==='POST'&&req.url.split('?')[0]==='/api/v1/embed/chat/sessions')return await auth.store.serialize(grant.ownerId,async()=>handler.call(this,req,reply));
            if(req.method==='POST'&&req.url.split('?')[0].endsWith('/messages')){
              try{return await auth.store.serialize(grant.ownerId,async()=>{
                const client=await execution.client(),active=await client.session.active();
                if(execution.singleService()?Boolean(active[grant.nativeId!]):Object.values(active).some(Boolean))throw new AuthProblem(409,'EMBED_EXECUTOR_BUSY');
                const ids=execution.singleService()?[grant.nativeId!]:[grant.nativeId!,...await auth.store.otherSessions(grant.ownerId,grant.id)];
                for(const id of ids)if((await client.session.inbox.list({sessionID:id})).length)throw new AuthProblem(409,'EMBED_EXECUTOR_BUSY');
                return handler.call(this,req,reply);
              });}catch(error){if(error instanceof Error&&error.message==='EMBED_EXECUTOR_BUSY')throw new AuthProblem(409,'EMBED_EXECUTOR_BUSY');throw error;}
            }
            return await handler.call(this,req,reply);
          }finally{await auth.finish(req);}
        });
      };
    });
    embedded.get('/api/v1/embed/scene-config',async req=>{
      const grant=await auth.grant(req);
      return execution.run(grant.ownerId,async()=>{
        const institution=grant.accessMode==='ANONYMOUS',value=await binding(institution);
        return wrap({workspace:value.directory,configPath:'',accessPath:'',username:null,canEdit:false,revision:'embedded',config:{version:1,scenes:{[institution?'institution-message':'message']:value}}});
      });
    });
    await registerChat(embedded,auth,{sessionMetadata:req=>auth.metadata(req),blockSensitiveForms:true,prefix:'/api/v1/embed/chat',workspace,sceneConfig,execution,multiUser:true,bindings:opts.bindings??new MysqlChatBindings(pool!),onSessionCreated:(req,id)=>auth.bind(req,id,execution.serviceIdentity())});
  });
}
