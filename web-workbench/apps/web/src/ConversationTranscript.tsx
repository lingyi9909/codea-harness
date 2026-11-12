import {lazy,Suspense} from 'react';
import type {ChatMessage,ChatStep,ToolProgress} from '../../../packages/contracts/src/chat.ts';
import './conversation-transcript.css';

const Body=lazy(()=>import('./ChatMarkdown.tsx').then(module=>({default:module.ChatMarkdown})));
function Markdown({text}:{text:string}){
  return <Suspense fallback={<p>{text}</p>}><Body text={text}/></Suspense>;
}

type Entry={message:string;part:number;step:ChatStep;commentary:boolean};
type Turn={id:string;question?:ChatMessage;response:ChatMessage[]};
type Expanded={message:string;part:number};
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

function Response({messages,progress,active,expanded,onExpand}:{
  messages:ChatMessage[];progress:ToolProgress[];active:boolean;expanded?:Expanded;onExpand:(value?:Expanded)=>void;
}){
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
  const details=entries.filter((e,i)=>e.step.kind!=='text'||e.commentary||i<lastTool);
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
      if(step.live)return <div className="live-thinking" key={key}><strong>思考过程</strong><Markdown text={step.text??''}/></div>;
      const detailPart=step.detailPart??part;
      const open=expanded?.message===message&&expanded.part===detailPart;
      return <div key={key} className="saved-thinking"><button type="button" aria-expanded={open}
        onClick={()=>onExpand(open?undefined:{message,part:detailPart})}>思考过程 {open?'▾':'▸'}</button>
        {open&&(step.text?<Markdown text={step.text}/>:<small>正在读取…</small>)}</div>;
    })}</div></details>}
    {status&&<p className="transcript-error" role="status">{status}</p>}
    {working&&!details.length&&<p className="process-note" role="status">正在处理…</p>}
    {answers.length>0&&<section className="turn-answer" aria-label="回答正文">{answers.map(({message,part,step})=>
      step.kind==='text'&&<Markdown key={message+':'+part} text={step.text}/>)}</section>}
    {files.size>0&&<small className="turn-files">涉及文件 · {[...files].join('、')}</small>}
  </article>;
}

export function ConversationTranscript({messages,progress,active,expanded,onExpand}:{
  messages:ChatMessage[];progress:ToolProgress[];active:boolean;expanded?:Expanded;onExpand:(value?:Expanded)=>void;
}){
  const turns=groupMessages(messages);
  let activeIndex=turns.length-1;
  while(activeIndex>=0&&turns[activeIndex].question?.queued)activeIndex--;
  return <div className="conversation-transcript">{turns.map((turn,i)=><section className="conversation-exchange" key={turn.id}>
    {turn.question&&<div className="user-question" aria-label="你的消息"><p>{turn.question.text}</p>
      {turn.question.files.length>0&&<small>引用文件 · {turn.question.files.join('、')}</small>}</div>}
    <Response messages={turn.response} progress={progress} active={active&&i===activeIndex}
      expanded={expanded} onExpand={onExpand}/>
  </section>)}{active&&!turns.length&&<p className="process-note" role="status">正在处理…</p>}</div>;
}
