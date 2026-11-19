import { describe,it } from 'node:test';
import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import { mkdtempSync,mkdirSync,writeFileSync,readFileSync,rmSync } from 'node:fs';
import { mkdir } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { execFileSync } from 'node:child_process';
import type { Pool } from 'mysql2/promise';
import { getRefs } from '../../packages/git-provider/src/repository.ts';
import { previewCommittedChanges } from '../../packages/git-provider/src/preview.ts';
import { prepareSnapshot } from '../../packages/git-provider/src/snapshot.ts';
import { collectReport } from '../../apps/server/src/workers/report-collector.ts';
import type { NativeWriteEvidence } from '../../apps/server/src/workers/report-collector.ts';
import { readOwnedArtifact } from '../../apps/server/src/modules/artifacts/service.ts';
import { AuthProblem } from '../../apps/server/src/auth.ts';
import type { Lease } from '../../packages/storage/src/slots.ts';

const report=['## 评审摘要','本轮复查摘要','## 调用链与覆盖范围','OrderController',
  '## 风险清单','HIGH: 无','## 问题明细','暂无','## 后续处理与评审边界','保持固定源码范围'].join('\n\n');
describe('T10 safe per-run report archive (real filesystem, SQL stub)',()=>{
  it('archives five-section report with same-run native write evidence and detects tampering',async()=>{
    const parent=mkdtempSync(join(tmpdir(),'wh-report-')),root=join(parent,'source'),data=join(parent,'data');
    mkdirSync(root);
    const git=(...args:string[])=>execFileSync('git',['-C',root,...args],{encoding:'utf8'});
    try{
      git('init','-q');git('config','user.name','Fixture');git('config','user.email','f@example.invalid');
      writeFileSync(join(root,'Controller.java'),'class Controller {}\n');
      git('add','.');git('commit','-qm','baseline');git('branch','baseline');
      writeFileSync(join(root,'Controller.java'),'class Controller { int id; }\n');
      git('add','.');git('commit','-qm','feature');
      const taskId=randomUUID(),runId=randomUUID(),ownerId=randomUUID(),repoId=randomUUID();
      const refs=await getRefs(root,repoId,'fixture');
      const base=refs.items.find(r=>r.fullName==='refs/heads/baseline');
      assert.ok(base);
      const proof=await previewCommittedChanges(root,repoId,'fixture',base.id);
      const built=await prepareSnapshot({taskId,sourceRoot:root,dataRoot:data,scope:proof.scope});
      const snapRoot=join(data,'tasks',taskId,'repo');
      await mkdir(join(snapRoot,'temp'));
      writeFileSync(join(snapRoot,'temp','codea-review-report.md'),report);
      const lease:Lease={slotKey:'local-default',taskId,runId,outboxId:randomUUID(),
        owner:'worker-test',fence:'1'};
      const evidence:NativeWriteEvidence={taskId,runId,nativeSessionId:'native-1',
        serviceIdentity:'service-1',nativeWriteMessageId:'write-msg-1',
        path:'temp/codea-review-report.md',toolOutcome:'SUCCEEDED',
        modelTerminated:true,pendingInputs:0,pendingInteractions:0,externalMutation:false,
        coverage:'FULL'};
      let status='COLLECTING',artifact:Record<string,unknown>|null=null,seq=2;
      const snapshotRow={
        run_id:runId,status,kind:'REVIEW',started_at:'2000-01-01 00:00:00.000',
        head_oid:proof.scope.headOID,base_oid:proof.scope.baseOID,
        merge_base_oid:proof.scope.mergeBaseOID,base_ref:proof.scope.baseRef,
        source_branch:proof.scope.sourceBranch,object_format:proof.scope.objectFormat,
        storage_key:built.snapshot.storageKey,manifest_sha256:built.snapshot.manifestHash,
        snapshot_status:'READY',native_session_id:'native-1',service_identity:'service-1',
      };
      const execute=async(sql:string,params:unknown[]=[])=>{
        if(sql.includes('FROM wh_runs r JOIN wh_scope_snapshots')||
          (sql.includes('FROM wh_runs r ')&&sql.includes('JOIN wh_scope_snapshots')))
          return [[{...snapshotRow,run_status:status}],[]];
        if(sql.includes('FROM wh_executor_slots')&&sql.includes('FOR UPDATE'))
          return [[{run_id:runId,lease_owner:lease.owner,fence:lease.fence,
            quarantined:0,valid:1}],[]];
        if(sql.includes('FROM wh_artifacts WHERE run_id='))
          return [artifact?[artifact]:[],[]];
        if(sql.includes('SELECT status FROM wh_runs'))return [[{status}],[]];
        if(sql.includes('MAX(version) AS version'))return [[{version:0}],[]];
        if(sql.startsWith('INSERT INTO wh_artifacts')){
          artifact={id:params[0],task_id:params[1],run_id:params[2],version:params[3],
            kind:'REVIEW_MARKDOWN',storage_key:params[4],sha256:params[5],
            size_bytes:params[6],coverage:params[7],validation:params[8],
            native_write_message_id:params[9],created_at:'2026-11-11 00:00:00.000'};
          return [{affectedRows:1},[]];
        }
        if(sql.startsWith('UPDATE wh_runs SET status=')){
          status='SUCCEEDED';return [{affectedRows:1},[]];
        }
        if(sql.includes('SELECT next_event_seq FROM wh_tasks WHERE id='))return [[{next_event_seq:String(seq)}],[]];
        if(sql.startsWith('UPDATE wh_tasks SET next_event_seq='))seq=Number(params[0]);
        if(sql.includes('FROM wh_artifacts a JOIN wh_tasks')||
          (sql.includes('FROM wh_artifacts a ')&&sql.includes('JOIN wh_tasks'))){
          if(!artifact||params[0]!==artifact.id||params[1]!==ownerId)return [[],[]];
          return [[{...artifact,snapshot_status:'READY',
            head_oid:proof.scope.headOID,base_oid:proof.scope.baseOID,
            merge_base_oid:proof.scope.mergeBaseOID,base_ref:proof.scope.baseRef,
            source_branch:proof.scope.sourceBranch,object_format:proof.scope.objectFormat}],[]];
        }
        return [{affectedRows:1},[]];
      };
      const tx={execute,query:async()=>[[],[]],beginTransaction:async()=>{},commit:async()=>{},
        rollback:async()=>{},release:()=>{}};
      const pool={execute,getConnection:async()=>tx} as unknown as Pool;
      const saved=await collectReport({pool,dataRoot:data},lease,evidence);
      assert.equal(saved.version,1);
      assert.equal(saved.coverage,'FULL');
      assert.equal(status,'SUCCEEDED');
      assert.equal(seq,4); // artifact.ready + run.status committed
      const trusted=await readOwnedArtifact({pool,ownerId,artifactId:saved.id,dataRoot:data});
      assert.equal(trusted.artifact.markdown,report);
      assert.equal(trusted.artifact.meta.sha256,saved.sha256);
      const archive=join(data,'artifacts',taskId,runId+'.md');
      writeFileSync(archive,report+'\nTAMPERED');
      await assert.rejects(()=>readOwnedArtifact({pool,ownerId,artifactId:saved.id,dataRoot:data}),
        (e:unknown)=>e instanceof AuthProblem&&e.code==='ARTIFACT_HASH_MISMATCH');
      await assert.rejects(()=>readOwnedArtifact({pool,ownerId:randomUUID(),
        artifactId:saved.id,dataRoot:data}),(e:unknown)=>e instanceof AuthProblem&&e.code==='NOT_FOUND');
    }finally {rmSync(parent,{recursive:true,force:true});}
  });
  it('requires trusted native write provenance, no model-claim-only success',async()=>{
    const fake={execute:async()=>{throw Error('DB should not be called');}} as unknown as Pool;
    const runId=randomUUID(),taskId=randomUUID();
    const lease:Lease={slotKey:'local-default',runId,taskId,outboxId:randomUUID(),
      owner:'fixture',fence:'10'};
    const correct:NativeWriteEvidence={taskId,runId,nativeSessionId:'real',
      serviceIdentity:'real',nativeWriteMessageId:'w1',path:'temp/codea-review-report.md',
      toolOutcome:'SUCCEEDED',modelTerminated:true,pendingInputs:0,pendingInteractions:0,
      externalMutation:false,coverage:'FULL'};
    await assert.rejects(()=>collectReport({pool:fake,dataRoot:'.'},lease,
      {...correct,nativeWriteMessageId:''}),
      (e:unknown)=>e instanceof AuthProblem&&e.code==='NATIVE_REPORT_EVIDENCE_REQUIRED');
    await assert.rejects(()=>collectReport({pool:fake,dataRoot:'.'},lease,
      {...correct,path:'../../business-database.sql'}),
      (e:unknown)=>e instanceof AuthProblem&&e.code==='NATIVE_REPORT_EVIDENCE_REQUIRED');
    await assert.rejects(()=>collectReport({pool:fake,dataRoot:'.'},lease,
      {...correct,pendingInputs:1 as 0}),
      (e:unknown)=>e instanceof AuthProblem&&e.code==='NATIVE_REPORT_EVIDENCE_REQUIRED');
  });
});
