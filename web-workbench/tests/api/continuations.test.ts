import { describe,it } from 'node:test';
import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import type { Pool } from 'mysql2/promise';
import { AuthProblem } from '../../apps/server/src/auth.ts';
import { createContinuation, listTaskMessages } from '../../apps/server/src/modules/messages/service.ts';

describe('T08 scoped continuation transactions (SQL stub, not live MySQL)',()=>{
  function fixture(){
    const ownerId=randomUUID(),taskId=randomUUID(),initialRun=randomUUID(),artifactId=randomUUID();
    let latestStatus='SUCCEEDED',snapshotStatus='READY',quarantined=0,queued=0;
    let written:Array<{kind:string;params:unknown[]}>=[],operations:Array<Record<string,unknown>>=[];
    let eventSeq=2;
    const execute=async(sql:string,params:unknown[]=[])=>{
      if(sql.includes("FROM wh_executor_slots"))return [[{run_id:null,quarantined}],[]];
      if(sql.includes('FROM wh_operations WHERE owner_id')){
        return [operations.filter(r=>r.owner_id===params[0]&&r.operation_key===params[1]),[]];
      }
      if(sql.includes('JOIN wh_scope_snapshots')&&sql.includes('FOR UPDATE'))
        return [[{id:taskId,next_run_number:2,snapshot_status:snapshotStatus}],[]];
      if(sql.includes('FROM wh_runs WHERE task_id=')&&sql.includes('ORDER BY run_number DESC'))
        return [[{id:initialRun,kind:'REVIEW',status:latestStatus,run_number:1,
          asset_versions:JSON.stringify({command:'codea-review'})}],[]];
      if(sql.includes('FROM wh_session_bindings'))return [[{task_id:taskId,continuity:'COMPLETE'}],[]];
      if(sql.includes('FROM wh_artifacts'))return [[{id:artifactId}],[]];
      if(sql.includes("COUNT(*) AS total"))return [[{total:queued}],[]];
      if(sql.includes('SELECT next_event_seq FROM wh_tasks WHERE id='))
        return [[{next_event_seq:String(eventSeq)}],[]];
      if(sql.startsWith('INSERT INTO wh_operations')){
        operations.push({owner_id:params[1],operation_key:params[2],action:params[3],
          request_sha256:params[4],task_id:params[5],run_id:params[6],
          status:'ACCEPTED',result_json:params[7],error_code:null});
      }
      if(sql.startsWith('UPDATE wh_tasks SET next_event_seq='))eventSeq=Number(params[0]);
      if(sql.startsWith('INSERT')||sql.startsWith('UPDATE'))written.push({kind:sql,params});
      return [[],[]];
    };
    const tx={execute,query:async()=>[[],[]],beginTransaction:async()=>{},commit:async()=>{},
      rollback:async()=>{},release:()=>{}};
    const pool={execute,getConnection:async()=>tx} as unknown as Pool;
    return {pool,ownerId,taskId,artifactId,writes:()=>written,
      status:(v:string)=>{latestStatus=v;},snapshot:(v:string)=>{snapshotStatus=v;},
      quarantine:(v:number)=>{quarantined=v;},queue:(v:number)=>{queued=v;}};
  }
  it('DISCUSS creates one new run, user message, operation and outbox; replay is stable',async()=>{
    const c=fixture(),operationId=randomUUID();
    const input={ownerId:c.ownerId,taskId:c.taskId,operationId,
      kind:'DISCUSS' as const,text:'请核查这里的事务回滚',quote:'public void create() {}',
      reportArtifactId:c.artifactId};
    const one=await createContinuation(input,{pool:c.pool});
    assert.equal(one.status,'ACCEPTED');
    assert.ok(one.runId);
    assert.equal((one.result as {kind:string}).kind,'DISCUSS');
    assert.equal(c.writes().filter(x=>x.kind.startsWith('INSERT INTO wh_runs')).length,1);
    assert.equal(c.writes().filter(x=>x.kind.startsWith('INSERT INTO wh_messages')).length,1);
    assert.equal(c.writes().filter(x=>x.kind.startsWith('INSERT INTO wh_outbox')).length,1);
    assert.deepEqual(await createContinuation(input,{pool:c.pool}),one);
    assert.equal(c.writes().filter(x=>x.kind.startsWith('INSERT INTO wh_runs')).length,1);
    await assert.rejects(()=>createContinuation({...input,text:'changed'},{pool:c.pool}),
      e=>e instanceof AuthProblem&&e.code==='OPERATION_CONFLICT');
  });
  it('RECHECK preserves the task and allocates a report-capable run',async()=>{
    const c=fixture();
    const one=await createContinuation({ownerId:c.ownerId,taskId:c.taskId,
      operationId:randomUUID(),kind:'RECHECK'},{pool:c.pool});
    assert.equal(one.status,'ACCEPTED');
    assert.equal((one.result as {kind:string}).kind,'RECHECK');
    assert.equal(c.writes().filter(x=>x.kind.startsWith('INSERT INTO wh_messages')).length,0);
  });
  it('blocks concurrent execution, wrong snapshot, unverified slot and oversized user text',async()=>{
    const c=fixture(),create=()=>createContinuation({ownerId:c.ownerId,taskId:c.taskId,
      operationId:randomUUID(),kind:'DISCUSS',text:'检查并发'},{pool:c.pool});
    c.status('RUNNING');
    await assert.rejects(create(),e=>e instanceof AuthProblem&&e.code==='LATEST_RUN_NOT_SUCCESSFUL');
    c.status('SUCCEEDED');c.snapshot('PENDING');
    await assert.rejects(create(),e=>e instanceof AuthProblem&&e.code==='SNAPSHOT_NOT_READY');
    c.snapshot('READY');c.quarantine(1);
    await assert.rejects(create(),e=>e instanceof AuthProblem&&e.code==='PREVIOUS_RUN_NOT_SETTLED');
    c.quarantine(0);c.queue(20);
    await assert.rejects(create(),e=>e instanceof AuthProblem&&e.code==='QUEUE_FULL');
    await assert.rejects(()=>createContinuation({ownerId:c.ownerId,taskId:c.taskId,
      operationId:randomUUID(),kind:'DISCUSS',text:'x'.repeat(8001)},{pool:c.pool}),
      e=>e instanceof AuthProblem&&e.code==='DISCUSSION_TEXT_INVALID');
    assert.equal(c.writes().length,0);
  });
  it('message listing requires owner and validates signed pagination key',async()=>{
    const ownerId=randomUUID(),taskId=randomUUID();
    const pool={execute:async(sql:string,params:unknown[])=>{
      if(sql.includes('SELECT id FROM wh_tasks'))return [[{id:taskId}],[]];
      if(sql.includes('FROM wh_messages'))return [[{
        id:randomUUID(),run_id:randomUUID(),role:'ASSISTANT',content:'完成',status:'COMPLETE',
        report_artifact_id:null,created_at:'2026-11-11 00:00:00.000',
        updated_at:'2026-11-11 00:00:00.000'}],[]];
      return [[],[]];
    }} as unknown as Pool;
    const output=await listTaskMessages({pool,ownerId,taskId,limit:10,cursorKey:'x'.repeat(40)});
    assert.equal(output.items[0].text,'完成');
    assert.equal(output.nextCursor,null);
    await assert.rejects(()=>listTaskMessages({pool,ownerId,taskId,limit:101,
      cursorKey:'x'.repeat(40)}),e=>e instanceof AuthProblem&&e.code==='LIMIT_INVALID');
  });
});
