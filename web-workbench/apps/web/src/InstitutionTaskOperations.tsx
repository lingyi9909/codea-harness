/**
 * 机构任务并发与用户聚合
 *
 * 职责：提供 3/5/10/15 档配置和按 X-Workbench-User-Id 的任务分组。
 * Java 研发阅读提示：当前档位由后端持久化；业务用户标识仅用于展示分组，不是访问凭证。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
/**
 * 「接口任务管理」的两个独立 UI 模块。
 *
 * 对 Java 开发者的理解方式：
 * - useState<T>：组件自己的页面状态（不是后端实体，也不持久化数据库）
 * - useEffect：首次展示/依赖变化时执行数据拉取（类似页面生命周期回调）
 * - JSX：用 TS 表达的视图模板，不是 HTML 字符串拼接
 * - api：浏览器对 C 后端的 HTTP 调用封装，类似前端版 Feign Client
 */
import {useEffect,useState} from 'react';
import Markdown from 'react-markdown';
import {api,ApiError,type InstitutionAdminTask,type InstitutionAdminTaskDetail,
  type InstitutionConcurrency,type InstitutionConcurrencyStatus,
  type InstitutionTaskPage,type InstitutionUserGroup,type InstitutionUserGroupPage} from './api/client.ts';
import './institution-task-operations.css';

// as const：把数字数组的元素约束为固定档位，避免用户输入任意数字。
const choices=[3,5,10,15] as const;
const labels:Record<InstitutionAdminTask['status'],string>={
  QUEUED:'排队中',RUNNING:'执行中',SUCCEEDED:'已完成',FAILED:'失败',UNKNOWN:'未知',
};
const displayTime=(s:string)=>new Date(s).toLocaleString('zh-CN',{
  timeZone:'Asia/Shanghai',year:'numeric',month:'2-digit',day:'2-digit',
  hour:'2-digit',minute:'2-digit',second:'2-digit',hour12:false,
});
// unknown 不能直接访问 .message；必须先用 instanceof 做运行时类型判断。
const errorText=(e:unknown)=>e instanceof Error?e.message:'请求失败';

/**
 * 并发设置控件（React 函数组件）。
 * props 里的 refresh 是父组件传入的刷新版本号；onForbidden 用于权限过期时退出。
 *
 * 页面里分三种状态：
 * status   = 服务端实际生效的档位和任务数
 * selected = 下拉框当前选择的档位（还没保存）
 * saving   = 正在发送 PATCH，防止重复提交
 */
