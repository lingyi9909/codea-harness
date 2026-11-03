import type { DTO, UUID } from './domain.ts';

export type NativeEnvelope = {
  sourceId?: string;
  cursor?: string;
  taskId: UUID;
  runId: UUID;
  /** Actual native event type; T07 maps it to business Event.type after validation. */
  type: string;
  data: Record<string, unknown>;
};
export type Continuity = 'COMPLETE' | 'GAP' | 'UNKNOWN';
