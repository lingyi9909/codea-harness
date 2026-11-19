import {test} from 'node:test';
import assert from 'node:assert/strict';
import {singleServicePermissions} from '../../apps/server/src/execution-registry.ts';
import {sqliteEmbedFixture} from '../support/sqlite-embed-fixture.ts';

/** The official V2 permission contract uses the LAST matching rule. */
function allowed(action:string){
  const matching=singleServicePermissions.filter(rule=>
    rule.action==='*'||rule.action===action);
  return matching.at(-1)?.effect==='allow';
}
test('anonymous shared service grants only the existing Skill and scoped DBHub query tools',()=>{
  for(const action of ['skill','dbhub_search_schema','dbhub_execute_sql'])assert.equal(allowed(action),true,action);
  for(const action of ['shell','edit','read','glob','grep','subagent','execute',
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
