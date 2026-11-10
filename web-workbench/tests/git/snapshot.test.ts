import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, rmSync, existsSync } from 'node:fs';
import { writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { execFileSync } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { getRefs, GitFailure } from '../../packages/git-provider/src/repository.ts';
import { previewCommittedChanges } from '../../packages/git-provider/src/preview.ts';
import { prepareSnapshot } from '../../packages/git-provider/src/snapshot.ts';
import { listSnapshotFiles, readSnapshotLines } from '../../packages/git-provider/src/files.ts';

describe('T04 immutable Git snapshot and bounded file evidence (real local Git)',()=>{
  it('freezes HEAD/base independently, blocks protected evidence and preserves source',async()=>{
    const parent=mkdtempSync(join(tmpdir(),'wh-snapshot-'));
    const root=join(parent,'source'),data=join(parent,'data');
    mkdirSync(root);
    const run=(...args:string[])=>execFileSync('git',['-C',root,...args],{encoding:'utf8'}).trim();
    try {
      run('init','-q');run('config','user.name','Snapshot Fixture');
      run('config','user.email','fixture@example.invalid');
      writeFileSync(join(root,'OrderController.java'),'class OrderController {}\n');
      writeFileSync(join(root,'DeletedController.java'),'class DeletedController {}\n');
      run('add','.');run('commit','-qm','baseline');
      run('branch','baseline');
      writeFileSync(join(root,'OrderController.java'),'class OrderController { int amount; }\n');
      writeFileSync(join(root,'.env'),'SECRET_IN_SOURCE_ONLY=1\n');
      rmSync(join(root,'DeletedController.java'));
      writeFileSync(join(root,'数据库.sql'),'select 1;\n');
      run('add','-A');run('commit','-qm','feature');
      writeFileSync(join(root,'dirty.txt'),'not committed\n');
      const initialHead=run('rev-parse','HEAD');
      const initialDirty=run('status','--porcelain=v1');
      const repoId=randomUUID(),secret='fixture-hmac-key-value-long-enough-0123456';
      const refs=await getRefs(root,repoId,secret);
      const base=refs.items.find(r=>r.fullName==='refs/heads/baseline');
      assert.ok(base);
      const p=await previewCommittedChanges(root,repoId,secret,base.id);
      const taskId=randomUUID();
      const built=await prepareSnapshot({taskId,sourceRoot:root,dataRoot:data,scope:p.scope});
      const snapshotRoot=join(data,'tasks',taskId,'repo');
      assert.equal(run('rev-parse','HEAD'),initialHead);
      assert.equal(run('status','--porcelain=v1'),initialDirty);
      assert.equal(execFileSync('git',['-C',snapshotRoot,'rev-parse','HEAD'],{encoding:'utf8'}).trim(),initialHead);
      assert.equal(execFileSync('git',['-C',snapshotRoot,'rev-parse','refs/heads/harness-review-base'],{encoding:'utf8'}).trim(),p.scope.mergeBaseOID);
      assert.equal(execFileSync('git',['-C',snapshotRoot,'status','--porcelain=v1'],{encoding:'utf8'}).trim(),'');
      assert.equal(existsSync(join(snapshotRoot,'.git','objects','info','alternates')),false);
      const fileList=await listSnapshotFiles(data,built.snapshot);
      assert.ok(fileList.items.some(f=>f.path==='OrderController.java'&&f.side==='HEAD'));
      assert.ok(fileList.items.some(f=>f.path==='数据库.sql'&&f.side==='HEAD'));
      assert.ok(fileList.items.some(f=>f.path==='DeletedController.java'&&f.side==='BASE'));
      assert.ok(fileList.items.every(f=>f.path!=='.env'));
      assert.ok(fileList.excluded.some(f=>f.path==='.env'));
      const old=fileList.items.find(f=>f.path==='OrderController.java'&&f.side==='BASE');
      assert.ok(old);
      const original=await readSnapshotLines(data,built.snapshot,old.id);
      assert.match(original.lines[0],/class OrderController/);
      assert.ok(!original.lines[0].includes('amount'));
      const deleted=fileList.items.find(f=>f.path==='DeletedController.java'&&f.side==='BASE');
      assert.ok(deleted);
      const evidence=await readSnapshotLines(data,built.snapshot,deleted.id);
      assert.match(evidence.lines[0],/DeletedController/);
      await assert.rejects(()=>readSnapshotLines(data,built.snapshot,old.id,1,501),
        (e:unknown)=>e instanceof GitFailure&&e.code==='INVALID_LINE_RANGE');
      await assert.rejects(()=>readSnapshotLines(data,built.snapshot,'1'.repeat(64)),
        (e:unknown)=>e instanceof GitFailure&&e.code==='FILE_NOT_BROWSABLE');
      writeFileSync(join(root,'OrderController.java'),'class OrderController { int newer; }\n');
      run('add','OrderController.java');run('commit','-qm','later source change');
      const frozen=await readSnapshotLines(data,built.snapshot,fileList.items.find(f=>f.path==='OrderController.java'&&f.side==='HEAD')!.id);
      assert.ok(frozen.lines[0].includes('amount'));
      const manifestPath=join(data,'tasks',taskId,'manifest.json');
      await writeFile(manifestPath,readFileSync(manifestPath,'utf8')+' ');
      await assert.rejects(()=>listSnapshotFiles(data,built.snapshot),
        (e:unknown)=>e instanceof GitFailure&&e.code==='MANIFEST_HASH_MISMATCH');
    }finally {rmSync(parent,{recursive:true,force:true});}
  });
  it('rejects any preexisting snapshot destination without silently overwriting',async()=>{
    const parent=mkdtempSync(join(tmpdir(),'wh-snapshot-conflict-'));
    const root=join(parent,'source'),data=join(parent,'data');
    mkdirSync(root);
    const git=(...a:string[])=>execFileSync('git',['-C',root,...a],{encoding:'utf8'});
    try{
      git('init','-q');git('config','user.email','fixture@example.invalid');git('config','user.name','Fixture');
      writeFileSync(join(root,'Order.java'),'class Order {}\n');
      git('add','.');git('commit','-qm','first');git('branch','baseline');
      writeFileSync(join(root,'Order.java'),'class Order { int id; }\n');
      git('add','.');git('commit','-qm','second');
      const repoId=randomUUID(),refs=await getRefs(root,repoId,'fixture');
      const ref=refs.items.find(r=>r.fullName==='refs/heads/baseline');
      assert.ok(ref);
      const proof=await previewCommittedChanges(root,repoId,'fixture',ref.id);
      const input={taskId:randomUUID(),sourceRoot:root,dataRoot:data,scope:proof.scope};
      await prepareSnapshot(input);
      await assert.rejects(()=>prepareSnapshot(input),
        (e:unknown)=>e instanceof GitFailure&&e.code==='SNAPSHOT_ALREADY_EXISTS');
    }finally{rmSync(parent,{recursive:true,force:true});}
  });
});
