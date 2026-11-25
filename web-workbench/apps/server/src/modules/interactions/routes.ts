/**
 * 工具交互 Controller
 *
 * 职责：接收权限确认、补参等原生工具交互请求。
 * Java 研发阅读提示：客户端可操作不代表服务端已授权，必须校验会话归属。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
import { randomUUID } from 'node:crypto';
import type { FastifyInstance } from 'fastify';
import type { Pool } from 'mysql2/promise';
import type { RequestAuth } from '../../auth.ts';
import { answerInteraction, listPendingInteractions } from './service.ts';

export function registerInteractionRoutes(app:FastifyInstance,opts:{pool:Pool;auth:RequestAuth}):void{
  const wrap=<T>(data:T)=>({data,requestId:randomUUID()});
  app.get<{Params:{taskId:string}}>('/api/v1/tasks/:taskId/interactions',async(req,reply)=>{
    const {ownerId}=await opts.auth.session(req);
    return reply.send(wrap({items:await listPendingInteractions(opts.pool,ownerId,req.params.taskId)}));
  });
  app.post<{Params:{interactionId:string};Body:{operationId:string;revision:number;answer:unknown}}>(
    '/api/v1/interactions/:interactionId/answers',async(req,reply)=>{
      const {ownerId}=await opts.auth.mutation(req);
      const body=req.body;
      const result=await answerInteraction({pool:opts.pool,ownerId,
        interactionId:req.params.interactionId,operationId:body?.operationId,
        revision:body?.revision,answer:body?.answer});
      return reply.code(202).send(wrap(result));
    });
}
