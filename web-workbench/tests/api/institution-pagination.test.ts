import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {InstitutionTaskStore} from '../../apps/server/src/institution-task-store.ts';

test('institution task/group lists paginate in SQL without duplicating records',async()=>{
  const root=await mkdtemp(join(tmpdir(),'institution-pages-'));
  const store=new InstitutionTaskStore(join(root,'tasks.sqlite'));
  try{
    for(let i=0;i<125;i++)
      store.create('platform-b','hash-main','request-'+i,'2026-11-25','000123','customer-001');
    for(let i=0;i<57;i++)
      store.create('platform-b','user-hash-'+i,'group-'+i,'2026-11-25','000456',i===1?'customer-001':'customer-'+i);
    const a=store.listAdmin(undefined,undefined,0,undefined,20);
    const b=store.listAdmin(undefined,undefined,20,undefined,20);
    const c=store.listAdmin(undefined,undefined,150,undefined,50);
    assert.equal(a.total,182);
    assert.equal(a.limit,20);
    assert.equal(a.items.length,20);
    assert.equal(b.items.length,20);
    assert.equal(c.items.length,32);
    assert.equal(new Set([...a.items,...b.items].map(row=>row.taskId)).size,40,
      'pagination should return distinct rows');
    const filtered=store.listAdmin(undefined,'customer-001',0,undefined,100);
    assert.equal(filtered.total,126);
    assert.equal(filtered.items.length,100);
    const byHash=store.listAdmin(undefined,undefined,0,'hash-main',50);
    assert.equal(byHash.total,125);
    assert.equal(byHash.items.length,50);
    const groupsA=store.groupsAdmin(undefined,0,20);
    const groupsB=store.groupsAdmin(undefined,20,20);
    assert.equal(groupsA.total,58);
    assert.equal(groupsA.items.length,20);
    assert.equal(groupsB.items.length,20);
    assert.equal(new Set([...groupsA.items,...groupsB.items].map(row=>row.userHash)).size,40);
    const groupsFiltered=store.groupsAdmin('customer-001',0,20);
    assert.equal(groupsFiltered.total,2,'two different caller hashes may share a self-reported label');
  }finally{store.close();await rm(root,{recursive:true,force:true});}
});

test('diagnostic events are retained, deduplicated and paginated without leaking source payloads',async()=>{
  const root=await mkdtemp(join(tmpdir(),'institution-diagnostic-pages-'));
  const database=join(root,'tasks.sqlite');
  const store=new InstitutionTaskStore(database);
  try{
    const task=store.create('platform-b','person-a','request-1','2026-11-26','000123').row;
    for(let n=0;n<65;n++)store.appendEvent(task.id,'NATIVE_TOOL_CALLED','INFO',undefined,'native-'+n);
    store.appendEvent(task.id,'NATIVE_TOOL_CALLED','INFO',undefined,'native-0');
    const first=store.eventsAdmin(task.id,0,20);
    const second=store.eventsAdmin(task.id,20,20);
    const last=store.eventsAdmin(task.id,60,20);
    assert.equal(first.total,66,'65 tool states plus initial task creation');
    assert.equal(first.items.length,20);
    assert.equal(second.items.length,20);
    assert.equal(last.items.length,6);
    assert.equal(new Set([...first.items,...second.items].map(item=>item.seq)).size,40);
    assert.equal(JSON.stringify(first).includes('000123'),false,'no raw msgSn in diagnostics');
    assert.equal(store.eventsAdmin(task.id,0,100).items.length,66);
  }finally{store.close();}
  const reopened=new InstitutionTaskStore(database);
  try{
    const page=reopened.listAdmin(undefined,undefined,0,undefined,20);
    assert.equal(page.total,1);
    assert.equal(reopened.eventsAdmin(page.items[0].taskId,0,100).total,67,
      'restart records one additional UNKNOWN reconciliation event for queued work');
    assert.equal(reopened.eventsAdmin(page.items[0].taskId,0,20).items[0].eventType,'TASK_RESTARTED');
    assert.throws(()=>reopened.eventsAdmin('missing',0,20),/INVESTIGATION_NOT_FOUND/);
  }finally{reopened.close();await rm(root,{recursive:true,force:true});}
});

test('diagnostic storage failures cannot alter existing task submission, execution or shutdown semantics',async()=>{
  const root=await mkdtemp(join(tmpdir(),'institution-diagnostic-fail-open-'));
  const store=new InstitutionTaskStore(join(root,'tasks.sqlite'));
  try{
    // Simulate an optional diagnostic-storage fault after the core task DB has opened.
    const raw=(store as unknown as {db:{exec:(sql:string)=>void}}).db;
    raw.exec('DROP TABLE institution_api_task_events');
    const first=store.create('b','owner','request-1','2026-11-26','000123');
    const second=store.create('b','owner','request-2','2026-11-26','000124');
    assert.equal(first.created,true);
    assert.equal(second.created,true);
    assert.equal(store.create('b','owner','request-1','2026-11-26','000123').created,false,
      'idempotency must remain intact if the event table is unavailable');
    const claimed=store.take();
    assert.ok([first.row.id,second.row.id].includes(claimed?.id??''),
      'a diagnostic write must not orphan the claimed job');
    store.finish(claimed!.id,'SUCCEEDED','Final answer',null);
    const completed=store.get('b','owner',claimed!.id);
    assert.equal(completed?.status,'SUCCEEDED');
    assert.equal(completed?.result_text,'Final answer');
    assert.equal(completed?.msg_sn,'');
    const pending=store.take();
    assert.ok(pending&&pending.id!==claimed?.id);
    store.stopUnfinished();
    assert.equal(store.get('b','owner',pending!.id)?.status,'UNKNOWN',
      'shutdown must still mark uncertain work UNKNOWN even when event writes fail');
  }finally{store.close();await rm(root,{recursive:true,force:true});}
});
