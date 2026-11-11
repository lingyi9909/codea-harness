import { it } from 'node:test';
import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { mkdtemp,mkdir,writeFile,rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { chromium,expect } from '@playwright/test';
import { createApp } from '../../apps/server/src/app.ts';
import { LocalAuth } from '../../apps/server/src/auth.ts';
import { configFromEnv,createPool } from '../../packages/storage/src/db.ts';
import type { OpenCodeAdapter } from '../../packages/contracts/src/ports.ts';
import { CAPABILITIES } from '../../packages/opencode-adapter/src/probe.ts';
import { REPORT_RELATIVE_PATH } from '../../packages/opencode-adapter/src/mapping.ts';

it('Chromium → Fastify → MySQL 8.4 → Git and scripted native review', {timeout:90000},async()=>{
  const config=configFromEnv(process.env);
  assert.equal(config.database,'harness_ci_test');
  const root=await mkdtemp(join(tmpdir(),'workbench-ui-'));
  const repo=join(root,'repo'),dataRoot=join(root,'data');
  const pool=await createPool(config);
  const browser=await chromium.launch();
  process.env.HARNESS_REPOSITORY_ROOTS=repo;
  process.env.HARNESS_DATA_DIR=dataRoot;
  process.env.HARNESS_REF_KEY=randomUUID();process.env.HARNESS_CURSOR_KEY=randomUUID();
  const code=randomUUID(),permissionId=randomUUID(),formId=randomUUID(),sessionId=randomUUID();
  let runId='',answered=false,review=true,stopped=false,discussionCount=0,seq=0;
  let permissionDone=false;
  const discussionIds:string[]=[];
  const report=['评审摘要','调用链与覆盖范围','风险清单','问题明细','后续处理与评审边界'].map(s=>'## '+s+'\nFixture evidence.').join('\n');
  const adapter:OpenCodeAdapter={
    async probe(){return CAPABILITIES.map(c=>({...c,status:'SUPPORTED',evidence:'UI test peer'}));},
    async createSession(taskId,snapshot){return {taskId,serviceIdentity:'UI fixture',sessionId,locationKey:snapshot.storageKey};},
    async putContext(){},
    async startReview(_binding,input){runId=input.runId;review=true;answered=Boolean(input.recheck);return {state:'ACCEPTED',inputId:runId};},
    async discuss(_binding,input){
      runId=input.runId;review=false;stopped=false;discussionIds.push(input.messageId);
      discussionCount++;return {state:'ACCEPTED',inputId:runId};
    },
    async inspect(){return {active:!answered,pendingInputs:0,pendingInteractions:answered?0:1,externalMutation:false,acceptedInputIds:[]};},
    async readMessages(){return {items:[],nextCursor:null};},
    async listInteractions(binding){
      if(answered)return [];
      if(permissionDone)return [{id:formId,taskId:binding.taskId,runId,revision:1,kind:'FORM',status:'PENDING',
        expiresAt:new Date(Date.now()+600000).toISOString(),nativeRequestId:'form',title:'Fixture question',
        fields:[{id:'detail',label:'Evidence detail',type:'TEXT',required:true,maxLength:100,options:[]}]}];
      return [{id:permissionId,taskId:binding.taskId,runId,revision:1,kind:'PERMISSION',
      status:'PENDING',expiresAt:new Date(Date.now()+600000).toISOString(),tool:'read',requestText:'Read OrderController',
      nativeRequestId:'permission',allowedDecisions:['ALLOW_ONCE','DENY']}];},
    async answer(_binding,id,answer){
      if(id===permissionId){assert.deepEqual(answer,{kind:'PERMISSION',decision:'ALLOW_ONCE'});permissionDone=true;}
      else{assert.equal(id,formId);assert.deepEqual(answer,{kind:'FORM',cancel:false,values:{detail:'confirmed'}});answered=true;}
      return {state:'ACCEPTED',inputId:'reply'};},
    async interrupt(){stopped=true;return {responseReceived:true,interrupted:true};},
    async *subscribe(binding,_cursor,signal){
      while(!answered)await delay(25,undefined,{signal});
      const envelope={taskId:binding.taskId,runId,type:'fixture',data:{}};
      if(!review){
        if(discussionCount===1)while(!stopped)await delay(25,undefined,{signal});
        yield {...envelope,cursor:String(++seq),observation:{inputId:runId,kind:'terminal',outcome:stopped?'STOPPED':'SUCCEEDED'}};
        return;
      }
      yield {...envelope,cursor:String(++seq),observation:{inputId:runId,kind:'message',nativeMessageId:'fixture-final-'+runId,
        text:'Review completed in fixture',status:'COMPLETE'}};
      await mkdir(join(binding.locationKey,'temp'),{recursive:true});
      await writeFile(join(binding.locationKey,REPORT_RELATIVE_PATH),report);
      yield {...envelope,cursor:String(++seq),observation:{inputId:runId,kind:'terminal',outcome:'SUCCEEDED',
        report:{path:REPORT_RELATIVE_PATH,coverage:'FULL',writeMessageId:'fixture-write'}}};
    },
  };
  const app=await createApp({pool,auth:new LocalAuth(code,4176),native:{
    adapter,nativeVersion:'UI-fixture',workerId:'ui-worker',dataRoot,repositoryRoots:[repo]}});
  try{
    await mkdir(repo);
    const git=(...args:string[])=>execFileSync('git',['-C',repo,...args],{stdio:'pipe'});
    git('init');git('config','user.name','UI Fixture');git('config','user.email','fixture@example.invalid');
    await writeFile(join(repo,'OrderController.java'),'class OrderController {}\n');
    git('add','.');git('commit','-m','base');git('branch','baseline');
    await writeFile(join(repo,'OrderController.java'),'class OrderController { int amount; }\n');
    git('add','.');git('commit','-m','changed');
    await app.listen({host:'127.0.0.1',port:4176});
    const page=await browser.newPage({viewport:{width:1488,height:900}});
    await page.goto('http://127.0.0.1:4176/');
    await page.getByPlaceholder('请输入本地终端显示的配对码').fill(code);
    await page.getByRole('button',{name:'连接本地工作台'}).click();
    await page.getByRole('navigation').getByRole('button',{name:'代码审查'}).click();
    await page.getByRole('button',{name:'添加本地仓库'}).click();
    const name='UI '+randomUUID();
    await page.getByLabel('仓库名称').fill(name);await page.getByLabel('本地绝对路径').fill(repo);
    await page.getByRole('button',{name:'登记仓库'}).click();
    await page.getByLabel('本地代码仓库').selectOption({label:name});
    await page.getByLabel('基线分支').selectOption({label:'refs/heads/baseline'});
    await page.getByRole('button',{name:'预览变更范围'}).click();
    await expect(page.getByText(/最近预检：1 个已变更文件/)).toBeVisible();
    await page.getByRole('button',{name:'开始 Review'}).click();
    await expect(page.getByRole('heading',{name:'原生工具授权'})).toBeVisible({timeout:20000});
    await page.getByPlaceholder('本轮执行中；暂不可并发发送').fill('saved draft');
    await page.reload();
    await expect(page.getByPlaceholder('本轮执行中；暂不可并发发送')).toHaveValue('saved draft');
    await page.getByRole('button',{name:'仅本次允许'}).click();
    await page.getByLabel('Evidence detail').fill('confirmed');
    await page.getByRole('button',{name:'提交原生表单'}).click();
    await expect(page.locator('.report-text').first()).toContainText('评审摘要',{timeout:20000});
    await expect(page.getByText('Review completed in fixture')).toBeVisible();
    await page.getByRole('button',{name:'查看固定版本源码'}).click();
    await page.getByLabel('文件与版本').selectOption({label:'HEAD · OrderController.java'});
    await expect(page.locator(".report-text").filter({hasText:"class OrderController"})).toContainText('amount');
    await page.getByRole('button',{name:'重新复查固定版本'}).click();
    await expect.poll(async()=>{
      const [rows]=await pool.execute<import('mysql2/promise').RowDataPacket[]>('SELECT COUNT(*) AS n FROM wh_artifacts WHERE task_id=(SELECT task_id FROM wh_runs WHERE id=?)',[runId]);
      return Number(rows[0].n);
    },{timeout:15000}).toBe(2);
    await page.getByPlaceholder('基于固定代码快照继续追问').fill('Explain the fixed revision');
    await page.getByRole('button',{name:'发送追问'}).click();
    await expect.poll(()=>discussionCount,{timeout:10000}).toBe(1);
    await page.getByRole('button',{name:'停止本轮执行'}).click();
    await page.getByRole('button',{name:'重试本轮'}).click();
    await expect.poll(()=>discussionCount,{timeout:15000}).toBe(2);
    await expect(page.locator('.card-head .chip').first()).toHaveText('SUCCEEDED',{timeout:15000});
    assert.equal(discussionIds[0],discussionIds[1]);
    await page.setViewportSize({width:390,height:844});
    assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth),true);
  }finally{await browser.close();await app.close();await pool.end();await rm(root,{recursive:true,force:true});}
});
