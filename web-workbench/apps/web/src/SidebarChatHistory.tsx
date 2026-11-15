import {useEffect,useState} from 'react';
import {api} from './api/client.ts';
import {readHistory} from './chatHistory.ts';

/** Browser-local history only: no new aggregation API and no invented sessions. */
export function SidebarChatHistory({onSelect,onViewAll}:{onSelect:(id:string)=>void;onViewAll:()=>void}){
  const [directory,setDirectory]=useState('');
  const [revision,setRevision]=useState(0);
  useEffect(()=>{
    let active=true;
    api.chatSettings().then(settings=>{if(active)setDirectory(settings.directories[0]??'');}).catch(()=>{});
    const refresh=()=>setRevision(n=>n+1);
    window.addEventListener('harness:chat-history',refresh);
    window.addEventListener('storage',refresh);
    window.addEventListener('focus',refresh);
    return()=>{active=false;window.removeEventListener('harness:chat-history',refresh);
      window.removeEventListener('storage',refresh);window.removeEventListener('focus',refresh);};
  },[]);
  // revision is a refresh signal for localStorage, not a second source of truth.
  const rows=revision>=0&&directory?readHistory(localStorage,'harness.chat',directory).slice(0,5):[];
  return <div className="recent-chat-list" aria-label="最近对话">
    {rows.map(row=><button className="recent-chat-link" key={row.id} title={row.title}
      onClick={()=>onSelect(row.id)}><span aria-hidden="true">▤</span><span>{row.title}</span></button>)}
    {!rows.length&&<p className="recent-chat-empty">暂无对话记录</p>}
    {rows.length>=5&&<button className="recent-chat-all" onClick={onViewAll}>查看全部对话</button>}
  </div>;
}
