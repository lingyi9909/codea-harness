import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {chromium,expect} from '@playwright/test';
import {createApp} from '../../apps/server/src/app.ts';
import {LocalAuth} from '../../apps/server/src/auth.ts';

test('browser username-only login and logout do not depend on crypto.randomUUID',async t=>{
  const workspace=await mkdtemp(join(tmpdir(),'username-browser-'));
  t.after(()=>rm(workspace,{recursive:true,force:true}));
  const port=31907,origin='http://127.0.0.1:'+port;
  const app=await createApp({auth:new LocalAuth('unused',port),workspace,localOnly:true});
  await app.listen({host:'127.0.0.1',port});t.after(()=>app.close());
  const browser=await chromium.launch({headless:true});t.after(()=>browser.close());
  const page=await browser.newPage();
  await page.addInitScript(()=>Object.defineProperty(crypto,'randomUUID',{value:undefined,configurable:true}));
  const loginBodies:unknown[]=[];
  page.on('request',r=>{if(r.url().endsWith('/auth/local-session'))loginBodies.push(r.postDataJSON());});
  await page.goto(origin);
  await expect(page.getByLabel('手机号',{exact:true})).toHaveCount(0);
  for(const bad of ['123','张三','abc def','abc_def']){
    await page.getByLabel('用户名',{exact:true}).fill(bad);
    await page.getByRole('button',{name:'登录工作台',exact:true}).click();
    await expect(page.getByRole('alert')).toContainText('至少包含一个英文字母');
  }
  assert.equal(loginBodies.length,0);
  for(const name of ['zhang','zhang01']){
    await page.getByLabel('用户名',{exact:true}).fill(name);
    await page.getByRole('button',{name:'登录工作台',exact:true}).click();
    await expect(page.getByRole('navigation')).toBeVisible();
    assert.deepEqual(loginBodies.at(-1),{name});
    await page.getByRole('button',{name:'退出登录'}).click();
    await expect(page.getByRole('heading',{name:'登录工作台',exact:true})).toBeVisible();
    await expect(page.getByRole('navigation')).toHaveCount(0);
    await expect(page.locator('.running-action')).toHaveCount(0);
    assert.equal((await browser.contexts()[0].cookies()).some(c=>c.name==='harness_session'),false);
  }
});
