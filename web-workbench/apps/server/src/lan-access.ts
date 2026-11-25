/**
 * 局域网访问边界
 *
 * 职责：实现或辅助判断 LAN 访问范围和请求约束。
 * Java 研发阅读提示：网络可达、登录状态与配置管理员权限是三个不同维度。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
/** Explicit opt-in for LAN-only HTTP testing. Production should use an HTTPS gateway. */
export function isPrivateLanIPv4(host:string):boolean {
  const octets=host.split('.');
  if(octets.length!==4||octets.some(part=>!/^(0|[1-9]\d{0,2})$/.test(part)||Number(part)>255))return false;
  const [first,second]=octets.map(Number);
  return first===10||(first===172&&second>=16&&second<=31)||(first===192&&second===168);
}

export function resolveListenHost(port:number,env:NodeJS.ProcessEnv=process.env):string {
  const host=env.HARNESS_HTTP_BIND??'127.0.0.1';
  if(host==='127.0.0.1')return host;
  if(env.HARNESS_ENABLE_LAN!=='1'||!(host==='0.0.0.0'||isPrivateLanIPv4(host)))
    throw new Error('LAN_LISTENER_REQUIRES_EXPLICIT_PRIVATE_NETWORK_OPT_IN');
  const expected=env.HARNESS_PUBLIC_ORIGIN;
  let url:URL;
  try{url=new URL(expected??'');}
  catch{throw new Error('LAN_PUBLIC_ORIGIN_INVALID');}
  if(url.origin!==expected||url.protocol!=='http:'||!isPrivateLanIPv4(url.hostname)||Number(url.port)!==port)
    throw new Error('LAN_PUBLIC_ORIGIN_INVALID');
  return host;
}
