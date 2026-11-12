import type { ChatStatus,ChatSettings,ChatAccessEntry,ChatModel } from '../../../../packages/contracts/src/chat.ts';
import type { components } from '../../../../packages/contracts/src/api.generated.ts';

export type Schema<K extends keyof components['schemas']> = components['schemas'][K];
export class ApiError extends Error {
  constructor(readonly status: number, readonly code: string, message: string) { super(message); }
}
export class HarnessApi {
  private csrf = '';
  setCsrf(token: string): void { this.csrf = token; }
  private async request<T>(path: string, init: RequestInit = {}): Promise<T> {
    if (!path.startsWith('/') || path.startsWith('//') || path.includes('..'))
      throw new Error('UNSAFE_API_PATH');
    const verb = init.method?.toUpperCase() ?? 'GET';
    const headers: Record<string, string> = { Accept: 'application/json' };
    if (init.body) headers['Content-Type'] = 'application/json';
    if (!['GET','HEAD'].includes(verb)) {
      if (!this.csrf) throw new ApiError(0, 'CSRF_REQUIRED', '本机会话未就绪，请重新连接');
      headers['X-CSRF-Token'] = this.csrf;
    }
    let response: Response;
    try {
      response = await fetch('/api/v1' + path, {
        ...init, method: verb, credentials: 'same-origin', redirect: 'error',
        headers: { ...headers, ...(init.headers ?? {}) }, cache: 'no-store',
      });
    } catch {
      throw new ApiError(0, 'OFFLINE', '连接已中断，操作是否受理尚不确定；请查询操作状态');
    }
    const raw = await response.text();
    let data: unknown;
    try { data = raw ? JSON.parse(raw) as unknown : null; }
    catch { throw new ApiError(response.status, 'INVALID_RESPONSE', '服务返回非 JSON 数据'); }
    if (!response.ok) {
      const error = (data as {error?:{code?:string;message?:string}}|null)?.error;
      throw new ApiError(response.status, error?.code ?? 'HTTP_' + response.status,
        error?.message ?? '服务请求失败');
    }
    if (typeof data !== 'object' || data === null || !('data' in data))
      throw new ApiError(response.status, 'CONTRACT_MISMATCH', '服务返回值不符合 OpenAPI 契约');
    return (data as {data:T}).data;
  }
  async pair(pairingCode: string): Promise<Schema<'Bootstrap'>> {
    const response = await fetch('/api/v1/auth/session', {
      method: 'POST', credentials: 'same-origin', redirect: 'error', cache: 'no-store',
      headers: { 'Content-Type':'application/json', Accept:'application/json' },
      body: JSON.stringify({ pairingCode }),
    });
    const body = await response.json() as { data?: Schema<'Bootstrap'>; error?: { message?: string } };
    if (!response.ok || !body.data) throw new ApiError(response.status, 'PAIRING_FAILED',
      body.error?.message ?? '配对失败，请检查一次性配对码');
    this.setCsrf(body.data.csrfToken);
    return body.data;
  }
  chatSettings(){return this.request<ChatSettings>('/chat/settings');}
  chatAccess(){return this.request<ChatAccessEntry[]>('/chat/access-log');}
  chatProject(directory:string){return this.request<{directory:string}>('/chat/project',{method:'POST',body:JSON.stringify({directory})});}
  chatModels(directory?:string){return this.request<ChatModel[]>('/chat/models',{method:'POST',body:JSON.stringify({directory})});}
  newChat(directory?:string,model?:Pick<ChatModel,'id'|'providerID'>,scenario?:'log-inspection'){return this.request<{id:string;directory:string}>('/chat/sessions',{method:'POST',body:JSON.stringify({directory,model,scenario})});}
  chat(id:string,expanded?:{message:string;part:number}){
    const query=expanded?'?'+new URLSearchParams({message:expanded.message,part:String(expanded.part)}):'';
    return this.request<ChatStatus>('/chat/sessions/'+encodeURIComponent(id)+query);
  }
  chatAction(id:string,action:'messages'|'permission'|'interrupt',body:object={}){
    return this.request<unknown>('/chat/sessions/'+encodeURIComponent(id)+'/'+action,{method:'POST',body:JSON.stringify(body)});
  }
  bootstrap() { return this.request<Schema<'Bootstrap'>>('/bootstrap'); }
  repositories() { return this.request<Schema<'RepositoryPage'>>('/repositories?limit=100'); }
  refs(repositoryId: string) { return this.request<Schema<'Refs'>>('/repositories/'+encodeURIComponent(repositoryId)+'/refs'); }
  register(name: string,path:string,operationId:string) {
    return this.request<Schema<'Repository'>>('/repositories',{method:'POST',body:JSON.stringify({name,path,operationId})});
  }
  preview(repositoryId:string,baseRefId:string,operationId:string) {
    return this.request<Schema<'Preview'>>('/review-previews',{method:'POST',
      body:JSON.stringify({repositoryId,baseRefId,operationId})});
  }
  createTask(previewId:string,operationId:string,controller?:string,notes?:string) {
    return this.request<Schema<'Operation'>>('/tasks',{method:'POST',body:JSON.stringify({
      previewId,operationId,...(controller?{controller}:{}),...(notes?{notes}:{})})});
  }
  operation(operationId:string) { return this.request<Schema<'Operation'>>('/operations/'+encodeURIComponent(operationId)); }
  tasks() { return this.request<Schema<'TaskPage'>>('/tasks?limit=100'); }
  task(taskId:string) { return this.request<Schema<'TaskDetail'>>('/tasks/'+encodeURIComponent(taskId)); }
  messages(taskId:string) { return this.request<Schema<'MessagePage'>>('/tasks/'+encodeURIComponent(taskId)+'/messages?limit=100'); }
  interactions(taskId:string) { return this.request<Schema<'InteractionList'>>('/tasks/'+encodeURIComponent(taskId)+'/interactions'); }
  answer(interactionId:string,revision:number,answer:Schema<'AnswerRequest'>['answer'],operationId:string) {
    return this.request<Schema<'Operation'>>('/interactions/'+encodeURIComponent(interactionId)+'/answers',{
      method:'POST',body:JSON.stringify({revision,answer,operationId})});
  }
  retry(runId:string,operationId:string){return this.request<Schema<'Operation'>>('/runs/'+encodeURIComponent(runId)+'/retries',
    {method:'POST',body:JSON.stringify({operationId})});}
  stop(runId:string,operationId:string) {
    return this.request<Schema<'Operation'>>('/runs/'+encodeURIComponent(runId)+'/stop',{
      method:'POST',body:JSON.stringify({operationId})});
  }
  discuss(taskId:string,text:string,operationId:string,reportArtifactId?:string,quote?:string) {
    return this.request<Schema<'Operation'>>('/tasks/'+encodeURIComponent(taskId)+'/messages',{
      method:'POST',body:JSON.stringify({text,operationId,...(reportArtifactId?{reportArtifactId}:{}),
        ...(quote?{quote}:{})})});
  }
  recheck(taskId:string,operationId:string) {
    return this.request<Schema<'Operation'>>('/tasks/'+encodeURIComponent(taskId)+'/rechecks',{
      method:'POST',body:JSON.stringify({operationId})});
  }
  files(taskId:string){return this.request<Schema<'FileList'>>('/tasks/'+encodeURIComponent(taskId)+'/files');}
  lines(taskId:string,id:string,start:number){return this.request<Schema<'FileLines'>>('/tasks/'+encodeURIComponent(taskId)+'/files/'+encodeURIComponent(id)+'?startLine='+start+'&endLine='+(start+199));}
  report(id:string) { return this.request<Schema<'Artifact'>>('/artifacts/'+encodeURIComponent(id)); }
}
export const api = new HarnessApi();
