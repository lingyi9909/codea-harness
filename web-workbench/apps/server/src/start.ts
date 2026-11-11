import { pathToFileURL } from 'node:url';
import type { SchedulerConfig } from './workers/scheduler.ts';
import { randomBytes } from 'node:crypto';
import { createPool } from '../../../packages/storage/src/db.ts';
import { runtimeConfig } from './config.ts';
import { LocalAuth } from './auth.ts';
import { createApp } from './app.ts';

export async function startWorkbench(native?:Omit<SchedulerConfig,'pool'|'signal'>){
  const config=runtimeConfig();
  const pool=await createPool(config.mysql);
  const pairingCode=randomBytes(32).toString('base64url');
  const printCode=(code:string)=>process.stdout.write('Code Harness local pairing code (valid 5 min, one use): '+code+'\n');
  const auth=new LocalAuth(pairingCode,config.port,process.env.HARNESS_DEV_ORIGIN,printCode);
  const app=await createApp({pool,auth,native});
  const shutdown=async()=>{await app.close();await pool.end();};
  process.once('SIGTERM',()=>{void shutdown()});
  process.once('SIGINT',()=>{void shutdown()});
  await app.listen({host:'127.0.0.1',port:config.port});
  printCode(pairingCode);
  process.stdout.write('Local API: http://127.0.0.1:'+config.port+'\n');
}
if(process.argv[1]&&pathToFileURL(process.argv[1]).href===import.meta.url) startWorkbench().catch(e=>{console.error('Server startup blocked:',e instanceof Error?e.message:'UNKNOWN');process.exitCode=2});
