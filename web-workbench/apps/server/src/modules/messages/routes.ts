import { randomUUID } from 'node:crypto';
import type { FastifyInstance } from 'fastify';
import type { Pool } from 'mysql2/promise';
import type { LocalAuth } from '../../auth.ts';
import { createContinuation, listTaskMessages } from './service.ts';

export function registerMessageRoutes(app:FastifyInstance,opts:{
  pool:Pool;auth:LocalAuth;cursorKey:string;
}):void{
  const wrap=<T>(data:T)=>({data,requestId:randomUUID()});
  app.get<{Params:{taskId:string};Querystring:{limit?:string;cursor?:string}}>(
    '/api/v1/tasks/:taskId/messages',async(req,reply)=>{
      const {ownerId}=opts.auth.session(req);
      return reply.send(wrap(await listTaskMessages({pool:opts.pool,ownerId,taskId:req.params.taskId,
        limit:Number(req.query.limit??20),cursor:req.query.cursor,cursorKey:opts.cursorKey})));
    });
  app.post<{Params:{taskId:string};Body:{
    operationId:string;text:string;quote?:string;reportArtifactId?:string;
  }}>('/api/v1/tasks/:taskId/messages',async(req,reply)=>{
    const {ownerId}=opts.auth.mutation(req);
    const body=req.body;
    const op=await createContinuation({ownerId,taskId:req.params.taskId,
      operationId:body?.operationId,kind:'DISCUSS',text:body?.text,quote:body?.quote,
      reportArtifactId:body?.reportArtifactId},{pool:opts.pool});
    return reply.code(202).send(wrap(op));
  });
  app.post<{Params:{taskId:string};Body:{operationId:string}}>(
    '/api/v1/tasks/:taskId/rechecks',async(req,reply)=>{
      const {ownerId}=opts.auth.mutation(req);
      const op=await createContinuation({ownerId,taskId:req.params.taskId,
        operationId:req.body?.operationId,kind:'RECHECK'},{pool:opts.pool});
      return reply.code(202).send(wrap(op));
    });
}
