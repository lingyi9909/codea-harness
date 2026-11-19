import type {ChatModel} from '../../../packages/contracts/src/chat.ts';

/** Select an actual catalog entry, retaining its provider and variant identity. */
export function institutionDefaultModel(models:ChatModel[]):ChatModel|undefined{
  const exact=models.find(m=>m.id.toLowerCase()==='kimi'||m.name.trim().toLowerCase()==='kimi');
  const kimi=exact??models.find(m=>(m.id+' '+m.name).toLowerCase().includes('kimi'));
  return kimi??(models.length===1?models[0]:undefined);
}
