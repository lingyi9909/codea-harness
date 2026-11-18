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
