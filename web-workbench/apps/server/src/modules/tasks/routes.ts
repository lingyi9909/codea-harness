import { randomUUID } from 'node:crypto';
import type { FastifyInstance } from 'fastify';
import type { Pool } from 'mysql2/promise';
import type { LocalAuth } from '../../auth.ts';
import { AuthProblem } from '../../auth.ts';
import { createTask, getOperation, getTask, listTasks, listRuns } from './service.ts';

export function registerTaskRoutes(app:FastifyInstance,opts:{
  pool:Pool;auth:LocalAuth;roots:string[];cursorKey:string;notesSupported:boolean;
}):void{
  const wrap=<T>(data:T)=>({data,requestId:randomUUID()});
  app.post<{Body:{operationId:string;previewId:string;title?:string;
    controller?:string;notes?:string}}>('/api/v1/tasks',async(req,reply)=>{
    const {ownerId}=opts.auth.mutation(req);
    if(!req.body||typeof req.body!=='object')throw new AuthProblem(400,'INVALID_REQUEST');
    const result=await createTask({...req.body,ownerId},opts);
    return reply.status(202).send(wrap(result));
  });
  app.get<{Params:{operationId:string}}>('/api/v1/operations/:operationId',async(req,reply)=>{
    const {ownerId}=opts.auth.session(req);
    return reply.send(wrap(await getOperation(opts.pool,ownerId,req.params.operationId)));
  });
  app.get<{Querystring:{limit?:string;cursor?:string;repositoryId?:string;status?:string}}>(
    '/api/v1/tasks',async(req,reply)=>{
      const {ownerId}=opts.auth.session(req);
      const limit=Number(req.query.limit??'20');
      return reply.send(wrap(await listTasks({...req.query,ownerId,limit},opts)));
    });
  app.get<{Params:{taskId:string}}>('/api/v1/tasks/:taskId',async(req,reply)=>{
    const {ownerId}=opts.auth.session(req);
    return reply.send(wrap(await getTask(opts.pool,ownerId,req.params.taskId)));
  });
  app.get<{Params:{taskId:string};Querystring:{limit?:string;cursor?:string}}>(
    '/api/v1/tasks/:taskId/runs',async(req,reply)=>{
      const {ownerId}=opts.auth.session(req);
      if(req.query.cursor)throw new AuthProblem(400,'CURSOR_NOT_IMPLEMENTED');
      const limit=Number(req.query.limit??'20');
      return reply.send(wrap(await listRuns(opts.pool,ownerId,req.params.taskId,limit)));
    });
}
