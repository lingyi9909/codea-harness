import type {Pool,RowDataPacket,ResultSetHeader} from 'mysql2/promise';
import {withTransaction} from './db.ts';
export type EmbedTicket={digest:string;ownerId:string;clientId:string;parentOrigin:string;expires:number};
export type EmbedGrant={id:string;digest:string;ownerId:string;parentOrigin:string;expires:number;accessMode?:'HOST'|'ANONYMOUS';nativeId?:string;serviceIdentity?:string;state:'NEW'|'CREATING'|'BOUND'|'FAILED'};
export interface EmbedStore {
  createSingle?(grant:Pick<EmbedGrant,'id'|'digest'|'expires'|'parentOrigin'>,serviceIdentity:string,maxPages:number):Promise<EmbedGrant|undefined>;
  allocate(grant:Pick<EmbedGrant,'id'|'digest'|'expires'|'parentOrigin'>,slots:{ownerId:string;serviceIdentity:string}[]):Promise<EmbedGrant|undefined>;
  recycled(id:string):Promise<void>;
  claimQuery(id:string,fingerprint:string):Promise<{state:string;fingerprint?:string;evidence?:string;claimed:boolean}>;
  finishQuery(id:string,evidence?:string):Promise<void>;
  ticket(ticket:EmbedTicket):Promise<void>;
  exchange(digest:string,parentOrigin:string,grant:Pick<EmbedGrant,'id'|'digest'|'expires'>):Promise<EmbedGrant|undefined>;
  find(digest:string):Promise<EmbedGrant|undefined>;
  reserve(id:string):Promise<boolean>;
  bind(id:string,nativeId:string,serviceIdentity:string):Promise<void>;
  fail(id:string):Promise<void>;
  revoke(id:string):Promise<void>;
  cleanupCandidates():Promise<EmbedGrant[]>;
  cleaned(id:string):Promise<void>;
  serialize<T>(ownerId:string,fn:()=>Promise<T>):Promise<T>;
  otherSessions(ownerId:string,grantId:string):Promise<string[]>;
}
const date=(n:number)=>new Date(n).toISOString().slice(0,23).replace('T',' ');
export class MysqlEmbedStore implements EmbedStore {
  private readonly pool:Pool;
  private readonly locks:Pool;
  constructor(pool:Pool,locks:Pool){this.pool=pool;this.locks=locks;}
  async allocate(g:Pick<EmbedGrant,'id'|'digest'|'expires'|'parentOrigin'>,slots:{ownerId:string;serviceIdentity:string}[]){
    for(const slot of [...slots].sort((a,b)=>a.ownerId.localeCompare(b.ownerId))){
      try{
        const grant=await this.serialize(slot.ownerId,()=>withTransaction(this.pool,async tx=>{
          await tx.execute("INSERT INTO wh_owners(id,subject,display_name,created_at,updated_at) VALUES (?,?,'Anonymous execution slot',UTC_TIMESTAMP(3),UTC_TIMESTAMP(3)) ON DUPLICATE KEY UPDATE id=id",[slot.ownerId,'embed-slot:'+slot.ownerId]);
          await tx.execute('SELECT id FROM wh_owners WHERE id=? FOR UPDATE',[slot.ownerId]);
          const [rows]=await tx.execute<RowDataPacket[]>('SELECT id FROM wh_embed_sessions WHERE owner_id=? AND (cleaned_at IS NULL OR (service_identity=? AND recycled_at IS NULL)) LIMIT 1',[slot.ownerId,slot.serviceIdentity]);
          if(rows.length)return undefined;
          await tx.execute("INSERT INTO wh_embed_sessions(id,digest,owner_id,parent_origin,expires_at,service_identity,access_mode) VALUES (?,?,?,?,?,?,'ANONYMOUS')",[g.id,g.digest,slot.ownerId,g.parentOrigin,date(g.expires),slot.serviceIdentity]);
          return {...g,...slot,state:'NEW' as const,accessMode:'ANONYMOUS' as const};
        }));
        if(grant)return grant;
      }catch(error){if(!(error instanceof Error)||error.message!=='EMBED_EXECUTOR_BUSY')throw error;}
    }
    return undefined;
  }
  async claimQuery(id:string,fingerprint:string){
    return withTransaction(this.pool,async tx=>{
      const [rows]=await tx.execute<RowDataPacket[]>('SELECT query_state,query_fingerprint,query_evidence FROM wh_embed_sessions WHERE id=? AND revoked_at IS NULL AND expires_at>UTC_TIMESTAMP(3) FOR UPDATE',[id]);
      const row=rows[0];if(!row)throw new Error('EMBED_SESSION_REQUIRED');
      if(row.query_state!=='UNUSED')return {state:String(row.query_state),fingerprint:String(row.query_fingerprint),evidence:row.query_evidence?String(row.query_evidence):undefined,claimed:false};
      await tx.execute("UPDATE wh_embed_sessions SET query_state='RUNNING',query_fingerprint=? WHERE id=?",[fingerprint,id]);
      return {state:'RUNNING',fingerprint,claimed:true};
    });
  }
  async finishQuery(id:string,evidence?:string){await this.pool.execute("UPDATE wh_embed_sessions SET query_state=?,query_evidence=? WHERE id=? AND query_state='RUNNING'",[evidence===undefined?'FAILED':'READY',evidence??null,id]);}
  async recycled(id:string){await this.pool.execute('UPDATE wh_embed_sessions SET recycled_at=UTC_TIMESTAMP(3) WHERE id=? AND cleaned_at IS NOT NULL',[id]);}
  async ticket(t:EmbedTicket){await this.pool.execute('INSERT INTO wh_embed_tickets(digest,owner_id,client_id,parent_origin,expires_at) VALUES (?,?,?,?,?)',[t.digest,t.ownerId,t.clientId,t.parentOrigin,date(t.expires)]);}
  async exchange(digest:string,parentOrigin:string,g:Pick<EmbedGrant,'id'|'digest'|'expires'>){
    return withTransaction(this.pool,async tx=>{
      const [rows]=await tx.execute<RowDataPacket[]>('SELECT owner_id,parent_origin FROM wh_embed_tickets WHERE digest=? AND parent_origin=? AND consumed_at IS NULL AND expires_at>UTC_TIMESTAMP(3) FOR UPDATE',[digest,parentOrigin]);
      if(!rows[0])return undefined;
      await tx.execute('UPDATE wh_embed_tickets SET consumed_at=UTC_TIMESTAMP(3) WHERE digest=?',[digest]);
      const grant:EmbedGrant={...g,ownerId:String(rows[0].owner_id),parentOrigin,state:'NEW'};
      await tx.execute('INSERT INTO wh_embed_sessions(id,digest,owner_id,parent_origin,expires_at) VALUES (?,?,?,?,?)',[g.id,g.digest,grant.ownerId,parentOrigin,date(g.expires)]);
      return grant;
    });
  }
  async find(digest:string){
    const [rows]=await this.pool.execute<RowDataPacket[]>(`SELECT id,owner_id,parent_origin,native_session_id,service_identity,access_mode,state,TIMESTAMPDIFF(MICROSECOND,'1970-01-01 00:00:00',expires_at)/1000 AS expires FROM wh_embed_sessions WHERE digest=? AND revoked_at IS NULL AND expires_at>UTC_TIMESTAMP(3)`,[digest]);
    const r=rows[0];return r?{id:String(r.id),digest,ownerId:String(r.owner_id),parentOrigin:String(r.parent_origin),accessMode:r.access_mode as EmbedGrant['accessMode'],nativeId:r.native_session_id?String(r.native_session_id):undefined,serviceIdentity:r.service_identity?String(r.service_identity):undefined,state:r.state as EmbedGrant['state'],expires:Number(r.expires)}:undefined;
  }
  async reserve(id:string){const [r]=await this.pool.execute<ResultSetHeader>("UPDATE wh_embed_sessions SET state='CREATING' WHERE id=? AND state='NEW' AND revoked_at IS NULL AND expires_at>UTC_TIMESTAMP(3)",[id]);return r.affectedRows===1;}
  async bind(id:string,nativeId:string,serviceIdentity:string){const [r]=await this.pool.execute<ResultSetHeader>("UPDATE wh_embed_sessions SET state='BOUND',native_session_id=?,service_identity=? WHERE id=? AND state='CREATING' AND revoked_at IS NULL AND expires_at>UTC_TIMESTAMP(3)",[nativeId,serviceIdentity,id]);if(r.affectedRows!==1)throw new Error('EMBED_BIND_FAILED');}
  async fail(id:string){await this.pool.execute("UPDATE wh_embed_sessions SET state='FAILED' WHERE id=? AND state='CREATING'",[id]);}
  async revoke(id:string){await this.pool.execute('UPDATE wh_embed_sessions SET revoked_at=UTC_TIMESTAMP(3) WHERE id=? AND revoked_at IS NULL',[id]);}
  async cleanupCandidates(){
    const [rows]=await this.pool.execute<RowDataPacket[]>(`SELECT id,digest,owner_id,parent_origin,native_session_id,service_identity,access_mode,state,TIMESTAMPDIFF(MICROSECOND,'1970-01-01 00:00:00',expires_at)/1000 AS expires FROM wh_embed_sessions WHERE (expires_at<=UTC_TIMESTAMP(3) OR revoked_at IS NOT NULL) AND (cleaned_at IS NULL OR (access_mode='ANONYMOUS' AND recycled_at IS NULL)) LIMIT 100`);
    return rows.map(r=>({id:String(r.id),digest:String(r.digest),ownerId:String(r.owner_id),parentOrigin:String(r.parent_origin),accessMode:r.access_mode as EmbedGrant['accessMode'],nativeId:r.native_session_id?String(r.native_session_id):undefined,serviceIdentity:r.service_identity?String(r.service_identity):undefined,state:r.state as EmbedGrant['state'],expires:Number(r.expires)}));
  }
  async cleaned(id:string){await this.pool.execute('UPDATE wh_embed_sessions SET cleaned_at=UTC_TIMESTAMP(3),query_evidence=NULL WHERE id=?',[id]);}
  async otherSessions(ownerId:string,grantId:string){
    const [rows]=await this.pool.execute<RowDataPacket[]>("SELECT native_session_id FROM wh_embed_sessions WHERE owner_id=? AND id<>? AND state='BOUND' AND revoked_at IS NULL AND expires_at>UTC_TIMESTAMP(3)",[ownerId,grantId]);
    return rows.map(r=>String(r.native_session_id));
  }
  async serialize<T>(ownerId:string,fn:()=>Promise<T>):Promise<T>{
    // Separate pool: lock holders must not starve the queries executed by fn().
    const connection=await this.locks.getConnection(),key='embed-submit:'+ownerId;let acquired=false;
    try{
      const [rows]=await connection.query<RowDataPacket[]>('SELECT GET_LOCK(?,1) AS acquired',[key]);
      acquired=Number(rows[0]?.acquired)===1;if(!acquired)throw new Error('EMBED_EXECUTOR_BUSY');
      return await fn();
    }finally{if(acquired)await connection.query('SELECT RELEASE_LOCK(?)',[key]).catch(()=>{});connection.release();}
  }
}
