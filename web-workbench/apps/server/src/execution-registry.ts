import {readFile,realpath} from 'node:fs/promises';
import {AsyncLocalStorage} from 'node:async_hooks';
import {isAbsolute,relative,sep} from 'node:path';
import {AuthProblem} from './auth.ts';
import type {chatConnection} from './modules/chat/connection.ts';
// Anonymous sessions share one service but never inherit the logged-in Workbench's
// broad Shell/file/network authority. OpenCode V2 uses last-matching-rule-wins.
// The DBHub *service* must separately enforce readonly=true, source authorization,
// timeouts and row limits: these rules restrict which tools may be invoked, not SQL.
export const singleServicePermissions=[
  {action:'*',resource:'*',effect:'deny' as const},
  {action:'skill',resource:'*',effect:'allow' as const},
  {action:'dbhub_search_schema',resource:'*',effect:'allow' as const},
  {action:'dbhub_execute_sql',resource:'*',effect:'allow' as const},
];
export const sharedWorkspacePermissions=[{action:'edit',resource:'*',effect:'deny' as const}];
type Client=Awaited<ReturnType<ReturnType<typeof chatConnection>>>;
export type ExecutionRecord={ownerId:string;serviceIdentity:string;url:string;authorization:string;isolation:'os-user'|'container';workspaceAccess?:'shared-readonly';directories:Record<string,string>};
const overlaps=(a:string,b:string)=>{const r=relative(a,b);return r===''||(!r.startsWith('..'+sep)&&r!=='..'&&!isAbsolute(r));};
export class ExecutionRegistry {
  private readonly records=new Map<string,ExecutionRecord>();
  private readonly context=new AsyncLocalStorage<Omit<ExecutionRecord,'isolation'>&{isolation:ExecutionRecord['isolation']|'session'}>();
  private readonly single?:{workspace:string;serviceIdentity:string};
  private readonly connect:(url:string,authorization:string)=>Promise<Client>;
  private readonly clients=new Map<string,Promise<Client>>();
  constructor(entries:ExecutionRecord[],connect:(url:string,authorization:string)=>Promise<Client>,options?:{workspace:string;singleServiceIdentity?:string}){
    this.connect=connect;
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
  sharedWorkspace(){return this.current().workspaceAccess==='shared-readonly';}
  permissions(){return this.single?structuredClone(singleServicePermissions):this.sharedWorkspace()?structuredClone(sharedWorkspacePermissions):undefined;}
  assertPermissions(session:{permissions?:{action:string;resource:string;effect:string}[]}){
    if(!this.sharedWorkspace())return;
    // Fail closed if the installed native API drops or mutates the deny rules.
    const rules=session.permissions,expected=this.single?singleServicePermissions:sharedWorkspacePermissions;
    if(!Array.isArray(rules)||rules.length!==expected.length||expected.some((rule,i)=>rules[i].action!==rule.action||rules[i].resource!==rule.resource||rules[i].effect!==rule.effect))throw new AuthProblem(409,'SHARED_WORKSPACE_PERMISSION_MISMATCH');
  }
  serviceIdentity(){return this.current().serviceIdentity;}
  directory(input:string){
    const directories=this.current().directories;
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
