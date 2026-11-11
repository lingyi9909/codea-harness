import { setTimeout as delay } from 'node:timers/promises';
import { dispatchControl, syncInteractions } from './native-controls.ts';
import { randomUUID } from 'node:crypto';
import type { Pool, RowDataPacket } from 'mysql2/promise';
import type { SessionBinding } from '../../../../packages/contracts/src/domain.ts';
import type { NativeEnvelope,NativeObservation } from '../../../../packages/contracts/src/events.ts';
import type { OpenCodeAdapter } from '../../../../packages/contracts/src/ports.ts';
import { withTransaction, type Tx } from '../../../../packages/storage/src/db.ts';
import { appendEvent } from '../../../../packages/storage/src/events.ts';
import { assertFencedLease, type Lease } from '../../../../packages/storage/src/slots.ts';
import { collectReport } from './report-collector.ts';

type Context = { pool: Pool; lease: Lease; binding: SessionBinding; adapter: OpenCodeAdapter;
  inputId: string; durable: boolean; dataRoot: string };
type Terminal = Extract<NativeObservation, {kind:'terminal'}>;

export function nativeCursor(value: unknown): string {
  if (typeof value !== 'string' || !/^(0|[1-9][0-9]{0,63})$/.test(value))
    throw new Error('NATIVE_CURSOR_INVALID');
  return value;
}

async function checkpoint(tx: Tx, c: Context) {
  await assertFencedLease(tx, c.lease);
  const [rows] = await tx.execute<RowDataPacket[]>(
    'SELECT b.native_log_seq,r.status,r.kind FROM wh_session_bindings b JOIN wh_runs r '+
    'ON r.task_id=b.task_id WHERE b.task_id=? AND b.native_session_id=? AND b.service_identity=? '+
    'AND b.location_key=? AND r.id=? AND r.native_input_id=? AND r.lease_fence=? FOR UPDATE',
    [c.lease.taskId,c.binding.sessionId,c.binding.serviceIdentity,c.binding.locationKey,
      c.lease.runId,c.inputId,c.lease.fence]);
  if (rows.length !== 1) throw new Error('NATIVE_BINDING_CHANGED');
  return rows[0];
}

async function persistMessage(tx: Tx, c: Context, m: Extract<NativeObservation,{kind:'message'}>) {
  if (!/^[!-~]{1,128}$/.test(m.nativeMessageId) || typeof m.text !== 'string' ||
      Buffer.byteLength(m.text)>2_000_000 || !['COMPLETE','INTERRUPTED'].includes(m.status))
    throw new Error('NATIVE_MESSAGE_INVALID');
  const [rows] = await tx.execute<RowDataPacket[]>(
    'SELECT id,run_id,role,content,status FROM wh_messages WHERE task_id=? AND native_message_id=? FOR UPDATE',
    [c.lease.taskId,m.nativeMessageId]);
  let id:string;
  if (rows.length) {
    const row = rows[0];
    if (row.run_id!==c.lease.runId || row.role!=='ASSISTANT')
      throw new Error('NATIVE_MESSAGE_CONFLICT');
    if(row.status!=='STREAMING'){
      if(row.content!==m.text || row.status!==m.status)throw new Error('NATIVE_MESSAGE_CONFLICT');
      return;
    }
    id=String(row.id);
    await tx.execute('UPDATE wh_messages SET content=?,status=?,updated_at=UTC_TIMESTAMP(3) WHERE id=?',[m.text,m.status,id]);
  }else{
    id=randomUUID();
    await tx.execute("INSERT INTO wh_messages(id,task_id,run_id,native_message_id,role,content,status,"+
    "created_at,updated_at) VALUES (?,?,?,?,'ASSISTANT',?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))",
    [id,c.lease.taskId,c.lease.runId,m.nativeMessageId,m.text,m.status]);
  }
  await appendEvent(tx,{taskId:c.lease.taskId,runId:c.lease.runId,type:'message.completed',
    data:{messageId:id,status:m.status}});
}

