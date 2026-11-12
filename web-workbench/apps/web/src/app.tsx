import { useCallback, useEffect, useRef, useState } from 'react';
import { api, ApiError } from './api/client.ts';
import type { Schema } from './api/client.ts';
import { subscribeTask } from './api/events.ts';
import Markdown from 'react-markdown';
import { ProfileLogin } from './ProfileLogin.tsx';
import { DirectChat } from './DirectChat.tsx';
import {LogInvestigation} from './LogInvestigation.tsx';
import { FileScene } from './FileScenes.tsx';
import { ScenarioHome, scenes, type Scene } from './ScenarioHome.tsx';
import { SourceView } from './SourceView.tsx';
import { InteractionPanel } from './InteractionPanel.tsx';

type Repository=Schema<'Repository'>;
type Ref=Schema<'Ref'>;
type Task=Schema<'TaskSummary'>;
type Detail=Schema<'TaskDetail'>;
type Interaction=Schema<'Interaction'>;
type Artifact=Schema<'Artifact'>;
type Message=Schema<'Message'>;
const isActive=(status:string)=>['QUEUED','PREPARING','RUNNING','WAITING_INPUT','WAITING_PERMISSION','COLLECTING','STOP_REQUESTED','RECOVERING'].includes(status);
const controllerPattern=/^[A-Za-z_][A-Za-z0-9_]*$/;
const text=(error:unknown)=>error instanceof Error?error.message:'请求失败';
const short=(s:string)=>s.length>12?s.slice(0,10)+'…':s;
const operationId=()=>crypto.randomUUID();

