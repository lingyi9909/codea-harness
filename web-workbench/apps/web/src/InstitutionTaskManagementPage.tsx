/**
 * 接口任务管理主页面
 *
 * 职责：展示管理员任务列表、状态筛选、最终结论与多个管理子视图。
 * Java 研发阅读提示：useState 管理局部显示状态，useEffect 负责周期查询，不会因此重新执行模型。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
/**
 * 「系统管理 → 接口任务管理」的页面入口。
 *
 * 与 Java MVC 最大的区别：React 组件同时描述「页面 UI」和「页面状态」。
 * 不要把 useState 中的变量理解成数据库数据；持久化状态仍由 C 的 SQLite 管理。
 *
 * 本页面负责权限态、筛选、自动刷新、任务详情；
 * 并发配置和按用户聚合拆在 InstitutionTaskOperations.tsx。
 */
import {useEffect,useState} from 'react';
import Markdown from 'react-markdown';
import {api,ApiError,type InstitutionAccessOverview,type InstitutionAdminTaskDetail,type InstitutionTaskPage,type InstitutionTaskStatus} from './api/client.ts';
import {ProfileLogin} from './ProfileLogin.tsx';
import {InstitutionConcurrencyControl,InstitutionUserGroupsView} from './InstitutionTaskOperations.tsx';
import './institution-tasks.css';

// Record<状态, 文案>：用 TS 声明完整的枚举映射，类似 Java 的 Map<Enum,String>。
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
/** Server-reported terminal diagnostic codes: no guessing whether a job really ran. */
const taskFailureReason:Record<string,string>={
  PROCESS_RESTARTED:'服务重启时任务尚未完成；原任务不会自动重跑',
  SERVER_STOPPED:'工作台服务关闭时任务仍在执行或排队',
  INSTITUTION_EXECUTION_TIMEOUT:'等待原生执行结果超过 10 分钟',
  INSTITUTION_POLL_UNAVAILABLE:'OpenCode 状态接口连续连接失败；任务是否完成仍需核验',
  INSTITUTION_EXECUTION_UNAVAILABLE:'模型、执行环境或服务请求异常；请检查 OpenCode 连接',
  INSTITUTION_WORKER_UNEXPECTED:'任务执行调度出现未预期异常',
  INSTITUTION_NATIVE_EXECUTION_FAILED:'OpenCode 已明确返回执行失败',
  INSTITUTION_PERMISSION_REQUIRED:'任务执行期间需要人工权限授权，已停止等待',
  INSTITUTION_EMPTY_RESULT:'原生会话执行完毕，但没有返回有效结论',
  INSTITUTION_SKILL_NOT_CONFIGURED:'机构排查 Skill 尚未配置',
  INSTITUTION_SKILL_NOT_FOUND:'配置的机构排查 Skill 不存在',
  INSTITUTION_MODEL_NOT_CONFIGURED:'没有可用的配置模型',
};
const describeTaskFailure=(code:string)=>taskFailureReason[code]||
  (code.startsWith('OPENCODE_HTTP_')?'OpenCode 接口返回 HTTP '+code.slice('OPENCODE_HTTP_'.length):
  '请检查工作台后端异常日志和 OpenCode Session');
// 通过 URL query 切换子视图：?view=users / ?view=access / 默认任务列表。
const pageView=new URLSearchParams(window.location.search).get('view');
const accessView=pageView==='access';
const usersView=pageView==='users';

/**
 * 「接口接入情况」只展示过去 7 天的调用记录，并不管理平台授权。
 * 这里的 clientId 是 C 服务配置标签，不证明请求者真实来自哪个平台。
 */
