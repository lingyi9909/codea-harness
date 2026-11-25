import {test} from 'node:test';
import assert from 'node:assert/strict';
import {HarnessApi} from '../../apps/web/src/api/client.ts';
import {configureUserStorage,scopedStorage,resetUserStorage} from '../../apps/web/src/userStorage.ts';

const fakeStorage=(values=new Map<string,string>()):Storage=>({
  get length(){return values.size;},
  key:i=>[...values.keys()][i]??null,
  getItem:k=>values.get(k)??null,
  setItem:(k,v)=>{values.set(k,v);},
  removeItem:k=>{values.delete(k);},
  clear:()=>values.clear(),
});

test('local and LAN browser caches are separately namespaced per authenticated owner',()=>{
  const backing=fakeStorage(new Map([
    ['harness.chat.history.legacy','a previous unsafe browser title'],
    ['harness.task','legacy review task'],
  ]));
  const session=scopedStorage(()=>backing);
  resetUserStorage();
  assert.equal(session.length,0,'never read browser storage before authentication');
  configureUserStorage('local:alice',false);
  assert.equal(session.getItem('harness.task'),null,'old unscoped data must not be exposed');
  session.setItem('harness.chat.title','Alice private conversation');
  session.setItem('harness.task','alice-task');
  configureUserStorage('local:bob',false);
  assert.equal(session.length,0,'new local owner must not inherit Alice browser history');
  assert.equal(session.getItem('harness.task'),null);
  session.setItem('harness.chat.title','Bob private conversation');
  configureUserStorage('lan:alice-1',true);
  assert.equal(session.length,0,'LAN owner must not inherit either local user');
  session.setItem('harness.chat.title','LAN owner 1');
  configureUserStorage('lan:alice-2',true);
  assert.equal(session.length,0,'new LAN login gets a fresh scope');
  configureUserStorage('local:alice',false);
  assert.equal(session.getItem('harness.chat.title'),'Alice private conversation');
  assert.equal(session.getItem('harness.task'),'alice-task');
  resetUserStorage();
  assert.equal(session.getItem('harness.chat.title'),null,'logout must hide all cached data');
});

test('bootstrap/pair assigns an owner scope without re-exposing pre-upgrade legacy caches',async()=>{
  const backing=fakeStorage(new Map([
    ['harness.scene','settings'],
    ['harness.task','task-from-another-user'],
    ['harness.draft.task-from-another-user','saved draft'],
  ]));
  const cache=scopedStorage(()=>backing),original=globalThis.fetch;
  resetUserStorage();
  globalThis.fetch=async()=>new Response(JSON.stringify({data:{
    owner:{id:'legacy-owner',name:'Local'},csrfToken:'csrf',identityMode:'local'
  }}),{status:200,headers:{'content-type':'application/json'}});
  try{
    await new HarnessApi().pair('pairing-code');
    assert.equal(cache.getItem('harness.task'),null);
    assert.equal(cache.getItem('harness.scene'),null);
    cache.setItem('harness.task','current-user-task');
    assert.equal(backing.getItem('harness.user.legacy-owner.harness.task'),'current-user-task');
    assert.equal(backing.getItem('harness.task'),'task-from-another-user','do not delete legacy data');
  }finally{globalThis.fetch=original;resetUserStorage();}
});
