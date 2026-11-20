import {useEffect,useState} from 'react';
import {api} from './api/client.ts';
import type {SceneAccessConfig,AccessScene,AccessToolChoice,AccessToolAction,CustomToolRule} from '../../../packages/contracts/src/chat.ts';
import './scene-access-management.css';

const scenes:{id:AccessScene;name:string;kind:'native'|'page'}[]=[
  {id:'home',name:'对话首页',kind:'native'},
  {id:'review',name:'代码审查',kind:'page'},
  {id:'logs',name:'公管问题排查',kind:'native'},
  {id:'message',name:'报文问题排查',kind:'native'},
  {id:'institution-message',name:'机构报文排查',kind:'native'},
  {id:'data',name:'公管数据分析',kind:'native'},
  {id:'component-lab',name:'公共组件测试',kind:'page'},
  {id:'analytics',name:'页面访问统计',kind:'page'},
];
const groups:{label:string;help:string;items:{action:AccessToolAction;label:string}[]}[]=[
  {label:'命令与 Agent',help:'控制 Shell 命令、Code Mode、Skill 和 Subagent',
    items:[{action:'shell',label:'Shell / PowerShell / CMD'},{action:'execute',label:'Code Mode（execute）'},
      {action:'skill',label:'Skill 调用'},{action:'subagent',label:'Subagent 调用'}]},
  {label:'文件与目录',help:'原生文件工具权限，Shell 获准后仍可能绕过文件工具限制',
    items:[{action:'read',label:'读取文件'},{action:'write',label:'写入文件'},
      {action:'edit',label:'编辑文件'},{action:'glob',label:'文件匹配 Glob'},
      {action:'grep',label:'内容搜索 Grep'},{action:'list',label:'列举目录'},
      {action:'external_directory',label:'工作目录外访问'}]},
  {label:'联网工具',help:'对应 OpenCode 环境中已安装的工具权限，未安装的工具不会因允许而出现',
    items:[{action:'webfetch',label:'网页抓取 WebFetch'},{action:'websearch',label:'网页搜索 WebSearch'}]},
  {label:'DBHub MCP',help:'保留原生 MCP 工具名，实际 SQL 读写仍取决于 DBHub 和数据库账户权限',
    items:[{action:'dbhub_search_objects',label:'搜索 Schema 对象'},
      {action:'dbhub_execute_sql',label:'执行 DBHub SQL'}]},
];
const choices:{value:AccessToolChoice;label:string}[]=[
  {value:'inherit',label:'继承默认'},{value:'allow',label:'允许'},{value:'ask',label:'每次询问'},{value:'deny',label:'拒绝'},
];
type Saved={config:SceneAccessConfig;revision:string};
const identifier=/^[a-z][a-z0-9_.:-]{0,79}$/;

