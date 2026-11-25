import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {setTimeout as delay} from 'node:timers/promises';
import {AuthProblem,LocalAuth} from '../../apps/server/src/auth.ts';
import {SceneConfigStore} from '../../apps/server/src/scene-config.ts';
import {SceneAccessStore} from '../../apps/server/src/scene-access.ts';
import {ExecutionRegistry} from '../../apps/server/src/execution-registry.ts';
import {InstitutionTaskStore} from '../../apps/server/src/institution-task-store.ts';
import {InstitutionIntegration,transientInstitutionPollError} from '../../apps/server/src/institution-integration.ts';
import type {chatConnection} from '../../apps/server/src/modules/chat/connection.ts';

type FinalState={status:string;errorCode:string|null};
async function execute(mode:'transient'|'native-error'|'prompt-rejected'){
  const root=await mkdtemp(join(tmpdir(),'institution-poll-recovery-'));
  const db=join(root,'tasks.sqlite');
  const sceneConfig=new SceneConfigStore(root);
  await sceneConfig.initialize();
  const config=sceneConfig.read();
  await sceneConfig.save({version:1,scenes:{...config.config.scenes,
    'institution-message':{directory:root,executor:{type:'skill',name:'institution-log-query'}}}},config.revision);
  const access=new SceneAccessStore(root);
  const tasks=new InstitutionTaskStore(db,root);
  let prompts=0,pollErrors=0;
  const native={
    skill:{list:async()=>({location:{directory:root},data:[{id:'skill-id',name:'institution-log-query'}]})},
    model:{list:async()=>({data:[{id:'kimi',name:'Kimi',providerID:'private',enabled:true}]})},
    session:{
      create:async(input:{location:unknown;permissions:unknown})=>({
        id:'native-session-1',location:input.location,permissions:input.permissions,
      }),
      prompt:async()=>{prompts++;if(mode==='prompt-rejected')throw new AuthProblem(502,'OPENCODE_HTTP_400');return {id:'submitted'};},
      active:async()=>({}),
      inbox:{list:async()=>[]},
      interrupt:async()=>({interrupted:true}),
      remove:async()=>({}),
    },
    message:{list:async()=>{
      if(mode==='transient'&&pollErrors++===0)
        throw new AuthProblem(502,'OPENCODE_HTTP_503');
      return {data:[{id:'answer',type:'assistant',finish:mode==='native-error'?'error':'stop',
        ...(mode==='native-error'?{error:{name:'NativeError'}}:{}),
        time:{completed:Date.now()},
        content:[{type:'text',text:mode==='native-error'?'':'实际排查结果'}]}]};
    }},
    permission:{list:async()=>[]},
  };
  const connection=async()=>native as unknown as Awaited<ReturnType<ReturnType<typeof chatConnection>>>;
  const execution=new ExecutionRegistry([],async()=>connection(),{
    workspace:root,singleServiceIdentity:'single:'+'f'.repeat(64),
  });
  const service=new InstitutionIntegration({clientId:'platform-b',databasePath:db},{
    workspace:root,sceneConfig,sceneAccess:access,
    auth:new LocalAuth('unused',4176),execution,nativeConnection:connection,
  },tasks);
  try{
    const created=service.create('user-hash','req-1','2026-11-25','000123','customer-001');
    const start=Date.now();
    let state:FinalState|undefined;
    while(Date.now()-start<9000){
      const row=tasks.listAdmin(undefined,undefined,0).items.find(row=>row.taskId===created.data.taskId);
      if(row&&['SUCCEEDED','FAILED','UNKNOWN'].includes(row.status)){
        state={status:row.status,errorCode:row.errorCode};break;
      }
      await delay(40);
    }
    assert.ok(state,'native task should reach terminal state');
    return {state,prompts,pollErrors};
  }finally{service.close();await rm(root,{recursive:true,force:true});}
}

test('temporary read-only native status API failure is retried, not UNKNOWN and never re-prompts', {timeout:15000},async()=>{
  const value=await execute('transient');
  assert.deepEqual(value.state,{status:'SUCCEEDED',errorCode:null});
  assert.equal(value.prompts,1,'even if a poll request fails, do not resubmit Prompt');
  assert.ok(value.pollErrors>=2,'read-only message lookup should retry');
});

test('a completed native assistant error is definitely FAILED, not UNKNOWN', {timeout:12000},async()=>{
  const value=await execute('native-error');
  assert.deepEqual(value.state,{status:'FAILED',errorCode:'INSTITUTION_NATIVE_EXECUTION_FAILED'});
  assert.equal(value.prompts,1);
});

test('native 4xx Prompt rejection is FAILED; it must never be considered possibly delivered', {timeout:12000},async()=>{
  const value=await execute('prompt-rejected');
  assert.deepEqual(value.state,{status:'FAILED',errorCode:'OPENCODE_HTTP_400'});
  assert.equal(value.prompts,1);
});

test('only network/read transient errors are retryable, not invalid native input',()=>{
  assert.equal(transientInstitutionPollError(new AuthProblem(502,'OPENCODE_HTTP_503')),true);
  assert.equal(transientInstitutionPollError(new Error('fetch failed: ECONNRESET')),true);
  assert.equal(transientInstitutionPollError(new AuthProblem(400,'INVALID_INSTITUTION_DATE')),false);
  assert.equal(transientInstitutionPollError(new Error('programmer bug')),false);
});
