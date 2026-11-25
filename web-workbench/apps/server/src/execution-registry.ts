/**
 * OpenCode 执行上下文
 *
 * 职责：为不同任务传递执行身份、工具权限与任务级上下文。
 * Java 研发阅读提示：可类比 Java ThreadLocal，但 Promise 并发并非一请求一线程。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
import {readFile,realpath} from 'node:fs/promises';
import {AsyncLocalStorage} from 'node:async_hooks';
import {isAbsolute,relative,sep} from 'node:path';
import {AuthProblem} from './auth.ts';
import type {chatConnection} from './modules/chat/connection.ts';
// Anonymous sessions use a restrictive policy by default. A separate, explicit
// operator opt-in permits unrestricted Shell in the institution scene only.
// WARNING: unrestricted Shell inherits the service process OS privileges and
// is NOT confined by the session's working directory. OpenCode V2 rules use
// last-matching-rule-wins.
// The DBHub *service* must separately enforce readonly=true, source authorization,
// timeouts and row limits: these rules restrict which tools may be invoked, not SQL.
// OpenCode V2: execute enables Code Mode; nested tools still check their
// individual permissions. The optional Shell override is deliberately
// separate from the default policy and NEVER changes MCP grants.
// The default four TeamAI recall commands come from the approved Agent.
// Opt-in Shell override additionally permits arbitrary commands and arguments.
// DBHub action names must match the configured MCP source exactly.
export const singleServicePermissions=[
  {action:'*',resource:'*',effect:'deny' as const},
  {action:'skill',resource:'*',effect:'allow' as const},
  {action:'execute',resource:'*',effect:'allow' as const},
  {action:'dbhub_search_objects',resource:'*',effect:'allow' as const},
  {action:'dbhub_execute_sql',resource:'*',effect:'allow' as const},
  {action:'shell',resource:'teamai recall "帮我查询数据库"',effect:'allow' as const},
  {action:'shell',resource:'teamai recall "数据库数据字典和表关联"',effect:'allow' as const},
  {action:'shell',resource:'teamai recall "数据库业务指标定义和统计口径"',effect:'allow' as const},
  {action:'shell',resource:'teamai recall "数据库业务流程和历史异常案例"',effect:'allow' as const},
  // The base policy remains the safe default in the absence of explicit opt-in.
];
export const unrestrictedInstitutionShellPermissions=[
  ...singleServicePermissions,
  // Allow institution Skills to execute workspace scripts and read/write files
  // through native tools as well as Shell. Arbitrary Shell commands can touch
  // paths outside the workspace: these rules are NOT an OS-level sandbox.
  ...(['shell','read','write','edit','glob','grep','list','external_directory'] as const)
    .map(action=>({action,resource:'*',effect:'allow' as const})),
];
export const sharedWorkspacePermissions=[{action:'edit',resource:'*',effect:'deny' as const}];
type Client=Awaited<ReturnType<ReturnType<typeof chatConnection>>>;
export type ExecutionRecord={ownerId:string;serviceIdentity:string;url:string;authorization:string;isolation:'os-user'|'container';workspaceAccess?:'shared-readonly';directories:Record<string,string>};
const overlaps=(a:string,b:string)=>{const r=relative(a,b);return r===''||(!r.startsWith('..'+sep)&&r!=='..'&&!isAbsolute(r));};
/** 阅读提示：维持执行者与原生连接上下文；多用户隔离在这里向异步任务传播，不可仅凭 URL 认定用户身份。 */
export class ExecutionRegistry {
  private readonly records=new Map<string,ExecutionRecord>();
  private readonly context=new AsyncLocalStorage<Omit<ExecutionRecord,'isolation'>&{isolation:ExecutionRecord['isolation']|'session'}>();
  private readonly single?:{workspace:string;serviceIdentity:string};
  private readonly allowInstitutionShell:boolean;
  private readonly connect:(url:string,authorization:string)=>Promise<Client>;
  private readonly clients=new Map<string,Promise<Client>>();
  private readonly trustedSceneDirectories=new Set<string>();
  constructor(entries:ExecutionRecord[],connect:(url:string,authorization:string)=>Promise<Client>,options?:{workspace:string;singleServiceIdentity?:string;allowInstitutionShell?:boolean}){
    this.connect=connect;
    this.allowInstitutionShell=Boolean(options?.singleServiceIdentity&&options.allowInstitutionShell);
    if(options?.singleServiceIdentity){
      if(entries.length||!isAbsolute(options.workspace)||!options.singleServiceIdentity.startsWith('single:'))throw new Error('SINGLE_SERVICE_CONFIG_INVALID');
      this.single={workspace:options.workspace,serviceIdentity:options.singleServiceIdentity};
    }
    const allSources=entries.flatMap(entry=>Object.keys(entry.directories));
    const urls=new Set<string>(),services=new Set<string>(),roots:{path:string;shared:boolean}[]=[],credentials=new Set<string>();
    for(const entry of entries){
      const url=new URL(entry.url);
      if(!/^[0-9a-f-]{36}$/.test(entry.ownerId)||!entry.serviceIdentity||entry.serviceIdentity.length>128||this.records.has(entry.ownerId)||
        !['os-user','container'].includes(entry.isolation)||!entry.authorization||credentials.has(entry.authorization)||url.username||url.password||url.search||url.hash||
        !(url.protocol==='https:'||(url.protocol==='http:'&&['127.0.0.1','localhost','[::1]'].includes(url.hostname)))||urls.has(url.href)||services.has(entry.serviceIdentity))throw new Error('INVALID_EXECUTION_REGISTRY');
      const shared=entry.workspaceAccess==='shared-readonly';
      if(entry.workspaceAccess!==undefined&&!shared)throw new Error('EXECUTION_WORKSPACE_ACCESS_INVALID');
      if(shared&&(!options?.workspace||!isAbsolute(options.workspace)))throw new Error('SHARED_WORKSPACE_REQUIRED');
      const destinations=Object.entries(entry.directories);
      if(!destinations.length)throw new Error('EXECUTION_DIRECTORIES_REQUIRED');
      if(shared&&(destinations.length!==1||destinations[0][0]!==options!.workspace||destinations[0][1]!==options!.workspace))throw new Error('SHARED_WORKSPACE_MUST_MATCH_STARTUP');
      for(const [source,target]of destinations){
        const conflicts=roots.some(root=>(overlaps(root.path,target)||overlaps(target,root.path))&&!(shared&&root.shared&&root.path===target));
        if(!isAbsolute(source)||!isAbsolute(target)||conflicts||!shared&&allSources.some(root=>overlaps(root,target)||overlaps(target,root)))throw new Error('EXECUTION_DIRECTORY_OVERLAP');
        roots.push({path:target,shared});
      }
      credentials.add(entry.authorization);urls.add(url.href);services.add(entry.serviceIdentity);this.records.set(entry.ownerId,structuredClone(entry));
    }
  }
  has(owner:string){return this.single?/^[0-9a-f-]{36}$/.test(owner):this.records.has(owner);}
  async run<T>(owner:string,fn:()=>Promise<T>){
    const record=this.single&&this.has(owner)?{ownerId:owner,serviceIdentity:this.single.serviceIdentity,url:'',authorization:'',isolation:'session' as const,workspaceAccess:'shared-readonly' as const,directories:{[this.single.workspace]:this.single.workspace}}:this.records.get(owner);if(!record)throw new AuthProblem(503,'EXECUTION_NOT_PROVISIONED');
    return this.context.run(record,fn);
  }
  private current(){const record=this.context.getStore();if(!record)throw new AuthProblem(403,'EXECUTION_CONTEXT_REQUIRED');return record;}
  singleService(){return Boolean(this.single);}
  /** Only server-validated, administrator-owned scene bindings may extend the
   * anonymous Session Location; never accept a browser path as authorization. */
  trustSceneDirectory(path:string){
    if(!this.single||!isAbsolute(path))throw new AuthProblem(403,'EMBED_DIRECTORY_FORBIDDEN');
    this.trustedSceneDirectories.add(path);
  }
  sharedWorkspace(){return this.current().workspaceAccess==='shared-readonly';}
  institutionShellEnabled(){return Boolean(this.single&&this.allowInstitutionShell);}
  permissions(){return this.single?structuredClone(this.allowInstitutionShell?unrestrictedInstitutionShellPermissions:singleServicePermissions):this.sharedWorkspace()?structuredClone(sharedWorkspacePermissions):undefined;}
  assertPermissions(session:{permissions?:{action:string;resource:string;effect:string}[]},override?:{action:string;resource:string;effect:string}[]){
    if(!this.sharedWorkspace())return;
    // Fail closed if the installed native API drops or mutates the deny rules.
    const rules=session.permissions,expected=override??(this.single?(this.allowInstitutionShell?unrestrictedInstitutionShellPermissions:singleServicePermissions):sharedWorkspacePermissions);
    if(!Array.isArray(rules)||rules.length!==expected.length||expected.some((rule,i)=>rules[i].action!==rule.action||rules[i].resource!==rule.resource||rules[i].effect!==rule.effect))throw new AuthProblem(409,'SHARED_WORKSPACE_PERMISSION_MISMATCH');
  }
  serviceIdentity(){return this.current().serviceIdentity;}
  directory(input:string){
    const directories=this.current().directories;
    if(this.single&&this.trustedSceneDirectories.has(input))return input;
    if(Object.hasOwn(directories,input))return directories[input];
    if(Object.values(directories).includes(input))return input;
    throw new AuthProblem(403,'EXECUTION_DIRECTORY_FORBIDDEN');
  }
  async client(){
    if(this.single){this.current();return this.connect('','');}
    const record=this.current();let pending=this.clients.get(record.ownerId);
    if(!pending){pending=this.connect(record.url,record.authorization).catch(error=>{this.clients.delete(record.ownerId);throw error;});this.clients.set(record.ownerId,pending);}
    return pending;
  }
}

export async function loadExecutionRegistry(path:string,workspace:string,connect:(url:string,authorization:string)=>Promise<Client>){
  const entries:ExecutionRecord[]=JSON.parse(await readFile(path,'utf8'));
  const startupWorkspace=await realpath(workspace);
  for(const entry of entries){
    if(entry.workspaceAccess==='shared-readonly'&&!entry.directories)entry.directories={[startupWorkspace]:startupWorkspace};
    const directories:Record<string,string>={};
    for(const [source,target]of Object.entries(entry.directories))directories[await realpath(source)]=await realpath(target);
    entry.directories=directories;
  }
  return new ExecutionRegistry(entries,connect,{workspace:startupWorkspace});
}
