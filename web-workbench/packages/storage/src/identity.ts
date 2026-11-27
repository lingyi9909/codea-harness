/**
 * 身份与归属存储
 *
 * 职责：持久化执行用户和业务身份关联信息。
 * Java 研发阅读提示：区分调用者自报 ID 与服务端已验证身份，防止数据跨租户访问。
 *
 * 仅补充模块注释，原有代码和接口行为保持不变。
 */
import { randomUUID } from 'node:crypto';
import type { Pool, RowDataPacket } from 'mysql2/promise';
import { withTransaction } from './db.ts';

export type Identity={id:string;name:string;phone:string;status:string};
export type StoredSession={owner:Identity;csrfDigest:string;expires:number};
export interface IdentityStore {
  identify(name:string,phone:string):Promise<Identity>;
  issue(ownerId:string,tokenDigest:string,csrfDigest:string,expires:number):Promise<void>;
  find(tokenDigest:string):Promise<StoredSession|undefined>;
  revoke(tokenDigest:string):Promise<void>;
}
const identity=(r:RowDataPacket):Identity=>({id:String(r.owner_id),name:String(r.username),phone:String(r.phone_normalized),status:String(r.status)});
const date=(ms:number)=>new Date(ms).toISOString().slice(0,23).replace('T',' ');
export class MysqlIdentityStore implements IdentityStore {
  private readonly pool:Pool;
  constructor(pool:Pool){this.pool=pool;}
  async identify(name:string,phone:string):Promise<Identity>{
    const read=async()=>{
      const [rows]=await this.pool.execute<RowDataPacket[]>('SELECT owner_id,username,phone_normalized,status FROM wh_user_profiles WHERE phone_normalized=?',[phone]);
      return rows[0]?identity(rows[0]):undefined;
    };
    const existing=await read();if(existing)return existing;
    const id=randomUUID();
    try{
      await withTransaction(this.pool,async tx=>{
        await tx.execute('INSERT INTO wh_owners(id,subject,display_name,created_at,updated_at) VALUES (?,?,?,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))',[id,'user:'+id,name]);
        await tx.execute('INSERT INTO wh_user_profiles(owner_id,username,phone_normalized,status,created_at,updated_at) VALUES (?,?,?,\'ACTIVE\',UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))',[id,name,phone]);
      });
    }catch(e){
      if((e as {code?:string}).code!=='ER_DUP_ENTRY')throw e;
      const winner=await read();if(winner)return winner;throw e;
    }
    return {id,name,phone,status:'ACTIVE'};
  }
  async profile(ownerId:string):Promise<Identity|undefined>{
    const [rows]=await this.pool.execute<RowDataPacket[]>('SELECT owner_id,username,phone_normalized,status FROM wh_user_profiles WHERE owner_id=?',[ownerId]);
    return rows[0]?identity(rows[0]):undefined;
  }
  async issue(ownerId:string,tokenDigest:string,csrfDigest:string,expires:number){
    await this.pool.execute('INSERT INTO wh_web_sessions(id,owner_id,token_digest,csrf_digest,expires_at,created_at) VALUES (?,?,?,?,?,UTC_TIMESTAMP(3))',
      [randomUUID(),ownerId,tokenDigest,csrfDigest,date(expires)]);
  }
  async find(tokenDigest:string):Promise<StoredSession|undefined>{
    const [rows]=await this.pool.execute<RowDataPacket[]>(`SELECT p.owner_id,p.username,p.phone_normalized,p.status,s.csrf_digest,
      TIMESTAMPDIFF(MICROSECOND,'1970-01-01 00:00:00',s.expires_at)/1000 AS expires FROM wh_web_sessions s JOIN wh_user_profiles p ON p.owner_id=s.owner_id
      WHERE s.token_digest=? AND s.revoked_at IS NULL AND s.expires_at>UTC_TIMESTAMP(3) AND p.status='ACTIVE'`,[tokenDigest]);
    return rows[0]?{owner:identity(rows[0]),csrfDigest:String(rows[0].csrf_digest),expires:Number(rows[0].expires)}:undefined;
  }
  async revoke(tokenDigest:string){
    await this.pool.execute('UPDATE wh_web_sessions SET revoked_at=UTC_TIMESTAMP(3) WHERE token_digest=? AND revoked_at IS NULL',[tokenDigest]);
  }
}
