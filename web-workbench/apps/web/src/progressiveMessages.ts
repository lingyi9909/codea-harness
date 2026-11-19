import {useLayoutEffect,useMemo,useRef,useState} from 'react';
import type {ChatMessage} from '../../../packages/contracts/src/chat.ts';

const graphemes=new Intl.Segmenter(undefined,{granularity:'grapheme'});

// Store a UTF-16 offset, but advance it only at a visible character boundary.
export class TextReveal {
  target='';
  offset=0;
  private budget=0;
  private speed=0;
  private next?:IterableIterator<Intl.SegmentData>;
  update(value:string){
    if(this.target.startsWith(value))return;
    if(!value.startsWith(this.target.slice(0,this.offset))){
      // Apply a genuine upstream correction once, without retyping its prefix.
      this.offset=value.length;
    }
    this.target=value;
    this.next=undefined;
    this.speed=Math.max(1/24,(value.length-this.offset)/640);
  }
  finish(){this.offset=this.target.length;this.next=undefined;this.budget=0;}
  advance(elapsed:number){
    this.budget+=elapsed*this.speed;
    const count=Math.floor(this.budget);
    if(!count)return;
    this.budget-=count;
    this.next??=graphemes.segment(this.target.slice(this.offset))[Symbol.iterator]();
    const until=Math.min(this.offset+count,this.target.length);
    while(this.offset<until){
      const item=this.next.next();
      if(item.done)break;
      this.offset+=item.value.segment.length;
    }
    if(this.offset===this.target.length)this.finish();
  }
}

export class MessageReveal {
  private cursors=new Map<string,TextReveal>();
  private history=new Set<string>();
  private liveIds=new Set<string>();
  private receivedSnapshot=false;
  private messages:ChatMessage[]=[];
  private fields=new Map<ChatMessage,Map<number,TextReveal>>();
  private moving=new Set<TextReveal>();

  update(messages:ChatMessage[],snapshot:ChatMessage[]|undefined,active:boolean,fresh:boolean){
    if(snapshot&&!this.receivedSnapshot){
      this.receivedSnapshot=true;
      if(!fresh)for(const m of snapshot){
        if(!this.liveIds.has(m.id)&&(m.complete||!active))this.history.add(m.id);
      }
    }
    this.messages=messages;
    this.fields.clear();
    this.moving.clear();
    for(const m of messages){
      if(m.role!=='assistant'||this.history.has(m.id))continue;
      this.liveIds.add(m.id);
      const fields=new Map<number,TextReveal>();
      const track=(index:number,key:string,value:string)=>{
        let cursor=this.cursors.get(key);
        if(!cursor){cursor=new TextReveal();this.cursors.set(key,cursor);}
        cursor.update(value);
        if(cursor.offset<cursor.target.length)this.moving.add(cursor);
        if(cursor.offset<cursor.target.length||value!==cursor.target)fields.set(index,cursor);
      };
      if(m.steps?.length)m.steps.forEach((s,i)=>{
        if((s.kind==='thinking'||s.kind==='text')&&s.text)track(i,JSON.stringify([m.id,s.kind,s.ordinal??i]),s.text);
      });
      else if(m.text)track(-1,JSON.stringify([m.id,'text',0]),m.text);
      if(fields.size)this.fields.set(m,fields);
    }
  }
  get pending(){return this.moving.size>0;}
  advance(elapsed:number){
    for(const c of this.moving){
      if(elapsed===Infinity)c.finish();else c.advance(elapsed);
      if(c.offset===c.target.length)this.moving.delete(c);
    }
  }
  project(){
    if(!this.fields.size)return this.messages;
    return this.messages.map(m=>{
      const fields=this.fields.get(m);
      if(!fields)return m;
      const value=(c:TextReveal)=>c.target.slice(0,c.offset);
      const fallback=fields.get(-1);
      if(fallback)return {...m,text:value(fallback)};
      return {...m,steps:m.steps?.map((step,i)=>{
        const c=fields.get(i);
        if(!c)return step;
        const text=value(c);
        if(step.kind==='thinking'&&c.offset<c.target.length)return {...step,text,live:true,complete:false};
        return {...step,text};
      })};
    });
  }
}

export function useProgressiveMessages(session:string,messages:ChatMessage[],snapshot:ChatMessage[]|undefined,active:boolean,fresh:boolean){
  const timeline=useMemo(()=>new MessageReveal(),[session]);
  const [view,paint]=useState({timeline,messages:[] as ChatMessage[]});
  const schedule=useRef<()=>void>(()=>{});
  useLayoutEffect(()=>{
    let frame=0;
    let last=performance.now();
    const motion=window.matchMedia('(prefers-reduced-motion: reduce)');
    const step=(now:number)=>{
      frame=0;
      if(!timeline.pending)return;
      timeline.advance(document.hidden||motion.matches?Infinity:Math.max(0,now-last));
      last=now;
      paint({timeline,messages:timeline.project()});
      if(timeline.pending)frame=requestAnimationFrame(step);
    };
    // Keep one clock running across rapid deltas and snapshot replacements.
    const start=()=>{
      if(!timeline.pending)return;
      if(document.hidden||motion.matches){cancelAnimationFrame(frame);step(performance.now());}
      else if(!frame){last=performance.now();frame=requestAnimationFrame(step);}
    };
    schedule.current=start;
    motion.addEventListener('change',start);
    document.addEventListener('visibilitychange',start);
    return ()=>{
      cancelAnimationFrame(frame);
      motion.removeEventListener('change',start);
      document.removeEventListener('visibilitychange',start);
    };
  },[timeline]);
  useLayoutEffect(()=>{
    timeline.update(messages,snapshot,active,fresh);
    paint({timeline,messages:timeline.project()});
    schedule.current();
  },[timeline,messages,snapshot,active,fresh]);
  return view.timeline===timeline?view.messages:[];
}
