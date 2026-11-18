import {DatabaseSync} from 'node:sqlite';
import {createHash,randomUUID} from 'node:crypto';
import {mkdirSync,readFileSync,readdirSync,lstatSync,chmodSync,existsSync} from 'node:fs';
import {dirname,isAbsolute,join} from 'node:path';
import {fileURLToPath} from 'node:url';
import {setTimeout as delay} from 'node:timers/promises';
import type {EmbedStore,EmbedGrant,EmbedTicket} from './embed.ts';
import type {ChatBindingStore} from './chat-bindings.ts';
type Row=Record<string,string|number|bigint|Uint8Array|null>;
const grant=(r:Row):EmbedGrant=>({id:String(r.id),digest:String(r.digest),ownerId:String(r.owner_id),parentOrigin:String(r.parent_origin),expires:Number(r.expires),accessMode:r.access_mode as EmbedGrant['accessMode'],state:r.state as EmbedGrant['state'],nativeId:r.native_session_id?String(r.native_session_id):undefined,serviceIdentity:r.service_identity?String(r.service_identity):undefined});
const alive=(pid:number)=>{if(!Number.isInteger(pid)||pid<=0)return true;try{process.kill(pid,0);return true;}catch(e){return !(e&&typeof e==='object'&&'code'in e&&e.code==='ESRCH');}};
/** Local-disk, single-host persistence. No native npm extension or database service. */
export class SqliteEmbedStore implements EmbedStore {
  private readonly db:DatabaseSync;
  private closing=false;
  private readonly pending=new Set<Promise<unknown>>();
  readonly bindings:ChatBindingStore;
  constructor(path:string){
    if(!isAbsolute(path)||path.startsWith('\\\\')||path.startsWith('//'))throw Error('SQLITE_LOCAL_ABSOLUTE_PATH_REQUIRED');
    mkdirSync(dirname(path),{recursive:true,mode:0o700});
    if(existsSync(path)&&lstatSync(path).isSymbolicLink())throw Error('SQLITE_SYMLINK_FORBIDDEN');
    this.db=new DatabaseSync(path);
    try{
      chmodSync(path,0o600);
      this.db.exec('PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON; PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL;');
      this.migrate();
    }catch(e){this.db.close();throw e;}
    this.bindings={
      create:async b=>{this.db.prepare('INSERT INTO wh_chat_session_bindings(service_identity,native_session_id,owner_id,scenario,directory) VALUES (?,?,?,?,?)').run(b.serviceIdentity,b.id,b.ownerId,b.scenario,b.directory);},
      get:async(serviceIdentity,id,ownerId)=>{const r=this.db.prepare('SELECT scenario,directory FROM wh_chat_session_bindings WHERE service_identity=? AND native_session_id=? AND owner_id=? AND deleted=0').get(serviceIdentity,id,ownerId);return r?{serviceIdentity,id,ownerId,scenario:String(r.scenario),directory:String(r.directory)}:undefined;},
      remove:async(service,id,owner)=>{this.db.prepare('UPDATE wh_chat_session_bindings SET deleted=1 WHERE service_identity=? AND native_session_id=? AND owner_id=?').run(service,id,owner);},
    };
  }
  private transaction<T>(fn:()=>T):T{
    this.db.exec('BEGIN IMMEDIATE');try{const value=fn();this.db.exec('COMMIT');return value;}catch(e){this.db.exec('ROLLBACK');throw e;}
  }
  private migrate(){
    this.transaction(()=>{
      const tables=this.db.prepare("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").all();
      if(tables.length&&!tables.some(t=>t.name==='wh_sqlite_migrations'))throw Error('SQLITE_UNTRACKED_DATABASE');
      this.db.exec('CREATE TABLE IF NOT EXISTS wh_sqlite_migrations(version TEXT PRIMARY KEY,sha256 TEXT NOT NULL)');
      const directory=fileURLToPath(new URL('../migrations/sqlite/',import.meta.url));
      const files=readdirSync(directory).filter(f=>/^\d{3}_[a-z0-9_]+\.sql$/.test(f)).sort();
      const recorded=this.db.prepare('SELECT version,sha256 FROM wh_sqlite_migrations ORDER BY version').all();
      if(recorded.some((r,i)=>r.version!==files[i]))throw Error('SQLITE_MIGRATION_HISTORY_MISMATCH');
      for(const [i,file]of files.entries()){
        const sql=readFileSync(join(directory,file),'utf8'),digest=createHash('sha256').update(sql).digest('hex');
        if(recorded[i]){if(recorded[i].sha256!==digest)throw Error('SQLITE_MIGRATION_CHECKSUM_MISMATCH');continue;}
        this.db.exec(sql);this.db.prepare('INSERT INTO wh_sqlite_migrations(version,sha256) VALUES (?,?)').run(file,digest);
      }
    });
  }
  check(){this.db.prepare('SELECT 1').get();}
  async ensureOwner(id:string,subject:string){this.db.prepare('INSERT INTO wh_owners(id,subject) VALUES (?,?) ON CONFLICT(id) DO NOTHING').run(id,subject);}
  async allocate(g:Pick<EmbedGrant,'id'|'digest'|'expires'|'parentOrigin'>,slots:{ownerId:string;serviceIdentity:string}[]){
    for(const slot of [...slots].sort((a,b)=>a.ownerId.localeCompare(b.ownerId))){
      try{
        const result=await this.serialize(slot.ownerId,async()=>this.transaction(()=>{
          this.db.prepare('INSERT INTO wh_owners(id,subject) VALUES (?,?) ON CONFLICT(id) DO NOTHING').run(slot.ownerId,'embed-slot:'+slot.ownerId);
          if(this.db.prepare('SELECT id FROM wh_embed_sessions WHERE owner_id=? AND (cleaned=0 OR (service_identity=? AND recycled=0)) LIMIT 1').get(slot.ownerId,slot.serviceIdentity))return undefined;
          this.db.prepare("INSERT INTO wh_embed_sessions(id,digest,owner_id,parent_origin,expires,service_identity,access_mode) VALUES (?,?,?,?,?,?,'ANONYMOUS')").run(g.id,g.digest,slot.ownerId,g.parentOrigin,g.expires,slot.serviceIdentity);
          return {...g,...slot,state:'NEW' as const,accessMode:'ANONYMOUS' as const};
        }));if(result)return result;
      }catch(e){if(!(e instanceof Error)||e.message!=='EMBED_EXECUTOR_BUSY')throw e;}
    }return undefined;
  }
  async ticket(t:EmbedTicket){this.db.prepare('INSERT INTO wh_embed_tickets(digest,owner_id,client_id,parent_origin,expires) VALUES (?,?,?,?,?)').run(t.digest,t.ownerId,t.clientId,t.parentOrigin,t.expires);}
  async exchange(digest:string,parentOrigin:string,g:Pick<EmbedGrant,'id'|'digest'|'expires'>){return this.transaction(()=>{
    const r=this.db.prepare('SELECT owner_id FROM wh_embed_tickets WHERE digest=? AND parent_origin=? AND consumed=0 AND expires>?').get(digest,parentOrigin,Date.now());if(!r)return undefined;
    this.db.prepare('UPDATE wh_embed_tickets SET consumed=1 WHERE digest=?').run(digest);
    this.db.prepare('INSERT INTO wh_embed_sessions(id,digest,owner_id,parent_origin,expires) VALUES (?,?,?,?,?)').run(g.id,g.digest,String(r.owner_id),parentOrigin,g.expires);
    return {...g,ownerId:String(r.owner_id),parentOrigin,state:'NEW' as const};
  });}
  async find(digest:string){const r=this.db.prepare('SELECT * FROM wh_embed_sessions WHERE digest=? AND revoked=0 AND expires>?').get(digest,Date.now());return r?grant(r):undefined;}
  async reserve(id:string){return Number(this.db.prepare("UPDATE wh_embed_sessions SET state='CREATING' WHERE id=? AND state='NEW' AND revoked=0 AND expires>?").run(id,Date.now()).changes)===1;}
  async bind(id:string,nativeId:string,service:string){if(Number(this.db.prepare("UPDATE wh_embed_sessions SET state='BOUND',native_session_id=?,service_identity=? WHERE id=? AND state='CREATING' AND revoked=0 AND expires>?").run(nativeId,service,id,Date.now()).changes)!==1)throw Error('EMBED_BIND_FAILED');}
  async fail(id:string){this.db.prepare("UPDATE wh_embed_sessions SET state='FAILED' WHERE id=? AND state='CREATING'").run(id);}
  async revoke(id:string){this.db.prepare('UPDATE wh_embed_sessions SET revoked=1 WHERE id=?').run(id);}
  async cleanupCandidates(){return this.db.prepare("SELECT * FROM wh_embed_sessions WHERE (expires<=? OR revoked=1) AND (cleaned=0 OR (access_mode='ANONYMOUS' AND recycled=0)) LIMIT 100").all(Date.now()).map(grant);}
  async cleaned(id:string){this.db.prepare('UPDATE wh_embed_sessions SET cleaned=1,query_evidence=NULL WHERE id=?').run(id);}
  async recycled(id:string){this.db.prepare('UPDATE wh_embed_sessions SET recycled=1 WHERE id=? AND cleaned=1').run(id);}
  async otherSessions(owner:string,id:string){return this.db.prepare("SELECT native_session_id FROM wh_embed_sessions WHERE owner_id=? AND id<>? AND state='BOUND' AND revoked=0 AND expires>?").all(owner,id,Date.now()).map(r=>String(r.native_session_id));}
  async claimQuery(id:string,fingerprint:string){return this.transaction(()=>{
    const r=this.db.prepare('SELECT query_state,query_fingerprint,query_evidence FROM wh_embed_sessions WHERE id=? AND revoked=0 AND expires>?').get(id,Date.now());if(!r)throw Error('EMBED_SESSION_REQUIRED');
    if(r.query_state!=='UNUSED')return {state:String(r.query_state),fingerprint:String(r.query_fingerprint),evidence:r.query_evidence?String(r.query_evidence):undefined,claimed:false};
    this.db.prepare("UPDATE wh_embed_sessions SET query_state='RUNNING',query_fingerprint=? WHERE id=?").run(fingerprint,id);return {state:'RUNNING',fingerprint,claimed:true};
  });}
  async finishQuery(id:string,evidence?:string){this.db.prepare("UPDATE wh_embed_sessions SET query_state=?,query_evidence=? WHERE id=? AND query_state='RUNNING'").run(evidence===undefined?'FAILED':'READY',evidence??null,id);}
  async serialize<T>(owner:string,fn:()=>Promise<T>):Promise<T>{
    if(this.closing)throw Error('SQLITE_CLOSED');
    const operation=this.locked(owner,fn);this.pending.add(operation);try{return await operation;}finally{this.pending.delete(operation);}
  }
  private async locked<T>(owner:string,fn:()=>Promise<T>){
    const nonce=randomUUID(),deadline=Date.now()+1000;let acquired=false;
    try{
      while(!acquired){
        acquired=Number(this.db.prepare('INSERT INTO wh_embed_locks(owner_id,nonce,pid) VALUES (?,?,?) ON CONFLICT(owner_id) DO NOTHING').run(owner,nonce,process.pid).changes)===1;
        if(acquired)break;
        const lock=this.db.prepare('SELECT pid FROM wh_embed_locks WHERE owner_id=?').get(owner);
        // Never auto-reclaim: an external reset process may outlive a crashed API.
        if(lock&&!alive(Number(lock.pid)))throw Error('SQLITE_EXECUTOR_RECOVERY_REQUIRED');
        if(Date.now()>=deadline)throw Error('EMBED_EXECUTOR_BUSY');await delay(10);
      }
      return await fn();
    }finally{if(acquired)this.db.prepare('DELETE FROM wh_embed_locks WHERE owner_id=? AND nonce=?').run(owner,nonce);}
  }
  recoveryLocks(){return this.db.prepare('SELECT owner_id,nonce,pid FROM wh_embed_locks').all().map(r=>({ownerId:String(r.owner_id),nonce:String(r.nonce),pid:Number(r.pid),holderAlive:alive(Number(r.pid))}));}
  recoverLock(owner:string,nonce:string,confirmedStopped:boolean){
    if(!confirmedStopped)throw Error('SQLITE_RECOVERY_CONFIRM_REQUIRED');
    const row=this.db.prepare('SELECT pid FROM wh_embed_locks WHERE owner_id=? AND nonce=?').get(owner,nonce);
    if(!row||alive(Number(row.pid)))throw Error('SQLITE_RECOVERY_LIVE_OR_CHANGED');
    this.db.prepare('DELETE FROM wh_embed_locks WHERE owner_id=? AND nonce=?').run(owner,nonce);
  }
  async backup(path:string){if(!isAbsolute(path)||existsSync(path))throw Error('SQLITE_NEW_BACKUP_PATH_REQUIRED');const {backup}=await import('node:sqlite');await backup(this.db,path);chmodSync(path,0o600);}
  async close(){this.closing=true;await Promise.allSettled([...this.pending]);this.db.close();}
}
