import {test} from 'node:test';
import assert from 'node:assert/strict';
import {ExecutionRegistry,type ExecutionRecord} from '../../apps/server/src/execution-registry.ts';
const records:ExecutionRecord[]=[0,1,2].map(i=>({ownerId:'00000000-0000-4000-8000-00000000000'+i,serviceIdentity:'service-'+i,url:'http://127.0.0.1:'+String(5000+i),authorization:'credential-'+i,isolation:'os-user',directories:{'/shared':'/isolated/user-'+i}}));
test('T3: three interleaved execution scopes retain service, directory and credential ownership',async()=>{
  const calls:string[][]=[];
  const router=new ExecutionRegistry(records,async(url,auth)=>{calls.push([url,auth]);return {url} as never;});
  const results=await Promise.all(records.map((record,i)=>router.run(record.ownerId,async()=>{
    await new Promise(resolve=>setTimeout(resolve,3-i));
    assert.equal(router.directory('/shared'),'/isolated/user-'+i);
    assert.equal(router.serviceIdentity(),record.serviceIdentity);
    assert.throws(()=>router.directory('/isolated/user-'+((i+1)%3)),/FORBIDDEN/);
    await router.client();return router.serviceIdentity();
  })));
  assert.equal(new Set(results).size,3);assert.equal(calls.length,3);
  await assert.rejects(router.run('unknown',async()=>{}),/EXECUTION_NOT_PROVISIONED/);
  assert.throws(()=>router.serviceIdentity(),/EXECUTION_CONTEXT_REQUIRED/);
});
test('T3: shared endpoints, credentials and overlapping writable roots are rejected',()=>{
  for(const replacement of [{url:records[0].url},{authorization:records[0].authorization},{directories:{'/shared':'/isolated/user-0/child'}},{isolation:'shared'}]){
    assert.throws(()=>new ExecutionRegistry([records[0],{...records[1],...replacement} as ExecutionRecord],async()=>({} as never)));
  }
});
test('shared readonly: three native services use exactly one startup workspace without weakening owner or credential checks',async()=>{
  const shared=records.map(r=>({...r,workspaceAccess:'shared-readonly' as const,directories:{'/shared':'/shared'}}));
  const calls:string[][]=[];const router=new ExecutionRegistry(shared,async(url,authorization)=>{calls.push([url,authorization]);return {} as never;},{workspace:'/shared'});
  await Promise.all(shared.map(r=>router.run(r.ownerId,async()=>{
    assert.equal(router.directory('/shared'),'/shared');assert.equal(router.sharedWorkspace(),true);
    assert.deepEqual(router.permissions(),[{action:'edit',resource:'*',effect:'deny'}]);
    router.assertPermissions({permissions:router.permissions()});
    assert.throws(()=>router.assertPermissions({permissions:[]}),/PERMISSION_MISMATCH/);
    assert.throws(()=>router.assertPermissions({permissions:[{action:'edit',resource:'*',effect:'allow'}]}),/PERMISSION_MISMATCH/);
    assert.throws(()=>router.directory('/other'),/FORBIDDEN/);await router.client();
  })));
  assert.equal(new Set(calls.map(c=>c[0])).size,3);assert.equal(new Set(calls.map(c=>c[1])).size,3);
  // The same directory is allowed only for this exact, explicitly readonly mode.
  for(const modified of [
    {...shared[1],workspaceAccess:undefined},
    {...shared[1],directories:{'/shared':'/shared/child'}},
    {...shared[1],directories:{'/shared':'/other'}},
    {...shared[1],directories:{'/shared':'/shared','/other':'/other'}},
    {...shared[1],url:shared[0].url},
    {...shared[1],authorization:shared[0].authorization},
  ])assert.throws(()=>new ExecutionRegistry([shared[0],modified],async()=>({} as never),{workspace:'/shared'}));
  assert.throws(()=>new ExecutionRegistry(shared,async()=>({} as never)),/SHARED_WORKSPACE_REQUIRED/);
});
