import {useState,type FormEvent} from 'react';
import './institution-message-investigation.css';

/** Browser-local calendar date. Day boundaries use local midnights, including DST days. */
export function institutionDayBounds(day:string){
  if(!/^\d{4}-\d{2}-\d{2}$/.test(day))return null;
  const [year,month,date]=day.split('-').map(Number);
  if(year<100||year>9999)return null;
  const start=new Date(year,month-1,date,0,0,0,0);
  if(start.getFullYear()!==year||start.getMonth()+1!==month||start.getDate()!==date||
    start.getHours()!==0||start.getMinutes()!==0||start.getSeconds()!==0)return null;
  // Do not assume a day is 24h: DST transitions can change the elapsed duration.
  const next=new Date(year,month-1,date+1,0,0,0,0);
  if(next.getTime()<=start.getTime())return null;
  return {start:start.getTime(),endInclusive:next.getTime()-1000,endExclusive:next.getTime()};
}
function localToday(){
  const now=new Date();
  return [now.getFullYear(),String(now.getMonth()+1).padStart(2,'0'),String(now.getDate()).padStart(2,'0')].join('-');
}

/**
 * Isolated page-only scene. The existing OpenCode prompt route persists message content,
 * so credentials must never be passed through it. A secure business credential bridge
 * is required before enabling the real investigation action.
 */
export function InstitutionMessageInvestigation(){
  const [day,setDay]=useState(localToday);
  const [msgSn,setMsgSn]=useState('');
  const [account,setAccount]=useState('');
  const [password,setPassword]=useState('');
  const [verificationCode,setVerificationCode]=useState('');
  const [passwordVisible,setPasswordVisible]=useState(false);
  const [notice,setNotice]=useState('');
  const bounds=institutionDayBounds(day);
  const ready=Boolean(bounds&&msgSn.trim()&&account.trim()&&password&&verificationCode.trim());

  function submit(event:FormEvent<HTMLFormElement>){
    event.preventDefault();
    if(!ready)return;
    // Never persist or serialize credentials into a model prompt, browser storage,
    // network request, UI result, history, or telemetry before a secure API exists.
    setPassword('');
    setVerificationCode('');
    setPasswordVisible(false);
    setNotice('当前已完成页面与参数校验。机构系统的安全认证及查询接口尚未接入，本次未发送账号、密码或验证码，也未启动排查。');
  }

  return <section className="scene-page institution-message">
    <header className="institution-heading">
      <div className="institution-symbol" aria-hidden="true">▣</div>
      <div>
        <h1>机构报文排查</h1>
        <p>填写机构系统查询条件，核查指定日期的报文处理情况。</p>
      </div>
    </header>
    <div className="institution-layout">
      <form className="institution-form-panel" onSubmit={submit} autoComplete="off">
        <h2><span aria-hidden="true">☷</span> 查询参数</h2>
        <div className="institution-divider"/>
        <label htmlFor="institution-date">日期 <span className="institution-required">*</span></label>
        <input id="institution-date" aria-label="日期" type="date" value={day} required
          aria-invalid={!bounds} onChange={e=>{setDay(e.target.value);setNotice('');}}/>
        <p className="institution-day-summary" role="status">
          {bounds?'系统将自动查询该日 00:00:00 — 23:59:59 的报文数据。':'请选择有效的查询日期。'}
        </p>

        <label htmlFor="institution-msg-sn">报文编号（msgSn） <span className="institution-required">*</span></label>
        <input id="institution-msg-sn" aria-label="msgSn" type="text" value={msgSn} required maxLength={256}
          spellCheck={false} autoComplete="off" placeholder="请输入报文编号"
          onChange={e=>{setMsgSn(e.target.value);setNotice('');}}/>

        <label htmlFor="institution-account">账号 <span className="institution-required">*</span></label>
        <input id="institution-account" aria-label="账号" type="text" value={account} required maxLength={128}
          spellCheck={false} autoComplete="off" placeholder="请输入账号"
          onChange={e=>{setAccount(e.target.value);setNotice('');}}/>

        <label htmlFor="institution-password">密码 <span className="institution-required">*</span></label>
        <div className="institution-secret">
          <input id="institution-password" aria-label="密码" type={passwordVisible?'text':'password'} value={password}
            required maxLength={256} autoComplete="off" placeholder="请输入密码"
            onChange={e=>{setPassword(e.target.value);setNotice('');}}/>
          <button type="button" aria-label={passwordVisible?'隐藏密码':'显示密码'}
            onClick={()=>setPasswordVisible(value=>!value)}>{passwordVisible?'隐藏':'显示'}</button>
        </div>

        <label htmlFor="institution-code">验证码 <span className="institution-required">*</span></label>
        <input id="institution-code" aria-label="验证码" type="password" value={verificationCode}
          required maxLength={64} autoComplete="off" placeholder="请输入验证码"
          onChange={e=>{setVerificationCode(e.target.value);setNotice('');}}/>

        <button className="institution-submit" type="submit" disabled={!ready}>
          <span aria-hidden="true">▶</span> 开始排查
        </button>
        {notice&&<p className="institution-notice" role="alert">{notice}</p>}
      </form>
      <section className="institution-result-panel" aria-labelledby="institution-result-title">
        <header><h2 id="institution-result-title">排查结果</h2></header>
        <div className="institution-result-empty">
          <div className="institution-chat-icon" aria-hidden="true">•••</div>
          <h3>请填写查询参数并开始排查</h3>
          <p>排查结果将在此处展示。账号、密码及验证码不会写入聊天记录。</p>
        </div>
      </section>
    </div>
  </section>;
}
