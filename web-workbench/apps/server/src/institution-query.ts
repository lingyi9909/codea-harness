import {createHash} from 'node:crypto';
import {AuthProblem} from './auth.ts';
import type {EmbedStore} from '../../../packages/storage/src/embed.ts';
export type InstitutionQuery={day:string;from:number;to:number;msgSn:string;account:string;password:string;verificationCode:string};
export type InstitutionQueryAdapter=(input:InstitutionQuery,scope:{grantId:string})=>Promise<{evidence:string}>;
export async function institutionEvidence(adapter:InstitutionQueryAdapter|undefined,store:EmbedStore,grantId:string,input:unknown){
  if(!adapter)throw new AuthProblem(503,'INSTITUTION_ADAPTER_NOT_CONFIGURED');
  const b=input as InstitutionQuery;
  if(!b||Object.keys(b).sort().join(',')!=='account,day,from,msgSn,password,to,verificationCode'||
    !/^\d{4}-\d{2}-\d{2}$/.test(b.day)||!Number.isSafeInteger(b.from)||!Number.isSafeInteger(b.to)||b.from%1000||b.to%1000||b.to-b.from<22*3600000||b.to-b.from>26*3600000||
    !['msgSn','account','password','verificationCode'].every(k=>typeof b[k as keyof InstitutionQuery]==='string'&&(b[k as keyof InstitutionQuery] as string).length>0&&(b[k as keyof InstitutionQuery] as string).length<257))throw new AuthProblem(400,'INSTITUTION_INPUT_INVALID');
  const fingerprint=createHash('sha256').update(JSON.stringify([b.day,b.from,b.to,b.msgSn,b.account])).digest('hex');
  const attempt=await store.claimQuery(grantId,fingerprint);
  if(!attempt.claimed){
    if(attempt.fingerprint!==fingerprint)throw new AuthProblem(409,'INSTITUTION_NEW_PAGE_REQUIRED');
    if(attempt.state==='READY'&&attempt.evidence)return {evidence:attempt.evidence};
    throw new AuthProblem(409,'INSTITUTION_OUTCOME_UNKNOWN');
  }
  try{
    const result=await adapter(b,{grantId});
    if(!result||typeof result.evidence!=='string'||!result.evidence.trim()||result.evidence.length>10000||
      [b.account,b.password,b.verificationCode].some(secret=>result.evidence.includes(secret))||/bearer\s+[\w.-]+|(?:access[_ -]?token|password|验证码|密码)\s*[:=]/i.test(result.evidence))throw Error('UNSAFE_INSTITUTION_EVIDENCE');
    await store.finishQuery(grantId,result.evidence);
    return {evidence:result.evidence};
  }catch{await store.finishQuery(grantId).catch(()=>{});throw new AuthProblem(502,'INSTITUTION_QUERY_FAILED');}
}
export function httpInstitutionAdapter(urlText:string,authorization:string):InstitutionQueryAdapter{
  const url=new URL(urlText);
  if(url.username||url.password||url.hash||url.search||!(url.protocol==='https:'||url.protocol==='http:'&&['127.0.0.1','localhost','[::1]'].includes(url.hostname))||!authorization)throw Error('INSTITUTION_ADAPTER_CONFIG_INVALID');
  return async(input,scope)=>{
    const response=await fetch(url,{method:'POST',redirect:'error',signal:AbortSignal.timeout(30000),headers:{Authorization:authorization,'Content-Type':'application/json','X-Workbench-Grant':scope.grantId},body:JSON.stringify(input)});
    if(!response.ok||!response.body)throw Error('INSTITUTION_ADAPTER_FAILED');
    const reader=response.body.getReader(),chunks:Uint8Array[]=[];let size=0;
    try{while(true){const {done,value}=await reader.read();if(done)break;size+=value.byteLength;if(size>32768)throw Error('INSTITUTION_RESPONSE_LIMIT');chunks.push(value);}}finally{await reader.cancel().catch(()=>{});}
    const body=JSON.parse(Buffer.concat(chunks).toString('utf8'));
    if(Object.keys(body).join(',')!=='evidence')throw Error('INSTITUTION_RESPONSE_INVALID');
    return {evidence:body.evidence};
  };
}
