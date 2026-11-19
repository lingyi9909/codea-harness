import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,mkdir,writeFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {createEmbeddedRuntime} from '../../apps/server/src/embedded-start.ts';

test('isolated embedded runtime uses private native executors and preserves per-visitor grants',async()=>{
  const root=await mkdtemp(join(tmpdir(),'workbench-embed-isolated-'));
  const workspace=join(root,'workspace'),privateA=join(root,'private-a'),privateB=join(root,'private-b');
  await Promise.all([workspace,privateA,privateB].map(p=>mkdir(p,{recursive:true})));
  const owners=['11111111-1111-4111-8111-111111111111','22222222-2222-4222-8222-222222222222'];
  const registry=join(root,'registry.json'),config=join(root,'embed.json'),storage=join(root,'state.sqlite');
  await writeFile(registry,JSON.stringify(owners.map((ownerId,i)=>({
    ownerId,serviceIdentity:'isolated-'+i,url:'http://127.0.0.1:'+(53001+i),
    authorization:'Bearer service-private-'+i,isolation:'os-user',
    directories:{[workspace]:i?privateB:privateA},
  }))));
  await writeFile(config,JSON.stringify({
    publicOrigin:'http://127.0.0.1:4176',clients:[],
    anonymous:{parentOrigins:['http://127.0.0.1:4176'],executionOwners:owners},
  }));
  const envKeys=['HARNESS_SQLITE_PATH','HARNESS_EMBED_EXECUTION_REGISTRY',
    'HARNESS_EMBED_ISOLATED_CONFIG','HARNESS_HTTP_PORT','HARNESS_PUBLIC_ORIGIN'];
  const original=Object.fromEntries(envKeys.map(k=>[k,process.env[k]]));
  try{
    Object.assign(process.env,{HARNESS_SQLITE_PATH:storage,HARNESS_EMBED_EXECUTION_REGISTRY:registry,
      HARNESS_EMBED_ISOLATED_CONFIG:config,HARNESS_HTTP_PORT:'4176',
      HARNESS_PUBLIC_ORIGIN:'http://127.0.0.1:4176'});
    const rt=await createEmbeddedRuntime(workspace);
    try{
      assert.equal(rt.execution.singleService(),false);
      assert.deepEqual(rt.auth.config.anonymous?.executionOwners,owners);
      for(let i=0;i<owners.length;i++){
        const target=i?privateB:privateA;
        await rt.execution.run(owners[i],async()=>{
          assert.equal(rt.execution.directory(workspace),target);
          assert.equal(rt.execution.permissions(),undefined,'private execution should keep native Skill/MCP permissions');
          assert.equal(rt.execution.sharedWorkspace(),false);
        });
      }
      const first=await rt.store.allocate({
        id:'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',digest:'hash1',parentOrigin:rt.publicOrigin,
        expires:Date.now()+60000},owners.map((ownerId,i)=>({ownerId,serviceIdentity:'isolated-'+i})));
      const second=await rt.store.allocate({
        id:'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb',digest:'hash2',parentOrigin:rt.publicOrigin,
        expires:Date.now()+60000},owners.map((ownerId,i)=>({ownerId,serviceIdentity:'isolated-'+i})));
      assert.ok(first&&second);
      assert.notEqual(first.ownerId,second.ownerId);
      assert.notEqual(first.serviceIdentity,second.serviceIdentity);
      assert.equal((await rt.store.allocate({
        id:'cccccccc-cccc-4ccc-8ccc-cccccccccccc',digest:'hash3',
        parentOrigin:rt.publicOrigin,expires:Date.now()+60000
      },owners.map((ownerId,i)=>({ownerId,serviceIdentity:'isolated-'+i})))),undefined,
        'all isolated slots must be reserved until cleanup/reset');
    }finally{await rt.store.close();}
  }finally{
    for(const k of envKeys){if(original[k]===undefined)delete process.env[k];else process.env[k]=original[k];}
    await rm(root,{recursive:true,force:true});
  }
});
