import type { DTO, UUID } from './domain.ts';

export type NativeEnvelope = {
  sourceId?: string;
  cursor?: string;
  taskId: UUID;
  runId: UUID;
  type: DTO<'Event'>['type'];
  data: Record<string, unknown>;
};
export type Continuity = 'COMPLETE' | 'GAP' | 'UNKNOWN';