async function finish(c: Context, terminal: Terminal, cursor?:string) {
  await syncInteractions(c);
  const state = await c.adapter.inspect(c.binding);
  if (state.active!==false || state.pendingInputs!==0 || state.pendingInteractions!==0 ||
      state.externalMutation || state.acceptedInputIds.length) throw new Error('TERMINATION_UNCONFIRMED');
  const report = await withTransaction(c.pool,async tx=>{
    const run = await checkpoint(tx,c);
    const status = terminal.outcome;
    if (!['RUNNING','WAITING_INPUT','WAITING_PERMISSION','STOP_REQUESTED'].includes(run.status) ||
        !['SUCCEEDED','FAILED','STOPPED'].includes(status)) throw new Error('RUN_NOT_FINISHABLE');
    const archive = status==='SUCCEEDED' && run.kind!=='DISCUSS';
    if(cursor&&!archive)await tx.execute("UPDATE wh_session_bindings SET native_log_seq=?,continuity=IF(continuity='GAP','GAP','COMPLETE'),"+
      "updated_at=UTC_TIMESTAMP(3) WHERE task_id=?",[cursor,c.lease.taskId]);
    if (archive && !terminal.report) throw new Error('REPORT_WRITE_EVIDENCE_MISSING');
    const next = archive?'COLLECTING':status;
    await tx.execute('UPDATE wh_runs SET status=?,finished_at=IF(?,NULL,UTC_TIMESTAMP(3)),'+
      'updated_at=UTC_TIMESTAMP(3) WHERE id=?',[next,archive,c.lease.runId]);
    await appendEvent(tx,{taskId:c.lease.taskId,runId:c.lease.runId,type:'run.status',
      data:{status:next,previousStatus:run.status,errorCode:null}});
    await tx.execute("UPDATE wh_operations SET status=?,error_code=?,updated_at=UTC_TIMESTAMP(3) "+
      "WHERE run_id=? AND action='RUN_STOP' AND status IN ('ACCEPTED','PROCESSING','UNKNOWN')",
      [status==='STOPPED'?'SUCCEEDED':'FAILED',status==='STOPPED'?null:'RUN_FINISHED_BEFORE_STOP',c.lease.runId]);
    await tx.execute("UPDATE wh_outbox SET status='CANCELLED',updated_at=UTC_TIMESTAMP(3) "+
      "WHERE run_id=? AND action IN ('ANSWER','INTERRUPT') AND status='PENDING'",[c.lease.runId]);
    if (!archive) await tx.execute('UPDATE wh_executor_slots SET run_id=NULL,lease_owner=NULL,'+
      'lease_until=NULL,updated_at=UTC_TIMESTAMP(3) WHERE slot_key=?',[c.lease.slotKey]);
    return archive ? terminal.report : undefined;
  });
  if (report) await collectReport(c,c.lease,{taskId:c.lease.taskId,runId:c.lease.runId,
    nativeSessionId:c.binding.sessionId,serviceIdentity:c.binding.serviceIdentity,
    nativeWriteMessageId:report.writeMessageId,path:report.path,toolOutcome:'SUCCEEDED',
    cursor,coverage:report.coverage,modelTerminated:true,pendingInputs:0,pendingInteractions:0,externalMutation:false});
}

async function* monitoredFrames(c:Context, start:string|undefined, signal:AbortSignal,flushAt:()=>number) {
  const cancel=new AbortController();
  const combined=AbortSignal.any([signal,cancel.signal]);
  const iterator=c.adapter.subscribe(c.binding,start,combined)[Symbol.asyncIterator]();
  let nextPoll=0,nextHistory=0;
  try {
    while(!combined.aborted){
      const ready=new AbortController();
      const read=iterator.next().then(value=>({value}),error=>({error})).finally(()=>ready.abort());
      const wake=AbortSignal.any([combined,ready.signal]);
      do {
        combined.throwIfAborted();
        if(Date.now()>=nextPoll){
          await syncInteractions(c);
          await dispatchControl(c);
          if(!c.durable && Date.now()>=nextHistory && c.adapter.recoverObservations){
            for await(const observation of c.adapter.recoverObservations(c.binding,c.inputId,combined)){
              yield {taskId:c.lease.taskId,runId:c.lease.runId,type:'snapshot',data:{},cursor:undefined,observation};
            }
            nextHistory=Date.now()+5000;
          }
          nextPoll=Date.now()+1000;
        }
        if(flushAt()<=Date.now())yield null;
        if(!wake.aborted) await delay(Math.max(1,Math.min(nextPoll,flushAt())-Date.now()),undefined,{signal:wake})
          .catch(error=>{if(!wake.aborted)throw error;});
      }while(!wake.aborted);
      combined.throwIfAborted();
      const result=await read;
      if('error' in result && !(result.error instanceof TypeError)) throw result.error;
      if('error' in result || result.value.done) throw new Error('NATIVE_STREAM_DISCONNECTED');
      yield result.value.value;
    }
    combined.throwIfAborted();
  }finally{
    cancel.abort();
    await iterator.return?.();
  }
}

