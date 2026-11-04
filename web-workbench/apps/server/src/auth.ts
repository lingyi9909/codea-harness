import type { FastifyInstance, FastifyReply, FastifyRequest } from 'fastify';
import { randomBytes,createHash,timingSafeEqual } from 'node:crypto';
import { allowedOrigin } from './config.ts';

const pairingCode=randomBytes(24).toString('base64url');
let codeUsed=false;
const codeExpires=Date.now()+5*60*1000;
const attempts:number[]=[];
const sessions=new Map<string,{csrf:string;expires:number}>();
export const pairingCodeForConsole=()=>pairingCode;
export const codeFingerprint=createHash('sha256').update(pairingCode).digest();

function cookieToken(req:FastifyRequest):string|undefined{
  const match=req.headers.cookie?.match(/(?:^|;\s*)harness_session=([^;]+)/);
  return match?.[1];
}
export function trustedRequest(req:FastifyRequest,mutation:boolean):void {
  const host=req.headers.host??'';
  if(!/^(127\.0\.0\.1|localhost)(:\d{1,5})?$/.test(host))throw httpError(403,'HOST_NOT_LOCAL');
  if(mutation) {
    const origin=req.headers.origin;
    const expected=allowedOrigin||('http://'+host);
    if(origin!==expected)throw httpError(403,'ORIGIN_INVALID');
  }
}
function httpError(status:number,code:string){
  return Object.assign(new Error(code),{statusCode:status,code});
}
export function requireSession(req:FastifyRequest):{csrf:string}{
  const cookie=cookieToken(req); const session=cookie?sessions.get(cookie):undefined;
  if(!session||session.expires<=Date.now())throw httpError(401,'AUTH_REQUIRED');
  return {csrf:session.csrf};
}
function requireCsrf(req:FastifyRequest):void{
  const session=requireSession(req);
  if(req.headers['x-csrf-token']!==session.csrf)throw httpError(403,'CSRF_INVALID');
}
export function registerAuth(app:FastifyInstance):void {
  app.addHook('onRequest',async req=>{
    const path=req.url.split('?')[0];
    const method=req.method.toUpperCase();
    const mutation=!['GET','HEAD'].includes(method);
    trustedRequest(req,mutation);
    if(path==='/api/v1/health'||(path==='/api/v1/auth/session'&&method==='POST'))return;
    requireSession(req);
    if(mutation)requireCsrf(req);
  });
  app.post('/api/v1/auth/session',async (req,reply)=>{
    const current=Date.now();
    while(attempts.length&&attempts[0]<current-60_000)attempts.shift();
    if(attempts.length>=5)throw httpError(429,'PAIRING_RATE_LIMIT');
    attempts.push(current);
    const input=(req.body as {pairingCode?:unknown})?.pairingCode;
    const buffer=typeof input==='string'?createHash('sha256').update(input).digest():Buffer.alloc(32);
    if(codeUsed||current>codeExpires||!timingSafeEqual(buffer,codeFingerprint))throw httpError(401,'PAIRING_INVALID');
    codeUsed=true;
    const token=randomBytes(32).toString('base64url');
    const csrf=randomBytes(32).toString('base64url');
    sessions.set(token,{csrf,expires:current+12*60*60*1000});
    reply.header('Set-Cookie',`harness_session=${token}; HttpOnly; SameSite=Strict; Path=/; Max-Age=43200`);
    reply.status(201).send({data:{session:'READY'},requestId:req.id});
  });
  app.delete('/api/v1/auth/session',async(req,reply)=>{
    const token=cookieToken(req);if(token)sessions.delete(token);
    reply.header('Set-Cookie','harness_session=; HttpOnly; SameSite=Strict; Path=/; Max-Age=0').status(204).send();
  });
}
export function csrfFor(req:FastifyRequest):string{return requireSession(req).csrf;}
export function httpFail(status:number,code:string):never{throw httpError(status,code);}
