import {test} from 'node:test';
import assert from 'node:assert/strict';
import type {ChatMessage,ChatStatus} from '../../packages/contracts/src/chat.ts';
import {completeOnNativeIdle,lacksTerminalEvidence,visibleToolEvidence,type PendingExecution} from '../../apps/web/src/executionLifecycle.ts';

const user=(id:string):ChatMessage=>({id,role:'user',text:id,files:[],tools:[]});
const assistant=(id:string,text:string):ChatMessage=>({id,role:'assistant',text,complete:true,files:[],tools:[],
  steps:[{kind:'text',text,ordinal:0}]});
const idle=(id:string,outcome:'succeeded'|'failed'|'interrupted'='succeeded'):ChatMessage=>
  ({id,role:'assistant',text:'',idleOutcome:outcome,files:[],tools:[],steps:[{kind:'notice',text:'本轮已完成'}]});
const status=(messages:ChatMessage[],active=false):ChatStatus=>
  ({id:'session-1',directory:'/shared',messages,active,permissions:[],forms:[]});

test('phase 1, tool phase, and phase 2 never count as a completed user turn',()=>{
  const before=[user('old-input'),assistant('old-answer','旧结果'),idle('old-idle')];
  const pending:PendingExecution={id:'session-1',baselineMessageIds:before.map(m=>m.id),
    baselineIdleIds:['old-idle'],submittedAt:1000};
  const phase1=status([...before,user('new-input'),assistant('stage-1','阶段 1：开始核查')],false);
  assert.equal(completeOnNativeIdle(pending,phase1),pending,
    'assistant.complete plus active=false is not a turn completion');
  assert.equal(lacksTerminalEvidence(phase1),true);
  // The assistant may finish several tool-call subturns before the native idle.
  const withTool=status([...phase1.messages,{...assistant('tool-step','工具处理中'),toolCallTurn:true,
    steps:[{kind:'tool',name:'dbhub',status:'running'}]}],false);
  assert.equal(completeOnNativeIdle(pending,withTool),pending);
  assert.equal(visibleToolEvidence(withTool),true);
  const phase2=status([...withTool.messages,assistant('stage-2','阶段 2：继续分析')],false);
  assert.equal(completeOnNativeIdle(pending,phase2),pending);
  const finished=status([...phase2.messages,idle('new-idle')],false);
  assert.equal(completeOnNativeIdle(pending,finished),null);
  assert.equal(lacksTerminalEvidence(finished),false);
});

test('old or late idle cannot prematurely finish a newly queued prompt',()=>{
  const before=[user('old-user'),assistant('old-answer','阶段 1')];
  const pending:PendingExecution={id:'session-1',baselineMessageIds:before.map(m=>m.id),baselineIdleIds:[],submittedAt:1};
  assert.equal(completeOnNativeIdle(pending,status([...before,idle('late-old-idle')],false)),pending);
  assert.equal(completeOnNativeIdle(pending,status([...before,idle('late-old-idle'),user('new-user'),assistant('phase','阶段 1')],false)),pending);
  assert.equal(completeOnNativeIdle(pending,status([...before,user('new-user'),assistant('phase','阶段 1'),idle('new-idle','failed')],false)),null);
  assert.equal(completeOnNativeIdle(pending,{...status(before),id:'other-session'}),pending);
});
