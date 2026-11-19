import { describe,it } from 'node:test';
import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import type { Pool, PoolConnection } from 'mysql2/promise';
import { appendEvent, parseEventSeq, readTaskEvents, taskLastSeq, EventError } from '../../packages/storage/src/events.ts';
import { withTransaction } from '../../packages/storage/src/db.ts';

describe('T07 persisted event and cursor semantics (SQL stub, not MySQL)',()=>{
  it('allocates monotonic string seq, dedupes by stable native ID and replays in order',async()=>{
    const taskId=randomUUID(),runId=randomUUID(),ownerId=randomUUID();
    let next=2n;
    let records:Array<{task_id:string;seq:string;run_id:string|null;event_type:string;
      dedupe_key:string|null;payload:string;created_at:string}>=[
      {task_id:taskId,seq:'1',run_id:runId,event_type:'task.created',dedupe_key:null,
        payload:JSON.stringify({taskId}),created_at:'2026-11-10 00:00:00.000'},
    ];
    let backup:{next:bigint;records:typeof records}|null=null;
    let commit=0,rollback=0;
    const execute=async(sql:string,params:unknown[]=[])=>{
      if(sql.includes('SELECT next_event_seq')&&sql.includes('FOR UPDATE'))
        return [[{next_event_seq:next.toString()}],[]];
      if(sql.includes('SELECT next_event_seq FROM wh_tasks WHERE id='))
        return [[{next_event_seq:next.toString()}],[]];
      if(sql.includes('SELECT seq FROM wh_events WHERE task_id=')&&sql.includes('dedupe_key=')){
        return [records.filter(e=>e.dedupe_key===params[1]&&e.task_id===params[0]).map(e=>({seq:e.seq})),[]];
      }
      if(sql.includes('INSERT INTO wh_events')){
        records.push({task_id:String(params[0]),seq:String(params[1]),run_id:params[2]===null?null:String(params[2]),
          event_type:String(params[3]),dedupe_key:params[4]===null?null:String(params[4]),
          payload:String(params[5]),created_at:'2026-11-10 00:00:01.000'});
      }
      if(sql.includes('UPDATE wh_tasks SET next_event_seq='))next=BigInt(String(params[0]));
      if(sql.includes('SELECT task_id,seq,run_id,event_type,payload,created_at FROM wh_events')){
        return [records.filter(e=>e.task_id===params[0]&&BigInt(e.seq)>BigInt(String(params[1])))
          .slice(0,Number(params[2])),[]];
      }
      return [[],[]];
    };
    const tx={execute,query:async()=>[[],[]],
      beginTransaction:async()=>{backup={next,records:records.map(r=>({...r}))};},
      commit:async()=>{commit++;backup=null;},
      rollback:async()=>{rollback++;if(backup){next=backup.next;records=backup.records;}backup=null;},
      release:()=>{}} as unknown as PoolConnection;
    const pool={execute,getConnection:async()=>tx} as unknown as Pool;
    const one={taskId,runId,type:'run.status' as const,
      data:{status:'RUNNING' as const,previousStatus:'QUEUED' as const,errorCode:null}};
    const seq=await withTransaction(pool,t=>appendEvent(t,one,'native-event-1'));
    assert.equal(seq,'2');
    assert.equal(await withTransaction(pool,t=>appendEvent(t,one,'native-event-1')),'2');
    assert.equal(records.length,2);
    assert.equal(next,3n);
    assert.equal(await taskLastSeq(pool,ownerId,taskId),'2');
    const replay=await readTaskEvents(pool,taskId,'0',500);
    assert.deepEqual(replay.map(e=>e.seq),['1','2']);
    assert.equal(replay[1].type,'run.status');
    assert.deepEqual((replay[1] as Extract<typeof replay[number],{type:'run.status'}>).data,one.data);
    await assert.rejects(()=>withTransaction(pool,async t=>{
      await appendEvent(t,{taskId,runId,type:'continuity.gap',
        data:{reason:'NO_NATIVE_LOG',recoveredThroughNativeCursor:null}});
      throw new Error('deliberate rollback');
    }),/deliberate rollback/);
    assert.equal(next,3n);
    assert.equal(records.length,2);
    assert.equal(rollback,1);
    assert.equal(commit,2);
  });
  it('rejects invalid, negative, fractional, huge and leading-zero SSE cursors',()=>{
    for(const cursor of ['-1','1.0','01','NaN','1e3','', '18446744073709551616']){
      assert.throws(()=>parseEventSeq(cursor),e=>e instanceof EventError);
    }
    assert.equal(parseEventSeq('18446744073709551615'),'18446744073709551615');
  });
  it('rejects oversized event payloads before any database write',async()=>{
    const tx={execute:async()=>{throw new Error('DB should not be called')}} as unknown as PoolConnection;
    await assert.rejects(()=>appendEvent(tx,{taskId:randomUUID(),runId:randomUUID(),
      type:'continuity.gap',data:{reason:'X'.repeat(20000),recoveredThroughNativeCursor:null}}),
      e=>e instanceof EventError&&e.code==='EVENT_INVALID_OR_TOO_LARGE');
  });
});
