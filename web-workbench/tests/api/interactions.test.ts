import { describe,it } from 'node:test';
import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import type { Pool } from 'mysql2/promise';
import { answerInteraction, listPendingInteractions } from '../../apps/server/src/modules/interactions/service.ts';
import { AuthProblem } from '../../apps/server/src/auth.ts';

describe('T09 owner-bound native interaction answers (transaction stub only)',()=>{
  const setup=(kind:'PERMISSION'|'FORM'='PERMISSION')=>{
    const ownerId=randomUUID(),taskId=randomUUID(),runId=randomUUID(),interactionId=randomUUID();
    const operationId=randomUUID(),nativeRequestId='native-fixture-1';
    const schema=kind==='PERMISSION'?{tool:'git',requestText:'git diff --stat',
      allowedDecisions:['ALLOW_ONCE','DENY']}:
      {title:'请确认',fields:[{id:'reason',label:'说明',type:'TEXT',required:true,
        options:[],maxLength:20},{id:'flag',label:'确认',type:'BOOLEAN',required:true,options:[],maxLength:0}]};
    const row={id:interactionId,task_id:taskId,run_id:runId,kind,status:'PENDING',revision:1,
      native_request_id:nativeRequestId,schema_json:JSON.stringify(schema),
      expires_at:'2026-11-11 10:00:00.000',not_expired:1,run_status:kind==='PERMISSION'?
      'WAITING_PERMISSION':'WAITING_INPUT',run_number:1};
    let writes=0,operation:{operation_key:string;action:string;request_sha256:string;task_id:string;
      run_id:string;status:string;result_json:string;error_code:null}|null=null;
    const execute=async(sql:string,params:unknown[]=[])=>{
      if(sql.includes("SELECT run_id,quarantined")&&sql.includes('wh_executor_slots'))
        return [[{run_id:runId,quarantined:0,valid:1}],[]];
      if(sql.includes('FROM wh_operations WHERE owner_id')){
        return [operation&&params[0]===ownerId&&params[1]===operation.operation_key?[operation]:[],[]];
      }
      if(sql.includes('FROM wh_interactions i JOIN wh_tasks')){
        return [params[0]===interactionId&&params[1]===ownerId?[row]:[],[]];
      }
      if(sql.includes('SELECT id FROM wh_runs WHERE task_id')){
        return [[{id:runId}],[]];
      }
      if(sql.includes('SELECT id FROM wh_tasks WHERE id=')){
        return [params[0]===taskId&&params[1]===ownerId?[{id:taskId}]:[],[]];
      }
      if(sql.includes('SELECT i.* FROM wh_interactions')){
        return [[row.status==='PENDING'?row:row],[]];
      }
      if(sql.startsWith('INSERT INTO wh_operations')){
        operation={operation_key:String(params[2]),action:String(params[3]),
          request_sha256:String(params[4]),task_id:String(params[5]),run_id:String(params[6]),
          status:'ACCEPTED',result_json:String(params[7]),error_code:null};
      }
      if(sql.startsWith('UPDATE wh_interactions'))row.status='ANSWERING';
      if(sql.startsWith('INSERT')||sql.startsWith('UPDATE'))writes++;
      return [[],[]];
    };
    const tx={execute,query:async()=>[[],[]],beginTransaction:async()=>{},commit:async()=>{},
      rollback:async()=>{},release:()=>{}};
    const pool={execute,getConnection:async()=>tx} as unknown as Pool;
    return {ownerId,taskId,runId,interactionId,operationId,row,pool,reads:()=>writes,
      reset:()=>{row.status='PENDING';}};
  };
  it('accepts only one allow-once permission and returns the original operation on retry',async()=>{
    const c=setup();
    const answer={kind:'PERMISSION',decision:'ALLOW_ONCE'};
    const input={pool:c.pool,ownerId:c.ownerId,interactionId:c.interactionId,
      operationId:c.operationId,revision:1,answer};
    const first=await answerInteraction(input);
    assert.equal(first.status,'ACCEPTED');
    assert.deepEqual(first.result,{interactionId:c.interactionId,revision:1});
    assert.equal(c.row.status,'ANSWERING');
    assert.equal(c.reads(),3); // interaction, operation, ANSWER outbox in one transaction
    assert.deepEqual(await answerInteraction(input),first);
    assert.equal(c.reads(),3);
    await assert.rejects(()=>answerInteraction({...input,answer:{kind:'PERMISSION',decision:'DENY'}}),
      e=>e instanceof AuthProblem&&e.code==='OPERATION_CONFLICT');
    assert.equal(c.reads(),3);
  });
  it('rejects global allow, unknown interaction, mismatched revision and stale waiting state',async()=>{
    const c=setup();
    const base={pool:c.pool,ownerId:c.ownerId,interactionId:c.interactionId,
      operationId:c.operationId,revision:1};
    await assert.rejects(()=>answerInteraction({...base,answer:{kind:'PERMISSION',decision:'ALLOW_ALWAYS'}}),
      e=>e instanceof AuthProblem&&e.code==='INVALID_PERMISSION_ANSWER');
    await assert.rejects(()=>answerInteraction({...base,interactionId:randomUUID(),
      answer:{kind:'PERMISSION',decision:'DENY'}}),
      e=>e instanceof AuthProblem&&e.code==='NOT_FOUND');
    await assert.rejects(()=>answerInteraction({...base,revision:3,
      answer:{kind:'PERMISSION',decision:'DENY'}}),
      e=>e instanceof AuthProblem&&e.code==='INTERACTION_NOT_CURRENT');
    assert.equal(c.reads(),0);
  });
  it('validates native form schema, required fields and cancel, without trusting user-supplied fields',async()=>{
    const c=setup('FORM');
    const base={pool:c.pool,ownerId:c.ownerId,interactionId:c.interactionId,
      operationId:c.operationId,revision:1};
    await assert.rejects(()=>answerInteraction({...base,answer:{
      kind:'FORM',cancel:false,values:{flag:true}}},
    ),e=>e instanceof AuthProblem&&e.code==='REQUIRED_FORM_FIELD_MISSING');
    await assert.rejects(()=>answerInteraction({...base,answer:{
      kind:'FORM',cancel:false,values:{reason:'OK',flag:true,admin:'yes'}}},
    ),e=>e instanceof AuthProblem&&e.code==='UNKNOWN_FORM_FIELD');
    const accepted=await answerInteraction({...base,answer:{
      kind:'FORM',cancel:false,values:{reason:'同意',flag:true}}});
    assert.equal(accepted.status,'ACCEPTED');
    assert.equal(c.row.status,'ANSWERING');
  });
  it('GET only returns current-user native requests',async()=>{
    const c=setup();
    assert.equal((await listPendingInteractions(c.pool,c.ownerId,c.taskId)).length,1);
    await assert.rejects(()=>listPendingInteractions(c.pool,randomUUID(),c.taskId),
      e=>e instanceof AuthProblem&&e.code==='NOT_FOUND');
  });
});
