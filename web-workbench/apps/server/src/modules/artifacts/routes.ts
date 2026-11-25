/**
 * 报告产物 HTTP 路由
 *
 * 职责：对外提供已生成报告与附件的查询或下载入口。
 * Java 研发阅读提示：访问前需要区分报告归属和允许读取的文件。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
import { randomUUID } from 'node:crypto';
import type { FastifyInstance } from 'fastify';
import type { Pool } from 'mysql2/promise';
import type { RequestAuth } from '../../auth.ts';
import { readOwnedArtifact } from './service.ts';

export function registerArtifactRoutes(app:FastifyInstance,opts:{
  pool:Pool;auth:RequestAuth;dataRoot:string;
}):void{
  app.get<{Params:{artifactId:string}}>('/api/v1/artifacts/:artifactId',async(req,reply)=>{
    const {ownerId}=await opts.auth.session(req);
    const data=await readOwnedArtifact({pool:opts.pool,ownerId,
      artifactId:req.params.artifactId,dataRoot:opts.dataRoot});
    reply.header('Cache-Control','no-store');
    return reply.send({data:data.artifact,requestId:randomUUID()});
  });
  app.get<{Params:{artifactId:string}}>('/api/v1/artifacts/:artifactId/download',async(req,reply)=>{
    const {ownerId}=await opts.auth.session(req);
    const {raw,artifact}=await readOwnedArtifact({pool:opts.pool,ownerId,
      artifactId:req.params.artifactId,dataRoot:opts.dataRoot});
    reply.header('Content-Type','text/markdown; charset=utf-8');
    reply.header('Content-Disposition','attachment; filename="codea-review-'+artifact.meta.id+'.md"');
    reply.header('Cache-Control','no-store');
    reply.header('X-Content-Type-Options','nosniff');
    return reply.send(raw);
  });
}
