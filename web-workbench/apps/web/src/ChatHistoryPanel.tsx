import {useEffect,useState} from 'react';
import {api} from './api/client.ts';
import {readHistory,type HistoryRow,type SceneKey} from './chatHistory.ts';
import './chat-history.css';

export function ChatHistoryPanel({scene,directory,active,disabled,onSelect}:{
  scene:SceneKey;directory:string;active:string;disabled:boolean;onSelect:(row:HistoryRow)=>void;
}){
  const [local]=useState(()=>readHistory(localStorage,scene,directory));
  const [remote,setRemote]=useState<HistoryRow[]>([]);
  const [cursor,setCursor]=useState<string>(),[next,setNext]=useState<string>();
  const [loading,setLoading]=useState(true),[error,setError]=useState(''),[retry,again]=useState(0);
  useEffect(()=>{
    let live=true;
    setLoading(true);setError('');
    api.chatHistory(directory||undefined,scene==='harness.logs'?'log-inspection':'direct-chat',cursor)
      .then(page=>{
        if(!live)return;
        setRemote(previous=>cursor?[...previous,...page.items]:page.items);
        setNext(page.next===cursor?undefined:page.next);
      }).catch(()=>{if(live)setError('暂时无法读取服务端历史。');})
      .finally(()=>{if(live)setLoading(false);});
    return ()=>{live=false;};
  },[scene,directory,cursor,retry]);
  const rows=new Map(local.map(row=>[row.id,row]));
  for(const row of remote){
    const known=rows.get(row.id);
    const nativeTitle=row.title?.trim();
    const meaningful=nativeTitle&&!/^Code Harness · (直接对话|日志排查)$/.test(nativeTitle);
    rows.set(row.id,{id:row.id,title:meaningful?nativeTitle:known?.title||'',updated:Math.max(row.updated||0,known?.updated??0)});
  }
  const sorted=Array.from(rows.values()).sort((a,b)=>b.updated-a.updated);
  return <section className="conversation-history" aria-label="历史对话" id="conversation-history">
    <h2>当前目录的历史对话</h2>
    <ul>{sorted.map(row=><li key={row.id}><button disabled={disabled} aria-current={active===row.id?'true':undefined} onClick={()=>onSelect(row)}>
      <span>{row.title||'对话 '+row.id}</span><small>{active===row.id?'当前 · ':''}{new Date(row.updated).toLocaleString()}</small>
    </button></li>)}</ul>
    {!sorted.length&&!loading&&!error&&<p>当前目录暂无历史对话。</p>}
    {loading&&<p role="status">正在读取历史对话…</p>}
    {error&&<p>{error} <button onClick={()=>again(n=>n+1)}>重试</button></p>}
    {next&&!loading&&!error&&<button onClick={()=>setCursor(next)}>加载更多</button>}
  </section>;
}
