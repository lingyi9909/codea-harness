import { spawnSync } from 'node:child_process';
import assert from 'node:assert/strict';
import { it } from 'node:test';
import { copyFile, mkdir, mkdtemp, readFile, realpath, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { delimiter, join, resolve } from 'node:path';
import { createApp } from '../../apps/server/src/app.ts';
import { LocalAuth } from '../../apps/server/src/auth.ts';

const port=4181,host='127.0.0.1:'+port,origin='http://'+host;
const bootstrap='/api/v1/bootstrap',open='/api/v1/auth/local-session';
it('packaged local sessions require a loopback Host and matching Origin, reuse their cookie and retain CSRF checks',async()=>{
  const app=await createApp({auth:new LocalAuth('unused',port),localOnly:true});
  try{
    const first=await app.inject({url:bootstrap,headers:{host}});
    assert.equal(first.statusCode,401);
    assert.equal(first.json().error.code,'LOCAL_SESSION_REQUIRED');
    for(const headers of [{host},{host,origin:'http://localhost:'+port},{host:'example.com:'+port,origin:'http://example.com:'+port}]){
      const denied=await app.inject({method:'POST',url:open,headers});
      assert.equal(denied.statusCode,403);
      assert.equal(denied.headers['set-cookie'],undefined);
    }
    const opened=await app.inject({method:'POST',url:open,headers:{host,origin}});
    assert.equal(opened.statusCode,200);
    const setCookie=String(opened.headers['set-cookie']);
    assert.match(setCookie,/HttpOnly; SameSite=Strict/);
    const cookie=setCookie.split(';')[0],csrf=opened.json().data.csrfToken;
    const reused=await app.inject({method:'POST',url:open,headers:{host,origin,cookie}});
    assert.equal(reused.headers['set-cookie'],undefined);
    assert.equal(reused.json().data.csrfToken,csrf);
    const refreshed=await app.inject({url:bootstrap,headers:{host,cookie}});
    assert.equal(refreshed.json().data.csrfToken,csrf);
    const logout='/api/v1/auth/session';
    for(const headers of [{host,origin,cookie},{host,origin:'http://example.com',cookie,'x-csrf-token':csrf}]){
      assert.equal((await app.inject({method:'DELETE',url:logout,headers})).statusCode,403);
    }
    assert.equal((await app.inject({method:'DELETE',url:logout,headers:{host,origin,cookie,'x-csrf-token':csrf}})).statusCode,204);
    assert.equal((await app.inject({url:bootstrap,headers:{host,cookie}})).statusCode,401);
  }finally{await app.close();}
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
    const opened=await app.inject({method:'POST',url:open,headers:{host,origin}});
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
