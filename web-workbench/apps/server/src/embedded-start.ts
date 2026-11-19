import {realpath,mkdir,readFile} from 'node:fs/promises';
import {dirname,isAbsolute,relative,join,sep,resolve} from 'node:path';
import {homedir} from 'node:os';
import {createHash} from 'node:crypto';
import {EmbeddedAuth,type EmbedConfig} from './embed-auth.ts';
import {ExecutionRegistry,loadExecutionRegistry} from './execution-registry.ts';
import {chatConnection} from './modules/chat/connection.ts';
import {SqliteEmbedStore} from '../../../packages/storage/src/sqlite-embed.ts';

export function defaultEmbeddedDatabase(){return join(process.env.LOCALAPPDATA??homedir(),'.smart-workbench','state','embedded.sqlite');}
export async function createEmbeddedRuntime(workspace?:string){
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
  const registryFile=process.env.HARNESS_EMBED_EXECUTION_REGISTRY;
  const configFile=process.env.HARNESS_EMBED_ISOLATED_CONFIG;
  if(Boolean(registryFile)!==Boolean(configFile))
    throw Error('EMBED_ISOLATED_REQUIRES_BOTH_REGISTRY_AND_CONFIG');
  const connection=chatConnection(true);
  let execution:ExecutionRegistry;
  let config:EmbedConfig;
  if(registryFile&&configFile){
    // Each anonymous visitor gets a separately provisioned OpenCode service,
    // OS/container identity and private execution directory. This is the only
    // mode in which ordinary Skill/MCP tools can retain native permissions.
    config=JSON.parse(await readFile(configFile,'utf8')) as EmbedConfig;
    if(config.publicOrigin!==publicOrigin||config.clients?.length||
      !config.anonymous?.executionOwners?.length||config.anonymous.singleService)
      throw Error('EMBED_ISOLATED_CONFIG_INVALID');
    execution=await loadExecutionRegistry(registryFile,root,
      (url,authorization)=>chatConnection(false,{url,authorization})());
    for(const owner of config.anonymous.executionOwners){
      if(!execution.has(owner))throw Error('EMBED_ISOLATED_OWNER_MISSING');
      await execution.run(owner,async()=>{
        if(execution.sharedWorkspace()||execution.singleService())
          throw Error('EMBED_ISOLATED_SHARED_WORKSPACE_FORBIDDEN');
        // Also verifies that the configured source directory is explicitly mapped.
        execution.directory(root);
      });
    }
  }else{
    config={publicOrigin,clients:[],anonymous:{parentOrigins:parents,executionOwners:[],singleService:{identity,maxPages:Number(process.env.HARNESS_EMBED_MAX_PAGES??100)}}};
    execution=new ExecutionRegistry([],async()=>connection(),{workspace:root,singleServiceIdentity:identity});
  }
  const store=new SqliteEmbedStore(path,registryFile?undefined:identity);
  try{return {auth:new EmbeddedAuth(config,store),execution,connection,store,workspace:root,publicOrigin,port};}
  catch(error){await store.close();throw error;}
}
/** Compatibility API: both launchers now start the same complete service. */
export async function startEmbeddedWorkbench(workspace?:string){
  const {startWorkbench}=await import('./start.ts');
  return startWorkbench(undefined,true,workspace);
}
