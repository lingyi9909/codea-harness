/**
 * 工作目录用户资料
 *
 * 职责：读取/核对当前工作目录对应的本地登录用户资料。
 * Java 研发阅读提示：用户自填姓名手机号不构成强身份认证。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
import { closeSync, constants, fstatSync, fsyncSync, lstatSync, mkdirSync, openSync, readFileSync, realpathSync,
  renameSync, unlinkSync, writeFileSync } from 'node:fs';
import { createHash, randomUUID } from 'node:crypto';
import { join, relative } from 'node:path';
import { AuthProblem } from './auth.ts';

type Profile={name:string;phone:string};
export function parseProfile(input:unknown):Profile {
  const invalid=()=>new AuthProblem(400,'PROFILE_INVALID');
  if(!input||typeof input!=='object'||Object.keys(input).some(key=>key!=='name'&&key!=='phone')||
    !('name' in input)||typeof input.name!=='string')throw invalid();
  const name=input.name;
  // Username is an exact, case-sensitive label. Never silently strip spaces.
  if(!/^(?=.*[A-Za-z])[A-Za-z0-9]{1,80}$/.test(name))
    throw new AuthProblem(400,'USERNAME_INVALID');
  // Older API clients can still send phone; the new UI never requests it.
  const supplied=(input as {phone?:unknown}).phone;
  if(supplied!==undefined&&(typeof supplied!=='string'||!supplied.trim()||supplied.length>40||/\p{Cc}/u.test(supplied)))throw invalid();
  const phone=typeof supplied==='string'?supplied.trim().replace(/[ ()-]/g,''):'';
  return {name,phone};
}
export const profileKey=(profile:Profile)=>createHash('sha256').update(JSON.stringify(profile)).digest('hex');
const writeFailed=()=>new AuthProblem(503,'PROFILE_WRITE_FAILED');

export class WorkspaceProfile {
  private readonly workspace:string;
  private readonly directory:string;
  private readonly file:string;
  constructor(workspace:string){
    this.workspace=workspace;
    this.directory=join(workspace,'.workbench');
    this.file=join(this.directory,'user.json');
  }
  private checkedDirectory(create:boolean){
    if(relative(this.workspace,realpathSync.native(this.workspace))!=='')throw writeFailed();
    if(create&&!lstatSync(this.directory,{throwIfNoEntry:false}))mkdirSync(this.directory,{mode:0o700});
    const directory=lstatSync(this.directory);
    if(!directory.isDirectory()||directory.isSymbolicLink()||relative(this.directory,realpathSync.native(this.directory))!=='')
      throw writeFailed();
    return directory;
  }
  private checkTarget(){
    const file=lstatSync(this.file,{throwIfNoEntry:false});
    if(file&&(!file.isFile()||file.isSymbolicLink()||relative(this.file,realpathSync.native(this.file))!==''))throw writeFailed();
  }
  save(profile:Profile){
    let temporary:string|undefined;
    try{
      const directory=this.checkedDirectory(true);
      this.checkTarget();
      const path=join(this.directory,'.user-'+randomUUID()+'.tmp');
      const fd=openSync(path,'wx',0o600);
      temporary=path;
      try{writeFileSync(fd,JSON.stringify(profile.phone?profile:{name:profile.name})+'\n','utf8');fsyncSync(fd);}finally{closeSync(fd);}
      const current=this.checkedDirectory(false);
      if(current.dev!==directory.dev||current.ino!==directory.ino)throw writeFailed();
      this.checkTarget();
      renameSync(path,this.file);
      temporary=undefined;
    }catch{throw writeFailed();}
    finally{if(temporary)try{unlinkSync(temporary);}catch{}}
  }
  matches(key:string):boolean{
    try{
      this.checkedDirectory(false);
      this.checkTarget();
      const fd=openSync(this.file,constants.O_RDONLY|(constants.O_NOFOLLOW??0));
      try{
        const info=fstatSync(fd);
        return info.isFile()&&info.size<=1024&&profileKey(parseProfile(JSON.parse(readFileSync(fd,'utf8'))))===key;
      }finally{closeSync(fd);}
    }catch{return false;}
  }
}
