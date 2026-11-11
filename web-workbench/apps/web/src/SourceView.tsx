import { useEffect,useState } from 'react';
import { api,type Schema } from './api/client.ts';

export function SourceView({taskId}:{taskId:string}){
  const [open,setOpen]=useState(false);
  const [files,setFiles]=useState<Schema<'File'>[]>([]);
  const [id,setId]=useState('');
  const [start,setStart]=useState(1);
  const [page,setPage]=useState<Schema<'FileLines'>|null>(null);
  const [error,setError]=useState('');
  useEffect(()=>{
    if(!open)return;
    let active=true;
    api.files(taskId).then(v=>{if(active){setFiles(v.items);setId(v.items[0]?.id??'');}})
      .catch(e=>{if(active)setError(String(e));});
    return ()=>{active=false;};
  },[taskId,open]);
  useEffect(()=>{
    if(!open||!id)return;
    let active=true;setPage(null);setError('');
    api.lines(taskId,id,start).then(v=>{if(active)setPage(v);}).catch(e=>{if(active)setError(String(e));});
    return ()=>{active=false;};
  },[taskId,id,start,open]);
  return <section className="detail-card">
    <button aria-expanded={open} onClick={()=>setOpen(!open)}>{open?'收起源码':'查看固定版本源码'}</button>
    {open&&<div>
      <label>文件与版本<select value={id} onChange={e=>{setId(e.target.value);setStart(1);}}>
        {files.map(f=><option key={f.id} value={f.id}>{f.side} · {f.path}</option>)}</select></label>
      {error&&<p role="alert">{error}</p>}
      {page&&<><pre className="report-text">{page.lines.map((line,i)=>String(page.startLine+i)+'  '+line).join('\n')}</pre>
        <button disabled={start===1} onClick={()=>setStart(Math.max(1,start-200))}>上一页</button>
        <button disabled={!page.hasMore} onClick={()=>setStart(page.endLine+1)}>下一页</button></>}
    </div>}
  </section>;
}
