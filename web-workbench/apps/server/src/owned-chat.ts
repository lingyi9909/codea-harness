/**
 * 会话归属判断
 *
 * 职责：辅助确保当前操作仅影响对应身份拥有的会话。
 * Java 研发阅读提示：用户归属检查必须在后端执行，不能依赖前端按钮可见性。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
import { AuthProblem } from './auth.ts';
import type {ChatBinding,ChatBindingStore} from '../../../packages/storage/src/chat-bindings.ts';
export async function requireChatBinding(store:ChatBindingStore,service:string,id:string,owner:string){
  const binding=await store.get(service,id,owner);
  if(!binding)throw new AuthProblem(404,'CHAT_NOT_FOUND');
  return binding;
}
export function matchNativeBinding(binding:ChatBinding,native:{id:string;metadata?:Record<string,unknown>;location:{directory:string}}){
  if(binding.id!==native.id||binding.ownerId!==native.metadata?.harnessOwner||binding.scenario!==native.metadata?.harnessScenario||binding.directory!==native.location.directory)
    throw new AuthProblem(404,'CHAT_NOT_FOUND');
}
