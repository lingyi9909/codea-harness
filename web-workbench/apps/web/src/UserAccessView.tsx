/**
 * 用户访问记录（仅本机系统管理员）：按登录会话审计，而非按浏览器随机 PV/UV 推断身份。
 *
 * 姓名与手机号由访问者自填，因此这里只表示“登记身份”，不是企业实名认证。
 * 最后活动时间由服务端 API 访问刷新；不能凭没有退出记录就认定用户在线。
 */
import {useEffect,useState} from 'react';
import {api,type UserAccessPage} from './api/client.ts';
import './user-access.css';

const displayTime=(iso:string|null)=>iso?new Date(iso).toLocaleString('zh-CN',{hour12:false}):'—';
export function UserAccessView(){
  const [days,setDays]=useState<7|30>(7);
  const [query,setQuery]=useState(''),[applied,setApplied]=useState('');
  const [offset,setOffset]=useState(0),[refresh,setRefresh]=useState(0);
  const [page,setPage]=useState<UserAccessPage|null>(null);
  const [error,setError]=useState(''),[loading,setLoading]=useState(false);
  useEffect(()=>{
    let alive=true;
    setLoading(true);
    api.userVisits(days,offset,applied).then(data=>{
      if(!alive)return;
      // A filter or the 90-day retention window may invalidate the old page.
      if(offset>0&&data.total<=offset){setOffset(Math.max(0,Math.ceil(data.total/20)-1)*20);return;}
      setPage(data);setError('');
    }).catch(err=>{
      if(alive){setPage(null);setError(err instanceof Error?err.message:'读取用户访问记录失败');}
    }).finally(()=>{if(alive)setLoading(false);});
    return()=>{alive=false;};
  },[days,offset,applied,refresh]);
  const pages=Math.max(1,Math.ceil((page?.total??0)/(page?.limit??20)));
  const current=Math.floor(offset/(page?.limit??20))+1;
  return <main className="analytics-page user-access-page">
    <header className="analytics-header">
      <div><small>WORKBENCH ACCESS</small><h1>用户访问记录</h1>
        <p>查看登录过工作台的用户、访问来源和最近活动；仅本机管理员可查阅。</p>
      </div>
      <nav className="analytics-admin-links" aria-label="系统管理">
        <a href="/analytics">页面访问统计</a>
        <a href="/config">场景配置</a>
        <a href="/institution-tasks">接口任务管理</a>
        <a href="/">返回工作台</a>
      </nav>
    </header>
    <div className="user-access-toolbar">
      <div role="group" aria-label="查询日期范围">
        <button type="button" aria-pressed={days===7} onClick={()=>{setDays(7);setOffset(0);}}>最近 7 天</button>
        <button type="button" aria-pressed={days===30} onClick={()=>{setDays(30);setOffset(0);}}>最近 30 天</button>
      </div>
      <form onSubmit={e=>{e.preventDefault();setApplied(query.trim());setOffset(0);}}>
        <input aria-label="搜索用户访问记录" type="search" value={query} maxLength={128}
          placeholder="搜索姓名、脱敏手机号或来源 IP" onChange={e=>setQuery(e.target.value)}/>
        <button type="submit">查询</button>
        {(query||applied)&&<button type="button" onClick={()=>{setQuery('');setApplied('');setOffset(0);}}>清除</button>}
      </form>
      <button type="button" onClick={()=>setRefresh(n=>n+1)}>刷新</button>
    </div>
    {error&&<p role="alert" className="user-access-error">{error}</p>}
    <div className="user-access-summary">
      <span>登录记录 <strong>{page?.total??'—'}</strong> 条</span>
      <span>登记用户 <strong>{page?.distinctUsers??'—'}</strong> 位</span>
      <span>{loading?'正在刷新…':'登录历史保留 90 天'}</span>
    </div>
    <div className="user-access-table-wrap">
      <table>
        <thead><tr><th>姓名 / 登记手机</th><th>来源 IP</th><th>接入方式</th>
          <th>登录时间</th><th>最近活动</th><th>退出记录</th></tr></thead>
        <tbody>{page?.items.map(item=><tr key={item.id}>
          <td><strong>{item.name}</strong><small>{item.phoneMask}</small></td>
          <td>{item.ip}</td><td>{item.scope==='lan'?'局域网':'本机'}</td>
          <td>{displayTime(item.loginAt)}</td><td>{displayTime(item.lastActiveAt)}</td>
          <td>{item.logoutAt?displayTime(item.logoutAt):'未记录退出'}</td>
        </tr>)}</tbody>
      </table>
      {page?.items.length===0&&<p className="user-access-empty">该时间范围暂无登录记录。</p>}
      {!page&&!error&&<p className="user-access-empty">正在查询用户登录记录…</p>}
    </div>
    <div className="user-access-pager">
      <span>第 {current} / {pages} 页 · 每页 20 条</span>
      <button type="button" disabled={offset===0||loading} onClick={()=>setOffset(Math.max(0,offset-20))}>上一页</button>
      <button type="button" disabled={!page||offset+page.limit>=page.total||loading}
        onClick={()=>setOffset(offset+20)}>下一页</button>
    </div>
    <p className="user-access-note">
      说明：姓名和手机号来自用户自行填写，不构成实名身份认证。未记录退出可能是关闭浏览器或服务重启，
      不代表仍在线。访问记录从部署此版本后开始收集，不会回填以前的登录历史。
    </p>
  </main>;
}
