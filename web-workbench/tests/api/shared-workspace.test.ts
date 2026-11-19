import {test} from 'node:test';
import assert from 'node:assert/strict';
import {chmod,readFile,writeFile,mkdtemp,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {embedFixture} from '../support/embed-fixture.ts';
const visit=async(f:Awaited<ReturnType<typeof embedFixture>>)=>{
  const r=await f.app.inject({method:'POST',url:'/api/v1/embed/anonymous',headers:f.headers(),payload:{}});assert.equal(r.statusCode,200,r.body);return r.json().data;
};
const create=async(f:Awaited<ReturnType<typeof embedFixture>>,v:{token:string;csrfToken:string})=>{
  const r=await f.app.inject({method:'POST',url:'/api/v1/embed/chat/sessions',headers:f.headers(v.token,v.csrfToken),payload:{scenario:'message-inspection'}});assert.equal(r.statusCode,201,r.body);return r.json().data.id as string;
};
test('one workspace: concurrent pages, native deny policy, restart and cleanup keep shared definitions intact',async()=>{
  let resets=0;const f=await embedFixture(undefined,undefined,true,{anonymous:true,sharedWorkspace:true,resetSlot:async()=>{resets++;}});
  try{
    const marker=join(f.workspace,'business-skill.md');await writeFile(marker,'shared readonly skill definition');
    const visits=await Promise.all([0,1,2].map(()=>visit(f))),ids=await Promise.all(visits.map(v=>create(f,v)));
    assert.equal(new Set([...f.nativeSessions.values()].map(s=>s.location.directory)).size,1);assert.ok([...f.nativeSessions.values()].every(s=>s.location.directory===f.workspace));
    assert.ok(f.records.every(r=>Object.values(r.directories).every(d=>d===f.workspace)));
    for(const s of f.nativeSessions.values())assert.deepEqual(s.permissions,[{action:'edit',resource:'*',effect:'deny'}]);
    await Promise.all(visits.map(async(v,i)=>{const r=await f.app.inject({method:'POST',url:'/api/v1/embed/chat/sessions/'+ids[i]+'/messages',headers:f.headers(v.token,v.csrfToken),payload:{text:'shared-'+i}});assert.equal(r.statusCode,202,r.body);}));
    assert.equal(f.calls.length,3);assert.ok(ids.every(id=>f.active[id]));
    const foreign=await f.app.inject({method:'POST',url:'/api/v1/embed/chat/sessions/'+ids[1]+'/interrupt',headers:f.headers(visits[0].token,visits[0].csrfToken),payload:{}});assert.equal(foreign.statusCode,404);
    await f.restart();for(let i=0;i<3;i++)assert.equal((await f.app.inject({url:'/api/v1/embed/chat/sessions/'+ids[i],headers:f.headers(visits[i].token)})).statusCode,200);
    assert.equal(f.calls.length,3);
    assert.equal((await f.app.inject({method:'DELETE',url:'/api/v1/embed/session',headers:f.headers(visits[0].token,visits[0].csrfToken)})).statusCode,204);
    assert.equal(resets,1);assert.equal(f.active[ids[1]],true);assert.equal(f.active[ids[2]],true);assert.equal(await readFile(marker,'utf8'),'shared readonly skill definition');
    const next=await visit(f);await create(f,next);assert.equal(await readFile(marker,'utf8'),'shared readonly skill definition');
  }finally{await f.close();}
});
test('one workspace: native permission omission or later mutation fails closed before execution',async()=>{
  const f=await embedFixture(undefined,undefined,false,{anonymous:true,sharedWorkspace:true});
  try{
    const v=await visit(f),id=await create(f,v);f.nativeSessions.get(id).permissions=[];
    const denied=await f.app.inject({method:'POST',url:'/api/v1/embed/chat/sessions/'+id+'/messages',headers:f.headers(v.token,v.csrfToken),payload:{text:'must not run'}});assert.equal(denied.statusCode,409);assert.equal(f.calls.length,0);
    const v2=await visit(f);const owner=[...f.store.grants.values()].find(g=>g.digest!==[...f.store.grants.values()][0].digest)!.ownerId;
    await f.execution.run(owner,async()=>{const client=await f.execution.client(),create=client.session.create;client.session.create=async input=>{const s=await create(input);delete s.permissions;return s;};});
    const missing=await f.app.inject({method:'POST',url:'/api/v1/embed/chat/sessions',headers:f.headers(v2.token,v2.csrfToken),payload:{scenario:'message-inspection'}});assert.equal(missing.statusCode,409);assert.equal(f.nativeSessions.size,1,'unprotected native Session is compensated');
  }finally{await f.close();}
});
test('shared readonly filesystem: definitions are readable, edits and new output files are denied',{skip:process.platform==='win32'||process.getuid?.()===0},async()=>{
  const directory=await mkdtemp(join(tmpdir(),'shared-readonly-')),file=join(directory,'skill.md');
  try{await writeFile(file,'shared');await chmod(file,0o444);await chmod(directory,0o555);assert.equal(await readFile(file,'utf8'),'shared');await assert.rejects(writeFile(file,'changed'),{code:'EACCES'});await assert.rejects(writeFile(join(directory,'result.json'),'private data'),{code:'EACCES'});}finally{await chmod(directory,0o700);await chmod(file,0o600);await rm(directory,{recursive:true,force:true});}
});
test('one workspace: private-profile reset never receives or removes the shared directory',async()=>{
  const privateRoot=await mkdtemp(join(tmpdir(),'private-profile-reset-')),log=join(privateRoot,'reset.json'),script=join(privateRoot,'reset.mjs');
  await writeFile(script,"import fs from 'node:fs';fs.writeFileSync(process.argv[2],JSON.stringify(process.argv.slice(3)));");
  const f=await embedFixture(undefined,undefined,false,{anonymous:true,sharedWorkspace:true,resetPrivateCommand:[process.execPath,script,log]});
  try{
    const marker=join(f.workspace,'keep.md');await writeFile(marker,'keep');const v=await visit(f);await create(f,v);
    const ended=await f.app.inject({method:'DELETE',url:'/api/v1/embed/session',headers:f.headers(v.token,v.csrfToken)});assert.equal(ended.statusCode,204);
    const args=JSON.parse(await readFile(log,'utf8'));assert.equal(args[0],'--owner');assert.equal(args[2],'--service');assert.equal(args.length,4);assert.equal(args.includes(f.workspace),false);assert.equal(args.includes('--directory'),false);
    assert.equal(await readFile(marker,'utf8'),'keep');assert.equal(f.store.recycledIds.size,1);
  }finally{await f.close();await rm(privateRoot,{recursive:true,force:true});}
});
test('one workspace: an edit permission reply cannot override the mandatory native deny',async()=>{
  const f=await embedFixture(undefined,undefined,false,{anonymous:true,sharedWorkspace:true});
  try{
    const v=await visit(f),id=await create(f,v),owner=f.nativeSessions.get(id).metadata.harnessOwner;
    await f.execution.run(owner,async()=>{const client=await f.execution.client();client.permission.list=async()=>[{id:'p-edit',sessionID:id,action:'edit',resources:[f.workspace],message:'try write'}] as never;});
    const r=await f.app.inject({method:'POST',url:'/api/v1/embed/chat/sessions/'+id+'/permission',headers:f.headers(v.token,v.csrfToken),payload:{requestId:'p-edit',revision:'a'.repeat(64),decision:'once'}});assert.equal(r.statusCode,403,r.body);assert.equal(r.json().error.code,'SHARED_WORKSPACE_READ_ONLY');
    f.config.anonymous!.resetCommand=[process.execPath,'must-not-run.mjs'];await assert.rejects(f.restart(),/SHARED_WORKSPACE_RESET_COMMAND_FORBIDDEN/);
  }finally{await f.close();}
});
