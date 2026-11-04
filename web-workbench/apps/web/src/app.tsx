import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { apiClient, ApiError, type TaskDetail, type TaskSummary, type ReviewPreview, type Interaction } from './api/client.ts';
import { subscribeTask } from './api/events.ts';

const short = (value: string | null | undefined) => value ? value.slice(0, 9) : '—';
const date = (value?: string) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '—';
const TERMINAL = new Set(['SUCCEEDED', 'FAILED', 'STOPPED']);
const BUSY = new Set(['QUEUED','PREPARING','RUNNING','WAITING_INPUT','WAITING_PERMISSION','COLLECTING','STOP_REQUESTED','RECOVERING']);
const stateText: Record<string, string> = {
  QUEUED:'排队中', PREPARING:'准备独立代码副本', RUNNING:'执行审查中', WAITING_INPUT:'等待原生输入',
  WAITING_PERMISSION:'等待工具授权', COLLECTING:'归档审查结果', STOP_REQUESTED:'正在停止',
  RECOVERING:'恢复核对中', SUCCEEDED:'审查完成', FAILED:'执行失败', STOPPED:'已停止',
};
const msg = (error: unknown) => error instanceof ApiError
  ? error.message + '（' + error.code + '）' : error instanceof Error ? error.message : '操作未完成';
const noopLink = (href: string): string => {
  if (href.startsWith('#')) return href;
  try {
    const url = new URL(href);
    return ['https:', 'http:'].includes(url.protocol) ? url.href : '';
  } catch { return ''; }
};

