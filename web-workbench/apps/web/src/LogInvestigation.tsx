import {useMemo,useRef,useState} from 'react';
import {DirectChat} from './DirectChat.tsx';

const localMinute=(date:Date)=>new Date(date.getTime()-date.getTimezoneOffset()*60000).toISOString().slice(0,16);
type Row={number:number;text:string;time:number};
function parseRow(text:string,i:number):Row{
  const stamp=text.match(/\b\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(?:[.,]\d+)?(?:Z|[+-]\d\d:\d\d)?/i)?.[0];
  return {number:i+1,text,time:stamp?Date.parse(stamp.replace(' ','T').replace(',','.')):NaN};
}
export function LogInvestigation(){
  const [form,setForm]=useState(()=>!sessionStorage.getItem('harness.logs'));
  const [params,setParams]=useState(''),[trace,setTrace]=useState(''),[keyword,setKeyword]=useState('');
  const [start,setStart]=useState(()=>localMinute(new Date(Date.now()-3600000)));
  const [end,setEnd]=useState(()=>localMinute(new Date(Date.now()+60000)));
  const [level,setLevel]=useState('全部'),[file,setFile]=useState<{name:string;rows:Row[]}|null>(null);
  const [fileError,setFileError]=useState(''),[reading,setReading]=useState(false);
  const generation=useRef(0),fileInput=useRef<HTMLInputElement>(null);
  const from=Date.parse(start),to=Date.parse(end),validTime=Number.isFinite(from)&&Number.isFinite(to)&&from<to;
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
    setForm(true);setParams('');setTrace('');setKeyword('');setLevel('全部');void load();
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
    <div className="log-grid"><label>开始时间<input type="datetime-local" value={start} onChange={e=>setStart(e.target.value)}/></label>
      <label>结束时间<input type="datetime-local" value={end} onChange={e=>setEnd(e.target.value)}/></label>
      <label>Trace ID<input value={trace} maxLength={256} onChange={e=>setTrace(e.target.value)} placeholder="与关键字至少填一项"/></label>
      <label>关键字<input value={keyword} maxLength={512} onChange={e=>setKeyword(e.target.value)} placeholder="例如 timeout"/></label></div>
    <small>{validTime?'本地时间 · 不含结束时刻':'结束时间必须晚于开始时间。'}</small>
    <details><summary>日志来源与级别{file?' · '+file.name:''}</summary>
      <label>日志级别<select value={level} onChange={e=>setLevel(e.target.value)}>{['全部','ERROR','WARN','INFO','DEBUG'].map(l=><option key={l}>{l}</option>)}</select></label>
      <label>导入本地日志<input ref={fileInput} type="file" accept=".log,.txt,.jsonl" onChange={e=>void load(e.target.files?.[0])}/></label>
      {(file||fileError)&&<button type="button" onClick={()=>{if(fileInput.current)fileInput.current.value='';void load();}}>移除日志文件</button>}
      <small>不导入时使用项目日志工具。导入后在本地筛选，发送时附最多 30 行、6000 字的摘录。</small>
      {file&&<><p>匹配 {matches.length} 行 · 无可识别时间 {file.rows.filter(r=>!Number.isFinite(r.time)).length} 行</p>
        <pre>{matches.slice(0,200).map(r=>r.number+'  '+r.text).join('\n')}</pre></>}
    </details>{fileError&&<p role="alert">{fileError}</p>}{reading&&<small>读取中…</small>}
  </fieldset>:<button className="secondary" onClick={()=>setForm(true)}>调整排查条件</button>;
  return <DirectChat investigation={{controls,initial:form,ready,prompt,onSent:()=>setForm(false),reset}}/>;
}
