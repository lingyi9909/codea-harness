import {httpInstitutionAdapter} from './institution-query.ts';
import {resolveListenHost} from './lan-access.ts';
import {EmbeddedAuth,type EmbedConfig} from './embed-auth.ts';
import {MysqlEmbedStore} from '../../../packages/storage/src/embed.ts';
import {readFile} from 'node:fs/promises';
import {ExecutionRegistry,loadExecutionRegistry} from './execution-registry.ts';
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

export async function startWorkbench(native?:Omit<SchedulerConfig,'pool'|'signal'>,localOnly=false,workspace?:string,multiUser=false,embeddedOnly=false){
  if(embeddedOnly){
    if(localOnly||multiUser)throw new Error('INCOMPATIBLE_RUN_MODES');
    return startWorkbench(native,true,workspace);
  }
  if(localOnly&&multiUser)throw new Error('INCOMPATIBLE_RUN_MODES');
  if(!workspace?.trim())throw new Error('STARTUP_WORKSPACE_REQUIRED: specify --workspace with an existing absolute directory.');
  if(localOnly&&!existsSync(fileURLToPath(new URL('../../web/dist/index.html',import.meta.url))))
    throw new Error('FRONTEND_BUILD_MISSING: run npm run web:build from web-workbench before starting the local app.');
  const config=localOnly?{port:Number(process.env.HARNESS_HTTP_PORT??4176),mysql:null}:runtimeConfig();
  if(!Number.isInteger(config.port)||config.port<1024||config.port>65535)throw new Error('INVALID_LOOPBACK_PORT');
  const listenHost=resolveListenHost(config.port);
  const pool=config.mysql?await createPool(config.mysql):undefined;
  let app:Awaited<ReturnType<typeof createApp>>|undefined;
  let embedLocks:Awaited<ReturnType<typeof createPool>>|undefined;
  let localEmbed:Awaited<ReturnType<typeof import('./embedded-start.ts').createEmbeddedRuntime>>|undefined;
  try{
  const pairingCode=randomBytes(32).toString('base64url');
  const printCode=(code:string)=>process.stdout.write('智能工作台本地配对码（5 分钟内有效，仅可使用一次）：'+code+'\n');
  const auth=multiUser?new MultiUserAuth(new MysqlIdentityStore(pool!),process.env.HARNESS_PUBLIC_ORIGIN??''):new LocalAuth(pairingCode,config.port,localOnly?undefined:process.env.HARNESS_DEV_ORIGIN,localOnly?undefined:printCode);
  let execution:ExecutionRegistry|undefined;
  if(multiUser&&process.env.HARNESS_EXECUTION_REGISTRY){
    execution=await loadExecutionRegistry(process.env.HARNESS_EXECUTION_REGISTRY,workspace,(url,authorization)=>chatConnection(false,{url,authorization})());
  }
  let embedded:EmbeddedAuth|undefined;
  if(localOnly){
    const {createEmbeddedRuntime}=await import('./embedded-start.ts');
    localEmbed=await createEmbeddedRuntime(workspace);execution=localEmbed.execution;embedded=localEmbed.auth;
  }
  if(!localOnly&&process.env.HARNESS_EMBED_CONFIG){
    if(!multiUser||!pool||!execution)throw new Error('EMBED_REQUIRES_MULTI_USER_EXECUTION');
    const embedConfig:EmbedConfig=JSON.parse(await readFile(process.env.HARNESS_EMBED_CONFIG,'utf8'));
    if(embedConfig.publicOrigin!==process.env.HARNESS_PUBLIC_ORIGIN)throw new Error('EMBED_ORIGIN_MISMATCH');
    embedLocks=await createPool(config.mysql!);
    embedded=new EmbeddedAuth(embedConfig,new MysqlEmbedStore(pool,embedLocks));
  }
  app=await createApp({pool,auth,native,nativeConnection:localEmbed?.connection,execution,embedded,embeddedBindings:localEmbed?.store.bindings,embeddedStorageCheck:localEmbed?()=>localEmbed!.store.check():undefined,institutionAdapter:process.env.HARNESS_INSTITUTION_ADAPTER_URL?httpInstitutionAdapter(process.env.HARNESS_INSTITUTION_ADAPTER_URL,process.env.HARNESS_INSTITUTION_ADAPTER_AUTH??''):undefined,mockBusiness:process.env.HARNESS_ENABLE_MOCK_BUSINESS==='1',localOnly,workspace,autoOpenCode:localOnly});
  if(localEmbed)app.addHook('onClose',async()=>{await localEmbed!.store.close();});
  const shutdown=async()=>{await app?.close();await pool?.end();await embedLocks?.end();};
  await app.listen({host:listenHost,port:config.port});
  if(listenHost!=='127.0.0.1')process.stdout.write('LAN 测试访问已开启：请限制 Windows 防火墙来源 IP；机构页使用 HARNESS_PUBLIC_ORIGIN。\n');
  process.once('SIGTERM',()=>{void shutdown()});
  process.once('SIGINT',()=>{void shutdown()});
  if(!localOnly&&!multiUser)printCode(pairingCode);
  if(localEmbed)process.stdout.write('机构执行模式: '+(localEmbed.execution.institutionShellEnabled()?'共享服务（机构页 Shell 全放行；具有工作台进程操作系统权限，非工作目录沙箱；请保护访问入口）':localEmbed.execution.singleService()?'共享服务（Code Mode + 固定 TeamAI recall + DBHub 白名单；Shell 默认拒绝）':'隔离执行槽（按各自原生权限运行 Skill/MCP）')+'\n');
  process.stdout.write('工作台: http://127.0.0.1:'+config.port+'/\n场景配置: http://127.0.0.1:'+config.port+'/config\n机构报文排查: '+(localEmbed?.publicOrigin??('http://127.0.0.1:'+config.port))+'/embed/institution-message\n');
  }catch(error){await app?.close().catch(()=>{});await embedLocks?.end().catch(()=>{});await pool?.end().catch(()=>{});await localEmbed?.store.close().catch(()=>{});throw error;}
}
async function main(){
  const {values}=parseArgs({options:{'local-only':{type:'boolean'},'multi-user':{type:'boolean'},'embedded-only':{type:'boolean'},workspace:{type:'string'}}});
  await startWorkbench(undefined,values['local-only'],values.workspace,values['multi-user'],values['embedded-only']);
}
if(process.argv[1]&&pathToFileURL(process.argv[1]).href===import.meta.url)
  main().catch(e=>{console.error('Server startup blocked:',e instanceof Error?e.message:'UNKNOWN');process.exitCode=2});
