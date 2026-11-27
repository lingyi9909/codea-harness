import { spawnSync } from 'node:child_process';
import assert from 'node:assert/strict';
import { it } from 'node:test';
import { copyFile, mkdir, mkdtemp, readFile, realpath, rm, symlink, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { delimiter, join, resolve } from 'node:path';
import { createApp } from '../../apps/server/src/app.ts';
import { LocalAuth } from '../../apps/server/src/auth.ts';
import {profileKey} from '../../apps/server/src/local-profile.ts';

const port=4181,host='127.0.0.1:'+port,origin='http://'+host;
const profile={name:'ExampleUser',phone:'+12025550123'};
const bootstrap='/api/v1/bootstrap',open='/api/v1/auth/local-session';
it('packaged local sessions require a loopback Host and matching Origin, reuse their cookie and retain CSRF checks',async t=>{
  const workspace=await mkdtemp(join(tmpdir(),'local-profile-'));
  t.after(()=>rm(workspace,{recursive:true,force:true}));
  const app=await createApp({auth:new LocalAuth('unused',port),localOnly:true,workspace});
  try{
    const first=await app.inject({url:bootstrap,headers:{host}});
    assert.equal(first.statusCode,401);
    assert.equal(first.json().error.code,'PROFILE_LOGIN_REQUIRED');
    const paired=await app.inject({method:'POST',url:'/api/v1/auth/session',headers:{host,origin},payload:{pairingCode:'unused'}});
    assert.equal(paired.statusCode,404);
    for(const url of ['/api/v1/chat/settings','/api/v1/tasks','/api/v1/repositories']){
      assert.equal((await app.inject({url,headers:{host}})).statusCode,401);
    }
    for(const payload of [{},{name:'   ',phone:profile.phone},{name:'test',phone:'   '},{name:'test',phone:12345678901}]){
      const denied=await app.inject({method:'POST',url:open,headers:{host,origin},payload});
      assert.equal(denied.statusCode,400);
      assert.ok(['PROFILE_INVALID','USERNAME_INVALID'].includes(denied.json().error.code));
      assert.equal(denied.headers['set-cookie'],undefined);
    }
    for(const headers of [{host},{host,origin:'http://localhost:'+port},{host:'example.com:'+port,origin:'http://example.com:'+port}]){
      const denied=await app.inject({method:'POST',url:open,headers});
      assert.equal(denied.statusCode,403);
      assert.equal(denied.headers['set-cookie'],undefined);
    }
    const opened=await app.inject({method:'POST',url:open,headers:{host,origin},payload:profile});
    assert.equal(opened.statusCode,200);
    assert.deepEqual(opened.json().data.owner,{id:'local:'+profileKey(profile),name:profile.name});
    assert.ok(!opened.body.includes(profile.phone));
    assert.deepEqual(JSON.parse(await readFile(join(workspace,'.workbench','user.json'),'utf8')),profile);
    const setCookie=String(opened.headers['set-cookie']);
    assert.match(setCookie,/HttpOnly; SameSite=Strict/);
    const cookie=setCookie.split(';')[0],csrf=opened.json().data.csrfToken;
    const reused=await app.inject({method:'POST',url:open,headers:{host,origin,cookie},payload:profile});
    assert.equal(reused.headers['set-cookie'],undefined);
    assert.equal(reused.json().data.csrfToken,csrf);
    const refreshed=await app.inject({url:bootstrap,headers:{host,cookie}});
    assert.equal(refreshed.json().data.csrfToken,csrf);
    assert.deepEqual(refreshed.json().data.owner,opened.json().data.owner);
    const different=await app.inject({method:'POST',url:open,headers:{host,origin},payload:{...profile,name:'AnotherUser'}});
    assert.equal(different.statusCode,200,'different visitors may enter the same workspace');
    assert.notEqual(different.json().data.owner.id,opened.json().data.owner.id);
    assert.equal((await app.inject({url:bootstrap,headers:{host,cookie}})).statusCode,200,
      'an additional visitor must not invalidate the original visitor');
    const loose=await app.inject({method:'POST',url:open,headers:{host,origin},
      payload:{name:'Visitor',phone:'employee-123'}});
    assert.equal(loose.statusCode,200,'self-reported phone labels do not require mobile regex');
    assert.equal(loose.json().data.owner.name,'Visitor');
    const logout='/api/v1/auth/session';
    for(const headers of [{host,origin,cookie},{host,origin:'http://example.com',cookie,'x-csrf-token':csrf}]){
      assert.equal((await app.inject({method:'DELETE',url:logout,headers})).statusCode,403);
    }
    assert.equal((await app.inject({method:'DELETE',url:logout,headers:{host,origin,cookie,'x-csrf-token':csrf}})).statusCode,204);
    assert.equal((await app.inject({url:bootstrap,headers:{host,cookie}})).statusCode,401);
  }finally{await app.close();}
});
it('allows parallel visitor profiles with isolated local sessions and account switching',async t=>{
  const workspace=await mkdtemp(join(tmpdir(),'local-profile-'));
  t.after(()=>rm(workspace,{recursive:true,force:true}));
  const app=await createApp({auth:new LocalAuth('unused',port),localOnly:true,workspace});
  t.after(()=>app.close());
  const candidates=[profile,{name:'ExampleTwo',phone:'staff-id-2'}];
  const login=(payload:typeof profile,headers:Record<string,string>={host,origin})=>
    app.inject({method:'POST',url:open,headers,payload});
  const attempts=await Promise.all(candidates.map(c=>login(c)));
  assert.ok(attempts.every(a=>a.statusCode===200),attempts.map(a=>a.body).join('\n'));
  const sessions=attempts.map(a=>({
    headers:{host,origin,cookie:String(a.headers['set-cookie']).split(';')[0]},
    owner:a.json().data.owner.id,
  }));
  assert.notEqual(sessions[0].owner,sessions[1].owner);
  for(const current of sessions){
    const response=await app.inject({url:bootstrap,headers:current.headers});
    assert.equal(response.statusCode,200,response.body);
    assert.equal(response.json().data.owner.id,current.owner);
  }
  // The shared user.json is advisory, not a gate for all browser sessions.
  await writeFile(join(workspace,'.workbench','user.json'),JSON.stringify(candidates[0]));
  assert.equal((await app.inject({url:bootstrap,headers:sessions[1].headers})).statusCode,200);
  const switched=await login({name:'SwitchedUser',phone:'12345'},sessions[0].headers);
  assert.equal(switched.statusCode,200,switched.body);
  assert.notEqual(switched.json().data.owner.id,sessions[0].owner);
  assert.equal((await app.inject({url:bootstrap,headers:sessions[0].headers})).statusCode,401,
    'switching an account must revoke its old cookie');
  assert.equal((await app.inject({url:bootstrap,headers:sessions[1].headers})).statusCode,200,
    'switching one browser must not revoke peer sessions');
});

it('LAN self-reported visitor entry handles concurrent peers and alternate private addresses',async t=>{
  const workspace=await mkdtemp(join(tmpdir(),'lan-visitor-entry-'));
  t.after(()=>rm(workspace,{recursive:true,force:true}));
  const auth=new LocalAuth('unused',port,undefined,undefined,{origin:'http://192.168.1.66:'+port});
  const app=await createApp({auth,localOnly:true,workspace});
  t.after(()=>app.close());
  const hostA='192.168.1.66:'+port,originA='http://'+hostA;
  const ids=new Set<string>();
  for(let i=0;i<32;i++){
    const login=await app.inject({method:'POST',url:open,remoteAddress:'192.168.1.51',
      headers:{host:hostA,origin:originA},payload:{name:'Peer'+i,phone:'staff-'+i}});
    assert.equal(login.statusCode,200,login.body);
    const id=login.json().data.owner.id;
    assert.ok(!ids.has(id),'each independently logged-in browser has its own owner');
    ids.add(id);
    if(i===0||i===31){
      const state=await app.inject({url:bootstrap,remoteAddress:'192.168.1.51',
        headers:{host:hostA,origin:originA,cookie:String(login.headers['set-cookie']).split(';')[0]}});
      assert.equal(state.statusCode,200,state.body);
      assert.equal(state.json().data.owner.id,id);
    }
  }
  const alternate='192.168.1.67:'+port;
  const allowed=await app.inject({method:'POST',url:open,remoteAddress:'192.168.1.52',
    headers:{host:alternate,origin:'http://'+alternate},
    payload:{name:'AlternateNic',phone:'extension-99'}});
  assert.equal(allowed.statusCode,200,allowed.body);
  const denied=await app.inject({method:'POST',url:open,remoteAddress:'192.168.1.53',
    headers:{host:alternate,origin:originA},payload:{name:'WrongOrigin',phone:'123'}});
  assert.equal(denied.statusCode,403,'cross-origin registration is still denied');
});

it('rejects redirected profile directories and files before opening a session',async t=>{
  const fixture=await mkdtemp(join(tmpdir(),'local-profile-'));
  t.after(()=>rm(fixture,{recursive:true,force:true}));
  const workspace=join(fixture,'workspace'),outside=join(fixture,'outside'),directory=join(workspace,'.workbench');
  await mkdir(workspace);
  await mkdir(outside);
  const app=await createApp({auth:new LocalAuth('unused',port),localOnly:true,workspace});
  t.after(()=>app.close());
  async function assertDenied(){
    const response=await app.inject({method:'POST',url:open,headers:{host,origin},payload:profile});
    assert.equal(response.statusCode,503);
    assert.equal(response.json().error.code,'PROFILE_WRITE_FAILED');
    assert.equal(response.headers['set-cookie'],undefined);
  }
  await rm(directory,{recursive:true,force:true}); // startup now creates the safe config folder
  await symlink(outside,directory,'junction');
  await assertDenied();
  await assert.rejects(readFile(join(outside,'user.json')),{code:'ENOENT'});
  await rm(directory);
  await mkdir(directory);
  // Junctions also exercise Windows reparse-point protection without elevated privileges.
  await symlink(outside,join(directory,'user.json'),'junction');
  await assertDenied();
});
it('the default server requires the pairing code and does not register automatic session creation',async()=>{
  const app=await createApp({auth:new LocalAuth('fixture',port)});
  try{
    assert.equal((await app.inject({method:'POST',url:open,headers:{host,origin}})).statusCode,404);
    assert.equal((await app.inject({url:bootstrap,headers:{host}})).json().error.code,'AUTH_REQUIRED');
    const paired=await app.inject({method:'POST',url:'/api/v1/auth/session',headers:{host,origin},payload:{pairingCode:'fixture'}});
    assert.equal(paired.statusCode,200);
  }finally{await app.close();}
});

it('binds packaged operations to the one existing project selected at startup',async t=>{
  const fixture=await realpath(await mkdtemp(join(tmpdir(),'local workspace ')));
  t.after(()=>rm(fixture,{recursive:true,force:true}));
  const project=join(fixture,'project A'),other=join(fixture,'project B'),child=join(project,'nested');
  await mkdir(child,{recursive:true});
  await mkdir(other);
  const previous=process.env.HARNESS_CHAT_ROOTS;
  process.env.HARNESS_CHAT_ROOTS=[other,project].join(delimiter);
  t.after(()=>{if(previous===undefined)delete process.env.HARNESS_CHAT_ROOTS;else process.env.HARNESS_CHAT_ROOTS=previous;});
  async function openWorkspace(workspace?:string){
    const app=await createApp({auth:new LocalAuth('unused',port),localOnly:true,workspace});
    t.after(()=>app.close());
    const opened=await app.inject({method:'POST',url:open,headers:{host,origin},payload:profile});
    const headers={host,origin,cookie:String(opened.headers['set-cookie']).split(';')[0],'x-csrf-token':opened.json().data.csrfToken};
    return {app,headers};
  }
  for(const [input,expected] of [[project,project],[undefined,other]] as const){
    const {app,headers}=await openWorkspace(input);
    const settings=await app.inject({url:'/api/v1/chat/settings',headers});
    assert.deepEqual(settings.json().data.directories,[expected]);
    const post=(path:string,payload:object)=>app.inject({method:'POST',url:'/api/v1/chat/'+path,headers,payload});
    assert.equal((await post('project',{directory:expected})).statusCode,404);
    for(const directory of [child,fixture]){
      assert.equal((await post('history',{scenario:'direct-chat',directory})).statusCode,403);
      assert.equal((await post('models',{directory})).statusCode,403);
      assert.equal((await post('sessions',{directory})).statusCode,403);
    }
  }
  const file=join(fixture,'plain file');
  await writeFile(file,'fixture');
  for(const path of [file,join(fixture,'missing')])
    await assert.rejects(()=>openWorkspace(path),/Workspace directory.*does not exist or is not a directory/);
});

it('Windows launcher resolves quoted project paths before changing into the bundle',{skip:process.platform!=='win32'},async t=>{
  const dir=await mkdtemp(join(tmpdir(),'workbench-launch-'));
  t.after(()=>rm(dir,{recursive:true,force:true}));
  const bundle=join(dir,'bundle'),entry=join(bundle,'scripts');
  await mkdir(entry,{recursive:true});
  await mkdir(join(dir,'project with spaces'));
  await copyFile(new URL('../../start-chat.cmd',import.meta.url),join(bundle,'start-chat.cmd'));
  // The real CMD now delegates to the interactive/non-interactive Node menu.
  // Stub only that entrypoint to verify quoting, resolved workspace and cwd.
  await writeFile(join(entry,'launch-workbench.mjs'),
    "import {writeFileSync} from 'node:fs';writeFileSync('argv.json',JSON.stringify({cwd:process.cwd(),args:process.argv.slice(2)}))");
  for(const arg of [' "project with spaces"','']){
    const result=spawnSync('cmd.exe',['/d','/c','bundle\\start-chat.cmd'+arg],{
      cwd:dir,encoding:'utf8',windowsVerbatimArguments:true,timeout:15000,
    });
    if(!arg){assert.equal(result.status,2,result.stderr+result.stdout);continue;}
    assert.equal(result.status,0,result.stderr+result.stdout);
    const data=JSON.parse(await readFile(join(bundle,'argv.json'),'utf8'));
    assert.equal(resolve(data.cwd),bundle);
    assert.equal(data.args.length,1);
    assert.equal(resolve(data.args[0]),join(dir,'project with spaces'));
  }
});
