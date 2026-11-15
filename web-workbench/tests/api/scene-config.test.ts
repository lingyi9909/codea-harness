import {it} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,mkdir,readFile,writeFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {createApp} from '../../apps/server/src/app.ts';
import {LocalAuth} from '../../apps/server/src/auth.ts';
import {profileKey} from '../../apps/server/src/local-profile.ts';

it('scene config is file-backed, editor-ACL guarded, and revision-safe',async t=>{
  const project=await mkdtemp(join(tmpdir(),'workbench-scenes-'));
  const outside=await mkdtemp(join(tmpdir(),'scene-business-'));
  t.after(()=>Promise.all([rm(project,{recursive:true,force:true}),rm(outside,{recursive:true,force:true})]));
  const port=4176,host='127.0.0.1:'+port,origin='http://'+host;
  const app=await createApp({localOnly:true,workspace:project,auth:new LocalAuth('unused',port)});
  t.after(()=>app.close());
  const profile={name:'业务开发',phone:'13800000000'};
  const login=await app.inject({method:'POST',url:'/api/v1/auth/local-session',headers:{host,origin},payload:profile});
  assert.equal(login.statusCode,200);
  const headers={host,origin,cookie:String(login.headers['set-cookie']).split(';')[0],
    'x-csrf-token':login.json().data.csrfToken};
  const read=()=>app.inject({method:'GET',url:'/api/v1/scene-config',headers});
  const write=(config:object,revision:string)=>app.inject({method:'PUT',url:'/api/v1/scene-config',headers,
    payload:{config,revision}});
  const before=await read();
  assert.equal(before.statusCode,200);
  assert.deepEqual(before.json().data.config,{version:1,scenes:{}});
  assert.equal(before.json().data.canEdit,false);
  assert.equal(before.json().data.identity,profileKey(profile));
  assert.equal((await write({version:1,scenes:{}},before.json().data.revision)).statusCode,403);
  const folder=join(project,'.workbench');
  await mkdir(folder,{recursive:true});
  await writeFile(join(folder,'config-access.json'),JSON.stringify({version:1,editors:[profileKey(profile)]}));
  assert.equal((await read()).json().data.canEdit,true);
  const config={version:1,scenes:{
    logs:{directory:outside,executor:{type:'skill',name:'log-query'}},
    message:{directory:project,executor:{type:'subagent',name:'message-helper'}},
    data:{directory:outside,executor:{type:'command',name:'public-analysis'}},
  }};
  const saved=await write(config,before.json().data.revision);
  assert.equal(saved.statusCode,200,JSON.stringify(saved.json()));
  assert.deepEqual(JSON.parse(await readFile(join(folder,'scenarios.json'),'utf8')).scenes,config.scenes);
  assert.equal(saved.json().data.config.scenes.logs.directory,outside);
  const stale=await write(config,before.json().data.revision);
  assert.equal(stale.statusCode,409);
  assert.equal(stale.json().error.code,'SCENE_CONFIG_STALE');
  const bad=await write({version:1,scenes:{data:{directory:join(outside,'missing'),
    executor:{type:'skill',name:'data-analysis'}}}},saved.json().data.revision);
  assert.equal(bad.statusCode,400);
  assert.equal(bad.json().error.code,'SCENE_DIRECTORY_INVALID');
  assert.equal((await app.inject({method:'PUT',url:'/api/v1/scene-config',
    headers:{host,cookie:headers.cookie},payload:{config,revision:saved.json().data.revision}})).statusCode,403);
  assert.equal((await read()).json().data.canEdit,true);
});
