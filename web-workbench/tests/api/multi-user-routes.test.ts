import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,mkdir,rm,realpath} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {randomUUID} from 'node:crypto';
import type {Pool} from 'mysql2/promise';
import {createApp} from '../../apps/server/src/app.ts';
import {MultiUserAuth} from '../../apps/server/src/multi-user-auth.ts';
import {ExecutionRegistry,type ExecutionRecord} from '../../apps/server/src/execution-registry.ts';
import type {Identity,StoredSession,IdentityStore} from '../../packages/storage/src/identity.ts';
test('T2/T3 HTTP injection: three users execute independently and all foreign session operations are denied',async()=>{
  const base=await realpath(await mkdtemp(join(tmpdir(),'v11-routes-'))),workspace=join(base,'shared');await mkdir(workspace);
  const people=new Map<string,Identity>(),sessions=new Map<string,StoredSession>();
  const store:IdentityStore={identify:async(name,phone)=>{let p=people.get(phone);if(!p){p={id:randomUUID(),name,phone,status:'ACTIVE'};people.set(phone,p);}return p;},issue:async(id,key,csrfDigest,expires)=>{sessions.set(key,{owner:[...people.values()].find(p=>p.id===id)!,csrfDigest,expires});},find:async key=>sessions.get(key),revoke:async key=>{sessions.delete(key);}};
  const auth=new MultiUserAuth(store,'https://workbench.example');
  const bindings=new Map<string,{owner:string;scenario:string;directory:string}>();
  const pool={execute:async(sql:string,args:unknown[])=>{
    if(sql.startsWith('INSERT INTO wh_chat_session_bindings')){bindings.set(args[0]+':'+args[1],{owner:String(args[2]),scenario:String(args[3]),directory:String(args[4])});return [{affectedRows:1}];}
    if(sql.includes('SELECT scenario,directory FROM wh_chat_session_bindings')){const r=bindings.get(args[0]+':'+args[1]);return [r&&r.owner===args[2]?[r]:[]];}
    if(sql.startsWith('UPDATE wh_chat_session_bindings')){bindings.delete(args[0]+':'+args[1]);return [{affectedRows:1}];}
    return [[]];
  }} as unknown as Pool;
  const identities=await Promise.all([0,1,2].map(i=>store.identify('User '+i,'+861380000000'+i)));
  const records:ExecutionRecord[]=await Promise.all(identities.map(async(p,i)=>{const dir=join(base,'user-'+i);await mkdir(dir);return {ownerId:p.id,serviceIdentity:'service-'+i,url:'http://127.0.0.1:'+String(6000+i),authorization:'credential-'+i,isolation:'os-user',directories:{[workspace]:dir}};}));
  const nativeSessions=new Map<string,any>(),active:Record<string,boolean>={},calls:string[]=[];
  const execution=new ExecutionRegistry(records,async(url)=>{
    const index=records.findIndex(r=>r.url===url),prefix='user-'+index;
    const client={session:{
      create:async(input:any)=>{const session={id:prefix+'-chat',...input,time:{updated:Date.now()}};nativeSessions.set(session.id,session);return session;},
      get:async({sessionID}:any)=>nativeSessions.get(sessionID),
      list:async()=>({data:[...nativeSessions.values()].filter(s=>s.id.startsWith(prefix))}),
      active:async()=>active,inbox:{list:async()=>[]},form:{list:async()=>[]},
      instructions:{entry:{put:async()=>{}}},
      prompt:async({sessionID}:any)=>{calls.push(sessionID);active[sessionID]=true;return {id:sessionID};},
      interrupt:async({sessionID}:any)=>{active[sessionID]=false;return {interrupted:true};},
      remove:async({sessionID}:any)=>{nativeSessions.delete(sessionID);},
      update:async({sessionID,title}:any)=>{nativeSessions.get(sessionID).title=title;},
    },model:{list:async()=>({data:[]})},message:{list:async()=>({data:[]})},permission:{list:async()=>[]}};
    return client as never;
  });
  const app=await createApp({pool,auth,execution,workspace});
  const headers=(cookie?:string,csrf?:string)=>({host:'workbench.example',origin:'https://workbench.example',...(cookie?{cookie}:{}),...(csrf?{'x-csrf-token':csrf}:{})});
  try{
    const users=await Promise.all([0,1,2].map(async i=>{const response=await app.inject({method:'POST',url:'/api/v1/auth/login',headers:headers(),payload:{username:'User '+i,phone:'1380000000'+i}});assert.equal(response.statusCode,200,response.body);return {cookie:String(response.headers['set-cookie']).split(';')[0],csrf:response.json().data.csrfToken};}));
    const ids=await Promise.all(users.map(async u=>{const r=await app.inject({method:'POST',url:'/api/v1/chat/sessions',headers:headers(u.cookie,u.csrf),payload:{scenario:'direct-chat'}});assert.equal(r.statusCode,201,r.body);return r.json().data.id as string;}));
    for(const [method,suffix,payload]of [
      ['GET','',undefined],['PATCH','',{title:'attack'}],['DELETE','',undefined],['POST','/messages',{text:'attack'}],['POST','/interrupt',{}],
      ['POST','/permission',{requestId:'p',decision:'once',revision:'a'.repeat(64)}],['POST','/form',{requestId:'f',revision:'a'.repeat(64),decision:'reply',answer:{}}],['GET','/activity',undefined],
    ]as const){const r=await app.inject({method,url:'/api/v1/chat/sessions/'+ids[1]+suffix,headers:headers(users[0].cookie,users[0].csrf),payload});assert.equal(r.statusCode,404,method+suffix+':'+r.body);}
    assert.equal(calls.length,0);
    await Promise.all(users.map(async(u,i)=>{const r=await app.inject({method:'POST',url:'/api/v1/chat/sessions/'+ids[i]+'/messages',headers:headers(u.cookie,u.csrf),payload:{text:'own request '+i}});assert.equal(r.statusCode,202,r.body);}));
    assert.equal(calls.length,3);
    const interrupted=await app.inject({method:'POST',url:'/api/v1/chat/sessions/'+ids[0]+'/interrupt',headers:headers(users[0].cookie,users[0].csrf)});assert.equal(interrupted.statusCode,200,interrupted.body);
    assert.equal(active[ids[0]],false);assert.equal(active[ids[1]],true);assert.equal(active[ids[2]],true);
    await app.inject({method:'DELETE',url:'/api/v1/auth/session',headers:headers(users[0].cookie,users[0].csrf)});
    assert.equal((await app.inject({url:'/api/v1/bootstrap',headers:headers(users[0].cookie)})).statusCode,401);
    assert.equal((await app.inject({url:'/api/v1/bootstrap',headers:headers(users[1].cookie)})).statusCode,200);
  }finally{await app.close();await rm(base,{recursive:true,force:true});}
});
