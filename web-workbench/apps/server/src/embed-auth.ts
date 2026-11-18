import {createHash,randomBytes,randomUUID,timingSafeEqual} from 'node:crypto';
import type {FastifyRequest,FastifyReply} from 'fastify';
import {AuthProblem,type RequestAuth} from './auth.ts';
import type {EmbedStore,EmbedGrant} from '../../../packages/storage/src/embed.ts';
export type EmbedClient={id:string;secret:string;parentOrigin:string;principals:Record<string,string>};
export type EmbedConfig={publicOrigin:string;clients:EmbedClient[];sessionTTLSeconds?:number;anonymous?:{parentOrigins:string[];executionOwners:string[];singleService?:{identity:string;maxPages?:number};resetCommand?:string[];resetPrivateCommand?:string[]}};
export const embedDigest=(s:string)=>createHash('sha256').update(s).digest('hex');
export function trustedOrigin(origin:string){
  const url=new URL(origin);
  if(url.origin!==origin||url.username||url.password||!(url.protocol==='https:'||url.protocol==='http:'&&['127.0.0.1','localhost','[::1]'].includes(url.hostname)))throw new Error('EMBED_HTTPS_ORIGIN_REQUIRED');
  return url;
}
export class EmbeddedAuth implements RequestAuth {
  readonly config:EmbedConfig;
  readonly store:EmbedStore;
  private readonly now:()=>number;
  private readonly sessions=new WeakMap<FastifyRequest,EmbedGrant>();
  private readonly listeners=new Map<string,Set<()=>void>>();
  private readonly issuance=new Map<string,number[]>();
  constructor(config:EmbedConfig,store:EmbedStore,now=Date.now){
    trustedOrigin(config.publicOrigin);
    const ids=new Set<string>(),secrets=new Set<string>(),owners=new Map<string,string>();
    config={...config,clients:config.clients??[]};
    if(!config.clients.length&&!config.anonymous)throw new Error('EMBED_CLIENTS_REQUIRED');
    for(const c of config.clients){
      trustedOrigin(c.parentOrigin);
      if(!/^[A-Za-z0-9_-]{1,80}$/.test(c.id)||ids.has(c.id)||typeof c.secret!=='string'||c.secret.length<32||secrets.has(c.secret)||!Object.keys(c.principals).length)throw new Error('EMBED_CLIENT_CONFIG_INVALID');
      ids.add(c.id);secrets.add(c.secret);
      for(const [subject,id]of Object.entries(c.principals)){
        if(!subject||subject.length>128||!/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(id)||owners.has(id))throw new Error('EMBED_PRINCIPAL_MAPPING_INVALID');
        owners.set(id,c.id+':'+subject);
      }
    }
    if(config.anonymous){
      const {parentOrigins,executionOwners,resetCommand,resetPrivateCommand,singleService}=config.anonymous;
      if(!Array.isArray(parentOrigins)||!parentOrigins.length||!Array.isArray(executionOwners)||(!singleService&&!executionOwners.length)||executionOwners.length>100)throw new Error('EMBED_ANONYMOUS_CONFIG_INVALID');
      if(singleService&&(executionOwners.length||config.clients.length||resetCommand||resetPrivateCommand||!/^single:[a-f0-9]{64}$/.test(singleService.identity)||!Number.isInteger(singleService.maxPages??100)||(singleService.maxPages??100)<1||(singleService.maxPages??100)>1000))throw new Error('SINGLE_SERVICE_CONFIG_INVALID');
      parentOrigins.forEach(trustedOrigin);
      for(const id of executionOwners){if(!/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(id)||owners.has(id))throw new Error('EMBED_SLOT_INVALID');owners.set(id,'anonymous');}
      for(const command of [resetCommand,resetPrivateCommand])if(command&&(!command.length||command.some(v=>typeof v!=='string'||!v)))throw new Error('EMBED_RESET_COMMAND_INVALID');
    }
    if(!Number.isInteger(config.sessionTTLSeconds??7200)||(config.sessionTTLSeconds??7200)<60||(config.sessionTTLSeconds??7200)>28800)throw new Error('EMBED_TTL_INVALID');
    this.config=structuredClone(config);this.store=store;this.now=now;
  }
  origin(req:FastifyRequest){return req.headers.host===new URL(this.config.publicOrigin).host&&(req.headers.origin===undefined||req.headers.origin===this.config.publicOrigin);}
  private check(req:FastifyRequest,mutation=false){if(!this.origin(req)||mutation&&req.headers.origin!==this.config.publicOrigin)throw new AuthProblem(403,'ORIGIN_INVALID');}
  async issue(req:FastifyRequest,input:unknown,ensureOwner:(id:string,subject:string)=>Promise<void>){
    // This endpoint is for the host backend. Its long-lived secret never goes to the browser.
    if(req.headers.host!==new URL(this.config.publicOrigin).host||req.headers.origin!==undefined)throw new AuthProblem(403,'EMBED_BACKEND_ONLY');
    const body=input as {clientId?:unknown;subject?:unknown};
    if(!body||typeof body.clientId!=='string'||typeof body.subject!=='string'||body.subject.length>128)throw new AuthProblem(400,'EMBED_IDENTITY_REQUIRED');
    const client=this.config.clients.find(c=>c.id===body.clientId),provided=req.headers.authorization??'';
    if(!client||!timingSafeEqual(Buffer.from(embedDigest(provided)),Buffer.from(embedDigest('Bearer '+client.secret))))throw new AuthProblem(401,'EMBED_CLIENT_INVALID');
    const ownerId=Object.hasOwn(client.principals,body.subject)?client.principals[body.subject]:undefined;
    if(!ownerId)throw new AuthProblem(403,'EMBED_PRINCIPAL_NOT_PROVISIONED');
    const recent=(this.issuance.get(client.id)??[]).filter(t=>t>this.now()-60000);
    if(recent.length>=120)throw new AuthProblem(429,'EMBED_TICKET_RATE_LIMIT');
    recent.push(this.now());this.issuance.set(client.id,recent);
    await ensureOwner(ownerId,embedDigest(client.id+':'+body.subject));
    const ticket=randomBytes(32).toString('base64url'),expires=this.now()+60000;
    await this.store.ticket({digest:embedDigest(ticket),ownerId,clientId:client.id,parentOrigin:client.parentOrigin,expires});
    return {ticket,expiresAt:expires,parentOrigin:client.parentOrigin,embedUrl:this.config.publicOrigin+'/embed/message?parentOrigin='+encodeURIComponent(client.parentOrigin)};
  }
  async exchange(req:FastifyRequest,input:unknown){
    this.check(req,true);const body=input as {ticket?:unknown;parentOrigin?:unknown};
    if(!body||typeof body.ticket!=='string'||!/^[\w-]{43}$/.test(body.ticket)||typeof body.parentOrigin!=='string')throw new AuthProblem(401,'EMBED_TICKET_INVALID');
    const token=randomBytes(32).toString('base64url'),expires=this.now()+(this.config.sessionTTLSeconds??7200)*1000;
    const grant=await this.store.exchange(embedDigest(body.ticket),body.parentOrigin,{id:randomUUID(),digest:embedDigest(token),expires});
    if(!grant)throw new AuthProblem(401,'EMBED_TICKET_INVALID');
    return {token,csrfToken:embedDigest('csrf:'+token),expiresAt:expires};
  }
  async anonymous(req:FastifyRequest,slots:{ownerId:string;serviceIdentity:string}[]){
    this.check(req,true);if(!this.config.anonymous)throw new AuthProblem(404,'EMBED_ANONYMOUS_DISABLED');
    const key='anonymous',recent=(this.issuance.get(key)??[]).filter(t=>t>this.now()-60000);
    if(recent.length>=120)throw new AuthProblem(429,'EMBED_SESSION_RATE_LIMIT');
    recent.push(this.now());this.issuance.set(key,recent);
    const token=randomBytes(32).toString('base64url'),expires=this.now()+(this.config.sessionTTLSeconds??7200)*1000;
    const input={id:randomUUID(),digest:embedDigest(token),expires,parentOrigin:this.config.publicOrigin},single=this.config.anonymous.singleService;
    if(single&&!this.store.createSingle)throw new AuthProblem(503,'SINGLE_SERVICE_STORAGE_REQUIRED');
    const grant=single?await this.store.createSingle!(input,single.identity,single.maxPages??100):await this.store.allocate(input,slots);
    if(!grant)throw new AuthProblem(503,'EMBED_CAPACITY_FULL');
    return {token,csrfToken:embedDigest('csrf:'+token),expiresAt:expires};
  }
  async grant(req:FastifyRequest){
    this.check(req);const header=req.headers.authorization??'';
    if(!/^Bearer [\w-]{43}$/.test(header))throw new AuthProblem(401,'EMBED_SESSION_REQUIRED');
    const grant=await this.store.find(embedDigest(header.slice(7)));
    if(!grant||grant.expires<=this.now())throw new AuthProblem(401,'EMBED_SESSION_REQUIRED');
    if(!(grant.accessMode==='ANONYMOUS'?grant.parentOrigin===this.config.publicOrigin&&(this.config.anonymous?.singleService?grant.ownerId===grant.id&&grant.serviceIdentity===this.config.anonymous.singleService.identity:this.config.anonymous?.executionOwners.includes(grant.ownerId)):this.config.clients.some(client=>client.parentOrigin===grant.parentOrigin&&Object.values(client.principals).includes(grant.ownerId)))){
      await this.store.revoke(grant.id);throw new AuthProblem(401,'EMBED_SESSION_REQUIRED');
    }
    this.sessions.set(req,grant);return grant;
  }
  async session(req:FastifyRequest){const grant=await this.grant(req);return {ownerId:grant.ownerId,csrf:embedDigest('csrf:'+(req.headers.authorization??'').slice(7)),expires:grant.expires};}
  async mutation(req:FastifyRequest){this.check(req,true);const session=await this.session(req);if(req.headers['x-csrf-token']!==session.csrf)throw new AuthProblem(403,'CSRF_INVALID');return session;}
  async authorize(req:FastifyRequest){
    const grant=await this.grant(req),path=req.url.split('?')[0];
    const id=(req.params as {id?:string})?.id;
    if(id&&(!grant.nativeId||id!==grant.nativeId))throw new AuthProblem(404,'CHAT_NOT_FOUND');
    if(path==='/api/v1/embed/chat/history'||req.method==='DELETE'||req.method==='PATCH')throw new AuthProblem(403,'EMBED_ACTION_FORBIDDEN');
    if(path==='/api/v1/embed/chat/project')throw new AuthProblem(403,'EMBED_ACTION_FORBIDDEN');
    if(req.method==='POST'){
      await this.mutation(req);
      const body=req.body as {scenario?:unknown;files?:unknown;directory?:unknown}|undefined;
      if(body?.scenario!==undefined&&body.scenario!=='message-inspection'&&!(grant.accessMode==='ANONYMOUS'&&body.scenario==='institution-message-inspection'))throw new AuthProblem(403,'EMBED_SCENARIO_FORBIDDEN');
      if(body?.files!==undefined)throw new AuthProblem(403,'EMBED_ATTACHMENTS_FORBIDDEN');
      if(path==='/api/v1/embed/chat/sessions'&&!await this.store.reserve(grant.id))throw new AuthProblem(409,'EMBED_ALREADY_INITIALIZED');
    }
    return grant;
  }
  metadata(req:FastifyRequest){const grant=this.sessions.get(req);if(!grant)throw new AuthProblem(401,'EMBED_SESSION_REQUIRED');return {harnessEmbedGrant:grant.id};}
  async bind(req:FastifyRequest,id:string,service:string){const grant=this.sessions.get(req);if(!grant)throw new AuthProblem(401,'EMBED_SESSION_REQUIRED');await this.store.bind(grant.id,id,service);}
  async finish(req:FastifyRequest){const grant=this.sessions.get(req);if(grant&&req.url.split('?')[0]==='/api/v1/embed/chat/sessions'&&req.method==='POST')await this.store.fail(grant.id);}
  async logout(req:FastifyRequest,_reply:FastifyReply){await this.mutation(req);const g=await this.grant(req);await this.store.revoke(g.id);for(const fn of this.listeners.get(g.id)??[])fn();this.listeners.delete(g.id);}
  onRevoke(req:FastifyRequest,fn:()=>void){const id=this.sessions.get(req)?.id;if(!id)return ()=>{};const set=this.listeners.get(id)??new Set();set.add(fn);this.listeners.set(id,set);return ()=>{set.delete(fn);if(!set.size)this.listeners.delete(id);};}
}
