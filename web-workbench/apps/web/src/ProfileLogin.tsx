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
    <section className="profile-card" aria-labelledby="profile-title">
      <div className="profile-brand"><span aria-hidden="true">✧</span>智能工作台</div>
      <h1 id="profile-title">登录工作台</h1>
      <form onSubmit={submit} autoComplete="off">
        <label>姓名<input name="name" value={name} required autoComplete="off" onChange={e=>setName(e.target.value)}/></label>
        <label>手机号<input name="phone" type="tel" value={phone} required maxLength={40} autoComplete="off" onChange={e=>setPhone(e.target.value)}/></label>
        <p className="profile-disclosure">登录后，姓名和手机号将保存到当前工作目录的 <code>.workbench/user.json</code>，供此目录中的 Skill 读取。</p>
        {error&&<p className="profile-error" role="alert">{error}</p>}
        <button className="profile-submit" disabled={busy} type="submit">{busy?'正在登录…':'登录工作台'}</button>
      </form>
    </section>
  </main>;
}
