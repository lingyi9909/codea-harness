import {it} from 'node:test';
import assert from 'node:assert/strict';
import {mkdir,mkdtemp,writeFile,chmod,rm,readFile,realpath} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {chromium,expect,type Page} from '@playwright/test';
import {createApp} from '../../apps/server/src/app.ts';
import {LocalAuth} from '../../apps/server/src/auth.ts';

async function observePaintedText(page:Page){
  return page.evaluateHandle(()=>{
      const seen=new Set<string>();
      new MutationObserver(()=>{
        for(const el of Array.from(document.querySelectorAll('.turn-answer p, .turn-reasoning p'))){
          const value=el.textContent??'';
          if(value)seen.add(value);
        }
      }).observe(document.body,{subtree:true,childList:true,characterData:true});
      return seen;
    });
}

async function login(page:Page){
  await expect(page.getByRole('heading',{name:'登录工作台'})).toBeVisible();
  await page.getByLabel('姓名',{exact:true}).fill('测试同事');
  await page.getByLabel('手机号',{exact:true}).fill('13800000000');
  await page.getByRole('button',{name:'登录工作台',exact:true}).click();
  await expect(page.getByRole('navigation')).toBeVisible();
  await expect(page.getByText('测试同事',{exact:true})).toBeVisible();
}

