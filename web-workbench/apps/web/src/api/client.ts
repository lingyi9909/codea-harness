import {BearerEventSource,type ChatEventSource} from './activity-source.ts';
import {configureUserStorage,resetUserStorage} from '../userStorage.ts';
import type { ChatStatus,ChatSettings,ChatHistoryPage,ChatScenario,ChatModel,SceneConfigResponse,SceneConfigValue } from '../../../../packages/contracts/src/chat.ts';
import type { components } from '../../../../packages/contracts/src/api.generated.ts';

export type Schema<K extends keyof components['schemas']> = components['schemas'][K];
export class ApiError extends Error {
  readonly status:number;
  readonly code:string;
  constructor(status: number, code: string, message: string) { super(message);this.status=status;this.code=code; }
}
export class HarnessApi {
  private csrf = '';
  private embedToken='';
  private base='/api/v1';
  businessVerificationEnabled=false;
  private connecting?: Promise<Schema<'Bootstrap'>>;
  setCsrf(token: string): void { this.csrf = token; }
  private async request<T>(path: string, init: RequestInit = {}): Promise<T> {
    if (!path.startsWith('/') || path.startsWith('//') || path.includes('..'))
      throw new Error('UNSAFE_API_PATH');
    const verb = init.method?.toUpperCase() ?? 'GET';
    const headers: Record<string, string> = { Accept: 'application/json' };
    if(this.embedToken)headers.Authorization='Bearer '+this.embedToken;
    if (init.body) headers['Content-Type'] = 'application/json';
    if (!['GET','HEAD'].includes(verb) && path!=='/auth/local-session') {
      if (!this.csrf) throw new ApiError(0, 'CSRF_REQUIRED', '本机会话未就绪，请重新连接');
      headers['X-CSRF-Token'] = this.csrf;
    }
    let response: Response;
    try {
      response = await fetch(this.base + path, {
        ...init, method: verb, credentials: this.embedToken?'omit':'same-origin', redirect: 'error',
        headers: { ...headers, ...(init.headers ?? {}) }, cache: 'no-store',
      });
    } catch {
      throw new ApiError(0, 'OFFLINE', ['GET','HEAD'].includes(verb)
        ? '无法连接本地工作台服务，请确认启动终端仍在运行，然后重试。'
        : '连接已中断，操作是否受理尚不确定；请查询操作状态');
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
    if (response.status===204 && verb==='DELETE' && ['/auth/session','/session'].includes(path)) return undefined as T;
    if (typeof data !== 'object' || data === null || !('data' in data))
      throw new ApiError(response.status, 'CONTRACT_MISMATCH', '服务返回值不符合 OpenAPI 契约');
    return (data as {data:T}).data;
  }
  activity(id:string):ChatEventSource{
    const url=this.base+'/chat/sessions/'+encodeURIComponent(id)+'/activity';
    return this.embedToken?new BearerEventSource(url,{Authorization:'Bearer '+this.embedToken}):new EventSource(url) as unknown as ChatEventSource;
  }
  async openEmbedded(ticket:string,parentOrigin:string){
    const response=await fetch('/api/v1/embed/exchange',{method:'POST',credentials:'omit',cache:'no-store',redirect:'error',headers:{'Content-Type':'application/json'},body:JSON.stringify({ticket,parentOrigin})});
    const body=await response.json();if(!response.ok||!body.data?.token||!body.data?.csrfToken)throw new ApiError(response.status,'EMBED_SESSION_REQUIRED','无法建立排查会话');
    this.base='/api/v1/embed';this.embedToken=body.data.token;this.csrf=body.data.csrfToken;this.businessVerificationEnabled=false;resetUserStorage();
    return body.data.expiresAt as number;
  }
  async closeEmbedded(){try{await this.request<void>('/session',{method:'DELETE'});}finally{this.clearEmbedded();}}
  clearEmbedded(){this.embedToken='';this.csrf='';this.base='/api/v1';resetUserStorage();}
  async pair(pairingCode: string): Promise<Schema<'Bootstrap'>> {
    const response = await fetch('/api/v1/auth/session', {
      method: 'POST', credentials: 'same-origin', redirect: 'error', cache: 'no-store',
      headers: { 'Content-Type':'application/json', Accept:'application/json' },
      body: JSON.stringify({ pairingCode }),
    });
    const body = await response.json() as { data?: Schema<'Bootstrap'>; error?: { message?: string } };
    if (!response.ok || !body.data) throw new ApiError(response.status, 'PAIRING_FAILED',
      body.error?.message ?? '配对失败，请检查一次性配对码');
    configureUserStorage(body.data.owner.id,(body.data as Schema<'Bootstrap'>&{identityMode?:string}).identityMode==='multi-user');
    this.businessVerificationEnabled=(body.data as Schema<'Bootstrap'>&{businessVerification?:boolean}).businessVerification===true;
    this.setCsrf(body.data.csrfToken);
    return body.data;
  }
  businessStatus(id:string){return this.request<{state:string;challengeId?:string;expiresAt?:number;maskedPhone?:string;verified?:boolean}>('/chat/sessions/'+encodeURIComponent(id)+'/business-verification');}
  submitBusinessCode(id:string,challenge:string,code:string){return this.request<{state:string;verified?:boolean}>('/chat/sessions/'+encodeURIComponent(id)+'/business-verification/'+encodeURIComponent(challenge)+'/code',{method:'POST',body:JSON.stringify({code})});}
  cancelBusinessCode(id:string,challenge:string){return this.request<unknown>('/chat/sessions/'+encodeURIComponent(id)+'/business-verification/'+encodeURIComponent(challenge),{method:'DELETE'});}
  chatSettings(){return this.request<ChatSettings>('/chat/settings');}
  sceneConfig(){return this.request<SceneConfigResponse>('/scene-config');}
  saveSceneConfig(config:SceneConfigValue,revision:string){return this.request<SceneConfigResponse>('/scene-config',{method:'PUT',body:JSON.stringify({config,revision})});}
  chatHistory(directory:string|undefined,scenario:ChatScenario,cursor?:string){
    return this.request<ChatHistoryPage>('/chat/history',{method:'POST',body:JSON.stringify({directory,scenario,cursor})});
  }
  chatModels(directory?:string){return this.request<ChatModel[]>('/chat/models',{method:'POST',body:JSON.stringify({directory})});}
  newChat(directory?:string,model?:Pick<ChatModel,'id'|'providerID'|'variant'>,scenario?:ChatScenario,title?:string){return this.request<{id:string;directory:string;title:string}>('/chat/sessions',{method:'POST',body:JSON.stringify({directory,model,scenario,title})});}
  chat(id:string){
    return this.request<ChatStatus>('/chat/sessions/'+encodeURIComponent(id));
  }
  renameChat(id:string,title:string){
    return this.request<{id:string;title:string}>('/chat/sessions/'+encodeURIComponent(id),{
      method:'PATCH',body:JSON.stringify({title}),
    });
  }
  deleteChat(id:string){
    return this.request<{id:string;deleted:true}>('/chat/sessions/'+encodeURIComponent(id),{method:'DELETE'});
  }
  translateReasoning(id:string,message:string,ordinal:number){
    return this.request<{text:string}>('/chat/sessions/'+encodeURIComponent(id)+'/reasoning/translation',{
      method:'POST',body:JSON.stringify({message,ordinal}),
    });
  }
  chatAction(id:string,action:'messages'|'permission'|'interrupt'|'model'|'form',body:object={}){
    return this.request<unknown>('/chat/sessions/'+encodeURIComponent(id)+'/'+action,{method:'POST',body:JSON.stringify(body)});
  }
  bootstrap() {
    this.connecting??=this.request<Schema<'Bootstrap'>>('/bootstrap').then(boot=>{this.businessVerificationEnabled=(boot as Schema<'Bootstrap'>&{businessVerification?:boolean}).businessVerification===true;configureUserStorage(boot.owner.id,(boot as Schema<'Bootstrap'>&{identityMode?:string}).identityMode==='multi-user');return boot;}).finally(()=>{this.connecting=undefined;});
    return this.connecting;
  }
  async login(name:string,phone:string) {
    const session=await this.request<Schema<'Bootstrap'>>('/auth/local-session',{
      method:'POST',body:JSON.stringify({name,phone}),
    });
    this.businessVerificationEnabled=(session as Schema<'Bootstrap'>&{businessVerification?:boolean}).businessVerification===true;
    configureUserStorage(session.owner.id,(session as Schema<'Bootstrap'>&{identityMode?:string}).identityMode==='multi-user');
    this.setCsrf(session.csrfToken);
    window.localStorage.setItem('harness.account-change',crypto.randomUUID());
    return session;
  }
  async logout() {
    await this.request<void>('/auth/session',{method:'DELETE'});
    this.setCsrf('');this.businessVerificationEnabled=false;resetUserStorage();
    window.localStorage.setItem('harness.account-change',crypto.randomUUID());
  }
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
