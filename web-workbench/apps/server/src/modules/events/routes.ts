import type { FastifyInstance } from 'fastify';
import type { Pool } from 'mysql2/promise';
import type { LocalAuth } from '../../auth.ts';
import { AuthProblem } from '../../auth.ts';
import { checkUuid } from '../repositories/service.ts';
import { EventError, parseEventSeq, readTaskEvents, taskLastSeq } from '../../../../../packages/storage/src/events.ts';
import { setTimeout as delay } from 'node:timers/promises';

export function registerEventRoutes(app:FastifyInstance,opts:{pool:Pool;auth:LocalAuth}):void{
  const connections=new Map<string,number>();
  app.get<{Params:{taskId:string};Querystring:{after?:string}}>(
    '/api/v1/tasks/:taskId/events',async(req,reply)=>{
      const {ownerId}=opts.auth.session(req);
      const taskId=checkUuid(req.params.taskId);
      if((connections.get(ownerId)??0)>=5)throw new AuthProblem(429,'SSE_CONNECTION_LIMIT');
      const requestCursor=req.headers['last-event-id']??req.query.after??'0';
      if(typeof requestCursor!=='string')throw new AuthProblem(400,'EVENT_CURSOR_INVALID');
      let after:string,last:string;
      try {
        after=parseEventSeq(requestCursor);
        last=await taskLastSeq(opts.pool,ownerId,taskId);
      }catch(e){
        if(e instanceof EventError)throw new AuthProblem(
          e.code==='TASK_NOT_FOUND'?404:400,e.code);
        throw e;
      }
      if(BigInt(after)>BigInt(last))throw new AuthProblem(409,'EVENT_CURSOR_INVALID');
      connections.set(ownerId,(connections.get(ownerId)??0)+1);
      reply.hijack();
      reply.raw.writeHead(200,{
        'Content-Type':'text/event-stream; charset=utf-8',
        'Cache-Control':'no-cache, no-transform',
        Connection:'keep-alive','X-Content-Type-Options':'nosniff',
        'X-Accel-Buffering':'no',
      });
      let ended=false;
      reply.raw.on('close',()=>{ended=true;});
      const pump=async()=>{
        let cursor=after,lastHeartbeat=Date.now();
        try{
          while(!ended&&!reply.raw.destroyed){
            const events=await readTaskEvents(opts.pool,taskId,cursor,500);
            for(const event of events){
              if(ended)break;
              // Stable SSE id is the persisted business seq, not a native event cursor.
              const frame='id: '+event.seq+'\nevent: '+event.type+'\ndata: '+
                JSON.stringify(event)+'\n\n';
              if(!reply.raw.write(frame)){ended=true;break;}
              cursor=event.seq;
            }
            const now=Date.now();
            if(now-lastHeartbeat>=15000&&!ended){
              if(!reply.raw.write(': heartbeat\n\n')){ended=true;break;}
              lastHeartbeat=now;
            }
            if(!ended)await delay(events.length===500?0:2000);
          }
        }catch{
          // Do not invent events or advance seq after a DB disconnect.
          ended=true;
        }finally{
          connections.set(ownerId,Math.max(0,(connections.get(ownerId)??1)-1));
          if(!reply.raw.destroyed)reply.raw.end();
        }
      };
      void pump();
      return reply;
    });
}
