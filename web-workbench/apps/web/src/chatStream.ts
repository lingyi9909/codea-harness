import {useCallback,useEffect,useRef,useState} from 'react';
import type {ChatFragment,ChatMessage,ChatStep,ToolProgress} from '../../../packages/contracts/src/chat.ts';

type Part={message:string;ordinal:number;kind:'thinking'|'text';text:string;at:number;ended?:number};
type Frame={session:string;parts:Part[];tools:ToolProgress[]};
type Connection={session:string;ready:Promise<void>;close:()=>void};

export function useChatStream(session:string){
  const connection=useRef<Connection|null>(null);
  const [frame,setFrame]=useState<Frame>({session:'',parts:[],tools:[]});
  const prune=useRef<(messages:ChatMessage[])=>void>(()=>{});
  const connect=useCallback((id:string)=>{
    if(connection.current?.session===id)return connection.current.ready;
    connection.current?.close();
    const source=new EventSource('/api/v1/chat/sessions/'+encodeURIComponent(id)+'/activity');
    const initial:Frame={session:id,parts:[],tools:[]};
    let current=initial,paint=0;
    const seen=new Set<string>();
    const interrupted=new Set<Part>();
    const publish=()=>{
      if(!paint)paint=requestAnimationFrame(()=>{paint=0;setFrame(current);});
    };
    // A server without live events still accepts prompts and supplies snapshots.
    let release!:()=>void;
    const ready=new Promise<void>(resolve=>{release=resolve;});
    const timeout=setTimeout(release,2000);
    const connected=()=>{clearTimeout(timeout);release();};
    source.addEventListener('ready',connected);
    source.onerror=()=>{
      connected();
      for(const part of current.parts)if(part.ended==null)interrupted.add(part);
    };
    source.onmessage=e=>{
      const tool=JSON.parse(e.data) as ToolProgress;
      if(current.tools.some(row=>row.id===tool.id))return;
      current={...current,tools:[...current.tools.slice(-11),tool]};publish();
    };
    source.addEventListener('fragment',e=>{
      const f=JSON.parse(e.data) as ChatFragment;
      if(seen.has(f.id))return;
      seen.add(f.id);
      if(seen.size>4096)seen.delete(seen.values().next().value!);
      const parts=[...current.parts];
      const index=parts.findIndex(p=>p.message===f.message&&p.ordinal===f.ordinal);
      const previous=parts[index];
      if(f.phase==='delta'&&previous?.ended!=null)return;
      if(f.phase==='started'&&previous)return;
      // A subscription may join mid-block. Mark the gap until the end event supplies the whole text.
      const prefix=previous?previous.text+(interrupted.delete(previous)?'…':''):'…';
      const next:Part={message:f.message,ordinal:f.ordinal,kind:f.kind,at:previous?.at??f.at,
        text:f.phase==='delta'?prefix+f.text:f.text,ended:f.phase==='ended'?f.at:undefined};
      if(index<0)parts.push(next);else parts[index]=next;
      current={...current,parts};publish();
    });
    const close=()=>{source.close();connected();cancelAnimationFrame(paint);};
    connection.current={session:id,ready,close};
    setFrame(initial);
    // Reclaim buffered text once the authoritative assistant message is available.
    prune.current=(messages)=>{
      const completed=new Set(messages.filter(m=>m.complete).map(m=>m.id));
      const parts=current.parts.filter(p=>!completed.has(p.message));
      for(const part of interrupted)if(completed.has(part.message))interrupted.delete(part);
      if(parts.length!==current.parts.length){current={...current,parts};publish();}
    };
    return ready;
  },[]);
  useEffect(()=>{
    if(session)void connect(session);
    return ()=>{if(connection.current?.session===session){connection.current.close();connection.current=null;}};
  },[session,connect]);
  return {connect,prune,parts:frame.session===session?frame.parts:[],tools:frame.session===session?frame.tools:[]};
}

export function streamingMessages(history:ChatMessage[],parts:Part[]):ChatMessage[]{
  if(!parts.length)return history;
  const messages=[...history];
  for(const part of parts){
    let index=messages.findIndex(m=>m.id===part.message);
    if(index<0){index=messages.length;messages.push({id:part.message,role:'assistant',text:'',files:[],tools:[]});}
    const message=messages[index];
    if(message.complete)continue;
    const steps=[...(message.steps??[])];
    const slot=steps.findIndex(s=>s.ordinal===part.ordinal);
    const saved=steps[slot];
    const text=part.ended==null&&saved?.kind==='text'&&saved.text.startsWith(part.text)?saved.text:part.text;
    const step:ChatStep=part.kind==='text'?{kind:'text',ordinal:part.ordinal,text}:
      {kind:'thinking',ordinal:part.ordinal,text,at:part.at,ended:part.ended,complete:part.ended!=null,hasText:!!text,live:true};
    if(slot>=0)steps[slot]=step;
    else{
      const after=steps.findIndex(s=>s.ordinal==null||s.ordinal>part.ordinal);
      steps.splice(after<0?steps.length:after,0,step);
    }
    messages[index]={...message,steps};
  }
  return messages;
}
