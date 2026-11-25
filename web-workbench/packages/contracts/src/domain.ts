/**
 * 任务与执行领域模型
 *
 * 职责：定义任务、状态或运行相关的共享领域结构。
 * Java 研发阅读提示：区分实体状态和接口展示 DTO，不应直接暴露内部实现字段。
 *
 * 仅补充模块注释，原有代码和接口行为保持不变。
 */
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
