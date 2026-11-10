import { describe,it } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync,mkdirSync,writeFileSync,rmSync,realpathSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { realpath } from 'node:fs/promises';
import { join } from 'node:path';
import { execFileSync } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { trustedRepository,getRefs,GitFailure } from '../../packages/git-provider/src/repository.ts';
import { previewCommittedChanges } from '../../packages/git-provider/src/preview.ts';
describe('T03 local committed Git preview (real git, no remote)',()=>{
  it('pins base OID and excludes uncommitted changes',async()=>{
    const parent=mkdtempSync(join(tmpdir(),'wh-git-')),root=join(parent,'repo');
    const git=(...args:string[])=>execFileSync('git',['-C',root,...args],{encoding:'utf8'});
    try {
      mkdirSync(root);git('init','-q');git('config','user.name','Tester');
      git('config','user.email','test@example.invalid');
      writeFileSync(join(root,'OrderController.java'),'class OrderController {}\n');
      git('add','.');git('commit','-qm','base');git('branch','base-for-review');
      writeFileSync(join(root,'OrderController.java'),'class OrderController { int a; }\n');
      git('add','.');git('commit','-qm','feature');
      writeFileSync(join(root,'untracked.sql'),'select 1;');
      assert.equal(await trustedRepository(root,[parent]),await realpath(root));
      const id=randomUUID(),secret='local-fixture-secret';
      const refs=await getRefs(root,id,secret);
      const base=refs.items.find(r=>r.fullName==='refs/heads/base-for-review');
      assert.ok(base);
      const preview=await previewCommittedChanges(root,id,secret,base.id);
      assert.equal(preview.changedFiles,1);
      assert.equal(preview.applicableFiles,1);
      assert.equal(preview.scope.headOID,refs.headOID);
      assert.equal(preview.dirtyCounts.untracked,1);
      await assert.rejects(()=>previewCommittedChanges(root,id,secret,'bad-ref'),
        e=>e instanceof GitFailure&&e.code==='BASE_REF_MISSING');
    }finally {rmSync(parent,{recursive:true,force:true});}
  });
});
