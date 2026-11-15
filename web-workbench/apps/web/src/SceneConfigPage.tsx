import {useEffect,useState} from 'react';
import {api} from './api/client.ts';
import type {SceneConfigResponse,SceneConfigValue,SceneId,SceneBinding} from '../../../packages/contracts/src/chat.ts';

const rows:{id:SceneId;name:string;description:string}[]=[
  {id:'logs',name:'公管问题排查',description:'基于时间范围、Trace ID 或关键词排查业务问题'},
  {id:'message',name:'报文问题排查',description:'使用绑定目录中的原生执行入口分析报文链路'},
  {id:'data',name:'公管数据分析',description:'直接使用 OpenCode 对话分析当前业务数据'},
];
const empty=():SceneConfigValue=>({version:1,scenes:{}});
const entry=(value:SceneConfigValue,id:SceneId):SceneBinding=>value.scenes[id]??{directory:'',executor:{type:'skill',name:''}};
function message(error:unknown){return error instanceof Error?error.message:String(error);}

export function SceneConfigPage(){
  const [loaded,setLoaded]=useState<SceneConfigResponse|null>(null);
  const [value,setValue]=useState<SceneConfigValue>(empty);
  const [error,setError]=useState(''),[notice,setNotice]=useState(''),[busy,setBusy]=useState(false);
  useEffect(()=>{
    let active=true;
    api.sceneConfig().then(result=>{
      if(!active)return;
      setLoaded(result);setValue(result.config);
    }).catch(e=>{if(active)setError(message(e));});
    return ()=>{active=false;};
  },[]);
  const update=(id:SceneId,patch:Partial<SceneBinding>|{executor:SceneBinding['executor']})=>{
    setValue(current=>({version:1,scenes:{...current.scenes,[id]:{...entry(current,id),...patch}}}));
    setNotice('');
  };
  const save=async()=>{
    if(!loaded?.canEdit||busy)return;
    setError('');setNotice('');
    const next=empty();
    for(const row of rows){
      const config=entry(value,row.id);
      const directory=config.directory.trim(),name=config.executor.name.trim();
      if(!directory&&!name)continue;
      if(!directory||!name){setError(row.name+'：必须同时填写工作目录与执行入口名称。');return;}
      next.scenes[row.id]={directory,executor:{type:config.executor.type,name}};
    }
    setBusy(true);
    try{
      const result=await api.saveSceneConfig(next,loaded.revision);
      setLoaded({...loaded,...result});setValue(result.config);
      setNotice('配置已写入当前启动目录。新的对话会使用最新绑定；已有会话不会自动迁移目录。');
    }catch(e){setError(message(e));}
    finally{setBusy(false);}
  };
  return <section className="scene-page scene-config-page">
    <div className="scene-title"><div><h1>场景配置</h1><p>为每个子场景指定工作目录与 OpenCode 原生执行入口。</p></div></div>
    {!loaded?<p role={error?'alert':'status'}>{error||'正在读取工作台配置…'}</p>:<>
      <div className="scene-config-meta">
        <p>启动工作目录：<code>{loaded.workspace}</code></p>
        <p>场景配置：<code>{loaded.configPath}</code></p>
        <p>编辑权限：<code>{loaded.accessPath}</code></p>
      </div>
      {!loaded.canEdit&&<div className="notice" role="status">
        当前身份只有查看权限。请由目录管理员在上述权限文件中加入当前身份标识，然后重新打开页面。
        {loaded.identity&&<p>当前身份标识：<code>{loaded.identity}</code></p>}
        该权限文件不能通过本页面修改。
      </div>}
      <div className="scene-config-list">
        {rows.map(row=>{
          const binding=entry(value,row.id);
          return <section className="form-panel scene-config-row" key={row.id}>
            <h2>{row.name}</h2>
            <p className="muted">{row.description}</p>
            <div className="scene-config-fields">
              <label>工作目录
                <input aria-label={row.name+'工作目录'} value={binding.directory} readOnly={!loaded.canEdit}
                  placeholder="例如 D:\\projects\\business-service" onChange={e=>update(row.id,{directory:e.target.value})}/>
              </label>
              <label>执行类型
                <select aria-label={row.name+'执行类型'} value={binding.executor.type} disabled={!loaded.canEdit}
                  onChange={e=>update(row.id,{executor:{...binding.executor,type:e.target.value as SceneBinding['executor']['type']}})}>
                  <option value="skill">Skill</option><option value="subagent">Subagent</option><option value="command">OpenCode Command</option>
                </select>
              </label>
              <label>入口名称
                <input aria-label={row.name+'入口名称'} value={binding.executor.name} readOnly={!loaded.canEdit}
                  placeholder={binding.executor.type==='skill'?'例如 log-query':binding.executor.type==='subagent'?'例如 business-diagnoser':'例如 codea-review'}
                  onChange={e=>update(row.id,{executor:{...binding.executor,name:e.target.value}})}/>
              </label>
            </div>
          </section>;
        })}
      </div>
      {error&&<p className="notice" role="alert">{error}</p>}
      {notice&&<p role="status">{notice}</p>}
      {loaded.canEdit&&<div className="form-actions"><button className="primary" disabled={busy} onClick={()=>void save()}>
        {busy?'保存中…':'保存场景配置'}</button></div>}
      <p className="muted">执行入口必须已经由对应目录的 OpenCode 配置加载。绑定修改后，已有会话将保持原目录，不会静默切换。</p>
    </>}
  </section>;
}
