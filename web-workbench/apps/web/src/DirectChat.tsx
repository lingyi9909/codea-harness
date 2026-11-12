import {useEffect,useLayoutEffect,useMemo,useRef,useState,type ReactNode} from 'react';
import {streamingMessages,useChatStream} from './chatStream.ts';
import {useProgressiveMessages} from './progressiveMessages.ts';
import {ChatHistoryPanel} from './ChatHistoryPanel.tsx';
import {activateChat,displayChatTitle,summarizeFirstInput,draftKey,readDraft,readHistory,rememberChat,savedChat,type SceneKey} from './chatHistory.ts';
import {ModelPicker} from './ModelPicker.tsx';
import {ChatRequests} from './ChatRequests.tsx';
import {api,ApiError} from './api/client.ts';
import type {ChatStatus,ChatSettings,ChatModel} from '../../../packages/contracts/src/chat.ts';

import {ConversationTranscript} from './ConversationTranscript.tsx';

function explain(e:unknown){
  const labels:Record<string,string>={DIRECTORY_OUTSIDE_CHAT_ROOTS:'该目录不在启动时配置的工作范围内。',FILE_OUTSIDE_WORKSPACE_OR_MISSING:'引用的文件不存在，或位于当前工作目录之外。',
    SERVICE_UNAVAILABLE:'OpenCode 暂时无法响应，请检查内网服务地址和鉴权配置。',PERMISSION_CHANGED:'授权请求已改变，请查看最新请求。',CHAT_NOT_FOUND:'原会话不可用，请新建对话。'};
  Object.assign(labels,{
    ABSOLUTE_DIRECTORY_REQUIRED:'请输入项目文件夹的绝对路径。',DIRECTORY_NOT_FOUND:'工作台找不到这个文件夹，请检查路径。',
    LOG_QUERY_SKILL_MISSING:'当前工作目录的 OpenCode 未加载自定义 Skill「log-query」。请在启动时指定已配置该 Skill 的内网项目目录。',
    LOG_QUERY_SKILL_AMBIGUOUS:'当前目录有多个同名的 log-query，请在 OpenCode 中保留要用于排查的那一个后重试。',
    LOG_QUERY_CATALOG_UNAVAILABLE:'无法读取当前工作目录的 OpenCode Skill 列表。请检查 OpenCode 服务连接后重试。',
    CHAT_SCENARIO_MISMATCH:'这个会话属于其他场景，请在当前页新建对话。',
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
type Investigation={controls:ReactNode;initial:boolean;ready:boolean;prompt:(text:string)=>string;onSent:(id:string)=>void;reset:(id:string,rangePrompt?:string)=>void};
export function DirectChat({investigation}:{investigation?:Investigation}={}){
  const [settings,configure]=useState<ChatSettings|null>(null),[error,fail]=useState('');
  useEffect(()=>{let live=true;api.chatSettings().then(s=>{if(live)configure(s);}).catch(e=>{if(live)fail(explain(e));});return()=>{live=false;};},[]);
  if(!settings)return <p role={error?'alert':'status'}>{error||'正在加载工作目录…'}</p>;
  const directory=settings.directories[0]??'';
  return <ChatWorkspace key={directory} directory={directory} settings={settings} investigation={investigation}/>;
}
function ChatWorkspace({investigation,directory,settings}:{
  investigation?:Investigation;directory:string;settings:ChatSettings;
}){
  const sessionKey:SceneKey=investigation?'harness.logs':'harness.chat';
  const [chosen,setChosen]=useState<ChatModel>();
  const [id,select]=useState(()=>savedChat(localStorage,sessionStorage,sessionKey,directory));
  const [cachedTitle,setCachedTitle]=useState(()=>readHistory(localStorage,sessionKey,directory).find(row=>row.id===id)?.title??'');
  const migrating=useRef(Boolean(id)&&!sessionStorage.getItem(sessionKey+'.scoped'));
  const [history,showHistory]=useState(false);
  const restoreContext=useRef(true),investigationRef=useRef(investigation);
  investigationRef.current=investigation;
  const composerKey=draftKey(sessionKey,directory,id);
  const [initialDraft]=useState(()=>readDraft(composerKey));
  const stream=useChatStream(id);
  const createdSession=useRef('');
  const [state,update]=useState<ChatStatus|null>(null),[draft,write]=useState(initialDraft.text),[refs,attach]=useState(initialDraft.files);
  const [showFiles,toggleFiles]=useState(Boolean(initialDraft.files)),[busy,wait]=useState(false),[error,fail]=useState(''),[refresh,bump]=useState(0);
  useEffect(()=>{
    if(draft||refs)sessionStorage.setItem(composerKey,JSON.stringify({text:draft,files:refs}));
    else sessionStorage.removeItem(composerKey);
  },[composerKey,draft,refs]);
  function switchTo(next:string){
    showHistory(false);
    if(next===id)return;
    const saved=readDraft(draftKey(sessionKey,directory,next));
    setCachedTitle(readHistory(localStorage,sessionKey,directory).find(row=>row.id===next)?.title??'');
    select(next);update(null);createdSession.current='';write(saved.text);attach(saved.files);toggleFiles(Boolean(saved.files));
    setChosen(undefined);fail('');migrating.current=false;restoreContext.current=true;
    activateChat(localStorage,sessionStorage,sessionKey,directory,next);
  }
  useEffect(()=>{
    if(!id){
      activateChat(localStorage,sessionStorage,sessionKey,directory,'');
      if(restoreContext.current){restoreContext.current=false;investigationRef.current?.reset('');}
      return;
    }
    let live=true;let timer:ReturnType<typeof setTimeout>;
    const poll=async()=>{let interval=1800;try{
      const value=await api.chat(id);
      if(!live)return;
      if(value.active)interval=900;
      if(restoreContext.current){
        if(migrating.current&&value.directory!==directory){
          rememberChat(localStorage,sessionKey,value.directory,{id,title:displayChatTitle(value.title,summarizeFirstInput(value.messages.find(m=>m.role==='user')?.text,Boolean(investigationRef.current))),updated:Date.now()});
          switchTo('');return;
        }
        migrating.current=false;restoreContext.current=false;
        const first=value.messages.find(m=>m.role==='user')?.text;
        const known=readHistory(localStorage,sessionKey,directory).find(row=>row.id===id);
        const title=displayChatTitle(value.title,known?.title||summarizeFirstInput(first,Boolean(investigationRef.current)));
        if(known?.title!==title)rememberChat(localStorage,sessionKey,directory,{id,title,updated:known?.updated??Date.now()});
        setCachedTitle(title);
        activateChat(localStorage,sessionStorage,sessionKey,directory,id);
        const rangePrompt=value.messages.slice().reverse().find(m=>m.role==='user'&&/^时间范围（起点含、终点不含）：/m.test(m.text));
        investigationRef.current?.reset(id,rangePrompt?.text);
      }
      update(value);stream.prune.current(value.messages);
    }catch(e){
      if(!live)return;
      if(migrating.current&&e instanceof ApiError&&e.code==='DIRECTORY_OUTSIDE_CHAT_ROOTS')switchTo('');
      else fail(explain(e));
    }finally{if(live)timer=setTimeout(poll,interval);}};
    void poll();return()=>{live=false;clearTimeout(timer);};
  },[id,refresh,sessionKey,directory,stream.requestsVersion]);
  async function run(fn:()=>Promise<void>){if(busy)return false;wait(true);fail('');try{await fn();bump(n=>n+1);return true;}catch(e){fail(explain(e));return false;}finally{wait(false);}}
  const currentTitle=id?displayChatTitle(state?.id===id?state.title:undefined,cachedTitle):'';
  const input=useRef<HTMLTextAreaElement>(null);
  const pending=state?.id===id&&Boolean(state.forms?.length||state.permissions.length);
  useEffect(()=>{
    if(!pending)return;
    const title=document.title;
    document.title='等待你回应 · '+(investigation?'日志排查':'直接对话');
    return ()=>{document.title=title;};
  },[pending,Boolean(investigation)]);
  const maySend=Boolean((investigation?.initial?investigation.ready:draft.trim())&&settings.configured&&(!id||state?.id===id)&&!pending&&!busy);
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
    if(!session){
      const title=draft.trim()?summarizeFirstInput(draft):summarizeFirstInput(text,Boolean(investigation));
      const s=await api.newChat(directory||undefined,chosen?{id:chosen.id,providerID:chosen.providerID,variant:chosen.variant}:undefined,investigation?'log-inspection':undefined,title);
      session=s.id;createdSession.current=session;setCachedTitle(s.title||title);
      rememberChat(localStorage,sessionKey,directory,{id:session,title:s.title||title,updated:Date.now()});
      activateChat(localStorage,sessionStorage,sessionKey,directory,session);
      restoreContext.current=false;sessionStorage.removeItem(composerKey);select(session);
    }
    await stream.connect(session);
    await api.chatAction(session,'messages',{text,scenario:investigation?'log-inspection':'direct-chat',files:refs.split(/\r?\n/).map(v=>v.trim()).filter(Boolean)});
    const prior=readHistory(localStorage,sessionKey,directory).find(row=>row.id===session);
    rememberChat(localStorage,sessionKey,directory,{id:session,title:currentTitle||prior?.title||'',updated:Date.now()});
    investigation?.onSent(session);write('');attach('');toggleFiles(false);
  });
  const feed=useRef<HTMLDivElement>(null),follow=useRef(true);
  const snapshot=state?.id===id?state:undefined;
  const streamAhead=stream.parts.some(part=>!snapshot?.messages.some(m=>m.id===part.message));
  const active=Boolean(!pending&&(snapshot?.active||streamAhead));
  const merged=useMemo(()=>streamingMessages(snapshot?.messages??[],stream.parts),[snapshot,stream.parts]);
  const messages=useProgressiveMessages(id,merged,snapshot?.messages,Boolean(snapshot?.active),createdSession.current===id);
  useLayoutEffect(()=>{follow.current=true;},[id]);
  useLayoutEffect(()=>{
    if(follow.current&&feed.current)feed.current.scrollTop=feed.current.scrollHeight;
  },[messages,stream.tools]);
  const interrupt=state?.id===id&&state.active&&<button className="running-action" disabled={busy} onClick={()=>run(async()=>{await api.chatAction(id,'interrupt');})}>正在执行 · 中断执行</button>;
  return <section className={investigation?"scene-page log-investigation":"scene-page"}><div className="scene-title"><div className="thread-heading"><h1>{investigation?'日志排查':'直接对话'}</h1>
      {currentTitle&&<h2 className="conversation-title" title={currentTitle}>{currentTitle}</h2>}</div>
    <div className="chat-history-actions"><button className="secondary" aria-expanded={history} aria-controls="conversation-history" disabled={busy} onClick={()=>showHistory(!history)}>历史对话</button>
      {id&&<button className="secondary" disabled={busy} onClick={()=>switchTo('')}>新建对话</button>}</div></div>
    {history&&<ChatHistoryPanel scene={sessionKey} directory={directory} active={id} disabled={busy} onSelect={row=>{
      rememberChat(localStorage,sessionKey,directory,row);switchTo(row.id);
    }}/>}
    <div className="chat-layout"><div className="chat-box">
      {(!investigation||messages.length>0||stream.tools.length>0||state?.active)&&<div className="chat-feed" ref={feed} aria-live="polite" onScroll={e=>{
        const el=e.currentTarget;follow.current=el.scrollHeight-el.clientHeight-el.scrollTop<48;
      }}>{!investigation&&!messages.length&&!state?.active&&<div className="chat-empty"><span className="chat-symbol" aria-hidden="true">✳</span><h2>想在这个目录里做什么？</h2>
        <button onClick={()=>write('请阅读 README，告诉我这个项目如何启动。')}>阅读 README，梳理启动步骤 ↗</button></div>}
        <ConversationTranscript key={id} messages={messages} progress={stream.tools} active={active}/>
      </div>}
      {error&&<p role="alert" className="notice">{error}</p>}
      {pending&&state?<div className="composer request-composer">
        <ChatRequests key={id} state={state} busy={busy}
          onAction={(action,payload)=>run(async()=>{await api.chatAction(id,action,payload);})}/>
        {interrupt&&<div className="composer-actions">{interrupt}</div>}
      </div>:<div className="composer">{investigation&&(!id||state?.id===id)&&<fieldset disabled={busy} className="investigation-controls">{investigation.controls}</fieldset>}<textarea ref={input} readOnly={busy} aria-label="对话内容" rows={2} maxLength={16000} placeholder={investigation?(investigation.initial?'补充排查问题（选填）…':'继续追问这次日志排查…'):'输入问题，或让 OpenCode 修改当前目录中的文件…'} value={draft} onChange={e=>write(e.target.value)}
        onKeyDown={e=>{
          if(e.key!=='Enter'||e.shiftKey||e.altKey||e.nativeEvent.isComposing||e.nativeEvent.keyCode===229)return;
          e.preventDefault();
          if(maySend&&!e.repeat)void send();
        }}/>
        {showFiles&&<label>文件相对路径，每行一个<textarea value={refs} rows={2} onChange={e=>attach(e.target.value)} placeholder="例如 src/app.ts"/></label>}
        <div className="composer-actions">
          <ModelPicker key={JSON.stringify([directory,id])} value={id?state?.model:chosen} allowDefault={!id}
            disabled={busy||!settings||Boolean(pending)||Boolean(id&&!state)}
            loadModels={()=>api.chatModels(directory||undefined)} onSelect={selectModel} onApplied={()=>input.current?.focus()}/>
          <button onClick={()=>toggleFiles(!showFiles)}>＋ 引用文件</button>
          {interrupt}
          <button className="primary" disabled={!maySend} title="Enter 发送，Shift+Enter 换行" onClick={send}>{busy?'提交中…':investigation?.initial?'开始排查':'发送 ↑'}</button></div>
      </div>}</div></div></section>;
}
