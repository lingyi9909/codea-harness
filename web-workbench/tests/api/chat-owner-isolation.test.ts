import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {randomUUID} from 'node:crypto';
import {createApp} from '../../apps/server/src/app.ts';
import {LocalAuth} from '../../apps/server/src/auth.ts';
import {profileKey} from '../../apps/server/src/local-profile.ts';
import type {chatConnection} from '../../apps/server/src/modules/chat/connection.ts';

const port=4189;
const host='127.0.0.1:'+port,origin='http://'+host;
const A={name:'测试甲',phone:'13800000211'},B={name:'测试乙',phone:'13800000212'};
const api='/api/v1/chat';

test('local users A/B cannot list, read, write, rename, delete or interrupt each other sessions',async t=>{
  const root=await mkdtemp(join(tmpdir(),'chat-owner-switch-'));
  t.after(()=>rm(root,{recursive:true,force:true}));
  type NativeSession={id:string;title:string;metadata:{harnessOwner:string;harnessScenario:string};
    location:{directory:string};time:{updated:number};};
  const sessions=new Map<string,NativeSession>();
  const writes:string[]=[];
  const native={
    session:{
      create:async(input:{title?:string;metadata:NativeSession['metadata'];location:{directory:string}})=>{
        const row={id:'ses_'+randomUUID(),title:input.title??'对话',metadata:input.metadata,
          location:input.location,time:{updated:Date.now()}};
        sessions.set(row.id,row);return row;
      },
      get:async({sessionID}:{sessionID:string})=>sessions.get(sessionID),
      list:async()=>({data:[...sessions.values()],cursor:{}}),
      update:async({sessionID}:{sessionID:string})=>{writes.push('update:'+sessionID);},
      remove:async({sessionID}:{sessionID:string})=>{writes.push('remove:'+sessionID);sessions.delete(sessionID);},
      active:async()=>({}),
      inbox:{list:async()=>[]},
      form:{list:async()=>[]},
      interrupt:async({sessionID}:{sessionID:string})=>{writes.push('interrupt:'+sessionID);return {interrupted:true};},
    },
    message:{list:async()=>({data:[],cursor:{}})},
    permission:{list:async()=>[]},
  };
  const client=async()=>native as unknown as Awaited<ReturnType<ReturnType<typeof chatConnection>>>;
  const app=await createApp({auth:new LocalAuth('unused',port),localOnly:true,workspace:root,nativeConnection:client});
  t.after(()=>app.close());

  async function login(profile:typeof A){
    const res=await app.inject({method:'POST',url:'/api/v1/auth/local-session',
      headers:{host,origin},payload:profile});
    assert.equal(res.statusCode,200,res.body);
    return {cookie:String(res.headers['set-cookie']).split(';')[0],csrf:String(res.json().data.csrfToken),
      ownerId:String(res.json().data.owner.id)};
  }
  type Login=Awaited<ReturnType<typeof login>>;
  const headers=(user:Login)=>({host,origin,cookie:user.cookie,'x-csrf-token':user.csrf});
  async function create(user:Login){
    const res=await app.inject({method:'POST',url:api+'/sessions',headers:headers(user),
      payload:{scenario:'direct-chat'}});
    assert.equal(res.statusCode,201,res.body);return res.json().data.id as string;
  }
  async function history(user:Login){
    const res=await app.inject({method:'POST',url:api+'/history',headers:headers(user),
      payload:{scenario:'direct-chat'}});
    assert.equal(res.statusCode,200,res.body);
    return res.json().data.items.map((row:{id:string})=>row.id) as string[];
  }
  async function assertOtherBlocked(user:Login,sessionId:string){
    const tests=[
      {method:'GET',url:api+'/sessions/'+sessionId},
      {method:'GET',url:api+'/sessions/'+sessionId+'/activity'},
      {method:'POST',url:api+'/sessions/'+sessionId+'/messages',payload:{text:'越权',scenario:'direct-chat'}},
      {method:'POST',url:api+'/sessions/'+sessionId+'/interrupt',payload:{}},
      {method:'POST',url:api+'/sessions/'+sessionId+'/permission',payload:{requestId:'bad',decision:'allow'}},
      {method:'PATCH',url:api+'/sessions/'+sessionId,payload:{title:'越权改名'}},
      {method:'DELETE',url:api+'/sessions/'+sessionId},
    ];
    for(const spec of tests){
      const res=await app.inject({method:spec.method as 'GET'|'POST'|'PATCH'|'DELETE',
        url:spec.url,headers:headers(user),...(spec.payload?{payload:spec.payload}:{})});
      assert.equal(res.statusCode,404,spec.method+' '+spec.url+':'+res.body);
      assert.equal(res.json().error.code,'CHAT_NOT_FOUND');
    }
  }
  const a1=await login(A);
  assert.equal(a1.ownerId,'local:'+profileKey(A));
  const idA=await create(a1);
  assert.deepEqual(await history(a1),[idA]);
  const readA=await app.inject({url:api+'/sessions/'+idA,headers:headers(a1)});
  assert.equal(readA.statusCode,200,readA.body);
  const oldShared={id:'ses_legacy_shared',title:'旧版本混合聊天',metadata:{
    harnessOwner:'local-workbench',harnessScenario:'direct-chat'
  },location:{directory:root},time:{updated:Date.now()}};
  sessions.set(oldShared.id,oldShared);
  assert.deepEqual(await history(a1),[idA]);
  await assertOtherBlocked(a1,oldShared.id);
  const logout=async(user:Login)=>{
    const res=await app.inject({method:'DELETE',url:'/api/v1/auth/session',headers:headers(user)});
    assert.equal(res.statusCode,204,res.body);
  };
  await logout(a1);
  const b1=await login(B);
  assert.equal(b1.ownerId,'local:'+profileKey(B));
  assert.notEqual(b1.ownerId,a1.ownerId);
  assert.deepEqual(await history(b1),[]);
  await assertOtherBlocked(b1,idA);
  const idB=await create(b1);
  assert.deepEqual(await history(b1),[idB]);
  assert.deepEqual(writes,[],'denied operations must not reach native service');
  await logout(b1);
  const a2=await login(A);
  assert.equal(a2.ownerId,a1.ownerId,'re-login should recover only this user prior sessions');
  assert.deepEqual(await history(a2),[idA]);
  await assertOtherBlocked(a2,idB);
  assert.deepEqual(writes,[]);
});

test('two LAN logins even with identical self-reported details have distinct session owners',async t=>{
  const root=await mkdtemp(join(tmpdir(),'chat-lan-owners-'));
  t.after(()=>rm(root,{recursive:true,force:true}));
  const auth=new LocalAuth('unused',port,undefined,undefined,{origin:'http://192.168.1.66:'+port});
  const app=await createApp({auth,localOnly:true,workspace:root});
  t.after(()=>app.close());
  const headers={host:'192.168.1.66:'+port,origin:'http://192.168.1.66:'+port};
  const login=()=>app.inject({method:'POST',url:'/api/v1/auth/local-session',
    remoteAddress:'192.168.1.15',headers,payload:{...A}});
  const first=await login(),second=await login();
  assert.equal(first.statusCode,200,first.body);
  assert.equal(second.statusCode,200,second.body);
  const one=first.json().data.owner.id as string,two=second.json().data.owner.id as string;
  assert.match(one,/^lan:/);
  assert.match(two,/^lan:/);
  assert.notEqual(one,two,'two LAN browsers must never share session ownership');
});
