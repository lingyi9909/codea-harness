import {useEffect,useRef,useState} from 'react';
import {api,ApiError} from './api/client.ts';
import type {ChatStatus,ChatSettings,ChatAccessEntry,ChatModel} from '../../../packages/contracts/src/chat.ts';

function explain(e:unknown){
  const labels:Record<string,string>={DIRECTORY_OUTSIDE_CHAT_ROOTS:'请选择已配置的工作目录或其子目录。',FILE_OUTSIDE_WORKSPACE_OR_MISSING:'引用的文件不存在，或位于当前工作目录之外。',
    SERVICE_UNAVAILABLE:'OpenCode 暂时无法响应，请检查内网服务地址和鉴权配置。',PERMISSION_CHANGED:'授权请求已改变，请查看最新请求。',CHAT_NOT_FOUND:'原会话不可用，请新建对话。'};
  Object.assign(labels,{
    ABSOLUTE_DIRECTORY_REQUIRED:'请输入项目文件夹的绝对路径。',DIRECTORY_NOT_FOUND:'工作台找不到这个文件夹，请检查路径。',
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
export function DirectChat(){
  const [settings,configure]=useState<ChatSettings|null>(null);
  const [preferred,setPreferred]=useState(()=>localStorage.getItem('harness.chat.project')??'');
  const [editing,edit]=useState(false),[pathText,typePath]=useState('');
  const [catalog,setCatalog]=useState<ChatModel[]|null>(null),[modelKey,setModelKey]=useState('');
  const keyFor=(m:ChatModel)=>JSON.stringify([m.providerID,m.id]);
  const chosen=catalog?.find(m=>keyFor(m)===modelKey);
  const [id,select]=useState(()=>sessionStorage.getItem('harness.chat')??'');
  const [state,update]=useState<ChatStatus|null>(null),[draft,write]=useState(''),[refs,attach]=useState('');
  const [showFiles,toggleFiles]=useState(false),[busy,wait]=useState(false),[error,fail]=useState(''),[refresh,bump]=useState(0);
  useEffect(()=>{let live=true;api.chatSettings().then(s=>{if(live){configure(s);}}).catch(e=>{if(live)fail(explain(e));});return()=>{live=false;};},[]);
  useEffect(()=>{
    sessionStorage.setItem('harness.chat',id);if(!id)return;
    let live=true;let timer:ReturnType<typeof setTimeout>;
    const poll=async()=>{try{const value=await api.chat(id);if(live)update(value);}
      catch(e){if(live)fail(explain(e));}finally{if(live)timer=setTimeout(poll,1800);}};
    void poll();return()=>{live=false;clearTimeout(timer);};
  },[id,refresh]);
  async function run(fn:()=>Promise<void>){if(busy)return;wait(true);fail('');try{await fn();bump(n=>n+1);}catch(e){fail(explain(e));}finally{wait(false);}}
  const directory=preferred||settings?.directories[0]||'';
  const current=id?state?.directory:directory;
  const saveProject=()=>run(async()=>{
    const saved=await api.chatProject(pathText.trim().replace(/^"(.*)"$/,'$1'));
    localStorage.setItem('harness.chat.project',saved.directory);setPreferred(saved.directory);
    if(saved.directory!==current){
      select('');update(null);setCatalog(null);setModelKey('');attach('');toggleFiles(false);
      if(id)write('');
    }
    edit(false);
  });
  const input=useRef<HTMLTextAreaElement>(null);
  const picker=useRef<HTMLDetailsElement>(null);
  const maySend=Boolean(draft.trim()&&settings?.configured&&!busy&&!editing);
  const modelLabel=state?.model?state.model.providerID+'/'+state.model.id:chosen?.name??(id&&!state?'会话模型':'默认模型');
  const loadModels=()=>run(async()=>{
    const list=await api.chatModels(directory||undefined);setCatalog(list);
    if(!list.some(m=>keyFor(m)===modelKey))setModelKey('');
  });
  const send=()=>run(async()=>{
    const text=draft.trim();if(!text||editing)return;
    let session=id;
    if(!session){const s=await api.newChat(directory||undefined,chosen?{id:chosen.id,providerID:chosen.providerID}:undefined);session=s.id;select(session);}
    await api.chatAction(session,'messages',{text,files:refs.split(/\r?\n/).map(v=>v.trim()).filter(Boolean)});
    write('');attach('');toggleFiles(false);
  });
  return <section className="scene-page"><div className="scene-title"><div><h1>直接对话</h1><p>和 OpenCode 在同一工作目录里读代码、改文件，或继续上一轮讨论。</p></div>
    {id&&<button className="secondary" disabled={busy||editing} onClick={()=>{select('');update(null);write('');attach('');fail('');}}>新建对话</button>}</div>
    <div className="chat-layout"><div className="chat-box">
      <div className="chat-feed" aria-live="polite">{!state?.messages.length&&<div className="chat-empty"><span className="chat-symbol" aria-hidden="true">✳</span><h2>想在这个目录里做什么？</h2>
        <button onClick={()=>write('请阅读 README，告诉我这个项目如何启动。')}>阅读 README，梳理启动步骤 ↗</button></div>}
        {state?.messages.map(m=><div key={m.id} className={'chat-turn '+m.role}><strong>{m.role==='user'?'你':'OpenCode'}</strong>
          {m.text&&<p>{m.text}</p>}{m.tools.map((t,i)=><small key={i}>{t.name} · {t.status}</small>)}
          {m.files.length>0&&<small>{m.role==='user'?'引用文件':'涉及文件'} · {m.files.join('、')}</small>}</div>)}
        {state?.permissions.map(p=><div className="chat-permission" key={p.id}><strong>OpenCode 请求授权 · {p.action}</strong><pre>{p.resources.join('\n')}</pre>
          {(['once','reject'] as const).map(d=><button key={d} disabled={busy} onClick={()=>run(async()=>{await api.chatAction(id,'permission',{requestId:p.id,revision:p.revision,decision:d});})}>{d==='once'?'仅本次允许':'拒绝'}</button>)}</div>)}
      </div>
      {error&&<p role="alert" className="notice">{error}</p>}
      <div className="composer"><textarea ref={input} readOnly={busy} aria-label="对话内容" rows={3} maxLength={16000} placeholder="输入问题，或让 OpenCode 修改当前目录中的文件…" value={draft} onChange={e=>write(e.target.value)}
        onKeyDown={e=>{
          if(e.key!=='Enter'||e.shiftKey||e.altKey||e.nativeEvent.isComposing||e.nativeEvent.keyCode===229)return;
          e.preventDefault();
          if(maySend&&!e.repeat)void send();
        }}/>
        {showFiles&&<label>文件相对路径，每行一个<textarea value={refs} rows={2} onChange={e=>attach(e.target.value)} placeholder="例如 src/app.ts"/></label>}
        <div className="composer-actions">
          {id?<span className="current-model" title={modelLabel}>{modelLabel}</span>:
            <details className="model-picker" ref={picker}
              onToggle={e=>{if(e.currentTarget.open&&!catalog&&!busy&&!editing)void loadModels();}}
              onBlur={e=>{if(e.relatedTarget&&!e.currentTarget.contains(e.relatedTarget))e.currentTarget.open=false;}}
              onKeyDown={e=>{if(e.key==='Escape'){e.currentTarget.open=false;input.current?.focus();}}}>
              <summary aria-label="选择模型" title={modelLabel} onClick={e=>{if(editing||busy||!settings)e.preventDefault();}}>{chosen?.name??'选择模型'}</summary>
              <div className="model-options">
                <label>OpenCode 模型<select disabled={busy||editing||!catalog} value={modelKey} onChange={e=>{
                  setModelKey(e.target.value);if(picker.current)picker.current.open=false;input.current?.focus();
                }}><option value="">使用 OpenCode 默认模型</option>{catalog?.map(m=><option key={keyFor(m)} value={keyFor(m)}>{m.name} · {m.providerID}/{m.id}</option>)}</select></label>
                {catalog?.length===0&&<small>当前项目没有已启用的模型。</small>}
                <button type="button" disabled={busy||editing} onClick={loadModels}>{busy?'加载中…':'刷新模型列表'}</button>
              </div>
            </details>}
          <button onClick={()=>toggleFiles(!showFiles)}>＋ 引用文件</button>
          {state?.active&&<button disabled={busy} onClick={()=>run(async()=>{await api.chatAction(id,'interrupt');})}>中断执行</button>}
          <button className="primary" disabled={!maySend} title="Enter 发送，Shift+Enter 换行" onClick={send}>{busy?'提交中…':'发送 ↑'}</button></div>
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
        {state&&<p>{state.active?'正在执行':state.outcome==='failed'?'本轮执行失败':state.outcome==='interrupted'?'本轮已中断':'等待输入'}</p>}
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
