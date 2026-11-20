import {useState,type FormEvent} from 'react';
import {CalendarDate,localDayBounds} from './TimeFields.tsx';
import './institution-message-investigation.css';
import {DirectChat} from './DirectChat.tsx';
import type {ChatModel} from '../../../packages/contracts/src/chat.ts';

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

/** Institution investigation accepts only a calendar day and the original msgSn string. */
export function InstitutionMessageInvestigation({embedded=false,globalModel}:{embedded?:boolean;globalModel?:ChatModel}={}){
  const [submitted,setSubmitted]=useState(false);
  const [editing,setEditing]=useState(false);
  const [applied,setApplied]=useState<{day:string;msgSn:string}|null>(null);
  const [day,setDay]=useState(localToday);
  const [msgSn,setMsgSn]=useState('');
  const [notice,setNotice]=useState('');
  const bounds=institutionDayBounds(day);
  const ready=Boolean((!embedded||globalModel)&&bounds&&msgSn.trim());
  function prompt(question:string){
    if(submitted&&!editing)return question;
    if(!bounds||!msgSn.trim())throw Error('请填写有效日期和报文编号。');
    return ['机构报文排查：请使用本场景绑定的执行入口，按以下条件查询并分析报文。',
      '请将本次请求视为一个完整的排查任务：连续执行定位业务、调用工具核验、分析证据和输出结论。中间阶段不得作为最终答复，不要停在“阶段1”并让用户另发“继续”。',
      '只有确实缺少不可推断的必要输入且工具也无法补齐时才提出追问；查询遇到权限被拒、Skill 未加载或工具失败，须立即明确指出阻断原因和未完成步骤，不得声称已经完成排查。',
      '请遵守工具权限，必须以实际查询证据给出事实、异常原因、证据缺口及下一步；无法查询时明确说明，不要编造查询结果。',
      '查询日期：'+day,'开始时间：'+day+' 00:00:00','结束时间：'+day+' 23:59:59',
      '查询条件 JSON（from/to 为毫秒时间戳，范围包含 from、不包含 to；msgSn 为字符串）：',
      JSON.stringify({day,from:bounds.start,to:bounds.endExclusive,msgSn:msgSn.trim()}),
      question&&'排查问题：'+question].filter(Boolean).join('\n');
  }

  function submit(event:FormEvent<HTMLFormElement>){
    event.preventDefault();
    if(!ready)return;
    setNotice('请在机构报文独立页面开始排查。');
  }

  function onSent(){
    setSubmitted(true);
    setEditing(false);
    setApplied({day,msgSn:msgSn.trim()});
  }
  function beginEdit(){
    if(applied){setDay(applied.day);setMsgSn(applied.msgSn);}
    setEditing(true);
  }
  function cancelEdit(){
    if(applied){setDay(applied.day);setMsgSn(applied.msgSn);}
    setEditing(false);
  }
  const controls=editing?<div className="institution-adjust-controls">
    <CalendarDate label="调整日期" value={day} onChange={setDay} invalid={!bounds}/>
    <label htmlFor="institution-adjust-msgsn">报文编号（msgSn）
      <input id="institution-adjust-msgsn" type="text" aria-label="调整 msgSn" required
        maxLength={256} autoComplete="off" spellCheck={false} value={msgSn} onChange={e=>setMsgSn(e.target.value)}/>
    </label>
    <p>系统按所选日期的 00:00:00—23:59:59 重新查询，原结果保留在当前会话中。</p>
  </div>:null;

  return <section className="scene-page institution-message">
    <header className="institution-heading">
      <div className="institution-symbol" aria-hidden="true">▣</div>
      <div>
        <h1>机构报文排查</h1>
        <p>填写机构系统查询条件，核查指定日期的报文处理情况。</p>
      </div>
    </header>
    <div className={"institution-layout"+(submitted?" institution-layout-active":"")}>
      {!submitted&&<form className="institution-form-panel" onSubmit={e=>{if(embedded)e.preventDefault();else submit(e);}} autoComplete="off">
        <h2><span aria-hidden="true">☷</span> 查询参数</h2>
        <div className="institution-divider"/>
        <CalendarDate label="日期 *" value={day} invalid={!bounds} onChange={value=>{setDay(value);setNotice('');}}/>
        <p className="institution-day-summary" role="status">
          {bounds?'系统将自动查询该日 00:00:00 — 23:59:59 的报文数据。':'请选择有效的查询日期。'}
        </p>

        <label htmlFor="institution-msg-sn">报文编号（msgSn） <span className="institution-required">*</span></label>
        <input id="institution-msg-sn" aria-label="msgSn" type="text" value={msgSn} disabled={embedded&&submitted} required maxLength={256}
          spellCheck={false} autoComplete="off" placeholder="请输入报文编号"
          onChange={e=>{setMsgSn(e.target.value);setNotice('');}}/>

        {!embedded&&<button className="institution-submit" type="submit" disabled={!ready}>
          <span aria-hidden="true">▶</span> 开始排查
        </button>}
        {notice&&<p className="institution-notice" role="alert">{notice}</p>}
      </form>}
      <section className="institution-result-panel" aria-labelledby="institution-result-title">
        <header><h2 id="institution-result-title">排查结果</h2></header>
        {embedded?<DirectChat sceneId="institution-message" embedded globalModel={globalModel} investigation={{controls,initial:!submitted,ready,allowAdjustment:true,prompt,onSent,reset:()=>{},beginEdit,cancel:cancelEdit,title:'机构报文排查',sessionKey:'harness.institution-message'}}/>:<div className="institution-result-empty">
          <div className="institution-chat-icon" aria-hidden="true">•••</div>
          <h3>请填写查询参数并开始排查</h3>
          <p>填写日期与报文编号，排查结果将在此处展示。</p>
        </div>}
      </section>
    </div>
  </section>;
}
