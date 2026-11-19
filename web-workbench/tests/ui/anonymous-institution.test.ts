import {test} from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {mkdir} from 'node:fs/promises';
import {chromium,expect} from '@playwright/test';
import {sqliteEmbedFixture} from '../support/sqlite-embed-fixture.ts';
import {SceneConfigStore} from '../../apps/server/src/scene-config.ts';
import {DatabaseSync} from 'node:sqlite';
test('anonymous institution: Kimi default, direct bound Skill without adapter, three iframe sessions, full-width responsive results and restart',{timeout:90000},async()=>{
  const origin='http://127.0.0.1:4186',parent='http://127.0.0.1:4187';
  const f=await sqliteEmbedFixture(origin,parent,false); // Deliberately no business query adapter.
  f.modelCatalog.splice(0,f.modelCatalog.length,{id:'other',providerID:'company',name:'Other',enabled:true},{id:'kimi-k2',providerID:'company',name:'Kimi K2',enabled:true});
  const config=new SceneConfigStore(f.workspace),current=config.read();
  await config.save({...current.config,scenes:{...current.config.scenes,'institution-message':{directory:f.workspace,executor:{type:'skill',name:'message-query'}}}},current.revision);
  const grants=()=>{const db=new DatabaseSync(f.path,{readOnly:true});try{return db.prepare('SELECT id FROM wh_embed_sessions ORDER BY id').all().map(r=>r.id);}finally{db.close();}};
  const host=createServer((_req,res)=>{res.setHeader('Content-Type','text/html');res.end(`<!doctype html><title>Host iframe only</title><style>body{margin:0}iframe{width:100%;height:100vh;border:0}</style><iframe title="机构报文排查" src="${origin}/embed/institution-message"></iframe>`);});
  let browser:Awaited<ReturnType<typeof chromium.launch>>|undefined;
  try{
    await f.app.listen({host:'127.0.0.1',port:4186});await new Promise<void>(resolve=>host.listen(4187,'127.0.0.1',resolve));
    browser=await chromium.launch({headless:true,...(process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE?{executablePath:process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE}:{}),args:['--disable-third-party-cookies']});
    const context=await browser.newContext({viewport:{width:1440,height:1000},timezoneId:'Asia/Shanghai'});await context.addCookies([{name:'harness_session',value:'unrelated-login',url:origin}]);
    const requests:Promise<{url:string;cookie:string;body:string}>[]=[];
    context.on('request',request=>{if(request.url().startsWith(origin+'/api/'))requests.push(request.allHeaders().then(h=>({url:request.url(),cookie:h.cookie??'',body:request.postData()??''})));});
    const pages=await Promise.all([0,1,2].map(async()=>{const page=await context.newPage();await page.goto(parent);return page;})),frames=pages.map(p=>p.frameLocator('iframe'));
    for(const frame of frames){for(const label of ['账号','密码','验证码'])await expect(frame.getByLabel(label,{exact:true})).toHaveCount(0);await expect(frame.getByLabel('日期',{exact:true})).toBeVisible();await expect(frame.getByRole('button',{name:/选择模型/})).toContainText('kimi-k2');await expect(frame.getByRole('button',{name:'历史对话',exact:true})).toHaveCount(0);await expect(frame.locator('.sidebar')).toHaveCount(0);}
    assert.equal(grants().length,3,'StrictMode must not create duplicate grants');
    await mkdir('screenshots',{recursive:true});await pages[0].screenshot({path:'screenshots/anonymous-institution-fresh.png'});
    await Promise.all(frames.map(async(frame,i)=>{
      await frame.getByLabel('日期',{exact:true}).fill('2026-11-19');await frame.getByLabel('msgSn',{exact:true}).fill('000'+i);
      await expect(frame.getByRole('button',{name:'开始排查',exact:true})).toBeEnabled();
      await frame.getByRole('button',{name:'开始排查',exact:true}).click();await expect(frame.getByText(/仅本会话结果：/)).toBeVisible({timeout:20000});
    }));
    assert.equal(f.calls.length,3);assert.equal(f.prompts.length,3);
    // Closed process details must not leak raw tool input/output into the page.
    const toolDetails=await frames[0].locator('body').evaluate(body=>{
      const details=document.createElement('details');details.className='turn-process';
      details.innerHTML='<summary>执行过程</summary><div class="process-list">原生工具输入输出</div>';
      body.appendChild(details);const content=details.querySelector('.process-list')!;
      const closed=getComputedStyle(content).display;
      details.open=true;const opened=getComputedStyle(content).display;details.remove();
      return {closed,opened};
    });
    assert.deepEqual(toolDetails,{closed:'none',opened:'grid'});
    // Native active status must surface both a progress cue and an interrupt action.
    const firstResult=await frames[0].getByText(/仅本会话结果：/).innerText();
    const firstSession=firstResult.split('仅本会话结果：').at(-1)!.trim();
    assert.ok(f.nativeSessions.has(firstSession));
    f.active[firstSession]=true;
    await expect(frames[0].getByRole('button',{name:'正在执行 · 中断执行'})).toBeVisible({timeout:10000});
    await expect(frames[0].getByText('正在执行，思考和工具进度会持续更新。')).toBeVisible();
    f.active[firstSession]=false;

    for(const input of f.prompts){assert.deepEqual(input.skills,[{id:'message-skill'}]);const conditions=JSON.parse(input.text.split('\n').find((line:string)=>line.startsWith('{'))!);assert.equal(conditions.day,'2026-11-19');assert.match(conditions.msgSn,/^000[0-2]$/);assert.equal(conditions.from,Date.UTC(2026,10,18,16));assert.equal(conditions.to,Date.UTC(2026,10,19,16));assert.equal(input.text.includes('已查询证据'),false);}
    assert.ok([...f.nativeSessions.values()].every(s=>s.model.id==='kimi-k2'&&s.location.directory===f.workspace));assert.ok([...f.nativeSessions.values()].every(s=>s.permissions?.some((r:{action:string;effect:string})=>r.action==='*'&&r.effect==='deny')));
    for(const width of [1920,1440,768,390]){
      await pages[1].setViewportSize({width,height:1000});
      await expect.poll(async()=>{
        const frame=pages[1].frames().find(v=>v.url().startsWith(origin))!;
        return frame.evaluate(()=>{const panel=document.querySelector('.institution-result-panel')!.getBoundingClientRect(),content=document.querySelector('.institution-message')!.getBoundingClientRect();return Math.abs(content.width-innerWidth)<2&&panel.width>innerWidth*.85&&innerWidth-panel.right<34&&document.documentElement.scrollWidth<=innerWidth+1;});
      }).toBe(true);
    }
    await pages[1].setViewportSize({width:1920,height:1000});await pages[1].screenshot({path:'screenshots/anonymous-institution-result.png'});
    const originalGrants=grants();await f.restart();await new Promise(resolve=>setTimeout(resolve,3300));await f.app.listen({host:'127.0.0.1',port:4186});
    for(const frame of frames){await expect(frame.getByText(/仅本会话结果：/)).toBeVisible();await expect(frame.getByText('服务暂时不可用，正在重新连接当前会话，请勿重复提交。')).toHaveCount(0,{timeout:15000});}
    assert.deepEqual(grants(),originalGrants);assert.equal(f.calls.length,3);
    await frames[1].getByLabel('对话内容').fill('继续解释原因');await frames[1].getByRole('button',{name:'发送 ↑',exact:true}).click();await expect.poll(()=>f.calls.length).toBe(4);assert.equal(f.prompts.at(-1).text,'继续解释原因');
    for(const page of pages){const frame=page.frames().find(v=>v.url().startsWith(origin))!;assert.deepEqual(await frame.evaluate(()=>({local:Object.keys(localStorage),session:Object.keys(sessionStorage)})),{local:[],session:[]});}
    const captured=await Promise.all(requests);assert.ok(captured.every(r=>r.cookie===''));assert.ok(captured.every(r=>!r.url.includes('token=')&&!r.url.includes('ticket=')));assert.equal(captured.filter(r=>r.url.endsWith('/institution-query')).length,0);
    await frames[0].getByRole('button',{name:'结束会话',exact:true}).click();await expect(frames[0].getByRole('status').filter({hasText:'本次会话已结束'})).toBeVisible();assert.equal(f.nativeSessions.size,2);
    await pages[0].reload();await expect(frames[0].getByLabel('msgSn',{exact:true})).toHaveValue('');await expect(frames[0].getByText(/仅本会话结果：/)).toHaveCount(0);assert.equal(f.calls.length,4);await expect(frames[1].getByText(/仅本会话结果：/)).toBeVisible();await context.close();
  }finally{await browser?.close();host.closeAllConnections();await new Promise<void>(resolve=>host.close(()=>resolve()));await f.close();}
});
