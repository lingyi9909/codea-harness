import {test} from 'node:test';
import assert from 'node:assert/strict';
import {embedFixture} from '../support/embed-fixture.ts';
import {EmbeddedAuth,embedDigest} from '../../apps/server/src/embed-auth.ts';
test('embedded: three users execute concurrently; all cross-grant operations fail, including a second window of the same user',async()=>{
  const f=await embedFixture();
  try{
    const visits=await Promise.all(['alice','bob','carol','alice'].map(f.visit));
    const ids=await Promise.all(visits.map(async v=>{const r=await f.app.inject({method:'POST',url:'/api/v1/embed/chat/sessions',headers:f.headers(v.token,v.csrfToken),payload:{scenario:'message-inspection'}});assert.equal(r.statusCode,201,r.body);return r.json().data.id;}));
    assert.equal(new Set(ids).size,4);
    for(const victim of [1,3])for(const [method,suffix,payload]of [
      ['GET','',undefined],['PATCH','',{title:'attack'}],['DELETE','',undefined],['POST','/messages',{text:'attack'}],['POST','/interrupt',{}],['GET','/activity',undefined],
      ['POST','/permission',{requestId:'p',revision:'a'.repeat(64),decision:'once'}],['POST','/form',{requestId:'f',revision:'a'.repeat(64),decision:'reply',answer:{}}],
    ]as const){const r=await f.app.inject({method,url:'/api/v1/embed/chat/sessions/'+ids[victim]+suffix,headers:f.headers(visits[0].token,visits[0].csrfToken),payload});assert.equal(r.statusCode,404,method+suffix+': '+r.body);}
    assert.equal(f.calls.length,0);
    await Promise.all(visits.slice(0,3).map(async(v,i)=>{const r=await f.app.inject({method:'POST',url:'/api/v1/embed/chat/sessions/'+ids[i]+'/messages',headers:f.headers(v.token,v.csrfToken),payload:{text:'trace-'+i,scenario:'message-inspection'}});assert.equal(r.statusCode,202,r.body);}));
    assert.equal(f.calls.length,3);assert.ok(ids.slice(0,3).every(id=>f.active[id]));
    const sameUser=await f.app.inject({method:'POST',url:'/api/v1/embed/chat/sessions/'+ids[3]+'/messages',headers:f.headers(visits[3].token,visits[3].csrfToken),payload:{text:'second page'}});
    assert.equal(sameUser.statusCode,409,sameUser.body);
    const end=await f.app.inject({method:'DELETE',url:'/api/v1/embed/session',headers:f.headers(visits[0].token,visits[0].csrfToken)});assert.equal(end.statusCode,204,end.body);
    assert.equal(f.active[ids[0]],false);assert.equal(f.active[ids[1]],true);assert.equal(f.active[ids[2]],true);
    assert.equal((await f.app.inject({url:'/api/v1/embed/session',headers:f.headers(visits[0].token)})).statusCode,401);
    assert.equal((await f.app.inject({url:'/api/v1/embed/session',headers:f.headers(visits[3].token)})).statusCode,200);
    const afterStop=await f.app.inject({method:'POST',url:'/api/v1/embed/chat/sessions/'+ids[3]+'/messages',headers:f.headers(visits[3].token,visits[3].csrfToken),payload:{text:'own second page'}});assert.equal(afterStop.statusCode,202,afterStop.body);
    assert.equal((await f.app.inject({url:'/api/v1/bootstrap',headers:f.headers(visits[1].token)})).statusCode,401);
  }finally{await f.close();}
});
test('embedded: expired grant cleanup stops only its native session',{timeout:25000},async()=>{
  const f=await embedFixture();
  try{
    const visits=await Promise.all(['alice','bob'].map(f.visit));
    const ids=await Promise.all(visits.map(async v=>{const r=await f.app.inject({method:'POST',url:'/api/v1/embed/chat/sessions',headers:f.headers(v.token,v.csrfToken),payload:{scenario:'message-inspection'}});return r.json().data.id;}));
    for(let i=0;i<2;i++){const r=await f.app.inject({method:'POST',url:'/api/v1/embed/chat/sessions/'+ids[i]+'/messages',headers:f.headers(visits[i].token,visits[i].csrfToken),payload:{text:'running'}});assert.equal(r.statusCode,202,r.body);}
    const grant=f.store.grants.get(embedDigest(visits[0].token))!;grant.expires=0;
    assert.equal((await f.app.inject({url:'/api/v1/embed/session',headers:f.headers(visits[0].token)})).statusCode,401);
    const deadline=Date.now()+20000;
    while(f.nativeSessions.has(ids[0])&&Date.now()<deadline)await new Promise(resolve=>setTimeout(resolve,100));
    assert.equal(f.nativeSessions.has(ids[0]),false);assert.equal(f.active[ids[1]],true);assert.ok(f.store.clean.has(grant.id));
  }finally{await f.close();}
});
test('embedded: one-use expiring host tickets, no browser secrets, bound scenario and directory, no history or duplicate creation',async()=>{
  const f=await embedFixture();
  try{
    const ticket=await f.ticket('alice');
    const exchange=()=>f.app.inject({method:'POST',url:'/api/v1/embed/exchange',headers:f.headers(),payload:{ticket:ticket.ticket,parentOrigin:'https://host.example'}});
    const responses=await Promise.all([exchange(),exchange()]);assert.deepEqual(responses.map(r=>r.statusCode).sort(),[200,401]);
    const v=responses.find(r=>r.statusCode===200)!.json().data;
    assert.equal(responses[0].headers['set-cookie'],undefined);
    const expired=await f.ticket('bob');f.store.tickets.get(embedDigest(expired.ticket))!.expires=0;
    assert.equal((await f.app.inject({method:'POST',url:'/api/v1/embed/exchange',headers:f.headers(),payload:{ticket:expired.ticket,parentOrigin:'https://host.example'}})).statusCode,401);
    const browserMint=await f.app.inject({method:'POST',url:'/api/v1/embed/tickets',headers:{...f.headers(),authorization:'Bearer '+f.config.clients[0].secret},payload:{clientId:'host',subject:'alice'}});assert.equal(browserMint.statusCode,403);
    assert.equal((await f.app.inject({method:'POST',url:'/api/v1/embed/chat/history',headers:f.headers(v.token,v.csrfToken),payload:{scenario:'message-inspection'}})).statusCode,403);
    assert.equal((await f.app.inject({method:'POST',url:'/api/v1/embed/chat/models',headers:f.headers(v.token,v.csrfToken),payload:{directory:f.records[1].directories[f.workspace]}})).statusCode,403);
    assert.equal((await f.app.inject({method:'POST',url:'/api/v1/embed/chat/sessions',headers:f.headers(v.token,'wrong'),payload:{scenario:'message-inspection'}})).statusCode,403);
    const created=await f.app.inject({method:'POST',url:'/api/v1/embed/chat/sessions',headers:f.headers(v.token,v.csrfToken),payload:{scenario:'message-inspection'}});assert.equal(created.statusCode,201,created.body);
    assert.equal((await f.app.inject({method:'POST',url:'/api/v1/embed/chat/sessions',headers:f.headers(v.token,v.csrfToken),payload:{scenario:'message-inspection'}})).statusCode,409);
    const id=created.json().data.id;
    assert.equal((await f.app.inject({method:'POST',url:'/api/v1/embed/chat/sessions/'+id+'/messages',headers:f.headers(v.token,v.csrfToken),payload:{text:'attack',files:['secret.txt']}})).statusCode,403);
    f.forms.set(id,[{id:'otp-form',sessionID:id,title:'认证',fields:[{key:'otp',title:'验证码',type:'string',required:true,default:'must-not-leak'}]}]);
    const secretForm=await f.app.inject({url:'/api/v1/embed/chat/sessions/'+id,headers:f.headers(v.token)});
    assert.equal(secretForm.statusCode,409,secretForm.body);assert.equal(secretForm.json().error.code,'EMBED_BUSINESS_AUTH_REQUIRED');assert.equal(secretForm.body.includes('must-not-leak'),false);
    assert.equal((await f.app.inject({url:'/api/v1/embed/config?parentOrigin=https%3A%2F%2Fevil.example',headers:f.headers()})).statusCode,403);
    assert.equal((await f.app.inject({url:'/api/v1/embed/session',headers:{...f.headers(v.token),origin:'https://evil.example'}})).statusCode,403);
    const page=await f.app.inject({url:'/embed/message',headers:{host:'workbench.example'}});
    if(page.statusCode===200)assert.equal(page.headers['content-security-policy'],"frame-ancestors 'self' https://host.example");
    const restarted=new EmbeddedAuth(f.config,f.store);assert.equal((await restarted.session({headers:f.headers(v.token)} as never)).ownerId,f.owners[0]);
    const g=f.store.grants.get(embedDigest(v.token))!;f.active[id]=true;
    f.nativeSessions.get(id).metadata.harnessOwner=f.owners[1];
    assert.equal((await f.app.inject({method:'DELETE',url:'/api/v1/embed/session',headers:f.headers(v.token,v.csrfToken)})).statusCode,204);
    assert.equal(f.active[id],true,'cleanup must not interrupt a metadata-mismatched session');assert.equal(f.store.clean.has(g.id),false);
  }finally{await f.close();}
});
