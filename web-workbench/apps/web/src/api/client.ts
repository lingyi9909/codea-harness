/**
 * 前端 HTTP API 客户端
 *
 * 职责：集中封装任务、配置、登录及机构排查的后端请求。
 * Java 研发阅读提示：泛型 Promise<T> 类比异步 Feign 返回值；TS 类型不会自动验证响应 JSON。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
import {BearerEventSource,type ChatEventSource} from './activity-source.ts';
import {configureUserStorage,resetUserStorage} from '../userStorage.ts';
import type { ChatStatus,ChatSettings,ChatHistoryPage,ChatScenario,ChatModel,SceneConfigResponse,SceneConfigValue,SceneAccessConfig } from '../../../../packages/contracts/src/chat.ts';
import type { components } from '../../../../packages/contracts/src/api.generated.ts';

export type Schema<K extends keyof components['schemas']> = components['schemas'][K];
/**
 * 机构 API 的前端 DTO（编译期类型）。
 *
 * Java 类比：InstitutionTaskStatus 是枚举，InstitutionAdminTask 是响应 DTO。
 * TS type 只帮助编辑器和 tsc 检查，运行时并不会自动验证 HTTP 返回的 JSON。
 * 运行时的参数校验位于 C 后端的 Fastify schema / 权限校验逻辑。
 */
