import {it} from 'node:test';
import assert from 'node:assert/strict';
import {mkdir,mkdtemp,writeFile,chmod,rm,readFile,realpath} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {chromium,expect} from '@playwright/test';
import {createApp} from '../../apps/server/src/app.ts';
import {LocalAuth} from '../../apps/server/src/auth.ts';

it('browser sends without a directory, starts installed CLI fixture and shows upstream failure',{timeout:90000},async()=>{
  const root=await mkdtemp(join(tmpdir(),'harness cli ')),file=join(root,'service.json');
  const win=process.platform==='win32',cli=join(root,win?'opencode.cmd':'opencode'),script=join(root,'fixture.cjs');
  const diagnosis=['## 排查结果','', '| 服务 | 耗时 |','| --- | ---: |','| 订单 | 120 ms |','',
    '> 已按 Trace ID 对齐。','','~~~json','{"count":2}','~~~','','- [x] 已核实时间区间'].join('\n');
  await writeFile(script,`
    const fs=require('node:fs');
    if(process.argv.slice(2).join(' ')!=='serve --service')process.exit(3);
    let output,generated=false,answered=false,permitted=false;
    const flag=process.env.HARNESS_OPENCODE_SERVICE_FILE;
    const until=async(name)=>{while(!fs.existsSync(flag+name))await new Promise(r=>setTimeout(r,20));};
    async function streamReply(){
      const emit=(type,ordinal,fields,id=type,sessionID='ses_logs_fixture')=>{
        output?.write('data: '+JSON.stringify({id,type,created:Date.now(),data:{sessionID,assistantMessageID:'live-msg',ordinal,...fields}})+'\\n\\n');
      };
      emit('session.reasoning.started',0,{});
      emit('session.reasoning.delta',0,{delta:'先核对'},'r1');
      emit('session.reasoning.delta',0,{delta:'先核对'},'r1');
      emit('session.reasoning.delta',0,{delta:'foreign-secret'},'foreign','other');
      await until('.next');
      emit('session.reasoning.delta',0,{delta:'时间线'},'r2');
      emit('session.reasoning.ended',0,{text:'先核对时间线',state:{opaque:'private-marker'}});
      emit('session.text.started',1,{});
      emit('session.text.delta',1,{delta:'已找到'},'t1');
      await until('.finish');
      emit('session.text.delta',1,{delta:'两条记录'},'t2');
      emit('session.text.ended',1,{text:'已找到两条记录'});
      generated=true;
    }
    let logSession;const logBase='/api/session/ses_logs_fixture';
    const server=require('node:http').createServer((req,res)=>{
      res.setHeader('Content-Type','application/json');
      if(req.url.split('?')[0]==='/api/model')return res.end(JSON.stringify({data:[
        {id:'intranet',providerID:'company',name:'Company fixture',enabled:true},
        {id:'analysis',providerID:'labs',name:'Analysis fixture',enabled:true,limit:{context:200000},variants:[{id:'high',headers:{token:'private-marker'}}]}]}));
      if(req.url==='/api/info')return res.end(JSON.stringify({pid:process.pid,version:'2.0.27'}));
      if(req.url==='/api/event'){
        res.setHeader('Content-Type','text/event-stream');res.flushHeaders();
        output=res;
        const progress=(sessionID,message)=>({type:'session.tool.progress',id:sessionID,created:Date.now(),data:{sessionID,metadata:{message}}});
        for(const e of [progress('other','cross-session-marker'),progress('ses_logs_fixture','已扫描两个文件')])
          res.write('data: '+JSON.stringify(e)+'\\n\\n');
        return;
      }
      if(req.url.split('?')[0]==='/api/skill'){
        const directory=new URL(req.url,'http://localhost').searchParams.get('location[directory]');
        return res.end(JSON.stringify({location:{directory},data:fs.existsSync(flag+'.skill')?[{id:'corp-logs-v1',name:'log-query'}]:[]}));
      }
      if(req.url===logBase)return res.end(JSON.stringify({data:logSession}));
      const save=(suffix,done)=>{let body='';req.on('data',s=>body+=s);req.on('end',()=>{
        fs.writeFileSync(flag+suffix,body);done(JSON.parse(body));res.writeHead(204);res.end();});};
      if(req.url===logBase+'/model')return save('.model',body=>{logSession.model=body.model;});
      if(req.url===logBase+'/form')return res.end(JSON.stringify({data:!answered&&fs.existsSync(flag+'.ask')?[
        {id:'frm_trace',sessionID:'ses_logs_fixture',title:'选择排查范围',fields:[
          {key:'scope',title:'检查哪些记录？',type:'string',required:true,options:[
            {value:'trace',label:'当前 Trace',description:'只检查本次请求关联的日志'},
            {value:'service',label:'整个服务',description:'检查时间段内同服务的所有请求'}]},
          {key:'service',title:'服务名称',type:'string',required:true}]}]:[]}));
      if(req.url===logBase+'/form/frm_trace/reply')return save('.answer',()=>{answered=true;});
      if(req.url===logBase+'/permission')return res.end(JSON.stringify({data:answered&&!permitted?[
        {id:'per_trace',sessionID:'ses_logs_fixture',action:'read',resources:['logs/orders.log']}]:[]}));
      if(req.url===logBase+'/permission/per_trace/reply')return save('.permission',()=>{permitted=true;});
      if(req.url===logBase+'/prompt'){
        req.setEncoding('utf8');let body='';req.on('data',s=>body+=s);req.on('end',()=>{
          if(!fs.existsSync(flag+'.prompt'))void streamReply();
          fs.writeFileSync(flag+'.prompt',body);
          res.end(JSON.stringify({data:{id:'msg_fixture',sessionID:'ses_logs_fixture'}}));
        });return;
      }
      if(req.url.startsWith(logBase+'/message'))return res.end(JSON.stringify({cursor:{},data:[...(generated?[{type:'assistant',id:'live-msg',finish:'stop',content:[
        {type:'reasoning',text:'先核对时间线'},{type:'text',text:'已找到两条记录'}]}]:[]),{type:'assistant',id:'assistant-log',content:[
        {type:'text',text:'开始核对匹配行。'},{type:'reasoning',text:'客户端可见的测试片段',state:{opaque:'private-marker'}},
        {type:'tool',name:'grep',state:{status:'completed',input:{pattern:'abc123'},content:[{type:'text',text:'命中第 2 行'}]}},
        {type:'text',text:'检查结束。'},{type:'text',text:${JSON.stringify(diagnosis)}}]}]}));
      if(req.url===logBase+'/inbox')return res.end(JSON.stringify({data:[]}));
      if(req.url==='/api/session/active')return res.end(JSON.stringify({data:fs.existsSync(flag+'.prompt')&&!generated?{ses_logs_fixture:{}}:{}}));
      if(req.url==='/api/session'&&req.method==='POST'){
        req.setEncoding('utf8');let body='';
        req.on('data',s=>body+=s);req.on('end',()=>{
          fs.writeFileSync(process.env.HARNESS_OPENCODE_SERVICE_FILE+'.body',body);
          const create=JSON.parse(body);
          if(create.metadata.harnessScenario==='log-inspection'){
            logSession={...create,id:'ses_logs_fixture'};res.end(JSON.stringify({data:logSession}));return;
          }
          res.writeHead(503);res.end('{}');
        });return;
      }
      res.writeHead(404);res.end('{}');
    });
    server.listen(0,'127.0.0.1',()=>fs.writeFileSync(process.env.HARNESS_OPENCODE_SERVICE_FILE,
      JSON.stringify({url:'http://127.0.0.1:'+server.address().port,pid:process.pid,version:'2.0.27'})));
    setTimeout(()=>process.exit(0),90000);
  `);
  await writeFile(cli,win?'@echo off\r\n"'+process.execPath+'" "%~dp0fixture.cjs" %*\r\n':'#!/bin/sh\nexec "'+process.execPath+'" "'+script+'" "$@"\n');
  await chmod(cli,0o755);
  const browser=await chromium.launch(process.env.HARNESS_TEST_CHROME?{channel:'chrome'}:{});
  const cwd=process.cwd(),saved={...process.env};
  process.chdir(root);
  process.env.PATH=root;
  process.env.HARNESS_OPENCODE_SERVICE_FILE=file;
  delete process.env.HARNESS_CHAT_ROOTS;delete process.env.HARNESS_REPOSITORY_ROOTS;delete process.env.HARNESS_OPENCODE_URL;
  const port=31881;
  const app=await createApp({auth:new LocalAuth('pair-fixture',port),autoOpenCode:true});
  async function recorded(suffix:string){
    const text=await readFile(file+suffix,'utf8').catch((e:NodeJS.ErrnoException)=>{if(e.code!=='ENOENT')throw e;return '';});
    return text?JSON.parse(text):{};
  }
  try{
    await app.listen({host:'127.0.0.1',port});
    const page=await browser.newPage();
    await page.goto('http://127.0.0.1:'+port);
    await page.getByLabel('一次性配对码').fill('pair-fixture');
    await page.getByRole('button',{name:'连接本地工作台'}).click();
    await page.getByRole('navigation').getByRole('button',{name:'直接对话',exact:true}).click();
    await expect(page.getByRole('heading',{name:'后台访问日志'})).toBeVisible();
    await expect(page.getByPlaceholder('输入绝对路径')).toHaveCount(0);
    const project=join(root,'业务项目');
    await mkdir(project);
    await page.getByRole('button',{name:'更改工作目录'}).click();
    await page.getByLabel('项目文件夹路径').fill(join(project,'missing'));
    await page.getByRole('button',{name:'使用此目录'}).click();
    await expect(page.getByRole('alert')).toContainText('找不到');
    await page.getByLabel('项目文件夹路径').fill(project);
    await page.getByRole('button',{name:'使用此目录'}).click();
    await expect(page.locator('.project-path')).toHaveText(await realpath(project));
    await page.reload();
    await expect(page.locator('.project-path')).toHaveText(await realpath(project));
    await page.locator('.composer-actions').getByLabel('选择模型').click();
    const search=page.getByRole('combobox',{name:'搜索模型'});
    await search.fill('company');
    await expect(page.getByRole('option',{name:/Analysis fixture/})).toHaveCount(0);
    await expect(page.getByRole('option',{name:/Company fixture/})).toBeVisible();
    await search.press('Enter');
    const composer=page.getByLabel('对话内容');
    await composer.fill('第一行');
    await composer.press('Shift+Enter');
    await expect(composer).toHaveValue('第一行\n');
    let sends=0;
    page.on('request',req=>{if(req.method()==='POST'&&new URL(req.url()).pathname==='/api/v1/chat/sessions')sends++;});
    await composer.dispatchEvent('keydown',{key:'Enter',isComposing:true});
    await composer.dispatchEvent('keydown',{key:'Enter',keyCode:229});
    await composer.press('End');
    await composer.pressSequentially('fixture');
    await expect(composer).toHaveValue('第一行\nfixture');
    assert.equal(sends,0);
    await composer.press('Enter');
    await expect(page.locator('.access-log')).toContainText('OpenCode · 503',{timeout:60000});
    await expect(page.locator('.access-log')).toContainText('工作台 · 502');
    assert.equal(sends,1);
    const created=JSON.parse(await readFile(file+'.body','utf8'));
    assert.deepEqual(created.model,{id:'intranet',providerID:'company'});
    assert.equal(created.location.directory,await realpath(project));
    await page.getByRole('navigation').getByRole('button',{name:'日志排查',exact:true}).click();
    const submit=page.getByRole('button',{name:'开始排查'});
    await expect(submit).toBeDisabled();
    await page.getByLabel('请求入参').fill('{"order":"A15"}');
    const quarter=page.getByRole('button',{name:'最近 15 分钟'});
    await quarter.click();
    await expect(quarter).toHaveAttribute('aria-pressed','true');
    await expect(page.locator('.range-footer')).toContainText('15 分钟');
    await page.evaluate(()=>{
      const native=HTMLInputElement.prototype.showPicker;
      HTMLInputElement.prototype.showPicker=function(){
        native.call(this);this.dataset.pickerCalls=String(Number(this.dataset.pickerCalls??0)+1);
      };
    });
    for(const label of ['开始时间','结束时间']){
      const input=page.getByLabel(label,{exact:true}),box=await input.boundingBox();
      assert.ok(box);
      for(const [index,x] of [8,box.width/2,box.width-10].entries()){
        await input.click({position:{x,y:box.height/2}});
        await expect(input).toHaveAttribute('data-picker-calls',String(index+1));
        await page.keyboard.press('Escape');
      }
    }
    await page.getByLabel('开始时间').fill('2026-11-12T10:00');
    await page.getByLabel('结束时间').fill('2026-11-12T11:00');
    await page.getByLabel('Trace ID',{exact:true}).fill('abc123');
    await page.getByLabel('关键字',{exact:true}).fill('timeout');
    await page.getByText('日志来源与级别',{exact:true}).click();
    await page.getByLabel('导入本地日志').setInputFiles({name:'service.log',mimeType:'text/plain',buffer:Buffer.from([
      '2026-11-12 09:59:59 ERROR abc123 timeout before-window','2026-11-12 10:20:00 ERROR abc123 timeout included',
      '2026-11-12 10:20:00 INFO abc123 timeout wrong-level','2026-11-12 11:00:00 ERROR abc123 timeout at-end',
      '2026-11-12 10:20:00 ERROR def timeout wrong-id'].join('\n'))});
    await page.getByLabel('日志级别').selectOption('ERROR');
    await expect(page.locator('.log-query pre')).toHaveText('2  2026-11-12 10:20:00 ERROR abc123 timeout included');
    await page.getByLabel('对话内容').fill('检查超时原因');
    await submit.click();
    await expect(page.getByRole('alert')).toContainText('未加载自定义 Skill「log-query」');
    await expect(page.getByLabel('对话内容')).toHaveValue('检查超时原因');
    await writeFile(file+'.skill','1');
    await submit.click();
    await expect(page.getByRole('button',{name:'调整排查条件'})).toBeVisible();
    const running=page.getByRole('button',{name:'正在执行 · 中断执行'});
    await expect(running).toBeVisible();
    await expect(running).toHaveCSS('background-color','rgb(170, 39, 46)');
    const feed=page.locator('.chat-feed');
    await expect(feed).toContainText('已扫描两个文件');
    await expect(feed.locator('.execution-tool')).toContainText('命中第 2 行');
    await feed.getByText('调用参数',{exact:true}).click();
    await expect(feed.locator('.execution-tool pre')).toContainText('abc123');
    await expect(feed).not.toContainText('cross-session-marker');
    const thought=feed.locator('.chat-turn').filter({hasText:'检查结束。'}).getByRole('button',{name:/思考过程/});
    await expect(thought).toBeVisible();
    await expect(feed).not.toContainText('客户端可见的测试片段');
    await thought.click();await expect(feed).toContainText('客户端可见的测试片段');
    await expect(feed).not.toContainText('private-marker');
    await thought.click();await expect(feed).not.toContainText('客户端可见的测试片段');
    await expect(feed.getByText('先核对',{exact:true})).toBeVisible();
    await expect(feed).not.toContainText('已找到');
    await expect(feed).not.toContainText('foreign-secret');
    await writeFile(file+'.next','1');
    await expect(feed.getByText('先核对时间线',{exact:true})).toBeVisible();
    await expect(feed.getByText('已找到',{exact:true})).toBeVisible();
    await expect(feed).not.toContainText('两条记录');
    await writeFile(file+'.finish','1');
    await expect(feed.getByText('已找到两条记录',{exact:true})).toBeVisible();
    await page.reload();
    await expect(feed.getByText('已找到两条记录',{exact:true})).toHaveCount(1);
    await expect(running).toHaveCount(0);
    await expect(feed.getByRole('heading',{name:'排查结果'})).toBeVisible();
    await expect(feed.getByRole('cell',{name:'120 ms'})).toBeVisible();
    await expect(feed.locator('pre code.language-json')).toContainText('"count"');
    await page.getByRole('button',{name:'选择模型'}).click();
    await page.getByRole('combobox',{name:'搜索模型'}).fill('analysis');
    await page.getByRole('option',{name:/Analysis fixture/}).click();
    await page.getByRole('combobox',{name:'模型变体'}).selectOption('high');
    await page.getByRole('button',{name:'使用此模型'}).click();
    await expect.poll(async()=>(await recorded('.model')).model).toEqual({id:'analysis',providerID:'labs',variant:'high'});
    await writeFile(file+'.ask','1');
    await expect(page.getByText('选择排查范围',{exact:true})).toBeVisible();
    await page.getByRole('radio',{name:/当前 Trace/}).check();
    await page.getByLabel('服务名称',{exact:false}).fill('orders');
    await page.getByLabel('对话内容').fill('保存这份草稿');
    await expect(page.getByRole('button',{name:/^发送/})).toBeDisabled();
    await expect(page).toHaveTitle(/等待你回应/);
    await page.getByRole('button',{name:'提交回答'}).click();
    await expect.poll(async()=>(await recorded('.answer')).answer).toEqual({scope:'trace',service:'orders'});
    await expect(page.getByText('logs/orders.log',{exact:true})).toBeVisible();
    await page.getByRole('button',{name:'仅本次允许'}).click();
    await expect.poll(async()=>(await recorded('.permission')).decision).toBe('once');
    await expect(page.getByRole('button',{name:/^发送/})).toBeEnabled();
    await expect(page.getByLabel('对话内容')).toHaveValue('保存这份草稿');
    assert.deepEqual((await recorded('.prompt')).skills,[{id:'corp-logs-v1'}]);
    const request=JSON.parse(await readFile(file+'.prompt','utf8')).text;
    for(const value of ['A15','检查超时原因','abc123','timeout','included','ERROR'])assert.ok(request.includes(value));
    for(const value of ['wrong-level','at-end','before-window','wrong-id'])assert.ok(!request.includes(value));
    await page.getByLabel('对话内容').fill('下一步怎么查？');
    await page.getByLabel('对话内容').press('Enter');
    await expect.poll(async()=>JSON.parse(await readFile(file+'.prompt','utf8')).text).toBe('下一步怎么查？');
    assert.deepEqual((await recorded('.prompt')).skills,[{id:'corp-logs-v1'}]);
    assert.deepEqual(await page.evaluate(()=>[sessionStorage.getItem('harness.chat'),sessionStorage.getItem('harness.logs')]),['','ses_logs_fixture']);
  }finally{
    process.chdir(cwd);
    for(const key of Object.keys(process.env))if(!(key in saved))delete process.env[key];
    Object.assign(process.env,saved);
    await browser.close();await app.close();
    const registration=await readFile(file,'utf8').catch(()=>'');
    if(registration){try{process.kill(JSON.parse(registration).pid);}catch{}}
    await rm(root,{recursive:true,force:true,maxRetries:12,retryDelay:150});
  }
});
