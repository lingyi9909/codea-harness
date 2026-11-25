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
      case 'LAN_LOGIN_RATE_LIMIT': return '登录尝试次数过多，请稍后重试。';
      case 'PHONE_REQUIRED': case 'PHONE_INVALID': case 'USERNAME_INVALID':
      case 'PROFILE_INVALID': return '请检查姓名和手机号。境外号码请带上国家或地区代码。';
      case 'PROFILE_IN_USE': return '另一个会话正在使用此工作目录。请先在该会话退出，再重新登录。';
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
    if(!/^(?:1[3-9][0-9]{9}|\+[1-9][0-9]{7,14})$/.test(phone.trim().replace(/[ ()-]/g,''))){
      setError('请输入有效的手机号。境外号码请带上国家或地区代码。');return;
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