export function InstitutionConcurrencyControl({refresh,onForbidden}:{
  refresh:number;onForbidden:()=>void;
}){
  const [status,setStatus]=useState<InstitutionConcurrencyStatus|null>(null);
  const [selected,setSelected]=useState<InstitutionConcurrency>(3);
  const [saving,setSaving]=useState(false);
  const [editing,setEditing]=useState(false);
  const [error,setError]=useState('');
  const [notice,setNotice]=useState('');
  // 只读查询：父组件每隔 5 秒提高 refresh，触发一次新请求。
  // live 标记用于忽略组件卸载之后才返回的异步响应。
  useEffect(()=>{
    let live=true;
    api.institutionConcurrency().then(result=>{
      if(live){setStatus(result);if(!editing&&!saving)setSelected(result.maxConcurrency);setError('');}
    }).catch(e=>{
      if(!live)return;
      if(e instanceof ApiError&&(e.status===403||e.status===401)){onForbidden();return;}
      setError(errorText(e));
    });
    return()=>{live=false;};
  // 注意：刷新操作不修改并发数，只有用户点击保存才发送 PATCH。
  // 此 effect 只按父组件的 refresh 刷新，不因每次传入新的回调引用而重新请求。
  // eslint-disable-next-line react-hooks/exhaustive-deps
  },[refresh]);
  /**
   * 管理员点击保存 -> PATCH /api/v1/institution-concurrency。
   * async/await 类似 Java CompletableFuture，但不需要显式创建线程。
   * 真正的权限及 CSRF 校验始终在后端进行。
   */
  const save=async()=>{
    setSaving(true);setNotice('');setError('');
    try{
      const result=await api.saveInstitutionConcurrency(selected);
      setStatus(result);setSelected(result.maxConcurrency);setEditing(false);
      setNotice('并发设置已保存，新的排队任务将按此上限执行。');
    }catch(e){
      if(e instanceof ApiError&&(e.status===403||e.status===401)){onForbidden();return;}
      setError(errorText(e));
    }finally{setSaving(false);}
  };
  return <section className="institution-operations-capacity" aria-label="任务并发配置">
    <div className="institution-operations-capacity-title">
      <div><strong>任务并发配置</strong><p>最多同时执行的机构报文排查任务数。设置保存在 C 服务器，可随时调整。</p></div>
      <div className="institution-operations-load">
        <span>运行中 <strong>{status?.running??'—'}</strong></span>
        <span>排队中 <strong>{status?.queued??'—'}</strong></span>
      </div>
    </div>
    <div className="institution-operations-capacity-picker">
      <label htmlFor="institution-max-concurrency">最大并发任务数</label>
      <select id="institution-max-concurrency" value={selected} disabled={!status||saving}
        onChange={e=>{setSelected(Number(e.target.value) as InstitutionConcurrency);setEditing(true);}}>
        {choices.map(n=><option key={n} value={n}>{n} 个任务</option>)}
      </select>
      <button type="button" disabled={!status||saving||selected===status.maxConcurrency}
        onClick={()=>void save()}>{saving?'保存中…':'保存配置'}</button>
      <small>当前生效：{status?.maxConcurrency??'—'} 个</small>
    </div>
    {error&&<p role="alert" className="institution-tasks-error">{error}</p>}
    {notice&&<p role="status" className="institution-operations-notice">{notice}</p>}
    <p className="institution-operations-hint">调低上限不会强行停止已运行任务，只限制后续新任务启动。15 并发需确认内网模型、DBHub 和数据库能够承受相应压力。</p>
  </section>;
}

/**
 * 「按业务用户聚合」页面：三层钻取关系
 * 1. groups：用户汇总（customer-001 -> 成功/失败/排队等数量）
 * 2. tasks：当前用户的任务列表（通过 userHash 过滤，避免字符串模糊匹配）
 * 3. taskDetail：当前选中任务的最终结论
 *
 * 分组使用后端生成的哈希标识；原始 X-Workbench-User-Id 仅用来展示和搜索，
 * 不能当成平台登录身份或授权凭据。
 */
