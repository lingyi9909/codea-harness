import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {randomUUID} from 'node:crypto';
import {setTimeout as sleep} from 'node:timers/promises';
import {SceneConfigStore} from '../../apps/server/src/scene-config.ts';
import {ExecutionRegistry} from '../../apps/server/src/execution-registry.ts';
import {LocalAuth} from '../../apps/server/src/auth.ts';
import {createApp} from '../../apps/server/src/app.ts';
import {InstitutionTaskStore} from '../../apps/server/src/institution-integration.ts';
import type {chatConnection} from '../../apps/server/src/modules/chat/connection.ts';

type Response={statusCode:number;body:string;json:()=>any};
const rootRoute='/api/v1/integrations/institution/investigations';
const adminRoot='/api/v1/institution-tasks';
const localhost={host:'127.0.0.1:4176',origin:'http://127.0.0.1:4176'};

async function until(fn:()=>boolean|Promise<boolean>,message:string,limit=7500){
  const end=Date.now()+limit;
  while(Date.now()<end){if(await fn())return;await sleep(35);}
  throw new Error(message);
}

test('bounded concurrency 3/5/10/15, user aggregation, restart persistence and ACL', {timeout:30000},async()=>{
  const root=await mkdtemp(join(tmpdir(),'inst-concurrent-'));
  const db=join(tmpdir(),'inst-concurrent-'+randomUUID()+'.sqlite');
  const saved=process.env.HARNESS_INSTITUTION_API_DB_PATH;
  const previous=process.env.HARNESS_INSTITUTION_API_MODEL_ID;
  let app:Awaited<ReturnType<typeof createApp>>|undefined;
  try{
    process.env.HARNESS_INSTITUTION_API_DB_PATH=db;
    delete process.env.HARNESS_INSTITUTION_API_MODEL_ID;
    const cfg=new SceneConfigStore(root);
    await cfg.initialize();
    const initial=cfg.read();
    await cfg.save({version:1,scenes:{...initial.config.scenes,
      'institution-message':{directory:root,executor:{type:'skill',name:'institution-log-query'}}}},initial.revision);

    const sessions=new Map<string,{id:string;location:{directory:string};permissions:unknown}>();
    const submitted=new Set<string>(),completed=new Set<string>();
    const native={
      skill:{list:async()=>({location:{directory:root},data:[{name:'institution-log-query',id:'skill-1'}]})},
      model:{list:async()=>({data:[{id:'kimi',providerID:'private',name:'Kimi',enabled:true}]})},
      session:{
        create:async(input:{location:{directory:string};permissions:unknown})=>{
          const session={id:'native-'+randomUUID(),location:input.location,permissions:input.permissions};
          sessions.set(session.id,session);return session;
        },
        prompt:async(input:{sessionID:string})=>{submitted.add(input.sessionID);return {id:'accepted'};},
        active:async()=>({}),inbox:{list:async()=>[]},remove:async()=>({}),
        interrupt:async()=>({interrupted:true}),
      },
      message:{list:async({sessionID}:{sessionID:string})=>({data:completed.has(sessionID)?[{
        id:'answer',type:'assistant',finish:'stop',time:{completed:Date.now()},
        content:[{type:'text',text:'完成 '+sessionID}],
      }]:[]})},
      permission:{list:async()=>[]},
    };
    const connection=async()=>native as unknown as Awaited<ReturnType<ReturnType<typeof chatConnection>>>;
    const execution=new ExecutionRegistry([],async()=>connection(),{
      workspace:root,singleServiceIdentity:'single:'+'d'.repeat(64),
    });
    app=await createApp({localOnly:true,workspace:root,auth:new LocalAuth('pair',4176),
      nativeConnection:connection,execution});
    const login=await app.inject({method:'POST',url:'/api/v1/auth/local-session',
      remoteAddress:'127.0.0.1',headers:localhost,
      payload:{name:'zhangzhanhui',phone:'13800000000'}});
    assert.equal(login.statusCode,200,login.body);
    const headers={...localhost,cookie:String(login.headers['set-cookie']).split(';')[0]};
    const writeHeaders={...headers,'x-csrf-token':login.json().data.csrfToken};
    const get=async(url:string)=>app!.inject({url,headers,remoteAddress:'127.0.0.1'});
    const change=async(n:number,extra:Record<string,string>={})=>app!.inject({
      url:'/api/v1/institution-concurrency',method:'PATCH',remoteAddress:'127.0.0.1',
      headers:{...writeHeaders,...extra},payload:{maxConcurrency:n},
    });
    const submit=async(i:number,user:string)=>{
      const result=await app!.inject({url:rootRoute,method:'POST',remoteAddress:'10.20.10.11',
        headers:{host:'10.20.40.50:4176','x-workbench-user-id':user},
        payload:{date:'2026-11-23',msgSn:'MSG-'+i,requestId:'request-'+i}});
      assert.equal(result.statusCode,202,result.body);
      return result.json().data.taskId as string;
    };
    const status=async()=>{
      const response=await get('/api/v1/institution-concurrency');
      assert.equal(response.statusCode,200,response.body);
      return response.json().data as {maxConcurrency:number;running:number;queued:number};
    };
    assert.deepEqual(await status(),{maxConcurrency:3,running:0,queued:0});
    const requests=await Promise.all([submit(1,'customer-001'),submit(2,'customer-002'),
      submit(3,'customer-001'),submit(4,'customer-002'),submit(5,'customer-001'),
      submit(6,'customer-001')]);
    assert.equal(new Set(requests).size,6);
    await until(async()=>{const x=await status();return x.running===3&&x.queued===3;},'3 parallel tasks not started');
    await until(()=>submitted.size===3,'3 native sessions were not submitted');
    const users=await get(adminRoot+'/users');
    assert.equal(users.statusCode,200,users.body);
    const entries=users.json().data.items;
    const customer1=entries.find((x:{externalUserId:string})=>x.externalUserId==='customer-001');
    assert.equal(customer1.total,4);
    assert.equal(customer1.running+customer1.queued,4);
    const details=await get(adminRoot+'?userHash='+customer1.userHash);
    assert.equal(details.statusCode,200,details.body);
    assert.equal(details.json().data.total,4);
    const restricted=await app.inject({url:adminRoot+'/users',remoteAddress:'10.20.10.11',
      headers:{host:'10.20.40.50:4176','x-workbench-user-id':'customer-001'}});
    assert.equal(restricted.statusCode,403);
    const badCsrf=await change(5,{'x-csrf-token':'wrong'});
    assert.equal(badCsrf.statusCode,403,badCsrf.body);
    assert.equal((await status()).maxConcurrency,3);
    const invalid=await change(4);
    assert.equal(invalid.statusCode,400,invalid.body);
    const raise=await change(5);
    assert.equal(raise.statusCode,200,raise.body);
    await until(async()=>{const x=await status();return x.running===5&&x.queued===1;},'5 parallel tasks not admitted');
    await until(()=>submitted.size===5,'5 native sessions were not submitted');
    const lower=await change(3);
    assert.equal(lower.statusCode,200,lower.body);
    assert.equal((await status()).running,5,'lowering limit must not cancel running work');
    // Releasing two tasks leaves three in flight, so the last queued task waits.
    for(const id of [...submitted].slice(0,2))completed.add(id);
    await until(async()=>{const x=await status();return x.running===3&&x.queued===1;},'two tasks did not finish');
    assert.equal(submitted.size,5,'lowered limit must not start another task');
    completed.add([...submitted][2]);
    await until(()=>submitted.size===6,'queued task not admitted when capacity is freed');
    for(const id of submitted)completed.add(id);
    await until(async()=>{const x=await status();return x.running===0&&x.queued===0;},
      'jobs did not complete after release');
    const final=await get(adminRoot+'/users?q=customer-001');
    assert.equal(final.statusCode,200,final.body);
    assert.equal(final.json().data.items[0].succeeded,4);
    // All allowed settings persist in the same SQLite file.
    for(const n of [10,15,5,3]){
      assert.equal((await change(n)).statusCode,200);
      assert.equal((await status()).maxConcurrency,n);
    }
    assert.equal((await change(15)).statusCode,200);
    await app.close();app=undefined;
    const restored=new InstitutionTaskStore(db);
    assert.equal(restored.concurrency(),15);
    assert.equal(restored.groupsAdmin('customer-001',0).items[0].succeeded,4);
    restored.close();
  }finally{
    if(app)await app.close();
    if(saved===undefined)delete process.env.HARNESS_INSTITUTION_API_DB_PATH;
    else process.env.HARNESS_INSTITUTION_API_DB_PATH=saved;
    if(previous===undefined)delete process.env.HARNESS_INSTITUTION_API_MODEL_ID;
    else process.env.HARNESS_INSTITUTION_API_MODEL_ID=previous;
    await rm(root,{recursive:true,force:true});
    await rm(db,{force:true});await rm(db+'-wal',{force:true});await rm(db+'-shm',{force:true});
  }
});
