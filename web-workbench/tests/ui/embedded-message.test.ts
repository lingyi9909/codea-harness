import {test} from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {chromium,expect} from '@playwright/test';
import {mkdir} from 'node:fs/promises';
import {embedFixture} from '../support/embed-fixture.ts';
test('iframe: three simultaneous visitors need no login/cookies, get independent results, and refresh starts fresh',{timeout:90000},async()=>{
  const origin='http://127.0.0.1:4176',parentOrigin='http://127.0.0.1:4177';
  const fixture=await embedFixture(origin,parentOrigin,false);
  const host=createServer(async(req,res)=>{
    const url=new URL(req.url??'/',parentOrigin);
    const subject=url.searchParams.get('subject')??'alice';
    if(!['alice','bob','carol'].includes(subject)){res.writeHead(403);res.end();return;}
    if(url.pathname==='/ticket'){
      res.setHeader('Content-Type','application/json');res.setHeader('Cache-Control','no-store');res.end(JSON.stringify(await fixture.ticket(subject)));return;
    }
    res.setHeader('Content-Type','text/html');
    res.end(`<!doctype html><title>Host fixture</title><style>body{margin:0}iframe{width:100%;height:100vh;border:0}</style><iframe title="报文排查" src="${origin}/embed/message?parentOrigin=${encodeURIComponent(parentOrigin)}"></iframe><script>
      const frame=document.querySelector('iframe');
      addEventListener('message',async(event)=>{
        if(event.origin!==${JSON.stringify(origin)}||event.source!==frame.contentWindow||event.data?.type!=='workbench.embed.ready')return;
        const response=await fetch('/ticket?subject='+${JSON.stringify(subject)}),data=await response.json();
        frame.contentWindow.postMessage({type:'workbench.embed.launch',nonce:event.data.nonce,ticket:data.ticket},${JSON.stringify(origin)});
      });</script>`);
  });
  let browser:Awaited<ReturnType<typeof chromium.launch>>|undefined;
  try{
    await fixture.app.listen({host:'127.0.0.1',port:4176});
    await new Promise<void>(resolve=>host.listen(4177,'127.0.0.1',resolve));
    browser=await chromium.launch({headless:true,...(process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE?{executablePath:process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE}:{}),args:['--disable-third-party-cookies']});
    const context=await browser.newContext({viewport:{width:1280,height:900}});
    await context.addCookies([{name:'harness_session',value:'unrelated-workbench-login',url:origin}]);
    const apiCookies:Promise<string>[]=[];
    context.on('request',request=>{if(request.url().startsWith(origin+'/api/v1/embed/'))apiCookies.push(request.allHeaders().then(h=>h.cookie??''));});
    const pages=await Promise.all(['alice','bob','carol'].map(async subject=>{const page=await context.newPage();await page.goto(parentOrigin+'/?subject='+subject);return page;}));
    const frames=pages.map(page=>page.frameLocator('iframe'));
    for(const frame of frames){
      await expect(frame.getByLabel('msgSn',{exact:true})).toBeVisible();
      await expect(frame.getByRole('button',{name:'登录工作台',exact:true})).toHaveCount(0);
      await expect(frame.getByRole('button',{name:'历史对话',exact:true})).toHaveCount(0);
      await expect(frame.locator('.sidebar')).toHaveCount(0);
    }
    await Promise.all(frames.map(async(frame,i)=>{await frame.getByLabel('msgSn',{exact:true}).fill('000'+i);await frame.getByRole('button',{name:'开始排查',exact:true}).click();await expect(frame.getByText(/仅本会话结果：/)).toBeVisible({timeout:20000});}));
    assert.equal(fixture.calls.length,3);
    for(let i=0;i<3;i++){
      const own=[...fixture.nativeSessions.values()].find(s=>s.metadata.harnessOwner===fixture.owners[i]);assert.ok(own);
      await expect(frames[i].getByText('仅本会话结果：'+own.id,{exact:true})).toBeVisible();
      const frame=pages[i].frames().find(f=>f.url().startsWith(origin+'/embed/message'))!;
      assert.deepEqual(await frame.evaluate(()=>({local:Object.keys(localStorage),tab:Object.keys(sessionStorage)})),{local:[],tab:[]});
    }
    assert.equal((await context.cookies()).find(c=>c.name==='harness_session')?.value,'unrelated-workbench-login');
    assert.ok((await Promise.all(apiCookies)).every(cookie=>cookie===''),'embedded API and SSE must omit existing workbench cookies');
    const previousCount=fixture.calls.length;
    await pages[0].reload();await expect(frames[0].getByLabel('msgSn',{exact:true})).toHaveValue('');
    await expect(frames[0].getByText(/仅本会话结果：/)).toHaveCount(0);
    await expect(frames[1].getByText(/仅本会话结果：/)).toBeVisible();assert.equal(fixture.calls.length,previousCount);
    await mkdir('screenshots',{recursive:true});await pages[0].screenshot({path:'screenshots/embedded-message-fresh.png'});await pages[1].screenshot({path:'screenshots/embedded-message-result.png'});
    await context.close();
  }finally{
    await browser?.close();host.closeAllConnections();await new Promise<void>(resolve=>host.close(()=>resolve()));await fixture.close();
  }
});
