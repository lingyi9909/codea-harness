import {AsyncLocalStorage} from 'node:async_hooks';
import type {FastifyInstance} from 'fastify';
import type {LocalAuth} from '../../auth.ts';
import type {ChatAccessEntry} from '../../../../../packages/contracts/src/chat.ts';

export function registerAccess(app:FastifyInstance,auth:LocalAuth){
  const entries:ChatAccessEntry[]=[];
  const context=new AsyncLocalStorage<string>();
  const path='/api/v1/chat/access-log';
  const add=(source:ChatAccessEntry['source'],operation:string,result:string,ms:number,requestId=context.getStore()??'-')=>{
    entries.push({time:new Date().toISOString(),requestId,source,operation,result,ms:Math.round(ms)});
    if(entries.length>300)entries.shift();
  };
  app.addHook('onRequest',(req,_reply,done)=>context.run(req.id,done));
  app.addHook('onResponse',async(req,reply)=>{
    const route=req.routeOptions.url;
    if(route?.startsWith('/api/v1/')&&route!==path)add('工作台',req.method+' '+route,String(reply.statusCode),reply.elapsedTime,req.id);
  });
  app.get(path,async(req,reply)=>{
    auth.session(req);
    reply.header('cache-control','no-store');
    return {data:entries.slice().reverse(),requestId:req.id};
  });
  return add;
}
