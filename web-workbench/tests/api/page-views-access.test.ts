import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {randomUUID} from 'node:crypto';
import {PageVisitStore,validPageId} from '../../apps/server/src/page-visits.ts';
import {SceneAccessStore,applyToolPolicy,defaultAccess} from '../../apps/server/src/scene-access.ts';
import {LocalAuth} from '../../apps/server/src/auth.ts';
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
    assert.equal(applyToolPolicy(undefined,initial.config.scenes.logs),undefined);
    const updated=defaultAccess();
    updated.scenes.logs={lan:false,shell:'deny',fileWrite:'deny',dbhub:'deny'};
    const saved=store.save(updated,initial.revision);
    assert.equal(saved.config.scenes.logs.lan,false);
    assert.deepEqual(applyToolPolicy(undefined,saved.config.scenes.logs),[
      {action:'shell',resource:'*',effect:'deny'},
      {action:'edit',resource:'*',effect:'deny'},
      {action:'write',resource:'*',effect:'deny'},
      {action:'dbhub_search_objects',resource:'*',effect:'deny'},
      {action:'dbhub_execute_sql',resource:'*',effect:'deny'},
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
  assert.match(opened.ownerId,/^lan:[0-9a-f]{48}$/);
  const cookie=responseHeaders['Set-Cookie'].split(';')[0];
  assert.equal(auth.session({...lan,headers:{...lan.headers,cookie}} as FastifyRequest).ownerId,opened.ownerId);
  assert.throws(()=>auth.session({...loopback,headers:{...loopback.headers,cookie}} as FastifyRequest),/SESSION_ORIGIN_MISMATCH/);
  assert.throws(()=>auth.openLan(loopback,reply,{name:'user',key:'key'},'1234567890ABCD'),/LAN_ORIGIN_FORBIDDEN/);
});
