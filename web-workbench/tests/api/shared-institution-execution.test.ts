import {test} from 'node:test';
import assert from 'node:assert/strict';
import {singleServicePermissions} from '../../apps/server/src/execution-registry.ts';
import {sqliteEmbedFixture} from '../support/sqlite-embed-fixture.ts';
import {SceneConfigStore} from '../../apps/server/src/scene-config.ts';
import {mkdir} from 'node:fs/promises';
import {join} from 'node:path';

/** Simplified OpenCode V2 ordered rules for known test patterns. */
function allowed(action:string,resource='*'){
  let effect:'deny'|'allow'|'ask'='deny';
  for(const rule of singleServicePermissions){
    if(rule.action!=='*'&&rule.action!==action)continue;
    const regex='^'+rule.resource.split('*')
      .map(piece=>[...piece].map(ch=>'.+?^$()[]{}|\\'.includes(ch)?'\\'+ch:ch).join('')).join('.*')+'$';
    if(new RegExp(regex).test(resource))effect=rule.effect;
  }
  return effect==='allow';
}
test('anonymous shared service permits scoped read-only discovery but never arbitrary Shell or code execution',()=>{
  for(const action of ['skill','dbhub_search_schema','dbhub_execute_sql'])
    assert.equal(allowed(action),true,action);
  for(const command of ['teamai recall 业务知识','teamai search 机构报文',
    'git status --short','git ls-files src'])
    assert.equal(allowed('shell',command),true,command);
  for(const command of ['powershell -Command Get-ChildItem','node script.js',
    'curl https://example.com','teamai publish secrets','teamai delete all',
    'git push origin main','rm -rf .','python test.py'])
    assert.equal(allowed('shell',command),false,command);
  for(const action of ['edit','read','glob','grep','subagent','execute',
    'external_directory','webfetch','dbhub_write_table','other_mcp_delete'])
    assert.equal(allowed(action),false,action);
});

test('three anonymous institution pages run existing message Skill on ONE OpenCode service with independent sessions',async()=>{
  const f=await sqliteEmbedFixture(undefined,undefined,true);
  try{
    const users=await Promise.all(Array.from({length:3},async()=>{
      const response=await f.app.inject({method:'POST',url:'/api/v1/embed/anonymous',headers:f.headers(),payload:{}});
      assert.equal(response.statusCode,200,response.body);
      return response.json().data as {token:string;csrfToken:string};
    }));
    const ids:string[]=[];
    for(const user of users){
      const response=await f.app.inject({
        method:'POST',url:'/api/v1/embed/chat/sessions',headers:f.headers(user.token,user.csrfToken),
        payload:{scenario:'institution-message-inspection'}});
      assert.equal(response.statusCode,201,response.body);
      ids.push(response.json().data.id);
    }
    assert.equal(new Set(ids).size,3);
    const native=[...f.nativeSessions.values()];
    assert.equal(native.length,3);
    for(const row of native){
      assert.deepEqual(row.permissions,singleServicePermissions);
      assert.equal(row.metadata.harnessScenario,'institution-message-inspection');
    }
    for(let i=0;i<3;i++){
      const response=await f.app.inject({
        method:'POST',url:'/api/v1/embed/chat/sessions/'+ids[i]+'/messages',
        headers:f.headers(users[i].token,users[i].csrfToken),
        payload:{scenario:'institution-message-inspection',text:'请排查报文 '+i}});
      assert.equal(response.statusCode,202,response.body);
    }
    assert.equal(f.prompts.length,3);
    for(const prompt of f.prompts){
      assert.deepEqual(prompt.skills,[{id:'message-skill'}],
        'no institution-specific duplicate setup; use configured message Skill');
    }
    for(let i=0;i<3;i++){
      const response=await f.app.inject({method:'GET',
        url:'/api/v1/embed/chat/sessions/'+ids[i],headers:f.headers(users[i].token)});
      assert.equal(response.statusCode,200,response.body);
      assert.equal(response.json().data.toolPolicy,'institution-readonly');
    }
    assert.equal((await f.app.inject({method:'POST',
      url:'/api/v1/embed/chat/sessions/'+ids[1]+'/interrupt',
      headers:f.headers(users[0].token,users[0].csrfToken),payload:{}})).statusCode,404);
    const stopped=await f.app.inject({method:'POST',
      url:'/api/v1/embed/chat/sessions/'+ids[0]+'/interrupt',
      headers:f.headers(users[0].token,users[0].csrfToken),payload:{}});
    assert.equal(stopped.statusCode,200,stopped.body);
    assert.equal(f.active[ids[0]],false);
    assert.equal(f.active[ids[1]],true);
    assert.equal(f.active[ids[2]],true);
  }finally{await f.close();}
});

