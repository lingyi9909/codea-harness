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

import {summarizeFirstInput,displayChatTitle} from '../../apps/web/src/chatHistory.ts';

it('summarizes the requested task in the first input, including multiline and long requests',()=>{
  assert.equal(summarizeFirstInput('你好！\n背景：用户反馈页面空白。\n请帮我排查订单详情页白屏的原因。\n附录：'+'日志'.repeat(200)),
    '排查订单详情页白屏的原因');
  assert.equal(summarizeFirstInput('Please fix the broken login redirect.\nSee the attached logs.'),'Fix the broken login redirect');
  assert.equal(summarizeFirstInput('请帮我\n修复导出 PDF 时的乱码。'),'修复导出 PDF 时的乱码');
  const long=summarizeFirstInput('排查'+'🛠'.repeat(70));
  assert.ok(Array.from(long).length<=48&&long.endsWith('…')&&!long.includes('�'));
});
it('titles log requests from the question or filters and preserves existing session names',()=>{
  const log='请根据以下条件排查日志，列出证据、异常原因及下一步。\nTrace ID：req-23\n关键字：支付超时\n日志级别：WARN';
  assert.equal(summarizeFirstInput(log,true),'排查支付超时 · req-23');
  assert.equal(summarizeFirstInput(log+'\n排查问题：为什么订单重复扣款？\n导入日志：orders.txt',true),'为什么订单重复扣款');
  assert.equal(displayChatTitle('团队自定义名称','缓存的名称'),'团队自定义名称');
  assert.equal(displayChatTitle('Code Harness · 直接对话','以前的对话'),'以前的对话');
});
