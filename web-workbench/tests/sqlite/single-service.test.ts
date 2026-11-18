import {test} from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {mkdtemp,mkdir,rm,realpath} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {createHash,randomUUID} from 'node:crypto';
import {chatConnection} from '../../apps/server/src/modules/chat/connection.ts';
import {ExecutionRegistry} from '../../apps/server/src/execution-registry.ts';
import {EmbeddedAuth} from '../../apps/server/src/embed-auth.ts';
import {createEmbeddedApp} from '../../apps/server/src/embedded-app.ts';
import {SqliteEmbedStore} from '../../packages/storage/src/sqlite-embed.ts';
test('one real HTTP service address: three Sessions run concurrently, remain owner-bound and stop independently',{timeout:20000},async()=>{
  const root=await realpath(await mkdtemp(join(tmpdir(),'one-native-'))),workspace=join(root,'workspace');await mkdir(workspace);
  const sessions=new Map<string,any>(),active:Record<string,boolean>={},prompted:string[]=[],paths:string[]=[];let serial=0;
  const peer=createServer(async(req,res)=>{
    const path=new URL(req.url!,'http://localhost').pathname;paths.push(path);assert.equal(req.headers.authorization,'Bearer private-native-service');
    let bytes='';for await(const chunk of req)bytes+=chunk;const body=bytes?JSON.parse(bytes):{};
    res.setHeader('Content-Type','application/json');const data=(v:unknown)=>res.end(JSON.stringify({data:v}));
    if(path==='/api/model')return data([{id:'private',providerID:'company',name:'private',enabled:true}]);
    if(path==='/api/session/active')return data(active);
    if(path==='/api/session'&&req.method==='POST'){const s={id:'session-'+(++serial),...body,time:{updated:Date.now()}};sessions.set(s.id,s);return data(s);}
    if(path.startsWith('/api/experimental/session/')){res.statusCode=204;return res.end();}
    const match=path.match(/^\/api\/session\/(session-\d+)(.*)$/);if(match){const[,id,suffix]=match;
      if(!sessions.has(id)){res.statusCode=404;return res.end('{}');}
      if(!suffix){if(req.method==='DELETE'){sessions.delete(id);res.statusCode=204;return res.end();}return data(sessions.get(id));}
      if(suffix==='/prompt'){prompted.push(id);active[id]=true;return data({id:'message-'+prompted.length,sessionID:id});}
      if(suffix==='/interrupt'){active[id]=false;return res.end(JSON.stringify({interrupted:true}));}
      if(suffix==='/message')return data([]);
      if(['/inbox','/permission','/form'].includes(suffix))return data([]);
    }
    res.statusCode=404;res.end('{}');
  });
  await new Promise<void>(resolve=>peer.listen(0,'127.0.0.1',resolve));const url='http://127.0.0.1:'+(peer.address() as {port:number}).port;
  const before={url:process.env.HARNESS_OPENCODE_URL,auth:process.env.HARNESS_OPENCODE_AUTHORIZATION};process.env.HARNESS_OPENCODE_URL=url;process.env.HARNESS_OPENCODE_AUTHORIZATION='Bearer private-native-service';
  const identity='single:'+createHash('sha256').update(url).digest('hex'),store=new SqliteEmbedStore(join(root,'state.sqlite'),identity),connection=chatConnection(false);
  const execution=new ExecutionRegistry([],async()=>connection(),{workspace,singleServiceIdentity:identity});
  const auth=new EmbeddedAuth({publicOrigin:'https://workbench.example',clients:[],anonymous:{parentOrigins:['https://portal.example'],executionOwners:[],singleService:{identity}}},store);
  const app=await createEmbeddedApp({auth,execution,workspace,bindings:store.bindings,ensureOwner:(id,subject)=>store.ensureOwner(id,subject),storageCheck:()=>store.check()});
  const headers=(v?:{token:string;csrfToken:string})=>({host:'workbench.example',origin:'https://workbench.example',...(v?{authorization:'Bearer '+v.token,'x-csrf-token':v.csrfToken}:{})});
  try{
    const visits=await Promise.all([0,1,2].map(async()=>{const r=await app.inject({method:'POST',url:'/api/v1/embed/anonymous',headers:headers(),payload:{}});assert.equal(r.statusCode,200,r.body);return r.json().data;}));
    const ids=await Promise.all(visits.map(async v=>{const r=await app.inject({method:'POST',url:'/api/v1/embed/chat/sessions',headers:headers(v),payload:{scenario:'message-inspection'}});assert.equal(r.statusCode,201,r.body);return r.json().data.id;}));
    assert.equal(new Set(ids).size,3);for(const s of sessions.values()){assert.equal(s.location.directory,workspace);assert.deepEqual(s.permissions,[{action:'*',resource:'*',effect:'deny'}]);}
    await Promise.all(visits.map(async(v,i)=>{const r=await app.inject({method:'POST',url:'/api/v1/embed/chat/sessions/'+ids[i]+'/messages',headers:headers(v),payload:{text:'分析已查询证据-'+i}});assert.equal(r.statusCode,202,r.body);}));
    assert.equal(prompted.length,3);assert.ok(ids.every(id=>active[id]));
    const foreign=await app.inject({method:'POST',url:'/api/v1/embed/chat/sessions/'+ids[1]+'/interrupt',headers:headers(visits[0]),payload:{}});assert.equal(foreign.statusCode,404);
    assert.equal((await app.inject({method:'DELETE',url:'/api/v1/embed/session',headers:headers(visits[0])})).statusCode,204);assert.equal(active[ids[1]],true);assert.equal(active[ids[2]],true);assert.equal(sessions.has(ids[0]),false);
    assert.ok(!paths.some(p=>/skill|command|mcp|shell|file/.test(p)));
    assert.equal((await app.inject({method:'POST',url:'/api/v1/embed/anonymous',headers:headers(),payload:{}})).statusCode,200);
    assert.throws(()=>execution.directory(workspace),/CONTEXT_REQUIRED/);
  }finally{await app.close();await store.close();peer.closeAllConnections();await new Promise<void>(resolve=>peer.close(()=>resolve()));if(before.url===undefined)delete process.env.HARNESS_OPENCODE_URL;else process.env.HARNESS_OPENCODE_URL=before.url;if(before.auth===undefined)delete process.env.HARNESS_OPENCODE_AUTHORIZATION;else process.env.HARNESS_OPENCODE_AUTHORIZATION=before.auth;await rm(root,{recursive:true,force:true});}
});
test('one service: broadening native tool permissions is rejected before any prompt',async()=>{
  const {sqliteEmbedFixture}=await import('../support/sqlite-embed-fixture.ts');const f=await sqliteEmbedFixture();
  try{
    const v=(await f.app.inject({method:'POST',url:'/api/v1/embed/anonymous',headers:f.headers(),payload:{}})).json().data;
    const made=await f.app.inject({method:'POST',url:'/api/v1/embed/chat/sessions',headers:f.headers(v.token,v.csrfToken),payload:{scenario:'message-inspection'}});assert.equal(made.statusCode,201,made.body);const id=made.json().data.id;
    f.nativeSessions.get(id).permissions=[{action:'edit',resource:'*',effect:'deny'}];
    const denied=await f.app.inject({method:'POST',url:'/api/v1/embed/chat/sessions/'+id+'/messages',headers:f.headers(v.token,v.csrfToken),payload:{text:'try shell or other-session files'}});assert.equal(denied.statusCode,409,denied.body);assert.equal(f.calls.length,0);
  }finally{await f.close();}
});
test('one service: page admission is atomic without executor recycling',async()=>{
  const root=await mkdtemp(join(tmpdir(),'single-admission-')),store=new SqliteEmbedStore(join(root,'state.sqlite'));
  const identity='single:'+'a'.repeat(64),fresh=()=>({id:randomUUID(),digest:randomUUID(),expires:Date.now()+60000,parentOrigin:'https://workbench.example'});
  try{
    const results=await Promise.all(Array.from({length:8},()=>store.createSingle(fresh(),identity,3))),grants=results.filter(Boolean);assert.equal(grants.length,3);assert.equal(new Set(grants.map(g=>g!.ownerId)).size,3);assert.ok(grants.every(g=>g!.ownerId===g!.id));
    await store.revoke(grants[0]!.id);assert.ok(await store.createSingle(fresh(),identity,3));
  }finally{await store.close();await rm(root,{recursive:true,force:true});}
});