test('the institution page inherits the existing message Skill directory, even if it differs from startup workspace',async()=>{
  const f=await sqliteEmbedFixture(undefined,undefined,false);
  try{
    const messageDirectory=join(f.workspace,'message-toolkit');
    await mkdir(messageDirectory);
    const config=new SceneConfigStore(f.workspace),original=config.read();
    const scenes={...original.config.scenes,message:{
      directory:messageDirectory,executor:{type:'skill' as const,name:'message-query'}}};
    await config.save({version:1,scenes},original.revision);
    const visitor=(await f.app.inject({method:'POST',url:'/api/v1/embed/anonymous',
      headers:f.headers(),payload:{}})).json().data;
    const created=await f.app.inject({
      method:'POST',url:'/api/v1/embed/chat/sessions',
      headers:f.headers(visitor.token,visitor.csrfToken),
      payload:{scenario:'institution-message-inspection'}});
    assert.equal(created.statusCode,201,created.body);
    const id=created.json().data.id;
    assert.equal(f.nativeSessions.get(id).location.directory,messageDirectory);
    const sent=await f.app.inject({method:'POST',
      url:'/api/v1/embed/chat/sessions/'+id+'/messages',
      headers:f.headers(visitor.token,visitor.csrfToken),
      payload:{scenario:'institution-message-inspection',text:'查询机构报文'}});
    assert.equal(sent.statusCode,202,sent.body);
    assert.deepEqual(f.prompts.at(-1)?.skills,[{id:'message-skill'}]);
    const forbidden=await f.app.inject({method:'POST',url:'/api/v1/embed/chat/models',
      headers:f.headers(visitor.token,visitor.csrfToken),
      payload:{directory:join(f.workspace,'not-authorized')}});
    assert.equal(forbidden.statusCode,403,forbidden.body);
  }finally{await f.close();}
});
).test(resource))effect=rule.effect;
  }
  return effect==='allow';
}
test('anonymous shared service permits scoped read-only discovery but never arbitrary Shell or code execution',()=>{
  for(const action of ['skill','dbhub_search_schema','dbhub_execute_sql'])
    assert.equal(allowed(action),true,action);
  for(const command of ['teamai recall 业务知识','teamai search 机构报文',
    'git status --short','git ls-files src']){
    assert.equal(allowed('shell',command),true,command);
  }
  for(const command of ['powershell -Command Get-ChildItem','node script.js',
    'curl https://example.com','teamai publish secrets','teamai delete all',
    'git push origin main','rm -rf .','python test.py'])
    assert.equal(allowed('shell',command),false,command);
  for(const action of ['edit','read','glob','grep','subagent','execute',
    'external_directory','webfetch','dbhub_write_table','other_mcp_delete'])
    assert.equal(allowed(action),false,action);
});