export function WorkbenchApp() {
  const [bootstrap,setBootstrap]=useState<Schema<'Bootstrap'>|null>(null);
  const [loginMode,setLoginMode]=useState<'profile'|'pair'|null>(null);
  const [connectionAttempt,setConnectionAttempt]=useState(0);
  const reconnect=()=>{setBootstrap(null);setLoginMode(null);setError('');setConnectionAttempt(n=>n+1);};
  const [pairingCode,setPairingCode]=useState('');
  const [repos,setRepos]=useState<Repository[]>([]);
  const [tasks,setTasks]=useState<Task[]>([]);
  const [selected,setSelected]=useState(()=>sessionStorage.getItem('harness.task')??'');
  const [scene,setScene]=useState<Scene>(()=>{
    const saved=sessionStorage.getItem('harness.scene')??'home';
    return Object.hasOwn(scenes,saved)?saved as Scene:'home';
  });
  const navigate=(next:Scene)=>{setSelected('');setScene(next);sessionStorage.setItem('harness.scene',next);};
  const selection=useRef(selected);
  selection.current=selected;
  const detailRequest=useRef(0);
  useEffect(()=>{sessionStorage.setItem('harness.task',selected);},[selected]);
  const [repoId,setRepoId]=useState('');
  const [refs,setRefs]=useState<Ref[]>([]);
  const [refId,setRefId]=useState('');
  const [branch,setBranch]=useState('');
  const [head,setHead]=useState('');
  const [controller,setController]=useState('');
  const [notes,setNotes]=useState('');
  const [preview,setPreview]=useState<Schema<'Preview'>|null>(null);
  const [detail,setDetail]=useState<Detail|null>(null);
  const [messages,setMessages]=useState<Message[]>([]);
  const [interactions,setInteractions]=useState<Interaction[]>([]);
  const [artifact,setArtifact]=useState<Artifact|null>(null);
  const [draft,setDraft]=useState('');
  const [quote,setQuote]=useState('');
  const [error,setError]=useState('');
  const [busy,setBusy]=useState(false);
  const [pendingOperation,setPending]=useState(()=>sessionStorage.getItem('harness.operation'));
  const setPendingOperation=(id:string|null)=>{
    if(id)sessionStorage.setItem('harness.operation',id);else sessionStorage.removeItem('harness.operation');
    setPending(id);
  };
  const [eventState,setEventState]=useState('未连接');
  const [gap,setGap]=useState('');
  const [progress,setProgress]=useState<Array<{seq:string;text:string}>>([]);
  const [registerOpen,setRegisterOpen]=useState(false);
  const [newName,setNewName]=useState('');
  const [newPath,setNewPath]=useState('');

  const pair=async()=>{
    setBusy(true);setError('');
    try {
      const result=await api.pair(pairingCode.trim());
      setBootstrap(result);setPairingCode('');
      await refreshLists();
    }catch(e){setError(text(e));}
    finally{setBusy(false);}
  };
  const refreshLists=useCallback(async()=>{
    const [r,t]=await Promise.all([api.repositories(),api.tasks()]);
    setRepos(r.items);setTasks(t.items);
  },[]);
  const loadDetail=useCallback(async(id:string)=>{
    const order=++detailRequest.current;
    const d=await api.task(id);
    const [m,i]=await Promise.all([api.messages(id),api.interactions(id)]);
    const last=d.reports.at(-1);
    const report=last?await api.report(last.id):null;
    if(selection.current!==id||order!==detailRequest.current)return d.lastSeq;
    setDetail(d);setMessages(m.items.reverse());setInteractions(i.items);
    setArtifact(report);return d.lastSeq;
  },[]);
  useEffect(()=>{
    let cancelled=false;
    (async()=>{
      try {
        const boot=await api.bootstrap();
        if(cancelled)return;
        api.setCsrf(boot.csrfToken);setBootstrap(boot);
        await refreshLists();
      }catch(e){
        if(cancelled)return;
        if(e instanceof ApiError&&e.status===401&&e.code==='PROFILE_LOGIN_REQUIRED')setLoginMode('profile');
        else if(e instanceof ApiError&&e.status===401&&e.code==='AUTH_REQUIRED')setLoginMode('pair');
        else setError(text(e));
      }
    })();
    return()=>{cancelled=true;};
  },[refreshLists,connectionAttempt]);
  useEffect(()=>{
    if(!repoId){setRefs([]);setRefId('');setHead('');setBranch('');setPreview(null);return;}
    let active=true;setPreview(null);setRefId('');
    api.refs(repoId).then(v=>{if(active){setRefs(v.items);setHead(v.headOID);setBranch(v.branch??'(detached HEAD)');}})
    .catch(e=>{if(active)setError(text(e));});
    return()=>{active=false;};
  },[repoId]);
  useEffect(()=>{
    if(!selected||!bootstrap){setDetail(null);setInteractions([]);setMessages([]);setArtifact(null);return;}
    let active=true,cleanup=()=>{};
    setDetail(null);setMessages([]);setArtifact(null);setInteractions([]);setGap('');setProgress([]);setDraft(sessionStorage.getItem('harness.draft.'+selected)??'');setQuote('');
    const fail=(e:unknown)=>{if(active){setError(text(e));if(e instanceof ApiError&&e.status===401)reconnect();}};
    let refreshing=false,dirty=false;
    const refresh=async()=>{
      dirty=true;
      if(refreshing)return;
      refreshing=true;
      try{while(active&&dirty){dirty=false;await loadDetail(selected);}}
      catch(e){fail(e);}finally{refreshing=false;}
    };
    loadDetail(selected).then(seq=>{
      if(!active)return;
      cleanup=subscribeTask(selected,{
        onEvent:event=>{
          const label=event.type==='run.status'?event.data.status:
            event.type==='tool.started'?event.data.tool+' · 开始':
            event.type==='tool.completed'?event.data.tool+' · '+event.data.outcome:
            event.type==='artifact.ready'?'已归档报告 v'+event.data.version:null;
          if(label)setProgress(current=>[...current.slice(-99),{seq:event.seq,text:label}]);
          if(event.type==='continuity.gap')setGap('部分原生过程记录缺失');
          if(BigInt(event.seq)>BigInt(seq))void refresh();
        },
        onState:s=>setEventState(s==='connected'?'实时事件已连接':s==='connecting'?'连接中':'已断线'),
        onGap:setGap,
      },'0');
    }).catch(fail);
    return ()=>{active=false;cleanup();};
  },[selected,bootstrap,loadDetail]);

  const act=async(fn:()=>Promise<void>)=>{
    setError('');setBusy(true);
    try{await fn();}catch(e){
      if(e instanceof ApiError && [400,403,409,422,429].includes(e.status))setPendingOperation(null);
      if(e instanceof ApiError && ['SCOPE_CHANGED','PREVIEW_EXPIRED','PREVIEW_USED'].includes(e.code))setPreview(null);
      if(e instanceof ApiError && e.status===401)reconnect();
      setError(text(e));
    }
    finally{setBusy(false);}
  };
  const previewScope=()=>act(async()=>{
    if(!repoId||!refId)throw new Error('请先选择仓库和基线');
    if(controller&&(!controllerPattern.test(controller)||controller.length>128))
      throw new Error('Controller 只能是 128 字符以内的 Java 简单类名');
    if(notes.length>2000)throw new Error('补充要求不能超过 2000 字');
    const p=await api.preview(repoId,refId,operationId());
    setPreview(p);
    if(p.applicableFiles===0)throw new Error('没有可审查的已提交业务变更');
    if(p.changedFiles===0)throw new Error('没有已提交差异');
  });
  const create=()=>act(async()=>{
    if(!preview||pendingOperation)return;
    const createId=operationId();setPendingOperation(createId);
    const op=await api.createTask(preview.id,createId,controller||undefined,notes||undefined);
    if(op.taskId){setSelected(op.taskId);await refreshLists();}
    else setError('任务受理中，请按操作号查询结果，禁止重新提交');
    if(op.taskId)setPendingOperation(null);
  });
  const checkOperation=()=>act(async()=>{
    if(!pendingOperation)return;
    const op=await api.operation(pendingOperation);
    if(op.taskId){setSelected(op.taskId);setPendingOperation(null);await refreshLists();}
    else if(op.status==='SUCCEEDED'||op.status==='FAILED'){setPendingOperation(null);await refreshLists();}
    else setError('操作状态：'+op.status+'；请继续查询，不能重新创建任务');
  });
  const register=()=>act(async()=>{
    const id=operationId();setPendingOperation(id);
    await api.register(newName,newPath,id);setRegisterOpen(false);
    await refreshLists();setPendingOperation(null);
  });
  const answer=(interaction:Interaction,response:Schema<'AnswerRequest'>['answer'])=>act(async()=>{
    const id=operationId();setPendingOperation(id);
    await api.answer(interaction.id,interaction.revision,response,id);
    await loadDetail(selected);setPendingOperation(null);
  });
  const stop=()=>act(async()=>{
    if(!detail)return;
    const id=operationId();setPendingOperation(id);
    await api.stop(detail.latestRun.id,id);await loadDetail(selected);
    setPendingOperation(null);
  });
  const retry=()=>act(async()=>{
    if(!detail)return;
    const id=operationId();setPendingOperation(id);
    await api.retry(detail.latestRun.id,id);await loadDetail(selected);setPendingOperation(null);
  });
  const recheck=()=>act(async()=>{
    if(!selected)return;
    const id=operationId();setPendingOperation(id);
    await api.recheck(selected,id);await loadDetail(selected);setPendingOperation(null);
  });
  const discuss=()=>act(async()=>{
    if(!selected||!draft.trim())return;
    const id=operationId();setPendingOperation(id);
    await api.discuss(selected,draft,id,artifact?.meta.id,quote||undefined);
    sessionStorage.removeItem('harness.draft.'+selected);setDraft('');setQuote('');await loadDetail(selected);setPendingOperation(null);
  });
  const ready=bootstrap?.scenarios.some(s=>s.id==='code-review'&&s.enabled)??false;
  const mutationBusy=busy||Boolean(pendingOperation);
  const active=Boolean(detail&&isActive(detail.latestRun.status));
  if(!bootstrap&&loginMode==='profile')return <ProfileLogin onLogin={async(name,phone)=>{
    const session=await api.login(name,phone);
    setBootstrap(session);
    await refreshLists().catch(e=>setError(text(e)));
  }}/>;
  if(!bootstrap&&!loginMode)return <main className="pair-page"><div className="pair-card">
    {error?<><p role="alert">{error}</p><button onClick={reconnect}>重新连接</button></>:<p role="status">正在连接本地工作台…</p>}
  </div></main>;
  if(!bootstrap) return <div className="pair-page">
    <div className="pair-card"><div className="brand"><span className="brand-icon">✧</span><div><strong>智能工作台</strong></div></div>
      <div className="eyebrow">LOCAL WORKBENCH</div><h1>连接工作台<span className="accent">.</span></h1>
      <p className="intro">输入 Windows 服务启动时显示的一次性配对码。此操作只连接本机服务，不启动 OpenCode。</p>
      {error&&<div className="alert" role="alert">{error}</div>}
      <label>一次性配对码<input type="password" autoComplete="off" maxLength={256}
        placeholder="请输入本地终端显示的配对码" value={pairingCode}
        onChange={e=>setPairingCode(e.target.value)} onKeyDown={e=>{if(e.key==='Enter'&&pairingCode.trim())void pair();}}/></label>
      <button className="primary" disabled={busy||!pairingCode.trim()} onClick={pair}>连接本地工作台</button>
      <p className="muted small">仅 localhost · HttpOnly 会话 · 不会修改全局 OpenCode 配置</p>
    </div>
  </div>;
  return <div className="app-shell">
    <aside className="sidebar">
      <div className="brand"><span className="brand-icon">✧</span><div><strong>智能工作台</strong></div></div>
      <nav aria-label="场景导航" className="scene-nav">{(Object.keys(scenes) as Scene[]).map(key=>
        <button key={key} className={'side-link '+(!selected&&scene===key?'selected':'')} onClick={()=>navigate(key)}>
          <span aria-hidden="true">{{home:'▦',chat:'✳',review:'⌘',logs:'≋',data:'▥'}[key]}</span>{scenes[key]}</button>)}</nav>
      <div className="side-section">任务历史 <button title="刷新任务" onClick={()=>refreshLists().catch(e=>setError(text(e)))}>↻</button></div>
      <div className="history">
        {tasks.map(task=><button key={task.id} className={'history-item '+(selected===task.id?'selected':'')}
          onClick={()=>setSelected(task.id)}>
          <span className="history-title">{task.title}</span><span className="history-meta">{short(task.headOID)} · {task.status}</span></button>)}
        {!tasks.length&&<p className="muted small">暂无历史任务</p>}
      </div>
      <div className="workbench-account"><span title={bootstrap.owner.name}>{bootstrap.owner.name}</span>
        <button disabled={busy} onClick={()=>void act(async()=>{
          await api.logout();
          window.location.reload();
        })}>退出登录</button>
      </div>
    </aside>
    <main className="main">
      <header className="topbar"><div className="breadcrumb">研发场景 <span>/</span> {selected?'任务详情':scenes[scene]}</div>
        {(selected||scene==='review')&&<span className={'state '+(ready?'state-ok':'state-warn')}>{ready?'● 服务已就绪':'● 服务未就绪'}</span>}</header>
      <div className="page">
      {error&&<div className="alert" role="alert">{error}<button onClick={()=>setError('')}>关闭</button></div>}
      {gap&&selected&&<div className="notice" role="status">{gap}<button onClick={()=>{loadDetail(selected).catch(e=>setError(text(e)));setGap('');}}>重新读取</button></div>}
      {pendingOperation&&<div className="notice">操作编号 {short(pendingOperation)}。请求结果不明时请查询，勿重复提交。
        <button disabled={busy} onClick={checkOperation}>查询操作结果</button></div>}
      {!selected&&scene==='home'?<ScenarioHome open={navigate}/>:!selected&&scene==='chat'?<DirectChat/>:
       !selected&&scene==='logs'?<LogInvestigation/>:!selected&&scene==='data'?<FileScene/>:!selected?<section className="workbench">
        <div className="eyebrow">SCENARIO / CODE REVIEW</div>
        <h1>代码审查工作台<span className="accent">.</span></h1>
        <p className="intro">选择本地仓库和 Git 基线，基于已提交变更运行原生 Codea Review。</p>
        <div className="form-panel">
          <div className="panel-head"><div><h2>创建 Review 任务</h2><p>执行前固定代码范围，原仓库保持不变。</p></div><span className="chip">Git · Local</span></div>
          <div className="form-grid">
            <label>本地代码仓库<select value={repoId} onChange={e=>setRepoId(e.target.value)}>
              <option value="">请选择已登记仓库</option>{repos.map(repo=><option value={repo.id} key={repo.id}>{repo.name}</option>)}</select></label>
            <label>基线分支<select value={refId} onChange={e=>{setRefId(e.target.value);setPreview(null);}}>
              <option value="">请选择基线</option>{refs.map(ref=><option key={ref.id} value={ref.id}>{ref.fullName}</option>)}</select></label>
          </div>
          <div className="scope-line"><span>当前分支：<strong>{branch||'—'}</strong></span><span>HEAD：<code>{head?short(head):'—'}</code></span></div>
          <button className="subtle" onClick={()=>setRegisterOpen(x=>!x)}>{registerOpen?'取消添加':'＋ 添加本地仓库'}</button>
          {registerOpen&&<div className="register form-grid"><label>仓库名称<input value={newName} maxLength={100} onChange={e=>setNewName(e.target.value)}/></label>
            <label>本地绝对路径<input value={newPath} maxLength={4096} onChange={e=>setNewPath(e.target.value)} placeholder="D:\\work\\project"/></label>
            <button className="secondary" disabled={busy||!newPath||!newName} onClick={register}>登记仓库</button></div>}
          <label className="full-label">关注入口 <span className="muted">（选填，仅 Java Controller 简名）</span>
            <input maxLength={128} value={controller} onChange={e=>setController(e.target.value)} placeholder="例如：OrderController"/></label>
          <label className="full-label">补充要求 <span className="muted">（选填）</span>
            <textarea disabled={!bootstrap.capabilities.some(c=>c.name==='instruction.entry'&&c.status==='SUPPORTED')} rows={4} maxLength={2000} value={notes} onChange={e=>setNotes(e.target.value)} placeholder="你希望重点核查的事务、幂等、金额、SQL 或权限风险…"/></label>
          <div className="scope-hint"><strong>固定审查范围</strong><span>只审查 Git 已提交变更，不包含 staged、unstaged 或 untracked 文件。</span></div>
          {preview&&<div className="preview-meta">最近预检：{preview.changedFiles} 个已变更文件，{preview.applicableFiles} 个适用文件；{preview.notice}</div>}
          <div className="form-actions"><span>{!ready?'服务尚未就绪，不能开始真实任务':''}</span><button className="primary" disabled={busy||!repoId||!refId} onClick={previewScope}>预览变更范围</button><button className="primary" disabled={busy||!!pendingOperation||!ready||!preview?.applicableFiles} onClick={create}>{busy?'正在处理…':'开始 Review  →'}</button></div>
        </div>
      </section>:<section className="task-page">
        <button className="back" onClick={()=>navigate('home')}>← 返回工作台</button>
        <div className="detail-heading"><div className="eyebrow">REVIEW TASK / {short(selected)}</div>
          <h1>{detail?.task.title??'正在读取任务…'}</h1><p>固定 HEAD：<code>{detail?.scope.headOID??'—'}</code></p>
          <p>基线：{detail?.scope.baseRef??'—'} · merge-base：<code>{detail?.scope.mergeBaseOID??'—'}</code></p></div>
        <div className="detail-grid">
          <div className="content-column">
            <section className="detail-card"><div className="card-head"><h2>执行过程</h2><span className="chip">{detail?.latestRun.status??'读取中'}</span></div>
              <p className="muted">真实执行状态来自服务端。{eventState}。不显示模型推测的进度。</p>
              <ol className="steps">{progress.map(entry=><li className="step" key={entry.seq}>
                <small>#{entry.seq}</small> {entry.text}</li>)}</ol>
              {detail?.latestRun.status==='STOP_REQUESTED'&&<p className="notice">正在停止；等待原生中断与最终状态核验。</p>}
              {detail&&['FAILED','STOPPED'].includes(detail.latestRun.status)&&<button disabled={mutationBusy||!ready} onClick={retry}>重试本轮</button>}
              {active&&<button className="secondary danger" disabled={mutationBusy||detail?.latestRun.status==='STOP_REQUESTED'} onClick={stop}>停止本轮执行</button>}
            </section>
            {detail?.snapshotStatus==='READY'&&<SourceView key={selected} taskId={selected}/>}
            {interactions.map(i=><InteractionPanel key={i.id} item={i} busy={mutationBusy}
              onAnswer={answerPayload=>answer(i,answerPayload)}/>)}
            <section className="detail-card">
              <div className="card-head"><h2>审查报告</h2><span className="chip">{artifact?'已归档':'等待产物'}</span></div>
              {artifact?<><div className="report-actions">
                  <button className="secondary" disabled={mutationBusy||active} onClick={recheck}>重新复查固定版本</button>
                  <button onClick={()=>navigator.clipboard.writeText(artifact.markdown)}>复制 Markdown</button>
                <a href={'/api/v1/artifacts/'+encodeURIComponent(artifact.meta.id)+'/download'}>下载报告</a></div>
                <article className="report-text"><Markdown skipHtml
                  disallowedElements={['img']} urlTransform={url=>/^https?:\/\//i.test(url)?url:''}
                  components={{a:({href,children})=>href?<a href={href} rel="noreferrer noopener" target="_blank">{children}</a>:<>{children}</>}}>
                  {artifact.markdown}</Markdown></article>
                <button className="subtle" onClick={()=>setQuote(window.getSelection()?.toString().slice(0,4000)??'')}>引用所选文本追问</button>
                {quote&&<blockquote className="quote">{quote}</blockquote>}</>
               :<p className="muted">未收到本轮经过验证并归档的真实 Markdown 报告。</p>}
            </section>
            <section className="detail-card"><div className="card-head"><h2>追问与讨论</h2></div>
              <div className="messages">{messages.map(m=><div className={'message '+m.role.toLowerCase()} key={m.id}>
                <strong>{m.role==='USER'?'你':'Codea Reviewer'}</strong><p>{m.text}</p></div>)}
                {!messages.length&&<p className="muted">暂无可显示的原生消息。</p>}</div>
              <textarea rows={3} maxLength={8000} placeholder={active?'本轮执行中；暂不可并发发送':'基于固定代码快照继续追问'}
                value={draft} onChange={e=>{setDraft(e.target.value);sessionStorage.setItem('harness.draft.'+selected,e.target.value);}}/>
              <div className="form-actions"><small>追问沿用原 Session，不改变 Git 范围。</small><button className="primary"
                disabled={mutationBusy||active||!draft.trim()||!detail} onClick={discuss}>发送追问</button></div>
            </section>
          </div>
          <aside className="meta-column"><div className="detail-card"><h2>固定版本</h2>
            <dl><dt>原分支</dt><dd>{detail?.scope.sourceBranch??'—'}</dd><dt>HEAD OID</dt><dd><code>{detail?.scope.headOID??'—'}</code></dd>
              <dt>Base OID</dt><dd><code>{detail?.scope.baseOID??'—'}</code></dd>
              <dt>审查状态</dt><dd>{detail?.latestRun.status??'—'}</dd>
              <dt>事件连续性</dt><dd>{detail?.continuity??'UNKNOWN'}</dd></dl></div></aside>
        </div>
      </section>}
      </div>
    </main>
  </div>;
}
