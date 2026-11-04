import type { DTO } from '../../../../packages/contracts/src/domain.ts';

export type Repository = DTO<'Repository'>;
export type RepositoryPage = DTO<'RepositoryPage'>;
export type RefList = DTO<'Refs'>;
export type ReviewPreview = DTO<'Preview'>;
export type TaskDetail = DTO<'TaskDetail'>;
export type TaskSummary = DTO<'TaskSummary'>;
export type TaskPage = DTO<'TaskPage'>;
export type MessagePage = DTO<'MessagePage'>;
export type Artifact = DTO<'Artifact'>;
export type RunPage = DTO<'RunPage'>;
export type Interaction = DTO<'Interaction'>;
export type InteractionList = DTO<'InteractionList'>;
export type Operation = DTO<'Operation'>;
export type Bootstrap = DTO<'Bootstrap'>;
export type FileList = DTO<'FileList'>;
export type FileLines = DTO<'FileLines'>;
export type Health = DTO<'Health'>;

const PREFIX = '/api/v1';
let csrfToken: string | null = null;
export const setCsrf = (token: string | null) => { csrfToken = token; };
export const operationId = (): string => crypto.randomUUID();

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly retryable: boolean;
  constructor(status: number, code: string, message: string, retryable = false) {
    super(message); this.name = 'ApiError'; this.status = status; this.code = code; this.retryable = retryable;
  }
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  if (!path.startsWith('/') || path.startsWith('//')) throw new Error('INVALID_API_PATH');
  const headers = new Headers(init.headers);
  if (init.body !== undefined) headers.set('Content-Type', 'application/json');
  if (csrfToken && init.method && !['GET','HEAD'].includes(init.method.toUpperCase()))
    headers.set('X-CSRF-Token', csrfToken);
  let response: Response;
  try {
    response = await fetch(PREFIX + path, {
      ...init, headers, credentials: 'same-origin', redirect: 'error', cache: 'no-store',
    });
  } catch {
    throw new ApiError(0, 'NETWORK_DISCONNECTED', '本地服务连接中断，请检查服务状态。', true);
  }
  if (response.status === 204) return undefined as T;
  let body: unknown;
  try { body = await response.json() as unknown; }
  catch { throw new ApiError(response.status, 'INVALID_RESPONSE', '服务响应不是有效 JSON。'); }
  if (!response.ok) {
    const o = body && typeof body === 'object' ? body as Record<string, unknown> : {};
    const e = o.error && typeof o.error === 'object' ? o.error as Record<string, unknown> : {};
    throw new ApiError(response.status, String(e.code ?? 'HTTP_ERROR'), String(e.message ?? '请求失败'), e.retryable === true);
  }
  if (!body || typeof body !== 'object' || !('data' in body)) throw new ApiError(response.status, 'INVALID_RESPONSE', '缺少 data 响应字段');
  return (body as {data:T}).data;
}

const post = <T>(path: string, data: Record<string, unknown>) =>
  api<T>(path, { method: 'POST', body: JSON.stringify(data) });

export const apiClient = {
  health: () => api<Health>('/health'),
  bootstrap: async () => { const value = await api<Bootstrap>('/bootstrap'); setCsrf(value.csrfToken); return value; },
  pair: (pairingCode: string) => post<unknown>('/auth/session', { pairingCode }),
  logout: () => api<void>('/auth/session', { method: 'DELETE' }),
  repositories: () => api<RepositoryPage>('/repositories?limit=100'),
  addRepository: (name: string, path: string) => post<Repository>('/repositories', { operationId: operationId(), name, path }),
  refs: (repoId: string) => api<RefList>('/repositories/' + encodeURIComponent(repoId) + '/refs'),
  preview: (repositoryId: string, baseRefId: string, id = operationId()) =>
    post<ReviewPreview>('/review-previews', { operationId:id, repositoryId, baseRefId }),
  createTask: (previewId: string, controller: string, notes: string, id = operationId()) =>
    post<Operation>('/tasks', { operationId:id, previewId, ...(controller ? {controller} : {}), ...(notes ? {notes} : {}) }),
  operation: (id: string) => api<Operation>('/operations/' + encodeURIComponent(id)),
  tasks: () => api<TaskPage>('/tasks?limit=100'),
  task: (id: string) => api<TaskDetail>('/tasks/' + encodeURIComponent(id)),
  runs: (id: string) => api<RunPage>('/tasks/' + encodeURIComponent(id) + '/runs?limit=100'),
  messages: (id: string) => api<MessagePage>('/tasks/' + encodeURIComponent(id) + '/messages?limit=100'),
  interactions: (id: string) => api<InteractionList>('/tasks/' + encodeURIComponent(id) + '/interactions'),
  artifact: (id: string) => api<Artifact>('/artifacts/' + encodeURIComponent(id)),
  artifactDownload: (id: string) => PREFIX + '/artifacts/' + encodeURIComponent(id) + '/download',
  files: (id: string) => api<FileList>('/tasks/' + encodeURIComponent(id) + '/files'),
  lines: (id: string, fileId: string, startLine = 1, endLine = 200) =>
    api<FileLines>('/tasks/' + encodeURIComponent(id) + '/files/' + encodeURIComponent(fileId) + '?startLine=' + startLine + '&endLine=' + endLine),
  stop: (runId: string, id = operationId()) => post<Operation>('/runs/' + encodeURIComponent(runId) + '/stop', { operationId:id }),
  retry: (runId: string, id = operationId()) => post<Operation>('/runs/' + encodeURIComponent(runId) + '/retries', { operationId:id }),
  recheck: (taskId: string, id = operationId()) => post<Operation>('/tasks/' + encodeURIComponent(taskId) + '/rechecks', { operationId:id }),
  discuss: (taskId: string, text: string, reportArtifactId?: string, quote?: string, id = operationId()) =>
    post<Operation>('/tasks/' + encodeURIComponent(taskId) + '/messages', { operationId:id, text, ...(reportArtifactId ? {reportArtifactId}:{}), ...(quote ? {quote}: {}) }),
  answer: (id: string, revision: number, answer: DTO<'AnswerRequest'>['answer'], op = operationId()) =>
    post<Operation>('/interactions/' + encodeURIComponent(id) + '/answers', {operationId:op,revision,answer}),
};