export async function collectNativeEvents(c:Context,signal:AbortSignal){
  for(let attempt=0;;attempt++){
    try { await collectConnection(c,signal); return; }
    catch(error){
      if((!c.durable&&!c.adapter.recoverObservations) || signal.aborted || attempt>=2 || !(error instanceof Error) ||
          error.message!=='NATIVE_STREAM_DISCONNECTED') throw error;
      await delay(250*(attempt+1),undefined,{signal});
    }
  }
}

async function collectConnection(c: Context, signal: AbortSignal) {
  const {taskId,runId} = c.lease;
  const start = await withTransaction(c.pool,async tx=>{
    const row = await checkpoint(tx,c);
    if (c.durable) return nativeCursor(row.native_log_seq??'0');
    await tx.execute("UPDATE wh_session_bindings SET continuity='GAP',updated_at=UTC_TIMESTAMP(3) WHERE task_id=?",[taskId]);
    await appendEvent(tx,{taskId,runId,type:'continuity.gap',
      data:{reason:'LIVE_STREAM_WITHOUT_REPLAY',recoveredThroughNativeCursor:row.native_log_seq}},
      JSON.stringify([runId,'live-gap']));
    return undefined;
  });
  let batch:NativeEnvelope[]=[];
  let bytes=0,deadline=Infinity;
  const flush=async()=>{
    if(batch.length)await persistDeltas(c,batch);
    batch=[];bytes=0;deadline=Infinity;
  };
  for await (const frame of monitoredFrames(c,start,signal,()=>deadline)) {
    if(!frame){await flush();continue;}
    if (frame.taskId!==taskId || frame.runId!==runId) throw new Error('EVENT_SCOPE_CHANGED');
    const cursor = c.durable ? nativeCursor(frame.cursor) : undefined;
    const ob=frame.observation;
    if(ob?.kind==='delta' && ob.inputId===c.inputId){
      if(typeof ob.text!=='string'||!ob.text||Buffer.byteLength(ob.text)>16384)throw new Error('NATIVE_DELTA_INVALID');
      const size=Buffer.byteLength(JSON.stringify(ob.text));
      if(batch.length && (size+bytes>4096 || batch[0].observation?.kind!=='delta' ||
        batch[0].observation.nativeMessageId!==ob.nativeMessageId))await flush();
      if(!batch.length)deadline=Date.now()+100;
      batch.push(frame);bytes+=size;
      if(bytes>=4096||Date.now()>=deadline)await flush();
      continue;
    }
    await flush();
    const terminal = await withTransaction(c.pool,async tx=>{
      const row = await checkpoint(tx,c);
      if (cursor && row.native_log_seq!==null && BigInt(cursor)<=BigInt(nativeCursor(row.native_log_seq)))
        return;
      const ob = frame.observation;
      if (ob?.inputId===c.inputId) {
        if (ob.kind==='terminal') return ob;
        if (ob.kind==='message') await persistMessage(tx,c,ob);
        if (ob.kind==='tool') await persistTool(tx,c,ob);
      }
      if (cursor) await tx.execute('UPDATE wh_session_bindings SET native_log_seq=?,updated_at=UTC_TIMESTAMP(3) WHERE task_id=?',
        [cursor,taskId]);
    });
    if (terminal) { await finish(c,terminal,cursor); return; }
  }
  throw new Error('NATIVE_STREAM_ENDED_BEFORE_TERMINATION');
}

