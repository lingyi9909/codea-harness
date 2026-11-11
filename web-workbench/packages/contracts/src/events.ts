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
