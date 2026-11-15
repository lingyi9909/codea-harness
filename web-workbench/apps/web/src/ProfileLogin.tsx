import { useState, type FormEvent } from 'react';
import { ApiError } from './api/client.ts';
import './profile-login.css';

function loginError(error:unknown){
  if(error instanceof ApiError){
    switch(error.code){
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
    <aside className="profile-aside" aria-label="工作台介绍">
      <div className="profile-brand"><strong>Code Harness</strong><span>智能研发工作台</span></div>
      <div className="profile-aside-content">
        <span className="profile-kicker">YOUR AI WORKSPACE</span>
        <h2>专注研发，<br/>让 AI 自然参与。</h2>
        <p>一个工作台，连接直接对话、代码审查、日志排查和数据分析。</p>
      </div>
      <p className="profile-aside-footer">本地工作台 · 在当前项目目录中使用</p>
    </aside>
    <section className="profile-main">
      <div className="profile-card" aria-labelledby="profile-title">
        <span className="profile-kicker">WORKBENCH ACCESS</span>
        <h1 id="profile-title">登录工作台</h1>
        <p className="profile-intro">填写使用者信息，继续进入当前项目。</p>
        <form onSubmit={submit} autoComplete="off">
          <label>姓名<input name="name" placeholder="请输入姓名" value={name} required autoComplete="off" onChange={e=>setName(e.target.value)}/></label>
          <label>手机号<input name="phone" type="tel" inputMode="tel" placeholder="请输入手机号" value={phone} required maxLength={40} autoComplete="off" onChange={e=>setPhone(e.target.value)}/></label>
          {error&&<p className="profile-error" role="alert">{error}</p>}
          <button className="profile-submit" disabled={busy} type="submit">{busy?'正在登录…':'登录工作台'}<span aria-hidden="true">→</span></button>
          <p className="profile-disclosure">姓名和手机号将保存到当前工作目录的 <code>.workbench/user.json</code>，供该目录的 Skill 读取。此登录不发送短信验证码。</p>
        </form>
      </div>
      <p className="profile-main-footer">Code Harness · 本地研发工作台</p>
    </section>
  </main>;
}
