import {spawn} from 'node:child_process';
import {existsSync} from 'node:fs';
import {mkdtemp,rm} from 'node:fs/promises';
import {createServer} from 'node:net';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {fileURLToPath} from 'node:url';
import {once} from 'node:events';
import {setTimeout as sleep} from 'node:timers/promises';

const root=fileURLToPath(new URL('../',import.meta.url));
if(!existsSync(join(root,'apps/web/dist/index.html')))
  throw new Error('Build the UI first: npm run web:build');

const workspace=await mkdtemp(join(tmpdir(),'codea-workbench-smoke-'));
const listener=createServer();
let child;
try{
  listener.listen(0,'127.0.0.1');
  await once(listener,'listening');
  const address=listener.address();
  if(!address||typeof address==='string')throw new Error('Expected TCP listener');
  const port=address.port;
  listener.close();
  await once(listener,'close');

  let output='';
  child=spawn(process.execPath,['--experimental-strip-types','apps/server/src/start.ts','--local-only','--workspace',workspace],{
    cwd:root,env:{...process.env,HARNESS_HTTP_PORT:String(port)},stdio:['ignore','pipe','pipe'],
  });
  for(const stream of [child.stdout,child.stderr])
    stream?.on('data',chunk=>{output+=String(chunk).slice(0,2000);});
  const url='http://127.0.0.1:'+port;
  let lastError='';
  let success=false;
  for(let attempt=0;attempt<70;attempt++){
    if(child.exitCode!==null)break;
    try{
      const response=await fetch(url+'/',{signal:AbortSignal.timeout(1000)});
      const html=await response.text();
      if(!response.ok)throw new Error('GET / -> HTTP '+response.status);
      if(!html.includes('<title>智能工作台</title>'))throw new Error('Missing workbench HTML title');
      const script=html.match(/src="(\/assets\/[^"]+\.js)"/);
      if(!script)throw new Error('Missing built JavaScript entry');
      const asset=await fetch(url+script[1],{signal:AbortSignal.timeout(1000)});
      if(!asset.ok)throw new Error('GET '+script[1]+' -> HTTP '+asset.status);
      success=true;break;
    }catch(error){lastError=error instanceof Error?error.message:String(error);}
    await sleep(250);
  }
  if(!success)throw new Error('Local workbench homepage smoke failed: '+lastError+'; server output: '+output.slice(-3000));
  process.stdout.write('Local-only homepage and JavaScript asset reachable on Node '+process.version+' (no MySQL/OpenCode).\n');
}finally{
  if(child&&!child.killed)child.kill();
  if(listener.listening)listener.close();
  await rm(workspace,{recursive:true,force:true});
}
