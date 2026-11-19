import {test} from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {mkdir} from 'node:fs/promises';
import {chromium,expect} from '@playwright/test';
import {sqliteEmbedFixture} from '../support/sqlite-embed-fixture.ts';
import {DatabaseSync} from 'node:sqlite';
test('anonymous institution: one OpenCode service, SQLite file, one workspace, iframe-only host, three pages, only day/msgSn and backend restart',{timeout:90000},async()=>{
  const origin='http://127.0.0.1:4186',parent='http://127.0.0.1:4187';let queryCalls=0;
  const f=await sqliteEmbedFixture(origin,parent,false,async input=>{queryCalls++;assert.deepEqual(Object.keys(input).sort(),['day','from','msgSn','to']);return {evidence:'查询报文 '+input.msgSn+' 的应答状态为失败。'};});
  const grants=()=>{const db=new DatabaseSync(f.path,{readOnly:true});try{return db.prepare('SELECT id FROM wh_embed_sessions ORDER BY id').all().map(r=>r.id);}finally{db.close();}};
  const host=createServer((_req,res)=>{res.setHeader('Content-Type','text/html');res.end(`<!doctype html><title>Host iframe only</title><style>body{margin:0}iframe{width:100%;height:100vh;border:0}</style><iframe title="机构报文排查" src="${origin}/embed/institution-message"></iframe>`);});
  let browser:Awaited<ReturnType<typeof chromium.launch>>|undefined;
  try{
    await f.app.listen({host:'127.0.0.1',port:4186});await new Promise<void>(resolve=>host.listen(4187,'127.0.0.1',resolve));
    browser=await chromium.launch({headless:true,...(process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE?{executablePath:process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE}:{}),args:['--disable-third-party-cookies']});
    const context=await browser.newContext({viewport:{width:1440,height:1000}});await context.addCookies([{name:'harness_session',value:'unrelated-login',url:origin}]);
    const requests:Promise<{url:string;cookie:string;body:string}>[]=[];
    context.on('request',request=>{if(request.url().startsWith(origin+'/api/'))requests.push(request.allHeaders().then(h=>({url:request.url(),cookie:h.cookie??'',body:request.postData()??''})));});
    const pages=await Promise.all([0,1,2].map(async()=>{const page=await context.newPage();await page.goto(parent);return page;})),frames=pages.map(p=>p.frameLocator('iframe'));
    for(const frame of frames){for(const label of ['账号','密码','验证码'])await expect(frame.getByLabel(label,{exact:true})).toHaveCount(0);await expect(frame.getByLabel('日期',{exact:true})).toBeVisible();await expect(frame.getByRole('button',{name:'历史对话',exact:true})).toHaveCount(0);await expect(frame.locator('.sidebar')).toHaveCount(0);await expect(frame.getByLabel('Trace ID')).toHaveCount(0);}
    assert.equal(grants().length,3,'StrictMode must not create duplicate grants');
    await mkdir('screenshots',{recursive:true});await pages[0].screenshot({path:'screenshots/anonymous-institution-fresh.png'});
    await Promise.all(frames.map(async(frame,i)=>{
      await frame.getByLabel('日期',{exact:true}).fill('2026-11-18');await frame.getByLabel('msgSn',{exact:true}).fill('000'+i);
      await frame.getByRole('button',{name:'开始排查',exact:true}).click();await expect(frame.getByText(/仅本会话结果：/)).toBeVisible({timeout:20000});
    }));
    assert.equal(f.calls.length,3);assert.equal(queryCalls,3);assert.ok([...f.nativeSessions.values()].every(s=>s.location.directory===f.workspace));assert.ok([...f.nativeSessions.values()].every(s=>s.permissions?.some((r:{action:string;effect:string})=>r.action==='*'&&r.effect==='deny')));const originalGrants=grants();
    await f.restart();await new Promise(resolve=>setTimeout(resolve,3300));await f.app.listen({host:'127.0.0.1',port:4186});
    for(const frame of frames){await expect(frame.getByText(/仅本会话结果：/)).toBeVisible();await expect(frame.getByText('服务暂时不可用，正在重新连接当前会话，请勿重复提交。')).toHaveCount(0,{timeout:15000});}
    assert.deepEqual(grants(),originalGrants);assert.equal(f.calls.length,3);assert.equal(queryCalls,3);
    await frames[1].getByLabel('对话内容').fill('继续解释原因');await frames[1].getByRole('button',{name:'发送 ↑',exact:true}).click();await expect.poll(()=>f.calls.length).toBe(4);assert.equal(queryCalls,3);
    for(const page of pages){const frame=page.frames().find(frame=>frame.url().startsWith(origin))!;assert.deepEqual(await frame.evaluate(()=>({local:Object.keys(localStorage),session:Object.keys(sessionStorage)})),{local:[],session:[]});}
    const captured=await Promise.all(requests);assert.ok(captured.every(r=>r.cookie===''));assert.ok(captured.every(r=>!r.url.includes('token=')&&!r.url.includes('ticket=')));
    const queries=captured.filter(r=>r.url.endsWith('/institution-query'));assert.equal(queries.length,3);
    for(const r of queries){const body=JSON.parse(r.body);assert.deepEqual(Object.keys(body).sort(),['day','from','msgSn','to']);assert.equal(body.to-body.from,86400000);assert.equal(body.from%1000,0);assert.equal(body.to%1000,0);}
    await pages[1].screenshot({path:'screenshots/anonymous-institution-result.png'});
    await frames[0].getByRole('button',{name:'结束会话',exact:true}).click();await expect(frames[0].getByRole('status')).toContainText('本次会话已结束');assert.equal(f.nativeSessions.size,2);
    await pages[0].reload();await expect(frames[0].getByLabel('msgSn',{exact:true})).toHaveValue('');await expect(frames[0].getByText(/仅本会话结果：/)).toHaveCount(0);assert.equal(f.calls.length,4);assert.equal(queryCalls,3);await expect(frames[1].getByText(/仅本会话结果：/)).toBeVisible();await context.close();
  }finally{await browser?.close();host.closeAllConnections();await new Promise<void>(resolve=>host.close(()=>resolve()));await f.close();}
});
