import {it} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,mkdir,readFile,writeFile,rm,realpath} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {createApp} from '../../apps/server/src/app.ts';
import {LocalAuth} from '../../apps/server/src/auth.ts';
import {defaultSceneConfiguration} from '../../apps/server/src/scene-config.ts';

it('scene config is file-backed, editor-ACL guarded, and revision-safe',async t=>{
  const project=await mkdtemp(join(tmpdir(),'workbench-scenes-'));
  const outside=await mkdtemp(join(tmpdir(),'scene-business-'));
  t.after(()=>Promise.all([rm(project,{recursive:true,force:true}),rm(outside,{recursive:true,force:true})]));
  const port=4176,host='127.0.0.1:'+port,origin='http://'+host;
  const app=await createApp({localOnly:true,workspace:project,auth:new LocalAuth('unused',port)});
  t.after(()=>app.close());
  const configDir=join(project,'.workbench');
  // Startup creates both empty files without granting editing privileges.
  assert.deepEqual(JSON.parse(await readFile(join(configDir,'scenarios.json'),'utf8')),
    defaultSceneConfiguration(await realpath(project)));
  assert.deepEqual(JSON.parse(await readFile(join(configDir,'config-access.json'),'utf8')),{version:1,editors:['zhangzhanhui']});
  await assert.rejects(readFile(join(configDir,'user.json'),'utf8'),{code:'ENOENT'});
  const profile={name:'BusinessDev',phone:'13800000000'};
  const login=await app.inject({method:'POST',url:'/api/v1/auth/local-session',headers:{host,origin},payload:profile});
  assert.equal(login.statusCode,200);
  assert.deepEqual(JSON.parse(await readFile(join(configDir,'user.json'),'utf8')),profile);
  const headers={host,origin,cookie:String(login.headers['set-cookie']).split(';')[0],
    'x-csrf-token':login.json().data.csrfToken};
  const read=()=>app.inject({method:'GET',url:'/api/v1/scene-config',headers});
  const write=(config:object,revision:string)=>app.inject({method:'PUT',url:'/api/v1/scene-config',headers,
    payload:{config,revision}});
  const before=await read();
  assert.equal(before.statusCode,200);
  assert.deepEqual(before.json().data.config,defaultSceneConfiguration(await realpath(project)));
  assert.equal(before.json().data.configPath,join(await realpath(project),'.workbench','scenarios.json'));
  assert.equal(before.json().data.accessPath,join(await realpath(project),'.workbench','config-access.json'));
  assert.equal(before.json().data.canEdit,false);
  assert.equal(before.json().data.username,'BusinessDev');
  assert.equal((await write({version:1,scenes:{}},before.json().data.revision)).statusCode,403);
  const folder=join(project,'.workbench');
  await mkdir(folder,{recursive:true});
  await writeFile(join(folder,'config-access.json'),JSON.stringify({version:1,editors:['BusinessDev']}));
  assert.equal((await read()).json().data.canEdit,true);
  const realProject=await realpath(project),realOutside=await realpath(outside);
  const config={version:1,scenes:{
    logs:{directory:realOutside,executor:{type:'skill',name:'log-query'}},
    message:{directory:realProject,executor:{type:'subagent',name:'message-helper'}},
    data:{directory:realOutside,executor:{type:'command',name:'public-analysis'}},
  }};
  const saved=await write(config,before.json().data.revision);
  assert.equal(saved.statusCode,200,JSON.stringify(saved.json()));
  assert.deepEqual(JSON.parse(await readFile(join(folder,'scenarios.json'),'utf8')).scenes,config.scenes);
  assert.equal(saved.json().data.config.scenes.logs.directory,realOutside);
  // Bound subscene directories must not become arbitrary direct-chat workspaces.
  const directOutside=await app.inject({method:'POST',url:'/api/v1/chat/sessions',headers,
    payload:{scenario:'direct-chat',directory:realOutside}});
  assert.equal(directOutside.statusCode,403,directOutside.body);
  assert.equal(directOutside.json().error.code,'DIRECTORY_OUTSIDE_CHAT_ROOTS');
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

it('paired sessions without a login username stay read-only despite editor ACL',async t=>{
  const workspace=await realpath(await mkdtemp(join(tmpdir(),'paired-scene-')));
  t.after(()=>rm(workspace,{recursive:true,force:true}));
  const port=4189,host='127.0.0.1:'+port,origin='http://'+host;
  const app=await createApp({auth:new LocalAuth('secret-once',port),workspace});
  t.after(()=>app.close());
  const pair=await app.inject({method:'POST',url:'/api/v1/auth/session',headers:{host,origin},
    payload:{pairingCode:'secret-once'}});
  assert.equal(pair.statusCode,200,pair.body);
  const headers={host,origin,cookie:String(pair.headers['set-cookie']).split(';')[0],
    'x-csrf-token':pair.json().data.csrfToken};
  const before=await app.inject({url:'/api/v1/scene-config',headers});
  assert.equal(before.statusCode,200);
  assert.equal(before.json().data.username,null);
  assert.equal(before.json().data.canEdit,false);
  // Even when the file names an editor, a pairing-only session has no login
  // username and must not gain configuration permissions.
  assert.deepEqual(JSON.parse(await readFile(join(workspace,'.workbench','config-access.json'),'utf8')),
    {version:1,editors:['zhangzhanhui']});
  const saved=await app.inject({method:'PUT',url:'/api/v1/scene-config',headers,
    payload:{config:{version:1,scenes:{}},revision:before.json().data.revision}});
  assert.equal(saved.statusCode,403,saved.body);
  assert.equal(saved.json().error.code,'SCENE_CONFIG_EDIT_FORBIDDEN');
});

it('only the exact configured login username can edit without hashes or phone coupling',async t=>{
  const workspace=await realpath(await mkdtemp(join(tmpdir(),'named-scene-')));
  t.after(()=>rm(workspace,{recursive:true,force:true}));
  const port=4195,host='127.0.0.1:'+port,origin='http://'+host;
  const app=await createApp({auth:new LocalAuth('unused',port),localOnly:true,workspace});
  t.after(()=>app.close());
  const login=await app.inject({method:'POST',url:'/api/v1/auth/local-session',headers:{host,origin},
    payload:{name:'zhangzhanhui',phone:'13900000000'}});
  assert.equal(login.statusCode,200,login.body);
  const headers={host,origin,cookie:String(login.headers['set-cookie']).split(';')[0],
    'x-csrf-token':login.json().data.csrfToken};
  const state=await app.inject({url:'/api/v1/scene-config',headers});
  assert.equal(state.statusCode,200);
  assert.equal(state.json().data.username,'zhangzhanhui');
  assert.equal(state.json().data.canEdit,true);
  const saved=await app.inject({method:'PUT',url:'/api/v1/scene-config',headers,
    payload:{config:{version:1,scenes:{}},revision:state.json().data.revision}});
  assert.equal(saved.statusCode,200,saved.body);
  // ACL matching is case-sensitive. A username typo or another user cannot edit.
  await writeFile(join(workspace,'.workbench','config-access.json'),
    JSON.stringify({version:1,editors:['ZhangZhanHui']}));
  assert.equal((await app.inject({url:'/api/v1/scene-config',headers})).json().data.canEdit,false);
  const forbidden=await app.inject({method:'PUT',url:'/api/v1/scene-config',headers,
    payload:{config:{version:1,scenes:{}},revision:saved.json().data.revision}});
  assert.equal(forbidden.statusCode,403);
  // No wildcard, malformed editor name or unapproved hash permits access.
  await writeFile(join(workspace,'.workbench','config-access.json'),
    JSON.stringify({version:1,editors:['*','zhangzhanhui']}));
  assert.equal((await app.inject({url:'/api/v1/scene-config',headers})).json().data.canEdit,false);
});

it('startup fills missing default scene bindings without overwriting custom configured scenes',async t=>{
  const workspace=await realpath(await mkdtemp(join(tmpdir(),'default-scene-')));
  const outside=await realpath(await mkdtemp(join(tmpdir(),'custom-scene-')));
  t.after(()=>Promise.all([rm(workspace,{recursive:true,force:true}),rm(outside,{recursive:true,force:true})]));
  await mkdir(join(workspace,'.workbench'),{recursive:true});
  const custom={directory:outside,executor:{type:'subagent',name:'business-diagnostics'}};
  const file=join(workspace,'.workbench','scenarios.json');
  await writeFile(file,JSON.stringify({version:1,scenes:{logs:custom}}));
  const app=await createApp({localOnly:true,workspace,auth:new LocalAuth('unused',4201)});
  t.after(()=>app.close());
  const config=JSON.parse(await readFile(file,'utf8'));
  assert.deepEqual(config.scenes.logs,custom,'existing binding is preserved');
  assert.deepEqual(config.scenes.message,{directory:workspace,executor:{type:'command',name:'models'}});
  assert.deepEqual(config.scenes.data,{directory:workspace,executor:{type:'builtin',name:'chat'}});
  assert.ok(!JSON.stringify(config.scenes.logs).includes('brainstorming'));
  // Second startup is idempotent and does not change the user-managed file.
  const existing=await readFile(file,'utf8');
  const next=await createApp({localOnly:true,workspace,auth:new LocalAuth('unused',4202)});
  await next.close();
  assert.equal(await readFile(file,'utf8'),existing);
});

it('LAN investigation pages can read scene runtime without gaining localhost config access',async t=>{
  const workspace=await realpath(await mkdtemp(join(tmpdir(),'lan-scene-runtime-')));
  t.after(()=>rm(workspace,{recursive:true,force:true}));
  const port=4176,host='192.168.21.10:'+port,origin='http://'+host;
  const app=await createApp({localOnly:true,workspace,
    auth:new LocalAuth('unused',port,undefined,undefined,{origin})});
  t.after(()=>app.close());
  // Model a colleague accessing from the LAN address, not localhost.
  const login=await app.inject({method:'POST',url:'/api/v1/auth/local-session',
    headers:{host,origin},payload:{name:'LanPeer',phone:'13800000002'}});
  assert.equal(login.statusCode,200,login.body);
  const headers={host,origin,cookie:String(login.headers['set-cookie']).split(';')[0]};
  const runtime=await app.inject({method:'GET',url:'/api/v1/scene-runtime',headers});
  assert.equal(runtime.statusCode,200,runtime.body);
  const data=runtime.json().data;
  assert.ok(data.config.scenes.logs.directory,'public-management scene must have a binding');
  assert.ok(data.config.scenes.message.directory,'message scene must have a binding');
  assert.ok(data.config.scenes.data.directory,'analytics scene must have a binding');
  assert.equal(data.canEdit,false);
  assert.equal(data.configPath,'');
  assert.equal(data.accessPath,'');
  const admin=await app.inject({method:'GET',url:'/api/v1/scene-config',headers});
  assert.equal(admin.statusCode,403);
  assert.equal(admin.json().error.code,'CONFIG_LOCAL_ONLY');
  const write=await app.inject({method:'PUT',url:'/api/v1/scene-config',headers,
    payload:{config:data.config,revision:data.revision}});
  assert.equal(write.statusCode,403,'fix must not relax admin configuration authorization');
});