function AccessOverviewView({refresh,onForbidden}:{refresh:number;onForbidden:()=>void}){
  const [overview,setOverview]=useState<InstitutionAccessOverview|null>(null);
  const [error,setError]=useState('');
  // React effect 在 refresh 变化时重新发起 GET。通过 live 避免卸载后更新 UI。
  useEffect(()=>{
    let live=true;
    api.institutionAccessOverview().then(data=>{if(live){setOverview(data);setError('');}})
      .catch(e=>{
        if(!live)return;
        if(e instanceof ApiError&&(e.status===403||e.status===401)){
          setOverview(null);onForbidden();return;
        }
        setError(errorText(e));
      });
    return()=>{live=false;};
  },[refresh,onForbidden]);
  return <section className="institution-access-section" aria-label="机构接口接入情况">
    <div className="institution-access-warning">
      <strong>当前接口：无应用层鉴权 · 无 IP 白名单</strong>
      <p>尚未向任何平台分配可验证的调用凭证。所有网络上能够连接 C 服务的客户端均可请求机构排查接口。下表是接口调用时提供的用户标识记录，<b>不是已授权平台或人员名单</b>。</p>
      <p>来源平台“{overview?.currentPlatformLabel??'platform-b'}”来自 C 的配置标签，不代表服务端已确认请求确实来自平台 B。</p>
    </div>
    {error&&<p className="institution-tasks-error" role="alert">{error}</p>}
    <div className="institution-access-summary">
      <div><span>访问控制模式</span><strong>开放直连（不鉴权）</strong></div>
      <div><span>平台身份验证</span><strong>未启用</strong></div>
      <div><span>平台访问白名单</span><strong>未配置</strong></div>
      <div><span>已观察到的业务用户</span><strong>{overview?.users.length??'—'}</strong></div>
    </div>
    <section className="institution-tasks-list institution-access-table" aria-label="已观察到的接口调用用户">
      <h2>调用记录中的业务用户</h2>
      <p>从最近 7 天已保存的任务中统计，最多显示 100 个不同用户。只记录客户端主动声明的信息；不等于认证结果。</p>
      <div className="institution-tasks-table-wrap"><table>
        <thead><tr><th>来源标签</th><th>X-Workbench-User-Id</th><th>任务数量</th><th>成功数量</th><th>最近提交时间</th></tr></thead>
        <tbody>{overview?.users.map(row=><tr key={row.clientId+':'+row.userHash}>
          <td>{row.clientId}</td>
          <td><strong>{row.externalUserId??'历史记录未保存原值'}</strong>
            {!row.externalUserId&&<small>摘要 {row.userHash}…，不可逆向恢复</small>}
          </td>
          <td>{row.taskCount}</td><td>{row.successCount}</td><td>{formatTime(row.lastSeen)}</td>
        </tr>)}</tbody>
      </table></div>
      {overview?.users.length===0&&<p className="institution-tasks-empty">最近 7 天暂无 API 任务调用记录。</p>}
      {!overview&&!error&&<p className="institution-tasks-empty">正在加载接口接入情况…</p>}
    </section>
  </section>;
}

/**
 * 主页面：根据登录/访问权限渲染不同状态。
 *
 * mode 是一个联合类型（loading | profile | pair | ready | denied | error），
 * 可以类比 Java 中的页面状态枚举；但在 TS 中无需定义单独的 enum 类。
 */
