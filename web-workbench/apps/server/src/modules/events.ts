import type { FastifyInstance } from 'fastify';
import { query, type DbRow } from '../../../../packages/storage/src/db.ts';
import { getOwnedTask, iso, parsed } from './tasks.ts';
import { paramRequired } from './ops.ts';
import { httpFail } from '../auth.ts';

const DECIMAL=/^(0|[1-9][0-9]*)$/;
export function registerEvents(app:FastifyInstance,getOwner:()=>Promise<string>):void{
  app.get<{Params:{taskId:string};Querystring:{after?:string}}>('/api/v1/tasks/:taskId/events',async(req,reply)=>{
    const taskId=paramRequired(req.params.taskId);
    const detail=await getOwnedTask(await getOwner(),taskId);
    const initial=req.query.after??req.headers['last-event-id']??'0';
    const after=typeof initial==='string'?initial:'';
    if(!DECIMAL.test(after)||BigInt(after)>BigInt(detail.lastSeq))httpFail(409,'EVENT_CURSOR_INVALID');
    reply.hijack();
    const response=reply.raw;
    response.writeHead(200,{'Content-Type':'text/event-stream; charset=utf-8',
      'Cache-Control':'no-cache, no-store, must-revalidate','X-Accel-Buffering':'no','Connection':'keep-alive'});
    response.write('retry: 3000\n\n');
    let closed=false,last=BigInt(after),pending=false;
    const send=async()=>{
      if(closed||pending)return;
      pending=true;
      try{
        const rows=await query<DbRow>(
          'SELECT CAST(seq AS CHAR) AS seq,run_id,event_type,payload,created_at FROM wh_events '+
          'WHERE task_id=? AND seq>? ORDER BY seq ASC LIMIT 100',[taskId,last.toString()]);
        for(const row of rows){
          if(closed)break;
          const seq=String(row.seq);
          if(!DECIMAL.test(seq)||BigInt(seq)<=last)continue;
          const event={taskId,runId:row.run_id===null?null:String(row.run_id),seq,
            type:String(row.event_type),occurredAt:iso(row.created_at),data:parsed<unknown>(row.payload)};
          response.write('id: '+seq+'\ndata: '+JSON.stringify(event)+'\n\n');
          last=BigInt(seq);
        }
      }catch{
        // Never fabricate events or successful delivery when MySQL is unavailable.
        closed=true;response.end();
      }finally{pending=false}
    };
    void send();
    const poll=setInterval(()=>{void send()},1500);
    const heartbeat=setInterval(()=>{if(!closed)response.write(': heartbeat\n\n')},12000);
    const shutdown=()=>{closed=true;clearInterval(poll);clearInterval(heartbeat)};
    req.raw.once('close',shutdown);
    response.once('close',shutdown);
    return reply;
  });
}
