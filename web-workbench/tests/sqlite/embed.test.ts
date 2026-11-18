import {fileURLToPath} from 'node:url';
import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm,writeFile,symlink} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {randomUUID,randomBytes,createHash} from 'node:crypto';
import {spawn} from 'node:child_process';
import {DatabaseSync} from 'node:sqlite';
import {SqliteEmbedStore} from '../../packages/storage/src/sqlite-embed.ts';
import {sqliteEmbedFixture} from '../support/sqlite-embed-fixture.ts';
const next=()=>({id:randomUUID(),digest:createHash('sha256').update(randomBytes(32)).digest('hex'),expires:Date.now()+60000,parentOrigin:'https://workbench.example'});
const child=(script:string,args:string[])=>new Promise<string>((resolve,reject)=>{const p=spawn(process.execPath,['--experimental-strip-types',script,...args],{stdio:['ignore','pipe','pipe']});let out='',err='';p.stdout.on('data',c=>out+=c);p.stderr.on('data',c=>err+=c);p.on('error',reject);p.on('exit',code=>code===0?resolve(out):reject(Error(err)));});
test('SQLite file: versioned migrations, atomic tickets, durable query/bindings, reset fence and consistent backup',async()=>{
  const dir=await mkdtemp(join(tmpdir(),'sqlite-store-')),path=join(dir,'state.sqlite');let store=new SqliteEmbedStore(path);
  try{
    const owner=randomUUID();await store.ensureOwner(owner,'owner');const ticket=next();await store.ticket({...ticket,ownerId:owner,clientId:'host'});
    const exchanged=await Promise.all([store.exchange(ticket.digest,ticket.parentOrigin,next()),store.exchange(ticket.digest,ticket.parentOrigin,next())]);assert.equal(exchanged.filter(Boolean).length,1);const grant=exchanged.find(Boolean)!;
    assert.deepEqual((await Promise.all([store.reserve(grant.id),store.reserve(grant.id)])).sort(),[false,true]);await store.bind(grant.id,'native','service');
    const binding={id:'native',ownerId:owner,serviceIdentity:'service',scenario:'message-inspection',directory:'/one-workspace'};await store.bindings.create(binding);
    assert.equal((await store.claimQuery(grant.id,'fingerprint')).claimed,true);await store.finishQuery(grant.id,'safe evidence');
    await store.close();store=new SqliteEmbedStore(path);assert.equal((await store.find(grant.digest))?.nativeId,'native');assert.deepEqual(await store.bindings.get('service','native',owner),binding);
    assert.equal((await store.claimQuery(grant.id,'fingerprint')).evidence,'safe evidence');assert.equal((await store.claimQuery(grant.id,'fingerprint')).claimed,false);assert.equal(await store.bindings.get('service','native',randomUUID()),undefined);
    const backup=join(dir,'backup.sqlite');await store.backup(backup);const restored=new SqliteEmbedStore(backup);assert.ok(await restored.find(grant.digest));await restored.close();
    await store.revoke(grant.id);assert.equal(await store.find(grant.digest),undefined);assert.equal(await store.allocate(next(),[{ownerId:owner,serviceIdentity:'service'}]),undefined);
    await store.cleaned(grant.id);assert.equal(await store.allocate(next(),[{ownerId:owner,serviceIdentity:'service'}]),undefined);await store.recycled(grant.id);assert.ok(await store.allocate(next(),[{ownerId:owner,serviceIdentity:'service'}]));
  }finally{await store.close();await rm(dir,{recursive:true,force:true});}
});
test('SQLite real processes: three slots cannot be double allocated; dead execution locks require explicit recovery',async()=>{
  const dir=await mkdtemp(join(tmpdir(),'sqlite-processes-')),path=join(dir,'state.sqlite'),script=join(dir,'worker.mjs'),store=new SqliteEmbedStore(path);
  const module=new URL('../../packages/storage/src/sqlite-embed.ts',import.meta.url).href;
  try{
    const slots=[0,1,2].map(i=>({ownerId:randomUUID(),serviceIdentity:'service-'+i}));
    await writeFile(script,`import {SqliteEmbedStore} from ${JSON.stringify(module)};import {randomUUID} from 'node:crypto';const s=new SqliteEmbedStore(process.argv[2]);if(process.argv[3]==='crash'){await s.serialize(process.argv[4],async()=>process.exit(0));}else{const g=await s.allocate({id:randomUUID(),digest:randomUUID(),expires:Date.now()+60000,parentOrigin:'https://workbench.example'},JSON.parse(process.argv[3]));console.log(JSON.stringify(g??null));await s.close();}`);
    const results=await Promise.all(Array.from({length:6},()=>child(script,[path,JSON.stringify(slots)]).then(JSON.parse)));const grants=results.filter(Boolean);assert.equal(grants.length,3);assert.equal(new Set(grants.map(g=>g.ownerId)).size,3);
    const owner=randomUUID();await child(script,[path,'crash',owner]);const lock=store.recoveryLocks().find(l=>l.ownerId===owner)!;assert.equal(lock.holderAlive,false);
    await assert.rejects(store.serialize(owner,async()=>{}),/RECOVERY_REQUIRED/);assert.throws(()=>store.recoverLock(owner,lock.nonce,false),/CONFIRM_REQUIRED/);store.recoverLock(owner,lock.nonce,true);assert.equal(await store.serialize(owner,async()=>'recovered'),'recovered');
    await store.serialize(owner,async()=>{const live=store.recoveryLocks().find(l=>l.ownerId===owner)!;assert.throws(()=>store.recoverLock(owner,live.nonce,true),/LIVE_OR_CHANGED/);});
  }finally{await store.close();await rm(dir,{recursive:true,force:true});}
});
test('SQLite: incompatible files, migration tampering and database symlinks are rejected',async()=>{
  const dir=await mkdtemp(join(tmpdir(),'sqlite-invalid-')),path=join(dir,'state.sqlite');
  try{
    const store=new SqliteEmbedStore(path);await store.close();const db=new DatabaseSync(path);db.exec("UPDATE wh_sqlite_migrations SET sha256='changed'");db.close();assert.throws(()=>new SqliteEmbedStore(path),/CHECKSUM_MISMATCH/);
    const unrelated=join(dir,'other.sqlite'),other=new DatabaseSync(unrelated);other.exec('CREATE TABLE unrelated(id INTEGER)');other.close();assert.throws(()=>new SqliteEmbedStore(unrelated),/UNTRACKED_DATABASE/);
    if(process.platform!=='win32'){const link=join(dir,'link.sqlite');await symlink(path,link);assert.throws(()=>new SqliteEmbedStore(link),/SYMLINK_FORBIDDEN/);}
  }finally{await rm(dir,{recursive:true,force:true});}
});
test('SQLite standalone HTTP: three pages share one workspace, survive database reopen, reject foreign operations and never register login',async()=>{
  let authCalls=0;const f=await sqliteEmbedFixture(undefined,undefined,true,async()=>{authCalls++;return {evidence:'安全日志证据'};});
  try{
    assert.equal((await f.app.inject({url:'/api/v1/health'})).json().data.storage,'sqlite');assert.equal((await f.app.inject({method:'POST',url:'/api/v1/auth/login',payload:{}})).statusCode,404);
    const visits=await Promise.all([0,1,2].map(async()=>{const r=await f.app.inject({method:'POST',url:'/api/v1/embed/anonymous',headers:f.headers(),payload:{}});assert.equal(r.statusCode,200,r.body);return r.json().data;}));
    const ids=await Promise.all(visits.map(async v=>{const r=await f.app.inject({method:'POST',url:'/api/v1/embed/chat/sessions',headers:f.headers(v.token,v.csrfToken),payload:{scenario:'message-inspection'}});assert.equal(r.statusCode,201,r.body);return r.json().data.id;}));
    await Promise.all(visits.map(async(v,i)=>assert.equal((await f.app.inject({method:'POST',url:'/api/v1/embed/chat/sessions/'+ids[i]+'/messages',headers:f.headers(v.token,v.csrfToken),payload:{text:'visitor-'+i}})).statusCode,202)));
    const query={day:'2026-11-18',from:Date.UTC(2026,10,17,16),to:Date.UTC(2026,10,18,16),msgSn:'0001',account:'private-account',password:'private-password',verificationCode:'001234'};
    const evidence=()=>f.app.inject({method:'POST',url:'/api/v1/embed/institution-query',headers:f.headers(visits[0].token,visits[0].csrfToken),payload:query});assert.equal((await evidence()).statusCode,200);
    await f.restart();assert.equal((await evidence()).statusCode,200);assert.equal(authCalls,1);assert.equal(f.calls.length,3);
    for(let i=0;i<3;i++){const r=await f.app.inject({url:'/api/v1/embed/chat/sessions/'+ids[i],headers:f.headers(visits[i].token)});assert.equal(r.statusCode,200,r.body);}
    for(const [method,suffix,payload]of [['GET','',undefined],['DELETE','',undefined],['PATCH','',{title:'foreign'}],['POST','/messages',{text:'foreign'}],['POST','/interrupt',{}],['GET','/activity',undefined],['POST','/permission',{requestId:'p',revision:'a'.repeat(64),decision:'once'}],['POST','/form',{requestId:'f',revision:'a'.repeat(64),decision:'reply',answer:{}}]]as const)assert.equal((await f.app.inject({method,url:'/api/v1/embed/chat/sessions/'+ids[1]+suffix,headers:f.headers(visits[0].token,visits[0].csrfToken),payload})).statusCode,404);
    assert.equal((await f.app.inject({method:'DELETE',url:'/api/v1/embed/session',headers:f.headers(visits[0].token,visits[0].csrfToken)})).statusCode,204);assert.equal(f.active[ids[1]],true);assert.equal(f.active[ids[2]],true);
    assert.ok([...f.nativeSessions.values()].every(s=>s.location.directory===f.workspace));
    const db=new DatabaseSync(f.path,{readOnly:true}),content=JSON.stringify(db.prepare('SELECT * FROM wh_embed_sessions').all());db.close();for(const secret of [query.account,query.password,query.verificationCode])assert.equal(content.includes(secret),false);
  }finally{await f.close();}
});
test('SQLite CLI: embedded-only starts without MySQL settings and restores its grant after process restart',{timeout:30000},async()=>{
  const {createServer}=await import('node:net');const server=createServer();await new Promise<void>(resolve=>server.listen(0,'127.0.0.1',resolve));const port=(server.address() as {port:number}).port;await new Promise<void>(resolve=>server.close(()=>resolve()));
  const dir=await mkdtemp(join(tmpdir(),'sqlite-cli-')),workspace=join(dir,'workspace'),db=join(dir,'private','embedded.sqlite');
  const {mkdir}=await import('node:fs/promises');await mkdir(workspace);
  const origin='http://127.0.0.1:'+port,owner=randomUUID(),config=join(dir,'embed.json'),registry=join(dir,'execution.json');
  await writeFile(config,JSON.stringify({publicOrigin:origin,clients:[],anonymous:{parentOrigins:[origin],executionOwners:[owner]}}));
  await writeFile(registry,JSON.stringify([{ownerId:owner,serviceIdentity:'cli-test',url:'http://127.0.0.1:59999',authorization:'private-cli-service',isolation:'os-user',workspaceAccess:'shared-readonly'}]));
  const start=fileURLToPath(new URL('../../apps/server/src/start.ts',import.meta.url));
  let processRef:ReturnType<typeof spawn>|undefined;let stderr='';
  const launch=async()=>{stderr='';processRef=spawn(process.execPath,['--experimental-strip-types',start,'--embedded-only','--workspace',workspace],{env:{...process.env,HARNESS_HTTP_PORT:String(port),HARNESS_EMBED_CONFIG:config,HARNESS_EXECUTION_REGISTRY:registry,HARNESS_SQLITE_PATH:db,HARNESS_PUBLIC_ORIGIN:origin,HARNESS_MYSQL_DATABASE:'',HARNESS_MYSQL_USER:'',HARNESS_MYSQL_PASSWORD:'',HARNESS_INSTITUTION_ADAPTER_URL:''},stdio:['ignore','ignore','pipe']});processRef.stderr!.on('data',c=>stderr+=c);for(let i=0;i<150;i++){try{const response=await fetch(origin+'/api/v1/health');if(response.ok){assert.equal((await response.json()).data.storage,'sqlite');return;}}catch{}if(processRef.exitCode!==null)throw Error(stderr);await new Promise(resolve=>setTimeout(resolve,50));}throw Error('CLI_START_TIMEOUT '+stderr);};
  const stop=async()=>{if(!processRef||processRef.exitCode!==null||processRef.signalCode!==null)return;const exited=new Promise<void>(resolve=>processRef!.once('exit',()=>resolve()));processRef.kill('SIGTERM');await exited;};
  try{
    await launch();const opened=await fetch(origin+'/api/v1/embed/anonymous',{method:'POST',headers:{Origin:origin,'Content-Type':'application/json'},body:'{}'});assert.equal(opened.status,200);const capability=(await opened.json()).data;
    await stop();await launch();const restored=await fetch(origin+'/api/v1/embed/session',{headers:{Origin:origin,Authorization:'Bearer '+capability.token}});assert.equal(restored.status,200);assert.equal((await restored.json()).data.state,'NEW');
  }finally{await stop();await rm(dir,{recursive:true,force:true});}
});
