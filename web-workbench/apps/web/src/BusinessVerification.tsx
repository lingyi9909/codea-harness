import {useEffect,useState,type FormEvent} from 'react';
import {api} from './api/client.ts';
// The code is ephemeral component state, never composer/history/Form input.
export function BusinessVerification({sessionId}:{sessionId:string}){
  const [status,setStatus]=useState<Awaited<ReturnType<typeof api.businessStatus>>|null>(null);
  const [code,setCode]=useState(''),[busy,setBusy]=useState(false),[error,setError]=useState('');
  useEffect(()=>{
    let live=true;
    const poll=()=>api.businessStatus(sessionId).then(s=>{if(live)setStatus(s);}).catch(()=>{});
    void poll();const interval=setInterval(()=>void poll(),2000);
    return()=>{live=false;clearInterval(interval);};
  },[sessionId]);
  if(!status||!['WAITING_CODE','VERIFYING','EXPIRED','FAILED','CANCELLED'].includes(status.state))return null;
  const submit=async(event:FormEvent)=>{
    event.preventDefault();if(busy||!status.challengeId)return;
    const value=code;setCode('');setBusy(true);setError('');
    try{const result=await api.submitBusinessCode(sessionId,status.challengeId,value);setStatus({...status,...result});if(result.verified===false)setError('验证码不正确，请重新输入。');}
    catch{setError('验证未通过或已过期，请重新发起业务验证。');}
    finally{setBusy(false);}
  };
  return <form className="notice" onSubmit={submit} autoComplete="off">
    <strong>业务系统验证（模拟日志平台）</strong>
    <p>已向手机号 {status.maskedPhone} 发起验证。</p>
    {status.state==='WAITING_CODE'?<label>验证码<input type="password" inputMode="numeric" autoComplete="off" maxLength={32} value={code} onChange={event=>setCode(event.target.value)} disabled={busy}/></label>:<p role="status">{status.state==='VERIFYING'?'正在验证…':status.state==='EXPIRED'?'验证码已过期':status.state==='CANCELLED'?'验证已取消':'验证失败'}</p>}
    {error&&<p role="alert">{error}</p>}
    {status.state==='WAITING_CODE'&&<button disabled={busy||!code} type="submit">验证</button>}
    {['WAITING_CODE','VERIFYING'].includes(status.state)&&<button type="button" disabled={busy} onClick={()=>{setCode('');setBusy(true);void api.cancelBusinessCode(sessionId,status.challengeId!).then(()=>setStatus({...status,state:'CANCELLED'})).catch(()=>setError('取消失败，请重试。')).finally(()=>setBusy(false));}}>取消</button>}
  </form>;
}
