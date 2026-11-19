import type { components } from './api.generated.ts';

export type UUID = string;
export type Seq = string;
export type DTO<K extends keyof components['schemas']> = components['schemas'][K];

export type ScopeSelection = { repositoryId: UUID; baseRefId: string };
export type Snapshot = {
  taskId: UUID;
  scope: DTO<'Scope'>;
  storageKey: string;
  manifestHash: string;
};
export type SessionBinding = {
  taskId: UUID;
  serviceIdentity: string;
  sessionId: string;
  locationKey: string;
};
export type NativeAcceptance =
  | { state: 'ACCEPTED'; inputId?: string }
  | { state: 'REJECTED'; reason?: string }
  | { state: 'UNKNOWN'; reason?: string };
export type NativeInspection = {
  active: boolean | 'UNKNOWN';
  pendingInputs: number | 'UNKNOWN';
  pendingInteractions: number | 'UNKNOWN';
  acceptedInputIds: string[];
  externalMutation: boolean;
};
