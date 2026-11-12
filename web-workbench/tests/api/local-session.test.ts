import assert from 'node:assert/strict';
import { it } from 'node:test';
import { createApp } from '../../apps/server/src/app.ts';
import { LocalAuth } from '../../apps/server/src/auth.ts';

const port=4181,host='127.0.0.1:'+port,origin='http://'+host;
const bootstrap='/api/v1/bootstrap',open='/api/v1/auth/local-session';
it('packaged local sessions require a loopback Host and matching Origin, reuse their cookie and retain CSRF checks',async()=>{
  const app=await createApp({auth:new LocalAuth('unused',port),localOnly:true});
  try{
    const first=await app.inject({url:bootstrap,headers:{host}});
    assert.equal(first.statusCode,401);
    assert.equal(first.json().error.code,'LOCAL_SESSION_REQUIRED');
    for(const headers of [{host},{host,origin:'http://localhost:'+port},{host:'example.com:'+port,origin:'http://example.com:'+port}]){
      const denied=await app.inject({method:'POST',url:open,headers});
      assert.equal(denied.statusCode,403);
      assert.equal(denied.headers['set-cookie'],undefined);
    }
    const opened=await app.inject({method:'POST',url:open,headers:{host,origin}});
    assert.equal(opened.statusCode,200);
    const setCookie=String(opened.headers['set-cookie']);
    assert.match(setCookie,/HttpOnly; SameSite=Strict/);
    const cookie=setCookie.split(';')[0],csrf=opened.json().data.csrfToken;
    const reused=await app.inject({method:'POST',url:open,headers:{host,origin,cookie}});
    assert.equal(reused.headers['set-cookie'],undefined);
    assert.equal(reused.json().data.csrfToken,csrf);
    const refreshed=await app.inject({url:bootstrap,headers:{host,cookie}});
    assert.equal(refreshed.json().data.csrfToken,csrf);
    const logout='/api/v1/auth/session';
    for(const headers of [{host,origin,cookie},{host,origin:'http://example.com',cookie,'x-csrf-token':csrf}]){
      assert.equal((await app.inject({method:'DELETE',url:logout,headers})).statusCode,403);
    }
    assert.equal((await app.inject({method:'DELETE',url:logout,headers:{host,origin,cookie,'x-csrf-token':csrf}})).statusCode,204);
    assert.equal((await app.inject({url:bootstrap,headers:{host,cookie}})).statusCode,401);
  }finally{await app.close();}
});
it('the default server requires the pairing code and does not register automatic session creation',async()=>{
  const app=await createApp({auth:new LocalAuth('fixture',port)});
  try{
    assert.equal((await app.inject({method:'POST',url:open,headers:{host,origin}})).statusCode,404);
    assert.equal((await app.inject({url:bootstrap,headers:{host}})).json().error.code,'AUTH_REQUIRED');
    const paired=await app.inject({method:'POST',url:'/api/v1/auth/session',headers:{host,origin},payload:{pairingCode:'fixture'}});
    assert.equal(paired.statusCode,200);
  }finally{await app.close();}
});
