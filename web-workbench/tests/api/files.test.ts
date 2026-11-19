import { describe,it } from 'node:test';
import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import type { Pool } from 'mysql2/promise';
import { loadTaskSnapshot } from '../../apps/server/src/modules/tasks/files.ts';
import { AuthProblem } from '../../apps/server/src/auth.ts';

describe('T04 task file owner and readiness API',()=>{
  it('does not expose the existence of another owner task',async()=>{
    const pool={execute:async()=>[[],[]]} as unknown as Pool;
    await assert.rejects(()=>loadTaskSnapshot(pool,randomUUID(),randomUUID()),
      (e:unknown)=>e instanceof AuthProblem&&e.status===404);
  });
  it('only allows READY snapshots tied to the requested task',async()=>{
    const taskId=randomUUID(),ownerId=randomUUID();
    const row={task_id:taskId,status:'PENDING'};
    let calls=0;
    const pool={execute:async(_sql:string,params:unknown[])=>{
      calls++;assert.deepEqual(params,[ownerId,taskId]);
      return [[row],[]];
    }} as unknown as Pool;
    await assert.rejects(()=>loadTaskSnapshot(pool,ownerId,taskId),
      (e:unknown)=>e instanceof AuthProblem&&e.code==='SNAPSHOT_NOT_READY');
    assert.equal(calls,1);
    Object.assign(row,{status:'READY',head_oid:'a'.repeat(40),base_oid:'b'.repeat(40),
      merge_base_oid:'c'.repeat(40),base_ref:'refs/heads/main',source_branch:'main',
      object_format:'sha1',storage_key:'tasks/'+taskId+'/repo',manifest_sha256:'d'.repeat(64)});
    const snap=await loadTaskSnapshot(pool,ownerId,taskId);
    assert.equal(snap.taskId,taskId);
    assert.equal(snap.scope.baseRef,'refs/heads/main');
    assert.equal(snap.storageKey,'tasks/'+taskId+'/repo');
  });
});
