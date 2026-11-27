/**
 * 本地资料登录页
 *
 * 职责：收集姓名、手机号等本机资料并提交登录。
 * Java 研发阅读提示：本地资料不是强认证；服务端管理员 ACL 才决定可否访问敏感接口。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
import { useState, type FormEvent } from 'react';
import { ApiError } from './api/client.ts';
import './profile-login.css';

function loginError(error:unknown){
  if(error instanceof ApiError){
    switch(error.code){
      case 'PROFILE_CONFLICT': return '用户名资料冲突，请联系管理员核对。';
      case 'USERNAME_INVALID': case 'PROFILE_INVALID': return '用户名只能包含英文字母和数字，且至少包含一个英文字母（1 至 80 位）。';
      case 'ORIGIN_INVALID': case 'LAN_ORIGIN_FORBIDDEN': case 'LAN_AUTH_CONFIG_INVALID':
        return '局域网访问地址不符合服务器设置。请使用工作台启动时公布的内网 IP 和端口访问。';
      case 'PROFILE_LOGIN_REQUIRED':return '登录会话已过期，请重新填写资料进入工作台。';
      case 'PROFILE_WRITE_FAILED': return '无法保存资料。请检查工作目录的写入权限后重试。';
      case 'OFFLINE': return '无法连接本地工作台。请确认服务已启动，再重新登录。';
    }
  }
  return '登录失败，请重试。';
}

export function ProfileLogin({onLogin}:{onLogin:(name:string)=>Promise<void>}){
  const [name,setName]=useState('');
  const [busy,setBusy]=useState(false);
  const [error,setError]=useState('');
  async function submit(event:FormEvent<HTMLFormElement>){
    event.preventDefault();
    if(busy)return;
    setError('');
    if(!/^(?=.*[A-Za-z])[A-Za-z0-9]{1,80}$/.test(name)){
      setError('用户名只能包含英文字母和数字，且至少包含一个英文字母（1 至 80 位）。');return;
    }
    setBusy(true);
    try{await onLogin(name);}
    catch(e){setError(loginError(e));}
    finally{setBusy(false);}
  }
  return <main className="profile-page">
    <section className="profile-card" aria-labelledby="profile-title">
      <aside className="profile-side">
        <div className="profile-brand"><strong>智能工作台</strong><span>智能研发场景协作</span></div>
        <div className="profile-side-copy">
          <span className="profile-overline">LOCAL AI WORKSPACE</span>
          <h2>让研发工作<br/>更专注、更清晰</h2>
        </div>
      </aside>
      <div className="profile-form-pane">
        <span className="profile-overline">欢迎使用</span>
        <h1 id="profile-title">登录工作台</h1>
        <form onSubmit={submit} autoComplete="off">
          <label>用户名<input name="name" value={name} required maxLength={80} autoComplete="username" placeholder="英文或英文加数字，如 zhang、zhang01" onChange={e=>setName(e.target.value)}/></label>
          {error&&<p className="profile-error" role="alert">{error}</p>}
          <button className="profile-submit" disabled={busy} type="submit">{busy?'正在登录…':'登录工作台'}<span aria-hidden="true">↗</span></button>
        </form>
      </div>
    </section>
  </main>;
}