export function InstitutionTaskManagementPage(){
  const [mode,setMode]=useState<'loading'|'profile'|'pair'|'ready'|'denied'|'error'>('loading');
  // 登录态/管理员权限只决定前端如何展示；后端每个接口仍重新校验权限。
  const [error,setError]=useState('');
  const [pairCode,setPairCode]=useState('');
  const [pairBusy,setPairBusy]=useState(false);

  // 任务列表：筛选条件、分页、当前结果。
  const [filter,setFilter]=useState<InstitutionTaskStatus|'ALL'>('ALL');
  const [query,setQuery]=useState('');
  const [appliedQuery,setAppliedQuery]=useState('');
  const [offset,setOffset]=useState(0);
  const [pageSize,setPageSize]=useState<20|50|100>(20);
  const [jump,setJump]=useState('1');
  const [page,setPage]=useState<InstitutionTaskPage|null>(null);
  const [listError,setListError]=useState('');
  const [loading,setLoading]=useState(false);

  // 单个任务详情，与任务列表分开请求，避免把敏感的完整结论发到列表接口。
  const [selectedId,setSelectedId]=useState('');
  const [detail,setDetail]=useState<InstitutionAdminTaskDetail|null>(null);
  const [detailError,setDetailError]=useState('');

  // 父页面的刷新计数器；值本身没有业务含义，只是触发 useEffect 的重新执行。
  const [refresh,setRefresh]=useState(0);

  // 1. 页面首次加载：复用工作台已有的登录会话，获取 CSRF Token。
  // 注意：bootstrap 成功只代表有登录态，不代表天然有管理员权限。
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

  // 2. 自动刷新：每 5 秒变化一次 refresh；浏览器标签页隐藏时暂不刷新。
  // 只查询已存在的任务，不会新建任务、重新投递 Prompt 或改变执行状态。
  useEffect(()=>{
    if(mode!=='ready')return;
    const timer=window.setInterval(()=>{
      if(!document.hidden)setRefresh(n=>n+1);
    },5000);
    return()=>window.clearInterval(timer);
  },[mode]);

  // 3. 任务列表：仅默认视图请求；切换状态/查询/页码或 refresh 时重查。
  useEffect(()=>{
    if(mode!=='ready'||accessView||usersView)return;
    let live=true;
    setLoading(true);
    api.institutionTasks(filter,appliedQuery,offset,undefined,pageSize).then(data=>{
      if(live){
        if(offset>0&&offset>=data.total){setOffset(Math.max(0,Math.ceil(data.total/pageSize)-1)*pageSize);return;}
        setPage(data);setListError('');setJump(String(Math.floor(offset/pageSize)+1));
      }
    }).catch(e=>{
      if(!live)return;
      if(e instanceof ApiError&&e.status===403){setMode('denied');return;}
      if(e instanceof ApiError&&e.status===401){setMode('profile');return;}
      setListError(errorText(e));
    }).finally(()=>{if(live)setLoading(false);});
    return()=>{live=false;};
  },[mode,filter,appliedQuery,offset,pageSize,refresh]);

  // 4. 单任务详情：通过 taskId 读最终 result.summary，成功后显示 Markdown。
  useEffect(()=>{
    if(mode!=='ready'||accessView||usersView||!selectedId)return;
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

  const totalPages=Math.max(1,Math.ceil((page?.total??0)/pageSize));
  function moveTo(index:number){
    if(!Number.isSafeInteger(index)||index<0||index>=totalPages||index*pageSize>1000000)return;
    setOffset(index*pageSize);setJump(String(index+1));
    setSelectedId('');setDetail(null);setDetailError('');
  }
  // 以下 JSX 是按 mode 分支返回的视图，类似 Java Controller 选择不同页面模板。
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

  // 通过 ?view=users 显示用户聚合；?view=access 显示接入记录；默认显示列表。
  return <main className="institution-tasks-page">
    <header className="institution-tasks-header">
      <div><small>SYSTEM ADMINISTRATION</small><h1>{accessView?'接口接入情况':'接口任务管理'}</h1>
        <p>{accessView?'查看机构 API 的开放策略与实际提交的业务用户标识（非授权名单）。':
          usersView?'按照业务用户标识聚合查看排查任务、各执行状态与最终结果。':
          '集中查看接口创建的排查任务、执行状态和最终返回结论。仅本机配置管理员可查看。'}</p></div>
      <nav aria-label="管理页面导航"><a href="/config">场景配置</a><a href="/analytics">访问统计</a><a href="/analytics?view=users">用户访问记录</a><a href="/">返回工作台</a></nav>
    </header>
    <div className="institution-tasks-tabs" role="navigation" aria-label="接口管理导航">
      <a href="/institution-tasks" aria-current={!accessView&&!usersView?'page':undefined}
        className={!accessView&&!usersView?'selected':''}>任务记录</a>
      <a href="/institution-tasks?view=users" aria-current={usersView?'page':undefined}
        className={usersView?'selected':''}>按业务用户聚合</a>
      <a href="/institution-tasks?view=access" aria-current={accessView?'page':undefined}
        className={accessView?'selected':''}>接口接入情况</a>
      <button type="button" onClick={()=>setRefresh(n=>n+1)}>刷新</button>
    </div>
    {accessView?<AccessOverviewView refresh={refresh} onForbidden={()=>setMode('denied')}/>:<>
      <InstitutionConcurrencyControl refresh={refresh} onForbidden={()=>setMode('denied')}/>
      {usersView?<InstitutionUserGroupsView refresh={refresh} onForbidden={()=>setMode('denied')}/>:<>
    <div className="institution-tasks-toolbar">
      <form onSubmit={e=>{e.preventDefault();setAppliedQuery(query.trim());setOffset(0);}}>
        <label htmlFor="institution-task-search">任务 ID / 请求号 / 业务用户标识</label>
        <input id="institution-task-search" type="search" value={query} onChange={e=>setQuery(e.target.value)}
          maxLength={128} placeholder="任务 ID、requestId 或 customer-001"/>
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
          <table><thead><tr><th>任务 / 请求号</th><th>业务用户</th><th>排查日期</th><th>执行状态</th><th>创建时间</th><th>操作</th></tr></thead>
            <tbody>{page?.items.map(task=><tr key={task.taskId} className={selectedId===task.taskId?'selected':''}>
              <td><strong title={task.taskId}>{task.taskId.slice(0,8)}…</strong>
                <small title={task.requestId}>{task.requestId}</small></td>
              <td title={task.externalUserId??'旧记录仅保存了摘要'}>{task.externalUserId??'旧记录'}</td>
              <td>{task.day}</td>
              <td><span className={'institution-task-status '+task.status.toLowerCase()}>
                {statusLabels[task.status]}</span>
                {task.errorCode&&<small className="institution-task-reason" title={describeTaskFailure(task.errorCode)}>
                  {task.errorCode}
                </small>}</td>
              <td>{formatTime(task.createdAt)}</td>
              <td><button type="button" className="institution-tasks-open" aria-label={'查看任务 '+task.taskId}
                onClick={()=>{setSelectedId(task.taskId);setDetail(null);setDetailError('');}}>详情</button></td>
            </tr>)}</tbody></table>
        </div>
        {!page&&!listError&&<p className="institution-tasks-empty">正在读取任务…</p>}
        {page?.items.length===0&&<p className="institution-tasks-empty">暂无符合条件的接口任务</p>}
        <div className="institution-tasks-pager" aria-label="接口任务分页">
          <label htmlFor="institution-page-size">每页
            <select id="institution-page-size" value={pageSize} onChange={e=>{
              setPageSize(Number(e.target.value) as 20|50|100);setOffset(0);setJump('1');
              setSelectedId('');setDetail(null);
            }}><option value={20}>20 条</option><option value={50}>50 条</option><option value={100}>100 条</option></select>
          </label>
          <button type="button" disabled={offset===0} onClick={()=>moveTo(Math.floor(offset/pageSize)-1)}>上一页</button>
          <span>第 {Math.floor(offset/pageSize)+1} / {totalPages} 页 · 共 {page?.total??0} 条</span>
          <button type="button" disabled={!page||offset+pageSize>=page.total}
            onClick={()=>moveTo(Math.floor(offset/pageSize)+1)}>下一页</button>
          <form onSubmit={e=>{e.preventDefault();moveTo(Number(jump)-1);}}>
            <input type="number" aria-label="跳转页码" min={1} max={totalPages} value={jump}
              onChange={e=>setJump(e.target.value)}/>
            <button type="submit" disabled={!jump||Number(jump)<1||Number(jump)>totalPages}>跳转</button>
          </form>
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
              <span>业务用户 ID</span><span>{detail.externalUserId??'历史记录仅保存摘要'}</span>
              <span>用户标识摘要</span><span>{detail.userHash}…</span>
              <span>排查日期</span><span>{detail.day}</span>
              <span>报文编号</span><span>{detail.msgSn??'已清理（任务结束后不保留）'}</span>
              <span>创建时间</span><span>{formatTime(detail.createdAt)}</span>
              <span>最后更新</span><span>{formatTime(detail.updatedAt)}</span>
            </div>
            {detail.errorCode&&<p className="institution-tasks-error" role="alert">
              异常代码：{detail.errorCode}。{describeTaskFailure(detail.errorCode)}
            </p>}
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
    </>}
    </>}
  </main>;
}
