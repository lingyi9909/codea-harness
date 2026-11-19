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
