import { describe,it } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync,mkdirSync,writeFileSync,rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { execFileSync } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import type { Pool } from 'mysql2/promise';
import { getRefs } from '../../packages/git-provider/src/repository.ts';
import { previewCommittedChanges } from '../../packages/git-provider/src/preview.ts';
import { createTask,getOperation } from '../../apps/server/src/modules/tasks/service.ts';
import { AuthProblem } from '../../apps/server/src/auth.ts';

describe('T05 task transaction and operation idempotency (deterministic SQL stub, not MySQL)',()=>{
  it('creates Task/Run/Outbox/Event once and replays the same operation identity',async()=>{
    const parent=mkdtempSync(join(tmpdir(),'wh-task-')),root=join(parent,'repo');
    mkdirSync(root);
    const git=(...a:string[])=>execFileSync('git',['-C',root,...a],{encoding:'utf8'});
    try{
      git('init','-q');git('config','user.name','Tester');git('config','user.email','t@example.invalid');
      writeFileSync(join(root,'Controller.java'),'class Controller {}\n');
      git('add','.');git('commit','-qm','first');git('branch','baseline');
      writeFileSync(join(root,'Controller.java'),'class Controller { int value; }\n');
      git('add','.');git('commit','-qm','second');
      const repositoryId=randomUUID(),ownerId=randomUUID(),previewId=randomUUID(),operationId=randomUUID();
      const refs=await getRefs(root,repositoryId,'fixture-signing-key-value');
      const base=refs.items.find(r=>r.fullName==='refs/heads/baseline');
      assert.ok(base);
      const proof=await previewCommittedChanges(root,repositoryId,'fixture-signing-key-value',base.id);
      const row={id:previewId,repository_id:repositoryId,repository_status:'READY',repository_name:'demo',
        canonical_path:root,head_oid:proof.scope.headOID,base_oid:proof.scope.baseOID,
        merge_base_oid:proof.scope.mergeBaseOID,base_ref:proof.scope.baseRef,
        object_format:proof.scope.objectFormat,source_branch:proof.scope.sourceBranch,not_expired:1};
      type OperationRow={operation_key:string;status:string;task_id:string;run_id:string;
        error_code:null;result_json:string;action:string;request_sha256:string};
      let stored:OperationRow|null=null,createdTaskId:string|null=null,insertCount=0,commits=0;
      let queued=0,expired=false;
      const execute=async(sql:string,params:unknown[]=[])=>{
        if(sql.includes('FROM wh_operations WHERE owner_id=')){
          return [stored&&params[0]===ownerId&&params[1]===stored.operation_key?[stored]:[],[]];
        }
        if(sql.includes('FROM wh_review_previews p JOIN wh_repositories')||
          sql.includes('FROM wh_review_previews p JOIN wh_repositories r'))
          return [[{...row,not_expired:expired?0:1}],[]];
        if(sql.includes('FROM wh_executor_slots'))return [[{slot_key:'local-default'}],[]];
        if(sql.includes('FROM wh_tasks WHERE preview_id='))return [createdTaskId?[{id:createdTaskId}]:[],[]];
        if(sql.includes('COUNT(*) AS total'))return [[{total:queued}],[]];
        if(sql.includes('INSERT INTO wh_operations')){
          stored={operation_key:String(params[2]),status:'ACCEPTED',
            task_id:String(params[5]),run_id:String(params[6]),error_code:null,
            result_json:String(params[7]),action:String(params[3]),request_sha256:String(params[4])};
        }
        if(sql.includes('INSERT INTO wh_tasks'))createdTaskId=String(params[0]);
        if(sql.startsWith('INSERT'))insertCount++;
        return [[],[]];
      };
      const tx={execute,query:async()=>[[],[]],beginTransaction:async()=>{},commit:async()=>{commits++;},
        rollback:async()=>{},release:()=>{}};
      const pool={execute,getConnection:async()=>tx} as unknown as Pool;
      const opts={pool,roots:[parent],cursorKey:'x'.repeat(40),notesSupported:false};
      const input={ownerId,operationId,previewId,controller:'Controller'};
      const result=await createTask(input,opts);
      assert.equal(result.status,'ACCEPTED');
      assert.ok(result.taskId);
      assert.ok(result.runId);
      assert.equal(insertCount,6); // Task, Snapshot, Run, Operation, Outbox, Event
      assert.equal(commits,1);
      assert.deepEqual(await createTask(input,opts),result);
      assert.equal(insertCount,6);
      assert.deepEqual(await getOperation(pool,ownerId,operationId),result);
      await assert.rejects(()=>createTask({...input,title:'changed'},opts),
        e=>e instanceof AuthProblem&&e.code==='OPERATION_CONFLICT');
      await assert.rejects(()=>createTask({...input,operationId:randomUUID()},opts),
        e=>e instanceof AuthProblem&&e.code==='PREVIEW_USED');
      assert.equal(insertCount,6);
      await assert.rejects(()=>createTask({...input,operationId:randomUUID(),notes:'new scope'},opts),
        e=>e instanceof AuthProblem&&e.code==='NOTES_UNSUPPORTED');
      stored=null;createdTaskId=null;queued=20;
      await assert.rejects(()=>createTask({...input,operationId:randomUUID()},opts),
        e=>e instanceof AuthProblem&&e.code==='QUEUE_FULL');
      queued=0;expired=true;
      await assert.rejects(()=>createTask({...input,operationId:randomUUID()},opts),
        e=>e instanceof AuthProblem&&e.code==='PREVIEW_EXPIRED');
    }finally{rmSync(parent,{recursive:true,force:true});}
  });
  it('requires a valid uuid and rejects malformed Controller names before persistence',async()=>{
    const fake={execute:async()=>{throw new Error('no DB expected');}} as unknown as Pool;
    const opts={pool:fake,roots:[],cursorKey:'x'.repeat(40),notesSupported:false};
    await assert.rejects(()=>createTask({ownerId:randomUUID(),operationId:'bad',
      previewId:randomUUID()},opts),
      e=>e instanceof AuthProblem&&e.code==='INVALID_UUID');
    await assert.rejects(()=>createTask({ownerId:randomUUID(),operationId:randomUUID(),
      previewId:randomUUID(),controller:'--exec'},opts),
      e=>e instanceof AuthProblem&&e.code==='INVALID_CONTROLLER');
  });
});
