import {test} from 'node:test';
import assert from 'node:assert/strict';
import {requireChatBinding,matchNativeBinding} from '../../apps/server/src/owned-chat.ts';
import {configureUserStorage,resetUserStorage,scopedStorage} from '../../apps/web/src/userStorage.ts';
import type {ChatBindingStore,ChatBinding} from '../../packages/storage/src/chat-bindings.ts';

test('T2: unbound, foreign, deleted and metadata-inconsistent chats fail closed',async()=>{
  const binding:ChatBinding={ownerId:'A',id:'native-a',serviceIdentity:'service-a',scenario:'direct-chat',directory:'/isolated/a'};
  let active=true;
  const store:ChatBindingStore={create:async()=>{},remove:async()=>{active=false;},get:async(service,id,owner)=>active&&service===binding.serviceIdentity&&id===binding.id&&owner===binding.ownerId?binding:undefined};
  assert.equal((await requireChatBinding(store,'service-a','native-a','A')).ownerId,'A');
  for(const [service,id,owner]of [['service-a','native-a','B'],['service-b','native-a','A'],['service-a','unknown','A']])await assert.rejects(requireChatBinding(store,service,id,owner),/CHAT_NOT_FOUND/);
  const native={id:'native-a',metadata:{harnessOwner:'A',harnessScenario:'direct-chat'},location:{directory:'/isolated/a'}};
  matchNativeBinding(binding,native);
  assert.throws(()=>matchNativeBinding(binding,{...native,metadata:{...native.metadata,harnessOwner:'B'}}),/CHAT_NOT_FOUND/);
  assert.throws(()=>matchNativeBinding(binding,{...native,location:{directory:'/shared'}}),/CHAT_NOT_FOUND/);
  await store.remove('service-a','native-a','A');await assert.rejects(requireChatBinding(store,'service-a','native-a','A'),/CHAT_NOT_FOUND/);
});
test('T2: browser switches A/B/C without reading legacy or foreign drafts/history/preferences',()=>{
  const map=new Map<string,string>(),raw:Storage={get length(){return map.size;},key:i=>[...map.keys()][i]??null,getItem:k=>map.get(k)??null,setItem:(k,v)=>{map.set(k,v);},removeItem:k=>{map.delete(k);},clear:()=>map.clear()};
  const cache=scopedStorage(()=>raw);resetUserStorage();raw.setItem('harness.chat','legacy');assert.equal(cache.getItem('harness.chat'),null);
  for(const owner of ['A','B','C']){configureUserStorage(owner,true);assert.equal(cache.getItem('harness.chat'),null);cache.setItem('harness.chat',owner);cache.setItem('harness.draft',owner+' draft');}
  configureUserStorage('A',true);assert.equal(cache.getItem('harness.chat'),'A');assert.equal(cache.length,2);cache.clear();
  configureUserStorage('B',true);assert.equal(cache.getItem('harness.draft'),'B draft');resetUserStorage();assert.equal(cache.length,0);
  configureUserStorage('local-workbench',false);assert.equal(cache.getItem('harness.chat'),'legacy');resetUserStorage();
});
