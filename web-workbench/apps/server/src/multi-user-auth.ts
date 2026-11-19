import { createHash, randomBytes, timingSafeEqual } from 'node:crypto';
import type { FastifyRequest,FastifyReply } from 'fastify';
import { AuthProblem,type Session } from './auth.ts';
import type { IdentityStore } from '../../../packages/storage/src/identity.ts';

const digest=(s:string)=>createHash('sha256').update(s).digest('hex');
const equal=(a:string,b:string)=>a.length===b.length&&timingSafeEqual(Buffer.from(a),Buffer.from(b));
const cookie='harness_session';
const token=(req:FastifyRequest)=>req.headers.cookie?.split(';').map(s=>s.trim()).find(s=>s.startsWith(cookie+'='))?.slice(cookie.length+1);
export function loginProfile(input:unknown){
  if(!input||typeof input!=='object'||Array.isArray(input))throw new AuthProblem(400,'PROFILE_INVALID');
  const {username,phone}=input as Record<string,unknown>;
  if(typeof username!=='string'||!username.trim()||Array.from(username.trim()).length>80||/[\x00-\x1f\x7f]/.test(username))throw new AuthProblem(400,'USERNAME_INVALID');
  if(typeof phone!=='string'||phone.length>32)throw new AuthProblem(400,'PHONE_REQUIRED');
  let normalized=phone.trim().replace(/[ -]/g,'');
  if(/^1[3-9]\d{9}$/.test(normalized))normalized='+86'+normalized;
  if(!/^\+[1-9]\d{7,14}$/.test(normalized))throw new AuthProblem(400,'PHONE_INVALID');
  return {name:username.trim(),phone:normalized};
}
export class MultiUserAuth {
  readonly mode='multi-user';
  private readonly originUrl:URL;
  private readonly attempts=new Map<string,number[]>();
  private readonly listeners=new Map<string,Set<()=>void>>();
  private readonly store:IdentityStore;
  private readonly now:()=>number;
  constructor(store:IdentityStore,origin:string,now=Date.now){
    this.store=store;this.now=now;
    this.originUrl=new URL(origin);
    if(this.originUrl.protocol!=='https:'||this.originUrl.origin!==origin||this.originUrl.username||this.originUrl.password)throw new Error('TRUSTED_HTTPS_ORIGIN_REQUIRED');
  }
  origin(req:FastifyRequest){
    return req.headers.host===this.originUrl.host&&(req.headers.origin===undefined||req.headers.origin===this.originUrl.origin);
  }
  private checkOrigin(req:FastifyRequest,required=false){
    if(!this.origin(req)||(required&&req.headers.origin!==this.originUrl.origin))throw new AuthProblem(403,'ORIGIN_INVALID');
  }
  async login(req:FastifyRequest,reply:FastifyReply,input:unknown):Promise<Session>{
    this.checkOrigin(req,true);
    const at=this.now(),key=req.ip;
    const recent=(this.attempts.get(key)??[]).filter(t=>t>at-60000);
    if(recent.length>=20)throw new AuthProblem(429,'LOGIN_RATE_LIMIT');
    recent.push(at);this.attempts.set(key,recent);
    if(this.attempts.size>1000)for(const [ip,times]of this.attempts)if(times.at(-1)!<at-60000)this.attempts.delete(ip);
    const profile=loginProfile(input),owner=await this.store.identify(profile.name,profile.phone);
    if(owner.status!=='ACTIVE'||owner.name!==profile.name)throw new AuthProblem(409,'PROFILE_CONFLICT');
    const raw=randomBytes(32).toString('base64url'),csrf=digest('csrf:'+raw),expires=at+43200000;
    await this.store.issue(owner.id,digest(raw),digest(csrf),expires);
    const old=token(req);if(old){await this.store.revoke(digest(old));this.close(old);}
    reply.header('Set-Cookie',cookie+'='+raw+'; HttpOnly; Secure; SameSite=Strict; Path=/; Max-Age=43200');
    return {ownerId:owner.id,profile:{name:owner.name,key:owner.id},csrf,expires};
  }
  async session(req:FastifyRequest):Promise<Session>{
    this.checkOrigin(req);
    const raw=token(req);if(!raw||!/^[A-Za-z0-9_-]{43}$/.test(raw))throw new AuthProblem(401,'AUTH_REQUIRED');
    const stored=await this.store.find(digest(raw)),csrf=digest('csrf:'+raw);
    if(!stored||stored.expires<=this.now()||stored.owner.status!=='ACTIVE'||!equal(stored.csrfDigest,digest(csrf)))throw new AuthProblem(401,'AUTH_REQUIRED');
    return {ownerId:stored.owner.id,profile:{name:stored.owner.name,key:stored.owner.id},csrf,expires:stored.expires};
  }
  async mutation(req:FastifyRequest){
    this.checkOrigin(req,true);
    const session=await this.session(req),csrf=req.headers['x-csrf-token'];
    if(typeof csrf!=='string'||!equal(digest(csrf),digest(session.csrf)))throw new AuthProblem(403,'CSRF_INVALID');
    return session;
  }
  async logout(req:FastifyRequest,reply:FastifyReply){
    await this.mutation(req);const raw=token(req)!;
    await this.store.revoke(digest(raw));this.close(raw);
    reply.header('Set-Cookie',cookie+'=; HttpOnly; Secure; SameSite=Strict; Path=/; Max-Age=0');
  }
  onRevoke(req:FastifyRequest,fn:()=>void){
    const key=digest(token(req)??''),set=this.listeners.get(key)??new Set();set.add(fn);this.listeners.set(key,set);
    return ()=>{set.delete(fn);if(!set.size)this.listeners.delete(key);};
  }
  private close(raw:string){const key=digest(raw);for(const fn of this.listeners.get(key)??[])fn();this.listeners.delete(key);}
}
