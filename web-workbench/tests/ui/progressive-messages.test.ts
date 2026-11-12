import assert from 'node:assert/strict';
import {it} from 'node:test';
import type {ChatFragment,ChatMessage} from '../../packages/contracts/src/chat.ts';
import {MessageReveal,TextReveal} from '../../apps/web/src/progressiveMessages.ts';
import {applyFragment,streamingMessages} from '../../apps/web/src/chatStream.ts';

const full='检查👩🏽‍💻程序 e\u0301 🇨🇳。';
function assistant(id:string,text:string,complete:boolean):ChatMessage{
  return {id,role:'assistant',text:'',files:[],tools:[],complete,steps:[
    {kind:'thinking',text,hasText:!!text,ordinal:0,complete},
    {kind:'text',text,ordinal:1}
  ]};
}
function texts(messages:ChatMessage[]){
  return messages.flatMap(m=>m.steps?.flatMap(s=>s.kind==='text'||s.kind==='thinking'?[s.text]:[])??[]);
}

it('reveals both public fields across live, stale and final polls and paints history immediately',()=>{
  const reel=new MessageReveal();
  const history=[assistant('history',full,true)];
  reel.update(history,history,false,false);
  assert.deepEqual(texts(reel.project()),[full,full]);
  const live=[...history,assistant('current',full,false)];
  reel.update(live,live,true,false);
  reel.advance(48);
  assert.deepEqual(texts(reel.project()).slice(2),['检查','检查']);
  const stale=[...history,assistant('current','检',false)];
  reel.update(stale,stale,true,false);
  assert.deepEqual(texts(reel.project()).slice(2),['检查','检查']);
  const final=[...history,assistant('current',full+'结束。',true)];
  reel.update(final,final,false,false);
  assert.deepEqual(texts(reel.project()).slice(2),['检查','检查']);
  reel.advance(Infinity);
  assert.deepEqual(texts(reel.project()).slice(2),[full+'结束。',full+'结束。']);
  assert.equal(reel.pending,false);
});

it('keeps grapheme clusters intact and catches up to large chunks',()=>{
  for(const content of [full,full.repeat(1000)]){
    const c=new TextReveal();
    const boundaries=new Set(Array.from(new Intl.Segmenter().segment(content),x=>x.index+x.segment.length));
    c.update(content);
    for(let i=0;i<40;i++){c.advance(24);assert.ok(boundaries.has(c.offset));}
    assert.equal(c.offset,content.length);
  }
});

it('preserves genuine live fields across a shorter final snapshot and avoids duplicate parts',()=>{
  const received={message:'reply',ordinal:0,kind:'thinking' as const,text:full,at:1,ended:2};
  const short=assistant('reply','检查',true);
  assert.deepEqual(texts(streamingMessages([short],[received])),[full,'检查']);
  const longer=assistant('reply',full+'写完了',false);
  assert.deepEqual(texts(streamingMessages([longer],[received])),[full+'写完了',full+'写完了']);
  const corrected=assistant('reply','修正。',true);
  assert.deepEqual(texts(streamingMessages([corrected],[received])),['修正。','修正。']);
});

it('keeps final-only text moving for a freshly created chat and drains it when motion is disabled',()=>{
  const reel=new MessageReveal();
  const final=[assistant('new',full,true)];
  reel.update(final,final,false,true);
  assert.ok(reel.pending);
  reel.advance(24);
  assert.deepEqual(texts(reel.project()),['检','检']);
  reel.advance(Infinity);
  assert.deepEqual(texts(reel.project()),[full,full]);
});

it('shows delta-only text and fills transport gaps from overlapping snapshots without replay',()=>{
  const f=(phase:ChatFragment['phase'],text:string):ChatFragment=>({id:text,message:'reply',at:1,phase,text,kind:'thinking',ordinal:0});
  const joining=applyFragment(undefined,f('delta','查找'));
  assert.equal(texts(streamingMessages([], [joining]))[0],'…查找');
  const tail=applyFragment(joining,f('delta','完成'));
  const snapshot=assistant('reply','准备查找',false);
  assert.equal(texts(streamingMessages([snapshot],[tail]))[0],'准备查找完成');
  const ended=applyFragment(tail,f('ended',''));
  assert.equal(texts(streamingMessages([assistant('reply','',true)],[ended]))[0],'…查找完成');
  const first=applyFragment(applyFragment(undefined,f('started','')),f('delta','准备'));
  const reconnected=applyFragment(first,f('delta','完成'),true);
  assert.equal(texts(streamingMessages([], [reconnected]))[0],'准备…完成');
  assert.equal(texts(streamingMessages([assistant('reply','准备查找完成。',true)],[reconnected]))[0],'准备查找完成。');
});

it('keeps the native snapshot type and timing when live text reuses a reasoning ordinal',()=>{
  const native=assistant('reply','公开摘要',true);
  native.steps=[
    {kind:'thinking',ordinal:0,text:'公开摘要',at:1000,ended:14250,complete:true,hasText:true},
    {kind:'text',ordinal:1,text:'中文回答。'}
  ];
  const stale={message:'reply',ordinal:0,kind:'text' as const,text:'公开摘要',at:900,ended:13000};
  const thinking={...stale,kind:'thinking' as const};
  for(const complete of [false,true]){
    const snapshot={...native,complete};
    for(const fragments of [[thinking,stale],[stale,thinking],[stale]]){
      const [merged]=streamingMessages([snapshot],fragments);
      assert.deepEqual(merged.steps?.filter(s=>s.kind==='text').map(s=>s.text),['中文回答。']);
      const thought=merged.steps?.find(s=>s.kind==='thinking');
      assert.equal(thought?.text,'公开摘要');
      assert.equal(thought?.at,1000);
      assert.equal(thought?.ended,14250);
    }
  }
});
