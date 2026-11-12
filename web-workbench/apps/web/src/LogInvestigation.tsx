import {useMemo,useRef,useState,type MouseEvent} from 'react';
import {DirectChat} from './DirectChat.tsx';

import './log-time-range.css';

const presets=[{minutes:15,label:'最近 15 分钟'},{minutes:60,label:'最近 1 小时'},{minutes:1440,label:'最近 24 小时'}];
const localZone=Intl.DateTimeFormat().resolvedOptions().timeZone;
type Bounds={from:number;to:number};
type TimeRange={start:string;end:string;from:number;to:number;preset:number|null};
function openTimePicker(event:MouseEvent<HTMLInputElement>){
  const field=event.currentTarget;
  if(!field.showPicker)return;
  try{
    field.showPicker();
    event.preventDefault();
  }catch{/* Keep the browser's native activation when programmatic opening is unavailable. */}
}
function localTime(instant:number){
  const date=new Date(instant);
  return new Date(instant-date.getTimezoneOffset()*60000).toISOString().slice(0,-1);
}
function rangeFromBounds({from,to}:Bounds,preset:number|null=null):TimeRange{
  return {start:localTime(from),end:localTime(to),from,to,preset};
}
function recentRange(minutes:number){
  const to=Date.now();
  return rangeFromBounds({from:to-minutes*60000,to},minutes);
}
function readAppliedRange():Bounds|null{
  const session=sessionStorage.getItem('harness.logs');
  try{
    const saved=JSON.parse(sessionStorage.getItem('harness.logs.range')??'null');
    if(session&&saved?.session===session&&Number.isFinite(saved.from)&&Number.isFinite(saved.to)&&
      Number.isFinite(new Date(saved.from).getTime())&&Number.isFinite(new Date(saved.to).getTime())&&saved.to>saved.from)
      return {from:saved.from,to:saved.to};
  }catch{/* Older sessions may have no saved range. */}
  return null;
}
function elapsed(ms:number){
  const hours=Math.floor(ms/3600000),minutes=Math.floor(ms%3600000/60000),seconds=ms%60000/1000;
  return [hours&&hours+' 小时',minutes&&minutes+' 分钟',seconds&&seconds+' 秒'].filter(Boolean).join(' ');
}
function parseLocal(value:string){
  const ms=Date.parse(value);
  if(!Number.isFinite(ms))return NaN;
  // Reject wall times skipped by a daylight-saving transition.
  const normalized=value.length===16?value+':00.000':value.length===19?value+'.000':value.padEnd(23,'0');
  return localTime(ms)===normalized?ms:NaN;
}
type Row={number:number;text:string;time:number};
function parseRow(text:string,i:number):Row{
  const stamp=text.match(/\b\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(?:[.,]\d+)?(?:Z|[+-]\d\d:\d\d)?/i)?.[0];
  return {number:i+1,text,time:stamp?Date.parse(stamp.replace(' ','T').replace(',','.')):NaN};
}
export function LogInvestigation(){
  const [form,setForm]=useState(()=>!sessionStorage.getItem('harness.logs'));
  const [params,setParams]=useState(''),[trace,setTrace]=useState(''),[keyword,setKeyword]=useState('');
  const [applied,setApplied]=useState(readAppliedRange);
  const [range,setRange]=useState(()=>applied?rangeFromBounds(applied):recentRange(60));
  const {from,to,start,end}=range;
  function editTime(side:'start'|'end',value:string){
    setRange(previous=>({...previous,[side]:value,[side==='start'?'from':'to']:parseLocal(value),preset:null}));
  }
  const [level,setLevel]=useState('全部'),[file,setFile]=useState<{name:string;rows:Row[]}|null>(null);
  const [fileError,setFileError]=useState(''),[reading,setReading]=useState(false);
  const generation=useRef(0),fileInput=useRef<HTMLInputElement>(null);
  const timeError=!start||!end?'请填写完整的开始和结束时间。':
    !Number.isFinite(from)||!Number.isFinite(to)?'所选时区中不存在这个本地时间。':
    from>=to?'结束时间必须晚于开始时间。':'';
  const validTime=!timeError;
  const ready=Boolean(validTime&&params.trim()&&(trace.trim()||keyword.trim())&&!reading&&!fileError);
  const matches=useMemo(()=>{
    const term=keyword.trim().toLowerCase(),id=trace.trim().toLowerCase();
    const severity=new RegExp('\\b'+level+'\\b','i');
    return file?.rows.filter(r=>r.time>=from&&r.time<to&&(level==='全部'||severity.test(r.text))&&
      r.text.toLowerCase().includes(id)&&r.text.toLowerCase().includes(term))??[];
  },[file,from,to,level,keyword,trace]);
  async function load(f?:File){
    const gen=++generation.current;setFile(null);setFileError('');setReading(!!f);
    if(!f)return;
    try{
      if(f.size>2*1024*1024)throw Error('请选择 2 MiB 以内的日志文件。');
      const text=new TextDecoder('utf-8',{fatal:true}).decode(await f.arrayBuffer());
      if(text.includes('\0'))throw Error('请选择 UTF-8 文本日志。');
      if(gen===generation.current)setFile({name:f.name,rows:text.split(/\r?\n/).map(parseRow).filter(r=>r.text.trim())});
    }catch(e){if(gen===generation.current)setFileError(e instanceof Error?e.message:'日志读取失败');}
    finally{if(gen===generation.current)setReading(false);}
  }
  function reset(){
    setApplied(null);sessionStorage.removeItem('harness.logs.range');setRange(recentRange(60));
    setForm(true);setParams('');setTrace('');setKeyword('');setLevel('全部');void load();
  }
  function onSent(){
    if(form){
      setApplied({from,to});
      sessionStorage.setItem('harness.logs.range',JSON.stringify({session:sessionStorage.getItem('harness.logs'),from,to}));
      setForm(false);
    }
  }
  function prompt(question:string){
    if(!form)return question;
    const lines=['请根据以下条件排查日志，列出证据、异常原因及下一步。','时间范围（起点含、终点不含）：'+new Date(from).toISOString()+' 至 '+new Date(to).toISOString(),
      '请求入参：'+params.trim(),trace.trim()&&'Trace ID：'+trace.trim(),keyword.trim()&&'关键字：'+keyword.trim(),
      '日志级别：'+level,question&&'排查问题：'+question];
    if(file){
      lines.push('导入日志：'+file.name+'；匹配 '+matches.length+' 行。以下为日志数据摘录（原始行号）：');
      const excerpt=matches.slice(0,30).map(r=>r.number+'  '+r.text).join('\n');
      lines.push(excerpt.slice(0,6000)||'无匹配行');
      if(matches.length>30||excerpt.length>6000)lines.push('摘录已截断。');
    }else lines.push('请用当前项目可用的日志工具或日志文件查询；若没有可用数据源，请说明缺少什么。');
    return lines.filter(Boolean).join('\n');
  }
  const controls=form?<fieldset className="log-query"><legend>排查条件</legend>
    <label>请求入参<textarea rows={2} maxLength={2000} value={params} onChange={e=>setParams(e.target.value)} placeholder={'粘贴请求参数（JSON 或文本），无参数填 {}'}/></label>
    <section className="log-time-range" aria-labelledby="log-range-title">
      <header><h2 id="log-range-title">时间范围</h2><span className="range-zone">本地时区 · {localZone}</span></header>
      <div className="range-presets" role="group" aria-label="快捷时间范围">
        {presets.map(p=><button key={p.minutes} type="button" aria-pressed={range.preset===p.minutes}
          onClick={()=>setRange(recentRange(p.minutes))}>{p.label}</button>)}
      </div>
      <div className="range-fields">
        <label>开始时间<input type="datetime-local" step="any" onClick={openTimePicker} value={start} onChange={e=>editTime('start',e.target.value)}
          aria-invalid={!Number.isFinite(from)} aria-describedby="log-range-feedback"/></label>
        <span className="range-direction" aria-hidden="true">→</span>
        <label>结束时间<input type="datetime-local" step="any" onClick={openTimePicker} value={end} onChange={e=>editTime('end',e.target.value)}
          aria-invalid={!Number.isFinite(to)||to<=from} aria-describedby="log-range-feedback"/></label>
      </div>
      <div className="range-footer" id="log-range-feedback">
        {timeError?<span role="alert">{timeError}</span>:<><strong>跨度 {elapsed(to-from)}</strong><span>含开始，不含结束</span></>}
      </div>
    </section>
    <div className="log-grid">
      <label>Trace ID<input value={trace} maxLength={256} onChange={e=>setTrace(e.target.value)} placeholder="与关键字至少填一项"/></label>
      <label>关键字<input value={keyword} maxLength={512} onChange={e=>setKeyword(e.target.value)} placeholder="例如 timeout"/></label></div>
    <details><summary>日志来源与级别{file?' · '+file.name:''}</summary>
      <label>日志级别<select value={level} onChange={e=>setLevel(e.target.value)}>{['全部','ERROR','WARN','INFO','DEBUG'].map(l=><option key={l}>{l}</option>)}</select></label>
      <label>导入本地日志<input ref={fileInput} type="file" accept=".log,.txt,.jsonl" onChange={e=>void load(e.target.files?.[0])}/></label>
      {(file||fileError)&&<button type="button" onClick={()=>{if(fileInput.current)fileInput.current.value='';void load();}}>移除日志文件</button>}
      <small>不导入时使用项目日志工具。导入后在本地筛选，发送时附最多 30 行、6000 字的摘录。</small>
      {file&&<><p>匹配 {matches.length} 行 · 无可识别时间 {file.rows.filter(r=>!Number.isFinite(r.time)).length} 行</p>
        <pre>{matches.slice(0,200).map(r=>r.number+'  '+r.text).join('\n')}</pre></>}
    </details>{fileError&&<p role="alert">{fileError}</p>}{reading&&<small>读取中…</small>}
  </fieldset>:<div className="log-applied-range">
    {applied&&<div><strong>上次排查范围</strong><span>{localTime(applied.from).replace('T',' ')} → {localTime(applied.to).replace('T',' ')} · {localZone}</span></div>}
    <button type="button" className="secondary" onClick={()=>setForm(true)}>调整排查条件</button>
  </div>;
  return <DirectChat investigation={{controls,initial:form,ready,prompt,onSent,reset}}/>;
}
