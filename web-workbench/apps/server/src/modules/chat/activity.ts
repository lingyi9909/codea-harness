import type {SessionMessagesResponse} from '@opencode/client';
import type {ChatStep} from '../../../../../packages/contracts/src/chat.ts';

type Message=SessionMessagesResponse['data'][number];
const sensitive=/password|passwd|secret|token|api.?key|authorization|cookie|credential|private.?key/i;
export function activityPreview(value:string,limit=4000){
  const clean=value.replace(/\b(Bearer|Basic)\s+[A-Za-z0-9+/=_.-]+/gi,'$1 [隐藏]')
    .replace(/(\b(?:api[_-]?key|access[_-]?token|password|secret)\s*[:=]\s*)[^\s,;]+/gi,'$1[隐藏]');
  return clean.length>limit?clean.slice(0,limit)+'\n…（摘录已截断）':clean;
}
function publicInput(input:unknown){
  if(!input||typeof input!=='object')return undefined;
  const json=JSON.stringify(input,(key,value)=>sensitive.test(key)?'[隐藏]':value,2);
  return json==='{}'?undefined:activityPreview(json,1600);
}
export function activity(m:Message,expandedPart?:number):ChatStep[]{
  const steps:ChatStep[]=[];
  if(m.type==='assistant'){
    for(const p of m.content){
      if(p.type==='reasoning'){
        const expanded=steps.length===expandedPart;
        steps.push({kind:'thinking',at:p.time?.created,ended:p.time?.completed,
          complete:p.time?.completed!=null||m.time?.completed!=null||Boolean(m.finish),
          hasText:Boolean(p.text?.trim()),...(expanded?{text:p.text??''}:{})});
      }
      if(p.type==='text'&&p.text)steps.push({kind:'text',text:p.text});
      if(p.type!=='tool')continue;
      const state=p.state;
      const result=state.status==='completed'||state.status==='error'?(state.content??[]).map(c=>
        c.type==='text'?c.text:(c.name??'文件附件')).join('\n'):'';
      const progress=state.status==='running'&&typeof state.metadata?.title==='string'?activityPreview(state.metadata.title,300):undefined;
      steps.push({kind:'tool',name:p.name,status:state.status,at:p.time?.ran??p.time?.created,
        ended:p.time?.completed,input:publicInput(state.input),result:result?activityPreview(result):undefined,
        progress,error:state.status==='error'&&state.error?.message?activityPreview(state.error.message):undefined});
    }
    if(m.retry)steps.push({kind:'notice',text:'重试 '+m.retry.attempt+' · '+activityPreview(m.retry.error.message),at:m.retry.at});
    if(m.error)steps.push({kind:'notice',text:'执行失败 · '+activityPreview(m.error.message),at:m.time?.completed});
  }
  if(m.type==='skill')steps.push({kind:'notice',text:'已加载 Skill · '+m.name,at:m.time?.created});
  if(m.type==='shell')steps.push({kind:'tool',name:'Shell',status:m.status,at:m.time?.created,ended:m.time?.completed,
    input:activityPreview(m.command,1600),result:m.output?activityPreview(m.output.output)+(m.output.truncated?'\n…（原始输出已截断）':''):undefined,
    progress:m.exit!=null?'退出码 '+m.exit:undefined});
  if(m.type==='compaction')steps.push({kind:'notice',text:'整理会话上下文 · '+m.status,at:m.time?.created});
  if(m.type==='idle')steps.push({kind:'notice',text:({succeeded:'本轮已完成',failed:'本轮失败',interrupted:'本轮已中断'})[m.outcome]+(m.error?' · '+activityPreview(m.error.message):''),at:m.time?.created});
  return steps;
}
