/**
 * 业务验证码处理
 *
 * 职责：协调业务验证流程中的挑战、状态与校验结果。
 * Java 研发阅读提示：区分浏览器登录态与具体业务的验证状态。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
import {randomBytes,randomUUID} from 'node:crypto';
import {AuthProblem} from './auth.ts';
import {UserContextResolver,type UserScope} from './user-context.ts';
export interface BusinessProvider {
  send(phone:string):Promise<{id:string;expires:number}>;
  verify(id:string,code:string):Promise<{token:string;expires:number}|undefined>;
  query(token:string,query:string):Promise<unknown>;
}
type State='WAITING_CODE'|'VERIFYING'|'AUTHORIZED'|'FAILED'|'EXPIRED'|'CANCELLED';
type Challenge={id:string;remote:string;state:State;expires:number;attempts:number;maskedPhone:string;sentAt:number};
export class BusinessVerificationBridge {
  private readonly challenges=new Map<string,Challenge>();
  private readonly access=new Map<string,{token:string;expires:number}>();
  private readonly sending=new Set<string>();
  private readonly resolver:UserContextResolver;
  private readonly provider:BusinessProvider;
  private readonly now:()=>number;
  constructor(resolver:UserContextResolver,provider:BusinessProvider,now=Date.now){this.resolver=resolver;this.provider=provider;this.now=now;}
  private key(scope:UserScope){const c=this.resolver.internal(scope);return JSON.stringify([c.ownerId,c.serviceIdentity,c.sessionId,c.target]);}
  private current(key:string){
    const challenge=this.challenges.get(key);
    if(challenge&&challenge.expires<=this.now()&&['WAITING_CODE','VERIFYING','AUTHORIZED'].includes(challenge.state)){challenge.state='EXPIRED';this.access.delete(key);}
    const access=this.access.get(key);if(access&&access.expires<=this.now())this.access.delete(key);
    return challenge;
  }
  status(scope:UserScope){
    const key=this.key(scope),challenge=this.current(key);
    if(this.access.has(key))return {state:'AUTHORIZED' as const};
    if(!challenge)return {state:'NEED_VERIFICATION' as const};
    return {state:challenge.state==='AUTHORIZED'?'EXPIRED' as const:challenge.state,challengeId:challenge.id,expiresAt:challenge.expires,maskedPhone:challenge.maskedPhone};
  }
  async send(scope:UserScope){
    const key=this.key(scope),old=this.current(key);
    if(this.access.has(key))return this.status(scope);
    if(this.sending.has(key)||old&&this.now()-old.sentAt<60000)throw new AuthProblem(429,'BUSINESS_SMS_RATE_LIMIT');
    this.sending.add(key);
    try{
      const {phone}=this.resolver.internal(scope);
      let remote:{id:string;expires:number};try{remote=await this.provider.send(phone);}catch{throw new AuthProblem(502,'BUSINESS_SEND_FAILED');}
      if(!remote.id||!Number.isFinite(remote.expires)||remote.expires<=this.now())throw new AuthProblem(502,'BUSINESS_SEND_FAILED');
      this.access.delete(key);this.challenges.set(key,{id:randomUUID(),remote:remote.id,state:'WAITING_CODE',expires:Math.min(remote.expires,this.now()+300000),attempts:0,maskedPhone:'****'+phone.slice(-4),sentAt:this.now()});
      return this.status(scope);
    }finally{this.sending.delete(key);}
  }
  async submit(scope:UserScope,id:string,code:unknown){
    const key=this.key(scope),challenge=this.current(key);
    if(!challenge||challenge.id!==id)throw new AuthProblem(404,'BUSINESS_CHALLENGE_NOT_FOUND');
    if(challenge.state!=='WAITING_CODE')throw new AuthProblem(409,'BUSINESS_CHALLENGE_CLOSED');
    if(typeof code!=='string'||code.length<1||code.length>32)throw new AuthProblem(400,'BUSINESS_CODE_INVALID');
    challenge.state='VERIFYING';challenge.attempts++;
    let verified:{token:string;expires:number}|undefined;
    try{verified=await this.provider.verify(challenge.remote,code);}catch{challenge.state='FAILED';throw new AuthProblem(502,'BUSINESS_VERIFY_FAILED');}
    // Cancel/expiry/replacement can happen while the target system is verifying.
    if(this.challenges.get(key)!==challenge||challenge.state!=='VERIFYING'||challenge.expires<=this.now()){
      this.current(key);throw new AuthProblem(409,'BUSINESS_CHALLENGE_CLOSED');
    }
    if(!verified?.token||!Number.isFinite(verified.expires)||verified.expires<=this.now()){
      challenge.state=challenge.attempts>=3?'FAILED':'WAITING_CODE';return {...this.status(scope),verified:false};
    }
    challenge.state='AUTHORIZED';challenge.expires=Math.min(verified.expires,this.now()+600000);
    this.access.set(key,{token:verified.token,expires:challenge.expires});return {...this.status(scope),verified:true};
  }
  cancel(scope:UserScope,id:string){
    const key=this.key(scope),challenge=this.current(key);
    if(!challenge||challenge.id!==id)throw new AuthProblem(404,'BUSINESS_CHALLENGE_NOT_FOUND');
    challenge.state='CANCELLED';this.access.delete(key);return this.status(scope);
  }
  async query(scope:UserScope,query:string){
    const key=this.key(scope);this.current(key);const access=this.access.get(key);
    if(!access)throw new AuthProblem(403,'BUSINESS_VERIFICATION_REQUIRED');
    try{return await this.provider.query(access.token,query);}catch{this.access.delete(key);throw new AuthProblem(502,'BUSINESS_QUERY_FAILED');}
  }
}
// Test/development adapter only. Never selected implicitly for a real target.
export class MockLogProvider implements BusinessProvider {
  private readonly codes=new Map<string,{code:string;expires:number}>();
  private readonly tokens=new Map<string,number>();
  private readonly now:()=>number;
  constructor(now=Date.now){this.now=now;}
  async send(_phone:string){const id=randomUUID(),expires=this.now()+120000;this.codes.set(id,{code:'123456',expires});return {id,expires};}
  async verify(id:string,code:string){const challenge=this.codes.get(id);if(!challenge||challenge.code!==code||challenge.expires<=this.now())return undefined;this.codes.delete(id);const token=randomBytes(32).toString('base64url'),expires=this.now()+300000;this.tokens.set(token,expires);return {token,expires};}
  async query(token:string,query:string){if((this.tokens.get(token)??0)<=this.now())throw new Error('EXPIRED');return {source:'mock-logs',query,items:[{message:'模拟日志平台返回；尚未连接真实内网'}]};}
}
