import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm,writeFile} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {randomUUID} from 'node:crypto';
import {setTimeout as delay} from 'node:timers/promises';
import {SceneConfigStore} from '../../apps/server/src/scene-config.ts';
import {SceneAccessStore} from '../../apps/server/src/scene-access.ts';
import {ExecutionRegistry,singleServicePermissions} from '../../apps/server/src/execution-registry.ts';
import {LocalAuth} from '../../apps/server/src/auth.ts';
import {createApp} from '../../apps/server/src/app.ts';
import {
  InstitutionTaskStore,institutionDayRange,institutionIntegrationConfig,
} from '../../apps/server/src/institution-integration.ts';
import type {chatConnection} from '../../apps/server/src/modules/chat/connection.ts';

const prefix='/api/v1/integrations/institution/investigations';
type InjectResult={statusCode:number;body:string;json:()=>any};
test('institution API needs no application secret or IP allowlist, and validates day boundaries',()=>{
  const defaults=institutionIntegrationConfig({});
  assert.equal(defaults.clientId,'platform-b');
  assert.ok(defaults.databasePath.endsWith('institution-api.sqlite'));
  const obsolete=institutionIntegrationConfig({
    HARNESS_INSTITUTION_API_KEY:'ignored-legacy-key',
    HARNESS_INSTITUTION_API_ALLOWED_IPS:'0.0.0.0',
  });
  assert.deepEqual(obsolete,defaults,'obsolete key/allowlist settings must not gate API availability');
  assert.throws(()=>institutionIntegrationConfig({
    HARNESS_INSTITUTION_API_CLIENT_ID:' invalid identifier',
  }),/CLIENT_CONFIG/);
  assert.deepEqual(institutionDayRange('2026-11-22'),{
    from:Date.parse('2026-11-22T00:00:00+08:00'),
    to:Date.parse('2026-11-23T00:00:00+08:00'),
  });
  assert.throws(()=>institutionDayRange('2026-02-30'),/INVALID_INSTITUTION_DATE/);
});

test('persistent store deduplicates requests and never automatically retries after restart',async()=>{
  const root=await mkdtemp(join(tmpdir(),'institution-store-'));
  try{
    const db=join(root,'tasks.sqlite');
    const first=new InstitutionTaskStore(db);
    const a=first.create('b','user-hash','req-1','2026-11-22','000123');
    const b=first.create('b','user-hash','req-1','2026-11-22','000123');
    assert.equal(a.created,true);
    assert.equal(b.created,false);
    assert.equal(b.row.id,a.row.id);
    assert.throws(()=>first.create('b','user-hash','req-1','2026-11-22','000124'),/REQUEST_ID_PAYLOAD_CONFLICT/);
    assert.equal(first.take()?.id,a.row.id);
    first.close();
    const restored=new InstitutionTaskStore(db);
    assert.equal(restored.get('b','user-hash',a.row.id)?.status,'UNKNOWN');
    assert.equal(restored.get('b','other',a.row.id),undefined);
    assert.equal(restored.take(),undefined,'a crashed task must not re-execute');
    restored.close();
  }finally{await rm(root,{recursive:true,force:true});}
});

