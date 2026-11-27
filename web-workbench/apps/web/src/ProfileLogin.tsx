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
      case 'PROFILE_CONFLICT': return '该手机号登记的姓名不一致，请联系管理员核对。';
      case 'PHONE_REQUIRED': case 'PHONE_INVALID': case 'USERNAME_INVALID':
      case 'PROFILE_INVALID': return '请填写姓名和手机号（仅作使用记录，不核验身份）。';
      case 'ORIGIN_INVALID': case 'LAN_ORIGIN_FORBIDDEN': case 'LAN_AUTH_CONFIG_INVALID':
        return '局域网访问地址不符合服务器设置。请使用工作台启动时公布的内网 IP 和端口访问。';
      case 'PROFILE_LOGIN_REQUIRED':return '登录会话已过期，请重新填写资料进入工作台。';
      case 'PROFILE_WRITE_FAILED': return '无法保存资料。请检查工作目录的写入权限后重试。';
      case 'OFFLINE': return '无法连接本地工作台。请确认服务已启动，再重新登录。';
    }
  }
  return '登录失败，请重试。';
}

export function ProfileLogin({onLogin}:{onLogin:(name:string,phone:string)=>Promise<void>}){
  const [name,setName]=useState('');
  const [phone,setPhone]=useState('');
  const [busy,setBusy]=useState(false);
  const [error,setError]=useState('');
  async function submit(event:FormEvent<HTMLFormElement>){
    event.preventDefault();
    if(busy)return;
    setError('');
    if(!name.trim()||Array.from(name.trim()).length>80){
      setError('请输入 1 至 80 个字符的姓名。');return;
    }
    // Visitor registration only; no SMS check or phone-number format matching.
    if(!phone.trim()||phone.trim().length>40){
      setError('请填写手机号（仅作访问记录，不进行身份验证）。');return;
    }
    setBusy(true);
    try{await onLogin(name.trim(),phone.trim());}
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
          <label>姓名<input name="name" value={name} required autoComplete="off" placeholder="请输入姓名" onChange={e=>setName(e.target.value)}/></label>
          <label>手机号<input name="phone" type="tel" value={phone} required maxLength={40} autoComplete="off" placeholder="请输入手机号" onChange={e=>setPhone(e.target.value)}/></label>
          {error&&<p className="profile-error" role="alert">{error}</p>}
          <button className="profile-submit" disabled={busy} type="submit">{busy?'正在登录…':'登录工作台'}<span aria-hidden="true">↗</span></button>
        </form>
      </div>
    </section>
  </main>;
}
