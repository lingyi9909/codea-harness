import {lazy,Suspense,useEffect,useRef,useState,type ReactNode} from 'react';
import {streamingMessages,useChatStream} from './chatStream.ts';
import {ModelPicker} from './ModelPicker.tsx';
import {ChatRequests} from './ChatRequests.tsx';
import {api,ApiError} from './api/client.ts';
import type {ChatStatus,ChatSettings,ChatAccessEntry,ChatModel,ChatStep} from '../../../packages/contracts/src/chat.ts';

const MarkdownBody=lazy(()=>import('./ChatMarkdown.tsx').then(module=>({default:module.ChatMarkdown})));
function ChatMarkdown({text}:{text:string}){
  return <Suspense fallback={<p>{text}</p>}><MarkdownBody text={text}/></Suspense>;
}
function explain(e:unknown){
  const labels:Record<string,string>={DIRECTORY_OUTSIDE_CHAT_ROOTS:'请选择已配置的工作目录或其子目录。',FILE_OUTSIDE_WORKSPACE_OR_MISSING:'引用的文件不存在，或位于当前工作目录之外。',
    SERVICE_UNAVAILABLE:'OpenCode 暂时无法响应，请检查内网服务地址和鉴权配置。',PERMISSION_CHANGED:'授权请求已改变，请查看最新请求。',CHAT_NOT_FOUND:'原会话不可用，请新建对话。'};
  Object.assign(labels,{
    ABSOLUTE_DIRECTORY_REQUIRED:'请输入项目文件夹的绝对路径。',DIRECTORY_NOT_FOUND:'工作台找不到这个文件夹，请检查路径。',
    FORM_CHANGED:'这个问题已更新或已经处理，请查看当前请求。',FORM_ANSWER_INVALID:'这份回答不符合问题要求，请检查后再提交。',FORM_SERVICE_UNAVAILABLE:'暂时无法读取待处理问题，请检查 OpenCode 连接。',
    CHAT_MODEL_UNAVAILABLE:'此模型当前未启用，请刷新模型列表后重新选择。',
    OPENCODE_DISCOVERY_FAILED:'未能自动连接本机 OpenCode。请确认已安装的 opencode 在 PATH 中，并支持 serve --service。',
    OPENCODE_WORKBENCH_URL:'OpenCode 服务地址填成了工作台自身的地址。请重启工作台，填写 OpenCode 后台服务地址。',
    OPENCODE_CONNECTION_FAILED:'创建会话时无法连接 OpenCode。请核对后台服务地址、端口和 HTTPS 证书。',
    OPENCODE_TIMEOUT:'创建会话等待 OpenCode 超过 25 秒，请查看 OpenCode 服务日志。',
    OPENCODE_SESSION_RESPONSE:'创建会话时收到不兼容的响应，请核对服务地址和公司 OpenCode API 版本。',
    OPENCODE_HTTP_401:'OpenCode 返回 401。请检查启动工作台时的 OpenCode 鉴权配置。',
    OPENCODE_HTTP_403:'OpenCode 返回 403。请核对账号或网关是否允许创建会话。',
    OPENCODE_HTTP_404:'OpenCode 返回 404。请核对服务根地址和此版本是否支持 POST /api/session。',
  });
  if(e instanceof ApiError&&/^OPENCODE_HTTP_\d{3}$/.test(e.code)&&!labels[e.code])
    return '创建会话时，OpenCode 服务或其网关返回 HTTP '+e.code.slice(-3)+'。';
  return e instanceof ApiError?(labels[e.code]??e.message):String(e);
}
type Investigation={controls:ReactNode;initial:boolean;ready:boolean;prompt:(text:string)=>string;onSent:()=>void;reset:()=>void};
export function DirectChat({investigation}:{investigation?:Investigation}={}){
  const sessionKey=investigation?'harness.logs':'harness.chat';
  const [settings,configure]=useState<ChatSettings|null>(null);
  const [preferred,setPreferred]=useState(()=>localStorage.getItem('harness.chat.project')??'');
  const [editing,edit]=useState(false),[pathText,typePath]=useState('');
  const [chosen,setChosen]=useState<ChatModel>();
  const [id,select]=useState(()=>sessionStorage.getItem(sessionKey)??'');
  const stream=useChatStream(id);
  const [expanded,expand]=useState<{session:string;message:string;part:number}>();
  const [state,update]=useState<ChatStatus|null>(null),[draft,write]=useState(''),[refs,attach]=useState('');
  const [showFiles,toggleFiles]=useState(false),[busy,wait]=useState(false),[error,fail]=useState(''),[refresh,bump]=useState(0);
  useEffect(()=>{let live=true;api.chatSettings().then(s=>{if(live){configure(s);}}).catch(e=>{if(live)fail(explain(e));});return()=>{live=false;};},[]);
  useEffect(()=>{
    sessionStorage.setItem(sessionKey,id);if(!id)return;
    let live=true;let timer:ReturnType<typeof setTimeout>;
    const poll=async()=>{let interval=1800;try{const value=await api.chat(id,expanded?.session===id?expanded:undefined);if(value.active)interval=900;if(live){update(value);stream.prune.current(value.messages);}}
      catch(e){if(live)fail(explain(e));}finally{if(live)timer=setTimeout(poll,interval);}};
    void poll();return()=>{live=false;clearTimeout(timer);};
  },[id,refresh,sessionKey,expanded,stream.requestsVersion]);
  async function run(fn:()=>Promise<void>){if(busy)return;wait(true);fail('');try{await fn();bump(n=>n+1);}catch(e){fail(explain(e));}finally{wait(false);}}
  const directory=preferred||settings?.directories[0]||'';
  const current=id?state?.directory:directory;
  const saveProject=()=>run(async()=>{
    const saved=await api.chatProject(pathText.trim().replace(/^"(.*)"$/,'$1'));
    localStorage.setItem('harness.chat.project',saved.directory);setPreferred(saved.directory);
    if(saved.directory!==current){
      investigation?.reset();
      select('');update(null);setChosen(undefined);attach('');toggleFiles(false);
      if(id)write('');
    }
    edit(false);
  });
  const input=useRef<HTMLTextAreaElement>(null);
  const pending=state?.id===id&&Boolean(state.forms?.length||state.permissions.length);
  useEffect(()=>{
    if(!pending)return;
    const title=document.title;
    document.title='等待你回应 · '+(investigation?'日志排查':'直接对话');
    return ()=>{document.title=title;};
  },[pending,Boolean(investigation)]);
  const maySend=Boolean((investigation?.initial?investigation.ready:draft.trim())&&settings?.configured&&!pending&&!busy&&!editing);
  async function selectModel(model:ChatModel|undefined){
    if(id&&model){
      await api.chatAction(id,'model',{id:model.id,providerID:model.providerID,variant:model.variant});
      update(previous=>previous?.id===id?{...previous,model}:previous);
      bump(n=>n+1);
    }else setChosen(model);
  }
  const send=()=>run(async()=>{
    if(!maySend)return;
    const text=investigation?investigation.prompt(draft.trim()):draft.trim();
    if(text.length>16000)throw Error('排查条件和问题合计过长，请缩短后再发送。');
    let session=id;
    if(!session){const s=await api.newChat(directory||undefined,chosen?{id:chosen.id,providerID:chosen.providerID,variant:chosen.variant}:undefined,investigation?'log-inspection':undefined);session=s.id;select(session);}
    await stream.connect(session);
    await api.chatAction(session,'messages',{text,files:refs.split(/\r?\n/).map(v=>v.trim()).filter(Boolean)});
    investigation?.onSent();write('');attach('');toggleFiles(false);
  });
  const feed=useRef<HTMLDivElement>(null),follow=useRef(true);
  useEffect(()=>{follow.current=true;},[id]);
  useEffect(()=>{
    if(follow.current&&feed.current)feed.current.scrollTop=feed.current.scrollHeight;
  },[state,stream.parts,stream.tools]);
  const messages=streamingMessages(state?.id===id?state.messages:[],stream.parts);
  return <section className={investigation?"scene-page log-investigation":"scene-page"}><div className="scene-title"><div><h1>{investigation?'日志排查':'直接对话'}</h1><p>{investigation?'按请求和时间定位日志，在同一会话中追问。':'和 OpenCode 在同一工作目录里读代码、改文件，或继续上一轮讨论。'}</p></div>
    {id&&<button className="secondary" disabled={busy||editing} onClick={()=>{investigation?.reset();select('');update(null);write('');attach('');fail('');}}>新建对话</button>}</div>
    <div className="chat-layout"><div className="chat-box">
      <div className="chat-feed" ref={feed} aria-live="polite" onScroll={e=>{
        const el=e.currentTarget;follow.current=el.scrollHeight-el.clientHeight-el.scrollTop<48;
      }}>{!messages.length&&!state?.active&&<div className="chat-empty"><span className="chat-symbol" aria-hidden="true">✳</span><h2>{investigation?'要排查哪次请求？':'想在这个目录里做什么？'}</h2>
        {!investigation&&<button onClick={()=>write('请阅读 README，告诉我这个项目如何启动。')}>阅读 README，梳理启动步骤 ↗</button>}</div>}
        {messages.map(m=><div key={m.id} className={'chat-turn '+m.role}><strong>{m.role==='user'?'你':'OpenCode'}</strong>
          {m.steps?.length?m.steps.map((step,i)=>{
            if(step.kind!=='thinking')return <ExecutionStep key={i} step={step}/>;
            if(step.live)return <details key={i} open className="thinking-step"><summary>思考过程 · {step.complete?'已结束':'进行中'}</summary><ChatMarkdown text={step.text??''}/></details>;
            const savedSteps=state?.messages.find(row=>row.id===m.id)?.steps;
            const part=savedSteps?.indexOf(step)??i;
            const open=expanded?.session===id&&expanded.message===m.id&&expanded.part===part;
            return <div className="thinking-step" key={i}><button type="button" aria-expanded={open} onClick={()=>
              expand(open?undefined:{session:id,message:m.id,part})}>思考过程 {open?'▾':'▸'}</button>
              <small>{step.complete?'已结束':state?.active?'进行中':'已记录'}{step.at!=null&&step.ended!=null?' · '+Math.max(0,(step.ended-step.at)/1000).toFixed(1)+' 秒':''}</small>
              {open&&(step.text?<ChatMarkdown text={step.text}/>:<p>{step.hasText?'正在读取…':'OpenCode 未提供可显示文本。'}</p>)}</div>;
          }):<>{m.text&&(m.role==='assistant'?<ChatMarkdown text={m.text}/>:<p>{m.text}</p>)}{m.tools.map((t,i)=><small key={i}>{t.name} · {t.status}</small>)}</>}
          {m.files.length>0&&<small>{m.role==='user'?'引用文件':'涉及文件'} · {m.files.join('、')}</small>}</div>)}
        {stream.tools.length>0&&<section className="execution-note" aria-label="工具实时进度"><strong>工具实时进度</strong>
          {stream.tools.map(row=><div key={row.id}><time>{new Date(row.at).toLocaleTimeString()}</time> {row.text}</div>)}</section>}
        {state?.active&&!pending&&<small role="status">OpenCode 正在处理本轮请求…</small>}
      </div>
      {state?.id===id&&<ChatRequests key={id} state={state} busy={busy}
        onAction={(action,payload)=>void run(async()=>{await api.chatAction(id,action,payload);})}/>}
      {error&&<p role="alert" className="notice">{error}</p>}
      <div className="composer">{investigation&&<fieldset disabled={busy||editing} className="investigation-controls">{investigation.controls}</fieldset>}<textarea ref={input} readOnly={busy} aria-label="对话内容" rows={3} maxLength={16000} placeholder={investigation?(investigation.initial?'补充排查问题（选填）…':'继续追问这次日志排查…'):'输入问题，或让 OpenCode 修改当前目录中的文件…'} value={draft} onChange={e=>write(e.target.value)}
        onKeyDown={e=>{
          if(e.key!=='Enter'||e.shiftKey||e.altKey||e.nativeEvent.isComposing||e.nativeEvent.keyCode===229)return;
          e.preventDefault();
          if(maySend&&!e.repeat)void send();
        }}/>
        {showFiles&&<label>文件相对路径，每行一个<textarea value={refs} rows={2} onChange={e=>attach(e.target.value)} placeholder="例如 src/app.ts"/></label>}
        <div className="composer-actions">
          <ModelPicker key={JSON.stringify([current,id])} value={id?state?.model:chosen} allowDefault={!id}
            disabled={busy||editing||!settings||Boolean(pending)||Boolean(id&&!state)}
            loadModels={()=>api.chatModels(current||undefined)} onSelect={selectModel} onApplied={()=>input.current?.focus()}/>
          <button onClick={()=>toggleFiles(!showFiles)}>＋ 引用文件</button>
          {state?.active&&<button disabled={busy} onClick={()=>run(async()=>{await api.chatAction(id,'interrupt');})}>中断执行</button>}
          <button className="primary" disabled={!maySend} title="Enter 发送，Shift+Enter 换行" onClick={send}>{busy?'提交中…':investigation?.initial?'开始排查':'发送 ↑'}</button></div>
      </div></div><aside className="context-box"><h2>工作目录</h2>
        {editing?<form onSubmit={e=>{e.preventDefault();void saveProject();}}>
          <label>项目文件夹路径<input autoFocus value={pathText} list="chat-projects" disabled={busy}
            placeholder="例如 D:\projects\order-service" onChange={e=>typePath(e.target.value)}/></label>
          <datalist id="chat-projects">{Array.from(new Set([directory,...(settings?.directories??[])])).filter(Boolean).map(d=><option key={d} value={d}/>)}</datalist>
          {id&&<p>更换目录后将新建对话，原会话仍使用原目录。</p>}
          <button disabled={busy||!pathText.trim()}>使用此目录</button>
          <button type="button" disabled={busy} onClick={()=>{edit(false);fail('');}}>取消</button>
        </form>:<div><code className="project-path">{current??'加载中…'}</code>
          <button disabled={busy||!settings} onClick={()=>{typePath(current??directory);edit(true);fail('');}}>更改工作目录</button></div>}
        <h2>后台访问日志</h2><AccessLog/>
        {state&&<p>{pending?'等待你回应':state.active?'正在执行':state.outcome==='failed'?'本轮执行失败':state.outcome==='interrupted'?'本轮已中断':'等待输入'}</p>}
      </aside></div></section>;
}

