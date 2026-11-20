import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm,mkdir,writeFile} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {randomUUID} from 'node:crypto';
import {PageVisitStore,validPageId} from '../../apps/server/src/page-visits.ts';
import {SceneAccessStore,applyToolPolicy,defaultAccess,ACCESS_SCENES,validateAccess} from '../../apps/server/src/scene-access.ts';
import {LocalAuth} from '../../apps/server/src/auth.ts';
import {createApp} from '../../apps/server/src/app.ts';
import type {FastifyRequest,FastifyReply} from 'fastify';

test('page visits deduplicate StrictMode and persist PV/UV without raw IDs',async()=>{
  const root=await mkdtemp(join(tmpdir(),'workbench-visits-'));
  try{
    const path=join(root,'page-visits.sqlite'),id=randomUUID(),other=randomUUID(),now=Date.now();
    const store=new PageVisitStore(path);
    store.record('home',id,now-5000);
    store.record('home',id,now-4900); // same view, not double counted
    store.record('home',id,now);
    store.record('home',other,now);
    store.record('logs',other,now);
    const view=store.summary(7,now);
    assert.equal(view.rows.find(row=>row.page==='home')?.views,3);
    assert.equal(view.rows.find(row=>row.page==='home')?.visitors,2);
    assert.equal(view.rows.find(row=>row.page==='logs')?.views,1);
    assert.equal(view.daily.some(row=>row.page==='home'),true);
    assert.equal(validPageId('home'),true);
    assert.equal(validPageId('arbitrary'),false);
    assert.equal(JSON.stringify(view).includes(id),false);
    store.close();
    const restored=new PageVisitStore(path);
    assert.equal(restored.summary().rows.find(row=>row.page==='home')?.views,3);
    restored.close();
  }finally{await rm(root,{recursive:true,force:true});}
});

test('scene policy defaults preserve native rules; a saved revocation prevents widening',async()=>{
  const root=await mkdtemp(join(tmpdir(),'workbench-access-'));
  try{
    const store=new SceneAccessStore(root),initial=store.read();
    assert.equal(initial.config.scenes.logs.lan,true);
    assert.equal(initial.config.scenes.home.lan,true);
    assert.equal(initial.config.scenes.review.lan,true);
    assert.equal(initial.config.scenes.message.lan,true);
    assert.equal(initial.config.scenes['institution-message'].lan,true);
    assert.equal(initial.config.scenes.data.lan,true);
    assert.equal(initial.config.scenes.analytics.lan,false);
    assert.equal(initial.config.scenes['component-lab'].lan,false);
    assert.equal(applyToolPolicy(undefined,initial.config.scenes.logs),undefined);
    const updated=defaultAccess();
    updated.scenes.logs.lan=false;
    updated.scenes.logs.tools.shell='deny';
    updated.scenes.logs.tools.write='deny';
    updated.scenes.logs.tools.edit='deny';
    updated.scenes.logs.tools.dbhub_search_objects='deny';
    updated.scenes.logs.tools.dbhub_execute_sql='deny';
    updated.scenes.logs.tools.subagent='ask';
    updated.scenes.logs.custom=[{action:'teamai_search',resource:'*',effect:'ask'}];
    const saved=store.save(updated,initial.revision);
    assert.equal(saved.config.version,2);
    assert.equal(saved.config.scenes.logs.lan,false);
    assert.deepEqual(applyToolPolicy(undefined,saved.config.scenes.logs),[
      {action:'shell',resource:'*',effect:'deny'},
      {action:'subagent',resource:'*',effect:'ask'},
      {action:'write',resource:'*',effect:'deny'},
      {action:'edit',resource:'*',effect:'deny'},
      {action:'dbhub_search_objects',resource:'*',effect:'deny'},
      {action:'dbhub_execute_sql',resource:'*',effect:'deny'},
      {action:'teamai_search',resource:'*',effect:'ask'},
    ]);
    assert.throws(()=>store.save(updated,initial.revision),/SCENE_ACCESS_STALE/);
    assert.throws(()=>store.save({version:1,scenes:{logs:{shell:'allow'}}},saved.revision),/SCENE_ACCESS_INVALID/);
  }finally{await rm(root,{recursive:true,force:true});}
});

