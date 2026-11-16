import {useEffect,useLayoutEffect,useMemo,useRef,useState,type ReactNode} from 'react';
import {streamingMessages,useChatStream} from './chatStream.ts';
import {useProgressiveMessages} from './progressiveMessages.ts';
import {ChatHistoryPanel} from './ChatHistoryPanel.tsx';
import {activateChat,displayChatTitle,summarizeFirstInput,draftKey,readDraft,readHistory,rememberChat,savedChat,type SceneKey} from './chatHistory.ts';
import {ChatRequests} from './ChatRequests.tsx';
import {api,ApiError} from './api/client.ts';
import type {ChatStatus,ChatSettings,ChatModel,ChatScenario,SceneId,SceneConfigResponse} from '../../../packages/contracts/src/chat.ts';

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
    SCENE_EXECUTOR_NOT_FOUND:'未发现配置的执行入口。请检查当前子场景的 Skill、Subagent 或命令是否已在绑定目录加载。',
    SCENE_NOT_CONFIGURED:'当前子场景尚未绑定工作目录和执行入口。',
    SCENE_DIRECTORY_CHANGED:'子场景绑定的目录已变更，请新建对话。',
    SCENE_DIRECTORY_MISMATCH:'提交的目录与子场景配置不一致。',
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
type Investigation={controls:ReactNode;initial:boolean;ready:boolean;prompt:(text:string)=>string;onSent:(id:string)=>void;reset:(id:string,rangePrompt?:string)=>void;title:string;sessionKey:'harness.logs'|'harness.message'};
const scenarioFor=(id?:SceneId):ChatScenario=>id==='logs'?'log-inspection':id==='message'?'message-inspection':id==='data'?'data-analysis':'direct-chat';
type ChatNavigation={kind:'new'|'history'|'select';id?:string;seq:number};
type GlobalModel=Pick<ChatModel,'id'|'providerID'|'variant'>;
export function DirectChat({investigation,navigation,onDirectoryChange,sceneId,globalModel}:{investigation?:Investigation;navigation?:ChatNavigation;onDirectoryChange?:(directory:string)=>void;sceneId?:SceneId;globalModel?:GlobalModel}={}){
  const [baseSettings,configure]=useState<ChatSettings|null>(null),[error,fail]=useState('');
  const [sceneSettings,setSceneSettings]=useState<SceneConfigResponse|null>(null);
  const [attempt,retry]=useState(0);
  // A bound scene only needs the workspace configuration. Do not gate its
  // initial form on an unrelated /chat/settings request or the OpenCode server.
  useEffect(()=>{
    if(sceneId)return;
    let live=true;
    fail('');configure(null);
    api.chatSettings().then(s=>{if(live)configure(s);}).catch(e=>{if(live)fail(explain(e));});
    return ()=>{live=false;};
  },[sceneId,attempt]);
  useEffect(()=>{
    if(!sceneId)return;
    let live=true;
    fail('');setSceneSettings(null);
    api.sceneConfig().then(value=>{if(live)setSceneSettings(value);}).catch(e=>{if(live)fail(explain(e));});
    return ()=>{live=false;};
  },[sceneId,attempt]);
  const directory=sceneId?sceneSettings?.config.scenes[sceneId]?.directory??'':baseSettings?.directories[0]??'';
  const settings:ChatSettings|null=sceneId
    ?sceneSettings?{configured:true,directories:[sceneSettings.workspace]}:null
    :baseSettings;
  useEffect(()=>{if(directory)onDirectoryChange?.(directory);},[directory,onDirectoryChange]);
  if(!settings)return <section className="scene-page" role={error?'alert':'status'}>
    {error?<><h1>无法读取工作台配置</h1><p className="notice">{error}</p>
      <p>请确认工作台进程仍在运行，或检查浏览器网络请求；这不是 Skill 加载错误。</p>
      <button className="secondary" type="button" onClick={()=>retry(n=>n+1)}>重试加载</button></>
      :<p>正在读取工作目录与场景配置…</p>}
  </section>;
  if(sceneId&&!directory)return <section className="scene-page">
    <div className="scene-title"><div><h1>当前子场景尚未配置</h1></div></div>
    <p className="notice">请到「场景配置」为此子场景指定工作目录与 Skill、Subagent 或 OpenCode 命令。</p>
    <p>在配置保存之前不会创建 OpenCode 会话，也不会在其他目录中代为执行。</p>
    <button className="secondary" type="button" onClick={()=>retry(n=>n+1)}>重新检查配置</button>
  </section>;
  const modelCommand=sceneId==='message'&&sceneSettings?.config.scenes.message?.executor.type==='command'&&
    sceneSettings.config.scenes.message.executor.name==='models';
  return <ChatWorkspace key={directory+'|'+(investigation?.sessionKey??(sceneId==='data'?'harness.data':'harness.chat'))}
    directory={directory} settings={settings} investigation={investigation} navigation={navigation} sceneId={sceneId}
    modelCommand={modelCommand} globalModel={globalModel}/>;
}
function ChatWorkspace({investigation,directory,settings,navigation,sceneId,modelCommand,globalModel}:{
  investigation?:Investigation;navigation?:ChatNavigation;directory:string;settings:ChatSettings;sceneId?:SceneId;
  modelCommand?:boolean;globalModel?:GlobalModel;
}){
  const scenario=scenarioFor(sceneId);
  const sessionKey:SceneKey=investigation?.sessionKey??(sceneId==='data'?'harness.data':'harness.chat');
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
  useEffect(()=>{window.dispatchEvent(new Event('harness:chat-history'));},[id,cachedTitle]);
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
    fail('');migrating.current=false;restoreContext.current=true;
    activateChat(localStorage,sessionStorage,sessionKey,directory,next);
    window.dispatchEvent(new Event('harness:chat-history'));
  }
  useEffect(()=>{
    if(!navigation?.seq||investigation)return;
    if(navigation.kind==='new')switchTo('');
    else if(navigation.kind==='select'&&navigation.id)switchTo(navigation.id);
    else if(navigation.kind==='history')showHistory(true);
  },[navigation?.seq]);
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
    document.title='等待你回应 · '+(investigation?.title??'直接对话');
    return ()=>{document.title=title;};
  },[pending,Boolean(investigation)]);
  const maySend=Boolean((investigation?.initial?investigation.ready:draft.trim())&&settings.configured&&(!id||state?.id===id)&&!pending&&!busy);
  const send=()=>run(async()=>{
    if(!maySend)return;
    const text=investigation?investigation.prompt(draft.trim()):draft.trim();
    if(text.length>16000)throw Error('排查条件和问题合计过长，请缩短后再发送。');
    let session=id;
    if(!session){
      const title=draft.trim()?summarizeFirstInput(draft):summarizeFirstInput(text,Boolean(investigation));
      const s=await api.newChat(directory||undefined,globalModel,scenario==='direct-chat'?undefined:scenario,title);
      session=s.id;createdSession.current=session;setCachedTitle(s.title||title);
      rememberChat(localStorage,sessionKey,directory,{id:session,title:s.title||title,updated:Date.now()});
      activateChat(localStorage,sessionStorage,sessionKey,directory,session);
      restoreContext.current=false;sessionStorage.removeItem(composerKey);select(session);
    }else if(globalModel&&state?.id===session&&
      (state.model?.id!==globalModel.id||state.model?.providerID!==globalModel.providerID||
        state.model?.variant!==globalModel.variant)){
      // A single global choice also applies to follow-up messages in an old
      // session. Switch first and fail closed if this directory lacks it.
      await api.chatAction(session,'model',globalModel);
      update(previous=>previous?.id===session?{...previous,model:globalModel}:previous);
    }
    await stream.connect(session);
    await api.chatAction(session,'messages',{text,scenario,files:refs.split(/\r?\n/).map(v=>v.trim()).filter(Boolean)});
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
  const homeEmpty=!investigation&&!id&&!messages.length&&!state?.active&&!pending;
  return <section className={investigation?"scene-page log-investigation":"scene-page silver-chat-home "+(homeEmpty?"silver-chat-empty":"silver-chat-active")}><div className="scene-title"><div className="thread-heading"><h1>{investigation?.title??(sceneId==='data'?'公管数据分析':'直接对话')}</h1>
      {currentTitle&&<h2 className="conversation-title" title={currentTitle}>{currentTitle}</h2>}</div>
    <div className="chat-history-actions"><button className="secondary" aria-expanded={history} aria-controls="conversation-history" disabled={busy} onClick={()=>showHistory(!history)}>历史对话</button>
      {id&&<button className="secondary new-conversation" disabled={busy} onClick={()=>switchTo('')}><span aria-hidden="true">＋</span> 新建对话</button>}</div></div>
    {history&&<ChatHistoryPanel scene={sessionKey} directory={directory} active={id} disabled={busy} onSelect={row=>{
      rememberChat(localStorage,sessionKey,directory,row);switchTo(row.id);
    }}/>}
    <div className="chat-layout"><div className="chat-box">
      {(!investigation||messages.length>0||stream.tools.length>0||state?.active)&&<div className="chat-feed" ref={feed} aria-live="polite" onScroll={e=>{
        const el=e.currentTarget;follow.current=el.scrollHeight-el.clientHeight-el.scrollTop<48;
      }}>{!investigation&&!id&&!messages.length&&!state?.active&&<div className="chat-empty"><p className="chat-project-hint">当前项目　/　{directory.split(/[\\/]/).filter(Boolean).at(-1)||'工作目录'}</p><h2>{sceneId==='data'?'公管数据分析':'直接开始你的工作'}</h2><p>{sceneId==='data'?'输入分析需求，在当前绑定目录中通过配置的执行入口进行分析。':'描述问题，OpenCode 将在当前项目中处理。'}</p></div>}
        <ConversationTranscript key={id} session={id} messages={messages} progress={stream.tools} active={active}/>
      </div>}
      {modelCommand&&!id&&<p className="notice" role="status">默认入口 /models 用于选择本次排查使用的模型，
        不是报文排查执行命令。选择模型后可继续用 OpenCode 对话排查。</p>}
      {error&&<p role="alert" className="notice">{error}</p>}
      {pending&&state?<div className="composer request-composer">
        <ChatRequests key={id} state={state} busy={busy}
          onAction={(action,payload)=>run(async()=>{await api.chatAction(id,action,payload);})}/>
        {interrupt&&<div className="composer-actions">{interrupt}</div>}
      </div>:<div className="composer">{investigation&&(!id||state?.id===id)&&<fieldset disabled={busy} className="investigation-controls">{investigation.controls}</fieldset>}<textarea ref={input} readOnly={busy} aria-label="对话内容" rows={2} maxLength={16000} placeholder={investigation?(investigation.initial?'补充排查问题（选填）…':'继续追问这次问题排查…'):'输入问题，或让 OpenCode 修改当前目录中的文件…'} value={draft} onChange={e=>write(e.target.value)}
        onKeyDown={e=>{
          if(e.key!=='Enter'||e.shiftKey||e.altKey||e.nativeEvent.isComposing||e.nativeEvent.keyCode===229)return;
          e.preventDefault();
          if(maySend&&!e.repeat)void send();
        }}/>
        {showFiles&&<label>文件相对路径，每行一个<textarea value={refs} rows={2} onChange={e=>attach(e.target.value)} placeholder="例如 src/app.ts"/></label>}
        <div className="composer-actions">
          <button onClick={()=>toggleFiles(!showFiles)}>＋ 引用文件</button>
          {interrupt}
          <button className="primary" disabled={!maySend} title="Enter 发送，Shift+Enter 换行" onClick={send}>{busy?'提交中…':investigation?.initial?'开始排查':'发送 ↑'}</button></div>
      </div>}
      {!investigation&&!id&&!messages.length&&!pending&&<div className="chat-quick-actions" aria-label="快捷提问">
        <button type="button" onClick={()=>{write('请阅读项目的目录结构和 README，解释各模块的职责。');input.current?.focus();}}>解释项目结构 <span aria-hidden="true">↗</span></button>
        <button type="button" onClick={()=>{write('请从入口开始梳理这个项目的主要调用链。');input.current?.focus();}}>梳理调用链 <span aria-hidden="true">↗</span></button>
      </div>}
    </div></div>
    {homeEmpty&&<p className="chat-workspace-path" title={directory}>工作目录：{directory}</p>}
  </section>;
}
