import { randomBytes } from 'node:crypto';
import { createPool } from '../../../packages/storage/src/db.ts';
import { runtimeConfig } from './config.ts';
import { LocalAuth } from './auth.ts';
import { createApp } from './app.ts';

async function start(){
  const config=runtimeConfig();
  const pool=await createPool(config.mysql);
  const pairingCode=randomBytes(32).toString('base64url');
  const auth=new LocalAuth(pairingCode,config.port,process.env.HARNESS_DEV_ORIGIN);
  const app=await createApp({pool,auth});
  const shutdown=async()=>{await app.close();await pool.end();};
  process.once('SIGTERM',()=>{void shutdown()});
  process.once('SIGINT',()=>{void shutdown()});
  await app.listen({host:'127.0.0.1',port:config.port});
  // Pairing code shown ONCE only on actual console, never logged to files or diagnostics.
  process.stdout.write('Code Harness local pairing code (valid 5 min, one use): '+pairingCode+'\n');
  process.stdout.write('Local API: http://127.0.0.1:'+config.port+'\n');
}
start().catch(e=>{console.error('Server startup blocked:',e instanceof Error?e.message:'UNKNOWN');process.exitCode=2});