export function SceneAccessManagement(){
  const [saved,setSaved]=useState<Saved|null>(null);
  const [config,setConfig]=useState<SceneAccessConfig|null>(null);
  const [selected,setSelected]=useState<AccessScene>('message');
  const [error,setError]=useState(''),[message,setMessage]=useState(''),[busy,setBusy]=useState(false);
  useEffect(()=>{
    let active=true;
    api.accessSettings().then(value=>{if(active){setSaved(value);setConfig(value.config);}})
      .catch(e=>{if(active)setError(e instanceof Error?e.message:'权限配置读取失败');});
    return()=>{active=false;};
  },[]);
  const policy=config?.scenes[selected];
  const update=(part:Partial<SceneAccessConfig['scenes'][AccessScene]>)=>{
    setConfig(current=>current?{...current,scenes:{...current.scenes,[selected]:{...current.scenes[selected],...part}}}:current);
    setMessage('');
  };
  const setTool=(action:AccessToolAction,value:AccessToolChoice)=>{
    if(policy)update({tools:{...policy.tools,[action]:value}});
  };
  const custom=(index:number,part:Partial<CustomToolRule>)=>{
    if(!policy)return;
    update({custom:policy.custom.map((rule,i)=>i===index?{...rule,...part}:rule)});
  };
  const addRule=()=>{if(policy&&policy.custom.length<16)update({custom:[...policy.custom,{action:'',resource:'*',effect:'ask'}]});};
  const removeRule=(index:number)=>{if(policy)update({custom:policy.custom.filter((_,i)=>i!==index)});};
  async function save(){
    if(!saved||!config||busy)return;
    for(const scene of scenes){
      for(const rule of config.scenes[scene.id].custom){
        if(!identifier.test(rule.action)||!rule.resource||rule.resource.length>512||/[\x00-\x1f\x7f]/.test(rule.resource)){
          setError(scene.name+'：自定义工具名或资源表达式不合法。');setSelected(scene.id);return;
        }
      }
    }
    setBusy(true);setError('');setMessage('');
    try{
      const next=await api.saveAccessSettings(config,saved.revision);
      setSaved(next);setConfig(next.config);setMessage('权限配置已保存。规则在新会话中生效；权限已改变的旧会话需重新创建。');
    }catch(e){setError(e instanceof Error?e.message:'权限保存失败');}
    finally{setBusy(false);}
  }
  const choice=(value:AccessToolChoice,onChange:(v:AccessToolChoice)=>void,label:string)=>
    <select aria-label={label} value={value} onChange={e=>onChange(e.target.value as AccessToolChoice)}>
      {choices.map(item=><option key={item.value} value={item.value}>{item.label}</option>)}
    </select>;
  return <section className="scene-access-manage" aria-labelledby="access-manage-heading">
    <header className="scene-access-heading"><div><h2 id="access-manage-heading">各页面权限管理</h2>
      <p>选择页面后配置局域网访问和具体工具权限。除配置页以外，各页面默认允许局域网访问。</p></div>
      <span className="scene-access-hint">OpenCode 原生权限 · 按场景生效</span>
    </header>
    {config&&policy?<div className="scene-access-workspace">
      <nav className="access-scenes" aria-label="选择权限场景">
        {scenes.map(scene=><button type="button" key={scene.id}
          className={'access-scene '+(selected===scene.id?'active':'')}
          aria-current={selected===scene.id?'page':undefined} onClick={()=>{setSelected(scene.id);setError('');}}>
          <span>{scene.name}</span>
          <small>{scene.kind==='native'?'原生执行场景':'页面访问配置'}</small>
        </button>)}
      </nav>
      <div className="access-detail">
        <div className="access-detail-top">
          <h3>{scenes.find(s=>s.id===selected)?.name}</h3>
          <label className="access-lan"><input type="checkbox" checked={policy.lan}
            onChange={e=>update({lan:e.target.checked})}/> 允许局域网访问</label>
        </div>
        {scenes.find(s=>s.id===selected)?.kind==='page'
          ?<div className="access-explainer">{selected==='review'
            ?'代码审查使用单独的 Review 执行器权限，此处只管理其页面的局域网访问，不会假装控制 Review 的 Shell/文件权限。'
            :'这是纯页面场景，不调用 OpenCode 工具，因此仅需管理局域网访问。'}</div>
          :<>
            <div className="access-explainer">
              <strong>权限优先级：</strong>继承默认不修改 OpenCode 原有权限；允许、询问、拒绝会覆盖当前场景对应工具的通用规则。操作系统账户和 MCP 服务仍有最终权限。
            </div>
            {groups.map(group=><section className="access-group" key={group.label}>
              <header><h4>{group.label}</h4><p>{group.help}</p></header>
              <div className="access-tool-grid">
                {group.items.map(({action,label})=><label className="access-tool" key={action}>
                  <span><strong>{label}</strong><code>{action}</code></span>
                  {choice(policy.tools[action],value=>setTool(action,value),selected+' '+label+'权限')}
                </label>)}
              </div>
            </section>)}
            <section className="access-group">
              <header><h4>其他 MCP / 自定义工具规则</h4>
                <p>填写 OpenCode 实际返回的权限 action 名称，可按资源或具体命令限制；最后匹配的规则优先。</p></header>
              <div className="access-custom">
                {policy.custom.map((rule,i)=><div className="access-custom-row" key={i}>
                  <label>工具 action<input value={rule.action} maxLength={80}
                    placeholder="例如：teamai_search"
                    onChange={e=>custom(i,{action:e.target.value})}/></label>
                  <label>资源 resource<input value={rule.resource} maxLength={512}
                    placeholder="* 或具体匹配资源"
                    onChange={e=>custom(i,{resource:e.target.value})}/></label>
                  <label>规则<select value={rule.effect} onChange={e=>custom(i,{effect:e.target.value as CustomToolRule['effect']})}>
                    <option value="allow">允许</option><option value="ask">询问</option><option value="deny">拒绝</option>
                  </select></label>
                  <button className="access-rule-remove" type="button" onClick={()=>removeRule(i)}>移除</button>
                </div>)}
                <button type="button" className="access-add" disabled={policy.custom.length>=16} onClick={addRule}>＋ 添加工具规则</button>
              </div>
            </section>
            <p className="access-warning"><strong>安全提示：</strong>如果 Shell 设为允许，即使文件写入工具被拒绝，Shell 仍可能运行脚本写文件。工作目录不是系统沙箱；机构匿名页的高权限也可由启动开关启用，必须限制访问者。</p>
          </>}
      </div>
    </div>:<p role={error?'alert':'status'}>{error||'正在读取各页面权限配置…'}</p>}
    {error&&config&&<p className="notice" role="alert">{error}</p>}
    {message&&<p className="notice" role="status">{message}</p>}
    {config&&<div className="access-save"><button className="primary" type="button" disabled={busy} onClick={()=>void save()}>
      {busy?'正在保存…':'保存所有页面权限'}
    </button><span>更改不会影响其他场景的规则</span></div>}
  </section>;
}