test('LAN colleague login requires explicit LAN code and cannot impersonate loopback administrator',()=>{
  const auth=new LocalAuth('pair',4176,undefined,undefined,{origin:'http://192.168.2.4:4176',code:'1234567890ABCD'});
  const lan={ip:'192.168.2.44',headers:{host:'192.168.2.4:4176',origin:'http://192.168.2.4:4176'}} as unknown as FastifyRequest;
  const loopback={ip:'127.0.0.1',headers:{host:'127.0.0.1:4176',origin:'http://127.0.0.1:4176'}} as unknown as FastifyRequest;
  const responseHeaders:Record<string,string>={};
  const reply={header:(name:string,value:string)=>{responseHeaders[name]=value;return reply;}} as unknown as FastifyReply;
  assert.equal(auth.isLan(lan),true);
  assert.equal(auth.isLocal(lan),false);
  assert.equal(auth.isLocal(loopback),true);
  assert.throws(()=>auth.openLan(lan,reply,{name:'user',key:'key'},'wrong'),/LAN_CODE_INVALID/);
  const opened=auth.openLan(lan,reply,{name:'user',key:'key'},'1234567890ABCD');
  assert.match(opened.ownerId,/^lan:[0-9a-f-]{36}$/);
  const cookie=responseHeaders['Set-Cookie'].split(';')[0];
  assert.equal(auth.session({...lan,headers:{...lan.headers,cookie}} as FastifyRequest).ownerId,opened.ownerId);
  assert.throws(()=>auth.session({...loopback,headers:{...loopback.headers,cookie}} as FastifyRequest),/SESSION_ORIGIN_MISMATCH/);
  assert.throws(()=>auth.openLan(loopback,reply,{name:'user',key:'key'},'1234567890ABCD'),/LAN_ORIGIN_FORBIDDEN/);
  const otherReplyHeaders:Record<string,string>={};
  const otherReply={header:(name:string,value:string)=>{otherReplyHeaders[name]=value;return otherReply;}} as unknown as FastifyReply;
  const sameSelfEnteredProfile=auth.openLan(lan,otherReply,{name:'user',key:'key'},'1234567890ABCD');
  assert.notEqual(sameSelfEnteredProfile.ownerId,opened.ownerId,
    'separate browser login must not inherit another coworker\'s native sessions');
});

test('LAN colleague can bootstrap workbench without being able to read or edit local configuration',async()=>{
  const root=await mkdtemp(join(tmpdir(),'workbench-lan-app-'));
  const visits=new PageVisitStore(join(root,'visits.sqlite'));
  const auth=new LocalAuth('unused',4176,undefined,undefined,{origin:'http://192.168.1.99:4176',code:'LAN-TEST-ACCESS-CODE'});
  const app=await createApp({auth,localOnly:true,workspace:root,pageVisits:visits});
  const remote={host:'192.168.1.99:4176',origin:'http://192.168.1.99:4176'};
  const localhost={host:'127.0.0.1:4176',origin:'http://127.0.0.1:4176'};
  try{
    const remoteLogin=await app.inject({method:'POST',url:'/api/v1/auth/local-session',remoteAddress:'192.168.1.18',
      headers:remote,payload:{name:'同事一',phone:'13800000010',lanCode:'LAN-TEST-ACCESS-CODE'}});
    assert.equal(remoteLogin.statusCode,200,remoteLogin.body);
    const remoteCookie=String(remoteLogin.headers['set-cookie']).split(';')[0];
    assert.match(remoteLogin.json().data.owner.id,/^lan:/);
    const colleague=await app.inject({url:'/api/v1/bootstrap',remoteAddress:'192.168.1.18',
      headers:{...remote,cookie:remoteCookie}});
    assert.equal(colleague.statusCode,200,colleague.body);
    assert.equal(colleague.json().data.identityMode,'lan');
    const allowed=await app.inject({url:'/api/v1/scene-runtime',remoteAddress:'192.168.1.18',
      headers:{...remote,cookie:remoteCookie}});
    assert.equal(allowed.statusCode,200,allowed.body);
    for(const url of ['/config','/api/v1/scene-config','/api/v1/scene-access']){
      const response=await app.inject({url,remoteAddress:'192.168.1.18',
        headers:{...remote,cookie:remoteCookie}});
      assert.equal(response.statusCode,403,url+':'+response.body);
    }
    const view=await app.inject({method:'POST',url:'/api/v1/page-visits',remoteAddress:'192.168.1.18',
      headers:remote,payload:{page:'logs',visitorId:randomUUID()}});
    assert.equal(view.statusCode,200,view.body);
    // These pages and analytics reads are localhost-only even if old policies
    // previously allowed them, while traffic from LAN business pages is recorded.
    for(const url of ['/analytics','/component-lab','/api/v1/page-visits','/api/v1/page-visits?days=30']){
      const blocked=await app.inject({url,remoteAddress:'192.168.1.18',
        headers:{...remote,cookie:remoteCookie}});
      assert.equal(blocked.statusCode,403,url+':'+blocked.body);
    }
    const editor=new SceneAccessStore(root),before=editor.read(),next=before.config;
    next.scenes.analytics.lan=true;
    next.scenes['component-lab'].lan=true;
    editor.save(next,before.revision);
    assert.equal(editor.read().config.scenes.analytics.lan,false);
    assert.equal(editor.read().config.scenes['component-lab'].lan,false);
    for(const url of ['/analytics','/component-lab','/api/v1/page-visits']){
      const blocked=await app.inject({url,remoteAddress:'192.168.1.18',
        headers:{...remote,cookie:remoteCookie}});
      assert.equal(blocked.statusCode,403,url+':'+blocked.body);
    }
    const localLogin=await app.inject({method:'POST',url:'/api/v1/auth/local-session',
      remoteAddress:'127.0.0.1',headers:localhost,payload:{name:'zhangzhanhui',phone:'13800000000'}});
    assert.equal(localLogin.statusCode,200,localLogin.body);
    const localCookie=String(localLogin.headers['set-cookie']).split(';')[0];
    const stats=await app.inject({url:'/api/v1/page-visits',remoteAddress:'127.0.0.1',
      headers:{...localhost,cookie:localCookie}});
    assert.equal(stats.statusCode,200,stats.body);
    assert.equal(stats.json().data.rows.find((row:{page:string})=>row.page==='logs').views,1);
  }finally{await app.close();visits.close();await rm(root,{recursive:true,force:true});}
});

