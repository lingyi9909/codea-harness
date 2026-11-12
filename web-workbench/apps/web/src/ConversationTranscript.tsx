import {lazy,Suspense,useEffect,useRef,useState} from 'react';
import type {ChatMessage,ChatStep,ToolProgress} from '../../../packages/contracts/src/chat.ts';
import {api,ApiError} from './api/client.ts';
import './conversation-transcript.css';

const Body=lazy(()=>import('./ChatMarkdown.tsx').then(module=>({default:module.ChatMarkdown})));
function Markdown({text}:{text:string}){
  return <Suspense fallback={<p>{text}</p>}><Body text={text}/></Suspense>;
}

type Entry={message:string;part:number;step:ChatStep;commentary:boolean};
type Turn={id:string;question?:ChatMessage;response:ChatMessage[]};
type ReasoningEntry=Entry & {step:Extract<ChatStep,{kind:'thinking'}>};
const toolStates:Record<string,string>={streaming:'准备中',running:'执行中',completed:'已完成',error:'失败',exited:'已退出',timeout:'超时',killed:'已停止'};
const toolNames:Record<string,string>={websearch:'搜索网页',webfetch:'读取网页',grep:'搜索内容',read:'读取文件',edit:'编辑文件',bash:'执行命令'};
function groupMessages(messages:ChatMessage[]){
  const turns:Turn[]=[];
  for(const message of messages){
    let turn=turns.at(-1);
    if(!turn||message.role==='user'){
      turn={id:message.id,question:message.role==='user'?message:undefined,response:[]};
      turns.push(turn);
    }
    if(message.role==='assistant')turn.response.push(message);
  }
  return turns;
}

function ToolRow({step}:{step:Extract<ChatStep,{kind:'tool'}>}){
  const name=toolNames[step.name]??step.name;
  const state=toolStates[step.status]??step.status;
  return <div className="execution-tool"><header><strong>{name}</strong>{name!==step.name&&<code>{step.name}</code>}
    <span className={'tool-state status-'+step.status}>{state}</span></header>
    {step.progress&&<p>{step.progress}</p>}
    {step.input&&<details><summary>调用参数</summary><pre>{step.input}</pre></details>}
    {step.result&&<details><summary>查看工具结果</summary>
      {step.name==='websearch'||step.name==='webfetch'
        ?<div className="web-result"><Markdown text={step.result}/></div>:<pre>{step.result}</pre>}</details>}
    {step.error&&<p className="transcript-error">{step.error}</p>}
  </div>;
}

