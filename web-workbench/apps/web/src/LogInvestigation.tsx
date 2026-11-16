import {useState} from 'react';
import {DirectChat} from './DirectChat.tsx';
import type {ChatModel} from '../../../packages/contracts/src/chat.ts';

import './log-time-range.css';

const presets=[{minutes:15,label:'最近 15 分钟'},{minutes:60,label:'最近 1 小时'},{minutes:1440,label:'最近 24 小时'}];
const localZone=Intl.DateTimeFormat().resolvedOptions().timeZone;
type Bounds={from:number;to:number};
type TimeRange={start:string;end:string;from:number;to:number;preset:number|null};
function localTime(instant:number){
  const date=new Date(instant);
  return new Date(instant-date.getTimezoneOffset()*60000).toISOString().slice(0,19);
}
function offsetAt(instant:number){
  const minutes=-new Date(instant).getTimezoneOffset(),absolute=Math.abs(minutes);
  return (minutes<0?'-':'+')+String(Math.floor(absolute/60)).padStart(2,'0')+':'+String(absolute%60).padStart(2,'0');
}
function formatBounds({from,to}:Bounds){
  return localTime(from)+offsetAt(from)+' 至 '+localTime(to)+offsetAt(to);
}
// The UI, saved ranges, and prompts share the same whole-second precision.
function seconds(instant:number){return Math.floor(instant/1000)*1000;}
function rangeFromBounds({from,to}:Bounds,preset:number|null=null):TimeRange{
  const start=seconds(from),end=seconds(to);
  return {start:localTime(start),end:localTime(end),from:start,to:end,preset};
}
function recentRange(minutes:number){
  const to=seconds(Date.now());
  return rangeFromBounds({from:to-minutes*60000,to},minutes);
}
function validBounds(from:unknown,to:unknown):Bounds|null{
  if(typeof from!=='number'||typeof to!=='number'||!Number.isFinite(from)||!Number.isFinite(to))return null;
  const start=seconds(from),end=seconds(to);
  return Number.isFinite(new Date(start).getTime())&&Number.isFinite(new Date(end).getTime())&&start<end
    ?{from:start,to:end}:null;
}
function readAppliedRange(session:string,rangePrompt?:string,storageKey='harness.logs'):Bounds|null{
  if(!session)return null;
  // Existing workbench prompts contain either offset-qualified bounds or UTC instants.
  const line=rangePrompt?.match(/^时间范围（起点含、终点不含）：(\S+) 至 (\S+)$/m);
  if(line&&line.slice(1).every(stamp=>/(?:Z|[+-]\d{2}:\d{2})$/.test(stamp))){
    const bounds=validBounds(Date.parse(line[1]),Date.parse(line[2]));
    if(bounds)return bounds;
  }
  const key=storageKey+'.range.'+session;
  // Previous log sessions used an unscoped range entry. Migrate them only for the public scene.
  for(const candidate of storageKey==='harness.logs'?[key,'harness.logs.range']:[key]){
    try{
      const saved=JSON.parse(sessionStorage.getItem(candidate)??'null');
      if(candidate===key||saved?.session===session){
        const bounds=validBounds(saved?.from,saved?.to);
        if(bounds){
          if(candidate!==key)sessionStorage.setItem(key,JSON.stringify(bounds));
          return bounds;
        }
      }
    }catch{/* An older browser entry may be incomplete. */}
  }
  return null;
}
function parsedFilters(prompt?:string){
  return {
    trace:prompt?.match(/^Trace ID：([^\r\n]*)$/m)?.[1]?.trim()??'',
    keyword:prompt?.match(/^关键字：([^\r\n]*)$/m)?.[1]?.trim()??'',
    msgSn:prompt?.match(/^msgSn：([^\r\n]*)$/m)?.[1]?.trim()??'',
  };
}
function readAppliedFilters(session:string,prompt?:string,storageKey='harness.logs'){
  const parsed=parsedFilters(prompt);
  if((parsed.trace||parsed.keyword||parsed.msgSn)||!session)return parsed;
  try{
    const saved:unknown=JSON.parse(sessionStorage.getItem(storageKey+'.filters.'+session)??'null');
    if(saved&&typeof saved==='object'&&!Array.isArray(saved)){
      const value=saved as Record<string,unknown>;
      // Older saved sessions have only trace/keyword; they remain readable.
      const msgSn=value.msgSn===undefined?'':value.msgSn;
      if(typeof value.trace==='string'&&typeof value.keyword==='string'&&typeof msgSn==='string'&&
        value.trace.length<=256&&value.keyword.length<=512&&msgSn.length<=256)
        return {trace:value.trace,keyword:value.keyword,msgSn};
    }
  }catch{/* A saved filter draft may be incomplete; never crash restoration. */}
  return parsed;
}
function elapsed(ms:number){
  const hours=Math.floor(ms/3600000),minutes=Math.floor(ms%3600000/60000),remainder=Math.floor(ms%60000/1000);
  return [hours&&hours+' 小时',minutes&&minutes+' 分钟',remainder&&remainder+' 秒'].filter(Boolean).join(' ');
}
function parseLocal(value:string){
  // Native datetime-local may omit :00; any fractional input is rejected
  // rather than silently flowing into the downstream log query.
  const normalized=value.length===16?value+':00':value;
  if(!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}$/.test(normalized))return NaN;
  const ms=Date.parse(normalized);
  if(!Number.isFinite(ms))return NaN;
  // Reject invalid dates and wall times skipped by a daylight-saving transition.
  return localTime(ms)===normalized?ms:NaN;
}
export function LogInvestigation({mode='public',globalModel}:{mode?:'public'|'message';globalModel?:Pick<ChatModel,'id'|'providerID'|'variant'>}={}){
  const storageKey:'harness.logs'|'harness.message'=mode==='message'?'harness.message':'harness.logs';
  const title=mode==='message'?'报文问题排查':'公管问题排查';
  const [form,setForm]=useState(()=>!sessionStorage.getItem(storageKey));
  const [trace,setTrace]=useState(''),[msgSn,setMsgSn]=useState(''),[keyword,setKeyword]=useState('');
  const [applied,setApplied]=useState(()=>readAppliedRange(sessionStorage.getItem(storageKey)??'',undefined,storageKey));
  const [appliedFilters,setAppliedFilters]=useState({trace:'',keyword:'',msgSn:''});
  const [range,setRange]=useState(()=>applied?rangeFromBounds(applied):recentRange(60));
  const {from,to,start,end}=range;
  function editTime(side:'start'|'end',value:string){
    setRange(previous=>({...previous,[side]:value,[side==='start'?'from':'to']:parseLocal(value),preset:null}));
  }
  const timeError=!start||!end?'请填写完整的开始和结束时间。':
    !Number.isFinite(from)||!Number.isFinite(to)?'所选时间无效，请重新选择。':
    from>=to?'结束时间必须晚于开始时间。':'';
  const validTime=!timeError;
  const ready=Boolean(validTime&&(trace.trim()||keyword.trim()||msgSn.trim()));
  function reset(session='',rangePrompt?:string){
    const restored=readAppliedRange(session,rangePrompt,storageKey);
    const filters=readAppliedFilters(session,rangePrompt,storageKey);
    setApplied(restored);setRange(restored?rangeFromBounds(restored):recentRange(60));
    setAppliedFilters(filters);setForm(!session);setTrace(filters.trace);setKeyword(filters.keyword);setMsgSn(filters.msgSn);
  }
  function onSent(session=sessionStorage.getItem(storageKey)){
    if(form){
      const filters={trace:trace.trim(),keyword:keyword.trim(),msgSn:msgSn.trim()};
      setApplied({from,to});setAppliedFilters(filters);
      if(session){
        sessionStorage.setItem(storageKey+'.range.'+session,JSON.stringify({from,to}));
        sessionStorage.setItem(storageKey+'.filters.'+session,JSON.stringify(filters));
      }
      setForm(false);
    }
  }
  function beginEdit(){
    if(applied)setRange(rangeFromBounds(applied));
    setTrace(appliedFilters.trace);setKeyword(appliedFilters.keyword);setMsgSn(appliedFilters.msgSn);
    setForm(true);
  }
  function cancelEdit(){
    if(applied)setRange(rangeFromBounds(applied));
    setTrace(appliedFilters.trace);setKeyword(appliedFilters.keyword);setMsgSn(appliedFilters.msgSn);
    setForm(false);
  }
  function prompt(question:string){
    if(!form)return question;
    const lines=['请根据以下条件排查日志，列出证据、异常原因及下一步。',
      mode==='message'?
        '排查场景：报文问题排查。关注报文收发、字段校验与处理链路；仅使用可验证的数据源，缺少报文查询工具时说明证据不足。':
        '排查场景：公管问题排查。关注公管业务操作及上下游链路，使用当前可用日志证据，不臆测数据库或服务状态。',
      '若当前配置的执行入口需要验证码等补充信息，请使用当前会话的 OpenCode question 表单收集回答，保留前导零，再继续排查。网页无法向后台脚本的 stdin 输入。',
      '查询时区：'+localZone+'。请按以下起止时刻查询；若工具需要无时区的本地时间，使用该时区对应的日期和时间。',
      '时间范围（起点含、终点不含）：'+formatBounds({from,to}),
      trace.trim()&&'Trace ID：'+trace.trim(),msgSn.trim()&&'msgSn：'+msgSn.trim(),keyword.trim()&&'关键字：'+keyword.trim(),
      question&&'排查问题：'+question,
      '请使用当前绑定目录中配置的工具执行；若不存在可验证的数据源，说明证据缺口。'];
    return lines.filter(Boolean).join('\n');
  }
  const summary=<div className="log-applied-range">
    <div className="log-applied-overview">
      <span className="log-applied-label">当前排查条件</span>
      {applied&&<span className="log-applied-time">
        <span aria-hidden="true">◷</span> {localTime(applied.from).replace('T',' ')} — {localTime(applied.to).replace('T',' ')}
      </span>}
      {appliedFilters.trace&&<span className="log-applied-chip" title={'Trace ID：'+appliedFilters.trace}>Trace ID · {appliedFilters.trace}</span>}
      {appliedFilters.msgSn&&<span className="log-applied-chip" title={'msgSn：'+appliedFilters.msgSn}>msgSn · {appliedFilters.msgSn}</span>}
      {appliedFilters.keyword&&<span className="log-applied-chip" title={'关键词：'+appliedFilters.keyword}>关键词 · {appliedFilters.keyword}</span>}
    </div>
    <button type="button" className="secondary log-adjust-button" onClick={beginEdit}>
      <span aria-hidden="true">⚙</span> 调整排查条件
    </button>
  </div>;
  const controls=form?<fieldset className="log-query"><legend>排查条件</legend>
    <section className="log-time-range" aria-labelledby="log-range-title">
      <header><h2 id="log-range-title">时间范围</h2><span className="range-zone">精确到秒</span></header>
      <div className="range-presets" role="group" aria-label="快捷时间范围">
        {presets.map(p=><button key={p.minutes} type="button" aria-pressed={range.preset===p.minutes}
          onClick={()=>setRange(recentRange(p.minutes))}>{p.label}</button>)}
      </div>
      <div className="range-fields">
        <label>开始时间<input required type="datetime-local" step="1" value={start} onChange={e=>editTime('start',e.target.value)}
          aria-invalid={!Number.isFinite(from)} aria-describedby="log-range-feedback"/></label>
        <span className="range-direction" aria-hidden="true">→</span>
        <label>结束时间<input required type="datetime-local" step="1" value={end} onChange={e=>editTime('end',e.target.value)}
          aria-invalid={!Number.isFinite(to)||to<=from} aria-describedby="log-range-feedback"/></label>
      </div>
      <div className="range-footer" id="log-range-feedback">
        {timeError?<span role="alert">{timeError}</span>:<><strong>跨度 {elapsed(to-from)}</strong><span>含开始，不含结束</span></>}
      </div>
    </section>
    <div className="log-grid">
      <label>Trace ID<input value={trace} maxLength={256} onChange={e=>setTrace(e.target.value)} placeholder="可选，三项至少填写一项"/></label>
      <label>msgSn<input value={msgSn} maxLength={256} onChange={e=>setMsgSn(e.target.value)} placeholder="可选，支持前导零"/></label>
      <label>关键词<input value={keyword} maxLength={512} onChange={e=>setKeyword(e.target.value)} placeholder="可选，三项至少填写一项"/></label>
    </div>
    {!ready&&<small className="muted" role="status">时间为必填；Trace ID、msgSn、关键词至少填写一项。</small>}
  </fieldset>:summary;
  return <DirectChat sceneId={mode==='message'?'message':'logs'} globalModel={globalModel} investigation={{controls,summary,initial:form,ready,prompt,onSent,reset,cancel:cancelEdit,title,sessionKey:storageKey}}/>;
}
