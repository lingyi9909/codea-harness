/**
 * 机构页面数据模型
 *
 * 职责：提供机构排查页面共用的数据类型和业务常量。
 * Java 研发阅读提示：注意前端类型是编译期约束，真实参数验证仍由服务端完成。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
import type {ChatModel} from '../../../packages/contracts/src/chat.ts';

/** Select an actual catalog entry, retaining its provider and variant identity. */
export function institutionDefaultModel(models:ChatModel[]):ChatModel|undefined{
  const exact=models.find(m=>m.id.toLowerCase()==='kimi'||m.name.trim().toLowerCase()==='kimi');
  const kimi=exact??models.find(m=>(m.id+' '+m.name).toLowerCase().includes('kimi'));
  return kimi??(models.length===1?models[0]:undefined);
}
