import {useEffect,useState} from 'react';
import Markdown from 'react-markdown';
import {api,ApiError,type InstitutionAdminTaskDetail,type InstitutionTaskPage,type InstitutionTaskStatus} from './api/client.ts';
import {ProfileLogin} from './ProfileLogin.tsx';
import './institution-tasks.css';

const statusLabels:Record<InstitutionTaskStatus,string>={
  QUEUED:'排队中',RUNNING:'执行中',SUCCEEDED:'已完成',FAILED:'执行失败',UNKNOWN:'状态未知',
};
const statuses:(InstitutionTaskStatus|'ALL')[]=['ALL','QUEUED','RUNNING','SUCCEEDED','FAILED','UNKNOWN'];
const formatTime=(value:string)=>new Date(value).toLocaleString('zh-CN',{
  timeZone:'Asia/Shanghai',hour12:false,year:'numeric',month:'2-digit',day:'2-digit',
  hour:'2-digit',minute:'2-digit',second:'2-digit',
});
const errorText=(e:unknown)=>e instanceof Error?e.message:'查询失败';
const active=(value:InstitutionTaskStatus)=>value==='QUEUED'||value==='RUNNING';

export function InstitutionTaskManagementPage(){
  const [mode,setMode]=useState<'loading'|'profile'|'pair'|'ready'|'denied'|'error'>('loading');
  const [error,setError]=useState('');
  const [pairCode,setPairCode]=useState(''),[pairBusy,setPairBusy]=useState(false);
  const [filter,setFilter]=useState<InstitutionTaskStatus|'ALL'>('ALL');
  const [query,setQuery]=useState(''),[appliedQuery,setAppliedQuery]=useState('');
  const [offset,setOffset]=useState(0);
  const [refresh,setRefresh]=useState(0);
  const [page,setPage]=useState<InstitutionTaskPage|null>(null);
  const [listError,setListError]=useState(''),[loading,setLoading]=useState(false);
  const [selectedId,setSelectedId]=useState('');
  const [detail,setDetail]=useState<InstitutionAdminTaskDetail|null>(null);
  const [detailError,setDetailError]=useState('');

  useEffect(()=>{
    let live=true;
    api.bootstrap().then(boot=>{
      if(!live)return;
      api.setCsrf(boot.csrfToken);
      setMode('ready');
    }).catch(e=>{
      if(!live)return;
      if(e instanceof ApiError&&e.status===401)
        setMode(e.code==='PROFILE_LOGIN_REQUIRED'?'profile':'pair');
      else{setError(errorText(e));setMode('error');}
    });
    return()=>{live=false;};
  },[]);

  // The B API is asynchronous: while this page is open, refresh both the list
  // and selected result every five seconds. Neither action launches new tasks.
  useEffect(()=>{
    if(mode!=='ready')return;
    const timer=window.setInterval(()=>{
      if(!document.hidden)setRefresh(n=>n+1);
    },5000);
    return()=>window.clearInterval(timer);
  },[mode]);

  useEffect(()=>{
    if(mode!=='ready')return;
    let live=true;
    setLoading(true);
    api.institutionTasks(filter,appliedQuery,offset).then(data=>{
      if(live){setPage(data);setListError('');}
    }).catch(e=>{
      if(!live)return;
      if(e instanceof ApiError&&e.status===403){setMode('denied');return;}
      if(e instanceof ApiError&&e.status===401){setMode('profile');return;}
      setListError(errorText(e));
    }).finally(()=>{if(live)setLoading(false);});
    return()=>{live=false;};
  },[mode,filter,appliedQuery,offset,refresh]);

  useEffect(()=>{
    if(mode!=='ready'||!selectedId)return;
    let live=true;
    api.institutionTask(selectedId).then(data=>{
      if(live){setDetail(data);setDetailError('');}
    }).catch(e=>{
      if(!live)return;
      if(e instanceof ApiError&&e.status===403){setMode('denied');return;}
      setDetailError(errorText(e));
    });
    return()=>{live=false;};
  },[mode,selectedId,refresh]);

  if(mode==='profile')return <ProfileLogin onLogin={async(name,phone)=>{
    await api.login(name,phone);setMode('ready');
  }}/>;
  if(mode==='pair')return <main className="pair-page"><form className="pair-card" onSubmit={e=>{
    e.preventDefault();if(pairBusy)return;setPairBusy(true);setError('');
    void api.pair(pairCode).then(()=>setMode('ready')).catch(e=>setError(errorText(e))).finally(()=>setPairBusy(false));
  }}><h1>接口任务管理</h1><label>本机配对码
    <input type="password" required value={pairCode} onChange={e=>setPairCode(e.target.value)} autoComplete="off"/>
  </label>{error&&<p role="alert">{error}</p>}
  <button type="submit" disabled={pairBusy||!pairCode.trim()}>连接工作台</button></form></main>;
  if(mode==='denied')return <main className="institution-tasks-page">
    <header className="institution-tasks-header"><div><small>ACCESS RESTRICTED</small><h1>无任务管理权限</h1>
      <p>此页面仅限 C 服务器本机，并要求当前登录用户名属于配置管理文件的 editors 名单。</p>
    </div><a href="/">返回工作台</a></header>
    <p role="alert">请由管理员检查启动目录下的 .workbench/config-access.json；任务数据未开放给当前用户。</p>
  </main>;
  if(mode!=='ready')return <main className="institution-tasks-page"><p role={error?'alert':'status'}>{error||'正在验证本机管理权限…'}</p></main>;

  return <main className="institution-tasks-page">
    <header className="institution-tasks-header">
      <div><small>INSTITUTION API OPERATIONS</small><h1>接口任务管理</h1>
        <p>集中查看平台 B 通过接口创建的机构报文排查任务、执行状态和最终返回结论。仅本机配置管理员可查看。</p></div>
      <nav aria-label="管理页面导航"><a href="/config">场景配置</a><a href="/">返回工作台</a></nav>
    </header>
    <div className="institution-tasks-toolbar">
      <form onSubmit={e=>{e.preventDefault();setAppliedQuery(query.trim());setOffset(0);}}>
        <label htmlFor="institution-task-search">任务 ID / 请求号</label>
        <input id="institution-task-search" type="search" value={query} onChange={e=>setQuery(e.target.value)}
          maxLength={128} placeholder="输入完整任务 ID 或 requestId"/>
        <button type="submit">查询</button>
        {(query||appliedQuery)&&<button type="button" className="institution-tasks-secondary" onClick={()=>{
          setQuery('');setAppliedQuery('');setOffset(0);
        }}>清除</button>}
      </form>
      <label htmlFor="institution-task-status">状态
        <select id="institution-task-status" value={filter} onChange={e=>{
          setFilter(e.target.value as InstitutionTaskStatus|'ALL');setOffset(0);
        }}>{statuses.map(s=><option key={s} value={s}>{s==='ALL'?'全部状态':statusLabels[s]}</option>)}</select>
      </label>
      <button type="button" className="institution-tasks-secondary" onClick={()=>setRefresh(n=>n+1)}>刷新</button>
    </div>
    <div className="institution-tasks-meta">
      <span>当前结果：{page?.total??'—'} 条</span>
      <span>{loading?'刷新中…':'每 5 秒自动刷新'} · 数据保留 7 天 · 北京时间</span>
    </div>
    {listError&&<p className="institution-tasks-error" role="alert">{listError}</p>}
    <div className="institution-tasks-layout">
      <section className="institution-tasks-list" aria-label="接口任务列表">
        <div className="institution-tasks-table-wrap">
          <table><thead><tr><th>任务 / 请求号</th><th>排查日期</th><th>执行状态</th><th>创建时间</th><th>操作</th></tr></thead>
            <tbody>{page?.items.map(task=><tr key={task.taskId} className={selectedId===task.taskId?'selected':''}>
              <td><strong title={task.taskId}>{task.taskId.slice(0,8)}…</strong>
                <small title={task.requestId}>{task.requestId}</small></td>
              <td>{task.day}</td>
              <td><span className={'institution-task-status '+task.status.toLowerCase()}>
                {statusLabels[task.status]}</span></td>
              <td>{formatTime(task.createdAt)}</td>
              <td><button type="button" className="institution-tasks-open" aria-label={'查看任务 '+task.taskId}
                onClick={()=>{setSelectedId(task.taskId);setDetail(null);setDetailError('');}}>详情</button></td>
            </tr>)}</tbody></table>
        </div>
        {!page&&!listError&&<p className="institution-tasks-empty">正在读取任务…</p>}
        {page?.items.length===0&&<p className="institution-tasks-empty">暂无符合条件的接口任务</p>}
        <div className="institution-tasks-pager">
          <button type="button" disabled={offset===0} onClick={()=>setOffset(n=>Math.max(0,n-50))}>上一页</button>
          <span>第 {Math.floor(offset/50)+1} 页</span>
          <button type="button" disabled={!page||offset+page.limit>=page.total} onClick={()=>setOffset(n=>n+50)}>下一页</button>
        </div>
      </section>
      <section className="institution-tasks-detail" aria-label="任务执行详情">
        {!selectedId?<div className="institution-tasks-empty">选择左侧任务，查看执行状态与最终返回结果。</div>:<>
          <header><h2>任务详情</h2><button type="button" className="institution-tasks-secondary"
            onClick={()=>setRefresh(n=>n+1)}>更新状态</button></header>
          {detailError&&<p role="alert" className="institution-tasks-error">{detailError}</p>}
          {!detail&&!detailError?<p className="institution-tasks-empty">正在加载任务详情…</p>:detail&&<>
            <div className="institution-tasks-fields">
              <span>状态</span><strong><span className={'institution-task-status '+detail.status.toLowerCase()}>
                {statusLabels[detail.status]}</span></strong>
              <span>任务 ID</span><code>{detail.taskId}</code>
              <span>来源系统</span><span>{detail.clientId}</span>
              <span>请求号</span><span>{detail.requestId}</span>
              <span>用户标识摘要</span><span>{detail.userHash}…</span>
              <span>排查日期</span><span>{detail.day}</span>
              <span>报文编号</span><span>{detail.msgSn??'已清理（任务结束后不保留）'}</span>
              <span>创建时间</span><span>{formatTime(detail.createdAt)}</span>
              <span>最后更新</span><span>{formatTime(detail.updatedAt)}</span>
            </div>
            {detail.errorCode&&<p className="institution-tasks-error" role="alert">异常代码：{detail.errorCode}</p>}
            <div className="institution-tasks-answer"><h3>最终返回结果</h3>
              {detail.result?.summary?<div className="institution-tasks-markdown"><Markdown>{detail.result.summary}</Markdown></div>:
                <p className="institution-tasks-empty">{active(detail.status)?'任务尚未完成，完成后会自动显示最终结论。':
                  detail.status==='UNKNOWN'?'执行状态无法确认，需人工核验；不会自动重跑。':
                  detail.status==='FAILED'?'任务执行失败，未产生最终结论。':'暂无最终返回结果。'}</p>}
            </div>
          </>}
        </>}
      </section>
    </div>
  </main>;
}
