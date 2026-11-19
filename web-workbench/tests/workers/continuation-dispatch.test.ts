import { describe,it } from 'node:test';
import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import type { OpenCodeAdapter } from '../../packages/contracts/src/ports.ts';
import { invokePreparedNative } from '../../apps/server/src/workers/scheduler.ts';

describe('T06/T08 native continuation dispatch selection (no real model)',()=>{
  it('calls fixed Review command only for first REVIEW or RECHECK',async()=>{
    const calls:unknown[]=[];
    const adapter={
      startReview:async(_binding:unknown,input:unknown)=>{
        calls.push({kind:'REVIEW',input});return{state:'ACCEPTED',inputId:'n1'};
      },
      discuss:async(_binding:unknown,input:unknown)=>{
        calls.push({kind:'DISCUSS',input});return{state:'ACCEPTED',inputId:'n2'};
      },
    } as unknown as OpenCodeAdapter;
    const binding={taskId:randomUUID(),serviceIdentity:'fixture-service',
      sessionId:'fixture-session',locationKey:'C:/fixture'};
    const firstRun=randomUUID(),secondRun=randomUUID(),discussionRun=randomUUID();
    const first=await invokePreparedNative(adapter,{binding,kind:'REVIEW',
      controller:'OrderController'},firstRun);
    assert.equal(first.state,'ACCEPTED');
    assert.deepEqual(calls[0],{kind:'REVIEW',input:{
      runId:firstRun,controller:'OrderController',
    }});
    await invokePreparedNative(adapter,{binding,kind:'RECHECK',
      controller:'OrderController'},secondRun);
    assert.deepEqual(calls[1],{kind:'REVIEW',input:{
      runId:secondRun,controller:'OrderController',recheck:true,
    }});
    const msg=randomUUID(),report=randomUUID();
    await invokePreparedNative(adapter,{binding,kind:'DISCUSS',discussion:{
      messageId:msg,text:'请继续核查金额处理',quote:'HIGH 资金风险',reportArtifactId:report,
    }},discussionRun);
    assert.deepEqual(calls[2],{kind:'DISCUSS',input:{
      runId:discussionRun,messageId:msg,text:'请继续核查金额处理',
      quote:'HIGH 资金风险',reportArtifactId:report,
    }});
  });
  it('refuses to send DISCUSS without a committed user message',async()=>{
    const adapter={
      startReview:async()=>{throw new Error('must not call');},
      discuss:async()=>{throw new Error('must not call');},
    } as unknown as OpenCodeAdapter;
    const binding={taskId:randomUUID(),serviceIdentity:'fixture',sessionId:'fixture',
      locationKey:'C:/fixture'};
    await assert.rejects(()=>invokePreparedNative(adapter,{binding,kind:'DISCUSS'},randomUUID()),
      /DISCUSSION_INPUT_MISSING/);
  });
});