export function InstitutionUserGroupsView({refresh,onForbidden}:{
  refresh:number;onForbidden:()=>void;
}){
  // 搜索输入与实际生效的查询条件分离：只有点击搜索才更新 applied。
  const [q,setQ]=useState('');
  const [applied,setApplied]=useState('');

  // 第一层：用户分组与分组分页（服务端 SQL 分页）。
  const [offset,setOffset]=useState(0);
  const [groupSize,setGroupSize]=useState<20|50|100>(20);
  const [groups,setGroups]=useState<InstitutionUserGroupPage|null>(null);
  const [selected,setSelected]=useState<InstitutionUserGroup|null>(null);

  // 第二层：点击用户后，查看这个用户的任务列表。
  const [tasksOffset,setTasksOffset]=useState(0);
  const [taskSize,setTaskSize]=useState<20|50|100>(20);
  const [tasks,setTasks]=useState<InstitutionTaskPage|null>(null);

  // 第三层：点击单个任务后，显示后端返回的最终排查结论。
  const [openedTask,setOpenedTask]=useState('');
  const [taskDetail,setTaskDetail]=useState<InstitutionAdminTaskDetail|null>(null);

  const [error,setError]=useState('');
  const [taskError,setTaskError]=useState('');
  // 第一层数据请求：查询用户聚合（服务端执行 SQL GROUP BY）。
  // applied / offset / refresh 变化时才重新获取。
  useEffect(()=>{
    let live=true;
    api.institutionTaskGroups(applied,offset,groupSize).then(data=>{
      if(live){
        if(offset>0&&offset>=data.total){setOffset(Math.max(0,Math.ceil(data.total/groupSize)-1)*groupSize);return;}
        setGroups(data);setError('');
      }
    }).catch(e=>{
      if(!live)return;
      if(e instanceof ApiError&&(e.status===403||e.status===401)){onForbidden();return;}
      setError(errorText(e));
    });
    return()=>{live=false;};
  // eslint-disable-next-line react-hooks/exhaustive-deps
  },[applied,offset,groupSize,refresh]);
  // 第二层数据请求：按 userHash 查询某用户的具体 taskId 列表。
  // userHash 是精确分组键，原始业务用户 ID 只是显示字段。
  useEffect(()=>{
    if(!selected)return;
    let live=true;
    api.institutionTasks('ALL','',tasksOffset,selected.userHash,taskSize).then(data=>{
      if(live){
        if(tasksOffset>0&&tasksOffset>=data.total){setTasksOffset(Math.max(0,Math.ceil(data.total/taskSize)-1)*taskSize);return;}
        setTasks(data);setTaskError('');
      }
    }).catch(e=>{
      if(!live)return;
      if(e instanceof ApiError&&(e.status===403||e.status===401)){onForbidden();return;}
      setTaskError(errorText(e));
    });
    return()=>{live=false;};
  // eslint-disable-next-line react-hooks/exhaustive-deps
  },[selected?.userHash,tasksOffset,taskSize,refresh]);
  // 第三层数据请求：选中 taskId 后读取最终结论；刷新时也会更新运行状态。
  useEffect(()=>{
    if(!openedTask)return;
    let live=true;
    api.institutionTask(openedTask).then(data=>{
      if(live){setTaskDetail(data);setTaskError('');}
    }).catch(e=>{
      if(!live)return;
      if(e instanceof ApiError&&(e.status===403||e.status===401)){onForbidden();return;}
      setTaskError(errorText(e));
    });
    return()=>{live=false;};
  // eslint-disable-next-line react-hooks/exhaustive-deps
  },[openedTask,refresh]);
  // JSX 中 groups?.items 是可选链：groups=null 时不访问 .items，避免空指针。
  // map() 类似 Java Stream.map()，将数据集合映射为表格行。
  return <section className="institution-user-groups" aria-label="按业务用户聚合任务">
    <div className="institution-user-groups-heading">
      <div><h2>按业务用户聚合</h2><p>以请求头 X-Workbench-User-Id 所属的任务标识分组，不把不同用户的任务混合。聚合仅用于查看，不代表用户经过鉴权。</p></div>
      <form onSubmit={e=>{e.preventDefault();setApplied(q.trim());setOffset(0);}}>
        <input aria-label="搜索业务用户" value={q} maxLength={128} onChange={e=>setQ(e.target.value)}
          placeholder="例如 customer-001"/>
        <button type="submit">搜索</button>
        {applied&&<button type="button" className="institution-operations-secondary"
          onClick={()=>{setQ('');setApplied('');setOffset(0);}}>清除</button>}
      </form>
    </div>
    {error&&<p className="institution-tasks-error" role="alert">{error}</p>}
    <div className="institution-user-groups-layout">
      <div className="institution-tasks-list institution-user-groups-list">
        <div className="institution-tasks-table-wrap"><table>
          <thead><tr><th>业务用户 ID</th><th>任务总数</th><th>运行</th><th>排队</th><th>成功</th><th>失败</th><th>未知</th><th>操作</th></tr></thead>
          <tbody>{groups?.items.map(group=><tr key={group.clientId+':'+group.userHash}
            className={selected?.userHash===group.userHash?'selected':''}>
            <td><strong title={group.externalUserId??group.userHash}>{group.externalUserId??'历史用户'}</strong>
              <small>{group.clientId}</small></td>
            <td>{group.total}</td><td>{group.running}</td><td>{group.queued}</td>
            <td>{group.succeeded}</td><td>{group.failed}</td><td>{group.unknown}</td>
            <td><button type="button" className="institution-tasks-open" onClick={()=>{
              setSelected(group);setTasksOffset(0);setTasks(null);setOpenedTask('');setTaskDetail(null);
            }}>查看任务</button></td>
          </tr>)}</tbody>
        </table></div>
        {groups?.items.length===0&&<p className="institution-tasks-empty">没有符合条件的业务用户任务。</p>}
        {!groups&&!error&&<p className="institution-tasks-empty">正在获取业务用户任务统计…</p>}
        <div className="institution-tasks-pager">
          <label>每页 <select aria-label="业务用户每页条数" value={groupSize} onChange={e=>{
            setGroupSize(Number(e.target.value) as 20|50|100);setOffset(0);setSelected(null);setGroups(null);
          }}><option value={20}>20 条</option><option value={50}>50 条</option><option value={100}>100 条</option></select></label>
          <span>共 {groups?.total??'—'} 个用户</span>
          <button type="button" disabled={offset===0} onClick={()=>setOffset(Math.max(0,offset-groupSize))}>上一页</button>
          <span>第 {Math.floor(offset/groupSize)+1} / {Math.max(1,Math.ceil((groups?.total??0)/groupSize))} 页</span>
          <button type="button" disabled={!groups||offset+groupSize>=groups.total} onClick={()=>setOffset(offset+groupSize)}>下一页</button>
        </div>
      </div>
      <section className="institution-tasks-detail institution-user-group-details" aria-label="选中用户的任务明细">
        {!selected?<p className="institution-tasks-empty">点击左侧某个业务用户的「查看任务」，查看其排查任务及最终结论。</p>:<>
          <header><h2>业务用户：{selected.externalUserId??'历史用户（仅保存摘要）'}</h2>
            <span>{tasks?.total??selected.total} 条任务</span></header>
          {taskError&&<p className="institution-tasks-error" role="alert">{taskError}</p>}
          <div className="institution-tasks-table-wrap"><table>
            <thead><tr><th>请求号</th><th>日期</th><th>状态</th><th>创建时间</th><th>操作</th></tr></thead>
            <tbody>{tasks?.items.map(task=><tr key={task.taskId}>
              <td title={task.requestId}>{task.requestId}</td>
              <td>{task.day}</td>
              <td><span className={'institution-task-status '+task.status.toLowerCase()}>{labels[task.status]}</span></td>
              <td>{displayTime(task.createdAt)}</td>
              <td><button type="button" className="institution-tasks-open" onClick={()=>{
                setOpenedTask(task.taskId);setTaskDetail(null);
              }}>查看结果</button></td>
            </tr>)}</tbody>
          </table></div>
          <div className="institution-tasks-pager">
            <label>每页 <select aria-label="用户任务每页条数" value={taskSize} onChange={e=>{
              setTaskSize(Number(e.target.value) as 20|50|100);setTasksOffset(0);setOpenedTask('');setTaskDetail(null);
            }}><option value={20}>20 条</option><option value={50}>50 条</option><option value={100}>100 条</option></select></label>
            <button type="button" disabled={tasksOffset===0} onClick={()=>{setTasksOffset(Math.max(0,tasksOffset-taskSize));setOpenedTask('');setTaskDetail(null);}}>上一页</button>
            <span>第 {Math.floor(tasksOffset/taskSize)+1} / {Math.max(1,Math.ceil((tasks?.total??0)/taskSize))} 页</span>
            <button type="button" disabled={!tasks||tasksOffset+taskSize>=tasks.total} onClick={()=>{setTasksOffset(tasksOffset+taskSize);setOpenedTask('');setTaskDetail(null);}}>下一页</button>
          </div>
          {openedTask&&<section className="institution-user-group-result">
            <h3>任务结果 <code>{openedTask}</code></h3>
            {!taskDetail?<p>正在获取任务结果…</p>:<>
              <p>状态：<strong>{labels[taskDetail.status]}</strong></p>
              {taskDetail.errorCode&&<p className="institution-tasks-error">错误：{taskDetail.errorCode}</p>}
              {taskDetail.result?.summary?
                <div className="institution-tasks-markdown"><Markdown>{taskDetail.result.summary}</Markdown></div>:
                <p>{taskDetail.status==='QUEUED'||taskDetail.status==='RUNNING'?
                  '任务尚未完成，页面每 5 秒更新。':'该任务没有最终结论。'}</p>}
            </>}
          </section>}
        </>}
      </section>
    </div>
  </section>;
}
