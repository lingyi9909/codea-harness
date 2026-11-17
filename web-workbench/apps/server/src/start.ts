import {readFile,realpath} from 'node:fs/promises';
import {ExecutionRegistry,type ExecutionRecord} from './execution-registry.ts';
import {chatConnection} from './modules/chat/connection.ts';
import { MultiUserAuth } from './multi-user-auth.ts';
import { MysqlIdentityStore } from '../../../packages/storage/src/identity.ts';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { existsSync } from 'node:fs';
import { parseArgs } from 'node:util';
import type { SchedulerConfig } from './workers/scheduler.ts';
import { randomBytes } from 'node:crypto';
import { createPool } from '../../../packages/storage/src/db.ts';
import { runtimeConfig } from './config.ts';
import { LocalAuth } from './auth.ts';
import { createApp } from './app.ts';

export async function startWorkbench(native?:Omit<SchedulerConfig,'pool'|'signal'>,localOnly=false,workspace?:string,multiUser=false){
  if(localOnly&&multiUser)throw new Error('INCOMPATIBLE_RUN_MODES');
  if(!workspace?.trim())throw new Error('STARTUP_WORKSPACE_REQUIRED: specify --workspace with an existing absolute directory.');
  if(localOnly&&!existsSync(fileURLToPath(new URL('../../web/dist/index.html',import.meta.url))))
    throw new Error('FRONTEND_BUILD_MISSING: run npm run web:build from web-workbench before starting the local app.');
  const config=localOnly?{port:Number(process.env.HARNESS_HTTP_PORT??4176),mysql:null}:runtimeConfig();
  if(!Number.isInteger(config.port)||config.port<1024||config.port>65535)throw new Error('INVALID_LOOPBACK_PORT');
  const pool=config.mysql?await createPool(config.mysql):undefined;
  const pairingCode=randomBytes(32).toString('base64url');
  const printCode=(code:string)=>process.stdout.write('智能工作台本地配对码（5 分钟内有效，仅可使用一次）：'+code+'\n');
  const auth=multiUser?new MultiUserAuth(new MysqlIdentityStore(pool!),process.env.HARNESS_PUBLIC_ORIGIN??''):new LocalAuth(pairingCode,config.port,localOnly?undefined:process.env.HARNESS_DEV_ORIGIN,localOnly?undefined:printCode);
  let execution:ExecutionRegistry|undefined;
  if(multiUser&&process.env.HARNESS_EXECUTION_REGISTRY){
    const entries:ExecutionRecord[]=JSON.parse(await readFile(process.env.HARNESS_EXECUTION_REGISTRY,'utf8'));
    for(const entry of entries){const directories:Record<string,string>={};for(const [source,target]of Object.entries(entry.directories))directories[await realpath(source)]=await realpath(target);entry.directories=directories;}
    execution=new ExecutionRegistry(entries,(url,authorization)=>chatConnection(false,{url,authorization})());
  }
  const app=await createApp({pool,auth,native,execution,mockBusiness:process.env.HARNESS_ENABLE_MOCK_BUSINESS==='1',localOnly,workspace,autoOpenCode:localOnly});
  const shutdown=async()=>{await app.close();await pool?.end();};
  process.once('SIGTERM',()=>{void shutdown()});
  process.once('SIGINT',()=>{void shutdown()});
  await app.listen({host:'127.0.0.1',port:config.port});
  if(!localOnly&&!multiUser)printCode(pairingCode);
  process.stdout.write('Local API: http://127.0.0.1:'+config.port+'\n');
}
async function main(){
  const {values}=parseArgs({options:{'local-only':{type:'boolean'},'multi-user':{type:'boolean'},workspace:{type:'string'}}});
  await startWorkbench(undefined,values['local-only'],values.workspace,values['multi-user']);
}
if(process.argv[1]&&pathToFileURL(process.argv[1]).href===import.meta.url)
  main().catch(e=>{console.error('Server startup blocked:',e instanceof Error?e.message:'UNKNOWN');process.exitCode=2});
