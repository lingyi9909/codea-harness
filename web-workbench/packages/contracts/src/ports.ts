import type { DTO, NativeAcceptance, NativeInspection, SessionBinding, Snapshot, UUID } from './domain.ts';
import type { NativeEnvelope } from './events.ts';

/** Harness port, never an assertion that the company's OpenCode SDK exposes these exact methods. */
export interface OpenCodeAdapter {
  probe(): Promise<DTO<'Capability'>[]>;
  createSession(taskId: UUID, snapshot: Snapshot): Promise<SessionBinding>;
  putContext(binding: SessionBinding, context: { scope: DTO<'Scope'>; notes?: string }): Promise<void>;
  startReview(binding: SessionBinding, input: { runId: UUID; controller?: string; recheck?: boolean }): Promise<NativeAcceptance>;
  discuss(binding: SessionBinding, input: { runId: UUID; messageId: UUID; text: string; quote?: string; reportArtifactId?: string }): Promise<NativeAcceptance>;
  inspect(binding: SessionBinding): Promise<NativeInspection>;
  readMessages(binding: SessionBinding, cursor?: string): Promise<{ items: DTO<'Message'>[]; nextCursor: string | null }>;
  subscribe(binding: SessionBinding, cursor?: string, signal?: AbortSignal): AsyncIterable<NativeEnvelope>;
  interrupt(binding: SessionBinding): Promise<{ responseReceived: boolean; interrupted?: boolean }>;
  listInteractions(binding: SessionBinding): Promise<Array<DTO<'Interaction'> & { nativeRequestId: string }>>;
  answer(binding: SessionBinding, nativeRequestId: string, answer: DTO<'AnswerRequest'>['answer']): Promise<NativeAcceptance>;
}
