import type {SessionMessagesResponse} from '@opencode/client';
import type {ChatFooter,ChatStep} from '../../../../../packages/contracts/src/chat.ts';

type Message=SessionMessagesResponse['data'][number];
// OpenCode v2.2.7 SessionTurn uses wall time for duration and provider-active
// time (created to streamed) for speed. Tool execution and queue time do not add tokens.
export function assistantFooters(messages:Message[],hasOlder:boolean){
  const footers=new Map<string,ChatFooter>();
  const legacy=!messages.some(m=>m.type==='idle');
  let boundaryKnown=!hasOlder;
  let promptSeen=false,promptTime:number|undefined;
  let totalTokens=0,providerMs=0,validRate=true;
  for(const m of messages){
    const input=m.type==='user'||m.type==='synthetic';
    if(m.type==='idle'||(input&&(legacy||!promptSeen))){
      promptSeen=input;
      promptTime=input?m.time?.created:undefined;
      totalTokens=0;providerMs=0;validRate=true;
      if(m.type==='idle')boundaryKnown=true;
    }
    if(m.type!=='assistant')continue;
    const start=m.time?.created,end=m.time?.completed,streamed=m.time?.streamed;
    if(Number.isFinite(start)&&streamed!==undefined&&Number.isFinite(streamed)&&streamed>=start&&
      m.tokens&&Number.isFinite(m.tokens.output)&&m.tokens.output>=0&&Number.isFinite(m.tokens.reasoning)&&m.tokens.reasoning>=0){
      totalTokens+=m.tokens.output+m.tokens.reasoning;
      providerMs+=streamed-start;
    }else validRate=false;
    if(end===undefined&&!m.finish&&!m.error)continue;
    const footer:ChatFooter={};
    if(m.agent)footer.agent=m.agent;
    if(m.model?.id)footer.model=m.model.id;
    const from=promptSeen?promptTime:start;
    if(boundaryKnown&&from!==undefined&&end!==undefined&&Number.isFinite(from)&&Number.isFinite(end)&&end>from)
      footer.durationMs=end-from;
    if(boundaryKnown&&validRate&&totalTokens>0&&providerMs>0)footer.tokensPerSecond=totalTokens*1000/providerMs;
    if(Object.keys(footer).length)footers.set(m.id,footer);
  }
  return footers;
}
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
export function activity(m:Message):ChatStep[]{
  const steps:ChatStep[]=[];
  if(m.type==='assistant'){
    for(const [ordinal,p] of m.content.entries()){
      if(p.type==='reasoning'){
        const text=typeof p.text==='string'?p.text:'';
        steps.push({kind:'thinking',ordinal,detailPart:steps.length,at:p.time?.created,ended:p.time?.completed,
          complete:p.time?.completed!=null||m.time?.completed!=null||Boolean(m.finish),
          hasText:Boolean(text.trim()),text});
      }
      if(p.type==='text'&&p.text)steps.push({kind:'text',ordinal,text:p.text});
      if(p.type!=='tool')continue;
      const state=p.state;
      const result=state.status==='completed'||state.status==='error'?(state.content??[]).map(c=>
        c.type==='text'?c.text:(c.name??'文件附件')).join('\n'):'';
      const progress=state.status==='running'&&typeof state.metadata?.title==='string'?activityPreview(state.metadata.title,300):undefined;
      steps.push({kind:'tool',ordinal,name:p.name,status:state.status,at:p.time?.ran??p.time?.created,
        ended:p.time?.completed,input:publicInput(state.input),result:result?activityPreview(result):undefined,
        progress,error:state.status==='error'&&state.error?.message?activityPreview(state.error.message):undefined});
    }
    if(m.retry)steps.push({kind:'notice',text:'重试 '+m.retry.attempt+' · '+activityPreview(m.retry.error.message),at:m.retry.at});
    if(m.error)steps.push({kind:'notice',problem:true,text:'执行失败 · '+activityPreview(m.error.message),at:m.time?.completed});
  }
  if(m.type==='skill')steps.push({kind:'notice',text:'已加载 Skill · '+m.name,at:m.time?.created});
  if(m.type==='shell')steps.push({kind:'tool',name:'Shell',status:m.status,at:m.time?.created,ended:m.time?.completed,
    input:activityPreview(m.command,1600),result:m.output?activityPreview(m.output.output)+(m.output.truncated?'\n…（原始输出已截断）':''):undefined,
    progress:m.exit!=null?'退出码 '+m.exit:undefined});
  if(m.type==='compaction')steps.push({kind:'notice',text:'整理会话上下文 · '+m.status,at:m.time?.created});
  if(m.type==='idle')steps.push({kind:'notice',problem:m.outcome!=='succeeded',text:({succeeded:'本轮已完成',failed:'本轮失败',interrupted:'本轮已中断'})[m.outcome]+(m.error?' · '+activityPreview(m.error.message):''),at:m.time?.created});
  return steps;
}
