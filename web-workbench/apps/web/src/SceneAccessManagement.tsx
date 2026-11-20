import {useEffect,useState} from 'react';
import {api} from './api/client.ts';
import type {SceneAccessConfig,AccessScene,AccessToolChoice} from '../../../packages/contracts/src/chat.ts';
import './scene-access-management.css';
const names:Record<AccessScene,string>={
  home:'对话首页',review:'代码审查',logs:'公管问题排查',message:'报文问题排查',
  'institution-message':'机构报文排查',data:'公管数据分析','component-lab':'组件测试',
};
const ids=Object.keys(names) as AccessScene[];
type Saved={config:SceneAccessConfig;revision:string};
const opts:[AccessToolChoice,string][]=[['inherit','沿用原生配置'],['allow','允许'],['deny','拒绝']];
export function SceneAccessManagement(){
  const [saved,setSaved]=useState<Saved|null>(null),[config,setConfig]=useState<SceneAccessConfig|null>(null);
  const [stats,setStats]=useState<Awaited<ReturnType<typeof api.pageStats>>|null>(null);
  const [error,setError]=useState(''),[message,setMessage]=useState(''),[busy,setBusy]=useState(false);
  useEffect(()=>{
    let active=true;
    Promise.all([api.accessSettings(),api.pageStats()]).then(([value,metrics])=>{
      if(!active)return;setSaved(value);setConfig(value.config);setStats(metrics);
    }).catch(e=>{if(active)setError(e instanceof Error?e.message:'统计与权限数据读取失败');});
    return()=>{active=false;};
  },[]);
  const patch=(id:AccessScene,part:Partial<SceneAccessConfig['scenes'][AccessScene]>)=>{
    setConfig(old=>old?{...old,scenes:{...old.scenes,[id]:{...old.scenes[id],...part}}}:old);
    setMessage('');
  };
  async function save(){
    if(!saved||!config||busy)return;
    setBusy(true);setError('');setMessage('');
    try{
      const next=await api.saveAccessSettings(config,saved.revision);
      setSaved(next);setConfig(next.config);setMessage('权限规则已保存。旧 OpenCode 会话如与新权限不一致，需要创建新会话。');
    }catch(e){setError(e instanceof Error?e.message:'保存失败');}
    finally{setBusy(false);}
  }
  const select=(value:AccessToolChoice,change:(v:AccessToolChoice)=>void)=><select value={value} onChange={e=>change(e.target.value as AccessToolChoice)}>
    {opts.map(([code,title])=><option key={code} value={code}>{title}</option>)}
  </select>;
  return <section className="scene-access-manage" aria-labelledby="access-manage-heading">
    <header><h2 id="access-manage-heading">页面访问及工具权限</h2>
      <p>统一控制各场景的局域网访问，以及创建新 OpenCode Session 时使用的工具策略。配置页始终仅限本机访问。</p></header>
    {config?<><div className="scene-access-list">
      {ids.map(id=><div className="scene-access-item" key={id}>
        <div className="scene-access-item-title"><strong>{names[id]}</strong>
          <label><input type="checkbox" checked={config.scenes[id].lan} onChange={e=>patch(id,{lan:e.target.checked})}/> 允许局域网</label>
        </div>
        <div className="scene-access-controls">
          <label>Shell {select(config.scenes[id].shell,value=>patch(id,{shell:value}))}</label>
          <label>文件写入 {select(config.scenes[id].fileWrite,value=>patch(id,{fileWrite:value}))}</label>
          <label>DBHub MCP <select value={config.scenes[id].dbhub} onChange={e=>patch(id,{dbhub:e.target.value as 'inherit'|'deny'})}>
            <option value="inherit">沿用原生配置</option><option value="deny">拒绝 DBHub</option>
          </select></label>
        </div>
      </div>)}
    </div>
    <p className="muted">说明：文件写入选项仅控制原生编辑工具；已开放的 Shell 本身仍可能写入文件。DBHub 设置仅覆盖已接入的 DBHub 工具，其他 MCP 权限继续遵循 OpenCode 配置。机构匿名页的基础安全权限与环境开关仍独立生效。</p>
    <button className="primary" disabled={busy} onClick={()=>void save()}>{busy?'保存中…':'保存页面权限'}</button>
    </>:<p>{error||'正在读取权限…'}</p>}
    {error&&config&&<p role="alert" className="notice">{error}</p>}
    {message&&<p role="status" className="notice">{message}</p>}
    <div className="scene-access-statistics">
      <header><h2>页面访问统计</h2><p>最近 7 天的页面访问次数（PV）和去重访客数（UV），不记录报文编号或消息正文。</p></header>
      {stats?<div className="statistics-table-wrap"><table><thead><tr>
        <th>页面</th><th>访问次数 PV</th><th>独立访客 UV</th><th>最近访问</th>
      </tr></thead><tbody>{stats.rows.map(row=><tr key={row.page}>
        <td>{row.name}</td><td>{row.views}</td><td>{row.visitors}</td>
        <td>{row.lastAt?new Date(row.lastAt).toLocaleString():'—'}</td>
      </tr>)}</tbody></table></div>:<p>{error||'正在读取访问统计…'}</p>}
    </div>
  </section>;
}
