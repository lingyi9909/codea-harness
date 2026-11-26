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
import {makeSafeFailure,encodeSafeFailure,decodeSafeFailure} from '../../apps/server/src/institution-failure-diagnostics.ts';
import type {chatConnection} from '../../apps/server/src/modules/chat/connection.ts';

type FinalState={status:string;errorCode:string|null};
async function execute(mode:'transient'|'native-error'|'prompt-rejected'|'stream-failed'){
  const root=await mkdtemp(join(tmpdir(),'institution-poll-recovery-'));
  const db=join(root,'tasks.sqlite');
  const workspace=join(root,'project');
  const {mkdir}=await import('node:fs/promises');await mkdir(workspace);
  const sceneConfig=new SceneConfigStore(workspace);
  await sceneConfig.initialize();
  const config=sceneConfig.read();
  await sceneConfig.save({version:1,scenes:{...config.config.scenes,
    'institution-message':{directory:workspace,executor:{type:'skill',name:'institution-log-query'}}}},config.revision);
  const access=new SceneAccessStore(workspace);
  const tasks=new InstitutionTaskStore(db,workspace);
  let prompts=0,pollErrors=0,activeReads=0;
  const native={
    ...(mode==='stream-failed'?{event:{subscribe:async function*(){throw new Error('STREAM_UNAVAILABLE');}}}:{}),
    skill:{list:async()=>({location:{directory:workspace},data:[{id:'skill-id',name:'institution-log-query'}]})},
    model:{list:async()=>({data:[{id:'kimi',name:'Kimi',providerID:'private',enabled:true}]})},
    session:{
      create:async(input:{location:unknown;permissions:unknown})=>({
        id:'native-session-1',location:input.location,permissions:input.permissions,
      }),
      prompt:async()=>{prompts++;if(mode==='prompt-rejected')throw new AuthProblem(502,'OPENCODE_HTTP_400');return {id:'submitted'};},
      active:async()=>mode==='stream-failed'&&activeReads++===0?{'native-session-1':true}:{} ,
      inbox:{list:async()=>[]},
      interrupt:async()=>({interrupted:true}),
      remove:async()=>({}),
    },
    message:{list:async()=>{
      if(mode==='transient'&&pollErrors++===0)
        throw new AuthProblem(502,'OPENCODE_HTTP_503');
      return {data:[{id:'answer',type:'assistant',
        ...(mode==='native-error'?{error:{name:'NativeError'}}:{finish:'stop'}),
        time:{completed:Date.now()},
        content:[
          ...(mode==='stream-failed'?[{id:'snapshot-part-1',type:'tool',tool:'dbhub',
            state:{status:'error',error:{code:'ECONNREFUSED',message:'connect ECONNREFUSED 10.1.2.3'}}}]:[]),
          {type:'text',text:mode==='native-error'?'':'实际排查结果'}]}]};
    }},
    permission:{list:async()=>[]},
  };
  const connection=async()=>native as unknown as Awaited<ReturnType<ReturnType<typeof chatConnection>>>;
  const execution=new ExecutionRegistry([],async()=>connection(),{
    workspace,singleServiceIdentity:'single:'+'f'.repeat(64),
  });
  const service=new InstitutionIntegration({clientId:'platform-b',databasePath:db},{
    workspace,sceneConfig,sceneAccess:access,
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
    const failures=tasks.eventsAdmin(created.data.taskId,0,100).items.filter(x=>x.failure);
    return {state,prompts,pollErrors,elapsedMs:Date.now()-start,activeReads,failures};
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
  assert.equal(transientInstitutionPollError(new AuthProblem(502,'OPENCODE_HTTP_400')),false);
  assert.equal(transientInstitutionPollError(new Error('fetch failed: ECONNRESET')),true);
  assert.equal(transientInstitutionPollError(new AuthProblem(400,'INVALID_INSTITUTION_DATE')),false);
  assert.equal(transientInstitutionPollError(new Error('programmer bug')),false);
});

test('an unavailable native event stream preserves legacy completion polling instead of delaying tasks', {timeout:12000},async()=>{
  const value=await execute('stream-failed');
  assert.deepEqual(value.state,{status:'SUCCEEDED',errorCode:null});
  assert.equal(value.prompts,1,'an unavailable event feed must never resubmit the original prompt');
  assert.ok(value.activeReads>=2,'a second read-only status check is still made');
  assert.ok(value.elapsedMs<4500,'an unavailable event feed must not force the slow event-mode fallback');
  assert.ok(value.failures.some(x=>x.eventType==='NATIVE_TOOL_FAILED'&&
    x.failure?.category==='NETWORK'&&x.failure?.origin==='session-snapshot'));
});

test('a native completion event wakes the shared observer before slow polling fallback', {timeout:10000},async()=>{
  const root=await mkdtemp(join(tmpdir(),'institution-events-'));
  const db=join(root,'tasks.sqlite'),workspace=join(root,'project');
  const {mkdir}=await import('node:fs/promises');await mkdir(workspace);
  const sceneConfig=new SceneConfigStore(workspace);
  await sceneConfig.initialize();
  const current=sceneConfig.read();
  await sceneConfig.save({version:1,scenes:{...current.config.scenes,
    'institution-message':{directory:workspace,executor:{type:'skill',name:'institution-log-query'}}}},current.revision);
  const sceneAccess=new SceneAccessStore(workspace);
  const tasks=new InstitutionTaskStore(db,workspace);
  let submitted=false,completed=false,polls=0,prompts=0,subscriptions=0;
  const native={
    event:{subscribe:async function*({signal}:{signal:AbortSignal}){
      subscriptions++;
      while(!submitted&&!signal.aborted)await delay(5);
      await delay(25);
      yield {id:'native-tool-failure-1',type:'session.tool.failed',
        created:Date.now(),data:{sessionID:'native-session-1',id:'call-1',name:'dbhub',
          error:{type:'ToolError',code:'ETIMEDOUT',
            message:"SELECT * FROM sensitive_table WHERE token='super-secret-token' ETIMEDOUT"}}};
      await delay(45);
      completed=true;
      yield {id:'native-diagnostic-1',type:'session.execution.succeeded',
        created:Date.now(),data:{sessionID:'native-session-1'}};
      while(!signal.aborted)await delay(20);
    }},
    skill:{list:async()=>({location:{directory:workspace},data:[{id:'skill-id',name:'institution-log-query'}]})},
    model:{list:async()=>({data:[{id:'kimi',name:'Kimi',providerID:'private',enabled:true}]})},
    session:{
      create:async(input:{location:unknown;permissions:unknown})=>({
        id:'native-session-1',location:input.location,permissions:input.permissions,
      }),
      prompt:async()=>{prompts++;submitted=true;return {id:'accepted'};},
      active:async()=>{polls++;return completed?{}:{'native-session-1':true};},
      inbox:{list:async()=>[]},interrupt:async()=>({interrupted:true}),remove:async()=>({}),
    },
    message:{list:async()=>({data:completed?[{id:'done',type:'assistant',
      finish:'stop',time:{completed:Date.now()},content:[{type:'text',text:'有效结论'}]}]:[]})},
    permission:{list:async()=>[]},
  };
  const connection=async()=>native as unknown as Awaited<ReturnType<ReturnType<typeof chatConnection>>>;
  const execution=new ExecutionRegistry([],async()=>connection(),{
    workspace,singleServiceIdentity:'single:'+'f'.repeat(64),
  });
  const service=new InstitutionIntegration({clientId:'platform-b',databasePath:db},{
    workspace,sceneConfig,sceneAccess,auth:new LocalAuth('unused',4176),
    execution,nativeConnection:connection,
  },tasks);
  try{
    const start=Date.now();
    const taskId=service.create('user-hash','request-1','2026-11-26','000123','customer-001').data.taskId;
    let state:string|undefined;
    while(Date.now()-start<3000){
      state=tasks.listAdmin(undefined,undefined,0).items.find(row=>row.taskId===taskId)?.status;
      if(state==='SUCCEEDED')break;
      await delay(25);
    }
    assert.equal(state,'SUCCEEDED','native completion should not wait 6.5 seconds');
    assert.ok(Date.now()-start<2000,'event should wake read-only status check promptly');
    assert.equal(prompts,1,'native events must never re-execute the prompt');
    assert.equal(subscriptions,1,'one shared subscriber, not a subscriber per poll');
    assert.ok(polls<=4,'native event path should avoid repeatedly polling four HTTP APIs');
    const events=tasks.eventsAdmin(taskId,0,50);
    assert.ok(events.items.some(row=>row.eventType==='NATIVE_EXECUTION_SUCCEEDED'));
    assert.ok(events.items.some(row=>row.eventType==='TASK_SUCCEEDED'));
    const failed=events.items.find(row=>row.eventType==='NATIVE_TOOL_FAILED');
    assert.equal(failed?.failure?.category,'TIMEOUT');
    assert.equal(failed?.failure?.toolName,'dbhub');
    assert.equal(failed?.failure?.toolCallId,'call-1');
    assert.equal(failed?.failure?.errorCode,'ETIMEDOUT');
    assert.equal(failed?.failure?.origin,'native-event');
    assert.equal(JSON.stringify(events).includes('super-secret-token'),false);
    assert.equal(JSON.stringify(events).includes('sensitive_table'),false);
  }finally{service.close();await rm(root,{recursive:true,force:true});}
});

test('native error diagnostics classify known faults but never save raw SQL, credentials or server paths',()=>{
  const raw={type:'ToolExecutionError',code:'ETIMEDOUT',
    message:"SQL SELECT * FROM account WHERE password='p@ss123' and token='secret-value'; connect to 10.10.3.22 failed"};
  const a=makeSafeFailure(raw,'dbhub.query','call-abc','native-event');
  assert.equal(a.category,'TIMEOUT');
  assert.equal(a.errorCode,'ETIMEDOUT');
  assert.equal(a.nativeType,'ToolExecutionError');
  assert.equal(a.toolCallId,'call-abc');
  const encoded=encodeSafeFailure(raw,'dbhub.query','call-abc','native-event');
  assert.equal(encoded.includes('account'),false);
  assert.equal(encoded.includes('p@ss123'),false);
  assert.equal(encoded.includes('secret-value'),false);
  assert.equal(encoded.includes('10.10.3.22'),false);
  assert.equal(decodeSafeFailure(encoded)?.category,'TIMEOUT');
  assert.equal(decodeSafeFailure('an older tool label'),null);
  assert.equal(makeSafeFailure({message:'Access denied: EACCES'},'shell','call-2','worker').category,'PERMISSION');
  assert.equal(makeSafeFailure({message:'Unknown column in SQL query'},'dbhub','call-3','worker').category,'SQL');
  assert.equal(makeSafeFailure({message:'MCP service unavailable'},'dbhub','call-4','worker').category,'MCP');
  assert.equal(makeSafeFailure({message:'something unexpected'},'shell','call-5','worker').category,'UNKNOWN');
});