async function persistTool(tx: Tx,c: Context,ob: Extract<NativeObservation,{kind:'tool'}>) {
  if (!/^[!-~]{1,128}$/.test(ob.callId) || !/^[\w.-]{1,128}$/.test(ob.tool))
    throw new Error('NATIVE_TOOL_INVALID');
  const scope={taskId:c.lease.taskId,runId:c.lease.runId};
  const data={toolCallId:ob.callId,tool:ob.tool,summary:ob.tool};
  const key=JSON.stringify([scope.runId,ob.callId,ob.phase]);
  if (ob.phase==='started') await appendEvent(tx,{...scope,type:'tool.started',data},key);
  else if (ob.phase==='completed' && ob.outcome && ['SUCCEEDED','FAILED','CANCELLED'].includes(ob.outcome))
    await appendEvent(tx,{...scope,type:'tool.completed',data:{...data,outcome:ob.outcome}},key);
  else throw new Error('NATIVE_TOOL_OUTCOME_INVALID');
}


async function persistDeltas(c:Context,batch:NativeEnvelope[]){
  const {taskId,runId}=c.lease;
  await withTransaction(c.pool,async tx=>{
    const binding=await checkpoint(tx,c);
    let cursor:string|null=binding.native_log_seq;
    const deltas:Array<Extract<NativeObservation,{kind:'delta'}>>=[];
    for(const frame of batch){
      if(c.durable){
        const next=nativeCursor(frame.cursor);
        if(cursor!==null && BigInt(next)<=BigInt(cursor))continue;
        cursor=next;
      }
      if(frame.observation?.kind==='delta')deltas.push(frame.observation);
    }
    if(!deltas.length)return;
    const key=deltas[0].nativeMessageId;
    if(!/^[!-~]{1,128}$/.test(key))throw new Error('NATIVE_MESSAGE_INVALID');
    const [rows]=await tx.execute<RowDataPacket[]>('SELECT id,run_id,status,role,content FROM wh_messages WHERE task_id=? AND native_message_id=? FOR UPDATE',[taskId,key]);
    const row=rows[0];
    if(row && (row.run_id!==runId || row.role!=='ASSISTANT'))throw new Error('NATIVE_MESSAGE_CONFLICT');
    if(!row||row.status==='STREAMING'){
      const original=row?String(row.content):'';
      const points=Array.from(original);
      const offset=points.length;
      for(const d of deltas){
        if(!/^(0|[1-9][0-9]{0,7})$/.test(d.offset)||Number(d.offset)>points.length)throw new Error('NATIVE_DELTA_GAP');
        let i=Number(d.offset);
        for(const char of d.text){
          if(i===points.length)points.push(char);
          else if(points[i]!==char)throw new Error('NATIVE_DELTA_CONFLICT');
          i++;
        }
      }
      const text=points.slice(offset).join('');
      if(text){
        if(Buffer.byteLength(original)+Buffer.byteLength(text)>2_000_000)throw new Error('NATIVE_MESSAGE_INVALID');
        const id=row?String(row.id):randomUUID();
        if(row)await tx.execute('UPDATE wh_messages SET content=CONCAT(content,?),updated_at=UTC_TIMESTAMP(3) WHERE id=?',[text,id]);
        else await tx.execute("INSERT INTO wh_messages(id,task_id,run_id,native_message_id,role,content,status,created_at,updated_at) "+
          "VALUES (?,?,?,?,'ASSISTANT',?,'STREAMING',UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))",[id,taskId,runId,key,text]);
        let chunk='',size=0,position=offset;
        const emit=async()=>{
          await appendEvent(tx,{taskId,runId,type:'message.delta',data:{messageId:id,offset:String(position),text:chunk}});
          position+=Array.from(chunk).length;chunk='';size=0;
        };
        for(const char of text){
          const n=Buffer.byteLength(JSON.stringify(char))-2;
          if(size+n>4096)await emit();
          chunk+=char;size+=n;
        }
        if(chunk)await emit();
      }
    }
    if(c.durable)await tx.execute('UPDATE wh_session_bindings SET native_log_seq=?,updated_at=UTC_TIMESTAMP(3) WHERE task_id=?',[cursor,taskId]);
  });
}
