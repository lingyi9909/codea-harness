import {test} from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {mkdtemp,mkdir,readFile,rm,realpath} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {createHash} from 'node:crypto';
import {DatabaseSync} from 'node:sqlite';
import {createApp} from '../../apps/server/src/app.ts';
import {LocalAuth} from '../../apps/server/src/auth.ts';
import {ExecutionRegistry} from '../../apps/server/src/execution-registry.ts';
import {EmbeddedAuth} from '../../apps/server/src/embed-auth.ts';
import {chatConnection} from '../../apps/server/src/modules/chat/connection.ts';
import {SqliteEmbedStore} from '../../packages/storage/src/sqlite-embed.ts';

test('one app: main and standalone pages share file-backed bindings; three visitors invoke the configured native Skill with ownership and permissions intact',{timeout:20000},async()=>{
  const root=await realpath(await mkdtemp(join(tmpdir(),'unified-pages-'))),workspace=join(root,'workspace');await mkdir(workspace);
  const sessions=new Map<string,any>(),active:Record<string,boolean>={},calls:{path:string;body:any}[]=[];let serial=0;
  const peer=createServer(async(req,res)=>{
    const path=new URL(req.url!,'http://localhost').pathname;let raw='';for await(const chunk of req)raw+=chunk;
    const body=raw?JSON.parse(raw):{};calls.push({path,body});res.setHeader('Content-Type','application/json');
    const send=(data:unknown)=>res.end(JSON.stringify({data}));
    if(path==='/api/model')return send([{id:'private',providerID:'company',enabled:true}]);
    if(path==='/api/skill')return res.end(JSON.stringify({location:{directory:workspace},data:[{id:'native-message-skill',name:'institution-query'}]}));
    if(path==='/api/session/active')return send(active);
    if(path==='/api/session'&&req.method==='POST'){const session={id:'session-'+(++serial),...body,time:{updated:Date.now()}};sessions.set(session.id,session);return send(session);}
    if(path.startsWith('/api/experimental/')){res.statusCode=204;return res.end();}
    const match=path.match(/^\/api\/session\/(session-\d+)(.*)$/);
    if(match){const [,id,suffix]=match;
      if(!sessions.has(id)){res.statusCode=404;return res.end('{}');}
      if(!suffix){if(req.method==='DELETE'){sessions.delete(id);res.statusCode=204;return res.end();}return send(sessions.get(id));}
      if(suffix==='/prompt'){active[id]=true;return send({id:'prompt-'+id});}
      if(suffix==='/interrupt'){active[id]=false;return res.end(JSON.stringify({interrupted:true}));}
      if(['/message','/inbox','/permission','/form'].includes(suffix))return send([]);
    }
    res.statusCode=404;res.end('{}');
  });
  await new Promise<void>(done=>peer.listen(0,'127.0.0.1',done));const url='http://127.0.0.1:'+(peer.address() as {port:number}).port;
  const before=process.env.HARNESS_OPENCODE_URL;process.env.HARNESS_OPENCODE_URL=url;
  const port=4196,origin='http://127.0.0.1:'+port,host='127.0.0.1:'+port,identity='single:'+createHash('sha256').update(url).digest('hex');
  const store=new SqliteEmbedStore(join(root,'private.sqlite'),identity),connection=chatConnection(false);
  const execution=new ExecutionRegistry([],async()=>connection(),{workspace,singleServiceIdentity:identity});
  const embedded=new EmbeddedAuth({publicOrigin:origin,clients:[],anonymous:{parentOrigins:['https://portal.example'],executionOwners:[],singleService:{identity}}},store);
  const app=await createApp({localOnly:true,workspace,nativeConnection:connection,auth:new LocalAuth('unused',port),execution,embedded,embeddedBindings:store.bindings,embeddedStorageCheck:()=>store.check()});
  const headers={host,origin};
  try{
    for(const path of ['/','/config','/embed/institution-message']){const r=await app.inject({url:path,headers});assert.equal(r.statusCode,200,r.body);assert.match(r.body,/<title>智能工作台<\/title>/);}
    assert.match(String((await app.inject({url:'/config',headers})).headers['content-security-policy']),/frame-ancestors 'none'/);
    const login=await app.inject({method:'POST',url:'/api/v1/auth/local-session',headers,payload:{name:'zhangzhanhui',phone:'13800000000'}});assert.equal(login.statusCode,200,login.body);
    const admin={...headers,cookie:String(login.headers['set-cookie']).split(';')[0],'x-csrf-token':login.json().data.csrfToken};
    const initial=(await app.inject({url:'/api/v1/scene-config',headers:admin})).json().data;
    assert.ok(initial.config.scenes.message);assert.ok(initial.config.scenes['institution-message']);
    const updated=structuredClone(initial.config);updated.scenes['institution-message']={directory:workspace,executor:{type:'skill',name:'institution-query'}};
    const saved=await app.inject({method:'PUT',url:'/api/v1/scene-config',headers:admin,payload:{config:updated,revision:initial.revision}});assert.equal(saved.statusCode,200,saved.body);
    assert.deepEqual(JSON.parse(await readFile(join(workspace,'.workbench','scenarios.json'),'utf8')),updated);
    assert.deepEqual(updated.scenes.message,initial.config.scenes.message,'ordinary message binding is preserved');
    assert.equal((await app.inject({method:'PUT',url:'/api/v1/scene-config',headers:admin,payload:{config:updated,revision:initial.revision}})).statusCode,409);
    const visitors=await Promise.all([0,1,2].map(async()=>{const r=await app.inject({method:'POST',url:'/api/v1/embed/anonymous',headers,payload:{}});assert.equal(r.statusCode,200,r.body);return r.json().data;}));
    const vh=(v:any)=>({...headers,authorization:'Bearer '+v.token,'x-csrf-token':v.csrfToken});
    const effective=(await app.inject({url:'/api/v1/embed/scene-config',headers:vh(visitors[0])})).json().data;
    assert.deepEqual(effective.config.scenes['institution-message'],updated.scenes['institution-message']);assert.equal(effective.canEdit,false);
    assert.equal((await app.inject({method:'PUT',url:'/api/v1/scene-config',headers:vh(visitors[0]),payload:{config:updated,revision:saved.json().data.revision}})).statusCode,401);
    const ids=await Promise.all(visitors.map(async v=>{const r=await app.inject({method:'POST',url:'/api/v1/embed/chat/sessions',headers:vh(v),payload:{scenario:'institution-message-inspection'}});assert.equal(r.statusCode,201,r.body);return r.json().data.id;}));
    await Promise.all(ids.map(async(id,i)=>{const r=await app.inject({method:'POST',url:'/api/v1/embed/chat/sessions/'+id+'/messages',headers:vh(visitors[i]),payload:{text:'分析本会话已获取证据',scenario:'institution-message-inspection'}});assert.equal(r.statusCode,202,r.body);}));
    for(const id of ids){assert.deepEqual(sessions.get(id).permissions,[{action:'*',resource:'*',effect:'deny'}]);assert.equal(sessions.get(id).metadata.harnessScenario,'institution-message-inspection');}
    const main=await app.inject({method:'POST',url:'/api/v1/chat/sessions',headers:admin,payload:{scenario:'institution-message-inspection'}});assert.equal(main.statusCode,201,main.body);
    assert.equal((await app.inject({method:'POST',url:'/api/v1/chat/sessions/'+main.json().data.id+'/messages',headers:admin,payload:{text:'同一绑定',scenario:'institution-message-inspection'}})).statusCode,202);
    const prompts=calls.filter(c=>c.path.endsWith('/prompt'));assert.equal(prompts.length,4);assert.ok(prompts.every(c=>JSON.stringify(c.body.skills)==='[{"id":"native-message-skill"}]'));
    // Upgrade compatibility: existing grants/native Sessions retain their old
    // stored scenario; new UI requests still use the institution binding.
    sessions.get(ids[2]).metadata.harnessScenario='message-inspection';active[ids[2]]=false;
    const old=new DatabaseSync(join(root,'private.sqlite'));
    old.prepare('UPDATE wh_chat_session_bindings SET scenario=? WHERE native_session_id=?').run('message-inspection',ids[2]);old.close();
    const resumed=await app.inject({method:'POST',url:'/api/v1/embed/chat/sessions/'+ids[2]+'/messages',headers:vh(visitors[2]),payload:{text:'升级后继续追问',scenario:'institution-message-inspection'}});
    assert.equal(resumed.statusCode,202,resumed.body);assert.deepEqual(calls.filter(c=>c.path.endsWith('/prompt')).at(-1)!.body.skills,[{id:'native-message-skill'}]);
    assert.equal((await app.inject({method:'POST',url:'/api/v1/embed/chat/sessions/'+ids[1]+'/interrupt',headers:vh(visitors[0]),payload:{}})).statusCode,404);
    assert.equal((await app.inject({method:'DELETE',url:'/api/v1/embed/session',headers:vh(visitors[0])})).statusCode,204);assert.equal(active[ids[1]],true);assert.equal(active[ids[2]],true);
  }finally{await app.close();await store.close();peer.closeAllConnections();await new Promise<void>(done=>peer.close(()=>done()));if(before===undefined)delete process.env.HARNESS_OPENCODE_URL;else process.env.HARNESS_OPENCODE_URL=before;await rm(root,{recursive:true,force:true});}
});
