import { createApp } from './app.ts';
import { apiPort, ensureDataDir } from './config.ts';
import { pairingCodeForConsole } from './auth.ts';

async function main(){
  ensureDataDir();
  const app=createApp();
  await app.listen({port:apiPort,host:'127.0.0.1'});
  console.log('Code Harness Workbench: http://127.0.0.1:'+apiPort);
  console.log('一次性本机配对码（5分钟有效；请勿转发或记录在日志中）：');
  console.log(pairingCodeForConsole());
  const shutdown=async()=>{await app.close();process.exitCode=0;};
  process.once('SIGINT',()=>{void shutdown();});
  process.once('SIGTERM',()=>{void shutdown();});
}
main().catch(e=>{
  console.error('Workbench startup failed: '+(e instanceof Error?e.message:'UNKNOWN_ERROR'));
  process.exitCode=1;
});
