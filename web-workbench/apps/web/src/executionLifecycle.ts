/**
 * 执行生命周期状态
 *
 * 职责：将后台执行反馈映射为前端可显示的阶段与动作。
 * Java 研发阅读提示：状态映射只影响显示，不会修改后台真实执行结果。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
import type {ChatMessage,ChatStatus} from '../../../packages/contracts/src/chat.ts';

const lastIndex=(messages:ChatMessage[],matches:(message:ChatMessage)=>boolean)=>{
  for(let index=messages.length-1;index>=0;index--)if(matches(messages[index]))return index;
  return -1;
};

/**
 * Native assistant messages may complete at intermediate stages. Only an idle
 * event belongs to the *whole* user turn; never treat a finished message or a
 * transiently empty session.active() response as the end of an investigation.
 */
export type PendingExecution={
  id:string;
  baselineIdleIds:string[];
  baselineMessageIds:string[];
  submittedAt:number;
};

export function completeOnNativeIdle(pending:PendingExecution|null,status:ChatStatus):PendingExecution|null{
  if(!pending||pending.id!==status.id)return pending;
  const baseline=new Set(pending.baselineIdleIds);
  const previous=new Set(pending.baselineMessageIds);
  const latestUser=lastIndex(status.messages,m=>m.role==='user'&&!previous.has(m.id));
  if(latestUser<0)return pending; // The queued user prompt may not be materialized yet.
  return status.messages.slice(latestUser+1).some(m=>m.idleOutcome!==undefined&&!baseline.has(m.id))?null:pending;
}

/** Last user turn, ignoring older completed turns in the same Session. */
export function lastTurn(status:ChatStatus){
  const from=lastIndex(status.messages,m=>m.role==='user');
  return from<0?[]:status.messages.slice(from+1);
}

/**
 * OpenCode builds without an idle marker may have a visible final assistant
 * message but no reliable signal that all tool calls/agent phases finished.
 * Flag the state as unconfirmed instead of silently removing the running cue.
 */
export function lacksTerminalEvidence(status:ChatStatus){
  const from=lastIndex(status.messages,m=>m.role==='user');
  return from>=0&&!status.messages.slice(from+1).some(m=>m.idleOutcome!==undefined);
}

export function visibleToolEvidence(status:ChatStatus){
  return lastTurn(status).some(m=>m.steps?.some(s=>s.kind==='tool'));
}
