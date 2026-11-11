import { randomUUID } from 'node:crypto';
import type { FastifyInstance } from 'fastify';
import type { Pool } from 'mysql2/promise';
import type { LocalAuth } from '../../auth.ts';
import { readOwnedArtifact } from './service.ts';

export function registerArtifactRoutes(app:FastifyInstance,opts:{
  pool:Pool;auth:LocalAuth;dataRoot:string;
}):void{
  app.get<{Params:{artifactId:string}}>('/api/v1/artifacts/:artifactId',async(req,reply)=>{
    const {ownerId}=opts.auth.session(req);
    const data=await readOwnedArtifact({pool:opts.pool,ownerId,
      artifactId:req.params.artifactId,dataRoot:opts.dataRoot});
    reply.header('Cache-Control','no-store');
    return reply.send({data:data.artifact,requestId:randomUUID()});
  });
  app.get<{Params:{artifactId:string}}>('/api/v1/artifacts/:artifactId/download',async(req,reply)=>{
    const {ownerId}=opts.auth.session(req);
    const {raw,artifact}=await readOwnedArtifact({pool:opts.pool,ownerId,
      artifactId:req.params.artifactId,dataRoot:opts.dataRoot});
    reply.header('Content-Type','text/markdown; charset=utf-8');
    reply.header('Content-Disposition','attachment; filename="codea-review-'+artifact.meta.id+'.md"');
    reply.header('Cache-Control','no-store');
    reply.header('X-Content-Type-Options','nosniff');
    return reply.send(raw);
  });
}
