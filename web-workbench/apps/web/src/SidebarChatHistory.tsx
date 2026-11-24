/**
 * 侧栏最近对话：只展示后端确认归属于当前登录会话的 OpenCode 会话。
 * 浏览器 LocalStorage 仅缓存标题，不能凭旧缓存推断会话归属。
 */
import {localStorage} from './userStorage.ts';
import {useEffect,useState} from 'react';
import {api} from './api/client.ts';
import {displayChatTitle,readHistory,type HistoryRow} from './chatHistory.ts';

export function SidebarChatHistory({onSelect,onViewAll}:{onSelect:(id:string)=>void;onViewAll:()=>void}){
  const [directory,setDirectory]=useState('');
  const [revision,setRevision]=useState(0);
  const [rows,setRows]=useState<HistoryRow[]>([]);
  useEffect(()=>{
    let active=true;
    api.chatSettings().then(settings=>{
      if(active)setDirectory(settings.directories[0]??'');
    }).catch(()=>{if(active){setDirectory('');setRows([]);}});
    const refresh=()=>setRevision(n=>n+1);
    window.addEventListener('harness:chat-history',refresh);
    window.addEventListener('storage',refresh);
    window.addEventListener('focus',refresh);
    return()=>{active=false;window.removeEventListener('harness:chat-history',refresh);
      window.removeEventListener('storage',refresh);window.removeEventListener('focus',refresh);};
  },[]);
  useEffect(()=>{
    if(!directory){setRows([]);return;}
    let active=true;
    // Never render a cached session merely because its ID exists in this browser.
    api.chatHistory(directory,'direct-chat').then(page=>{
      if(!active)return;
      const cached=new Map(readHistory(localStorage,'harness.chat',directory).map(row=>[row.id,row]));
      setRows(page.items.slice(0,5).map(row=>({
        id:row.id,title:displayChatTitle(row.title,cached.get(row.id)?.title),updated:row.updated,
      })));
    }).catch(()=>{if(active)setRows([]);});
    return()=>{active=false;};
  },[directory,revision]);
  return <div className="recent-chat-list" aria-label="最近对话">
    {rows.map(row=><button className="recent-chat-link" key={row.id} title={row.title}
      onClick={()=>onSelect(row.id)}><span aria-hidden="true">▤</span><span>{row.title}</span></button>)}
    {!rows.length&&<p className="recent-chat-empty">暂无对话记录</p>}
    {rows.length>=5&&<button className="recent-chat-all" onClick={onViewAll}>查看全部对话</button>}
  </div>;
}
