import assert from 'node:assert/strict';
import {it} from 'node:test';
import {rememberChat,readHistory,savedChat,activateChat} from '../../apps/web/src/chatHistory.ts';

class MemoryStorage{
  private items=new Map<string,string>();
  get length(){return this.items.size;}
  key(index:number){return Array.from(this.items.keys())[index]??null;}
  getItem(key:string){return this.items.get(key)??null;}
  setItem(key:string,value:string){this.items.set(key,value);}
  removeItem(key:string){this.items.delete(key);}
  clear(){this.items.clear();}
}
it('keeps previous chats when creating a new one and restores the active chat per project and scene',()=>{
  const durable=new MemoryStorage(),tab=new MemoryStorage();
  tab.setItem('harness.chat','old');
  assert.equal(savedChat(durable,tab,'harness.chat','/a'),'old');
  for(const id of ['old','new']){
    rememberChat(durable,'harness.chat','/a',{id,title:id,updated:id==='old'?1:2});
    activateChat(durable,tab,'harness.chat','/a',id);
  }
  activateChat(durable,tab,'harness.chat','/a','');
  assert.deepEqual(readHistory(durable,'harness.chat','/a').map(c=>c.id),['new','old']);
  assert.deepEqual(readHistory(durable,'harness.logs','/a'),[]);
  assert.deepEqual(readHistory(durable,'harness.chat','/b'),[]);
  activateChat(durable,tab,'harness.chat','/a','old');
  assert.equal(savedChat(durable,tab,'harness.chat','/b'),'');
  assert.equal(savedChat(durable,new MemoryStorage(),'harness.chat','/a'),'old');
});
