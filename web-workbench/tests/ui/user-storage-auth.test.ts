import {test} from 'node:test';
import assert from 'node:assert/strict';
import {HarnessApi} from '../../apps/web/src/api/client.ts';
import {scopedStorage,resetUserStorage} from '../../apps/web/src/userStorage.ts';
test('pairing restores legacy task and draft caches before the authenticated UI renders',async()=>{
  const values=new Map([['harness.scene','settings'],['harness.task','task-a'],['harness.draft.task-a','saved draft']]);
  const raw:Storage={get length(){return values.size;},key:i=>[...values.keys()][i]??null,getItem:k=>values.get(k)??null,setItem:(k,v)=>{values.set(k,v);},removeItem:k=>{values.delete(k);},clear:()=>values.clear()};
  const cache=scopedStorage(()=>raw),original=globalThis.fetch;resetUserStorage();
  assert.equal(cache.getItem('harness.task'),null);
  globalThis.fetch=async()=>new Response(JSON.stringify({data:{owner:{id:'legacy-owner',name:'Local'},csrfToken:'csrf',identityMode:'local'}}),{status:200,headers:{'content-type':'application/json'}});
  try{
    await new HarnessApi().pair('pairing-code');
    assert.equal(cache.getItem('harness.task'),'task-a');
    assert.equal(cache.getItem('harness.scene'),'settings');
    assert.equal(cache.getItem('harness.draft.task-a'),'saved draft');
    cache.setItem('harness.task','task-b');assert.equal(raw.getItem('harness.task'),'task-b');
  }finally{globalThis.fetch=original;resetUserStorage();}
});