export type InstitutionTaskStatus='QUEUED'|'RUNNING'|'SUCCEEDED'|'FAILED'|'UNKNOWN';
export type InstitutionAdminTask={
  taskId:string;clientId:string;userHash:string;externalUserId:string|null;requestId:string;
  day:string;msgSn:string|null;status:InstitutionTaskStatus;errorCode:string|null;
  hasResult?:boolean;createdAt:string;updatedAt:string;
};
// 交叉类型 A & B：在列表 DTO 的基础上增加 detail 独有字段，类似 Java DTO 继承/组合。
export type InstitutionAdminTaskDetail=InstitutionAdminTask&{result:{summary:string}|null};
export type InstitutionTaskPage={total:number;limit:number;offset:number;items:InstitutionAdminTask[]};
// 数值联合类型：仅接受四种并发档位；提交时后端仍会再次做运行时校验。
export type InstitutionConcurrency=3|5|10|15;
export type InstitutionConcurrencyStatus={maxConcurrency:InstitutionConcurrency;queued:number;running:number};
/** 后端按 clientId + userHash 聚合的任务统计，供 React 绘制分组表格。 */
export type InstitutionUserGroup={
  clientId:string;userHash:string;externalUserId:string|null;
  total:number;queued:number;running:number;succeeded:number;failed:number;unknown:number;lastSeen:string;
};
export type InstitutionUserGroupPage={total:number;offset:number;limit:number;items:InstitutionUserGroup[]};
/** 接口使用观测数据：不是平台认证、授权白名单或可信身份信息。 */
export type InstitutionAccessOverview={
  authMode:'NONE';platformIdentityVerified:false;platformAllowlistEnabled:false;
  currentPlatformLabel:string;retentionDays:number;
  users:Array<{clientId:string;externalUserId:string|null;userHash:string;
    taskCount:number;successCount:number;firstSeen:string;lastSeen:string}>;
};
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
  /**
   * 统一 HTTP 客户端（Java 类比：Feign/RestTemplate 的公共拦截器）。
   * <T> 是「预期响应的类型」；Promise<T> 表示未来异步返回 T。
   * fetch 是浏览器原生请求 API，credentials:'same-origin' 负责带上工作台 Cookie。
   * 注意：GET 与 PATCH 使用不同的权限策略；PATCH 必须携带 CSRF。
   */
  private async request<T>(path: string, init: RequestInit = {}): Promise<T> {
    if (!path.startsWith('/') || path.startsWith('//') || path.includes('..'))
      throw new Error('UNSAFE_API_PATH');
    if(this.base==='/api/v1/embed'&&!this.embedToken)throw new ApiError(401,'EMBED_SESSION_REQUIRED','本次排查会话已结束。');
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
    if(this.base==='/api/v1/embed'&&!this.embedToken)throw new ApiError(401,'EMBED_SESSION_REQUIRED','本次排查会话已结束。');
    const url=this.base+'/chat/sessions/'+encodeURIComponent(id)+'/activity';
    return this.embedToken?new BearerEventSource(url,{Authorization:'Bearer '+this.embedToken}):new EventSource(url) as unknown as ChatEventSource;
  }
  async openAnonymousEmbedded(){
    const response=await fetch('/api/v1/embed/anonymous',{method:'POST',credentials:'omit',cache:'no-store',redirect:'error',headers:{'Content-Type':'application/json'},body:'{}'});
    const body=await response.json();if(!response.ok||!body.data?.token||!body.data?.csrfToken)throw new ApiError(response.status,body.error?.code??'EMBED_SESSION_REQUIRED',body.error?.code==='EMBED_CAPACITY_FULL'?'当前排查名额已用完，请稍后重新打开页面。':'无法建立独立排查会话');
    this.base='/api/v1/embed';this.embedToken=body.data.token;this.csrf=body.data.csrfToken;this.businessVerificationEnabled=false;resetUserStorage();return body.data.expiresAt as number;
  }
  embeddedSession(){return this.request<{sessionId:string|null;state:string;expiresAt:number}>('/session');}
  institutionEvidence(input:{day:string;from:number;to:number;msgSn:string}){return this.request<{evidence:string}>('/institution-query',{method:'POST',body:JSON.stringify(input)});}
  async openEmbedded(ticket:string,parentOrigin:string){
    const response=await fetch('/api/v1/embed/exchange',{method:'POST',credentials:'omit',cache:'no-store',redirect:'error',headers:{'Content-Type':'application/json'},body:JSON.stringify({ticket,parentOrigin})});
    const body=await response.json();if(!response.ok||!body.data?.token||!body.data?.csrfToken)throw new ApiError(response.status,'EMBED_SESSION_REQUIRED','无法建立排查会话');
    this.base='/api/v1/embed';this.embedToken=body.data.token;this.csrf=body.data.csrfToken;this.businessVerificationEnabled=false;resetUserStorage();
    return body.data.expiresAt as number;
  }
  async closeEmbedded(){try{await this.request<void>('/session',{method:'DELETE'});}finally{this.clearEmbedded();}}
  clearEmbedded(){this.embedToken='';this.csrf='';resetUserStorage();}
  async pair(pairingCode: string): Promise<Schema<'Bootstrap'>> {
    const response = await fetch('/api/v1/auth/session', {
      method: 'POST', credentials: 'same-origin', redirect: 'error', cache: 'no-store',
      headers: { 'Content-Type':'application/json', Accept:'application/json' },
      body: JSON.stringify({ pairingCode }),
    });
    const body = await response.json() as { data?: Schema<'Bootstrap'>; error?: { message?: string } };
    if (!response.ok || !body.data) throw new ApiError(response.status, 'PAIRING_FAILED',
      body.error?.message ?? '配对失败，请检查一次性配对码');
    configureUserStorage(body.data.owner.id,(body.data as Schema<'Bootstrap'>&{identityMode?:string}).identityMode!=='local');
    this.businessVerificationEnabled=(body.data as Schema<'Bootstrap'>&{businessVerification?:boolean}).businessVerification===true;
    this.setCsrf(body.data.csrfToken);
    return body.data;
  }
  businessStatus(id:string){return this.request<{state:string;challengeId?:string;expiresAt?:number;maskedPhone?:string;verified?:boolean}>('/chat/sessions/'+encodeURIComponent(id)+'/business-verification');}
  submitBusinessCode(id:string,challenge:string,code:string){return this.request<{state:string;verified?:boolean}>('/chat/sessions/'+encodeURIComponent(id)+'/business-verification/'+encodeURIComponent(challenge)+'/code',{method:'POST',body:JSON.stringify({code})});}
  cancelBusinessCode(id:string,challenge:string){return this.request<unknown>('/chat/sessions/'+encodeURIComponent(id)+'/business-verification/'+encodeURIComponent(challenge),{method:'DELETE'});}
  chatSettings(){return this.request<ChatSettings>('/chat/settings');}
  sceneConfig(){return this.request<SceneConfigResponse>('/scene-config');}
  sceneRuntime(){return this.request<SceneConfigResponse>('/scene-runtime');}
  accessConfig(){return this.request<SceneAccessConfig & {revision?:string}>('/scene-access');}
  accessSettings(){return this.request<{config:SceneAccessConfig;revision:string}>('/scene-access');}
  saveAccessSettings(config:SceneAccessConfig,revision:string){
    return this.request<{config:SceneAccessConfig;revision:string}>('/scene-access',{method:'PUT',body:JSON.stringify({config,revision})});
  }
  pageStats(days:7|30=7){return this.request<{days:number;rows:{page:string;name:string;views:number;visitors:number;lastAt:number|null}[];daily:{page:string;day:string;views:number;visitors:number}[]}>(`/page-visits?days=${days}`);}
  /**
   * 接口任务列表：对应 C 的 GET /api/v1/institution-tasks。
   * userHash?:string 表示可选参数，URLSearchParams 负责正确编码查询参数。
   */
  institutionTasks(status:InstitutionTaskStatus|'ALL'='ALL',q='',offset=0,userHash?:string){
    const params=new URLSearchParams({status,q,offset:String(offset)});
    if(userHash)params.set('userHash',userHash);
    return this.request<InstitutionTaskPage>('/institution-tasks?'+params.toString());
  }
  /** 按 B 自报业务用户分组（后端 SQL GROUP BY），查询结果不包含完整最终结论。 */
  institutionTaskGroups(q='',offset=0){
    const params=new URLSearchParams({q,offset:String(offset)});
    return this.request<InstitutionUserGroupPage>('/institution-tasks/users?'+params.toString());
  }
  /** 查询当前最大并发档位和运行中/排队中的任务数。 */
  institutionConcurrency(){
    return this.request<InstitutionConcurrencyStatus>('/institution-concurrency');
  }
  /** 用户点击「保存配置」才发出 PATCH；request() 会自动带 CSRF 请求头。 */
  saveInstitutionConcurrency(maxConcurrency:InstitutionConcurrency){
    return this.request<InstitutionConcurrencyStatus>('/institution-concurrency',{
      method:'PATCH',body:JSON.stringify({maxConcurrency}),
    });
  }
  /** 管理员选中一条任务时才查询详细结果，避免列表一次性暴露所有结论。 */
  institutionTask(id:string){
    return this.request<InstitutionAdminTaskDetail>('/institution-tasks/'+encodeURIComponent(id));
  }
  /** 已观察到的接口调用情况；不能据此推断平台真实身份或授权。 */
  institutionAccessOverview(){
    return this.request<InstitutionAccessOverview>('/institution-access');
  }
  private pageVisitorId?:string;
  recordPageVisit(page:string){
    // Keep tracking identity in memory. Anonymous iframes must not write to
    // browser storage; an HTTP LAN origin may also lack crypto.randomUUID.
    if(!this.pageVisitorId){
      const bytes=new Uint8Array(16);
      crypto.getRandomValues(bytes);
      bytes[6]=(bytes[6]&15)|64;
      bytes[8]=(bytes[8]&63)|128;
      const hex=Array.from(bytes,b=>b.toString(16).padStart(2,'0')).join('');
      this.pageVisitorId=[hex.slice(0,8),hex.slice(8,12),hex.slice(12,16),hex.slice(16,20),hex.slice(20)].join('-');
    }
    const visitorId=this.pageVisitorId;
    void fetch('/api/v1/page-visits',{method:'POST',credentials:'omit',headers:{'Content-Type':'application/json'},body:JSON.stringify({page,visitorId}),keepalive:true}).catch(()=>{});
  }
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
    this.connecting??=this.request<Schema<'Bootstrap'>>('/bootstrap').then(boot=>{this.businessVerificationEnabled=(boot as Schema<'Bootstrap'>&{businessVerification?:boolean}).businessVerification===true;configureUserStorage(boot.owner.id,(boot as Schema<'Bootstrap'>&{identityMode?:string}).identityMode!=='local');return boot;}).finally(()=>{this.connecting=undefined;});
    return this.connecting;
  }
  async login(name:string,phone:string,lanCode?:string) {
    const session=await this.request<Schema<'Bootstrap'>>('/auth/local-session',{
      method:'POST',body:JSON.stringify({name,phone,...(lanCode?{lanCode}:{})}),
    });
    this.businessVerificationEnabled=(session as Schema<'Bootstrap'>&{businessVerification?:boolean}).businessVerification===true;
    configureUserStorage(session.owner.id,(session as Schema<'Bootstrap'>&{identityMode?:string}).identityMode!=='local');
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
