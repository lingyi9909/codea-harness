/**
 * 智能工作台 React 主应用
 *
 * 职责：组织场景侧栏、直接对话、系统管理和页面切换。
 * Java 研发阅读提示：JSX 描述视图，useState/useEffect 管理页面状态和副作用；真实权限仍在后端。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
import {localStorage,sessionStorage} from './userStorage.ts';
import {browserRandomUUID} from './browser-random-id.ts';
import { useCallback, useEffect, useRef, useState } from 'react';
import { api, ApiError } from './api/client.ts';
import type { Schema } from './api/client.ts';
import { subscribeTask } from './api/events.ts';
import Markdown from 'react-markdown';
import { ProfileLogin } from './ProfileLogin.tsx';
import { DirectChat } from './DirectChat.tsx';
import { ModelPicker } from './ModelPicker.tsx';
import type { ChatModel, SceneConfigResponse } from '../../../packages/contracts/src/chat.ts';
import {LogInvestigation} from './LogInvestigation.tsx';
import { FileScene } from './FileScenes.tsx';
import { scenes, scenarioGroups, groupForScene, type Scene, type ScenarioGroup } from './ScenarioHome.tsx';
import { SidebarChatHistory } from './SidebarChatHistory.tsx';
import { SourceView } from './SourceView.tsx';
import { SceneConfigPage } from './SceneConfigPage.tsx';
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
const operationId=()=>browserRandomUUID();
const globalModelKey='harness.global.model';
const onLan=()=>!['localhost','127.0.0.1','::1','[::1]'].includes(window.location.hostname);
type SelectedModel=Pick<ChatModel,'id'|'providerID'|'variant'>;
function savedGlobalModel():SelectedModel|undefined{
  try{
    const raw:unknown=JSON.parse(localStorage.getItem(globalModelKey)??'null');
    if(!raw||typeof raw!=='object'||Array.isArray(raw))return undefined;
    const model=raw as Record<string,unknown>;
    if(typeof model.id!=='string'||!model.id||typeof model.providerID!=='string'||!model.providerID||
      (model.variant!==undefined&&typeof model.variant!=='string'))return undefined;
    return {id:model.id,providerID:model.providerID,
      ...(typeof model.variant==='string'&&model.variant?{variant:model.variant}:{})};
  }catch{return undefined;}
}

/** 阅读提示：React 根业务组件：组织场景、侧栏和管理目录。useState 是页面状态，权限必须由后端重复验证。 */
export function WorkbenchApp() {
  useEffect(()=>{const changed=(event:StorageEvent)=>{if(event.key==='harness.account-change')window.location.reload();};window.addEventListener('storage',changed);return()=>window.removeEventListener('storage',changed);},[]);
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
    return Object.hasOwn(scenes,saved)&&(!onLan()||saved!=='settings')?saved as Scene:'home';
  });
  const [expandedGroup,setExpandedGroup]=useState<ScenarioGroup|null>(()=>groupForScene(scene));
  const [adminVisible,setAdminVisible]=useState(false);
  const [adminExpanded,setAdminExpanded]=useState(true);
  const [chatNavigation,setChatNavigation]=useState<{kind:'new'|'history'|'select';id?:string;seq:number}>({kind:'history',seq:0});
  const [chatDirectory,setChatDirectory]=useState('');
  // One model selection for all OpenCode chats (home, investigations, analytics).
  // Stored on this browser only; the server independently validates each
  // model against the effective bound directory before admitting a request.
  const [globalModel,setGlobalModel]=useState<SelectedModel|undefined>(savedGlobalModel);
  const [sceneModelConfig,setSceneModelConfig]=useState<SceneConfigResponse|null>(null);
  useEffect(()=>{
    if(globalModel)localStorage.setItem(globalModelKey,JSON.stringify(globalModel));
    else localStorage.removeItem(globalModelKey);
  },[globalModel]);
  // Never infer administrator rights from localhost alone. The backend
  // checks the same config-access.json editor ACL used for configuration writes.
  useEffect(()=>{
    if(!bootstrap||onLan()){setAdminVisible(false);return;}
    let live=true;setAdminVisible(false);
    api.accessSettings().then(()=>{if(live)setAdminVisible(true);})
      .catch(()=>{if(live)setAdminVisible(false);});
    return()=>{live=false;};
  },[bootstrap,connectionAttempt]);
  useEffect(()=>{
    if(!bootstrap)return;
    let live=true;setSceneModelConfig(null);
    api.sceneRuntime().then(value=>{if(live)setSceneModelConfig(value);}).catch(()=>{});
    return()=>{live=false;};
  },[bootstrap,scene]);
  useEffect(()=>{
    if(!bootstrap)return;
    api.recordPageVisit(selected||scene==='review'?'review':
      scene==='logs'?'logs':scene==='message'?'message':scene==='data'?'data':
      scene==='settings'?'config':'home');
  },[bootstrap,scene,selected]);
  const sceneDirectory=scene==='logs'?sceneModelConfig?.config.scenes.logs?.directory:
    scene==='message'?sceneModelConfig?.config.scenes.message?.directory:
    scene==='data'?sceneModelConfig?.config.scenes.data?.directory:undefined;
  const modelDirectory=sceneDirectory??(scene==='home'||scene==='chat'?chatDirectory:undefined)??sceneModelConfig?.workspace;
  const modelCommand=scene==='message'&&sceneModelConfig?.config.scenes.message?.executor.type==='command'&&
    sceneModelConfig.config.scenes.message.executor.name==='models';
  const chooseGlobalModel=(model:ChatModel|undefined)=>{
    setGlobalModel(model?{id:model.id,providerID:model.providerID,...(model.variant?{variant:model.variant}:{})}:undefined);
  };
  const navigate=(next:Scene)=>{
    // Scene navigation never creates a native session; the chat restores its own saved id/draft.
    const target=next==='chat'?'home':next;
    setSelected('');setScene(target);setChatDirectory('');setExpandedGroup(groupForScene(target));sessionStorage.setItem('harness.scene',target);
  };
  const openChat=(kind:'new'|'history'|'select',id?:string)=>{
    navigate('home');setChatNavigation(previous=>({kind,id,seq:previous.seq+1}));
  };
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
        const savedScene=sessionStorage.getItem('harness.scene')??'home';
        const restoredScene=Object.hasOwn(scenes,savedScene)&&(!onLan()||savedScene!=='settings')?savedScene as Scene:'home';
        api.setCsrf(boot.csrfToken);
        setScene(restoredScene);setExpandedGroup(groupForScene(restoredScene));
        setGlobalModel(savedGlobalModel());setSelected(sessionStorage.getItem('harness.task')??'');
        setPending(sessionStorage.getItem('harness.operation'));setBootstrap(boot);
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
  if(!bootstrap&&loginMode==='profile')return <ProfileLogin onLogin={async(name)=>{
    await api.login(name);
    window.location.reload();
  }}/>;
  if(!bootstrap&&!loginMode)return <main className="pair-page"><div className="pair-card">
    {error?<><p role="alert">{error}</p><button onClick={reconnect}>重新连接</button></>:<p role="status">正在连接本地工作台…</p>}
  </div></main>;
  if(!bootstrap) return <main className="profile-page pair-page">
    <section className="profile-card pair-login-card" aria-labelledby="pair-title">
      <aside className="profile-side">
        <div className="profile-brand"><strong>智能工作台</strong><span>智能研发场景协作</span></div>
        <div className="profile-side-copy">
          <span className="profile-overline">LOCAL AI WORKSPACE</span>
          <h2>连接你的<br/>研发工作台</h2>
          <p>在统一的本地界面中使用 OpenCode 对话、代码审查、问题排查和数据分析。</p>
        </div>
        <span className="profile-side-footer">本地连接 · 离线可用</span>
      </aside>
      <div className="profile-form-pane">
        <span className="profile-overline">一次性验证</span>
        <h1 id="pair-title">连接工作台</h1>
        <p className="profile-subtitle">输入 Windows 服务启动时显示的一次性配对码。连接本机服务不会启动 OpenCode。</p>
        {error&&<p className="profile-error" role="alert">{error}</p>}
        <label>一次性配对码<input type="password" autoComplete="off" maxLength={256}
          placeholder="请输入本地终端显示的配对码" value={pairingCode}
          onChange={e=>setPairingCode(e.target.value)} onKeyDown={e=>{if(e.key==='Enter'&&pairingCode.trim())void pair();}}/></label>
        <button className="profile-submit" disabled={busy||!pairingCode.trim()} onClick={pair}>
          {busy?'正在连接…':'连接本地工作台'}<span aria-hidden="true">↗</span>
        </button>
        <p className="profile-footnote">仅 localhost · HttpOnly 会话 · 不修改全局 OpenCode 配置</p>
      </div>
    </section>
  </main>;
  return <div className="app-shell">
    <aside className="sidebar">
      <div className="brand"><div><strong>智能工作台</strong><small>智能研发场景协作</small></div></div>
      <button className="new-chat-action" onClick={()=>openChat('new')}><span aria-hidden="true">＋</span> 新建对话</button>
      <nav aria-label="场景导航" className="scene-nav">
        <button className={'side-link '+(!selected&&(scene==='home'||scene==='chat')?'selected':'')} aria-current={!selected&&(scene==='home'||scene==='chat')?'page':undefined} onClick={()=>navigate('home')}><span aria-hidden="true">◫</span> 对话首页</button>
        <div className="side-section">专项场景</div>
        <button className={'side-link '+(!selected&&scene==='review'?'selected':'')}
          aria-current={!selected&&scene==='review'?'page':undefined} onClick={()=>navigate('review')}>
          <span className="scenario-icon review" aria-hidden="true">⌘</span>代码审查
        </button>
        {scenarioGroups.map(group=><div className="scenario-group" key={group.id}>
          <button type="button" className={'side-link scenario-parent '+(expandedGroup===group.id?'expanded':'')}
            aria-expanded={expandedGroup===group.id} aria-controls={'scenario-'+group.id}
            onClick={()=>setExpandedGroup(previous=>previous===group.id?null:group.id)}>
            <span className={'scenario-icon '+group.id} aria-hidden="true">{group.icon}</span>{group.label}
            <span className="scenario-caret" aria-hidden="true">{expandedGroup===group.id?'⌄':'›'}</span>
          </button>
          {expandedGroup===group.id&&<div className="scenario-children" id={'scenario-'+group.id} role="group" aria-label={group.label+'子场景'}>
            {group.children.map(child=><button key={child} className={'side-link scenario-child '+(!selected&&scene===child?'selected':'')}
              aria-current={!selected&&scene===child?'page':undefined} onClick={()=>navigate(child)}>
              <span className="scenario-child-mark" aria-hidden="true">·</span>{scenes[child]}
            </button>)}
          </div>}
        </div>)}
        <button className="side-link mobile-history-link" onClick={()=>openChat('history')}><span aria-hidden="true">▤</span>历史对话</button>
        {adminVisible&&<div className="scenario-group admin-group" role="group" aria-label="系统管理">
          <div className="side-section">管理功能</div>
          <button type="button" className={'side-link scenario-parent '+(adminExpanded?'expanded':'')}
            aria-expanded={adminExpanded} aria-controls="system-admin-pages"
            onClick={()=>setAdminExpanded(open=>!open)}>
            <span className="scenario-icon admin" aria-hidden="true">⚙</span>系统管理
            <span className="scenario-caret" aria-hidden="true">{adminExpanded?'⌄':'›'}</span>
          </button>
          {adminExpanded&&<div className="scenario-children admin-children" id="system-admin-pages" role="group" aria-label="系统管理子页面">
            <a className="side-link scenario-child" href="/config"><span className="scenario-child-mark" aria-hidden="true">·</span>场景配置</a>
            <a className="side-link scenario-child" href="/institution-tasks"><span className="scenario-child-mark" aria-hidden="true">·</span>接口任务管理</a>
            <a className="side-link scenario-child" href="/institution-tasks?view=access"><span className="scenario-child-mark" aria-hidden="true">·</span>接口接入情况</a>
            <a className="side-link scenario-child" href="/analytics"><span className="scenario-child-mark" aria-hidden="true">·</span>访问统计</a>
             <a className="side-link scenario-child" href="/analytics?view=users"><span className="scenario-child-mark" aria-hidden="true">·</span>用户访问记录</a>
            <a className="side-link scenario-child" href="/component-lab"><span className="scenario-child-mark" aria-hidden="true">·</span>组件测试</a>
          </div>}
        </div>}
      </nav>
      <div className="side-section history-heading">最近对话 <button title="查看完整对话历史" onClick={()=>openChat('history')}>全部</button></div>
      <SidebarChatHistory onSelect={id=>openChat('select',id)} onViewAll={()=>openChat('history')}/>
      {(tasks.length>0||selected||scene==='review')&&<>
        <div className="side-section task-history-heading">Review 任务历史 <button title="刷新任务" onClick={()=>refreshLists().catch(e=>setError(text(e)))}>↻</button></div>
        <div className="history task-history">{tasks.map(task=><button key={task.id} className={'history-item '+(selected===task.id?'selected':'')} onClick={()=>{setSelected(task.id);setScene('review');}}><span className="history-title">{task.title}</span><span className="history-meta">{short(task.headOID)} · {task.status}</span></button>)}</div>
      </>}
      <div className="workbench-account"><span className="account-avatar" aria-hidden="true">{bootstrap.owner.name.slice(0,1)}</span><span title={bootstrap.owner.name}>{bootstrap.owner.name}</span>
        <button aria-label="退出登录" disabled={busy} onClick={()=>void act(async()=>{await api.logout();window.location.reload();})}>退出</button>
      </div>
    </aside>
    <main className="main">
      <header className="topbar"><div className="workspace-heading"><strong>工作台</strong>
        <span className="heading-divider" aria-hidden="true"/>
        <span className="workspace-context">{selected?'任务详情':scene==='home'||scene==='chat'?'对话首页':scenes[scene]}</span></div>
        <div className="topbar-tools">
          <div className="global-model-control" title="统一应用于 OpenCode 直接对话及专项场景；代码审查任务仍按 Review 服务配置执行">
            <span className="global-model-label">全局模型</span>
            <ModelPicker key={scene} value={globalModel} allowDefault
              initialOpen={Boolean(modelCommand)}
              loadModels={()=>api.chatModels(modelDirectory||undefined)}
              onSelect={chooseGlobalModel}/>
          </div>
          {selected||scene==='review'?<span className={'state '+(ready?'state-ok':'state-warn')}>{ready?'● 审查服务已就绪':'● 审查服务未就绪'}</span>:
            <span className="workbench-connection"><span className="connection-dot" aria-hidden="true"/>本地工作台已连接</span>}
        </div></header>
      <div className="page">
      {error&&<div className="alert" role="alert">{error}<button onClick={()=>setError('')}>关闭</button></div>}
      {gap&&selected&&<div className="notice" role="status">{gap}<button onClick={()=>{loadDetail(selected).catch(e=>setError(text(e)));setGap('');}}>重新读取</button></div>}
      {pendingOperation&&<div className="notice">操作编号 {short(pendingOperation)}。请求结果不明时请查询，勿重复提交。
        <button disabled={busy} onClick={checkOperation}>查询操作结果</button></div>}
      {!selected&&(scene==='home'||scene==='chat')?<DirectChat navigation={chatNavigation} onDirectoryChange={setChatDirectory} globalModel={globalModel}/>:
       !selected&&(scene==='logs'||scene==='message')?<LogInvestigation key={scene} mode={scene==='message'?'message':'public'} globalModel={globalModel}/>:!selected&&scene==='data'?<DirectChat sceneId="data" onDirectoryChange={setChatDirectory} globalModel={globalModel}/>:!selected&&scene==='settings'&&!onLan()?<SceneConfigPage/>:!selected?<section className="workbench">
        <div className="eyebrow">CODE REVIEW / 固定版本审查</div>
        <h1>代码审查</h1>
        <p className="intro">选择本地仓库和 Git 基线，基于已提交变更运行原生 Codea Review。</p>
        <div className="form-panel">
          <div className="panel-head"><div><h2>选择范围 <span className="step-arrow">→</span> 预览变更 <span className="step-arrow">→</span> 开始审查</h2><p>固定 Git 已提交版本，原仓库保持不变。</p></div><span className="chip">Git · Local</span></div>
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