function hasEnglishProse(text:string){
  const words=text.match(/\b[a-z]+(?:['’-][a-z]+)*\b/gi)?.length??0;
  const han=text.match(/\p{Script=Han}/gu)?.length??0;
  return words>0&&(han===0||words>Math.max(3,han));
}

function TranslatableReasoning({session,message,ordinal,original,open}:{
  session:string;message:string;ordinal:number;original:string;open:boolean;
}){
  const [translated,setTranslated]=useState('');
  const [showChinese,setShowChinese]=useState(false);
  const [pending,setPending]=useState(false);
  const [error,setError]=useState('');
  async function translate(){
    if(translated){setShowChinese(!showChinese);return;}
    setError('');setPending(true);
    try{
      const {text}=await api.translateReasoning(session,message,ordinal);
      setTranslated(text);setShowChinese(true);
    }catch(e){
      if(e instanceof ApiError&&e.code==='REASONING_TRANSLATION_UNAVAILABLE')setError('当前 OpenCode 版本不支持翻译。');
      else if(e instanceof ApiError&&e.code==='REASONING_NOT_FOUND')setError('找不到这段思考原文。');
      else setError('翻译失败，请重试。');
    }finally{setPending(false);}
  }
  if(!open)return null;
  return <><Markdown text={showChinese?translated:original}/>
    <button className="reasoning-translate" type="button" disabled={pending}
      title={translated?'切换原文和译文':'使用 OpenCode 将这段思考译成中文'} onClick={translate}>
      {pending?'翻译中…':showChinese?'查看原文':translated?'查看译文':'译成中文'}
    </button>{error&&<small role="status" className="transcript-error">{error}</small>}
  </>;
}

function Reasoning({session,entries,running}:{session:string;entries:ReasoningEntry[];running:boolean}){
  const [open,setOpen]=useState(running);
  const chosen=useRef(false);
  useEffect(()=>{
    if(running&&!chosen.current)setOpen(true);
  },[running]);
  return <details className="turn-reasoning" open={open}>
    <summary onClick={event=>{event.preventDefault();chosen.current=true;setOpen(!open);}}>
      <span className="process-chevron" aria-hidden="true">›</span>
      <strong>思考过程</strong><small>{entries.length} 段</small>
    </summary>
    <div className="reasoning-body">{entries.map(({message,step,part})=>{
      const original=step.text??'';
      return <div className="reasoning-part" key={message+':'+(step.ordinal??part)}>
        {step.complete&&step.ordinal!=null&&hasEnglishProse(original)
          ?<TranslatableReasoning key={original} session={session} message={message} ordinal={step.ordinal} original={original} open={open}/>
          :open&&<Markdown text={original}/>}
      </div>;
    })}</div>
  </details>;
}

function Response({session,messages,progress,active}:{session:string;messages:ChatMessage[];progress:ToolProgress[];active:boolean}){
  const entries:Entry[]=[];
  const files=new Set<string>();
  let lastTool=-1;
  for(const m of messages){
    for(const file of m.files)files.add(file);
    const steps=m.steps?.length?m.steps:[
      ...(m.text?[{kind:'text' as const,text:m.text}]:[]),
      ...m.tools.map(t=>({kind:'tool' as const,...t})),
    ];
    steps.forEach((step,part)=>{
      if(step.kind==='thinking'&&!step.hasText&&!step.text?.trim())return;
      if(step.kind==='tool')lastTool=entries.length;
      entries.push({message:m.id,part,step,commentary:!!m.toolCallTurn});
    });
    for(const event of progress.filter(p=>p.message===m.id)){
      entries.push({message:m.id,part:-entries.length-1,commentary:false,step:{kind:'notice',at:event.at,text:event.text}});
    }
  }
  const thinking=entries.filter((entry):entry is ReasoningEntry=>entry.step.kind==='thinking'&&Boolean(entry.step.text?.trim()));
  const details=entries.filter((e,i)=>e.step.kind!=='thinking'&&(e.step.kind!=='text'||e.commentary||i<lastTool));
  const answers=entries.filter((e,i)=>e.step.kind==='text'&&!e.commentary&&i>lastTool);
  const tools=details.filter(e=>e.step.kind==='tool');
  const failedTools=tools.filter(({step})=>step.kind==='tool'&&(step.status==='error'||step.status==='timeout'));
  const problem=details.slice().reverse().find(e=>e.step.kind==='notice'&&e.step.problem);
  const runningTool=tools.find(e=>e.step.kind==='tool'&&['running','streaming'].includes(e.step.status))?.step;
  const status=problem?.step.kind==='notice'?problem.step.text:undefined;
  const working=active&&!answers.length;
  if(!entries.length&&!files.size&&!active)return null;
  return <article className="response-turn" aria-label="OpenCode 回答">
    <header className="response-author"><span aria-hidden="true">✳</span>OpenCode</header>
    {!!details.length&&<details className="turn-process"><summary><span className="process-chevron" aria-hidden="true">›</span>
      <span>执行过程</span>{tools.length>0&&<small>{tools.length} 次工具调用</small>}
      {failedTools.length>0&&<small className="process-issue">{failedTools.length} 次异常</small>}
      {working&&<small role="status">{runningTool?.kind==='tool'?'正在'+(toolNames[runningTool.name]??runningTool.name):'进行中'}</small>}
    </summary><div className="process-list">{details.map(({step,message,part})=>{
      const key=message+':'+part;
      if(step.kind==='tool')return <ToolRow key={key} step={step}/>;
      if(step.kind==='notice')return <div className={step.problem?'transcript-error':'process-note'} key={key}>{step.at!=null&&<time>{new Date(step.at).toLocaleTimeString()} </time>}{step.text}</div>;
      if(step.kind==='text')return <div className="process-comment" key={key}><Markdown text={step.text}/></div>;
      return null;
    })}</div></details>}
    {thinking.length>0&&<Reasoning session={session} entries={thinking} running={active||thinking.some(({step})=>step.live&&!step.complete)}/>}
    {status&&<p className="transcript-error" role="status">{status}</p>}
    {working&&!details.length&&!thinking.length&&<p className="process-note" role="status">正在处理…</p>}
    {answers.length>0&&<section className="turn-answer" aria-label="回答正文">{answers.map(({message,part,step})=>
      step.kind==='text'&&<Markdown key={message+':'+part} text={step.text}/>)}</section>}
    {files.size>0&&<small className="turn-files">涉及文件 · {[...files].join('、')}</small>}
  </article>;
}

export function ConversationTranscript({session,messages,progress,active}:{
  session:string;messages:ChatMessage[];progress:ToolProgress[];active:boolean;
}){
  const turns=groupMessages(messages);
  let activeIndex=turns.length-1;
  while(activeIndex>=0&&turns[activeIndex].question?.queued)activeIndex--;
  return <div className="conversation-transcript">{turns.map((turn,i)=><section className="conversation-exchange" key={turn.id}>
    {turn.question&&<div className="user-question" aria-label="你的消息"><p>{turn.question.text}</p>
      {turn.question.files.length>0&&<small>引用文件 · {turn.question.files.join('、')}</small>}</div>}
    <Response session={session} messages={turn.response} progress={progress} active={active&&i===activeIndex}/>
  </section>)}{active&&!turns.length&&<p className="process-note" role="status">正在处理…</p>}</div>;
}
