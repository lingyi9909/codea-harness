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
  let skillIDs:string[]=[],skillLookupFailure=false,skillDirectory=root;
  let failure=0,formMode='legacy',formTitle='选择下一步';
  const formID='frm_example';
  const fields=[
    {key:'mode',type:'string',options:[{value:'trace',label:'日志',metadata:'opaque-field'}],default:'trace'},
    {key:'sources',type:'multiselect',options:[{value:'web',label:'网页'}],required:true,minItems:1},
    {key:'count',type:'integer',minimum:1,when:[{key:'mode',op:'eq',value:'trace'}]},
    {key:'confirm',type:'boolean',required:true},
    {key:'ack',type:'external',url:'https://example.com/'},
    {key:'token',type:'string',hidden:true,default:'server-only-default'},
    {key:'extra',type:'string',when:[{key:'token',op:'eq',value:'server-only-default'},{key:'sources',op:'eq',value:'web'}]}];

  const peer=createServer(async(req,res)=>{
    const path=new URL(req.url!,'http://localhost').pathname;
    let bytes='';for await(const chunk of req)bytes+=chunk;
    const body=bytes?JSON.parse(bytes):{};
    received.push({path,body});
    res.setHeader('content-type','application/json');
    const data=(value:unknown)=>res.end(JSON.stringify({data:value}));
    if(path==='/api/skill'){
      assert.equal(new URL(req.url!,'http://localhost').searchParams.get('location[directory]'),root);
      if(skillLookupFailure){res.statusCode=503;return data({});}
      return res.end(JSON.stringify({location:{directory:skillDirectory},data:skillIDs.map(id=>({id,name:'log-query',content:'private fixture instructions'}))}));
    }
    if(path==='/api/model')return res.end(JSON.stringify({data:[
      {id:'corp-alias',modelID:'vendor-model',name:'Private',providerID:'corp',enabled:true,headers:{authorization:'fixture-secret'}},
      {id:'inactive',providerID:'corp',name:'Inactive',enabled:false}]}));
    if(path==='/api/info')return res.end(JSON.stringify({version:'2.0.27',pid:process.pid}));
    if(path==='/api/session'&&failure){res.statusCode=failure;return data({message:'sensitive upstream detail'});}
    if(path==='/api/session'&&req.method==='GET'){
      const query=new URL(req.url!,'http://localhost').searchParams;
      assert.equal(query.get('directory'),root);
      const item={id,title:session.title??'旧会话',location:{directory:root},time:{updated:17},metadata:{harnessOwner:'local-workbench',harnessScenario:'direct-chat'}};
      const page=query.get('cursor')?[{...item,id:'ses_older'}]:[item,
        {...item,id:'ses_other',metadata:{...item.metadata,harnessOwner:'other'}},
        {...item,id:'ses_logs',metadata:{...item.metadata,harnessScenario:'log-inspection'}},
        {...item,id:'ses_folder',location:{directory:join(root,'another')}},
        {...item,id:'ses_tui',metadata:{}},{...item,id:'ses_child',parentID:id}];
      return res.end(JSON.stringify({data:page,cursor:query.get('cursor')?{}:{next:'next-page'}}));
    }
    if(path==='/api/session'){session={id,title:body.title,location:body.location,metadata:body.metadata};return data(session);}
    if(path===base)return data(session);
    if(path===base+'/prompt')return data({id:'msg_1',sessionID:id});
    if(path===base+'/message')return res.end(JSON.stringify({cursor:{},data:[{id:'msg_2',type:'assistant',content:[
      {type:'reasoning',text:'client-visible fixture note',state:{opaque:'private reasoning'}},{type:'text',text:'示例回执'},{type:'tool',name:'edit',state:{status:'completed',input:{path:'notes.txt',apiKey:'fixture-private-key'},content:[{type:'text',text:'文件已更新'}],metadata:{raw:'internal-only'}}}],snapshot:{files:['notes.txt']}}]}));
    if(path===base+'/form'&&formMode!=='legacy')return data([
      {id:'other',sessionID:'unowned',title:'other session',fields},
      ...(formMode==='pending'?[{id:formID,sessionID:id,title:formTitle,fields,metadata:{private:'opaque-form'}}]:[])]);
    if(path===base+'/form/'+formID+'/reply'||(path===base+'/form/'+formID&&req.method==='DELETE')){
      formMode='done';res.statusCode=204;return res.end();}
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
    assert.equal((await app.inject({url:'/api/v1/chat/access-log',headers})).statusCode,404);
    const create=await app.inject({method:'POST',url:'/api/v1/chat/sessions',headers,payload:{}});
    assert.equal(create.statusCode,201,create.body);assert.equal(create.json().data.directory,root);
    const history=(payload:object)=>app.inject({method:'POST',url:'/api/v1/chat/history',headers,payload});
    const historyRequest={directory:root,scenario:'direct-chat'};
    const firstPage=await history(historyRequest);
    assert.equal(firstPage.statusCode,200,firstPage.body);
    assert.deepEqual(firstPage.json().data,{items:[{id,title:'Code Harness · 直接对话',updated:17}],next:'next-page'});
    assert.deepEqual((await history({...historyRequest,cursor:'next-page'})).json().data,{items:[{id:'ses_older',title:'Code Harness · 直接对话',updated:17}]});
    assert.equal((await history({...historyRequest,directory:tmpdir()})).statusCode,403);
    const options=await app.inject({method:'POST',url:'/api/v1/chat/models',headers});
    assert.deepEqual(options.json().data,[{id:'corp-alias',name:'Private',providerID:'corp'}]);
    const model={providerID:'corp',id:'corp-alias'};
    const title='阅读项目说明并梳理启动步骤';
    const createWith=(id:string)=>app.inject({method:'POST',url:'/api/v1/chat/sessions',headers,payload:{title,model:{...model,id}}});
    assert.equal((await createWith('inactive')).statusCode,409);
    const titled=await createWith(model.id);
    assert.equal(titled.statusCode,201);assert.equal(titled.json().data.title,title);
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
    assert.equal(view.json().data.title,title);
    assert.equal((await history(historyRequest)).json().data.items[0].title,title);
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
    assert.deepEqual(view.json().data.forms,[]);
    formMode='pending';
    const pendingView=await app.inject({url,headers});
    const [form]=pendingView.json().data.forms;
    assert.equal(pendingView.json().data.forms.length,1);
    assert.ok(!pendingView.body.includes('opaque-field')&&!pendingView.body.includes('opaque-form')&&!pendingView.body.includes('server-only-default'));
    const submit={requestId:formID,revision:form.revision,decision:'reply',answer:{mode:'trace',sources:['web'],count:2,confirm:false,ack:true,extra:'visible'}};
    assert.equal((await post('/form',{...submit,requestId:'other'})).statusCode,409);
    assert.equal((await post('/form',{...submit,answer:{...submit.answer,count:0}})).statusCode,400);
    assert.equal((await post('/form',{...submit,answer:{...submit.answer,ack:false}})).statusCode,400);
    formTitle='更新后的问题';
    assert.equal((await post('/form',submit)).statusCode,409);
    formTitle='选择下一步';
    assert.equal((await post('/form',submit)).statusCode,200);
    assert.deepEqual(received.find(r=>r.path===base+'/form/'+formID+'/reply')?.body,{answer:{...submit.answer,token:'server-only-default'}});
    assert.equal((await post('/form',submit)).statusCode,409);
    formMode='pending';
    assert.equal((await post('/form',{requestId:formID,revision:form.revision,decision:'cancel'})).statusCode,200);
    assert.equal(formMode,'done');
    const logCreate=()=>app.inject({method:'POST',url:'/api/v1/chat/sessions',headers,payload:{scenario:'log-inspection'}});
    assert.ok(!received.some(r=>r.path==='/api/skill'));
    assert.ok(calls.every(call=>!('skills' in call.body)));
    assert.equal((await post('/messages',{text:'日志页',scenario:'log-inspection'})).statusCode,409);
    assert.equal((await logCreate()).json().error.code,'LOG_QUERY_SKILL_MISSING');
    skillIDs=['intranet-skill-7'];
    assert.equal((await logCreate()).statusCode,201);
    const ask=()=>post('/messages',{text:'查订单链路',files:['notes.txt']});
    assert.equal((await ask()).statusCode,202);
    const promptCalls=()=>received.filter(r=>r.path===base+'/prompt');
    assert.deepEqual(promptCalls().at(-1)?.body.skills,[{id:'intranet-skill-7'}]);
    assert.deepEqual(promptCalls().at(-1)?.body.files,calls[0].body.files);
    skillIDs=['updated-id'];
    assert.equal((await ask()).statusCode,202);
    assert.deepEqual(promptCalls().at(-1)?.body.skills,[{id:'updated-id'}]);
    skillIDs=[];
    assert.equal((await ask()).json().error.code,'LOG_QUERY_SKILL_MISSING');
    skillIDs=['one','two'];
    assert.equal((await ask()).json().error.code,'LOG_QUERY_SKILL_AMBIGUOUS');
    skillDirectory=tmpdir();
    assert.equal((await ask()).json().error.code,'LOG_QUERY_CATALOG_UNAVAILABLE');
    skillLookupFailure=true;
    assert.equal((await ask()).json().error.code,'LOG_QUERY_CATALOG_UNAVAILABLE');
    session.metadata={harnessScenario:'code-review'};
    assert.equal((await post('/form',submit)).statusCode,404);
    assert.equal((await post('/messages',{text:'hello'})).statusCode,404);
    assert.equal(received.filter(r=>r.path===base+'/prompt').length,4);
  }finally{await app.close();await new Promise<void>(ok=>peer.close(()=>ok()));await rm(root,{recursive:true,force:true});}
});
