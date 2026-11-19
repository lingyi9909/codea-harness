import type { Schema } from './client.ts';

export type EventStreamState='connecting'|'connected'|'disconnected';
const eventTypes=[
  'task.created','scope.ready','run.status','message.delta','message.completed',
  'tool.started','tool.completed','interaction.pending','interaction.resolved',
  'artifact.ready','continuity.gap',
] as const;
export function subscribeTask(taskId:string,callbacks:{
  onEvent:(value:Schema<'Event'>)=>void;
  onState:(value:EventStreamState)=>void;
  onGap:(reason:string)=>void;
},after?:string):()=>void{
  // EventSource automatically sends Last-Event-ID upon reconnect. The server
  // explicitly prioritizes that header over the initial after cursor.
  const base='/api/v1/tasks/'+encodeURIComponent(taskId)+'/events';
  const url=after&&/^(0|[1-9][0-9]*)$/.test(after)?base+'?after='+after:base;
  const source=new EventSource(url,{withCredentials:true});
  callbacks.onState('connecting');
  let lastSeq=after&&/^(0|[1-9][0-9]*)$/.test(after)?BigInt(after):0n;
  const consume=(event:Event)=>{
    try{
      const value=JSON.parse((event as MessageEvent<string>).data) as Schema<'Event'>;
      if(value.taskId!==taskId||typeof value.seq!=='string'||!/^(0|[1-9][0-9]*)$/.test(value.seq)||
        !eventTypes.some(t=>t===value.type))throw new Error('INVALID_EVENT');
      const seq=BigInt(value.seq);
      if(seq<=lastSeq)return;
      if(seq!==lastSeq+1n)callbacks.onGap('持久事件序号存在缺口，正在重新读取任务状态');
      lastSeq=seq;
      callbacks.onEvent(value);
    }catch{callbacks.onGap('事件格式或连续性异常，请重新读取任务状态');}
  };
  // The server emits SSE "event: run.status" etc; onmessage alone does NOT
  // receive named events. Subscribe to the complete persisted event contract.
  for(const type of eventTypes)source.addEventListener(type,consume);
  source.addEventListener('message',consume); // fallback for default SSE frames
  source.onopen=()=>callbacks.onState('connected');
  source.onerror=()=>{
    callbacks.onState('disconnected');
    callbacks.onGap('事件流中断，客户端会尝试重连；任务状态以服务端为准');
  };
  return()=>{source.close();};
}