test('version 1 page permissions migrate on read without overwriting the old file',async()=>{
  const root=await mkdtemp(join(tmpdir(),'workbench-access-v1-'));
  try{
    const folder=join(root,'.workbench');
    await mkdir(folder);
    const file=join(folder,'scene-access.json');
    const old={version:1,scenes:Object.fromEntries(ACCESS_SCENES.filter(id=>id!=='analytics').map(id=>[
      id,{lan:true,shell:id==='message'?'allow':'inherit',fileWrite:id==='message'?'deny':'inherit',
        dbhub:id==='message'?'deny':'inherit'},
    ]))};
    await writeFile(file,JSON.stringify(old));
    const store=new SceneAccessStore(root);
    const loaded=store.read();
    assert.equal(loaded.config.version,2);
    assert.equal(loaded.config.scenes.message.tools.shell,'allow');
    assert.equal(loaded.config.scenes.message.tools.edit,'deny');
    assert.equal(loaded.config.scenes.message.tools.write,'deny');
    assert.equal(loaded.config.scenes.message.tools.dbhub_execute_sql,'deny');
    assert.equal(loaded.config.scenes.analytics.lan,false);
    assert.equal(loaded.config.scenes['component-lab'].lan,false);
    for(const id of ['home','review','logs','message','institution-message','data'] as const)
      assert.equal(loaded.config.scenes[id].lan,true,id);
    assert.equal(JSON.parse(await (await import('node:fs/promises')).readFile(file,'utf8')).version,1);
    const saved=store.save(loaded.config,loaded.revision);
    assert.equal(saved.config.version,2);
    assert.throws(()=>validateAccess({...loaded.config,scenes:{...loaded.config.scenes,message:{
      ...loaded.config.scenes.message,custom:[{action:'*',resource:'*',effect:'allow'}]
    }}}),/SCENE_ACCESS_INVALID/);
  }finally{await rm(root,{recursive:true,force:true});}
});

test('pre-existing v2 policies that enabled utility pages cannot override localhost restriction',async()=>{
  const root=await mkdtemp(join(tmpdir(),'workbench-old-lan-policy-'));
  try{
    const directory=join(root,'.workbench');
    await mkdir(directory);
    const old=defaultAccess();
    old.scenes.analytics.lan=true;
    old.scenes['component-lab'].lan=true;
    await writeFile(join(directory,'scene-access.json'),JSON.stringify(old));
    const store=new SceneAccessStore(root),read=store.read();
    assert.equal(read.config.scenes.analytics.lan,false);
    assert.equal(read.config.scenes['component-lab'].lan,false);
    assert.equal(read.config.scenes.message.lan,true);
    assert.equal(read.config.scenes.review.lan,true);
    const saved=store.save(read.config,read.revision);
    assert.equal(saved.config.scenes.analytics.lan,false);
    assert.equal(saved.config.scenes['component-lab'].lan,false);
  }finally{await rm(root,{recursive:true,force:true});}
});
