import {readFile,realpath,mkdir} from 'node:fs/promises';
import {dirname,isAbsolute,relative,join,sep} from 'node:path';
import {homedir} from 'node:os';
import {EmbeddedAuth,type EmbedConfig} from './embed-auth.ts';
import {loadExecutionRegistry} from './execution-registry.ts';
import {chatConnection} from './modules/chat/connection.ts';
import {httpInstitutionAdapter} from './institution-query.ts';
import {SqliteEmbedStore} from '../../../packages/storage/src/sqlite-embed.ts';
import {createEmbeddedApp} from './embedded-app.ts';
export function defaultEmbeddedDatabase(){return join(process.env.LOCALAPPDATA??homedir(),'.smart-workbench','state','embedded.sqlite');}
export async function startEmbeddedWorkbench(workspace?:string){
  if(!workspace||!isAbsolute(workspace))throw Error('STARTUP_WORKSPACE_REQUIRED');
  const root=await realpath(workspace),path=process.env.HARNESS_SQLITE_PATH??defaultEmbeddedDatabase();
  if(!isAbsolute(path))throw Error('SQLITE_LOCAL_ABSOLUTE_PATH_REQUIRED');
  await mkdir(dirname(path),{recursive:true,mode:0o700});
  const parent=await realpath(dirname(path)),rel=relative(root,parent);
  if(rel===''||(!rel.startsWith('..'+sep)&&rel!=='..'&&!isAbsolute(rel)))throw Error('SQLITE_MUST_BE_OUTSIDE_SHARED_WORKSPACE');
  if(!process.env.HARNESS_EMBED_CONFIG||!process.env.HARNESS_EXECUTION_REGISTRY)throw Error('EMBED_CONFIG_REQUIRED');
  const config:EmbedConfig=JSON.parse(await readFile(process.env.HARNESS_EMBED_CONFIG,'utf8'));
  if(process.env.HARNESS_PUBLIC_ORIGIN&&process.env.HARNESS_PUBLIC_ORIGIN!==config.publicOrigin)throw Error('EMBED_ORIGIN_MISMATCH');
  const execution=await loadExecutionRegistry(process.env.HARNESS_EXECUTION_REGISTRY,root,(url,authorization)=>chatConnection(false,{url,authorization})());
  const store=new SqliteEmbedStore(path);let app:Awaited<ReturnType<typeof createEmbeddedApp>>|undefined;
  try{
    app=await createEmbeddedApp({auth:new EmbeddedAuth(config,store),execution,workspace:root,bindings:store.bindings,ensureOwner:(id,subject)=>store.ensureOwner(id,subject),storageCheck:()=>store.check(),institutionAdapter:process.env.HARNESS_INSTITUTION_ADAPTER_URL?httpInstitutionAdapter(process.env.HARNESS_INSTITUTION_ADAPTER_URL,process.env.HARNESS_INSTITUTION_ADAPTER_AUTH??''):undefined});
    const port=Number(process.env.HARNESS_HTTP_PORT??4176);if(!Number.isInteger(port)||port<1024||port>65535)throw Error('INVALID_LOOPBACK_PORT');
    await app.listen({host:'127.0.0.1',port});
    let shutdown:Promise<void>|undefined;
    const close=()=>shutdown??=(async()=>{await app!.close();await store.close();})();
    process.once('SIGINT',()=>{void close();});process.once('SIGTERM',()=>{void close();});
    process.stdout.write('机构报文排查: http://127.0.0.1:'+port+'/embed/institution-message\nStorage: SQLite (local file)\n');
  }catch(e){await app?.close().catch(()=>{});await store.close().catch(()=>{});throw e;}
}
