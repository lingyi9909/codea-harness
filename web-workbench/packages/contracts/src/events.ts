/**
 * 业务事件类型契约
 *
 * 职责：定义后台事件传输和消费时使用的数据结构。
 * Java 研发阅读提示：事件并不等价于业务最终成功，需结合持久化状态判断。
 *
 * 仅补充模块注释，原有代码和接口行为保持不变。
 */
import type { DTO, UUID } from './domain.ts';

export type NativeEnvelope = {
  observation?: NativeObservation;
  sourceId?: string;
  cursor?: string;
  taskId: UUID;
  runId: UUID;
  /** Actual native event type; T07 maps it to business Event.type after validation. */
  type: string;
  data: Record<string, unknown>;
};
export type Continuity = 'COMPLETE' | 'GAP' | 'UNKNOWN';

/** A pinned-build decoder must correlate each observation to the accepted native input. */
export type NativeObservation = { inputId: string } & (
  | { kind: 'tool'; phase: 'started' | 'completed'; callId: string; tool: string;
      outcome?: 'SUCCEEDED' | 'FAILED' | 'CANCELLED' }
  | { kind: 'delta'; nativeMessageId:string; offset:string; text:string }
  | { kind: 'message'; nativeMessageId: string; text: string; status: 'COMPLETE' | 'INTERRUPTED' }
  | { kind: 'terminal'; outcome: 'SUCCEEDED' | 'FAILED' | 'STOPPED';
      report?: { path: string; writeMessageId: string; coverage: 'FULL' | 'PARTIAL' } }
);
