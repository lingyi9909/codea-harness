import { createHash } from 'node:crypto';
import type { Pool, PoolConnection, RowDataPacket } from 'mysql2/promise';
import type { DTO } from '../../contracts/src/domain.ts';

type DistributedOmit<T,K extends PropertyKey>=T extends unknown?Omit<T,Extract<K,keyof T>>:never;
export type EventInput=DistributedOmit<DTO<'Event'>,'seq'|'occurredAt'>;
export type PersistedEvent=DTO<'Event'>;
const types=new Set(['task.created','scope.ready','run.status','message.delta','message.completed',
  'tool.started','tool.completed','interaction.pending','interaction.resolved',
  'artifact.ready','continuity.gap']);
const maxSeq=18446744073709551615n;
export class EventError extends Error{
  readonly code:string;
  constructor(code:string){super(code);this.code=code;}
}
export function parseEventSeq(value:unknown):string{
  if(typeof value!=='string'||!/^(0|[1-9][0-9]*)$/.test(value)||
    value.length>20||BigInt(value)>maxSeq)throw new EventError('EVENT_CURSOR_INVALID');
  return value;
}
const iso=(s:unknown)=>String(s).replace(' ','T')+'Z';
export function eventFromRow(row:RowDataPacket):PersistedEvent{
  let data:unknown=row.payload;
  if(typeof data==='string'){
    try{data=JSON.parse(data) as unknown;}catch{throw new EventError('EVENT_PAYLOAD_INVALID');}
  }
  if(!data||typeof data!=='object'||!types.has(String(row.event_type)))
    throw new EventError('EVENT_PAYLOAD_INVALID');
  return {taskId:String(row.task_id),runId:row.run_id===null?null:String(row.run_id),
    seq:String(row.seq),type:String(row.event_type),occurredAt:iso(row.created_at),
    data} as PersistedEvent;
}
export async function appendEvent(tx:PoolConnection,event:EventInput,dedupeKey?:string):Promise<string>{
  if(!types.has(event.type)||Buffer.byteLength(JSON.stringify(event.data),'utf8')>16384)
    throw new EventError('EVENT_INVALID_OR_TOO_LARGE');
  const hash=dedupeKey?createHash('sha256').update(dedupeKey).digest('hex'):null;
  const [tasks]=await tx.execute<RowDataPacket[]>(
    'SELECT next_event_seq FROM wh_tasks WHERE id=? FOR UPDATE',[event.taskId]);
  if(!tasks.length)throw new EventError('TASK_NOT_FOUND');
  if(hash){
    const [existing]=await tx.execute<RowDataPacket[]>(
      'SELECT seq FROM wh_events WHERE task_id=? AND dedupe_key=?',
      [event.taskId,hash]);
    if(existing.length)return String(existing[0].seq);
  }
  const next=BigInt(String(tasks[0].next_event_seq));
  if(next<1n||next>=maxSeq)throw new EventError('EVENT_SEQUENCE_EXHAUSTED');
  await tx.execute(
    'INSERT INTO wh_events(task_id,seq,run_id,event_type,dedupe_key,payload,created_at) '+
    'VALUES (?,?,?,?,?,?,UTC_TIMESTAMP(3))',
    [event.taskId,next.toString(),event.runId,event.type,hash,JSON.stringify(event.data)]);
  await tx.execute('UPDATE wh_tasks SET next_event_seq=? WHERE id=?',
    [(next+1n).toString(),event.taskId]);
  return next.toString();
}
export async function taskLastSeq(pool:Pool,ownerId:string,taskId:string):Promise<string>{
  const [rows]=await pool.execute<RowDataPacket[]>(
    'SELECT next_event_seq FROM wh_tasks WHERE id=? AND owner_id=?',[taskId,ownerId]);
  if(!rows.length)throw new EventError('TASK_NOT_FOUND');
  const next=BigInt(String(rows[0].next_event_seq));
  if(next<1n)throw new EventError('EVENT_SEQUENCE_INVALID');
  return (next-1n).toString();
}
export async function readTaskEvents(pool:Pool,taskId:string,after:string,limit=500):
  Promise<PersistedEvent[]>{
  parseEventSeq(after);
  if(!Number.isInteger(limit)||limit<1||limit>500)throw new EventError('EVENT_LIMIT_INVALID');
  const [rows]=await pool.execute<RowDataPacket[]>(
    'SELECT task_id,seq,run_id,event_type,payload,created_at FROM wh_events '+
    'WHERE task_id=? AND seq>? ORDER BY seq ASC LIMIT ?',
    [taskId,after,limit]);
  return rows.map(eventFromRow);
}
