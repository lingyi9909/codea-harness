/**
 * 业务请求用户上下文
 *
 * 职责：向任务及工具执行传递当前用户相关的上下文。
 * Java 研发阅读提示：不要将调用方自报身份直接理解成企业认证。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
import {AuthProblem} from './auth.ts';
import type {Identity} from '../../../packages/storage/src/identity.ts';
import type {ChatBinding} from '../../../packages/storage/src/chat-bindings.ts';
export type UserScope=Readonly<{context:'trusted-business-call'}>;
type Context={ownerId:string;sessionId:string;serviceIdentity:string;target:string;username:string;phone:string};
export class UserContextResolver {
  private readonly contexts=new WeakMap<UserScope,Context>();
  private readonly profile:(owner:string)=>Promise<Identity|undefined>;
  constructor(profile:(owner:string)=>Promise<Identity|undefined>){this.profile=profile;}
  // Called by server code only after authentication + DB/native ownership checks.
  async bind(ownerId:string,binding:ChatBinding,target:string):Promise<UserScope>{
    if(binding.ownerId!==ownerId||target!=='mock-logs')throw new AuthProblem(403,'BUSINESS_TARGET_FORBIDDEN');
    const identity=await this.profile(ownerId);
    if(!identity||identity.id!==ownerId||identity.status!=='ACTIVE')throw new AuthProblem(401,'AUTH_REQUIRED');
    const scope=Object.freeze({context:'trusted-business-call' as const});
    this.contexts.set(scope,{ownerId,sessionId:binding.id,serviceIdentity:binding.serviceIdentity,target,username:identity.name,phone:identity.phone});
    return scope;
  }
  internal(scope:UserScope){const context=this.contexts.get(scope);if(!context)throw new AuthProblem(403,'UNTRUSTED_TOOL_CONTEXT');return {...context};}
  public(scope:UserScope){const {ownerId,sessionId,target,username,phone}=this.internal(scope);return {ownerId,sessionId,target,username,maskedPhone:'****'+phone.slice(-4)};}
}