function AccessLog(){
  const [rows,setRows]=useState<ChatAccessEntry[]>([]),[error,fail]=useState('');
  useEffect(()=>{
    let live=true,timer:ReturnType<typeof setTimeout>;
    const poll=async()=>{
      try{const value=await api.chatAccess();if(live){setRows(value);fail('');}}
      catch(e){if(live)fail(explain(e));}
      finally{if(live)timer=setTimeout(poll,2500);}
    };
    void poll();return()=>{live=false;clearTimeout(timer);};
  },[]);
  return <div className="access-log">{error&&<p role="alert">{error}</p>}{!rows.length&&<p>等待访问记录…</p>}
    {rows.map((row,i)=><div key={i}><strong>{row.source} · {row.result}</strong>
      <code>{row.operation}</code><small>{new Date(row.time).toLocaleTimeString()} · {row.ms} ms · {row.requestId}</small></div>)}</div>;
}

const toolStates:Record<string,string>={streaming:'正在准备调用',running:'执行中',completed:'已完成',error:'失败',exited:'已退出',timeout:'超时',killed:'已停止'};
function ExecutionStep({step}:{step:Exclude<ChatStep,{kind:'thinking'}>}){
  if(step.kind==='text')return <ChatMarkdown text={step.text}/>;
  const time=step.at?new Date(step.at).toLocaleTimeString():undefined;
  if(step.kind==='notice')return <div className="execution-note">{time&&<time>{time} </time>}{step.text}</div>;
  return <div className="execution-tool"><header><strong>{step.name}</strong><span>{toolStates[step.status]??step.status}</span>
    {time&&<time>{time}</time>}</header>
    {step.progress&&<p>{step.progress}</p>}
    {step.input&&<details><summary>调用参数</summary><pre>{step.input}</pre></details>}
    {step.result&&<><div className="tool-result">{step.result.length>180?step.result.slice(0,180)+'…':step.result}</div>
      {step.result.length>180&&<details><summary>展开结果摘录</summary><pre>{step.result}</pre></details>}</>}
    {step.error&&<p className="notice">工具错误 · {step.error}</p>}
  </div>;
}
