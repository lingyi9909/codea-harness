import {useEffect,useState} from 'react';
import {api,ApiError} from './api/client.ts';
import {ProfileLogin} from './ProfileLogin.tsx';
import {SceneConfigPage} from './SceneConfigPage.tsx';
import './standalone-config.css';

/** A separate document; the workbench shell never mounts the configuration editor. */
export function StandaloneConfigPage(){
  useEffect(()=>{api.recordPageVisit('config');},[]);
  const [mode,setMode]=useState<'loading'|'profile'|'pair'|'ready'|'error'>('loading');
  const [error,setError]=useState(''),[code,setCode]=useState(''),[busy,setBusy]=useState(false);
  useEffect(()=>{
    let live=true;
    api.bootstrap().then(boot=>{if(live){api.setCsrf(boot.csrfToken);setMode('ready');}}).catch(e=>{
      if(!live)return;
      if(e instanceof ApiError&&e.status===401)setMode(e.code==='PROFILE_LOGIN_REQUIRED'?'profile':'pair');
      else{setError(e instanceof Error?e.message:String(e));setMode('error');}
    });
    return()=>{live=false;};
  },[]);
  if(mode==='profile')return <ProfileLogin onLogin={async(name,phone)=>{await api.login(name,phone);setMode('ready');}}/>;
  if(mode==='pair')return <main className="pair-page"><form className="pair-card" onSubmit={e=>{
    e.preventDefault();if(busy)return;setBusy(true);setError('');
    void api.pair(code).then(()=>setMode('ready')).catch(e=>setError(e instanceof Error?e.message:String(e))).finally(()=>setBusy(false));
  }}><h1>连接场景配置</h1><label>一次性配对码<input type="password" value={code} onChange={e=>setCode(e.target.value)} required autoComplete="off"/></label>
    {error&&<p role="alert">{error}</p>}<button disabled={busy||!code.trim()}>连接工作台</button></form></main>;
  if(mode!=='ready')return <main className="standalone-config"><p role={error?'alert':'status'}>{error||'正在读取配置访问权限…'}</p></main>;
  return <main className="standalone-config"><header className="standalone-config-header"><strong>智能工作台</strong><a href="/">返回工作台</a></header><SceneConfigPage/></main>;
}
