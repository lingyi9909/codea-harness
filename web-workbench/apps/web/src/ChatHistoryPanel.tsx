import {useEffect,useState} from 'react';
import {api} from './api/client.ts';
import {displayChatTitle,forgetChat,readHistory,rememberChat,type HistoryRow,type SceneKey} from './chatHistory.ts';
import './chat-history.css';

export function ChatHistoryPanel({scene,directory,active,disabled,running,onSelect,onRenamed,onDeleted}:{
  scene:SceneKey;directory:string;active:string;disabled:boolean;running?:string;
  onSelect:(row:HistoryRow)=>void;onRenamed:(row:HistoryRow)=>void;onDeleted:(id:string)=>void;
}){
  const [revision,refresh]=useState(0),[remote,setRemote]=useState<HistoryRow[]>([]);
  const [cursor,setCursor]=useState<string>(),[next,setNext]=useState<string>();
  const [loading,setLoading]=useState(true),[error,setError]=useState(''),[retry,again]=useState(0);
  const [editing,setEditing]=useState(''),[deleting,setDeleting]=useState(''),[draft,setDraft]=useState('');
  const [saving,setSaving]=useState('');
  useEffect(()=>{
    let live=true;
    setLoading(true);setError('');
    api.chatHistory(directory||undefined,scene==='harness.chat'?'direct-chat':scene==='harness.message'?'message-inspection':scene==='harness.data'?'data-analysis':'log-inspection',cursor)
      .then(page=>{
        if(!live)return;
        setRemote(previous=>cursor?[...previous,...page.items]:page.items);
        setNext(page.next===cursor?undefined:page.next);
      }).catch(()=>{if(live)setError('暂时无法读取服务端历史。');})
      .finally(()=>{if(live)setLoading(false);});
    return ()=>{live=false;};
  },[scene,directory,cursor,retry]);
  // OpenCode title is authoritative; browser history only caches the index.
  const local=readHistory(localStorage,scene,directory);
  void revision;
  const rows=new Map(local.map(row=>[row.id,row]));
  for(const row of remote){
    const known=rows.get(row.id);
    rows.set(row.id,{id:row.id,title:displayChatTitle(row.title,known?.title),updated:Math.max(row.updated||0,known?.updated??0)});
  }
  const sorted=Array.from(rows.values()).sort((a,b)=>b.updated-a.updated);

  function editRow(row:HistoryRow){
    setError('');setDeleting('');setEditing(row.id);setDraft(row.title);
  }
  async function saveTitle(row:HistoryRow){
    const title=draft.trim();
    if(!title||Array.from(title).length>120){
      setError('标题不能为空，且最多 120 个字符。');return;
    }
    if(title===row.title){setEditing('');return;}
    setSaving(row.id);setError('');
    try{
      const result=await api.renameChat(row.id,title);
      const updated={...row,title:result.title};
      rememberChat(localStorage,scene,directory,updated);
      setRemote(previous=>previous.map(item=>item.id===row.id?{...item,title:result.title}:item));
      refresh(n=>n+1);setEditing('');
      onRenamed(updated);
      window.dispatchEvent(new Event('harness:chat-history'));
    }catch(e){setError(e instanceof Error?'重命名失败：'+e.message:'重命名失败，请重试。');}
    finally{setSaving('');}
  }
  async function deleteRow(row:HistoryRow){
    setSaving(row.id);setError('');
    try{
      await api.deleteChat(row.id);
      forgetChat(localStorage,sessionStorage,scene,directory,row.id);
      setRemote(previous=>previous.filter(item=>item.id!==row.id));
      refresh(n=>n+1);setDeleting('');
      onDeleted(row.id);
      window.dispatchEvent(new Event('harness:chat-history'));
    }catch(e){setError(e instanceof Error?'删除失败：'+e.message:'删除失败，请重试。');}
    finally{setSaving('');}
  }
  return <section className="conversation-history" aria-label="历史对话" id="conversation-history">
    <div className="history-panel-heading"><h2>历史对话</h2><small>可重命名或删除</small></div>
    <ul>{sorted.map(row=><li key={row.id}>
      {editing===row.id?<div className="history-inline-editor">
        <label>对话名称<input aria-label="对话名称" autoFocus maxLength={120} value={draft} disabled={Boolean(saving)}
          onChange={e=>setDraft(e.target.value)} onKeyDown={e=>{if(e.key==='Enter'){e.preventDefault();void saveTitle(row);}
            if(e.key==='Escape')setEditing('');}}/></label>
        <div className="history-inline-actions">
          <button type="button" className="history-save" disabled={Boolean(saving)||!draft.trim()} onClick={()=>void saveTitle(row)}>保存</button>
          <button type="button" disabled={Boolean(saving)} onClick={()=>setEditing('')}>取消</button>
        </div>
      </div>:deleting===row.id?<div className="history-delete-confirm">
        <p>确定删除「{row.title||'未命名对话'}」？OpenCode 中的会话及聊天记录也将被永久删除，无法恢复。</p>
        <div className="history-inline-actions">
          <button type="button" className="history-danger" disabled={Boolean(saving)} onClick={()=>void deleteRow(row)}>确认删除</button>
          <button type="button" disabled={Boolean(saving)} onClick={()=>setDeleting('')}>取消</button>
        </div>
      </div>:<div className="history-row">
        <button type="button" className="history-open" disabled={disabled||Boolean(saving)}
          aria-current={active===row.id?'true':undefined} onClick={()=>onSelect(row)}>
          <span className="history-row-title">{row.title||'对话 '+row.id}</span>
          <small>{active===row.id?'当前 · ':''}{new Date(row.updated).toLocaleString()}</small>
        </button>
        <div className="history-row-actions">
          <button type="button" disabled={disabled||loading||Boolean(saving)} onClick={()=>editRow(row)}>重命名</button>
          <button type="button" className="history-delete" disabled={disabled||loading||Boolean(saving)||running===row.id}
            title={running===row.id?'会话执行中，结束后可删除':undefined}
            onClick={()=>{setError('');setEditing('');setDeleting(row.id);}}>删除</button>
        </div>
      </div>}
    </li>)}</ul>
    {!sorted.length&&!loading&&!error&&<p>当前目录暂无历史对话。</p>}
    {loading&&<p role="status">正在读取历史对话…</p>}
    {error&&<p role="alert" className="history-error">{error}</p>}
    {next&&!loading&&!error&&<button type="button" className="history-load-more" onClick={()=>setCursor(next)}>加载更多</button>}
    {!sorted.length&&error&&<button type="button" className="history-load-more" onClick={()=>{setError('');again(n=>n+1);}}>重试读取</button>}
  </section>;
}
