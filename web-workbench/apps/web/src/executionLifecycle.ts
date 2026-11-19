import type {ChatStatus} from '../../../packages/contracts/src/chat.ts';

/**
 * Native assistant messages may complete at intermediate stages. Only an idle
 * event belongs to the *whole* user turn; never treat a finished message or a
 * transiently empty session.active() response as the end of an investigation.
 */
export type PendingExecution={
  id:string;
  baselineIdleIds:string[];
  submittedAt:number;
};

export function completeOnNativeIdle(pending:PendingExecution|null,status:ChatStatus):PendingExecution|null{
  if(!pending||pending.id!==status.id)return pending;
  const baseline=new Set(pending.baselineIdleIds);
  return status.messages.some(m=>m.idleOutcome!==undefined&&!baseline.has(m.id))?null:pending;
}

/** Last user turn, ignoring older completed turns in the same Session. */
export function lastTurn(status:ChatStatus){
  const from=status.messages.findLastIndex(m=>m.role==='user');
  return from<0?[]:status.messages.slice(from+1);
}

/**
 * OpenCode builds without an idle marker may have a visible final assistant
 * message but no reliable signal that all tool calls/agent phases finished.
 * Flag the state as unconfirmed instead of silently removing the running cue.
 */
export function lacksTerminalEvidence(status:ChatStatus){
  const from=status.messages.findLastIndex(m=>m.role==='user');
  return from>=0&&!status.messages.slice(from+1).some(m=>m.idleOutcome!==undefined);
}

export function visibleToolEvidence(status:ChatStatus){
  return lastTurn(status).some(m=>m.steps?.some(s=>s.kind==='tool'));
}
