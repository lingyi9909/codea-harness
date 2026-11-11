import {useEffect,useState} from 'react';
import {api,ApiError} from './api/client.ts';
import type {ChatStatus,ChatSettings} from '../../../packages/contracts/src/chat.ts';

function explain(e:unknown){
  const labels:Record<string,string>={DIRECTORY_OUTSIDE_CHAT_ROOTS:'请选择已配置的工作目录或其子目录。',FILE_OUTSIDE_WORKSPACE_OR_MISSING:'引用的文件不存在，或位于当前工作目录之外。',
    SERVICE_UNAVAILABLE:'OpenCode 暂时无法响应，请检查内网服务地址和鉴权配置。',PERMISSION_CHANGED:'授权请求已改变，请查看最新请求。',CHAT_NOT_FOUND:'原会话不可用，请新建对话。'};
  return e instanceof ApiError?(labels[e.code]??e.message):String(e);
}
export function DirectChat(){
  const [settings,configure]=useState<ChatSettings|null>(null),[directory,choose]=useState('');
  const [id,select]=useState(()=>sessionStorage.getItem('harness.chat')??'');
  const [state,update]=useState<ChatStatus|null>(null),[draft,write]=useState(''),[refs,attach]=useState('');
  const [showFiles,toggleFiles]=useState(false),[busy,wait]=useState(false),[error,fail]=useState(''),[refresh,bump]=useState(0);
  useEffect(()=>{let live=true;api.chatSettings().then(s=>{if(live){configure(s);choose(s.directories[0]??'');}}).catch(e=>{if(live)fail(explain(e));});return()=>{live=false;};},[]);
  useEffect(()=>{
    sessionStorage.setItem('harness.chat',id);if(!id)return;
    let live=true;let timer:ReturnType<typeof setTimeout>;
    const poll=async()=>{try{const value=await api.chat(id);if(live)update(value);}
      catch(e){if(live)fail(explain(e));}finally{if(live)timer=setTimeout(poll,1800);}};
    void poll();return()=>{live=false;clearTimeout(timer);};
  },[id,refresh]);
  async function run(fn:()=>Promise<void>){if(busy)return;wait(true);fail('');try{await fn();bump(n=>n+1);}catch(e){fail(explain(e));}finally{wait(false);}}
  const send=()=>run(async()=>{
    const text=draft.trim();if(!text)return;
    let session=id;
    if(!session){const s=await api.newChat(directory);session=s.id;select(session);}
    await api.chatAction(session,'messages',{text,files:refs.split(/\r?\n/).map(v=>v.trim()).filter(Boolean)});
    write('');attach('');toggleFiles(false);
  });
  return <section className="scene-page"><div className="scene-title"><div><h1>直接对话</h1><p>和 OpenCode 在同一工作目录里读代码、改文件，或继续上一轮讨论。</p></div>
    {id&&<button className="secondary" disabled={busy} onClick={()=>{select('');update(null);write('');attach('');fail('');}}>新建对话</button>}</div>
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
      <div className="composer"><textarea aria-label="对话内容" rows={3} maxLength={16000} placeholder="输入问题，或让 OpenCode 修改当前目录中的文件…" value={draft} onChange={e=>write(e.target.value)}
        onKeyDown={e=>{if(e.key==='Enter'&&(e.ctrlKey||e.metaKey)&&settings?.configured&&directory&&!busy)void send();}}/>
        {showFiles&&<label>文件相对路径，每行一个<textarea value={refs} rows={2} onChange={e=>attach(e.target.value)} placeholder="例如 src/app.ts"/></label>}
        <div className="composer-actions"><button onClick={()=>toggleFiles(!showFiles)}>＋ 引用文件</button>
          {state?.active&&<button disabled={busy} onClick={()=>run(async()=>{await api.chatAction(id,'interrupt');})}>中断执行</button>}
          <button className="primary" disabled={busy||!settings?.configured||!directory||!draft.trim()} onClick={send}>{busy?'提交中…':'发送 ↑'}</button></div>
      </div></div><aside className="context-box"><h2>工作目录</h2><label>OpenCode 运行位置<input list="chat-roots" value={state?.directory??directory} disabled={!!id||busy} onChange={e=>choose(e.target.value)} placeholder="输入绝对路径"/></label>
        <datalist id="chat-roots">{settings?.directories.map(d=><option key={d} value={d}/>)}</datalist>
        <p>文件修改会写入此目录。工具权限沿用 OpenCode 配置。</p>
        {settings&&!settings.configured&&<p className="setup-note">待连接 OpenCode。请在服务端设置 HARNESS_OPENCODE_URL 和 HARNESS_CHAT_ROOTS。</p>}
        {state&&<p>{state.active?'正在执行':state.outcome==='failed'?'本轮执行失败':state.outcome==='interrupted'?'本轮已中断':'等待输入'}</p>}
      </aside></div></section>;
}
