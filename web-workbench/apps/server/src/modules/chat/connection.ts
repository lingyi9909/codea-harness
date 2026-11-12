import {OpenCode} from '@opencode/client';
import {spawn} from 'node:child_process';
import {setTimeout as delay} from 'node:timers/promises';
import {homedir} from 'node:os';
import {existsSync} from 'node:fs';
import {delimiter,isAbsolute,join} from 'node:path';
import {Service,type Endpoint} from '@opencode/client/service';
import {AuthProblem} from '../../auth.ts';
import type {registerAccess} from './access.ts';

type Log=ReturnType<typeof registerAccess>;
const local=(host:string)=>['127.0.0.1','localhost','[::1]'].includes(host);
export function chatConnection(log:Log,mayStart:boolean){
  let cached:ReturnType<typeof connect>|undefined,expires=0;
  async function connect(){
    const start=Date.now();
    let endpoint=process.env.HARNESS_OPENCODE_URL;
    let authorization=process.env.HARNESS_OPENCODE_AUTHORIZATION;
    if(!endpoint){
      const file=process.env.HARNESS_OPENCODE_SERVICE_FILE??join(process.env.XDG_STATE_HOME??join(homedir(),'.local','state'),'opencode','service.json');
      const options={file};
      let found:Endpoint|undefined=await Service.discover(options);
      if(!found&&existsSync(file)){
        log('服务发现','已有本机注册记录','健康检查失败或服务协议不兼容',Date.now()-start);
      }else if(!found&&mayStart){
        log('服务发现','opencode serve --service','启动后台服务',Date.now()-start);
        const launch=process.platform==='win32'?windowsCommand():{command:'opencode',args:['serve','--service']};
        if(!launch){
          log('服务发现','查找 opencode','PATH 中未找到 OpenCode',Date.now()-start);
        }else{
          const child=spawn(launch.command,launch.args,{detached:true,windowsHide:true,stdio:'ignore',
            windowsVerbatimArguments:'shim' in launch,env:{...process.env,...('shim' in launch?{HARNESS_INSTALLED_CLI:launch.shim}:{})}});
          let failed=false;
          child.once('error',e=>{failed=true;log('服务发现','启动进程',(e as NodeJS.ErrnoException).code??'启动失败',Date.now()-start);});
          child.once('exit',code=>{if(code){failed=true;log('服务发现','后台进程退出',String(code),Date.now()-start);}});
          child.unref();
          const deadline=Date.now()+30000;
          while(!found&&!failed&&Date.now()<deadline){
            await delay(250);
            found=await Service.discover(options);
          }
        }
      }
      if(!found){log('服务发现','本机 OpenCode','未发现可用的 2.x 后台服务',Date.now()-start);
        throw new AuthProblem(503,'OPENCODE_DISCOVERY_FAILED');}
      endpoint=found.url;
      authorization=Service.headers(found)?.authorization??authorization;
    }
    const url=new URL(endpoint);
    if(url.username||url.password||url.hash||url.search||!(url.protocol==='https:'||(url.protocol==='http:'&&local(url.hostname))))
      throw new AuthProblem(400,'INVALID_OPENCODE_URL');
    if(url.protocol==='http:'&&local(url.hostname)&&url.port===(process.env.HARNESS_HTTP_PORT??'4176'))
      throw new AuthProblem(400,'OPENCODE_WORKBENCH_URL');
    log('服务发现',url.origin,process.env.HARNESS_OPENCODE_URL?'已使用预配置地址':'已自动连接',Date.now()-start);
    return OpenCode.make({baseUrl:endpoint,headers:authorization?{authorization}:{},fetch:async(input,init)=>{
      const begin=Date.now(),pathname=new URL(String(input)).pathname.slice(url.pathname.replace(/\/$/,'').length);
      const operation=(init?.method??'GET')+' '+pathname.replace(/(\/session\/)(?!active(?:\/|$))[^/]+/,'$1:id').replace(/(\/permission\/)[^/]+/,'$1:id');
      let response:Response;
      try{response=await fetch(input,{...init,redirect:'manual',signal:pathname==='/api/event'&&init?.signal?init.signal:AbortSignal.timeout(25000)});}
      catch(e){
        cached=undefined;
        const cause=e instanceof Error?e.cause:null;
        const code=cause&&typeof cause==='object'&&'code' in cause?String(cause.code):'';
        log('OpenCode',operation,/^[A-Z0-9_]{1,60}$/.test(code)?code:e instanceof Error&&e.name==='TimeoutError'?'超时':'连接失败',Date.now()-begin);
        throw e;
      }
      log('OpenCode',operation,String(response.status),Date.now()-begin);
      if(response.ok)return response;
      if(response.status===401||response.status===403)cached=undefined;
      await response.body?.cancel();
      throw new AuthProblem(502,'OPENCODE_HTTP_'+response.status);
    }});
  }
  return ()=>{
    if(!cached||Date.now()>expires){
      expires=Date.now()+120000;
      cached=connect().catch(e=>{cached=undefined;throw e;});
    }
    return cached;
  };
}

// Use the installed command from an absolute PATH entry. npm installations commonly expose a .cmd shim.
function windowsCommand(){
  if(process.platform!=='win32')return;
  for(const dir of (process.env.PATH??'').split(delimiter)){
    const base=dir.replace(/^"|"$/g,'');
    if(!isAbsolute(base))continue;
    const exe=join(base,'opencode.exe');
    if(existsSync(exe))return {command:exe,args:['serve','--service']};
    const shim=join(base,'opencode.cmd');
    if(existsSync(shim))return {command:join(process.env.SystemRoot??'C:\\Windows','System32','cmd.exe'),
      args:['/d','/v:off','/s','/c','""%HARNESS_INSTALLED_CLI%" serve --service"'],shim};
  }
}