test('B creates and reads only its own institution result; no extra privileged API',async()=>{
  const root=await mkdtemp(join(tmpdir(),'institution-b2b-'));
  const db=join(tmpdir(),'institution-api-'+randomUUID()+'.sqlite');
  const envKeys=[
    'HARNESS_INSTITUTION_API_ALLOWED_IPS','HARNESS_INSTITUTION_API_CLIENT_ID',
    'HARNESS_INSTITUTION_API_DB_PATH',
    'HARNESS_INSTITUTION_API_MODEL_ID',
  ] as const;
  const old=Object.fromEntries(envKeys.map(key=>[key,process.env[key]]));
  let app:Awaited<ReturnType<typeof createApp>>|undefined;
  try{
    process.env.HARNESS_INSTITUTION_API_CLIENT_ID='platform-b';
    delete process.env.HARNESS_INSTITUTION_API_ALLOWED_IPS;
    process.env.HARNESS_INSTITUTION_API_DB_PATH=db;
    delete process.env.HARNESS_INSTITUTION_API_MODEL_ID;
    const settings=new SceneConfigStore(root);
    await settings.initialize();
    const current=settings.read();
    await settings.save({version:1,scenes:{...current.config.scenes,
      'institution-message':{directory:root,executor:{type:'skill',name:'institution-log-query'}}}},current.revision);
    const calls:{prompt?:Record<string,unknown>;created?:Record<string,unknown>;count:number}={count:0};
    const sessions=new Map<string,{id:string;location:{directory:string};permissions:unknown;metadata:unknown}>();
    const results=new Map<string,unknown[]>();
    const native={
      skill:{list:async()=>({location:{directory:root},data:[{name:'institution-log-query',id:'institution-skill'}]})},
      model:{list:async()=>({data:[{id:'kimi',providerID:'private',name:'Kimi',enabled:true}]})},
      session:{
        create:async(input:Record<string,unknown>)=>{
          calls.created=input;
          const session={id:'native-'+randomUUID(),location:input.location as {directory:string},
            permissions:input.permissions,metadata:input.metadata};
          sessions.set(session.id,session);
          return session;
        },
        prompt:async(input:{sessionID:string;text:string;skills:unknown;delivery:string})=>{
          calls.prompt=input;
          calls.count++;
          results.set(input.sessionID,[{
            id:'assistant-answer',type:'assistant',finish:'stop',time:{completed:Date.now()},
            content:[{type:'text',text:'报文 0000123 的最终排查结论'}],
          }]);
          return {id:'accepted'};
        },
        active:async()=>({}),inbox:{list:async()=>[]},
        remove:async()=>({}),
        interrupt:async()=>({interrupted:true}),
      },
      message:{list:async({sessionID}:{sessionID:string})=>({data:results.get(sessionID)??[]})},
      permission:{list:async()=>[]},
    };
    const connection=async()=>native as unknown as Awaited<ReturnType<ReturnType<typeof chatConnection>>>;
    const execution=new ExecutionRegistry([],async()=>connection(),{
      workspace:root,singleServiceIdentity:'single:'+'f'.repeat(64),
    });
    app=await createApp({
      localOnly:true,workspace:root,auth:new LocalAuth('pair',4176),
      nativeConnection:connection,execution,
    });
    const headers={
      host:'10.10.40.50:4176',
      'x-workbench-user-id':'customer-01',
    };
    const submit=(requestId:string,msgSn:string,extra:Record<string,string>={})=>app!.inject({
      method:'POST',url:prefix,remoteAddress:'10.10.20.30',
      headers:{...headers,...extra},
      payload:{date:'2026-11-22',msgSn,requestId},
    });
    // The API accepts direct requests regardless of originating client IP.
    // Source access must be controlled by network infrastructure, not this app.
    assert.equal(Object.hasOwn(headers,'authorization'),false);
    assert.equal(process.env.HARNESS_INSTITUTION_API_ALLOWED_IPS,undefined);
    const created=await submit('r1','0000123');
    assert.equal(created.statusCode,202,created.body);
    const taskId=created.json().data.taskId as string;
    assert.match(taskId,/^[0-9a-f-]{36}$/);
    const anotherIp=await app.inject({url:prefix+'/'+taskId,remoteAddress:'10.10.20.31',headers});
    assert.equal(anotherIp.statusCode,200,anotherIp.body);
    const forwarded=await app.inject({url:prefix+'/'+taskId,remoteAddress:'10.10.20.31',
      headers:{...headers,'x-forwarded-for':'10.10.20.30'}});
    assert.equal(forwarded.statusCode,200,forwarded.body);
    const replay=await submit('r1','0000123');
    assert.equal(replay.statusCode,200,replay.body);
    assert.equal(replay.json().data.taskId,taskId);
    const conflict=await submit('r1','0000124');
    assert.equal(conflict.statusCode,409,conflict.body);
    const anotherUser=await app.inject({url:prefix+'/'+taskId,remoteAddress:'10.10.20.30',
      headers:{...headers,'x-workbench-user-id':'customer-02'}});
    assert.equal(anotherUser.statusCode,404,anotherUser.body);
    let final:{status:string;result:{summary:string}|null}|undefined;
    for(let i=0;i<80;i++){
      const response:InjectResult=
        await app.inject({url:prefix+'/'+taskId,remoteAddress:'10.10.20.30',headers});
      assert.equal(response.statusCode,200,response.body);
      final=response.json().data;
      if(final?.status==='SUCCEEDED')break;
      await delay(20);
    }
    assert.equal(final?.status,'SUCCEEDED');
    assert.equal(final?.result?.summary,'报文 0000123 的最终排查结论');
    assert.equal(calls.count,1,'idempotent replay must not rerun native prompt');
    assert.deepEqual((calls.prompt as {skills:unknown}).skills,[{id:'institution-skill'}]);
    assert.match(String(calls.prompt?.text),/0000123/);
    assert.equal(JSON.stringify(final).includes('native-'),false);
    assert.deepEqual(calls.created?.permissions,singleServicePermissions);
    assert.equal((calls.created?.metadata as Record<string,string>).harnessScenario,'institution-message-inspection');

    // Admin task monitoring is independent of the public keyless B API. A B
    // caller cannot self-assert admin via X-Workbench-User-Id.
    const noSession=await app.inject({url:'/api/v1/institution-tasks',remoteAddress:'127.0.0.1',
      headers:{host:'127.0.0.1:4176','x-workbench-user-id':'zhangzhanhui'}});
    assert.equal(noSession.statusCode,401,noSession.body);
    const adminLogin=await app.inject({method:'POST',url:'/api/v1/auth/local-session',
      remoteAddress:'127.0.0.1',
      headers:{host:'127.0.0.1:4176',origin:'http://127.0.0.1:4176'},
      payload:{name:'zhangzhanhui',phone:'13800000000'}});
    assert.equal(adminLogin.statusCode,200,adminLogin.body);
    const adminCookie=String(adminLogin.headers['set-cookie']).split(';')[0];
    const adminHeaders={host:'127.0.0.1:4176',cookie:adminCookie};
    const remoteAdmin=await app.inject({url:'/api/v1/institution-tasks',
      remoteAddress:'10.10.20.30',headers:{...adminHeaders,host:'10.10.20.30:4176'}});
    assert.equal(remoteAdmin.statusCode,403,remoteAdmin.body);
    const remotePage=await app.inject({url:'/institution-tasks',remoteAddress:'10.10.20.30',
      headers:{host:'10.10.20.30:4176'}});
    assert.equal(remotePage.statusCode,403,remotePage.body);
    const adminList=await app.inject({url:'/api/v1/institution-tasks?status=SUCCEEDED&q=r1&offset=0',
      remoteAddress:'127.0.0.1',headers:adminHeaders});
    assert.equal(adminList.statusCode,200,adminList.body);
    assert.equal(adminList.json().data.total,1);
    assert.equal(adminList.json().data.items[0].taskId,taskId);
    assert.equal(adminList.json().data.items[0].status,'SUCCEEDED');
    assert.equal(adminList.json().data.items[0].hasResult,true);
    assert.equal(JSON.stringify(adminList.json()).includes('最终排查结论'),false,
      'listing must never expose full report content');
    const detail=await app.inject({url:'/api/v1/institution-tasks/'+taskId,
      remoteAddress:'127.0.0.1',headers:adminHeaders});
    assert.equal(detail.statusCode,200,detail.body);
    assert.equal(detail.json().data.result.summary,'报文 0000123 的最终排查结论');
    assert.equal(JSON.stringify(detail.json()).includes('native-'),false);
    const invalidFilter=await app.inject({url:'/api/v1/institution-tasks?status=ANYTHING',
      remoteAddress:'127.0.0.1',headers:adminHeaders});
    assert.equal(invalidFilter.statusCode,400,invalidFilter.body);
    const invalidDetail=await app.inject({url:'/api/v1/institution-tasks/not-a-task',
      remoteAddress:'127.0.0.1',headers:adminHeaders});
    assert.equal(invalidDetail.statusCode,404,invalidDetail.body);
    // Revoking the same configured editor permission takes effect at once.
    await writeFile(join(root,'.workbench','config-access.json'),
      JSON.stringify({version:1,editors:['another-admin']}));
    const revoked=await app.inject({url:'/api/v1/institution-tasks',
      remoteAddress:'127.0.0.1',headers:adminHeaders});
    assert.equal(revoked.statusCode,403,revoked.body);
    const revokedDetail=await app.inject({url:'/api/v1/institution-tasks/'+taskId,
      remoteAddress:'127.0.0.1',headers:adminHeaders});
    assert.equal(revokedDetail.statusCode,403,revokedDetail.body);
    for(const forbidden of ['/config','/analytics','/api/v1/scene-access']){
      const response:InjectResult=
        await app.inject({url:forbidden,remoteAddress:'10.10.20.30',headers});
      assert.equal(response.statusCode,403,forbidden+response.body);
    }
  }finally{
    if(app)await app.close();
    for(const key of envKeys){
      if(old[key]===undefined)delete process.env[key];else process.env[key]=old[key];
    }
    await rm(root,{recursive:true,force:true});
    await rm(db,{force:true});await rm(db+'-wal',{force:true});await rm(db+'-shm',{force:true});
  }
});