function Notice({children, kind='info'}: {children: React.ReactNode;kind?: 'info'|'error'|'warn'}) {
  return <div className={'notice notice-'+kind} role={kind==='error'?'alert':'status'}>{children}</div>;
}
function HeaderBadge({status}:{status:string}) {
  return <span className={'status status-'+(TERMINAL.has(status)?status.toLowerCase():'working')}>{stateText[status]??status}</span>;
}
function Pairing({onPaired}:{onPaired:()=>void}) {
  const [pairingCode,setPairingCode]=useState('');
  const pair = useMutation({mutationFn:()=>apiClient.pair(pairingCode),onSuccess:onPaired});
  return <div className="pair-page"><div className="pair-box">
    <div className="mark">◈</div><h1>连接 Code Harness</h1>
    <p>请在本机服务终端读取一次性配对码，完成本机会话认证。</p>
    <form onSubmit={e=>{e.preventDefault();pair.mutate()}}>
      <label htmlFor="pair-code">本机配对码</label>
      <input id="pair-code" type="password" value={pairingCode} onChange={e=>setPairingCode(e.target.value)}
        autoComplete="one-time-code" placeholder="输入终端显示的配对码" required />
      {pair.error&&<Notice kind="error">{msg(pair.error)}</Notice>}
      <button className="primary full" type="submit" disabled={pair.isPending||!pairingCode.trim()}>
        {pair.isPending?'正在验证':'连接本地工作台'}</button>
    </form><small>仅支持 localhost · 不向外网发送认证信息</small>
  </div></div>;
}
function AddRepository({onClose}:{onClose:()=>void}) {
  const q=useQueryClient();
  const [name,setName]=useState('');
  const [path,setPath]=useState('');
  const action=useMutation({mutationFn:()=>apiClient.addRepository(name.trim(),path.trim()),
    onSuccess:()=>{void q.invalidateQueries({queryKey:['repositories']});onClose();}});
  return <div className="drawer-backdrop" onMouseDown={event=>{if(event.target===event.currentTarget)onClose()}}>
    <div className="modal-panel" role="dialog" aria-modal="true" aria-label="添加本地仓库">
      <button className="close" aria-label="关闭" onClick={onClose}>×</button>
      <h2>添加本地 Git 仓库</h2><p className="muted">只登记允许目录内的仓库，不执行 Git 写操作。</p>
      <form onSubmit={e=>{e.preventDefault();action.mutate()}}>
        <label htmlFor="repo-name">仓库名称</label><input id="repo-name" maxLength={100} value={name}
          onChange={e=>setName(e.target.value)} required placeholder="例如：订单服务"/>
        <label htmlFor="repo-path">本机绝对路径</label><input id="repo-path" maxLength={4096}
          value={path} onChange={e=>setPath(e.target.value)} required placeholder="D:\\workspace\\order-service"/>
        {action.error&&<Notice kind="error">{msg(action.error)}</Notice>}
        <div className="actions end"><button type="button" className="ghost" onClick={onClose}>取消</button>
          <button type="submit" className="primary" disabled={action.isPending}>登记仓库</button></div>
      </form>
    </div>
  </div>;
}
function Compose({onOpenTask,enabled,disabledReason}:{onOpenTask:(id:string)=>void;enabled:boolean;disabledReason:string}) {
  const qc=useQueryClient();
  const repos=useQuery({queryKey:['repositories'],queryFn:apiClient.repositories});
  const [repositoryId,setRepositoryId]=useState('');
  const [baseRefId,setBaseRefId]=useState('');
  const [controller,setController]=useState('');
  const [notes,setNotes]=useState('');
  const [preview,setPreview]=useState<ReviewPreview|null>(null);
  const [repoModal,setRepoModal]=useState(false);
  const selected=repos.data?.items.find(x=>x.id===repositoryId);
  const refs=useQuery({queryKey:['refs',repositoryId],queryFn:()=>apiClient.refs(repositoryId),enabled:!!repositoryId});
  const previewAction=useMutation({mutationFn:()=>apiClient.preview(repositoryId,baseRefId),onSuccess:setPreview});
  const start=useMutation({mutationFn:()=>apiClient.createTask(preview!.id,controller.trim(),notes.trim()),
    onSuccess:result=>{void qc.invalidateQueries({queryKey:['tasks']});if(result.taskId)onOpenTask(result.taskId);}});
  const selectedBase=refs.data?.items.find(x=>x.id===baseRefId);
  const validController=!controller||(/^[A-Za-z_][A-Za-z0-9_]*$/.test(controller)&&controller.length<=128);
  const expired=preview?Date.now()>=new Date(preview.expiresAt).getTime():false;
  const changed=preview&&(!selectedBase||!repositoryId);
  return <div className="compose">
    <div className="eyebrow"><span className="eyebrow-dot"/> SCENARIO · CODE REVIEW</div>
    <h1>从一个明确的代码范围开始</h1>
    <p className="lead">基于本地 Git 已提交变更，复用公司 OpenCode 原生 Agent 审查。<br/>先固定代码快照，再执行、追问和复查。</p>
    <section className="form-card">
      <div className="form-head"><span className="form-icon">⌘</span><div><h2>创建代码 Review</h2><p>选择仓库与本地基线分支，审查范围在创建时固定。</p></div></div>
      <div className="field-grid">
        <div className="field"><label htmlFor="repository">本地代码仓库</label>
          <div className="select-stack"><select id="repository" value={repositoryId} onChange={e=>{setRepositoryId(e.target.value);setBaseRefId('');setPreview(null)}}>
            <option value="">请选择已登记的仓库</option>
            {repos.data?.items.map(repo=><option key={repo.id} value={repo.id}>{repo.name} · {repo.status}</option>)}</select>
          <button className="minor-action" type="button" onClick={()=>setRepoModal(true)}>＋ 添加仓库</button></div>
          {selected&&<small className="path-hint" title={selected.path}>{selected.path}</small>}
        </div>
        <div className="field"><label htmlFor="base-ref">比较基线分支</label>
          <select id="base-ref" value={baseRefId} disabled={!repositoryId||!refs.data} onChange={e=>{setBaseRefId(e.target.value);setPreview(null)}}>
            <option value="">选择本地已存在的 ref</option>
            {refs.data?.items.map(ref=><option key={ref.id} value={ref.id}>{ref.fullName} · {short(ref.oid)}</option>)}
          </select><small>仅使用本地 ref，不自动 fetch/pull</small>
        </div>
      </div>
      {refs.data&&<div className="ref-note"><span>当前分支 <b>{refs.data.branch??'(detached HEAD)'}</b></span><span>HEAD <code>{short(refs.data.headOID)}</code></span><span>对象格式 {refs.data.objectFormat}</span></div>}
      <div className="field"><label htmlFor="controller">关注 Controller <span className="optional">可选</span></label>
        <input id="controller" maxLength={128} value={controller} onChange={e=>setController(e.target.value)}
          placeholder="例如 OrderController（仅支持 Java 类简名）"/>
        {!validController&&<small className="error-text">只能输入英文、数字和下划线，首字符不能是数字。</small>}
      </div>
      <div className="field"><label htmlFor="notes">补充审查要求 <span className="optional">可选 · 不改变审查范围</span></label>
        <textarea id="notes" rows={4} maxLength={2000} value={notes} onChange={e=>setNotes(e.target.value)}
          placeholder="例如：重点检查订单幂等性、事务边界和 SQL 数据一致性……"/>
        <span className="counter">{notes.length}/2000</span>
      </div>
      <div className="range-notice"><span className="lock">▣</span><div><strong>只分析 Git 已提交的变更</strong>
        <p>忽略 staged、unstaged 和 untracked 业务文件；工作台在独立代码副本中执行，不更改原仓库。</p></div></div>
      {preview&&<div className="preview-block">
        <div className="subheading">固定范围预览</div>
        <div className="preview-stats"><span><b>{preview.changedFiles}</b> 个已提交变更文件</span><span><b>{preview.applicableFiles}</b> 个可审文件</span><span>HEAD <code>{short(preview.scope.headOID)}</code></span></div>
        <div className="preview-stats secondary"><span>merge-base <code>{short(preview.scope.mergeBaseOID)}</code></span>
          <span>未纳入：暂存 {preview.dirtyCounts.staged} · 未暂存 {preview.dirtyCounts.unstaged} · 新文件 {preview.dirtyCounts.untracked}</span></div>
        <p>{preview.notice}</p>{expired&&<Notice kind="warn">预览已过期，请重新获取范围。</Notice>}
        {changed&&<Notice kind="warn">仓库或基线发生切换，请重新预览。</Notice>}</div>}
      {repos.error&&<Notice kind="error">{msg(repos.error)}</Notice>}
      {refs.error&&<Notice kind="error">{msg(refs.error)}</Notice>}
      {previewAction.error&&<Notice kind="error">{msg(previewAction.error)}</Notice>}
      {start.error&&<Notice kind="error">{msg(start.error)}</Notice>}
      {!enabled&&<Notice kind="warn">{disabledReason||'OpenCode 原生能力尚未完成验证，暂不能启动真实 Review。'}</Notice>}
      <div className="actions between">
        <button className="secondary-button" disabled={!repositoryId||!baseRefId||previewAction.isPending}
          onClick={()=>previewAction.mutate()}>{previewAction.isPending?'获取范围中…':'预览已提交范围'}</button>
        <button className="primary" disabled={!enabled||!preview||expired||!!changed||!validController||start.isPending}
          onClick={()=>start.mutate()}>{start.isPending?'正在创建…':'开始 Code Review'} <span>→</span></button>
      </div>
    </section>
    <div className="below"><div><span>01</span><strong>代码范围固定</strong><p>HEAD 与 merge-base 的完整 OID 锁定，独立副本。</p></div>
      <div><span>02</span><strong>原生 Agent 执行</strong><p>Command、Skill、权限和模型均由 OpenCode 处理。</p></div>
      <div><span>03</span><strong>报告持续可追溯</strong><p>五章 Markdown 归档；同范围追问和复查。</p></div></div>
    {repoModal&&<AddRepository onClose={()=>setRepoModal(false)}/>}
  </div>;
}
function InteractionCard({interaction,onApplied}:{interaction:Interaction;onApplied:()=>void}) {
  const [values,setValues]=useState<Record<string,string|boolean|string[]>>({});
  const action=useMutation({mutationFn:(answer: {kind:'PERMISSION';decision:'ALLOW_ONCE'|'DENY'}|
      {kind:'FORM';cancel:boolean;values:Record<string,string|boolean|string[]>})=>
      apiClient.answer(interaction.id,interaction.revision,answer),onSuccess:onApplied});
  const pending=interaction.status==='PENDING'&&new Date(interaction.expiresAt).getTime()>Date.now();
  return <section className="interaction-card">
    <div className="interaction-heading"><span>⚑</span><strong>{interaction.kind==='PERMISSION'?'原生工具权限请求':'原生补充信息请求'}</strong>
      <span className="minor-status">{interaction.status}</span></div>
    {interaction.kind==='PERMISSION'?<>
      <p>工具：<b>{interaction.tool}</b> · 仅作用于当前 Session/Run 的本次请求。</p>
      <pre className="permission-command">{interaction.requestText}</pre>
      <div className="actions end"><button className="danger-outline" disabled={!pending||action.isPending||!interaction.allowedDecisions.includes('DENY')}
        onClick={()=>action.mutate({kind:'PERMISSION',decision:'DENY'})}>拒绝</button>
        <button className="primary" disabled={!pending||action.isPending||!interaction.allowedDecisions.includes('ALLOW_ONCE')}
          onClick={()=>action.mutate({kind:'PERMISSION',decision:'ALLOW_ONCE'})}>本次允许</button></div>
    </>:<><p>{interaction.title}</p>{interaction.fields.map(field=><div className="field" key={field.id}><label>{field.label}{field.required?' *':''}</label>
      {field.type==='BOOLEAN'?<input type="checkbox" checked={values[field.id]===true} onChange={e=>setValues(v=>({...v,[field.id]:e.target.checked}))}/>:
      field.type==='SINGLE_SELECT'?<select value={String(values[field.id]??'')} onChange={e=>setValues(v=>({...v,[field.id]:e.target.value}))}><option value="">请选择</option>{field.options.map(opt=><option key={opt}>{opt}</option>)}</select>:
      field.type==='MULTI_SELECT'?<select multiple value={Array.isArray(values[field.id])?values[field.id] as string[]:[]} onChange={e=>setValues(v=>({...v,[field.id]:Array.from(e.target.selectedOptions,o=>o.value)}))}>{field.options.map(opt=><option key={opt}>{opt}</option>)}</select>:
      <input maxLength={field.maxLength} value={String(values[field.id]??'')} onChange={e=>setValues(v=>({...v,[field.id]:e.target.value}))}/>}</div>)}
      <div className="actions end"><button className="ghost" disabled={!pending||action.isPending}
        onClick={()=>action.mutate({kind:'FORM',cancel:true,values:{}})}>取消请求</button>
        <button className="primary" disabled={!pending||action.isPending||interaction.fields.some(f=>f.required&&(values[f.id]===undefined||values[f.id]===''))}
          onClick={()=>action.mutate({kind:'FORM',cancel:false,values})}>提交信息</button></div></>}
    {action.error&&<Notice kind="error">{msg(action.error)}</Notice>}
    {!pending&&<Notice kind="warn">该请求已失效或正在处理，请刷新核对。</Notice>}
  </section>;
}
function FileDrawer({taskId,onClose}:{taskId:string;onClose:()=>void}) {
  const files=useQuery({queryKey:['files',taskId],queryFn:()=>apiClient.files(taskId)});
  const [fileId,setFileId]=useState('');
  const [page,setPage]=useState(1);
  const lines=useQuery({queryKey:['lines',taskId,fileId,page],queryFn:()=>apiClient.lines(taskId,fileId,page,Math.min(page+199,page+499)),enabled:!!fileId});
  return <div className="drawer-backdrop" onMouseDown={e=>{if(e.target===e.currentTarget)onClose()}}>
    <aside className="source-drawer" role="dialog" aria-modal="true" aria-label="固定版本代码证据">
      <div className="drawer-title"><div><h2>固定版本源码</h2><p>只读取本次任务已归档的快照，不读取原仓库最新代码。</p></div>
        <button className="close" aria-label="关闭" onClick={onClose}>×</button></div>
      <div className="source-layout"><div className="file-names">
        {files.data?.items.map(file=><button key={file.id} title={file.path} className={fileId===file.id?'selected':''}
          onClick={()=>{setFileId(file.id);setPage(1)}}>{file.path}</button>)}
        {files.error&&<Notice kind="error">{msg(files.error)}</Notice>}
      </div><div className="code-preview">
        {!fileId&&<p className="empty">选择文件查看固定版本的行号与内容。</p>}
        {lines.isLoading&&<p className="muted">加载代码证据…</p>}
        {lines.error&&<Notice kind="error">{msg(lines.error)}</Notice>}
        {lines.data&&<><div className="source-file-label">{lines.data.file.path} · {lines.data.file.side} · {short(lines.data.file.blobOID)}</div>
          <pre>{lines.data.lines.map((line,index)=><div key={index}><span className="line-no">{lines.data!.startLine+index}</span>{line}</div>)}</pre>
          {lines.data.hasMore&&<button className="ghost" onClick={()=>setPage(p=>p+200)}>读取后续 200 行</button>}</>}
      </div></div>
    </aside></div>;
}
function TaskWorkspace({taskId,onBack}:{taskId:string;onBack:()=>void}) {
  const qc=useQueryClient();
  const task=useQuery({queryKey:['task',taskId],queryFn:()=>apiClient.task(taskId),refetchInterval:5000});
  const messages=useQuery({queryKey:['messages',taskId],queryFn:()=>apiClient.messages(taskId),refetchInterval:8000});
  const interactions=useQuery({queryKey:['interactions',taskId],queryFn:()=>apiClient.interactions(taskId),refetchInterval:4000});
  const runs=useQuery({queryKey:['runs',taskId],queryFn:()=>apiClient.runs(taskId),refetchInterval:7000});
  const [selectedReport,setSelectedReport]=useState<string|null>(null);
  const [tab,setTab]=useState<'execution'|'report'>('execution');
  const [sourceOpen,setSourceOpen]=useState(false);
  const [quote,setQuote]=useState('');
  const [followup,setFollowup]=useState('');
  const [liveWarning,setLiveWarning]=useState(false);
  const detail=task.data;
  const reportId=selectedReport??detail?.reports.at(-1)?.id;
  const artifact=useQuery({queryKey:['artifact',reportId],queryFn:()=>apiClient.artifact(reportId!),enabled:!!reportId});
  const refresh=()=>{for(const key of ['task','messages','interactions','runs','tasks'])void qc.invalidateQueries({queryKey:[key]})};
  useEffect(()=>{
    if (!detail) return;
    return subscribeTask(taskId,detail.lastSeq,()=>{setLiveWarning(false);refresh()},()=>setLiveWarning(true));
  },[taskId,detail?.lastSeq]);
  const operation=useMutation({mutationFn:(action:'stop'|'retry'|'recheck'|'discuss')=>{
    if(!detail) throw new Error('TASK_NOT_FOUND');
    if(action==='stop')return apiClient.stop(detail.latestRun.id);
    if(action==='retry')return apiClient.retry(detail.latestRun.id);
    if(action==='recheck')return apiClient.recheck(taskId);
    return apiClient.discuss(taskId,followup.trim(),reportId,quote.trim()||undefined);
  },onSuccess:()=>{setFollowup('');setQuote('');refresh();}});
  const busy=detail?BUSY.has(detail.latestRun.status):true;
  const failed=detail&&['FAILED','STOPPED'].includes(detail.latestRun.status);
  const reportAllowed=detail&&detail.reports.length>0&&!busy;
  const lineReport=useMemo(()=>[...(detail?.reports??[])].sort((a,b)=>a.version-b.version),[detail?.reports]);
  return <div className="task-workspace">
    <div className="backline"><button className="back" onClick={onBack}>← 返回工作台</button><span>任务 / {short(taskId)}</span></div>
    {task.isLoading&&<p className="muted">正在读取任务记录…</p>}
    {task.error&&<Notice kind="error">{msg(task.error)}</Notice>}
    {detail&&<>
      <div className="task-heading"><div><div className="eyebrow">CODE REVIEW · 固定范围</div>
        <h1>{detail.task.title}</h1><p>{detail.task.createdAt&&date(detail.task.createdAt)} · {detail.task.scenarioId}</p></div>
        <HeaderBadge status={detail.latestRun.status}/></div>
      <div className="scope-strip"><span>HEAD <code>{short(detail.scope.headOID)}</code></span>
        <span>基线 <strong>{detail.scope.baseRef}</strong></span>
        <span>merge-base <code>{short(detail.scope.mergeBaseOID)}</code></span>
        <span>快照 {detail.snapshotStatus}</span></div>
      {detail.continuity!=='COMPLETE'&&<Notice kind="warn">事件历史连续性：{detail.continuity}。页面以服务端持久状态及消息重新对账，可能缺少部分实时事件。</Notice>}
      {liveWarning&&<Notice kind="warn">实时事件流断开，仍会定期读取持久化任务状态，不会自动重复启动模型。</Notice>}
      <div className="task-tabs"><button className={tab==='execution'?'active':''} onClick={()=>setTab('execution')}>执行过程</button>
        <button className={tab==='report'?'active':''} onClick={()=>setTab('report')}>报告与追问 {detail.reports.length?('· '+detail.reports.length):''}</button>
        <button onClick={()=>setSourceOpen(true)}>固定源码 ↗</button></div>
      {tab==='execution'?<>
        <section className="content-card">
          <div className="content-head"><div><h2>任务执行轨迹</h2><p>仅显示后端已记录的阶段和状态，不推测百分比。</p></div>
            {busy&&<button className="stop-button" disabled={operation.isPending||detail.latestRun.status==='STOP_REQUESTED'} onClick={()=>operation.mutate('stop')}>■ {detail.latestRun.status==='STOP_REQUESTED'?'正在停止':'停止任务'}</button>}
            {failed&&<button className="secondary-button" disabled={operation.isPending} onClick={()=>operation.mutate('retry')}>重试失败轮次</button>}
          </div>
          <div className="run-steps">
            <div className={'run-step '+(detail.snapshotStatus==='READY'?'done':'')}><div className="step-dot">1</div><div><strong>固定已提交代码范围</strong><p>快照：{detail.snapshotStatus} · OID：{short(detail.scope.headOID)}</p></div></div>
            {runs.data?.items.slice().reverse().map(run=><div className="run-step" key={run.id}>
              <div className="step-dot">{run.number+1}</div><div><strong>第 {run.number} 轮 · {run.kind}</strong>
                <p>状态：{stateText[run.status]??run.status} · 受理：{run.dispatchState} · 开始：{date(run.startedAt??undefined)}</p>
                {run.errorCode&&<p className="error-text">错误代码：{run.errorCode}</p>}</div></div>)}
          </div>
          {runs.error&&<Notice kind="error">{msg(runs.error)}</Notice>}
          {busy&&<Notice>当前轮次仍在执行或等待确认。网络中断不会自动发起第二次模型请求。</Notice>}
          {failed&&<Notice kind="error">该轮次未成功完成。请核对错误或重试；历史成功报告不会被覆盖。</Notice>}
        </section>
        {!!interactions.data?.items.length&&<section className="interaction-list">{interactions.data.items.map(item=>
          <InteractionCard key={item.id} interaction={item} onApplied={refresh}/>)}</section>}
        {interactions.error&&<Notice kind="error">{msg(interactions.error)}</Notice>}
      </>:<><section className="content-card">
        <div className="content-head"><div><h2>代码审查报告</h2><p>保留每一轮完成的真实 Markdown 产物。</p></div>
          <div className="actions">
            {reportAllowed&&<button className="ghost" disabled={operation.isPending} onClick={()=>operation.mutate('recheck')}>↻ 固定范围复查</button>}
            {reportId&&<a className="secondary-button" href={apiClient.artifactDownload(reportId)}>↓ 下载 .md</a>}
          </div></div>
        {!!lineReport.length&&<div className="report-versions">{lineReport.map(report=>
          <button key={report.id} className={report.id===reportId?'active':''} onClick={()=>setSelectedReport(report.id)}>版本 {report.version} · {date(report.createdAt)}</button>)}</div>}
        {artifact.data&&<div className="report-body" onMouseUp={()=>{
          const selection=window.getSelection()?.toString().trim();
          if(selection&&selection.length<=4000)setQuote(selection);
        }}><ReactMarkdown remarkPlugins={[remarkGfm]} urlTransform={noopLink} skipHtml>
          {artifact.data.markdown}</ReactMarkdown></div>}
        {!reportId&&<div className="report-empty"><span>▤</span><h3>尚未生成报告</h3><p>报告必须来自原生执行的本轮实际写入文件，不会展示示例结果。</p></div>}
        {artifact.error&&<Notice kind="error">{msg(artifact.error)}</Notice>}
      </section>
      <section className="content-card discussion-card"><div className="content-head"><div><h2>同一固定范围下追问</h2>
        <p>复用原生 Session，不改变 HEAD、merge-base 或业务代码。</p></div></div>
        {messages.data?.items.slice().reverse().map(message=><div key={message.id} className={'message message-'+message.role.toLowerCase()}>
          <strong>{message.role==='USER'?'你':'OpenCode'}</strong><span>{date(message.createdAt)}</span>
          <p>{message.text}</p></div>)}
        {messages.error&&<Notice kind="error">{msg(messages.error)}</Notice>}
        {quote&&<div className="quoted"><button aria-label="移除引用" onClick={()=>setQuote('')}>×</button><strong>引用报告片段</strong><p>{quote}</p></div>}
        <textarea rows={3} maxLength={8000} value={followup} onChange={e=>setFollowup(e.target.value)}
          placeholder={reportAllowed?'围绕当前审查报告继续提问…':'等待当前轮次结束并成功生成报告后可追问'} disabled={!reportAllowed}/>
        <div className="actions between"><span className="counter">{followup.length}/8000</span>
          <button className="primary" disabled={!reportAllowed||!followup.trim()||operation.isPending}
            onClick={()=>operation.mutate('discuss')}>发送追问 →</button></div>
      </section></>}
      {operation.error&&<Notice kind="error">{msg(operation.error)}</Notice>}
      {sourceOpen&&<FileDrawer taskId={taskId} onClose={()=>setSourceOpen(false)}/>}
    </>}
  </div>;
}
export function App() {
  const qc=useQueryClient();
  const boot=useQuery({queryKey:['bootstrap'],queryFn:apiClient.bootstrap,retry:false});
  const history=useQuery({queryKey:['tasks'],queryFn:apiClient.tasks,enabled:!!boot.data,refetchInterval:10000});
  const [selectedTask,setSelectedTask]=useState<string|null>(null);
  const [sidebarOpen,setSidebarOpen]=useState(false);
  const [readyNotice,setReadyNotice]=useState(false);
  const health=useQuery({queryKey:['health'],queryFn:apiClient.health,refetchInterval:30000,retry:false});
  if (boot.isLoading) return <div className="loading-page">◈ <span>正在连接本地 Code Harness…</span></div>;
  if (boot.error instanceof ApiError && boot.error.status===401)
    return <Pairing onPaired={()=>void qc.invalidateQueries({queryKey:['bootstrap']})}/>;
  if (boot.error)return <div className="loading-page error-view"><div><h2>本地工作台不可用</h2>
    <p>{msg(boot.error)}</p><button className="secondary-button" onClick={()=>void qc.invalidateQueries({queryKey:['bootstrap']})}>重新连接</button></div></div>;
  const enabled=boot.data?.scenarios.some(s=>s.id==='code-review'&&s.enabled)??false;
  const disabledReason=boot.data?.scenarios.find(s=>s.id==='code-review')?.reason??'原生能力尚未验证';
  const open=(id:string)=>{setSelectedTask(id);setSidebarOpen(false)};
  return <div className="layout">
    <aside className={'sidebar '+(sidebarOpen?'open':'')}>
      <div className="branding"><span className="brand-logo">◈</span><div><strong>Code Harness</strong><small>研发场景工作台</small></div></div>
      <div className="sidebar-group"><div className="nav-caption">WORKSPACE</div>
        <button className={'nav-item '+(!selectedTask?'active':'')} onClick={()=>{setSelectedTask(null);setSidebarOpen(false)}}><span>▦</span> 代码 Review <span className="nav-dot"/></button>
      </div>
      <div className="sidebar-group sidebar-history"><div className="nav-caption with-action">任务历史 <button title="刷新任务历史" onClick={()=>void qc.invalidateQueries({queryKey:['tasks']})}>↻</button></div>
        {history.data?.items.map((task:TaskSummary)=><button key={task.id} className={'history-item '+(selectedTask===task.id?'active':'')} onClick={()=>open(task.id)}>
          <span className="history-icon">⌁</span><span className="history-detail"><strong>{task.title}</strong>
            <small>{short(task.headOID)} · {stateText[task.status]??task.status}</small></span></button>)}
        {!history.data?.items.length&&<p className="sidebar-empty">暂无任务记录</p>}
      </div>
      <div className="sidebar-bottom"><div className="local-tag"><span className={'signal '+(health.data?.status==='READY'?'green':'amber')}/>
        <div><strong>本地执行环境</strong><small>{health.data?.status==='READY'?'已就绪':'待验证或不可用'}</small></div></div>
        <div className="local-meta">Node / OpenCode · localhost</div></div>
    </aside>
    <main className="main-area"><header className="topbar">
      <button className="mobile-menu" onClick={()=>setSidebarOpen(x=>!x)} aria-label="切换侧栏">☰</button>
      <span>Code Harness <b>/</b> {selectedTask?'任务详情':'代码 Review'}</span>
      <div className="top-actions"><span className="local-mark">● 本机工作台</span>
        <button title="显示环境状态" onClick={()=>setReadyNotice(x=>!x)} className="icon-action">ⓘ</button></div>
    </header>{readyNotice&&<div className="env-details"><strong>执行能力状态</strong>
      <p>服务：{health.data?.status??'未连接'} · OpenCode：{health.data?.opencode??'未验证'} · DB：{health.data?.database??'未连接'}</p>
      <p>只有实际验证支持的原生能力才会启用 Review，未知能力不会自动降级为模拟执行。</p></div>}
      <div className="main-content">{selectedTask?<TaskWorkspace taskId={selectedTask} onBack={()=>setSelectedTask(null)}/>:
        <Compose onOpenTask={open} enabled={enabled} disabledReason={disabledReason}/>}</div>
    </main></div>;
}
