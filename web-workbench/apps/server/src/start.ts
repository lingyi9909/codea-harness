import { fileURLToPath, pathToFileURL } from 'node:url';
import { existsSync } from 'node:fs';
import { parseArgs } from 'node:util';
import type { SchedulerConfig } from './workers/scheduler.ts';
import { randomBytes } from 'node:crypto';
import { createPool } from '../../../packages/storage/src/db.ts';
import { runtimeConfig } from './config.ts';
import { LocalAuth } from './auth.ts';
import { createApp } from './app.ts';

export async function startWorkbench(native?:Omit<SchedulerConfig,'pool'|'signal'>,localOnly=false,workspace?:string){
  if(workspace!==undefined&&!localOnly)throw new Error('--workspace requires --local-only');
  if(localOnly&&!existsSync(fileURLToPath(new URL('../../web/dist/index.html',import.meta.url))))
    throw new Error('FRONTEND_BUILD_MISSING: run npm run web:build from web-workbench before starting the local app.');
  const config=localOnly?{port:Number(process.env.HARNESS_HTTP_PORT??4176),mysql:null}:runtimeConfig();
  if(!Number.isInteger(config.port)||config.port<1024||config.port>65535)throw new Error('INVALID_LOOPBACK_PORT');
  const pool=config.mysql?await createPool(config.mysql):undefined;
  const pairingCode=randomBytes(32).toString('base64url');
  const printCode=(code:string)=>process.stdout.write('Code Harness local pairing code (valid 5 min, one use): '+code+'\n');
  const auth=new LocalAuth(pairingCode,config.port,localOnly?undefined:process.env.HARNESS_DEV_ORIGIN,localOnly?undefined:printCode);
  const app=await createApp({pool,auth,native,localOnly,workspace,autoOpenCode:localOnly});
  const shutdown=async()=>{await app.close();await pool?.end();};
  process.once('SIGTERM',()=>{void shutdown()});
  process.once('SIGINT',()=>{void shutdown()});
  await app.listen({host:'127.0.0.1',port:config.port});
  if(!localOnly)printCode(pairingCode);
  process.stdout.write('Local API: http://127.0.0.1:'+config.port+'\n');
}
async function main(){
  const {values}=parseArgs({options:{'local-only':{type:'boolean'},workspace:{type:'string'}}});
  await startWorkbench(undefined,values['local-only'],values.workspace);
}
if(process.argv[1]&&pathToFileURL(process.argv[1]).href===import.meta.url)
  main().catch(e=>{console.error('Server startup blocked:',e instanceof Error?e.message:'UNKNOWN');process.exitCode=2});
