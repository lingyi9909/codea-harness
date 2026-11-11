import { it } from 'node:test';
import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import type { RowDataPacket } from 'mysql2/promise';
import { createPool,configFromEnv } from '../../packages/storage/src/db.ts';
import { runMigrations } from '../../packages/storage/src/migrate.ts';
import { collectNativeEvents } from '../../apps/server/src/workers/event-collector.ts';
import { dispatchControl,syncInteractions } from '../../apps/server/src/workers/native-controls.ts';
import { requestStop } from '../../apps/server/src/modules/runs/stop.ts';
import { answerInteraction } from '../../apps/server/src/modules/interactions/service.ts';
import type { OpenCodeAdapter } from '../../packages/contracts/src/ports.ts';
import type { NativeEnvelope, NativeObservation } from '../../packages/contracts/src/events.ts';
import { readTaskEvents } from '../../packages/storage/src/events.ts';

it('MySQL 8.4 with a scripted native peer: replay, control delivery and terminal reconciliation',async()=>{
  const config=configFromEnv(process.env);
  if(config.database!=='harness_ci_test')throw new Error('DEDICATED_CI_DATABASE_REQUIRED');
  await runMigrations(config,config.database,fileURLToPath(new URL('../../packages/storage/migrations',import.meta.url)));
  const pool=await createPool(config);
  const [owner,repo,preview,taskId,runId,sessionId,interactionId]=Array.from({length:7},()=>randomUUID());
  const lease={slotKey:'local-default' as const,taskId,runId,outboxId:randomUUID(),owner:'worker',fence:'1'};
  const binding={taskId,serviceIdentity:'fixture',sessionId,locationKey:'fixture'};
  const inputId='accepted-fixture';
  let answers=0,interrupts=0,pending=true;
  const starts:string[]=[];
  const types:NativeObservation[]=[
    {inputId:'older-input',kind:'message',status:'COMPLETE',nativeMessageId:'old',text:'old'},
    {inputId,kind:'tool',phase:'started',callId:'c1',tool:'read'},
    {inputId,kind:'delta',nativeMessageId:'m1',offset:'0',text:'临🌱'},
    {inputId,kind:'delta',nativeMessageId:'m1',offset:'2',text:'时'},
    {inputId,kind:'message',status:'COMPLETE',nativeMessageId:'m1',text:'public reply'},
    {inputId,kind:'terminal',outcome:'SUCCEEDED'}];
  const unexpected=async():Promise<never>=>{throw new Error('UNEXPECTED_NATIVE_CALL');};
  const adapter:OpenCodeAdapter={
    probe:unexpected,createSession:unexpected,putContext:unexpected,startReview:unexpected,
    discuss:unexpected,readMessages:unexpected,
    async *subscribe(_binding,cursor){
      starts.push(String(cursor));
      const indexes=starts.length===1?[0,1,2,3,4]:[4,5];
      for(const index of indexes){
        const envelope:NativeEnvelope={taskId,runId,type:'fixture',data:{},cursor:String(index+1),
          observation:types[index]};
        yield envelope;
      }
    },
    async listInteractions(){return pending?[{kind:'PERMISSION',nativeRequestId:'fixture-request',
      id:interactionId,taskId,runId,status:'PENDING',revision:1,tool:'read',requestText:'Read fixture',
      expiresAt:new Date(Date.now()+600000).toISOString(),allowedDecisions:['DENY']}]:[];},
    async answer(_binding,id,answer){
      assert.equal(id,interactionId);assert.deepEqual(answer,{kind:'PERMISSION',decision:'DENY'});
      answers++;pending=false;return {state:'ACCEPTED',inputId:'reply'};
    },
    async interrupt(){interrupts++;return {responseReceived:true,interrupted:false};},
    async inspect(){return {active:false,pendingInputs:0,pendingInteractions:0,externalMutation:false,acceptedInputIds:[]};}
  };
  const ctx={pool,binding,lease,adapter,inputId,durable:true,dataRoot:'.'};
  try{
    await pool.execute("INSERT INTO wh_owners VALUES (?,?,'Fixture',NOW(3),NOW(3))",[owner,owner]);
    await pool.execute("INSERT INTO wh_repositories(id,owner_id,name,canonical_path,path_sha256,created_at,updated_at) "+
      "VALUES (?,?,'fixture','fixture',SHA2(?,256),NOW(3),NOW(3))",[repo,owner,repo]);
    await pool.execute("INSERT INTO wh_review_previews(id,owner_id,repository_id,base_ref_id,base_ref,head_oid,base_oid,"+
      "merge_base_oid,object_format,summary,expires_at,created_at) VALUES (?,?,?,'base','base','a','b','b','sha1',JSON_OBJECT(),NOW(3),NOW(3))",
      [preview,owner,repo]);
    await pool.execute("INSERT INTO wh_tasks(id,owner_id,repository_id,preview_id,title,created_at,updated_at) "+
      "VALUES (?,?,?,?,'fixture',NOW(3),NOW(3))",[taskId,owner,repo,preview]);
    await pool.execute("INSERT INTO wh_runs(id,task_id,run_number,kind,status,dispatch_state,native_input_id,asset_versions,lease_fence,created_at,updated_at) "+
      "VALUES (?,?,1,'DISCUSS','RUNNING','ACCEPTED',?,JSON_OBJECT(),1,NOW(3),NOW(3))",[runId,taskId,inputId]);
    await pool.execute("INSERT INTO wh_session_bindings(task_id,service_identity,native_session_id,location_key,native_version,capabilities,created_at,updated_at) "+
      "VALUES (?,'fixture',?,'fixture','fixture',JSON_ARRAY(),NOW(3),NOW(3))",[taskId,sessionId]);
    const [slot]=await pool.query<RowDataPacket[]>("SELECT run_id,quarantined FROM wh_executor_slots WHERE slot_key='local-default'");
    assert.equal(slot[0].run_id,null);assert.equal(slot[0].quarantined,0);
    await pool.execute("UPDATE wh_executor_slots SET run_id=?,lease_owner='worker',fence=1,lease_until=NOW(3)+INTERVAL 30 SECOND WHERE slot_key='local-default' AND run_id IS NULL",[runId]);
    await syncInteractions(ctx);
    const answer=await answerInteraction({pool,ownerId:owner,interactionId,revision:1,operationId:randomUUID(),
      answer:{kind:'PERMISSION',decision:'DENY'}});
    await dispatchControl(ctx);await dispatchControl(ctx);await syncInteractions(ctx);
    const stop=await requestStop(pool,{ownerId:owner,runId,operationId:randomUUID()});
    await dispatchControl(ctx);await dispatchControl(ctx);
    assert.equal(answers,1);assert.equal(interrupts,1);
    await collectNativeEvents(ctx,AbortSignal.timeout(15000));
    assert.deepEqual(starts,['0','5']);
    const [states]=await pool.execute<RowDataPacket[]>('SELECT status FROM wh_runs WHERE id=?',[runId]);
    assert.equal(states[0].status,'SUCCEEDED');
    const [messages]=await pool.execute<RowDataPacket[]>('SELECT content FROM wh_messages WHERE task_id=?',[taskId]);
    assert.deepEqual(messages.map(r=>r.content),['public reply']);
    const events=await readTaskEvents(pool,taskId,'0');
    const delta=events.filter(e=>e.type==='message.delta');
    assert.equal(delta.map(e=>e.data.text).join(''),'临🌱时');
    assert.equal(delta[0].data.offset,'0');
    const completed=events.filter(e=>e.type==='message.completed');
    assert.equal(completed.length,1);
    assert.equal(completed[0].data.messageId,delta[0].data.messageId);
    assert.equal(events.filter(e=>e.type==='tool.started').length,1);
    assert.deepEqual(events.map(e=>e.seq),events.map((_,i)=>String(i+1)));
    const [ops]=await pool.execute<RowDataPacket[]>('SELECT operation_key,status FROM wh_operations WHERE run_id=?',[runId]);
    assert.equal(ops.find(r=>r.operation_key===answer.operationId)?.status,'SUCCEEDED');
    assert.equal(ops.find(r=>r.operation_key===stop.operationId)?.status,'FAILED');
  }finally{await pool.end();}
});
