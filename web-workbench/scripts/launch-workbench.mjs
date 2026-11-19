import {spawn} from 'node:child_process';
import {stat} from 'node:fs/promises';
import {networkInterfaces} from 'node:os';
import {basename,resolve} from 'node:path';
import {createInterface} from 'node:readline/promises';
import {fileURLToPath} from 'node:url';
import {isPrivateLanIPv4} from '../apps/server/src/lan-access.ts';

// Kept outside the TypeScript runtime: this menu runs in Windows offline ZIPs,
// where Node is available but no npm build/install step is required.
export function privateLanInterfaces(interfaces=networkInterfaces()){
  const found=new Map();
  for(const [name,items] of Object.entries(interfaces)){
    for(const item of items??[]){
      if(item.family!=='IPv4'||item.internal||!isPrivateLanIPv4(item.address))continue;
      if(!found.has(item.address))found.set(item.address,{address:item.address,adapter:name});
    }
  }
  const virtual=/virtual|vEthernet|vmware|virtualbox|docker|wsl|hyper-v|loopback|tunnel|tap|bridge/i;
  const priority=name=>virtual.test(name)?1:0;
  return [...found.values()].sort((a,b)=>priority(a.adapter)-priority(b.adapter)||a.adapter.localeCompare(b.adapter));
}

export function launchEnvironment(env,{scope='local',address,allowShell=false}={}){
  const result={...env};
  if(scope==='local'||scope==='lan'){
    const port=Number(env.HARNESS_HTTP_PORT??4176);
    if(!Number.isInteger(port)||port<1024||port>65535)throw Error('无效端口 HARNESS_HTTP_PORT');
    const origin=scope==='local'?'http://127.0.0.1:'+port:'http://'+address+':'+port;
    if(scope==='lan'&&!isPrivateLanIPv4(address??''))throw Error('局域网 IP 必须是实际的私有 IPv4 地址');
    result.HARNESS_ENABLE_LAN=scope==='lan'?'1':'0';
    result.HARNESS_HTTP_BIND=scope==='lan'?'0.0.0.0':'127.0.0.1';
    result.HARNESS_PUBLIC_ORIGIN=origin;
    result.HARNESS_EMBED_PARENTS=origin;
  }
  // A new session must be created after each permission mode change.
  result.HARNESS_INSTITUTION_ALLOW_SHELL=allowShell?'1':'0';
  return result;
}

export function menuChoice(value,size,defaultChoice=1){
  const text=String(value).trim();
  if(!text)return defaultChoice;
  if(!/^\d+$/.test(text))return null;
  const number=Number(text);
  return number>=1&&number<=size?number:null;
}

async function select(rl,heading,items,defaultChoice=1){
  process.stdout.write('\n'+heading+'\n');
  for(const [index,item] of items.entries())process.stdout.write('  ['+(index+1)+'] '+item+'\n');
  while(true){
    const value=await rl.question('请选择（默认 '+defaultChoice+'）：');
    const chosen=menuChoice(value,items.length,defaultChoice);
    if(chosen!==null)return chosen;
    process.stdout.write('请输入菜单中的数字。\n');
  }
}

export async function chooseEnvironment(env,{input=process.stdin,output=process.stdout,interfaces=networkInterfaces(),questions}={}){
  if(!input.isTTY&&!questions)return {...env}; // CI and scripted startup preserve caller-supplied values.
  const rl=questions?{question:async()=>questions.shift()??'',close(){}}:createInterface({input,output});
  try{
    const scopeChoice=await select(rl,'一、访问方式',[
      '只允许本机访问（默认）',
      '允许局域网同事访问机构报文页面',
      '沿用已有环境配置（高级用户）',
    ]);
    const scope=scopeChoice===1?'local':scopeChoice===2?'lan':'existing';
    let address;
    if(scope==='lan'){
      const interfacesFound=privateLanInterfaces(interfaces);
      if(!interfacesFound.length)throw Error('找不到可用的私有 IPv4 地址；请检查网卡连接或使用高级环境配置。');
      const ipChoice=await select(rl,'二、选择你电脑的局域网 IP',interfacesFound.map(i=>i.address+'  ('+i.adapter+')'));
      address=interfacesFound[ipChoice-1].address;
    }
    let allowShell=(await select(rl,(scope==='lan'?'三':'二')+'、机构报文排查执行权限',[
      '受限 Shell（默认，保留原来的安全白名单）',
      '开放 Shell 和常用文件读写工具（运行脚本、写数据）',
    ]))===2;
    if(scope==='lan'&&allowShell){
      const confirmation=await select(rl,'风险确认：局域网访问者可能通过机构页面执行本机命令。',[
        '取消高权限，使用受限 Shell（默认）',
        '已了解风险，仍然开放 Shell 和文件读写',
      ]);
      if(confirmation!==2)allowShell=false;
    }
    return launchEnvironment(env,{scope,address,allowShell});
  }finally{rl.close();}
}

async function main(){
  if(process.argv.length!==3)throw Error('用法：启动工作台.cmd "D:\\teamai"');
  const workspace=resolve(process.argv[2]);
  if(!(await stat(workspace).catch(()=>null))?.isDirectory())throw Error('工作目录不存在：'+workspace);
  process.stdout.write('\n========== 智能工作台 =========='+'\n工作目录：'+workspace+'\n');
  const env=await chooseEnvironment(process.env);
  const origin=env.HARNESS_PUBLIC_ORIGIN??'http://127.0.0.1:'+(env.HARNESS_HTTP_PORT??4176);
  process.stdout.write('\n启动配置：'+(env.HARNESS_HTTP_BIND==='0.0.0.0'?'局域网访问':'本机访问')+
    '；机构 Shell：'+(env.HARNESS_INSTITUTION_ALLOW_SHELL==='1'?'开放':'受限')+'\n');
  if(env.HARNESS_HTTP_BIND==='0.0.0.0'){
    process.stdout.write('同事访问：'+origin+'/embed/institution-message\n');
    process.stdout.write('提示：Windows 防火墙需仅允许授权同事的 IP 访问服务端口。\n');
  }
  const root=fileURLToPath(new URL('../',import.meta.url));
  const child=spawn(process.execPath,['--experimental-strip-types','apps/server/src/start.ts','--local-only','--workspace',workspace],{
    cwd:root,stdio:'inherit',env,windowsHide:false,
  });
  await new Promise((done,reject)=>{
    child.once('error',reject);
    child.once('exit',(code,signal)=>{process.exitCode=code??(signal?1:0);done();});
  });
}

if(process.argv[1]&&resolve(process.argv[1])===fileURLToPath(import.meta.url))
  main().catch(error=>{process.stderr.write('启动失败：'+(error instanceof Error?error.message:String(error))+'\n');process.exitCode=2;});
