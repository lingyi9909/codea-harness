import {realpath,mkdir} from 'node:fs/promises';
import {dirname,isAbsolute,relative,join,sep,resolve} from 'node:path';
import {homedir} from 'node:os';
import {createHash} from 'node:crypto';
import {EmbeddedAuth,type EmbedConfig} from './embed-auth.ts';
import {ExecutionRegistry} from './execution-registry.ts';
import {chatConnection} from './modules/chat/connection.ts';
import {httpInstitutionAdapter} from './institution-query.ts';
import {SqliteEmbedStore} from '../../../packages/storage/src/sqlite-embed.ts';
import {createEmbeddedApp} from './embedded-app.ts';
export function defaultEmbeddedDatabase(){return join(process.env.LOCALAPPDATA??homedir(),'.smart-workbench','state','embedded.sqlite');}
export async function startEmbeddedWorkbench(workspace?:string){
  if(!workspace||!isAbsolute(workspace))throw Error('STARTUP_WORKSPACE_REQUIRED');
  const port=Number(process.env.HARNESS_HTTP_PORT??4176);if(!Number.isInteger(port)||port<1024||port>65535)throw Error('INVALID_LOOPBACK_PORT');
  const root=await realpath(workspace),path=process.env.HARNESS_SQLITE_PATH??defaultEmbeddedDatabase();
  if(!isAbsolute(path))throw Error('SQLITE_LOCAL_ABSOLUTE_PATH_REQUIRED');
  await mkdir(dirname(path),{recursive:true,mode:0o700});
  const parent=await realpath(dirname(path)),rel=relative(root,parent);
  if(rel===''||(!rel.startsWith('..'+sep)&&rel!=='..'&&!isAbsolute(rel)))throw Error('SQLITE_MUST_BE_OUTSIDE_SHARED_WORKSPACE');
  const locator=process.env.HARNESS_OPENCODE_URL??resolve(process.env.HARNESS_OPENCODE_SERVICE_FILE??join(process.env.XDG_STATE_HOME??join(homedir(),'.local','state'),'opencode','service.json'));
  const identity='single:'+createHash('sha256').update(JSON.stringify([locator,root])).digest('hex');
  const publicOrigin=process.env.HARNESS_PUBLIC_ORIGIN??'http://127.0.0.1:'+port;
  const parents=process.env.HARNESS_EMBED_PARENTS?.split(',').map(s=>s.trim()).filter(Boolean)??[publicOrigin];
  const config:EmbedConfig={publicOrigin,clients:[],anonymous:{parentOrigins:parents,executionOwners:[],singleService:{identity,maxPages:Number(process.env.HARNESS_EMBED_MAX_PAGES??100)}}};
  const connection=chatConnection(true);
  const execution=new ExecutionRegistry([],async()=>connection(),{workspace:root,singleServiceIdentity:identity});
  const store=new SqliteEmbedStore(path,identity);let app:Awaited<ReturnType<typeof createEmbeddedApp>>|undefined;
  try{
    app=await createEmbeddedApp({auth:new EmbeddedAuth(config,store),execution,workspace:root,bindings:store.bindings,ensureOwner:(id,subject)=>store.ensureOwner(id,subject),storageCheck:()=>store.check(),institutionAdapter:process.env.HARNESS_INSTITUTION_ADAPTER_URL?httpInstitutionAdapter(process.env.HARNESS_INSTITUTION_ADAPTER_URL,process.env.HARNESS_INSTITUTION_ADAPTER_AUTH??''):undefined});
    await app.listen({host:'127.0.0.1',port});
    let shutdown:Promise<void>|undefined;
    const close=()=>shutdown??=(async()=>{await app!.close();await store.close();})();
    process.once('SIGINT',()=>{void close();});process.once('SIGTERM',()=>{void close();});
    process.stdout.write('机构报文排查: '+publicOrigin+'/embed/institution-message\nStorage: SQLite (local file)\nOpenCode: one service, independent Sessions, analysis-only permissions\n');
  }catch(e){await app?.close().catch(()=>{});await store.close().catch(()=>{});throw e;}
}
