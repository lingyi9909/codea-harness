import { spawnSync } from 'node:child_process';
import assert from 'node:assert/strict';
import { it } from 'node:test';
import { copyFile, mkdir, mkdtemp, readFile, realpath, rm, symlink, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { delimiter, join, resolve } from 'node:path';
import { createApp } from '../../apps/server/src/app.ts';
import { LocalAuth } from '../../apps/server/src/auth.ts';

const port=4181,host='127.0.0.1:'+port,origin='http://'+host;
const profile={name:'示例用户',phone:'+12025550123'};
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
    for(const payload of [{},{name:'   ',phone:profile.phone},{name:'test',phone:'abc'},{name:'test',phone:12345678901}]){
      const denied=await app.inject({method:'POST',url:open,headers:{host,origin},payload});
      assert.equal(denied.statusCode,400);
      assert.equal(denied.json().error.code,'PROFILE_INVALID');
      assert.equal(denied.headers['set-cookie'],undefined);
    }
    for(const headers of [{host},{host,origin:'http://localhost:'+port},{host:'example.com:'+port,origin:'http://example.com:'+port}]){
      const denied=await app.inject({method:'POST',url:open,headers});
      assert.equal(denied.statusCode,403);
      assert.equal(denied.headers['set-cookie'],undefined);
    }
    const opened=await app.inject({method:'POST',url:open,headers:{host,origin},payload:profile});
    assert.equal(opened.statusCode,200);
    assert.deepEqual(opened.json().data.owner,{id:'local-workbench',name:profile.name});
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
    const different=await app.inject({method:'POST',url:open,headers:{host,origin},payload:{...profile,name:'另一示例'}});
    assert.equal(different.statusCode,409);
    assert.equal(different.headers['set-cookie'],undefined);
    assert.deepEqual(JSON.parse(await readFile(join(workspace,'.workbench','user.json'),'utf8')),profile);
    const logout='/api/v1/auth/session';
    for(const headers of [{host,origin,cookie},{host,origin:'http://example.com',cookie,'x-csrf-token':csrf}]){
      assert.equal((await app.inject({method:'DELETE',url:logout,headers})).statusCode,403);
    }
    assert.equal((await app.inject({method:'DELETE',url:logout,headers:{host,origin,cookie,'x-csrf-token':csrf}})).statusCode,204);
    assert.equal((await app.inject({url:bootstrap,headers:{host,cookie}})).statusCode,401);
  }finally{await app.close();}
});
it('serializes different logins, allows a switch after logout, and gates externally changed profiles',async t=>{
  const workspace=await mkdtemp(join(tmpdir(),'local-profile-'));
  t.after(()=>rm(workspace,{recursive:true,force:true}));
  const app=await createApp({auth:new LocalAuth('unused',port),localOnly:true,workspace});
  t.after(()=>app.close());
  const candidates=[profile,{name:'示例二',phone:'13900000000'}];
  const login=(payload:typeof profile)=>app.inject({method:'POST',url:open,headers:{host,origin},payload});
  const attempts=await Promise.all(candidates.map(login));
  const winner=attempts.findIndex(r=>r.statusCode===200);
  assert.notEqual(winner,-1);
  assert.equal(attempts[1-winner].json().error.code,'PROFILE_IN_USE');
  const file=join(workspace,'.workbench','user.json');
  assert.deepEqual(JSON.parse(await readFile(file,'utf8')),candidates[winner]);
  const sessionHeaders=(r:typeof attempts[number])=>({host,origin,cookie:String(r.headers['set-cookie']).split(';')[0],'x-csrf-token':r.json().data.csrfToken});
  await app.inject({method:'DELETE',url:'/api/v1/auth/session',headers:sessionHeaders(attempts[winner])});
  const switched=await login(candidates[1-winner]);
  assert.equal(switched.statusCode,200);
  assert.deepEqual(JSON.parse(await readFile(file,'utf8')),candidates[1-winner]);
  await writeFile(file,JSON.stringify(candidates[winner]));
  for(const url of [bootstrap,'/api/v1/chat/settings','/api/v1/tasks']){
    const denied=await app.inject({url,headers:sessionHeaders(switched)});
    assert.equal(denied.statusCode,401);
    assert.equal(denied.json().error.code,'PROFILE_LOGIN_REQUIRED');
  }
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
  const bundle=join(dir,'bundle'),entry=join(bundle,'apps','server','src');
  await mkdir(entry,{recursive:true});
  await mkdir(join(dir,'project with spaces'));
  await copyFile(new URL('../../start-chat.cmd',import.meta.url),join(bundle,'start-chat.cmd'));
  await writeFile(join(entry,'start.ts'),"require('node:fs').writeFileSync('argv.json',JSON.stringify(process.argv.slice(2)))");
  for(const arg of [' "project with spaces"','']){
    const result=spawnSync('cmd.exe',['/d','/c','bundle\\start-chat.cmd'+arg],{
      cwd:dir,encoding:'utf8',windowsVerbatimArguments:true,timeout:15000,
    });
    assert.equal(result.status,0,result.stderr+result.stdout);
    const argv=JSON.parse(await readFile(join(bundle,'argv.json'),'utf8'));
    assert.equal(argv[0],'--local-only');
    if(arg){assert.equal(argv[1],'--workspace');assert.equal(resolve(argv[2]),join(dir,'project with spaces'));}
    else assert.deepEqual(argv,['--local-only']);
  }
});
