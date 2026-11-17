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
