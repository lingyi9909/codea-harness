import mysql, { type Pool, type PoolConnection, type RowDataPacket, type ResultSetHeader } from 'mysql2/promise';
import { createHash, randomUUID } from 'node:crypto';

let pool: Pool | undefined;
export const utcNow = () => new Date().toISOString().slice(0,23).replace('T',' ');
export const futureTime = (seconds:number) => new Date(Date.now()+seconds*1000).toISOString().slice(0,23).replace('T',' ');
export const uuid = randomUUID;
export const sha256 = (s:string|Buffer) => createHash('sha256').update(s).digest('hex');
export type DbRow = RowDataPacket & Record<string, unknown>;

export function config(migration=false) {
  const database=process.env.HARNESS_MYSQL_DATABASE;
  if (!database || !/^[a-zA-Z][a-zA-Z0-9_]{2,63}$/.test(database)) throw new Error('MYSQL_DATABASE_NAME_REQUIRED');
  const username=migration?process.env.HARNESS_MYSQL_MIGRATION_USER:process.env.HARNESS_MYSQL_USER;
  const password=migration?process.env.HARNESS_MYSQL_MIGRATION_PASSWORD:process.env.HARNESS_MYSQL_PASSWORD;
  if (!username||!password) throw new Error(migration?'MYSQL_MIGRATION_CREDENTIALS_REQUIRED':'MYSQL_APP_CREDENTIALS_REQUIRED');
  const host=process.env.HARNESS_MYSQL_HOST??'127.0.0.1';
  const port=Number(process.env.HARNESS_MYSQL_PORT??'3306');
  if (!Number.isInteger(port)||port<1||port>65535) throw new Error('MYSQL_PORT_INVALID');
  const args={
    host,port,user:username,password,database,
    charset:'utf8mb4',timezone:'Z' as const,dateStrings:true,
    supportBigNumbers:true,bigNumberStrings:true,decimalNumbers:false,
    waitForConnections:true,connectionLimit:4,queueLimit:20,
    enableKeepAlive:true,connectTimeout:5000,
    multipleStatements:false,
  };
  // MySQL TLS to be validated against the corporate CA, never disable certificate verification.
  return args;
}
export function db(): Pool {
  if (!pool) pool=mysql.createPool(config());
  return pool;
}
export async function withTransaction<T>(fn:(tx:PoolConnection)=>Promise<T>):Promise<T> {
  const conn=await db().getConnection();
  try { await conn.beginTransaction();const result=await fn(conn);await conn.commit();return result; }
  catch(e){await conn.rollback();throw e;}
  finally{conn.release();}
}
export async function query<T extends DbRow>(sql:string, params:(string|number|boolean|null)[]=[]):Promise<T[]> {
  const [rows]=await db().execute<RowDataPacket[]>(sql,params);
  return rows as T[];
}
export async function execute(sql:string,params:(string|number|boolean|null)[]=[]):Promise<ResultSetHeader> {
  const [out]=await db().execute<ResultSetHeader>(sql,params); return out;
}
export async function databaseReady():Promise<boolean>{
  try{await query('SELECT 1');return true;}catch{return false;}
}
export async function ensureLocalOwner():Promise<{id:string;name:string}> {
  const rows=await query<DbRow>('SELECT id,display_name FROM wh_owners WHERE subject=? LIMIT 1',['local-user']);
  if(rows.length)return {id:String(rows[0].id),name:String(rows[0].display_name)};
  const id=uuid(), now=utcNow();
  await execute('INSERT INTO wh_owners (id,subject,display_name,created_at,updated_at) VALUES (?,?,?,?,?) ON DUPLICATE KEY UPDATE id=id',
    [id,'local-user','本机研发用户',now,now]);
  const selected=await query<DbRow>('SELECT id,display_name FROM wh_owners WHERE subject=? LIMIT 1',['local-user']);
  if(!selected.length)throw new Error('LOCAL_OWNER_NOT_INITIALIZED');
  return {id:String(selected[0].id),name:String(selected[0].display_name)};
}
