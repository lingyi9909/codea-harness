import {test} from 'node:test';
import assert from 'node:assert/strict';
import {randomUUID} from 'node:crypto';
import {MultiUserAuth,loginProfile} from '../../apps/server/src/multi-user-auth.ts';
import type {Identity,IdentityStore,StoredSession} from '../../packages/storage/src/identity.ts';
import type {FastifyRequest,FastifyReply} from 'fastify';

class MemoryIdentityStore implements IdentityStore {
  people=new Map<string,Identity>();sessions=new Map<string,StoredSession>();
  async identify(name:string,phone:string){let owner=this.people.get(phone);if(!owner){owner={id:randomUUID(),name,phone,status:'ACTIVE'};this.people.set(phone,owner);}return owner;}
  async identifyUsername(name:string){return this.identify(name,'username:'+name);}
  async issue(id:string,digest:string,csrfDigest:string,expires:number){const owner=[...this.people.values()].find(p=>p.id===id)!;this.sessions.set(digest,{owner,csrfDigest,expires});}
  async find(digest:string){return this.sessions.get(digest);}
  async revoke(digest:string){this.sessions.delete(digest);}
}
const origin='https://workbench.example';
function request(cookie?:string,csrf?:string){return {ip:'127.0.0.1',headers:{host:'workbench.example',origin,cookie,'x-csrf-token':csrf}} as unknown as FastifyRequest;}
function reply(){const headers=new Map<string,string>();return {headers,value:{header:(k:string,v:string)=>{headers.set(k,v);}} as FastifyReply};}
test('T1: three simultaneous users, stable identities, restart, CSRF and independent logout',async()=>{
  const store=new MemoryIdentityStore(),auth=new MultiUserAuth(store,origin);
  const users=await Promise.all([0,1,2].map(async i=>{const res=reply();const session=await auth.login(request(),res.value,{username:'user '+i,phone:'1380000000'+i});return {session,cookie:res.headers.get('Set-Cookie')!.split(';')[0]};}));
  assert.equal(new Set(users.map(u=>u.session.ownerId)).size,3);
  assert.equal(store.sessions.size,3);
  for(const {session,cookie}of users){assert.match(cookie,/^harness_session=/);assert.equal((await new MultiUserAuth(store,origin).session(request(cookie))).ownerId,session.ownerId);}
  const res=reply(),again=await auth.login(request(),res.value,{username:'user 0',phone:'+8613800000000'});
  assert.equal(again.ownerId,users[0].session.ownerId);
  await assert.rejects(auth.login(request(),reply().value,{username:'other',phone:'13800000000'}),/PROFILE_CONFLICT/);
  await assert.rejects(auth.mutation(request(users[0].cookie,users[1].session.csrf)),/CSRF_INVALID/);
  let closed=false;auth.onRevoke(request(users[0].cookie),()=>{closed=true;});
  await auth.logout(request(users[0].cookie,users[0].session.csrf),reply().value);assert.equal(closed,true);
  await assert.rejects(auth.session(request(users[0].cookie)),/AUTH_REQUIRED/);
  assert.equal((await auth.session(request(users[1].cookie))).ownerId,users[1].session.ownerId);
  for(const s of store.sessions.values()){assert.equal('token' in s,false);assert.match(s.csrfDigest,/^[a-f0-9]{64}$/);}
});
test('T1: validation, host/origin restrictions, disabled identities and expiry',async()=>{
  for(const input of [null,{},[],{username:'a',phone:''},{username:'',phone:'13800000000'},{username:'a',phone:'not-a-phone'}])assert.throws(()=>loginProfile(input));
  assert.deepEqual(loginProfile({username:' A ',phone:'138 0000 0000'}),{name:'A',phone:'+8613800000000'});
  assert.throws(()=>new MultiUserAuth(new MemoryIdentityStore(),'http://workbench.example'));
  let now=1000;const store=new MemoryIdentityStore(),auth=new MultiUserAuth(store,origin,()=>now),res=reply();
  const session=await auth.login(request(),res.value,{username:'A',phone:'13800000000'}),cookie=res.headers.get('Set-Cookie')!.split(';')[0];
  assert.match(res.headers.get('Set-Cookie')!,/HttpOnly; Secure; SameSite=Strict/);
  const bad=request(cookie);bad.headers.origin='https://evil.example';await assert.rejects(auth.session(bad),/ORIGIN_INVALID/);
  now=session.expires;await assert.rejects(auth.session(request(cookie)),/AUTH_REQUIRED/);
});

test('username-only server login survives restart and isolates independent users',async()=>{
  const store=new MemoryIdentityStore();
  store.identifyUsername=async name=>store.identify(name,'username:'+name);
  const auth=new MultiUserAuth(store,origin),r=reply();
  const first=await auth.login(request(),r.value,{username:'zhang01'});
  const cookie=r.headers.get('Set-Cookie')!.split(';')[0];
  const peer=await auth.login(request(),reply().value,{username:'li'});
  assert.notEqual(first.ownerId,peer.ownerId);
  await auth.logout(request(cookie,first.csrf),reply().value);
  await assert.rejects(auth.session(request(cookie)),/AUTH_REQUIRED/);
  const again=await new MultiUserAuth(store,origin).login(request(),reply().value,{username:'zhang01'});
  assert.equal(again.ownerId,first.ownerId);
  await assert.rejects(auth.login(request(),reply().value,{username:'123'}),/USERNAME_INVALID/);
});
