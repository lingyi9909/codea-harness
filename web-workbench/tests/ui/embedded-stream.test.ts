import {test} from 'node:test';
import assert from 'node:assert/strict';
import {BearerEventSource} from '../../apps/web/src/api/activity-source.ts';
test('embedded streaming uses an Authorization header, parses chunked SSE, and never puts credentials in the URL',async()=>{
  const original=globalThis.fetch,controller=new AbortController();let url='',authorization='',credentials:RequestCredentials|undefined;
  const stream=new ReadableStream<Uint8Array>({start(c){const encoder=new TextEncoder();for(const part of ['event: rea','dy\ndata: {}\n\n','event: fragment\r\ndata: {"text":"仅本会话"}\r\n\r\n','data: {"id":"tool-1"}\n\n'])c.enqueue(encoder.encode(part));}});
  globalThis.fetch=async(input,init)=>{url=String(input);authorization=(init?.headers as Record<string,string>).Authorization;credentials=init?.credentials;return new Response(stream,{headers:{'content-type':'text/event-stream'}});};
  const source=new BearerEventSource('/api/v1/embed/chat/sessions/own/activity',{Authorization:'Bearer private-capability'});
  try{
    const values:string[]=[];
    await new Promise<void>((resolve,reject)=>{
      const timeout=setTimeout(()=>reject(Error('SSE_TIMEOUT')),2000);
      const append=(e:MessageEvent<string>)=>{values.push(e.data);if(values.length===3){clearTimeout(timeout);resolve();}};
      source.addEventListener('ready',append);source.addEventListener('fragment',append);source.onmessage=append;
    });
    assert.deepEqual(values,['{}','{"text":"仅本会话"}','{"id":"tool-1"}']);assert.equal(url.includes('private-capability'),false);assert.equal(authorization,'Bearer private-capability');assert.equal(credentials,'omit');
  }finally{source.close();controller.abort();globalThis.fetch=original;}
});
test('embedded streaming reconnects after a temporary HTTP 503',async()=>{
  const original=globalThis.fetch;let calls=0;
  globalThis.fetch=async()=>{calls++;if(calls===1)return new Response('temporarily unavailable',{status:503});return new Response(new ReadableStream({start(c){c.enqueue(new TextEncoder().encode('event: ready\ndata: {}\n\n'));}}),{headers:{'content-type':'text/event-stream'}});};
  const source=new BearerEventSource('/api/v1/embed/chat/sessions/own/activity',{Authorization:'Bearer capability'});
  try{await new Promise<void>((resolve,reject)=>{const timeout=setTimeout(()=>reject(Error('RECONNECT_TIMEOUT')),5000);source.addEventListener('ready',()=>{clearTimeout(timeout);resolve();});});assert.equal(calls,2);}finally{source.close();globalThis.fetch=original;}
});
test('an ended embedded API cannot fall back to the workbench cookie APIs',async()=>{
  const {HarnessApi}=await import('../../apps/web/src/api/client.ts');const original=globalThis.fetch;let calls=0;
  globalThis.fetch=async()=>{calls++;return new Response(JSON.stringify({data:{token:'temporary',csrfToken:'csrf',expiresAt:Date.now()+60000}}),{headers:{'content-type':'application/json'}});};
  try{const api=new HarnessApi();await api.openAnonymousEmbedded();api.clearEmbedded();await assert.rejects(api.chatModels(),/本次排查会话已结束/);assert.throws(()=>api.activity('own'),/本次排查会话已结束/);assert.equal(calls,1);}finally{globalThis.fetch=original;}
});

test('embedded SSE parses authoritative native lifecycle without exposing it as chat text',async()=>{
  const original=globalThis.fetch;
  const encoded=new TextEncoder().encode('event: lifecycle\ndata: {"id":"native-finished","status":"succeeded","at":123}\n\n');
  globalThis.fetch=async()=>new Response(new ReadableStream<Uint8Array>({start(c){c.enqueue(encoded);}}),
    {headers:{'content-type':'text/event-stream'}});
  const source=new BearerEventSource('/api/v1/embed/chat/sessions/one/activity',{Authorization:'Bearer capability'});
  try{
    const value=await new Promise<{status:string;id:string;at:number}>((resolve,reject)=>{
      const timeout=setTimeout(()=>reject(new Error('LIFECYCLE_TIMEOUT')),2000);
      source.addEventListener('lifecycle',event=>{clearTimeout(timeout);resolve(JSON.parse(event.data));});
    });
    assert.deepEqual(value,{id:'native-finished',status:'succeeded',at:123});
  }finally{source.close();globalThis.fetch=original;}
});
