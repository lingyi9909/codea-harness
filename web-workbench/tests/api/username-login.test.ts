import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm,readFile} from 'node:fs/promises';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {createApp} from '../../apps/server/src/app.ts';
import {LocalAuth} from '../../apps/server/src/auth.ts';

const port=4191,host='127.0.0.1:'+port,origin='http://'+host;
test('username only login accepts English or alphanumeric, rejects other names and logs out',async t=>{
  const workspace=await mkdtemp(join(tmpdir(),'username-login-'));
  t.after(()=>rm(workspace,{recursive:true,force:true}));
  const app=await createApp({auth:new LocalAuth('unused',port),localOnly:true,workspace});
  t.after(()=>app.close());
  const login=(name:unknown)=>app.inject({method:'POST',url:'/api/v1/auth/local-session',headers:{host,origin},payload:{name}});
  for(const name of ['zhang','zhang01','A','01zhang']){
    const opened=await login(name);
    assert.equal(opened.statusCode,200,opened.body);
    const cookie=String(opened.headers['set-cookie']).split(';')[0],csrf=opened.json().data.csrfToken;
    const owner=opened.json().data.owner;
    assert.equal(owner.name,name);
    assert.deepEqual(JSON.parse(await readFile(join(workspace,'.workbench/user.json'),'utf8')),{name});
    assert.equal((await app.inject({method:'DELETE',url:'/api/v1/auth/session',headers:{host,origin,cookie,'x-csrf-token':csrf}})).statusCode,204);
    assert.equal((await app.inject({url:'/api/v1/bootstrap',headers:{host,cookie}})).statusCode,401);
    assert.deepEqual((await login(name)).json().data.owner,owner);
  }
  for(const name of ['123','张三','abc_def','abc-def','abc def','',' abc','abc ','Ａbc','a'.repeat(81),null]){
    const bad=await login(name);
    assert.equal(bad.statusCode,400,JSON.stringify(name)+': '+bad.body);
    assert.equal(bad.headers['set-cookie'],undefined);
  }
});