test('three anonymous institution pages run existing message Skill on ONE OpenCode service with independent sessions',async()=>{
  const f=await sqliteEmbedFixture(undefined,undefined,true);
  try{
    const users=await Promise.all(Array.from({length:3},async()=>{
      const response=await f.app.inject({method:'POST',url:'/api/v1/embed/anonymous',headers:f.headers(),payload:{}});
      assert.equal(response.statusCode,200,response.body);
      return response.json().data as {token:string;csrfToken:string};
    }));
    const ids:string[]=[];
    for(const user of users){
      const response=await f.app.inject({
        method:'POST',url:'/api/v1/embed/chat/sessions',headers:f.headers(user.token,user.csrfToken),
        payload:{scenario:'institution-message-inspection'}});
      assert.equal(response.statusCode,201,response.body);
      ids.push(response.json().data.id);
    }
    assert.equal(new Set(ids).size,3);
    const native=[...f.nativeSessions.values()];
    assert.equal(native.length,3);
    for(const row of native){
      assert.deepEqual(row.permissions,singleServicePermissions);
      assert.equal(row.metadata.harnessScenario,'institution-message-inspection');
    }
    for(let i=0;i<3;i++){
      const response=await f.app.inject({
        method:'POST',url:'/api/v1/embed/chat/sessions/'+ids[i]+'/messages',
        headers:f.headers(users[i].token,users[i].csrfToken),
        payload:{scenario:'institution-message-inspection',text:'请排查报文 '+i}});
      assert.equal(response.statusCode,202,response.body);
    }
    assert.equal(f.prompts.length,3);
    for(const prompt of f.prompts){
      assert.deepEqual(prompt.skills,[{id:'message-skill'}],
        'no institution-specific duplicate setup; use configured message Skill');
    }
    for(let i=0;i<3;i++){
      const response=await f.app.inject({method:'GET',
        url:'/api/v1/embed/chat/sessions/'+ids[i],headers:f.headers(users[i].token)});
      assert.equal(response.statusCode,200,response.body);
      assert.equal(response.json().data.toolPolicy,'institution-readonly');
    }
    assert.equal((await f.app.inject({method:'POST',
      url:'/api/v1/embed/chat/sessions/'+ids[1]+'/interrupt',
      headers:f.headers(users[0].token,users[0].csrfToken),payload:{}})).statusCode,404);
    const stopped=await f.app.inject({method:'POST',
      url:'/api/v1/embed/chat/sessions/'+ids[0]+'/interrupt',
      headers:f.headers(users[0].token,users[0].csrfToken),payload:{}});
    assert.equal(stopped.statusCode,200,stopped.body);
    assert.equal(f.active[ids[0]],false);
    assert.equal(f.active[ids[1]],true);
    assert.equal(f.active[ids[2]],true);
  }finally{await f.close();}
});

test('the institution page inherits the existing message Skill directory, even if it differs from startup workspace',async()=>{
  const f=await sqliteEmbedFixture(undefined,undefined,false);
  try{
    const messageDirectory=join(f.workspace,'message-toolkit');
    await mkdir(messageDirectory);
    const config=new SceneConfigStore(f.workspace),original=config.read();
    const scenes={...original.config.scenes,message:{
      directory:messageDirectory,executor:{type:'skill' as const,name:'message-query'}}};
    await config.save({version:1,scenes},original.revision);
    const visitor=(await f.app.inject({method:'POST',url:'/api/v1/embed/anonymous',
      headers:f.headers(),payload:{}})).json().data;
    const created=await f.app.inject({
      method:'POST',url:'/api/v1/embed/chat/sessions',
      headers:f.headers(visitor.token,visitor.csrfToken),
      payload:{scenario:'institution-message-inspection'}});
    assert.equal(created.statusCode,201,created.body);
    const id=created.json().data.id;
    assert.equal(f.nativeSessions.get(id).location.directory,messageDirectory);
    const sent=await f.app.inject({method:'POST',
      url:'/api/v1/embed/chat/sessions/'+id+'/messages',
      headers:f.headers(visitor.token,visitor.csrfToken),
      payload:{scenario:'institution-message-inspection',text:'查询机构报文'}});
    assert.equal(sent.statusCode,202,sent.body);
    assert.deepEqual(f.prompts.at(-1)?.skills,[{id:'message-skill'}]);
    const forbidden=await f.app.inject({method:'POST',url:'/api/v1/embed/chat/models',
      headers:f.headers(visitor.token,visitor.csrfToken),
      payload:{directory:join(f.workspace,'not-authorized')}});
    assert.equal(forbidden.statusCode,403,forbidden.body);
  }finally{await f.close();}
});
