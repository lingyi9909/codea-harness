import {AsyncLocalStorage} from 'node:async_hooks';
import {isAbsolute,relative,sep} from 'node:path';
import {AuthProblem} from './auth.ts';
import type {chatConnection} from './modules/chat/connection.ts';
type Client=Awaited<ReturnType<ReturnType<typeof chatConnection>>>;
export type ExecutionRecord={ownerId:string;serviceIdentity:string;url:string;authorization:string;isolation:'os-user'|'container';directories:Record<string,string>};
const overlaps=(a:string,b:string)=>{const r=relative(a,b);return r===''||(!r.startsWith('..'+sep)&&r!=='..'&&!isAbsolute(r));};
export class ExecutionRegistry {
  private readonly records=new Map<string,ExecutionRecord>();
  private readonly context=new AsyncLocalStorage<ExecutionRecord>();
  private readonly connect:(url:string,authorization:string)=>Promise<Client>;
  private readonly clients=new Map<string,Promise<Client>>();
  constructor(entries:ExecutionRecord[],connect:(url:string,authorization:string)=>Promise<Client>){
    this.connect=connect;
    const urls=new Set<string>(),services=new Set<string>(),roots:string[]=[],credentials=new Set<string>();
    for(const entry of entries){
      const url=new URL(entry.url);
      if(!/^[0-9a-f-]{36}$/.test(entry.ownerId)||!entry.serviceIdentity||entry.serviceIdentity.length>128||this.records.has(entry.ownerId)||
        !['os-user','container'].includes(entry.isolation)||!entry.authorization||credentials.has(entry.authorization)||url.username||url.password||url.search||url.hash||
        !(url.protocol==='https:'||(url.protocol==='http:'&&['127.0.0.1','localhost','[::1]'].includes(url.hostname)))||urls.has(url.href)||services.has(entry.serviceIdentity))throw new Error('INVALID_EXECUTION_REGISTRY');
      const destinations=Object.entries(entry.directories);
      if(!destinations.length)throw new Error('EXECUTION_DIRECTORIES_REQUIRED');
      for(const [source,target]of destinations){
        if(!isAbsolute(source)||!isAbsolute(target)||overlaps(source,target)||overlaps(target,source)||roots.some(root=>overlaps(root,target)||overlaps(target,root)))throw new Error('EXECUTION_DIRECTORY_OVERLAP');
        roots.push(target);
      }
      credentials.add(entry.authorization);urls.add(url.href);services.add(entry.serviceIdentity);this.records.set(entry.ownerId,structuredClone(entry));
    }
  }
  async run<T>(owner:string,fn:()=>Promise<T>){
    const record=this.records.get(owner);if(!record)throw new AuthProblem(503,'EXECUTION_NOT_PROVISIONED');
    return this.context.run(record,fn);
  }
  private current(){const record=this.context.getStore();if(!record)throw new AuthProblem(403,'EXECUTION_CONTEXT_REQUIRED');return record;}
  serviceIdentity(){return this.current().serviceIdentity;}
  directory(input:string){
    const directories=this.current().directories;
    if(Object.hasOwn(directories,input))return directories[input];
    if(Object.values(directories).includes(input))return input;
    throw new AuthProblem(403,'EXECUTION_DIRECTORY_FORBIDDEN');
  }
  async client(){
    const record=this.current();let pending=this.clients.get(record.ownerId);
    if(!pending){pending=this.connect(record.url,record.authorization).catch(error=>{this.clients.delete(record.ownerId);throw error;});this.clients.set(record.ownerId,pending);}
    return pending;
  }
}
