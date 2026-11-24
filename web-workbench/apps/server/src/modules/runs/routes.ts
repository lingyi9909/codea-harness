/**
 * 执行记录 Controller
 *
 * 职责：向前端暴露运行状态、详情和控制入口。
 * Java 研发阅读提示：区分持久化 Run 与实时 OpenCode Session 的不同状态。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
import type { FastifyInstance } from 'fastify';
import type { Pool } from 'mysql2/promise';
import { randomUUID } from 'node:crypto';
import type { RequestAuth } from '../../auth.ts';
import { retryRun } from './retry.ts';
import type { OpenCodeAdapter } from '../../../../../packages/contracts/src/ports.ts';
import { requestStop } from './stop.ts';
export function registerRunStopRoute(app:FastifyInstance,opts:{pool:Pool;auth:RequestAuth;adapter?:OpenCodeAdapter}):void{
  app.post<{Params:{runId:string};Body:{operationId:string}}>('/api/v1/runs/:runId/retries',async(req,reply)=>{
    const {ownerId}=await opts.auth.mutation(req);
    const op=await retryRun(opts.pool,ownerId,req.params.runId,req.body?.operationId,opts.adapter);
    return reply.code(202).send({data:op,requestId:randomUUID()});
  });
  app.post<{Params:{runId:string};Body:{operationId:string}}>('/api/v1/runs/:runId/stop',
    async(req,reply)=>{
      const session=await opts.auth.mutation(req);
      const op=await requestStop(opts.pool,{ownerId:session.ownerId,
        runId:req.params.runId,operationId:req.body?.operationId});
      return reply.code(202).send({data:op,requestId:randomUUID()});
    });
}
