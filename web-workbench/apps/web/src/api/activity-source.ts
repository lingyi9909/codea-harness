export interface ChatEventSource {
  onmessage:((event:MessageEvent<string>)=>void)|null;
  onerror:(()=>void)|null;
  addEventListener(type:string,listener:(event:MessageEvent<string>)=>void):void;
  close():void;
}
// EventSource cannot attach Authorization. Embedded pages use fetch streaming;
// the capability is never placed in an SSE URL, cookie or browser storage.
export class BearerEventSource implements ChatEventSource {
  onmessage:ChatEventSource['onmessage']=null;
  onerror:ChatEventSource['onerror']=null;
  private readonly listeners=new Map<string,((event:MessageEvent<string>)=>void)[]>();
  private readonly stop=new AbortController();
  private retry?:ReturnType<typeof setTimeout>;
  private readonly url:string;
  private readonly headers:Record<string,string>;
  constructor(url:string,headers:Record<string,string>){this.url=url;this.headers=headers;void this.connect();}
  addEventListener(type:string,listener:(event:MessageEvent<string>)=>void){const values=this.listeners.get(type)??[];values.push(listener);this.listeners.set(type,values);}
  private async connect(){
    try{
      const response=await fetch(this.url,{headers:this.headers,credentials:'omit',cache:'no-store',redirect:'error',signal:this.stop.signal});
      if(response.status>=500){await response.body?.cancel();throw new Error('SSE_TEMPORARILY_UNAVAILABLE');}
      if(!response.ok||!response.headers.get('content-type')?.includes('text/event-stream')||!response.body){await response.body?.cancel();this.onerror?.();return;}
      const reader=response.body.getReader(),decoder=new TextDecoder();let buffer='';
      try{
        while(!this.stop.signal.aborted){
          const {done,value}=await reader.read();if(done)break;
          buffer=(buffer+decoder.decode(value,{stream:true})).replace(/\r\n/g,'\n');
          if(buffer.length>1024*1024)throw new Error('SSE_FRAME_TOO_LARGE');
          let end:number;
          while((end=buffer.indexOf('\n\n'))>=0){
            const frame=buffer.slice(0,end);buffer=buffer.slice(end+2);
            let type='message';const data:string[]=[];
            for(const line of frame.split('\n')){if(line.startsWith('event:'))type=line.slice(6).trim();else if(line.startsWith('data:'))data.push(line.slice(5).replace(/^ /,''));}
            if(!data.length)continue;
            const event=new MessageEvent<string>(type,{data:data.join('\n')});
            if(type==='message')this.onmessage?.(event);
            for(const listener of this.listeners.get(type)??[])listener(event);
          }
        }
      }finally{await reader.cancel().catch(()=>{});reader.releaseLock();}
    }catch{/* The regular message snapshot remains the recovery path. */}
    if(!this.stop.signal.aborted){this.onerror?.();this.retry=setTimeout(()=>void this.connect(),2000);}
  }
  close(){this.stop.abort();if(this.retry)clearTimeout(this.retry);this.listeners.clear();}
}
