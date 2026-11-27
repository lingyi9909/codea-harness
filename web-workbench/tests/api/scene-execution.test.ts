import {it} from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {mkdtemp,mkdir,writeFile,rm,realpath} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {createApp} from '../../apps/server/src/app.ts';
import {LocalAuth} from '../../apps/server/src/auth.ts';
import {defaultSceneConfiguration} from '../../apps/server/src/scene-config.ts';

it('routes each configured subscene through its own native executor and directory',async t=>{
  const root=await realpath(await mkdtemp(join(tmpdir(),'scene-entry-')));
  const dir=await realpath(await mkdtemp(join(tmpdir(),'scene-project-')));
  t.after(()=>Promise.all([rm(root,{recursive:true,force:true}),rm(dir,{recursive:true,force:true})]));
  const conf=join(root,'.workbench');
  await mkdir(conf,{recursive:true});
  await writeFile(join(conf,'scenarios.json'),JSON.stringify({version:1,scenes:{
    logs:{directory:dir,executor:{type:'skill',name:'log-query'}},
    message:{directory:dir,executor:{type:'subagent',name:'message-checker'}},
    data:{directory:dir,executor:{type:'command',name:'analyse-data'}},
  }}));
  const observed:{url:string;body:any}[]=[];
  const sessions=new Map<string,any>();
  let created=0,skillName='log-query';
  const peer=createServer(async(req,res)=>{
    let input='';for await(const chunk of req)input+=chunk;
    const body=input?JSON.parse(input):{};
    const url=new URL(req.url!,'http://localhost');
    observed.push({url:url.pathname,body});
    res.setHeader('content-type','application/json');
    const send=(data:unknown)=>res.end(JSON.stringify({data}));
    if(url.pathname==='/api/info')return send({version:'2.0.27',pid:process.pid});
    if(url.pathname==='/api/skill')return res.end(JSON.stringify({
      location:{directory:dir},data:[{id:'skill-1',name:skillName}]}));
    if(url.pathname==='/api/agent')return send([{id:'message-checker',name:'message-checker',mode:'subagent'}]);
    if(url.pathname==='/api/command')return send([{name:'analyse-data',template:'Analyse $ARGUMENTS'}]);
    if(url.pathname==='/api/session'&&req.method==='POST'){
      const id='ses_scene_'+(++created);
      const record={id,title:body.title,location:body.location,metadata:body.metadata};
      sessions.set(id,record);return send(record);
    }
    const session=url.pathname.match(/^\/api\/session\/(ses_scene_\d+)$/)?.[1];
    if(session)return send(sessions.get(session));
    if(url.pathname.includes('/instructions/entries/')){res.statusCode=204;return res.end();}
    if(url.pathname.match(/^\/api\/session\/ses_scene_\d+\/prompt$/))return send({id:'prompt-1'});
    if(url.pathname.match(/^\/api\/session\/ses_scene_\d+\/command$/)){res.statusCode=204;return res.end();}
    res.statusCode=404;return send({});
  });
  await new Promise<void>(done=>peer.listen(0,'127.0.0.1',done));
  t.after(()=>new Promise<void>(done=>peer.close(()=>done())));
  const address=peer.address();
  assert.ok(address&&typeof address==='object');
  const previous=process.env.HARNESS_OPENCODE_URL;
  process.env.HARNESS_OPENCODE_URL='http://127.0.0.1:'+address.port;
  t.after(()=>{if(previous===undefined)delete process.env.HARNESS_OPENCODE_URL;else process.env.HARNESS_OPENCODE_URL=previous;});
  const port=4188,host='127.0.0.1:'+port,origin='http://'+host;
  const app=await createApp({auth:new LocalAuth('unused',port),localOnly:true,workspace:root});
  t.after(()=>app.close());
  const signed=await app.inject({method:'POST',url:'/api/v1/auth/local-session',headers:{host,origin},
    payload:{name:'Developer',phone:'13800000000'}});
  assert.equal(signed.statusCode,200,signed.body);
  const headers={host,origin,cookie:String(signed.headers['set-cookie']).split(';')[0],
    'x-csrf-token':signed.json().data.csrfToken};
  const create=async(scenario:string,directory?:string)=>app.inject({method:'POST',url:'/api/v1/chat/sessions',headers,
    payload:{scenario,...(directory?{directory}:{})}});
  const send=async(id:string,scenario:string,text:string)=>app.inject({method:'POST',
    url:'/api/v1/chat/sessions/'+id+'/messages',headers,payload:{scenario,text}});
  const wrong=await create('direct-chat',dir);
  assert.equal(wrong.statusCode,403,wrong.body);
  assert.equal(created,0,'unscoped direct chat is rejected before native admission');
  const log=await create('log-inspection');
  assert.equal(log.statusCode,201,log.body);
  assert.equal(log.json().data.directory,dir);
  const logResult=await send(log.json().data.id,'log-inspection','查找 Trace ID ABC');
  assert.equal(logResult.statusCode,202,logResult.body);
  assert.deepEqual(observed.filter(row=>row.url.endsWith('/prompt')).at(-1)?.body.skills,[{id:'skill-1'}]);

  const msg=await create('message-inspection');
  assert.equal(msg.statusCode,201,msg.body);
  const msgResult=await send(msg.json().data.id,'message-inspection','核对报文流');
  assert.equal(msgResult.statusCode,202,msgResult.body);
  const agentPrompt=observed.filter(row=>row.url.endsWith('/prompt')).at(-1)?.body;
  assert.ok(agentPrompt.text.startsWith('@message-checker 核对报文流'));
  assert.deepEqual(agentPrompt.agents,[{name:'message-checker',mention:{start:0,end:16,text:'@message-checker'}}]);

  const data=await create('data-analysis');
  assert.equal(data.statusCode,201,data.body);
  const dataResult=await send(data.json().data.id,'data-analysis','汇总数据趋势');
  assert.equal(dataResult.statusCode,202,dataResult.body);
  const command=observed.filter(row=>/^\/api\/session\/ses_scene_\d+\/command$/.test(row.url)).at(-1)?.body;
  assert.equal(command.name,'analyse-data');
  assert.match(command.text,/汇总数据趋势/);
  assert.equal(observed.filter(row=>/^\/api\/session\/ses_scene_\d+\/command$/.test(row.url)).length,1);

  const fake=await send(log.json().data.id,'data-analysis','wrong scene');
  assert.equal(fake.statusCode,409);
  skillName='not-the-configured-skill';
  const missing=await send(log.json().data.id,'log-inspection','再查一次');
  assert.equal(missing.statusCode,409);
  assert.equal(missing.json().error.code,'SCENE_EXECUTOR_NOT_FOUND');

  // Stock /models is an interactive model picker, never a custom task command.
  await writeFile(join(conf,'scenarios.json'),JSON.stringify(defaultSceneConfiguration(root)));
  const callsBefore=observed.filter(row=>row.url.includes('/ses_scene_')&&row.url.endsWith('/command')).length;
  const modelsScene=await create('message-inspection');
  assert.equal(modelsScene.statusCode,201,modelsScene.body);
  const modelsSend=await send(modelsScene.json().data.id,'message-inspection','排查报文');
  assert.equal(modelsSend.statusCode,202,modelsSend.body);
  assert.equal(observed.filter(row=>row.url.includes('/ses_scene_')&&row.url.endsWith('/command')).length,callsBefore);
  assert.equal(observed.filter(row=>row.url.endsWith('/prompt')).at(-1)?.body.text,'排查报文');
  const builtinData=await create('data-analysis');
  assert.equal(builtinData.statusCode,201,builtinData.body);
  const analysis=await send(builtinData.json().data.id,'data-analysis','分析数据');
  assert.equal(analysis.statusCode,202,analysis.body);
  assert.equal(observed.filter(row=>row.url.endsWith('/prompt')).at(-1)?.body.text,'分析数据');
});
