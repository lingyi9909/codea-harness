import type { FastifyInstance } from 'fastify';
import { randomUUID } from 'node:crypto';
import { AuthProblem } from '../../auth.ts';
import type { Pool } from 'mysql2/promise';
import type { LocalAuth } from '../../auth.ts';
import { GitFailure, getRefs, trustedRepository } from '../../../../../packages/git-provider/src/repository.ts';
import { checkUuid, findRepository, listRepositories, registerRepository } from './service.ts';
import { createPreview } from './preview-service.ts';

type Deps={pool:Pool;auth:LocalAuth;roots:string[];refKey:string};
const wrap=<T>(data:T)=>({data,requestId:randomUUID()});
export function registerGitRoutes(app:FastifyInstance,deps:Deps):void{
  app.get<{Querystring:{limit?:string;cursor?:string}}>('/api/v1/repositories',async(req,reply)=>{
    const {ownerId}=deps.auth.session(req);
    if(req.query.cursor)throw new AuthProblem(400,'CURSOR_NOT_SUPPORTED');
    const limit=Number(req.query.limit??20);
    if(!Number.isInteger(limit)||limit<1||limit>100)throw new AuthProblem(400,'LIMIT_INVALID');
    const repos=await listRepositories(deps.pool,ownerId,limit);
    return reply.send(wrap({items:repos,nextCursor:null}));
  });
  app.post<{Body:{operationId:string;name:string;path:string}}>('/api/v1/repositories',async(req,reply)=>{
    const {ownerId}=deps.auth.mutation(req);
    if(!req.body||typeof req.body!=='object')throw new AuthProblem(400,'INVALID_REQUEST');
    const repo=await registerRepository({...req.body,ownerId},deps);
    return reply.status(201).send(wrap(repo));
  });
  app.get<{Params:{repositoryId:string}}>('/api/v1/repositories/:repositoryId/refs',async(req,reply)=>{
    const {ownerId}=deps.auth.session(req);
    const repo=await findRepository(deps.pool,ownerId,checkUuid(req.params.repositoryId));
    if(repo.status!=='READY')throw new AuthProblem(409,'REPOSITORY_UNAVAILABLE');
    if(deps.refKey.length<32)throw new AuthProblem(503,'REF_SIGNING_KEY_REQUIRED');
    try {
      const root=await trustedRepository(repo.path,deps.roots);
      const refs=await getRefs(root,repo.id,deps.refKey);
      return reply.send(wrap(refs));
    }catch(e){
      if(e instanceof GitFailure)throw new AuthProblem(422,e.code);
      throw e;
    }
  });
  app.post<{Body:{operationId:string;repositoryId:string;baseRefId:string}}>(
    '/api/v1/review-previews',async(req,reply)=>{
      const {ownerId}=deps.auth.mutation(req);
      if(!req.body||typeof req.body!=='object')throw new AuthProblem(400,'INVALID_REQUEST');
      const result=await createPreview({...req.body,ownerId},deps);
      return reply.status(201).send(wrap(result));
    });

}
