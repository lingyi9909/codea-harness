import mysql from 'mysql2/promise';
import type { Pool, PoolConnection } from 'mysql2/promise';

export type Tx = PoolConnection;
export type DbConfig = {
  host: string; port: number; database: string; user: string; password: string;
  tlsCa?: string;
};

export function configFromEnv(env: NodeJS.ProcessEnv): DbConfig {
  const database=env.HARNESS_MYSQL_DATABASE;
  const user=env.HARNESS_MYSQL_USER;
  const password=env.HARNESS_MYSQL_PASSWORD;
  if (!database || !/^[a-zA-Z][a-zA-Z0-9_]{0,62}$/.test(database))
    throw new Error('HARNESS_DATABASE_MISSING_OR_INVALID');
  if (!user || password === undefined) throw new Error('HARNESS_DATABASE_CREDENTIALS_MISSING');
  const port=Number(env.HARNESS_MYSQL_PORT??'3306');
  if (!Number.isInteger(port)||port<1||port>65535) throw new Error('HARNESS_MYSQL_PORT_INVALID');
  return {host:env.HARNESS_MYSQL_HOST??'127.0.0.1',port,database,user,password,
    ...(env.HARNESS_MYSQL_TLS_CA?{tlsCa:env.HARNESS_MYSQL_TLS_CA}:{})};
}
export async function createPool(config: DbConfig): Promise<Pool> {
  const pool=mysql.createPool({
    host:config.host,port:config.port,user:config.user,password:config.password,
    database:config.database, waitForConnections:true,connectionLimit:4,queueLimit:20,
    supportBigNumbers:true,bigNumberStrings:true,dateStrings:true,timezone:'Z',
    multipleStatements:false,decimalNumbers:false,
    ...(config.tlsCa?{ssl:{ca:config.tlsCa,rejectUnauthorized:true}}:{}),
  });
  try {
    const c=await pool.getConnection();
    try {
      const [rows]=await c.query<Array<mysql.RowDataPacket>>('SELECT VERSION() AS version, DATABASE() AS db');
      if (rows[0]?.db!==config.database) throw new Error('HARNESS_DATABASE_IDENTITY_MISMATCH');
      const version=String(rows[0]?.version??'');
      if(!/^8\.4(?:\.|\b)/.test(version)) throw new Error('MYSQL_8_4_REQUIRED');
      await c.query("SET SESSION time_zone = '+00:00'");
    } finally { c.release(); }
  } catch(e){await pool.end();throw e;}
  return pool;
}
export async function withTransaction<T>(pool: Pool, fn:(tx:Tx)=>Promise<T>):Promise<T> {
  const tx=await pool.getConnection();
  try {
    await tx.query("SET SESSION time_zone = '+00:00'");
    await tx.beginTransaction();
    try { const result=await fn(tx);await tx.commit();return result; }
    catch(e){await tx.rollback();throw e;}
  } finally {tx.release();}
}
