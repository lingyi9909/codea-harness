import assert from 'node:assert/strict';
import {it} from 'node:test';
import {rememberChat,readHistory,savedChat,activateChat,forgetChat,uniqueChatTitle} from '../../apps/web/src/chatHistory.ts';

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
  assert.equal(summarizeFirstInput(log,true),'排查 · Trace ID req-23 · 关键词 支付超时');
  assert.equal(summarizeFirstInput(log+'\n排查问题：为什么订单重复扣款？\n导入日志：orders.txt',true),'为什么订单重复扣款 · Trace ID req-23');
  assert.equal(summarizeFirstInput(log+'\nmsgSn：0000456\n排查问题：核查失败',true),'核查失败 · msgSn 0000456');
  assert.equal(displayChatTitle('团队自定义名称','缓存的名称'),'团队自定义名称');
  assert.equal(displayChatTitle('Code Harness · 直接对话','以前的对话'),'以前的对话');
  assert.equal(displayChatTitle('智能工作台 · 专项场景','该用户真实的排查标题'),'该用户真实的排查标题');
});

it('uniquifies repeated generated titles and forgets only the deleted session',()=>{
  const durable=new MemoryStorage(),tab=new MemoryStorage(),scene='harness.logs',directory='/a';
  rememberChat(durable,scene,directory,{id:'first',title:'重复标题',updated:100});
  rememberChat(durable,scene,directory,{id:'second',title:'重复标题 · 2',updated:200});
  rememberChat(durable,scene,'/elsewhere',{id:'first',title:'仍在',updated:300});
  assert.equal(uniqueChatTitle('重复标题',readHistory(durable,scene,directory)),'重复标题 · 3');
  activateChat(durable,tab,scene,directory,'first');
  tab.setItem('harness.logs.composer.'+JSON.stringify([directory,'first']),JSON.stringify({text:'草稿',files:''}));
  tab.setItem('harness.logs.filters.first','{}');
  tab.setItem('harness.logs.range.first','{}');
  forgetChat(durable,tab,scene,directory,'first');
  assert.deepEqual(readHistory(durable,scene,directory).map(row=>row.id),['second']);
  assert.equal(savedChat(durable,tab,scene,directory),'');
  assert.deepEqual(readHistory(durable,scene,'/elsewhere').map(row=>row.id),['first']);
  assert.equal(tab.getItem('harness.logs.filters.first'),null);
  assert.equal(tab.getItem('harness.logs.range.first'),null);
  assert.equal(tab.getItem('harness.logs.composer.'+JSON.stringify([directory,'first'])),null);
});
