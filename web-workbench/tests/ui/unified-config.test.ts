import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFile,mkdir} from 'node:fs/promises';
import {join} from 'node:path';
import {chromium,expect} from '@playwright/test';
import {sqliteEmbedFixture} from '../support/sqlite-embed-fixture.ts';
import {createApp} from '../../apps/server/src/app.ts';
import {LocalAuth} from '../../apps/server/src/auth.ts';
import {EmbeddedAuth} from '../../apps/server/src/embed-auth.ts';

test('standalone config: no workbench mount, independent URL, same saved institution binding and anonymous page',{timeout:60000},async()=>{
  const port=4197,origin='http://127.0.0.1:'+port;
  const f=await sqliteEmbedFixture(origin,origin,false);await f.app.close();
  const app=await createApp({localOnly:true,workspace:f.workspace,auth:new LocalAuth('unused',port),execution:f.execution,
    embedded:new EmbeddedAuth(f.config,f.store),embeddedBindings:f.store.bindings,embeddedStorageCheck:()=>f.store.check()});
  let browser:Awaited<ReturnType<typeof chromium.launch>>|undefined;
  try{
    await app.listen({host:'127.0.0.1',port});browser=await chromium.launch({headless:true});const page=await browser.newPage();
    // Main conversation transport is outside this editor test; configuration,
    // login, filesystem persistence and the anonymous page use the real app.
    await page.route('**/api/v1/chat/models',route=>route.fulfill({json:{data:[]}}));
    await page.route('**/api/v1/chat/history',route=>route.fulfill({json:{data:{items:[]}}}));
    await page.goto(origin+'/');await page.getByLabel('姓名',{exact:true}).fill('zhangzhanhui');await page.getByLabel('手机号',{exact:true}).fill('13800000000');
    await page.getByRole('button',{name:'登录工作台'}).click();await expect(page.getByRole('navigation',{name:'场景导航'})).toBeVisible();
    // Existing public/message entrypoints must keep their original forms and
    // navigation after adding the independent institution iframe.
    const navigation=page.getByRole('navigation',{name:'场景导航'});
    await navigation.getByRole('button',{name:/问题排查/}).click();
    for(const scene of ['公管问题排查','报文问题排查']){
      await navigation.getByRole('button',{name:scene,exact:true}).click();
      await expect(page.getByRole('heading',{name:scene})).toBeVisible();
      for(const field of ['Trace ID','msgSn','关键词'])
        await expect(page.getByLabel(field,{exact:true})).toBeVisible();
      await expect(page.getByRole('button',{name:'调整排查条件'})).toHaveCount(0);
    }
    // Administration is a dedicated ACL-gated parent before recent chats,
    // not an isolated link buried underneath the conversation history.
    const admin=page.getByRole('group',{name:'系统管理'});
    await expect(admin.getByRole('button',{name:'系统管理',exact:true})).toBeVisible();
    for(const name of ['场景配置','接口任务管理','接口接入情况','访问统计','组件测试'])
      await expect(admin.getByRole('link',{name,exact:true})).toBeVisible();
    assert.equal(await page.locator('.sidebar .admin-group').evaluate(node=>{
      const history=document.querySelector('.sidebar .history-heading');
      return history!==null&&Boolean(node.compareDocumentPosition(history)&Node.DOCUMENT_POSITION_FOLLOWING);
    }),true,'system management should appear before recent chat history');
    await admin.getByRole('link',{name:'场景配置',exact:true}).click();
    await expect(page.getByRole('heading',{name:'场景配置'})).toBeVisible();
    await page.goto(origin+'/config');await expect(page.getByRole('heading',{name:'场景配置'})).toBeVisible();await expect(page.getByRole('navigation',{name:'场景导航'})).toHaveCount(0);
    await expect(page.getByRole('heading',{name:'各页面权限管理'})).toBeVisible();
    await expect(page.getByText('Shell / PowerShell / CMD',{exact:true})).toBeVisible();
    await expect(page.getByText('Subagent 调用',{exact:true})).toBeVisible();
    await expect(page.getByLabel('机构报文排查工作目录')).toHaveValue(f.workspace);
    await page.getByLabel('机构报文排查执行类型').selectOption('skill');await page.getByLabel('机构报文排查入口名称').fill('message-query');
    await page.getByRole('button',{name:'保存场景配置'}).click();await expect(page.getByRole('status').last()).toContainText('配置已写入');
    const file=join(f.workspace,'.workbench','scenarios.json'),saved=JSON.parse(await readFile(file,'utf8'));
    assert.equal(saved.scenes['institution-message'].executor.name,'message-query');assert.equal(saved.scenes.message.executor.name,'message-query');
    await page.reload();await expect(page.getByLabel('机构报文排查入口名称')).toHaveValue('message-query');
    await mkdir('screenshots',{recursive:true});await page.screenshot({path:'screenshots/unified-standalone-config.png',fullPage:true});
    await page.goto(origin+'/analytics');
    await expect(page.getByRole('heading',{name:'页面访问统计'})).toBeVisible();
    await expect(page.getByRole('button',{name:'最近 30 天'})).toBeVisible();
    await page.getByRole('button',{name:'最近 30 天'}).click();
    await expect(page.getByRole('button',{name:'最近 30 天'})).toHaveAttribute('aria-pressed','true');
    const visit=await browser.newPage();await visit.goto(origin+'/embed/institution-message');
    await expect(visit.getByRole('button',{name:'结束会话'})).toBeVisible();await expect(visit.getByLabel('姓名',{exact:true})).toHaveCount(0);
    const effective=await visit.evaluate(async()=>{
      // The token stays within HarnessApi; inspect its existing read-only response
      // rather than passing administrative cookies to the embedded page.
      const response=await fetch('/api/v1/scene-config',{credentials:'omit'});return response.status;
    });assert.equal(effective,401);
    await expect(visit.getByRole('button',{name:'开始排查',exact:true})).toBeDisabled();
    await visit.getByLabel('日期',{exact:true}).fill('2026-11-19');await visit.getByLabel('msgSn',{exact:true}).fill('MSG-001');
    for(const label of ['账号','密码','验证码'])await expect(visit.getByLabel(label,{exact:true})).toHaveCount(0);
    await visit.getByRole('button',{name:'开始排查',exact:true}).click();await expect(visit.getByText(/仅本会话结果：/)).toBeVisible();
    assert.equal(f.calls.length,1);assert.equal([...f.nativeSessions.values()][0].metadata.harnessScenario,'institution-message-inspection');
    assert.equal(await readFile(file,'utf8'),JSON.stringify(saved,null,2)+'\n');
    await visit.screenshot({path:'screenshots/unified-institution-result.png',fullPage:true});
  }finally{await browser?.close();await app.close();await f.close();}
});
