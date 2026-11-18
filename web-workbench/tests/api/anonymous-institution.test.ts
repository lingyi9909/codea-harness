import {test} from 'node:test';
import assert from 'node:assert/strict';
import {embedFixture} from '../support/embed-fixture.ts';
import {embedDigest} from '../../apps/server/src/embed-auth.ts';
const query={day:'2026-11-18',from:Date.UTC(2026,10,17,16),to:Date.UTC(2026,10,18,16),msgSn:'000123',account:'business-account',password:'private-password',verificationCode:'001234'};
test('anonymous: three visitors, no host integration, persistent restart, cross-session rejection and independent stop',async()=>{
  const f=await embedFixture(undefined,undefined,true,{anonymous:true,resetSlot:async()=>{}});
  try{
    const visits=await Promise.all([0,1,2].map(async()=>{const r=await f.app.inject({method:'POST',url:'/api/v1/embed/anonymous',headers:f.headers(),payload:{}});assert.equal(r.statusCode,200,r.body);assert.equal(r.headers['set-cookie'],undefined);return r.json().data;}));
    assert.equal(f.store.tickets.size,0);
    const ids=await Promise.all(visits.map(async v=>{const r=await f.app.inject({method:'POST',url:'/api/v1/embed/chat/sessions',headers:f.headers(v.token,v.csrfToken),payload:{scenario:'message-inspection'}});assert.equal(r.statusCode,201,r.body);return r.json().data.id;}));
    assert.equal(new Set(ids).size,3);assert.equal(new Set([...f.store.grants.values()].map(g=>g.ownerId)).size,3);
    await Promise.all(visits.map(async(v,i)=>assert.equal((await f.app.inject({method:'POST',url:'/api/v1/embed/chat/sessions/'+ids[i]+'/messages',headers:f.headers(v.token,v.csrfToken),payload:{text:'visitor-'+i}})).statusCode,202)));
    await f.restart();
    for(let i=0;i<3;i++){
      const r=await f.app.inject({url:'/api/v1/embed/session',headers:f.headers(visits[i].token)});assert.equal(r.statusCode,200,r.body);assert.equal(r.json().data.sessionId,ids[i]);
      assert.equal((await f.app.inject({url:'/api/v1/embed/chat/sessions/'+ids[i],headers:f.headers(visits[i].token)})).statusCode,200);
    }
    assert.equal(f.calls.length,3,'API restart must not replay prompts');
    for(const [method,suffix,payload]of [['GET','',undefined],['DELETE','',undefined],['PATCH','',{title:'foreign'}],['POST','/interrupt',{}],['POST','/messages',{text:'foreign'}],['POST','/permission',{requestId:'p',revision:'a'.repeat(64),decision:'once'}],['POST','/form',{requestId:'f',revision:'a'.repeat(64),decision:'reply',answer:{}}],['GET','/activity',undefined]]as const){
      const r=await f.app.inject({method,url:'/api/v1/embed/chat/sessions/'+ids[1]+suffix,headers:f.headers(visits[0].token,visits[0].csrfToken),payload});assert.equal(r.statusCode,404,method+suffix+':'+r.body);
    }
    assert.equal((await f.app.inject({method:'POST',url:'/api/v1/embed/anonymous',headers:f.headers(),payload:{}})).statusCode,503);
    assert.equal((await f.app.inject({method:'DELETE',url:'/api/v1/embed/session',headers:f.headers(visits[0].token,visits[0].csrfToken)})).statusCode,204);
    assert.equal(f.active[ids[0]],false);assert.equal(f.active[ids[1]],true);assert.equal(f.active[ids[2]],true);
    const next=await f.app.inject({method:'POST',url:'/api/v1/embed/anonymous',headers:f.headers(),payload:{}});assert.equal(next.statusCode,200,next.body);
    assert.notEqual(next.json().data.token,visits[0].token);assert.equal((await f.app.inject({url:'/api/v1/embed/session',headers:f.headers(next.json().data.token)})).json().data.sessionId,null);
    assert.equal((await f.app.inject({url:'/api/v1/embed/session',headers:f.headers(visits[0].token)})).statusCode,401);
    assert.equal((await f.app.inject({method:'POST',url:'/api/v1/embed/anonymous',headers:{...f.headers(),origin:'https://evil.example'},payload:{}})).statusCode,403);
    assert.equal((await f.app.inject({url:'/api/v1/embed/session?token='+visits[1].token,headers:{host:'workbench.example',cookie:'harness_session='+visits[1].token}})).statusCode,401);
  }finally{await f.close();}
});
test('anonymous: execution slots are not reused before a trusted reset; expired and retired grants survive a restart',async()=>{
  const f=await embedFixture(undefined,undefined,false,{anonymous:true});
  try{
    const visits=[];
    for(let i=0;i<3;i++){const r=await f.app.inject({method:'POST',url:'/api/v1/embed/anonymous',headers:f.headers(),payload:{}});visits.push(r.json().data);}
    const first=visits[0];assert.equal((await f.app.inject({method:'DELETE',url:'/api/v1/embed/session',headers:f.headers(first.token,first.csrfToken)})).statusCode,204);
    await f.restart();
    assert.equal((await f.app.inject({method:'POST',url:'/api/v1/embed/anonymous',headers:f.headers(),payload:{}})).statusCode,503);
    f.store.grants.get(embedDigest(visits[1].token))!.expires=0;
    assert.equal((await f.app.inject({url:'/api/v1/embed/session',headers:f.headers(visits[1].token)})).statusCode,401);
    assert.equal((await f.app.inject({method:'POST',url:'/api/v1/embed/anonymous',headers:f.headers(),payload:{}})).statusCode,503);
  }finally{await f.close();}
});
test('institution: credentials go only to the adapter; persisted evidence is reused after restart without reauthentication',async()=>{
  let calls=0;const f=await embedFixture(undefined,undefined,false,{anonymous:true,institutionAdapter:async(input,scope)=>{calls++;assert.deepEqual(input,query);assert.ok(scope.grantId);return {evidence:'已找到指定报文，其应答状态为失败。'};}});
  try{
    const v=(await f.app.inject({method:'POST',url:'/api/v1/embed/anonymous',headers:f.headers(),payload:{}})).json().data;
    const request=()=>f.app.inject({method:'POST',url:'/api/v1/embed/institution-query',headers:f.headers(v.token,v.csrfToken),payload:query});
    const results=await Promise.all([request(),request()]);assert.ok(results.some(r=>r.statusCode===200));assert.equal(calls,1);
    await f.restart();assert.equal((await request()).statusCode,200);assert.equal(calls,1);
    assert.equal((await f.app.inject({method:'POST',url:'/api/v1/embed/institution-query',headers:f.headers(v.token,v.csrfToken),payload:{...query,msgSn:'different'}})).statusCode,409);
    assert.equal((await f.app.inject({method:'POST',url:'/api/v1/embed/institution-query',headers:f.headers(v.token,'wrong'),payload:query})).statusCode,403);
    const serialized=JSON.stringify([...f.store.grants.values(),...f.store.queries.values(),...f.messages.values(),...results.map(r=>r.json())]);
    for(const secret of [query.password,query.verificationCode])assert.equal(serialized.includes(secret),false);
    assert.equal(f.calls.length,0,'querying evidence must not invoke the model');
  }finally{await f.close();}
});
test('institution: missing adapters, secret echoes and ambiguous/restarted attempts fail closed',async()=>{
  for(const mode of ['missing','echo','interrupted']as const){
    let calls=0;const f=await embedFixture(undefined,undefined,false,{anonymous:true,...(mode==='missing'?{}:{institutionAdapter:async()=>{calls++;return {evidence:'password='+query.password};}})});
    try{
      const v=(await f.app.inject({method:'POST',url:'/api/v1/embed/anonymous',headers:f.headers(),payload:{}})).json().data;
      if(mode==='interrupted'){
        const grant=f.store.grants.get(embedDigest(v.token))!;
        f.store.queries.set(grant.id,{state:'RUNNING',fingerprint:'unknown'});await f.restart();
      }
      const r=await f.app.inject({method:'POST',url:'/api/v1/embed/institution-query',headers:f.headers(v.token,v.csrfToken),payload:query});assert.equal(r.statusCode,mode==='missing'?503:mode==='echo'?502:409);
      assert.equal(r.body.includes(query.password),false);assert.equal(f.calls.length,0);assert.equal(calls,mode==='echo'?1:0);
    }finally{await f.close();}
  }
});
