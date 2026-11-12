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
  await writeFile(script,`
    const fs=require('node:fs');
    if(process.argv.slice(2).join(' ')!=='serve --service')process.exit(3);
    const server=require('node:http').createServer((req,res)=>{
      res.setHeader('Content-Type','application/json');
      if(req.url.split('?')[0]==='/api/model')return res.end(JSON.stringify({data:[
        {id:'intranet',providerID:'company',name:'Company fixture',enabled:true}]}));
      if(req.url==='/api/info')return res.end(JSON.stringify({pid:process.pid,version:'2.0.27'}));
      if(req.url==='/api/session'&&req.method==='POST'){
        req.setEncoding('utf8');let body='';
        req.on('data',s=>body+=s);req.on('end',()=>{
          fs.writeFileSync(process.env.HARNESS_OPENCODE_SERVICE_FILE+'.body',body);
          res.writeHead(503);res.end('{}');
        });return;
      }
      res.writeHead(404);res.end('{}');
    });
    server.listen(0,'127.0.0.1',()=>fs.writeFileSync(process.env.HARNESS_OPENCODE_SERVICE_FILE,
      JSON.stringify({url:'http://127.0.0.1:'+server.address().port,pid:process.pid,version:'2.0.27'})));
    setTimeout(()=>process.exit(0),25000);
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
    await page.getByLabel('OpenCode 模型').selectOption({label:'Company fixture · company/intranet'});
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
