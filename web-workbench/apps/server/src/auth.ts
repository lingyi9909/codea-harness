import { createHash, randomBytes, randomUUID, timingSafeEqual } from 'node:crypto';
import type { FastifyReply, FastifyRequest } from 'fastify';
import type { Pool, RowDataPacket } from 'mysql2/promise';

type Session={ownerId:string;csrf:string;expires:number;profile?:{name:string;key:string}};
const hash=(s:string)=>createHash('sha256').update(s).digest();
const safeEqual=(a:Buffer,b:Buffer)=>a.length===b.length&&timingSafeEqual(a,b);
const COOKIE='harness_session';
export class LocalAuth {
  private pairingHash:Buffer;
  private paired=false;
  private createdAt=Date.now();
  private readonly attempts:number[]=[];
  private readonly sessions=new Map<string,Session>();
  private readonly port:number;
  private readonly devOrigin?:string;
  private readonly publishCode?: (code:string)=>void;
  constructor(pairingCode:string,port:number,devOrigin?:string,publishCode?:(code:string)=>void){
    this.publishCode=publishCode;
    this.port=port;this.devOrigin=devOrigin;
    this.pairingHash=hash(pairingCode);
    if(devOrigin&&!/^http:\/\/(127\.0\.0\.1|localhost):4175$/.test(devOrigin))
      throw new Error('UNSAFE_DEVELOPMENT_ORIGIN');
  }
  private refreshPairing(){
    if(!this.publishCode)return;
    const now=Date.now();
    for(const [key,session] of this.sessions)if(session.expires<=now)this.sessions.delete(key);
    if(this.sessions.size || (!this.paired && now-this.createdAt<=300000))return;
    const code=randomBytes(32).toString('base64url');
    this.createdAt=now;this.pairingHash=hash(code);this.paired=false;
    this.publishCode(code);
  }
  origin(req:FastifyRequest):boolean{
    const host=req.headers.host??'';
    if(!new RegExp('^(127\\.0\\.0\\.1|localhost|\\[::1\\]):'+this.port+'$').test(host))
      return false;
    const origin=req.headers.origin;
    return origin===undefined||origin==='http://'+host||origin===this.devOrigin;
  }
  pair(req:FastifyRequest,reply:FastifyReply,provided:unknown,ownerId:string):Session{
    if(!this.origin(req))throw new AuthProblem(403,'ORIGIN_INVALID');
    this.refreshPairing();
    const now=Date.now();
    if(typeof provided!=='string'||provided.length>256)throw new AuthProblem(401,'PAIRING_INVALID');
    while(this.attempts.length&&this.attempts[0]<now-60000)this.attempts.shift();
    if(this.attempts.length>=5)throw new AuthProblem(429,'PAIRING_RATE_LIMIT');
    this.attempts.push(now);
    if(this.paired||now-this.createdAt>300000||!safeEqual(hash(provided),this.pairingHash))
      throw new AuthProblem(401,'PAIRING_INVALID');
    this.paired=true;
    return this.issue(reply,ownerId);
  }
  requireLocalOrigin(req:FastifyRequest){
    if(!this.origin(req)||req.headers.origin!=='http://'+req.headers.host)
      throw new AuthProblem(403,'ORIGIN_INVALID');
  }
  openLocal(req:FastifyRequest,reply:FastifyReply,profile:{name:string;key:string},save:()=>void):Session{
    this.requireLocalOrigin(req);
    const now=Date.now();
    for(const [key,session] of this.sessions){
      if(session.expires<=now)this.sessions.delete(key);
      else if(session.profile?.key!==profile.key)throw new AuthProblem(409,'PROFILE_IN_USE');
    }
    // Saving and issuing are synchronous so overlapping logins cannot interleave.
    save();
    try{return this.session(req);}catch(e){
      if(!(e instanceof AuthProblem)||e.code!=='AUTH_REQUIRED')throw e;
    }
    return this.issue(reply,'local-workbench',profile);
  }
  private issue(reply:FastifyReply,ownerId:string,profile?:Session['profile']):Session{
    const token=randomBytes(32).toString('base64url');
    const session={ownerId,profile,csrf:randomBytes(32).toString('base64url'),expires:Date.now()+43200000};
    this.sessions.set(hash(token).toString('hex'),session);
    reply.header('Set-Cookie',COOKIE+'='+token+'; HttpOnly; SameSite=Strict; Path=/; Max-Age=43200');
    return session;
  }
  session(req:FastifyRequest):Session{
    if(!this.origin(req))throw new AuthProblem(403,'ORIGIN_INVALID');
    this.refreshPairing();
    const token=req.headers.cookie?.split(';').map(s=>s.trim()).find(s=>s.startsWith(COOKIE+'='))?.slice(COOKIE.length+1);
    if(!token)throw new AuthProblem(401,'AUTH_REQUIRED');
    const key=hash(token).toString('hex');
    const session=this.sessions.get(key);
    if(!session||session.expires<Date.now()){this.sessions.delete(key);throw new AuthProblem(401,'AUTH_REQUIRED');}
    return session;
  }
  mutation(req:FastifyRequest):Session{
    const session=this.session(req);
    if(req.headers.origin===undefined)throw new AuthProblem(403,'ORIGIN_REQUIRED');
    const token=req.headers['x-csrf-token'];
    if(typeof token!=='string'||!safeEqual(hash(token),hash(session.csrf)))
      throw new AuthProblem(403,'CSRF_INVALID');
    return session;
  }
  logout(req:FastifyRequest,reply:FastifyReply):void{
    this.mutation(req);
    const token=req.headers.cookie?.split(';').map(s=>s.trim()).find(s=>s.startsWith(COOKIE+'='))?.slice(COOKIE.length+1);
    if(token)this.sessions.delete(hash(token).toString('hex'));
    this.refreshPairing();
    reply.header('Set-Cookie',COOKIE+'=; HttpOnly; SameSite=Strict; Path=/; Max-Age=0');
  }
}
export class AuthProblem extends Error{
  readonly status:number; readonly code:string;
  constructor(status:number,code:string){super(code);this.status=status;this.code=code;}
}
export async function ensureLocalOwner(pool:Pool):Promise<{id:string;name:string}>{
  const subject='local-single-user';
  const [rows]=await pool.execute<RowDataPacket[]>('SELECT id,display_name FROM wh_owners WHERE subject=?',[subject]);
  if(rows.length)return {id:String(rows[0].id),name:String(rows[0].display_name)};
  const id=randomUUID();
  const now=new Date().toISOString().slice(0,23).replace('T',' ');
  try {
    await pool.execute('INSERT INTO wh_owners(id,subject,display_name,created_at,updated_at) VALUES (?,?,?,?,?)',
      [id,subject,'Local Developer',now,now]);
    return{id,name:'Local Developer'};
  }catch(e){
    const [again]=await pool.execute<RowDataPacket[]>('SELECT id,display_name FROM wh_owners WHERE subject=?',[subject]);
    if(again.length)return {id:String(again[0].id),name:String(again[0].display_name)};
    throw e;
  }
}
