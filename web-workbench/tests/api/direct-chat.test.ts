import {it} from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {mkdtemp,realpath,writeFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {pathToFileURL} from 'node:url';
import {createApp} from '../../apps/server/src/app.ts';
import {LocalAuth} from '../../apps/server/src/auth.ts';

it('uses the official client against a scripted OpenCode HTTP peer; no model is invoked',async()=>{
  const root=await realpath(await mkdtemp(join(tmpdir(),'chat-contract-')));
  const id='ses_chat_fixture',per='per_fixture',base='/api/session/'+id;
  const received:Array<{path:string;body:Record<string,unknown>}>=[];
  let session:Record<string,unknown>={},pending=true;
  let failure=0;
  const peer=createServer(async(req,res)=>{
    const path=new URL(req.url!,'http://localhost').pathname;
    let bytes='';for await(const chunk of req)bytes+=chunk;
    const body=bytes?JSON.parse(bytes):{};
    received.push({path,body});
    res.setHeader('content-type','application/json');
    const data=(value:unknown)=>res.end(JSON.stringify({data:value}));
    if(path==='/api/model')return res.end(JSON.stringify({data:[
      {id:'corp-alias',modelID:'vendor-model',name:'Private',providerID:'corp',enabled:true,headers:{authorization:'fixture-secret'}},
      {id:'inactive',providerID:'corp',name:'Inactive',enabled:false}]}));
    if(path==='/api/info')return res.end(JSON.stringify({version:'2.0.27',pid:process.pid}));
    if(path==='/api/session'&&failure){res.statusCode=failure;return data({message:'sensitive upstream detail'});}
    if(path==='/api/session'){session={id,location:body.location,metadata:body.metadata};return data(session);}
    if(path===base)return data(session);
    if(path===base+'/prompt')return data({id:'msg_1',sessionID:id});
    if(path===base+'/message')return res.end(JSON.stringify({cursor:{},data:[{id:'msg_2',type:'assistant',content:[
      {type:'reasoning',text:'client-visible fixture note',state:{opaque:'private reasoning'}},{type:'text',text:'示例回执'},{type:'tool',name:'edit',state:{status:'completed',input:{path:'notes.txt',apiKey:'fixture-private-key'},content:[{type:'text',text:'文件已更新'}],metadata:{raw:'internal-only'}}}],snapshot:{files:['notes.txt']}}]}));
    if(path===base+'/inbox')return data([]);
    if(path==='/api/session/active')return data({});
    if(path===base+'/permission')return data(pending?[{id:per,sessionID:id,action:'edit',resources:['notes.txt']}]:[]);
    if(path===base+'/permission/'+per+'/reply'){pending=false;res.statusCode=204;return res.end();}
    if(path===base+'/interrupt')return res.end(JSON.stringify({interrupted:true}));
    res.statusCode=404;res.end();
  });
  await new Promise<void>(ok=>peer.listen(0,'127.0.0.1',ok));
  const address=peer.address();assert.ok(address&&typeof address==='object');
  delete process.env.HARNESS_OPENCODE_URL;
  process.env.HARNESS_OPENCODE_SERVICE_FILE=join(root,'registration.json');
  await writeFile(process.env.HARNESS_OPENCODE_SERVICE_FILE,JSON.stringify({pid:process.pid,version:'2.0.27',
    url:'http://127.0.0.1:'+address.port,password:'fixture-sensitive'}));
  process.env.HARNESS_CHAT_ROOTS=root;
  const app=await createApp({auth:new LocalAuth('fixture',4176)});
  try{
    await writeFile(join(root,'notes.txt'),'A fixture document');
    const host='127.0.0.1:4176',origin='http://'+host;
    const pair=await app.inject({method:'POST',url:'/api/v1/auth/session',headers:{host,origin},payload:{pairingCode:'fixture'}});
    const headers={host,origin,cookie:String(pair.headers['set-cookie']).split(';')[0],'x-csrf-token':pair.json().data.csrfToken};
    const outside=await app.inject({method:'POST',url:'/api/v1/chat/project',headers,payload:{directory:tmpdir()}});
    assert.equal(outside.statusCode,403);
    for(failure of [401,403,404,503]){
      const rejected=await app.inject({method:'POST',url:'/api/v1/chat/sessions',headers,payload:{directory:root}});
      assert.equal(rejected.json().error.code,'OPENCODE_HTTP_'+failure);
      assert.ok(!rejected.body.includes('sensitive upstream detail'));
    }
    failure=0;
    const logPath='/api/v1/chat/access-log';
    assert.equal((await app.inject({url:logPath,headers:{host,origin}})).statusCode,401);
    const logs=await app.inject({url:logPath,headers});
    const upstream=logs.json().data.find((r:{source:string;result:string})=>r.source==='OpenCode'&&r.result==='503');
    assert.ok(upstream);
    assert.ok(logs.json().data.some((r:{source:string;result:string;requestId:string})=>r.source==='工作台'&&r.result==='502'&&r.requestId===upstream.requestId));
    assert.ok(!logs.body.includes('fixture-sensitive')&&!logs.body.includes(root)&&!logs.body.includes('sensitive upstream detail'));
    const create=await app.inject({method:'POST',url:'/api/v1/chat/sessions',headers,payload:{}});
    assert.equal(create.statusCode,201,create.body);assert.equal(create.json().data.directory,root);
    const options=await app.inject({method:'POST',url:'/api/v1/chat/models',headers});
    assert.deepEqual(options.json().data,[{id:'corp-alias',name:'Private',providerID:'corp'}]);
    const model={providerID:'corp',id:'corp-alias'};
    const createWith=(id:string)=>app.inject({method:'POST',url:'/api/v1/chat/sessions',headers,payload:{model:{...model,id}}});
    assert.equal((await createWith('inactive')).statusCode,409);
    assert.equal((await createWith(model.id)).statusCode,201);
    assert.deepEqual(received.filter(r=>r.path==='/api/session').at(-1)?.body.model,model);
    const url='/api/v1/chat/sessions/'+id;
    const post=(suffix:string,payload:object)=>app.inject({method:'POST',url:url+suffix,headers,payload});
    assert.equal((await post('/messages',{text:'阅读这个文件',files:['notes.txt']})).statusCode,202);
    assert.equal((await post('/messages',{text:'继续整理文档'})).statusCode,202);
    const calls=received.filter(r=>r.path===base+'/prompt');
    assert.equal(calls.length,2);
    assert.deepEqual(calls[0].body.files,[{uri:pathToFileURL(join(root,'notes.txt')).href,name:'notes.txt'}]);
    assert.equal((await post('/messages',{text:'查看父目录',files:['../']})).statusCode,400);
    const view=await app.inject({method:'GET',url,headers});
    assert.equal(view.statusCode,200,view.body);
    assert.equal(view.body.includes('private reasoning'),false);
    assert.deepEqual(view.json().data.messages[0].files,['notes.txt']);
    assert.deepEqual(view.json().data.messages[0].tools,[{name:'edit',status:'completed'}]);
    const steps=view.json().data.messages[0].steps;
    assert.deepEqual(steps.map((s:{kind:string})=>s.kind),['thinking','text','tool']);
    assert.match(steps[2].input,/notes.txt/);
    assert.equal(steps[2].result,'文件已更新');
    for(const secret of ['fixture-private-key','internal-only'])assert.ok(!view.body.includes(secret));
    assert.ok(!view.body.includes('client-visible fixture note'));
    const expanded=await app.inject({url:url+'?message=msg_2&part=0',headers});
    assert.equal(expanded.json().data.messages[0].steps[0].text,'client-visible fixture note');
    assert.ok(!expanded.body.includes('private reasoning'));
    const permission=view.json().data.permissions[0];
    const answer={requestId:per,revision:permission.revision,decision:'once'};
    assert.equal((await post('/permission',{...answer,revision:'stale'})).statusCode,409);
    assert.equal((await post('/permission',answer)).statusCode,200);
    assert.equal((await post('/interrupt',{})).statusCode,200);
    session.metadata={harnessScenario:'code-review'};
    assert.equal((await post('/messages',{text:'hello'})).statusCode,404);
    assert.equal(received.filter(r=>r.path===base+'/prompt').length,2);
  }finally{await app.close();await new Promise<void>(ok=>peer.close(()=>ok()));await rm(root,{recursive:true,force:true});}
});