it('browser uses the startup directory, starts the CLI fixture and shows upstream failure',{timeout:90000},async()=>{
  const root=await mkdtemp(join(tmpdir(),'harness cli ')),file=join(root,'service.json');
  const win=process.platform==='win32',cli=join(root,win?'opencode.cmd':'opencode'),script=join(root,'fixture.cjs');
  const firstReasoning='先核对本次查询的起止日期和时区，再按时间顺序检查每个匹配项。';
  const finalAnswer='已找到两条记录，来自相同的服务，请对照这两条记录的请求时间。';
  const diagnosis=['## 排查结果','', '| 服务 | 耗时 |','| --- | ---: |','| 订单 | 120 ms |','',
    '> 已按 Trace ID 对齐。','','~~~json','{"count":2}','~~~','','- [x] 已核实时间区间'].join('\n');
  await writeFile(script,`
    const fs=require('node:fs');
    if(process.argv.slice(2).join(' ')!=='serve --service')process.exit(3);
    let output,generated=false,answered=false,permitted=false,verified=false;
    const flag=process.env.HARNESS_OPENCODE_SERVICE_FILE;
    const until=async(name)=>{while(!fs.existsSync(flag+name))await new Promise(r=>setTimeout(r,20));};
    async function streamReply(){
      const emit=(type,ordinal,fields,id=type,sessionID='ses_logs_fixture')=>{
        output?.write('data: '+JSON.stringify({id,type,created:Date.now(),data:{sessionID,assistantMessageID:'live-msg',ordinal,...fields}})+'\\n\\n');
      };
      emit('session.reasoning.started',0,{});
      emit('session.reasoning.delta',0,{delta:${JSON.stringify(firstReasoning)}},'r1');
      emit('session.reasoning.delta',0,{delta:${JSON.stringify(firstReasoning)}},'r1');
      emit('session.reasoning.delta',0,{delta:'foreign-secret'},'foreign','other');
      await until('.next');
      emit('session.reasoning.delta',0,{delta:'时间线'},'r2');
      emit('session.reasoning.ended',0,{text:${JSON.stringify(firstReasoning+'时间线')},state:{opaque:'private-marker'}});
      emit('session.text.started',1,{});
      emit('session.text.delta',1,{delta:'已找到'},'t1');
      await until('.finish');
      emit('session.text.delta',1,{delta:${JSON.stringify(finalAnswer.slice(3))}},'t2');
      emit('session.text.ended',1,{text:${JSON.stringify(finalAnswer)}});
      generated=true;
    }
    let logSession,secondLog;const secondBase='/api/session/ses_second_log';const logBase='/api/session/ses_logs_fixture';
    const server=require('node:http').createServer((req,res)=>{
      res.setHeader('Content-Type','application/json');
      if(req.url.split('?')[0]==='/api/model')return res.end(JSON.stringify({data:[
        {id:'intranet',providerID:'company',name:'Company fixture',enabled:true},
        {id:'analysis',providerID:'labs',name:'Analysis fixture',enabled:true,limit:{context:200000},variants:[{id:'high',headers:{token:'private-marker'}}]}]}));
      if(req.url==='/api/info')return res.end(JSON.stringify({pid:process.pid,version:'2.0.27'}));
      if(req.url==='/api/event'){
        res.setHeader('Content-Type','text/event-stream');res.flushHeaders();
        output=res;
        const progress=(sessionID,message)=>({type:'session.tool.progress',id:sessionID,created:Date.now(),data:{sessionID,assistantMessageID:'assistant-log',metadata:{message}}});
        for(const e of [progress('other','cross-session-marker'),progress('ses_logs_fixture','已扫描两个文件')])
          res.write('data: '+JSON.stringify(e)+'\\n\\n');
        return;
      }
      if(req.url.split('?')[0]==='/api/skill'){
        const directory=new URL(req.url,'http://localhost').searchParams.get('location[directory]');
        return res.end(JSON.stringify({location:{directory},data:fs.existsSync(flag+'.skill')?[{id:'corp-logs-v1',name:'log-query'}]:[]}));
      }
      if(req.method==='PUT'&&req.url.startsWith('/api/experimental/session/')&&req.url.endsWith('/instructions/entries/workbench-language')){
        req.resume();res.writeHead(204);return res.end();
      }
      if(req.url===logBase)return res.end(JSON.stringify({data:logSession}));
      const save=(suffix,done)=>{let body='';req.on('data',s=>body+=s);req.on('end',()=>{
        fs.writeFileSync(flag+suffix,body);done(JSON.parse(body));res.writeHead(204);res.end();});};
      if(req.url===secondBase&&req.method==='PATCH'){
        let body='';req.on('data',s=>body+=s);req.on('end',()=>{
          secondLog={...secondLog,title:JSON.parse(body).title};
          res.writeHead(204);res.end();
        });return;
      }
      if(req.url===secondBase&&req.method==='DELETE'){
        secondLog=null;res.writeHead(204);return res.end();
      }
      if(req.url===secondBase){
        if(!secondLog){res.writeHead(404);return res.end('{}');}
        return res.end(JSON.stringify({data:secondLog}));
      }
      if(req.url===secondBase+'/prompt'){
        let body='';req.on('data',s=>body+=s);req.on('end',()=>{
          fs.writeFileSync(flag+'.second-prompt',body);
          res.end(JSON.stringify({data:{id:'msg_second',sessionID:'ses_second_log'}}));
        });return;
      }
      if(req.url.startsWith(secondBase+'/message'))return res.end(JSON.stringify({cursor:{},data:[{id:'second-msg',type:'assistant',content:[{type:'text',text:'第二个会话的结果'}]}]}));
      if(req.url.startsWith(secondBase+'/'))return res.end(JSON.stringify({data:[]}));
      if(req.url===logBase+'/model')return save('.model',body=>{logSession.model=body.model;});
      if(req.url===logBase+'/form'&&permitted&&!verified)return res.end(JSON.stringify({data:[
        {id:'frm_code',sessionID:'ses_logs_fixture',title:'验证本次查询',fields:[{
          key:'q0',type:'string',title:'验证码',description:'输入测试页面给出的代码',required:true,custom:true,
          options:[{value:'手动输入',label:'手动输入'}]}]}]}));
      if(req.url===logBase+'/form/frm_code/reply')return save('.code',()=>{verified=true;});
      if(req.url===logBase+'/form')return res.end(JSON.stringify({data:!answered&&fs.existsSync(flag+'.ask')?[
        {id:'frm_trace',sessionID:'ses_logs_fixture',title:'选择排查范围',fields:[
          {key:'scope',title:'检查哪些记录？',type:'string',required:true,options:[
            {value:'trace',label:'当前 Trace',description:'只检查本次请求关联的日志'},
            {value:'service',label:'整个服务',description:'检查时间段内同服务的所有请求'}]},
          {key:'service',title:'服务名称',type:'string',required:true},
          {key:'note',title:'补充说明',type:'string',custom:true,options:[{value:'预设说明',label:'预设说明'}]}]}]:[]}));
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
        {type:'reasoning',text:${JSON.stringify(firstReasoning+'时间线')}},{type:'text',text:${JSON.stringify(finalAnswer)}}]}]:[]),{type:'assistant',id:'assistant-log',content:[
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
            if(logSession){secondLog={...create,id:'ses_second_log'};res.end(JSON.stringify({data:secondLog}));return;}
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
  const shots=join(cwd,'screenshots');
  await mkdir(shots,{recursive:true});
  process.chdir(root);
  process.env.PATH=root;
  process.env.HARNESS_OPENCODE_SERVICE_FILE=file;
  const project=join(root,'业务项目');
  await mkdir(project);
  const settingsDir=join(project,'.workbench');
  await mkdir(settingsDir,{recursive:true});
  await writeFile(join(settingsDir,'scenarios.json'),JSON.stringify({version:1,scenes:{
    logs:{directory:project,executor:{type:'skill',name:'log-query'}},
    message:{directory:project,executor:{type:'skill',name:'log-query'}},
    data:{directory:project,executor:{type:'skill',name:'log-query'}},
  }}));
  process.env.HARNESS_CHAT_ROOTS=project;
  delete process.env.HARNESS_REPOSITORY_ROOTS;delete process.env.HARNESS_OPENCODE_URL;
  const port=31881;
  const app=await createApp({auth:new LocalAuth('unused',port),localOnly:true,workspace:project,autoOpenCode:true});
  async function recorded(suffix:string){
    const text=await readFile(file+suffix,'utf8').catch((e:NodeJS.ErrnoException)=>{if(e.code!=='ENOENT')throw e;return '';});
    return text?JSON.parse(text):{};
  }
  try{
    await app.listen({host:'127.0.0.1',port});
    const page=await browser.newPage({timezoneId:'Asia/Shanghai'});
    await page.addInitScript(()=>localStorage.setItem('harness.chat.project','/stale-project'));
    const retiredRequests:string[]=[];
    let sessionRequests=0;
    page.on('request',request=>{
      const path=new URL(request.url()).pathname;
      if(path==='/api/v1/auth/local-session')sessionRequests++;
      if(path==='/api/v1/chat/access-log'||path==='/api/v1/chat/project')retiredRequests.push(path);
    });
    await page.goto('http://127.0.0.1:'+port);
    await expect(page.getByLabel('姓名',{exact:true})).toBeVisible();
    await expect(page.locator('.profile-side')).toBeVisible();
    // Keep the login form simple: no disclosure, promotional copy or footer.
    for(const removed of [
      '登录后，姓名和手机号将保存到当前工作目录',
      '在同一个工作台中连接 OpenCode 对话',
      '填写本地工作台资料，继续当前项目。',
      '仅用于本地工作台资料登记，不进行短信验证。',
      '本地运行 · 资源随项目使用',
    ])await expect(page.getByText(removed,{exact:false})).toHaveCount(0);
    await expect(page.getByRole('heading',{name:'登录工作台'})).toBeVisible();
    await expect(page.getByLabel('姓名',{exact:true})).toBeVisible();
    await expect(page.getByLabel('手机号',{exact:true})).toBeVisible();
    await expect(page.locator('.profile-submit')).toHaveCSS('background-color','rgb(36, 39, 44)');
    await page.screenshot({path:join(shots,'00-login-silver.png')});
    await expect(page.getByRole('navigation')).toHaveCount(0);
    await page.getByLabel('姓名',{exact:true}).fill('测试同事');
    await page.getByLabel('手机号',{exact:true}).fill('12');
    await page.getByRole('button',{name:'登录工作台',exact:true}).click();
    await expect(page.getByRole('alert')).toContainText('请输入有效的手机号');
    assert.equal(sessionRequests,0);
    await login(page);
    const contactCache=await page.evaluate(()=>JSON.stringify([localStorage,sessionStorage,document.cookie]));
    for(const value of ['测试同事','13800000000'])assert.ok(!contactCache.includes(value));
    await expect(page.getByRole('navigation')).toBeVisible();
    await expect(page.getByLabel('一次性配对码')).toHaveCount(0);
    await expect(page.getByLabel('对话内容')).toBeVisible();
    await expect(page.locator('.main > .topbar')).toBeVisible();
    const topModel=page.locator('.main > .topbar').getByRole('button',{name:'选择模型'});
    await expect(topModel).toBeVisible();
    await expect(page.locator('.composer-actions').getByLabel('选择模型')).toHaveCount(0);
    await expect(page.locator('.workspace-context')).toHaveText('对话首页');
    await expect(page.locator('.workspace-pill')).toHaveCount(0);
    await expect(page.locator('.chat-project-hint')).toHaveCount(0);
    await expect(page.locator('.chat-workspace-path')).toHaveCount(0);
    await expect(page.locator('.sidebar')).toHaveCSS('width','282px');
    await expect(page.getByRole('navigation').getByRole('button',{name:'代码审查'})).toHaveCSS('color','rgb(43, 48, 56)');
    await expect(page.locator('.silver-chat-home')).toHaveClass(/silver-chat-empty/);
    await page.screenshot({path:join(shots,'01-home-empty.png')});
    assert.equal(sessionRequests,1);
    const sessionCookie=async()=>(await page.context().cookies()).find(c=>c.name==='harness_session')?.value;
    const cookie=await sessionCookie();
    assert.ok(cookie);
    await page.reload();
    await expect(page.getByRole('navigation')).toBeVisible();
    assert.equal(sessionRequests,1);
    assert.equal(await sessionCookie(),cookie);
    // The home is already the native OpenCode chat; do not navigate to a separate scene.
    await expect(page.getByRole('navigation').getByRole('button',{name:'对话首页',exact:true})).toHaveAttribute('aria-current','page');
    await expect(page.locator('.sidebar .brand')).toContainText('智能工作台');
    // The browser must render the actual file-backed editor and respect ACL changes.
    await expect(page.getByRole('button',{name:'场景配置'})).toHaveCount(0);
    await page.goto('http://127.0.0.1:'+port+'/config');
    await expect(page.getByRole('heading',{name:'场景配置'})).toBeVisible();
    await expect(page.getByLabel('公管问题排查工作目录')).toHaveValue(project);
    await expect(page.getByRole('button',{name:'保存场景配置'})).toHaveCount(0);
    await page.screenshot({path:join(shots,'09-scene-config-readonly.png')});
    await writeFile(join(settingsDir,'config-access.json'),
      JSON.stringify({version:1,editors:['测试同事']}));
    await page.reload();
    await expect(page.getByRole('button',{name:'保存场景配置'})).toBeVisible();
    await page.getByRole('button',{name:'保存场景配置'}).click();
    await expect(page.getByRole('status').last()).toContainText('配置已写入当前启动目录');
    await page.screenshot({path:join(shots,'10-scene-config-editor.png')});
    // A missing scenario binding must show actionable local configuration guidance,
    // not an offline error or an attempt to contact OpenCode.
    await page.goto('http://127.0.0.1:'+port+'/');
    await expect(page.getByRole('navigation',{name:'场景导航'})).toBeVisible();
    const sceneFile=join(settingsDir,'scenarios.json');
    const configured=await readFile(sceneFile,'utf8');
    // Built-in /models must open the actual model selector, not issue a fake
    // custom command in OpenCode when the message scene first opens.
    const modelScene=JSON.parse(configured);
    modelScene.scenes.message={directory:project,executor:{type:'command',name:'models'}};
    await writeFile(sceneFile,JSON.stringify(modelScene));
    const sceneNav=page.getByRole('navigation',{name:'场景导航'});
    await sceneNav.getByRole('button',{name:'问题排查',exact:true}).click();
    await sceneNav.getByRole('button',{name:'报文问题排查',exact:true}).click();
    await expect(page.getByRole('dialog')).toBeVisible();
    await expect(page.getByText('默认入口 /models', {exact:false})).toBeVisible();
    await page.screenshot({path:join(shots,'12-default-model-picker.png')});
    await page.getByRole('button',{name:'关闭模型选择'}).click();
    await writeFile(sceneFile,JSON.stringify({version:1,scenes:{}}));
    await sceneNav.getByRole('button',{name:'公管问题排查',exact:true}).click();
    await expect(page.getByRole('heading',{name:'当前子场景尚未配置'})).toBeVisible();
    await expect(page.getByText('连接已中断')).toHaveCount(0);
    await page.screenshot({path:join(shots,'11-scene-unconfigured.png')});
    await writeFile(sceneFile,configured);
    await page.getByRole('button',{name:'重新检查配置'}).click();
    await expect(page.getByRole('heading',{name:'公管问题排查'})).toBeVisible();
    await sceneNav.getByRole('button',{name:'对话首页'}).click();
    await expect(page.locator('.chat-hero,.scene-cards')).toHaveCount(0);
    await expect(page).toHaveTitle('智能工作台');
    async function checkChatLayout(){
      await expect(page.getByLabel('对话内容')).toBeVisible();
      await expect(page.getByRole('button',{name:'更改工作目录'})).toHaveCount(0);
      await expect(page.getByRole('heading',{name:'后台访问日志'})).toHaveCount(0);
      const layout=await page.locator('.chat-layout').boundingBox(),box=await page.locator('.chat-box').boundingBox();
      assert.ok(layout&&box&&Math.abs(layout.width-box.width)<1);
    }
    await checkChatLayout();
    await topModel.click();
    const search=page.getByRole('combobox',{name:'搜索模型'});
    await search.fill('company');
    await expect(page.getByRole('option',{name:/Analysis fixture/})).toHaveCount(0);
    await expect(page.getByRole('option',{name:/Company fixture/})).toBeVisible();
    await page.screenshot({path:join(shots,'02-model-picker.png')});
    await search.press('Enter');
    await expect(page.locator('.topbar').getByRole('button',{name:'选择模型'})).toHaveAttribute('title','company/intranet');
    assert.deepEqual(await page.evaluate(()=>JSON.parse(localStorage.getItem('harness.global.model')??'null')),
      {id:'intranet',providerID:'company'});
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
    await expect(page.getByRole('alert')).toContainText('HTTP 503',{timeout:60000});
    assert.equal(sends,1);
    const created=JSON.parse(await readFile(file+'.body','utf8'));
    assert.deepEqual(created.model,{id:'intranet',providerID:'company'});
    assert.equal(created.location.directory,await realpath(project));
    const nav=page.getByRole('navigation',{name:'场景导航'});
    const problem=nav.getByRole('button',{name:'问题排查',exact:true});
    await expect(problem).toHaveAttribute('aria-expanded','false');
    await problem.click();
    await expect(problem).toHaveAttribute('aria-expanded','true');
    await expect(nav.getByRole('button',{name:'公管问题排查'})).toBeVisible();
    await expect(nav.getByRole('button',{name:'报文问题排查'})).toBeVisible();
    await nav.getByRole('button',{name:'报文问题排查'}).click();
    await expect(page.getByRole('heading',{name:'报文问题排查'})).toBeVisible();
    await expect(page.getByLabel('msgSn',{exact:true})).toBeVisible();
    const messageSubmit=page.getByRole('button',{name:'开始排查'});
    await expect(messageSubmit).toBeDisabled();
    await page.getByLabel('msgSn',{exact:true}).fill('000456');
    await expect(messageSubmit).toBeEnabled();
    await expect(page.locator('.topbar').getByRole('button',{name:'选择模型'})).toHaveAttribute('title','company/intranet');
    await page.getByLabel('对话内容').fill('报文模式独立草稿');
    await nav.getByRole('button',{name:'公管问题排查'}).click();
    await expect(page.getByRole('heading',{name:'公管问题排查'})).toBeVisible();
    await expect(page.getByLabel('msgSn',{exact:true})).toHaveValue('');
    await expect(page.locator('.topbar').getByRole('button',{name:'选择模型'})).toHaveAttribute('title','company/intranet');
    await expect(page.getByLabel('对话内容')).toHaveValue('');
    await nav.getByRole('button',{name:'报文问题排查'}).click();
    await expect(page.getByLabel('对话内容')).toHaveValue('报文模式独立草稿');
    await nav.getByRole('button',{name:'公管问题排查'}).click();
    await expect(problem).toHaveAttribute('aria-expanded','true');
    await expect(nav.getByRole('button',{name:'公管问题排查'})).toHaveAttribute('aria-current','page');
    await expect(nav.getByRole('button',{name:'数据分析',exact:true})).toHaveAttribute('aria-expanded','false');
    const submit=page.getByRole('button',{name:'开始排查'});
    await checkChatLayout();
    await expect(page.getByText('要排查哪次请求？')).toHaveCount(0);
    await expect(page.locator('.chat-feed')).toHaveCount(0);
    // The investigation composer must have breathing room between the
    // filtering criteria, optional question and footer actions on Windows.
    await expect(page.getByText('补充排查问题',{exact:true})).toBeVisible();
    const filterBox=await page.locator('.log-grid').boundingBox();
    const questionBox=await page.locator('.investigation-question-heading').boundingBox();
    const textareaBox=await page.getByLabel('对话内容').boundingBox();
    const actionBox=await page.locator('.log-investigation .composer-actions').boundingBox();
    assert.ok(filterBox&&questionBox&&textareaBox&&actionBox);
    assert.ok(questionBox.y-(filterBox.y+filterBox.height)>=20,'separate optional question from filters');
    assert.ok(textareaBox.y-(questionBox.y+questionBox.height)>=8,'leave space below question label');
    assert.ok(actionBox.y-(textareaBox.y+textareaBox.height)>=18,'separate submit actions from text area');
    await page.screenshot({path:join(shots,'03-log-criteria.png')});
    await page.setViewportSize({width:390,height:844});
    const stacked=await Promise.all(['开始时刻','结束时刻'].map(label=>page.getByLabel(label,{exact:true}).boundingBox()));
    assert.ok(stacked[0]&&stacked[1]&&stacked[1].y>=stacked[0].y+stacked[0].height,
      'narrow layouts stack date/time inputs vertically');
    assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),
      'investigation criteria must not overflow the narrow viewport');
    await page.screenshot({path:join(shots,'03-log-criteria-mobile.png')});
    await page.setViewportSize({width:1280,height:720});
    await expect(submit).toBeDisabled();
    await page.getByLabel('msgSn',{exact:true}).fill('000123');
    await expect(submit).toBeEnabled();
    await page.getByLabel('msgSn',{exact:true}).fill('');
    await expect(submit).toBeDisabled();
    await expect(page.getByRole('textbox',{name:'请求入参'})).toHaveCount(0);
    const quarter=page.getByRole('button',{name:'最近 15 分钟'});
    await page.clock.setFixedTime(new Date('2026-11-12T00:05:43.125+08:00'));
    await quarter.click();
    await expect(page.getByLabel('开始日期',{exact:true})).toHaveValue('2026-11-11');
    await expect(page.getByLabel('开始时刻',{exact:true})).toHaveValue('23:50:43');
    await expect(page.getByLabel('结束日期',{exact:true})).toHaveValue('2026-11-12');
    await expect(page.getByLabel('结束时刻',{exact:true})).toHaveValue('00:05:43');
    await expect(page.locator('.range-zone')).toHaveText('精确到秒');
    await expect(quarter).toHaveAttribute('aria-pressed','true');
    await expect(page.locator('.range-footer')).toContainText('15 分钟');
    // No native time scroll columns: exact HH:mm:ss remains stable under
    // mouse-wheel scrolling; dates keep Chromium's built-in calendar popup.
    for(const [date,time] of [['开始日期','开始时刻'],['结束日期','结束时刻']] as const){
      await expect(page.getByLabel(date)).toHaveAttribute('type','date');
      await expect(page.getByLabel(time)).toHaveAttribute('type','text');
    }
    await page.getByLabel('开始时刻').hover();
    await page.mouse.wheel(0,720);
    await expect(page.getByLabel('开始时刻')).toHaveValue('23:50:43');
    // A selected calendar day is displayed as ONE day, not as technical
    // midnight-to-next-midnight bounds. The native prompt remains half-open.
    await page.getByRole('button',{name:'选择某一天'}).click();
    await expect(page.getByLabel('查询日期')).toBeVisible();
    await page.getByLabel('查询日期').fill('2026-11-11');
    await expect(page.locator('.range-day-summary')).toContainText('2026/11/11 · 全天');
    await expect(page.locator('.range-day-summary')).toContainText('00:00:00 — 23:59:59');
    await expect(page.getByLabel('开始日期')).toHaveCount(0);
    await expect(page.getByLabel('结束日期')).toHaveCount(0);
    await expect(page.locator('.range-footer')).toContainText('已选择 2026/11/11 全天');
    // Switching directly between dates must keep exactly one day in view.
    await page.getByLabel('查询日期').fill('2026-11-10');
    await expect(page.locator('.range-day-summary')).toContainText('2026/11/10 · 全天');
    await page.getByLabel('查询日期').fill('2026-11-11');
    await expect(page.locator('.range-day-summary')).toContainText('2026/11/11 · 全天');
    await page.getByLabel('msgSn',{exact:true}).fill('000123');
    await expect(submit).toBeEnabled();
    await page.getByLabel('msgSn',{exact:true}).fill('');
    await page.getByRole('button',{name:'自定义范围'}).click();
    await expect(page.getByLabel('开始日期')).toHaveValue('2026-11-11');
    await expect(page.getByLabel('结束日期')).toHaveValue('2026-11-12');
    await quarter.click();
    await page.getByLabel('开始日期').fill('2026-11-11');
    await page.getByLabel('开始时刻').fill('23:57:11');
    await page.getByLabel('结束日期').fill('2026-11-12');
    await page.getByLabel('结束时刻').fill('00:03:42');
    await expect(page.getByLabel('开始时刻')).toHaveValue('23:57:11');
    await expect(page.getByLabel('结束时刻')).toHaveValue('00:03:42');
    await expect(page.locator('.range-footer')).toContainText('6 分钟 31 秒');
    await expect(page.locator('.log-time-range')).toBeVisible();
    await page.getByLabel('Trace ID',{exact:true}).fill('abc123');
    await expect(submit).toBeEnabled();
    await page.getByLabel('Trace ID',{exact:true}).fill('');
    await expect(submit).toBeDisabled();
    await page.getByLabel('关键词',{exact:true}).fill('timeout');
    await expect(submit).toBeEnabled();
    await page.getByLabel('Trace ID',{exact:true}).fill('abc123');
    await page.getByLabel('msgSn',{exact:true}).fill('000123');
    await expect(page.getByLabel('日志级别')).toHaveCount(0);
    await expect(page.getByLabel('导入本地日志')).toHaveCount(0);
    await page.getByLabel('对话内容').fill('检查超时原因');
    await submit.click();
    await expect(page.getByRole('alert')).toContainText('未发现配置的执行入口');
    await expect(page.getByLabel('对话内容')).toHaveValue('检查超时原因');
    const painted=await observePaintedText(page);
    await writeFile(file+'.skill','1');
    // Regression: first submit must never mount the adjustment drawer, even
    // briefly between native session creation and message submission.
    await page.evaluate(()=>{
      const watcher=new MutationObserver(()=>{
        if(document.querySelector('.investigation-drawer-backdrop'))
          document.documentElement.dataset.autoDrawerOpened='true';
      });
      watcher.observe(document.body,{childList:true,subtree:true});
      window.addEventListener('harness:stop-drawer-observer',()=>watcher.disconnect(),{once:true});
    });
    await submit.click();
    const adjustment=page.getByRole('button',{name:'调整排查条件'});
    await expect(page.locator('.log-investigation-active > .scene-title').getByRole('button',{name:'调整排查条件'})).toBeVisible();
    await expect(page.getByRole('dialog',{name:'调整排查条件'})).toHaveCount(0);
    assert.deepEqual((await recorded('.body')).model,{id:'intranet',providerID:'company'},
      'the global model must apply to new investigation sessions');
    const expectedRange='2026-11-11T23:57:11+08:00 至 2026-11-12T00:03:42+08:00';
    const summaryBar=page.locator('.investigation-summary-bar');
    await expect(summaryBar).toHaveCount(0);
    await expect(page.getByText('当前排查条件',{exact:true})).toHaveCount(0);
    await page.evaluate(()=>window.dispatchEvent(new Event('harness:stop-drawer-observer')));
    await expect(page.locator('html')).not.toHaveAttribute('data-auto-drawer-opened','true');
    const sentRange=(await recorded('.prompt')).text;
    assert.ok(sentRange.includes(expectedRange));
    assert.ok(sentRange.includes('Asia/Shanghai'));
    assert.ok(sentRange.includes('msgSn：000123'));
    const running=page.getByRole('button',{name:'正在执行 · 中断执行'});
    await expect(running).toBeVisible();
    await expect(running).toHaveCSS('background-color','rgb(170, 39, 46)');
    const feed=page.locator('.chat-feed');
    const processRow=feed.locator('.turn-process').first();
    await expect(processRow).toBeVisible();
    await expect(feed.getByText('命中第 2 行',{exact:true})).not.toBeVisible();
    await processRow.locator('> summary').click();
    await expect(feed).toContainText('已扫描两个文件');
    await expect(feed.locator('.execution-tool')).toContainText('命中第 2 行');
    await feed.getByText('调用参数',{exact:true}).click();
    await expect(feed.locator('.execution-tool pre').first()).toContainText('abc123');
    await expect(feed).not.toContainText('cross-session-marker');
    const thought=feed.locator('.turn-reasoning');
    await expect(thought.locator('summary')).toContainText('思考');
    await expect(feed.getByText('客户端可见的测试片段',{exact:true})).toBeVisible();
    await expect(feed.getByText(firstReasoning,{exact:true})).toBeVisible();
    await expect(thought.locator('.reasoning-live-text')).toContainText(firstReasoning);
    await expect(feed).toHaveCSS('overflow-anchor','none');
    await expect(feed).toHaveCSS('scroll-behavior','auto');
    // Streamed thought text may grow, but it must not shift the workbench
    // heading or the bottom reply composer as it is progressively painted.
    const stableFrame=async()=>{
      const heading=await page.locator('.log-investigation-active > .scene-title').boundingBox();
      const composer=await page.locator('.log-investigation-active .composer').boundingBox();
      assert.ok(heading&&composer);
      return {headingY:heading.y,composerY:composer.y};
    };
    const initialFrame=await stableFrame();
    await expect(feed).not.toContainText('private-marker');
    await thought.locator('summary').click();
    await expect(feed.getByText(firstReasoning,{exact:true})).not.toBeVisible();
    await thought.locator('summary').focus();await page.keyboard.press('Enter');
    await expect(feed.getByText(firstReasoning,{exact:true})).toBeVisible();
    await expect(feed).not.toContainText('已找到');
    await expect(feed).not.toContainText('foreign-secret');
    await writeFile(file+'.next','1');
    await expect(feed.getByText(firstReasoning+'时间线',{exact:true})).toBeVisible();
    await expect(feed.getByText('已找到',{exact:true})).toBeVisible();
    const afterFrame=await stableFrame();
    assert.ok(Math.abs(initialFrame.headingY-afterFrame.headingY)<2,'live thinking must not move the scene header');
    assert.ok(Math.abs(initialFrame.composerY-afterFrame.composerY)<2,'live thinking must not move the reply composer');
    await expect(feed).not.toContainText('两条记录');
    await writeFile(file+'.finish','1');
    await expect(feed.getByText(finalAnswer,{exact:true})).toBeVisible();
    const observations=await painted.evaluate(prefixes=>[...prefixes]);
    for(const chunk of [firstReasoning,finalAnswer]){
      const partial=new Set(observations.filter(s=>s.length>=4&&s.length<chunk.length&&chunk.startsWith(s)));
      assert.ok(partial.size>=3,'a whole upstream chunk produces multiple visible prefixes: '+chunk);
    }
    await page.reload();
    await expect(summaryBar).toHaveCount(0);
    await expect(page.getByRole('dialog',{name:'调整排查条件'})).toHaveCount(0);
    await expect(feed.getByText(finalAnswer,{exact:true})).toHaveCount(1);
    await expect(running).toHaveCount(0);
    await expect(feed.locator('.turn-process').first()).not.toHaveAttribute('open','');
    await expect(feed.getByRole('heading',{name:'排查结果'})).toBeVisible();
    await page.screenshot({path:join(shots,'04-log-results.png')});
    // Result-first editing: the editor overlays the answer rather than
    // growing the composer; Escape and Cancel must restore applied criteria.
    const answerBefore=await feed.boundingBox(),composerBefore=await page.locator('.log-investigation-active .composer').boundingBox();
    assert.ok(answerBefore&&composerBefore&&answerBefore.height>=220,
      'results must have a substantial independent reading area');
    await adjustment.click();
    const dialog=page.getByRole('dialog',{name:'调整排查条件'});
    await expect(dialog).toBeVisible();
    await expect(dialog.getByLabel('Trace ID',{exact:true})).toHaveValue('abc123');
    await expect(dialog.getByLabel('msgSn',{exact:true})).toHaveValue('000123');
    await expect(dialog.getByLabel('关键词',{exact:true})).toHaveValue('timeout');
    await expect(dialog.getByRole('button',{name:'应用并重新排查'})).toBeEnabled();
    const answerDuring=await feed.boundingBox(),composerDuring=await page.locator('.log-investigation-active .composer').boundingBox();
    assert.ok(answerDuring&&composerDuring);
    assert.ok(Math.abs(answerBefore.height-answerDuring.height)<3,'editing must not shrink the result pane');
    assert.ok(Math.abs(composerBefore.height-composerDuring.height)<3,'editing must not grow the reply composer');
    await page.screenshot({path:join(shots,'04a-log-adjustment-drawer.png')});
    await dialog.getByLabel('Trace ID',{exact:true}).fill('temporary-trace');
    await dialog.getByLabel('msgSn',{exact:true}).fill('000987');
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await expect(summaryBar).toHaveCount(0);
    await adjustment.click();
    await expect(dialog.getByLabel('Trace ID',{exact:true})).toHaveValue('abc123');
    await expect(dialog.getByLabel('msgSn',{exact:true})).toHaveValue('000123');
    await dialog.getByLabel('关键词',{exact:true}).fill('draft-keyword');
    await dialog.getByRole('button',{name:'取消调整'}).click();
    await expect(dialog).toHaveCount(0);
    await adjustment.click();
    await expect(dialog.getByLabel('关键词',{exact:true})).toHaveValue('timeout');
    await dialog.getByRole('button',{name:'关闭调整面板'}).click();
    await expect(dialog).toHaveCount(0);
    await expect(feed.getByText(finalAnswer,{exact:true})).toBeVisible();
    await expect(feed.getByRole('cell',{name:'120 ms'})).toBeVisible();
    await expect(feed.locator('pre code.language-json')).toContainText('"count"');
    await page.getByRole('button',{name:'选择模型'}).click();
    await page.getByRole('combobox',{name:'搜索模型'}).fill('analysis');
    await page.getByRole('option',{name:/Analysis fixture/}).click();
    await page.getByRole('combobox',{name:'模型变体'}).selectOption('high');
    await page.getByRole('button',{name:'使用此模型'}).click();
    await expect(page.locator('.topbar').getByRole('button',{name:'选择模型'})).toHaveAttribute('title','labs/analysis');
    assert.deepEqual(await page.evaluate(()=>JSON.parse(localStorage.getItem('harness.global.model')??'null')),
      {id:'analysis',providerID:'labs',variant:'high'});
    await page.reload();
    await expect(page.locator('.topbar').getByRole('button',{name:'选择模型'})).toHaveAttribute('title','labs/analysis');
    await expect(page.getByLabel('对话内容')).toBeVisible();
    await page.getByLabel('对话内容').fill('保存这份草稿');
    await expect(page.getByLabel('对话内容')).toHaveValue('保存这份草稿');
    await expect(page.getByRole('button',{name:/^发送/})).toBeEnabled();
    await writeFile(file+'.ask','1');
    await expect(page.getByText('选择排查范围',{exact:true})).toBeVisible();
    await page.screenshot({path:join(shots,'05-log-form-waiting.png')});
    // Sidebar starts a direct chat; the log scene owns its independent new-investigation action.
    await page.locator('.log-investigation').getByRole('button',{name:'新建对话'}).click();
    await expect(page.getByLabel('对话内容')).toHaveValue('');
    await expect(page.getByText('选择排查范围',{exact:true})).toHaveCount(0);
    await page.clock.setFixedTime(new Date('2026-11-12T00:05:43.125+08:00'));
    await quarter.click();
    await page.getByLabel('Trace ID',{exact:true}).fill('second-trace');
    await page.getByLabel('对话内容').fill('第二次排查');
    await submit.click();
    await expect(feed).toContainText('第二个会话的结果');
    assert.deepEqual((await recorded('.body')).model,{id:'analysis',providerID:'labs',variant:'high'},
      'new conversations use the latest global model, even after switching scenes');
    await expect(page.getByRole('button',{name:'调整排查条件'})).toBeVisible();
    const presetBounds='2026-11-11T23:50:43+08:00 至 2026-11-12T00:05:43+08:00';
    await expect(summaryBar).toHaveCount(0);
    await expect(page.getByRole('dialog',{name:'调整排查条件'})).toHaveCount(0);
    assert.ok((await recorded('.second-prompt')).text.includes(presetBounds));
    await page.getByLabel('对话内容').fill('第二个会话草稿');
    await page.getByRole('button',{name:'历史对话'}).click();
    const history=page.getByRole('region',{name:'历史对话'});
    await expect(history.getByRole('button',{name:/第二次排查/})).toHaveAttribute('aria-current','true');
    await history.getByRole('button',{name:/检查超时原因/}).click();
    await expect(feed).not.toContainText('第二个会话的结果');
    await expect(page.getByText('选择排查范围',{exact:true})).toBeVisible();
    await page.getByRole('radio',{name:/当前 Trace/}).check();
    await page.getByLabel('服务名称',{exact:false}).fill('orders');
    await page.getByRole('textbox',{name:'补充说明'}).fill('手动说明');
    await expect(page.getByLabel('对话内容')).toHaveCount(0);
    await expect(page.getByRole('button',{name:'提交回答'})).toBeEnabled();
    await expect(page).toHaveTitle(/等待你回应/);
    await page.getByRole('button',{name:'提交回答'}).click();
    await expect.poll(async()=>(await recorded('.answer')).answer).toEqual({scope:'trace',service:'orders',note:'手动说明'});
    await expect(page.getByText('logs/orders.log',{exact:true})).toBeVisible();
    await page.screenshot({path:join(shots,'06-log-permission-waiting.png')});
    await page.getByRole('button',{name:'仅本次允许'}).click();
    await expect.poll(async()=>(await recorded('.permission')).decision).toBe('once');
    const codeForm=page.getByRole('form',{name:'验证本次查询'});
    const codeInput=codeForm.getByRole('textbox',{name:/验证码/});
    await expect(codeInput).toBeEditable();
    await expect(page.getByLabel('对话内容')).toHaveCount(0);
    await expect(codeInput).toHaveValue('');
    await codeForm.getByText('可选回答',{exact:true}).click();
    await codeForm.getByRole('button',{name:'手动输入'}).click();
    await expect(codeInput).toHaveValue('手动输入');
    const requests:string[]=[];
    page.on('request',req=>{if(req.method()==='POST')requests.push(new URL(req.url()).pathname);});
    const code='004831';
    await codeInput.fill(code);
    await codeInput.press('Enter');
    await expect.poll(async()=>await recorded('.code')).toEqual({answer:{q0:code}});
    await expect(page.getByLabel('对话内容')).toHaveValue('保存这份草稿');
    assert.deepEqual(requests,['/api/v1/chat/sessions/ses_logs_fixture/form']);
    await expect(feed).not.toContainText(code);
    assert.ok(!(await readFile(file+'.prompt','utf8')).includes(code));
    await expect(page.getByRole('button',{name:/^发送/})).toBeEnabled();
    assert.deepEqual((await recorded('.prompt')).skills,[{id:'corp-logs-v1'}]);
    const request=JSON.parse(await readFile(file+'.prompt','utf8')).text;
    for(const value of ['检查超时原因','abc123','timeout'])assert.ok(request.includes(value));
    assert.ok(!request.includes('日志级别：'));
    assert.ok(!request.includes('导入日志：'));
    await page.getByLabel('对话内容').fill('下一步怎么查？');
    await page.getByLabel('对话内容').press('Enter');
    await expect.poll(async()=>(await recorded('.model')).model).toEqual({id:'analysis',providerID:'labs',variant:'high'});
    await expect.poll(async()=>JSON.parse(await readFile(file+'.prompt','utf8')).text).toBe('下一步怎么查？');
    assert.deepEqual((await recorded('.prompt')).skills,[{id:'corp-logs-v1'}]);
    await expect(summaryBar).toHaveCount(0);
    assert.equal(await page.evaluate(()=>JSON.parse(sessionStorage.getItem('harness.logs.filters.ses_logs_fixture')??'{}').msgSn),'000123');
    await page.getByRole('button',{name:'历史对话'}).click();
    await history.getByRole('button',{name:/第二次排查/}).click();
    await page.reload();
    await expect(page.getByLabel('对话内容')).toHaveValue('第二个会话草稿');
    await expect(feed).toContainText('第二个会话的结果');
    await expect(summaryBar).toHaveCount(0);
    await expect(page.getByRole('dialog',{name:'调整排查条件'})).toHaveCount(0);
    await page.getByRole('button',{name:'历史对话'}).click();
    await history.getByRole('button',{name:/检查超时原因/}).click();
    await expect(summaryBar).toHaveCount(0);
    await expect(page.getByRole('dialog',{name:'调整排查条件'})).toHaveCount(0);
    assert.deepEqual(await page.evaluate(()=>[sessionStorage.getItem('harness.chat'),sessionStorage.getItem('harness.logs')]),['','ses_logs_fixture']);
    // Applying conditions reuses the current session, closes the drawer,
    // preserves a pending reply draft and does not attach reply-only files.
    const pendingDraft='未发送的追问草稿';
    await page.getByLabel('对话内容').fill(pendingDraft);
    await adjustment.click();
    await expect(dialog.getByLabel('Trace ID',{exact:true})).toHaveValue('abc123');
    await expect(dialog.getByLabel('msgSn',{exact:true})).toHaveValue('000123');
    await dialog.getByLabel('msgSn',{exact:true}).fill('000789');
    await dialog.getByLabel('关键词',{exact:true}).fill('retry-timeout');
    await expect(dialog.getByRole('button',{name:'应用并重新排查'})).toBeEnabled();
    await dialog.getByRole('button',{name:'应用并重新排查'}).click();
    await expect(dialog).toHaveCount(0);
    await expect(summaryBar).toHaveCount(0);
    await expect.poll(()=>page.evaluate(()=>JSON.parse(sessionStorage.getItem('harness.logs.filters.ses_logs_fixture')??'{}')))
      .toMatchObject({keyword:'retry-timeout',msgSn:'000789'});
    await expect(page.getByLabel('对话内容')).toHaveValue(pendingDraft);
    await expect.poll(async()=>JSON.parse(await readFile(file+'.prompt','utf8')).text)
      .toContain('关键字：retry-timeout');
    const rerun=await recorded('.prompt');
    assert.ok(rerun.text.includes('msgSn：000789'));
    assert.ok(!rerun.text.includes(pendingDraft));
    assert.deepEqual(rerun.files,[]);
    assert.equal(await page.evaluate(()=>sessionStorage.getItem('harness.logs')),'ses_logs_fixture');
    assert.deepEqual(retiredRequests,[]);
    // History management must persist in native OpenCode, not only localStorage.
    await page.getByRole('button',{name:'历史对话'}).click();
    const secondRow=history.locator('li').filter({hasText:'第二次排查'});
    await secondRow.getByRole('button',{name:'重命名'}).click();
    // Editing replaces the title text with an input value; the original
    // text-filtered row locator therefore no longer matches while editing.
    await history.getByRole('textbox',{name:'对话名称'}).fill('放弃的草稿标题');
    await history.getByRole('button',{name:'取消'}).click();
    await expect(secondRow).toContainText('第二次排查');
    await secondRow.getByRole('button',{name:'重命名'}).click();
    await history.getByRole('textbox',{name:'对话名称'}).fill('订单超时调查 · 人工命名');
    await history.getByRole('button',{name:'保存'}).click();
    await expect(history.getByRole('button',{name:/订单超时调查 · 人工命名/})).toBeVisible();
    await page.reload();
    await page.getByRole('button',{name:'历史对话'}).click();
    await history.getByRole('button',{name:/订单超时调查 · 人工命名/}).click();
    await expect(page.locator('.conversation-title')).toHaveText('订单超时调查 · 人工命名');
    await page.getByRole('button',{name:'历史对话'}).click();
    const renamedRow=history.locator('li').filter({hasText:'订单超时调查 · 人工命名'});
    await renamedRow.getByRole('button',{name:'删除'}).click();
    await expect(renamedRow).toContainText('无法恢复');
    await renamedRow.getByRole('button',{name:'取消'}).click();
    await expect(renamedRow.getByRole('button',{name:'删除'})).toBeVisible();
    await renamedRow.getByRole('button',{name:'删除'}).click();
    await renamedRow.getByRole('button',{name:'确认删除'}).click();
    await expect(history.getByRole('button',{name:/订单超时调查 · 人工命名/})).toHaveCount(0);
    await expect(page.locator('.log-investigation-setup')).toBeVisible();
    await page.reload();
    await page.getByRole('button',{name:'历史对话'}).click();
    await expect(history.getByRole('button',{name:/订单超时调查 · 人工命名/})).toHaveCount(0);
    await page.getByRole('button',{name:'退出登录'}).click();
    await expect(page.getByRole('heading',{name:'登录工作台'})).toBeVisible();
    assert.equal(await sessionCookie(),undefined);
    await page.reload();
    await expect(page.getByLabel('手机号',{exact:true})).toHaveValue('');
    assert.equal(sessionRequests,1);
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

it('groups a multi-step weather response and keeps verification output out of the answer',async()=>{
  const port=31882,id='ses_weather_example';
  const root=await realpath(await mkdtemp(join(tmpdir(),'harness weather ')));
  await mkdir(join(root,'.workbench'),{recursive:true});
  await writeFile(join(root,'.workbench','scenarios.json'),JSON.stringify({version:1,scenes:{
    data:{directory:root,executor:{type:'skill',name:'analysis-fixture'}},
  }}));
  const app=await createApp({localOnly:true,workspace:root,auth:new LocalAuth('unused',port)});
  const browser=await chromium.launch(process.env.HARNESS_TEST_CHROME?{channel:'chrome'}:{});

  try{
    await app.listen({host:'127.0.0.1',port});
    const page=await browser.newPage({viewport:{width:1280,height:800}});
    await page.addInitScript(id=>{sessionStorage.setItem('harness.scene','chat');sessionStorage.setItem('harness.chat',id);},id);
    const answer='北京明天晴，早晚温差较大。\n\n- **白天**：晴，最高 **15℃**\n- **夜间**：晴，最低 **3℃**\n\n数据来源：[中央气象台](https://www.nmc.cn/publish/forecast/ABJ/beijing.html)';
    const tool=(name:string,result:string)=>({kind:'tool',name,status:'completed',result});
    const assistant=(id:string,steps:unknown[])=>({id,role:'assistant',text:'',steps,tools:[],files:[],complete:true});
    const english='Check the date on the weather report.';
    const chinese='核对天气预报的发布日期。';
    const translations:{message:string;ordinal:number}[]=[];
    await page.route('**/chat/sessions/'+id+'/reasoning/translation',async route=>{
      assert.equal(route.request().method(),'POST');
      translations.push(route.request().postDataJSON());
      if(translations.length===1)return route.fulfill({status:502,json:{error:{code:'REASONING_TRANSLATION_FAILED'}}});
      return route.fulfill({json:{data:{text:chinese},requestId:'fixture'}});
    });
    const messages=[
      {id:'q',role:'user',text:'北京明天的天气如何',files:[],tools:[]},
      assistant('search',[{kind:'text',text:'先查北京的预报。'},tool('websearch','搜索结果列表'),{kind:'thinking',hasText:false,complete:true}]),
      assistant('fetch',[{kind:'thinking',ordinal:4,text:english,at:1000,ended:9600,hasText:true,complete:true},{kind:'thinking',ordinal:5,text:'预报地区与北京一致。',hasText:true,complete:true},tool('webfetch','向右滑动填充拼图'),tool('webfetch','北京天气预报原始页面')]),
      assistant('answer',[{kind:'text',text:answer}]),
      assistant('done',[{kind:'notice',text:'本轮已完成'}]),
    ];
    await page.route('**/chat/sessions/'+id+'/activity',route=>route.fulfill({status:204}));
    await page.route('**/chat/sessions/'+id,route=>route.fulfill({json:{data:{
      id,directory:root,title:'北京明天的天气',active:false,messages,permissions:[],forms:[]},requestId:'fixture'}}));
    await page.goto('http://127.0.0.1:'+port);
    await login(page);
    const response=page.getByRole('article',{name:'OpenCode 回答'});
    const shots=join(process.cwd(),'screenshots');
    await mkdir(shots,{recursive:true});
    const body=response.getByRole('region',{name:'回答正文'});
    await expect(response).toHaveCount(1);
    await expect(body.getByRole('link',{name:'中央气象台'})).toBeVisible();
    await expect(body).toContainText('15℃');
    await page.screenshot({path:join(shots,'07-conversation-active.png')});
    const answerBox=await body.boundingBox(),questionBox=await page.locator('.user-question').boundingBox();
    const composerBox=await page.locator('.composer').boundingBox();
    assert.ok(questionBox&&answerBox&&composerBox);
    const readingWidth=await page.locator('.conversation-transcript').boundingBox();
    assert.ok(readingWidth&&readingWidth.width<=921,'active assistant transcript has a comfortable max width');
    assert.ok(Math.abs(answerBox.x-questionBox.x)<=4,'question and reply share a reading edge');
    // Silver Precision keeps the composer docked at the bottom instead of directly beneath short replies.
    assert.ok(composerBox.y>=answerBox.y+answerBox.height,'composer does not cover the response');
    assert.ok(composerBox.y+composerBox.height<=800+2,'composer remains visible in the viewport');
    await expect(body).not.toContainText('拼图');
    await expect(page.getByText('向右滑动填充拼图',{exact:true})).not.toBeVisible();
    const historyThought=response.locator('.turn-reasoning');
    await expect(historyThought).toHaveAttribute('open','');
    await expect(historyThought.locator('summary')).toContainText('思考');
    await expect(body).not.toContainText(english);
    await historyThought.locator('summary').click();
    await expect(historyThought).not.toHaveAttribute('open','');
    await historyThought.locator('summary').focus();await page.keyboard.press('Enter');
    await expect(historyThought.getByText(english,{exact:true})).toBeVisible();
    const translate=historyThought.getByRole('button',{name:'译成中文',exact:true});
    await expect(translate).toHaveCount(1);
    assert.deepEqual(translations,[]);
    await translate.click();
    await expect(historyThought.getByRole('status')).toContainText('翻译失败，请重试');
    await expect(historyThought.getByText(english,{exact:true})).toBeVisible();
    await translate.click();
    await expect(historyThought.getByText(chinese,{exact:true})).toBeVisible();
    await expect(historyThought.getByText('预报地区与北京一致。')).toBeVisible();
    await historyThought.locator('summary').click();
    await historyThought.locator('summary').click();
    await historyThought.getByRole('button',{name:'查看原文'}).click();
    await expect(historyThought.getByText(english,{exact:true})).toBeVisible();
    await historyThought.getByRole('button',{name:'查看译文'}).click();
    await expect(historyThought.getByText(chinese,{exact:true})).toBeVisible();
    assert.deepEqual(translations,[{message:'fetch',ordinal:4},{message:'fetch',ordinal:4}]);
    await historyThought.locator('summary').click();
    const processRow=response.locator('.turn-process > summary');
    await expect(processRow).toContainText('3 次工具调用');
    await processRow.focus();await page.keyboard.press('Enter');
    const fetches=response.locator('.execution-tool').filter({hasText:'webfetch'});
    await expect(fetches).toHaveCount(2);
    await expect(response.getByText('先查北京的预报。',{exact:true})).toBeVisible();
    await expect(response.getByText(/OpenCode 未提供可显示文本/)).toHaveCount(0);
    await fetches.first().getByText('查看工具结果').click();
    await expect(fetches.first().getByText('向右滑动填充拼图')).toBeVisible();
    await processRow.click();
    await page.setViewportSize({width:390,height:844});
    await expect(body).toBeVisible();
    assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
    // Mobile must not hide history recovery or logout behind the compact sidebar.
    await expect(page.getByRole('button',{name:'退出登录'})).toBeVisible();
    await page.getByRole('navigation',{name:'场景导航'}).getByRole('button',{name:'历史对话'}).click();
    await expect(page.getByRole('region',{name:'历史对话'})).toBeVisible();
    await page.getByRole('main').getByRole('button',{name:'历史对话',exact:true}).click();
    await expect(page.getByRole('region',{name:'历史对话'})).toHaveCount(0);
    await page.reload();
    await expect(page.locator('.turn-process')).not.toHaveAttribute('open','');
    await expect(body).toContainText('15℃');
    await page.setViewportSize({width:1280,height:800});
    messages.push(assistant('long',[{kind:'text',text:Array.from({length:80},(_,i)=>'记录 '+i).join('\n\n')}]));
    await page.reload();
    await expect(body.getByText('记录 79',{exact:true})).toHaveCount(1);
    assert.ok(await page.locator('.chat-feed').evaluate(el=>el.scrollHeight>el.clientHeight+100));
    const pinned=await page.locator('.composer').boundingBox();
    assert.ok(pinned&&pinned.y+pinned.height<=800,'long history scrolls without displacing the composer');
    const fallbackAnswer='这里是通过轮询补齐的一整段回答，用来检查能否逐字显示。';
    const nextQuestion='补充说明';
    const painted=await observePaintedText(page);
    await page.route('**/chat/sessions/'+id+'/messages',async route=>{
      messages.push({id:'new-user',role:'user',text:nextQuestion,tools:[],files:[]},
        Object.assign(assistant('only-snapshot',[
          {kind:'thinking',ordinal:0,text:english,at:1000,ended:9600,hasText:true,complete:true},
          {kind:'text',ordinal:1,text:fallbackAnswer}
        ]),{footer:{agent:'build',model:'fixture',durationMs:13900,tokensPerSecond:36.1}}),
        Object.assign(assistant('finished',[{kind:'notice',text:'本轮已完成'}]),{idleOutcome:'succeeded'}));
      await route.fulfill({status:202,json:{data:{id:'new-user'},requestId:'fixture'}});
    });
    await page.getByLabel('对话内容').fill(nextQuestion);
    await page.getByLabel('对话内容').press('Enter');
    await expect(response.last().getByText(fallbackAnswer,{exact:true})).toBeVisible();
    const latest=response.last();
    await expect(latest.locator('.turn-reasoning > summary')).toContainText('8.6s');
    await expect(latest.locator('.turn-reasoning')).toContainText(english);
    await expect(latest.locator('.turn-answer')).not.toContainText(english);
    await expect(latest.locator('.turn-process')).toHaveCount(0);
    await expect(latest.locator('.turn-answer ~ .turn-metrics')).toHaveText('build · fixture · 13.9s · 36.1 tok/s');
    const prefixes=await painted.evaluate(seen=>[...seen]);
    assert.ok(prefixes.filter(s=>s.length>1&&s.length<fallbackAnswer.length&&fallbackAnswer.startsWith(s)).length>=3,
      'snapshot-only completion is paced for the new turn');
    // Switching scenarios must keep the same OpenCode conversation and must not POST a new session.
    const createdSessions:string[]=[];
    page.on('request',request=>{
      if(request.method()==='POST'&&new URL(request.url()).pathname==='/api/v1/chat/sessions')
        createdSessions.push(request.url());
    });
    const navigation=page.getByRole('navigation',{name:'场景导航'});
    await navigation.getByRole('button',{name:'数据分析',exact:true}).click();
    await expect(navigation.getByRole('button',{name:'数据分析',exact:true})).toHaveAttribute('aria-expanded','true');
    await navigation.getByRole('button',{name:'公管数据分析',exact:true}).click();
    await expect(page.getByRole('heading',{name:'公管数据分析'})).toBeVisible();
    await expect(page.locator('.topbar').getByRole('button',{name:'选择模型'})).toBeVisible();
    await expect(page.getByLabel('对话内容')).toBeVisible();
    await page.getByLabel('对话内容').fill('请分析公管订单超时原因');
    await expect(page.getByLabel('对话内容')).toHaveValue('请分析公管订单超时原因');
    await page.screenshot({path:join(shots,'08-data-chat.png')});
    await expect(page.getByRole('heading',{name:'直接开始你的工作'})).toHaveCount(0);
    await navigation.getByRole('button',{name:'对话首页',exact:true}).click();
    await expect(response.last()).toContainText(fallbackAnswer);
    await expect(page.getByLabel('对话内容')).toBeVisible();
    await page.getByLabel('对话内容').fill('跨场景保存这份草稿');
    await navigation.getByRole('button',{name:'代码审查',exact:true}).click();
    await expect(page.getByRole('heading',{name:'代码审查',exact:true})).toBeVisible();
    await expect(page.locator('.topbar').getByRole('button',{name:'选择模型'})).toBeVisible();
    await navigation.getByRole('button',{name:'对话首页',exact:true}).click();
    await expect(page.getByLabel('对话内容')).toHaveValue('跨场景保存这份草稿');
    assert.deepEqual(createdSessions,[],'navigating never creates a native OpenCode session');
    await page.getByRole('complementary').getByRole('button',{name:'新建对话'}).click();
    await expect(page.getByRole('heading',{name:'直接开始你的工作'})).toBeVisible();
    await expect(page.getByLabel('对话内容')).toHaveValue('');
    await expect(response).toHaveCount(0);
    await expect(page.getByRole('button',{name:'北京明天的天气',exact:true})).toBeVisible();
    await page.getByRole('button',{name:'北京明天的天气',exact:true}).click();
    await expect(response.last()).toContainText(fallbackAnswer);
    assert.deepEqual(createdSessions,[],'restoring a sidebar history item reuses its native session');
    await page.getByRole('complementary').getByRole('button',{name:'全部',exact:true}).click();
    const sidebarHistory=page.getByRole('region',{name:'历史对话'});
    await expect(sidebarHistory).toBeVisible();
    await page.getByRole('main').getByRole('button',{name:'历史对话',exact:true}).click();
    await expect(sidebarHistory).toHaveCount(0);



  }finally{await browser.close();await app.close();await rm(root,{recursive:true,force:true});}
});
