/**
 * 页面访问统计管理页
 *
 * 职责：展示工作台各页面的访问次数和相关统计。
 * Java 研发阅读提示：访问统计是运营信息，不能替代可验证的身份认证和审计日志。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
import {useCallback,useEffect,useMemo,useState} from 'react';
import {api,ApiError} from './api/client.ts';
import {ProfileLogin} from './ProfileLogin.tsx';
import {UserAccessView} from './UserAccessView.tsx';
import './page-analytics.css';

type Stat=Awaited<ReturnType<typeof api.pageStats>>;
export function PageAnalyticsPage(){
  const [mode,setMode]=useState<'loading'|'profile'|'pair'|'ready'|'error'>('loading');
  const [error,setError]=useState(''),[pairCode,setPairCode]=useState(''),[busy,setBusy]=useState(false);
  const [days,setDays]=useState<7|30>(7);
  const [stats,setStats]=useState<Stat|null>(null),[statsError,setStatsError]=useState('');
  useEffect(()=>{
    api.recordPageVisit('analytics');
    let live=true;
    api.bootstrap().then(boot=>{if(live){api.setCsrf(boot.csrfToken);setMode('ready');}})
      .catch(e=>{
        if(!live)return;
        if(e instanceof ApiError&&e.status===401)setMode(e.code==='PROFILE_LOGIN_REQUIRED'?'profile':'pair');
        else{setError(e instanceof Error?e.message:String(e));setMode('error');}
      });
    return()=>{live=false;};
  },[]);
  const load=useCallback(async(range:7|30)=>{
    setStatsError('');
    try{setStats(await api.pageStats(range));}
    catch(e){setStatsError(e instanceof Error?e.message:'无法读取访问数据');}
  },[]);
  useEffect(()=>{if(mode==='ready')void load(days);},[mode,days,load]);
  const daily=useMemo(()=>{
    const index=new Map<string,number>();
    for(const row of stats?.daily??[])index.set(row.day,(index.get(row.day)??0)+row.views);
    return Array.from({length:days},(_,i)=>{
      const day=new Date(Date.now()-(days-i-1)*86400000).toISOString().slice(0,10);
      return {day,views:index.get(day)??0};
    });
  },[stats,days]);
  const max=Math.max(1,...daily.map(row=>row.views));
  const total=stats?.rows.reduce((n,row)=>n+row.views,0)??0;
  const today=stats?.daily.filter(row=>row.day===new Date().toISOString().slice(0,10))
    .reduce((n,row)=>n+row.views,0)??0;
  if(mode==='profile')return <ProfileLogin onLogin={async(name,phone)=>{
    await api.login(name,phone);setMode('ready');
  }}/>;
  if(mode==='pair')return <main className="pair-page"><form className="pair-card" onSubmit={e=>{
    e.preventDefault();if(busy)return;setBusy(true);setError('');
    void api.pair(pairCode).then(()=>setMode('ready')).catch(e=>setError(e instanceof Error?e.message:String(e))).finally(()=>setBusy(false));
  }}><h1>访问统计登录</h1><label>配对码<input type="password" value={pairCode} required onChange={e=>setPairCode(e.target.value)}/></label>
    {error&&<p role="alert">{error}</p>}<button disabled={busy||!pairCode.trim()}>进入统计</button></form></main>;
  if(mode!=='ready')return <main className="analytics-page"><p role={error?'alert':'status'}>{error||'正在验证访问权限…'}</p></main>;
  if(new URLSearchParams(window.location.search).get('view')==='users')return <UserAccessView/>;
  return <main className="analytics-page">
    <header className="analytics-header">
      <div><small>WORKBENCH INSIGHTS</small><h1>页面访问统计</h1>
        <p>按子页面查看访问趋势和使用情况，统计结果不包含报文正文、手机号与操作内容。</p>
      </div>
      <nav className="analytics-admin-links" aria-label="系统管理"><a href="/analytics?view=users">用户访问记录</a><a href="/config">场景配置</a><a href="/institution-tasks">接口任务管理</a><a href="/institution-tasks?view=access">接口接入情况</a><a href="/component-lab">组件测试</a><a href="/">返回工作台</a></nav>
    </header>
    <div className="analytics-toolbar"><div role="group" aria-label="统计日期范围">
      <button type="button" aria-pressed={days===7} className={days===7?'active':''} onClick={()=>setDays(7)}>最近 7 天</button>
      <button type="button" aria-pressed={days===30} className={days===30?'active':''} onClick={()=>setDays(30)}>最近 30 天</button>
    </div><button type="button" className="analytics-refresh" onClick={()=>void load(days)}>刷新数据</button></div>
    {statsError&&<p className="notice" role="alert">{statsError}</p>}
    <div className="analytics-summary">
      <section><span>总访问次数（PV）</span><strong>{total.toLocaleString()}</strong><small>选定周期内的所有页面访问</small></section>
      <section><span>今日访问次数</span><strong>{today.toLocaleString()}</strong><small>按 UTC 统计日期</small></section>
      <section><span>统计页面数</span><strong>{stats?.rows.length??'—'}</strong><small>含工作台、独立页面与配置页</small></section>
    </div>
    <section className="analytics-panel">
      <header><h2>每日访问趋势</h2><p>访问次数 PV，按 UTC 日期汇总</p></header>
      <div className="analytics-trend" aria-label="每日访问次数柱状图" role="img">
        {daily.map(row=><div className="analytics-bar-col" key={row.day} title={row.day+'：'+row.views+' 次'}>
          <span>{row.views||''}</span>
          <div className="analytics-bar-track"><div style={{height:(row.views/max*100)+'%'}} className="analytics-bar"/></div>
          <small>{days===7?row.day.slice(5):row.day.slice(8)}</small>
        </div>)}
      </div>
    </section>
    <section className="analytics-panel">
      <header><h2>各页面访问明细</h2><p>UV 按浏览器页面运行实例去重，刷新或重新打开页面可能被视为新访客。</p></header>
      <div className="analytics-table-wrap"><table><thead><tr><th>页面</th><th>访问次数 PV</th><th>访问实例 UV</th><th>最近访问</th></tr></thead>
      <tbody>{stats?.rows.map(row=><tr key={row.page}><td>{row.name}</td>
        <td>{row.views.toLocaleString()}</td><td>{row.visitors.toLocaleString()}</td>
        <td>{row.lastAt?new Date(row.lastAt).toLocaleString('zh-CN'):'—'}</td>
      </tr>)}</tbody></table></div>
      {!stats&&<p className="analytics-empty">正在加载页面访问统计…</p>}
    </section>
  </main>;
}
