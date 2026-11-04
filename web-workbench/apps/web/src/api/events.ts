import type { Schema } from './client.ts';

export type EventStreamState = 'connecting' | 'connected' | 'disconnected';
export function subscribeTask(taskId:string, callbacks:{
  onEvent:(value:Schema<'Event'>)=>void;
  onState:(value:EventStreamState)=>void;
  onGap:(reason:string)=>void;
}):()=>void {
  const url='/api/v1/tasks/'+encodeURIComponent(taskId)+'/events';
  const source=new EventSource(url,{withCredentials:true});
  callbacks.onState('connecting');
  const seen=new Set<string>();
  source.onopen=()=>callbacks.onState('connected');
  source.onmessage=event=>{
    try {
      const value=JSON.parse(event.data) as Schema<'Event'>;
      if (value.taskId!==taskId) return;
      const seq=value.seq;
      if (seen.has(seq)) return;
      seen.add(seq);
      if (seen.size>1000) seen.clear();
      callbacks.onEvent(value);
    } catch { callbacks.onGap('事件载荷无法解析，请重新读取任务状态'); }
  };
  source.onerror=()=>{callbacks.onState('disconnected');callbacks.onGap('事件流中断，正在等待连接恢复；状态以服务端为准');};
  return ()=>{source.close();};
}
