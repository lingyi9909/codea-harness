import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync, rmSync, readFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

function git(args:string[]):string{
  return execFileSync('git',args,{encoding:'utf8',stdio:['ignore','pipe','pipe']}).trim();
}

describe('T04 local committed-only independent snapshot (no model)',()=>{
  it('materializes fixed HEAD and base in an independent clone without touching original',async()=>{
    const root=mkdtempSync(join(tmpdir(),'harness-snapshot-'));
    const source=join(root,'original'),tasks=join(root,'tasks','task-one');
    try{
      mkdirSync(source);
      git(['init','-q',source]);
      git(['-C',source,'config','user.name','HarnessTest']);
      git(['-C',source,'config','user.email','harness-test@example.invalid']);
      mkdirSync(join(source,'src'));
      writeFileSync(join(source,'src','OrderService.java'),'class OrderService {}');
      git(['-C',source,'add','.']);
      git(['-C',source,'commit','-qm','initial']);
      const base=git(['-C',source,'rev-parse','HEAD']);
      writeFileSync(join(source,'src','OrderService.java'),'class OrderService { void run() {} }');
      git(['-C',source,'add','.']);
      git(['-C',source,'commit','-qm','change']);
      const head=git(['-C',source,'rev-parse','HEAD']);
      process.env.HARNESS_REPOSITORY_ROOTS=root;
      const {makeFrozenSnapshot}=await import('../../apps/server/src/workers/snapshot.ts');
      const {originalRepositoryFingerprint}=await import('../../packages/opencode-adapter/src/git-guard.ts');
      const fingerprint=originalRepositoryFingerprint(source);
      const snapshot=await makeFrozenSnapshot({
        taskId:'20fa366a-e4f1-4acd-b454-7cb76ff18941',
        repositoryPath:source,taskDirectory:tasks,
        scope:{headOID:head,baseOID:base,mergeBaseOID:base,baseRef:'refs/heads/main',
          sourceBranch:'main',objectFormat:'sha1'},
      });
      assert.equal(git(['-C',snapshot.storageKey,'rev-parse','HEAD']),head);
      assert.equal(git(['-C',snapshot.storageKey,'rev-parse','harness-review-base']),base);
      assert.equal(git(['-C',snapshot.storageKey,'status','--porcelain']),'');
      assert.equal(readFileSync(join(snapshot.storageKey,'src','OrderService.java'),'utf8'),
        'class OrderService { void run() {} }');
      assert.equal(snapshot.manifest.files.length,1);
      assert.equal(snapshot.manifest.files[0].path,'src/OrderService.java');
      assert.equal(snapshot.manifestSha256.length,64);
      assert.equal(originalRepositoryFingerprint(source),fingerprint);
    } finally {rmSync(root,{recursive:true,force:true});}
  });
});
