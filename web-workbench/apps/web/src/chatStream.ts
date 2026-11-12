import {useCallback,useEffect,useRef,useState} from 'react';
import type {ChatFragment,ChatMessage,ChatStep,ToolProgress} from '../../../packages/contracts/src/chat.ts';

type Part={message:string;ordinal:number;kind:'thinking'|'text';text:string;at:number;ended?:number;runs?:string[]};
type Frame={session:string;parts:Part[];tools:ToolProgress[]};
type Connection={session:string;ready:Promise<void>;close:()=>void};

const emptyParts:Part[]=[];
const emptyTools:ToolProgress[]=[];

// Each run is contiguous native text. A new run starts after an SSE gap.
function fillGaps(snapshot:string,runs:string[]){
  if(!snapshot)return runs.join('…');
  let result=snapshot;
  let position=0;
  for(const run of runs){
    if(!run)continue;
    const covered=result.indexOf(run,position);
    if(covered>=0){position=covered+run.length;continue;}
    // The snapshot can end in the middle of this run. Only join on an
    // observed overlap; keep an explicit gap if the missing text is unknown.
    const limit=Math.min(result.length-position,run.length);
    let overlap=limit;
    while(overlap>0&&!result.endsWith(run.slice(0,overlap)))overlap--;
    result+=(overlap?'':'…')+run.slice(overlap);
    position=result.length;
  }
  return result;
}

export function applyFragment(previous:Part|undefined,f:ChatFragment,interrupted=false):Part{
  let runs=previous?.runs;
  let text=f.text;
  if(f.phase==='delta'){
    if(!previous||interrupted)runs=[...(runs??[previous?.text??'']),f.text];
    else if(runs)runs=[...runs.slice(0,-1),runs[runs.length-1]+f.text];
    text=runs?runs.join('…'):(previous?.text??'')+f.text;
  }
  if(f.phase==='ended'){
    text=runs?fillGaps(f.text,runs):previous?.text.startsWith(f.text)?previous.text:f.text;
    if(f.text===text)runs=undefined;
  }
  return {message:f.message,ordinal:f.ordinal,kind:f.kind,at:previous?.at??f.at,
    text,runs,ended:f.phase==='ended'?f.at:undefined};
}

function mergedText(part:Part,snapshot:string,complete:boolean){
  if(part.runs)return fillGaps(snapshot||part.text,part.runs);
  if(part.text.startsWith(snapshot))return part.text;
  if(snapshot.startsWith(part.text)||complete)return snapshot;
  return part.text;
}

export function useChatStream(session:string){
  const [requestsVersion,refreshRequests]=useState(0);
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
    const flush=()=>{cancelAnimationFrame(paint);paint=0;setFrame(current);};
    const publish=()=>{
      if(document.hidden)flush();
      else if(!paint)paint=requestAnimationFrame(flush);
    };
    const onVisibility=()=>{if(document.hidden&&paint)flush();};
    document.addEventListener('visibilitychange',onVisibility);
    // A server without live events still accepts prompts and supplies snapshots.
    let release!:()=>void;
    const ready=new Promise<void>(resolve=>{release=resolve;});
    const timeout=setTimeout(release,2000);
    const connected=()=>{clearTimeout(timeout);release();};
    source.addEventListener('ready',connected);
    source.addEventListener('requests',()=>refreshRequests(n=>n+1));
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
      const next=applyFragment(previous,f,interrupted.delete(previous));
      if(index<0)parts.push(next);else parts[index]=next;
      current={...current,parts};publish();
    });
    const close=()=>{document.removeEventListener('visibilitychange',onVisibility);source.close();connected();cancelAnimationFrame(paint);};
    connection.current={session:id,ready,close};
    setFrame(initial);
    // Reclaim buffered text once the authoritative assistant message is available.
    prune.current=(messages)=>{
      const completed=new Map(messages.filter(m=>m.complete).map(m=>[m.id,m]));
      const parts=current.parts.filter(p=>{
        const message=completed.get(p.message);
        const step=message?.steps?.find(s=>s.ordinal===p.ordinal&&s.kind===p.kind);
        const saved=step&&(step.kind==='thinking'||step.kind==='text')?step.text:undefined;
        // Keep public live content when a final snapshot omitted or truncated it.
        return !message||mergedText(p,saved??'',true)!==(saved??'');
      });
      for(const part of interrupted)if(completed.has(part.message))interrupted.delete(part);
      if(parts.length!==current.parts.length){current={...current,parts};publish();}
    };
    return ready;
  },[]);
  useEffect(()=>{
    if(session)void connect(session);
    return ()=>{if(connection.current?.session===session){connection.current.close();connection.current=null;}};
  },[session,connect]);
  return {connect,prune,requestsVersion,parts:frame.session===session?frame.parts:emptyParts,tools:frame.session===session?frame.tools:emptyTools};
}

export function streamingMessages(history:ChatMessage[],parts:Part[]):ChatMessage[]{
  if(!parts.length)return history;
  const messages=[...history];
  for(const part of parts){
    let index=messages.findIndex(m=>m.id===part.message);
    if(index<0){
      const queued=messages.findIndex(m=>m.queued);
      index=queued>0?queued:messages.length;
      messages.splice(index,0,{id:part.message,role:'assistant',text:'',files:[],tools:[]});
    }
    const message=messages[index];

    const steps=[...(message.steps??[])];
    const slot=steps.findIndex(s=>s.ordinal===part.ordinal);
    const saved=steps[slot];
    const snapshot=saved?.kind==='text'||saved?.kind==='thinking'?saved.text??'':'';
    // Prefer the longer compatible prefix, even if the poll beat the SSE flush.
    const text=mergedText(part,snapshot,Boolean(message.complete));
    const complete=Boolean(message.complete||part.ended!=null||(saved?.kind==='thinking'&&saved.complete));
    const step:ChatStep=part.kind==='text'?{kind:'text',ordinal:part.ordinal,text}:
      {kind:'thinking',ordinal:part.ordinal,text,at:part.at,ended:part.ended,complete,hasText:!!text,live:true};
    if(slot>=0)steps[slot]=step;
    else{
      const after=steps.findIndex(s=>s.ordinal==null||s.ordinal>part.ordinal);
      steps.splice(after<0?steps.length:after,0,step);
    }
    messages[index]={...message,steps};
  }
  return messages;
}
