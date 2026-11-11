import { randomUUID } from 'node:crypto';
import type { FastifyInstance } from 'fastify';
import type { Pool } from 'mysql2/promise';
import type { LocalAuth } from '../../auth.ts';
import { answerInteraction, listPendingInteractions } from './service.ts';

export function registerInteractionRoutes(app:FastifyInstance,opts:{pool:Pool;auth:LocalAuth}):void{
  const wrap=<T>(data:T)=>({data,requestId:randomUUID()});
  app.get<{Params:{taskId:string}}>('/api/v1/tasks/:taskId/interactions',async(req,reply)=>{
    const {ownerId}=opts.auth.session(req);
    return reply.send(wrap({items:await listPendingInteractions(opts.pool,ownerId,req.params.taskId)}));
  });
  app.post<{Params:{interactionId:string};Body:{operationId:string;revision:number;answer:unknown}}>(
    '/api/v1/interactions/:interactionId/answers',async(req,reply)=>{
      const {ownerId}=opts.auth.mutation(req);
      const body=req.body;
      const result=await answerInteraction({pool:opts.pool,ownerId,
        interactionId:req.params.interactionId,operationId:body?.operationId,
        revision:body?.revision,answer:body?.answer});
      return reply.code(202).send(wrap(result));
    });
}
