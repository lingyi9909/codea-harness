import { createHash } from 'node:crypto';
import { readFile, readdir } from 'node:fs/promises';
import { resolve, join } from 'node:path';
import type { Connection, RowDataPacket } from 'mysql2/promise';
import mysql from 'mysql2/promise';
import type { DbConfig } from './db.ts';

/** MySQL DDL auto-commits; migrations use a dedicated admin command, never app startup. */
export function migrationStatements(sql:string):string[] {
  if (/\bDROP\s+(?:TABLE|DATABASE)\b/i.test(sql)) throw new Error('DROP_MIGRATION_NOT_ALLOWED');
  const stripped=sql.split(/\r?\n/).filter(line=>!line.trimStart().startsWith('--')).join('\n');
  return stripped.split(';').map(s=>s.trim()).filter(Boolean);
}
export async function runMigrations(config:DbConfig,expectedDatabase:string,migrationDirectory:string):Promise<void> {
  if(config.database!==expectedDatabase||!expectedDatabase)
    throw new Error('MIGRATION_DATABASE_CONFIRMATION_MISMATCH');
  // A separately granted migration user is required; never reuse runtime account.
  const conn:Connection=await mysql.createConnection({
    host:config.host,port:config.port,user:config.user,password:config.password,
    database:config.database,timezone:'Z',multipleStatements:false,
    supportBigNumbers:true,bigNumberStrings:true,
    ...(config.tlsCa?{ssl:{ca:config.tlsCa,rejectUnauthorized:true}}:{}),
  });
  let locked=false;
  try {
    const [dbRows]=await conn.query<RowDataPacket[]>('SELECT DATABASE() AS db, VERSION() AS v');
    if(dbRows[0]?.db!==expectedDatabase)throw new Error('WRONG_DATABASE_SELECTED');
    if(!/^8\.4(?:\.|\b)/.test(String(dbRows[0]?.v)))throw new Error('MYSQL_8_4_REQUIRED');
    const [lockRows]=await conn.query<RowDataPacket[]>('SELECT GET_LOCK(?,30) AS acquired',
      ['code_harness_migration_'+expectedDatabase]);
    if(Number(lockRows[0]?.acquired)!==1)throw new Error('MIGRATION_LOCK_UNAVAILABLE');
    locked=true;
    await conn.query(`CREATE TABLE IF NOT EXISTS wh_schema_migrations (
      version VARCHAR(100) NOT NULL PRIMARY KEY,
      sha256 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
      applied_at DATETIME(3) NOT NULL
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci`);
    const [existing]=await conn.query<RowDataPacket[]>(
      "SELECT version,sha256 FROM wh_schema_migrations ORDER BY version");
    const versions=new Map(existing.map(r=>[String(r.version),String(r.sha256)]));
    const migrationFiles=(await readdir(migrationDirectory)).filter(f=>/^\d{3}_[a-z0-9_]+\.sql$/.test(f)).sort();
    if(!migrationFiles.length)throw new Error('MIGRATION_FILES_MISSING');
    if(versions.size===0){
      const [tables]=await conn.query<RowDataPacket[]>(
        "SELECT TABLE_NAME AS n FROM information_schema.tables WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME LIKE 'wh\\_%'");
      if(tables.some(t=>t.n!=='wh_schema_migrations'))
        throw new Error('UNTRACKED_SCHEMA_TABLES_EXIST: MANUAL_RECOVERY_REQUIRED');
    }
    for(const filename of migrationFiles){
      const sql=await readFile(resolve(migrationDirectory,filename),'utf8');
      const hash=createHash('sha256').update(sql).digest('hex');
      const previous=versions.get(filename);
      if(previous){if(previous!==hash)throw new Error('MIGRATION_CHECKSUM_MISMATCH: '+filename);continue;}
      // DDL implicit commits; if a statement fails the operator must restore test DB
      // rather than blindly rerun an unknown partial migration.
      if(filename!==migrationFiles[versions.size])throw new Error('MIGRATION_OUT_OF_ORDER');
      for(const statement of migrationStatements(sql))await conn.query(statement);
      await conn.execute('INSERT INTO wh_schema_migrations(version,sha256,applied_at) VALUES (?,?,UTC_TIMESTAMP(3))',
        [filename,hash]);
      versions.set(filename,hash);
    }
  } finally {
    if(locked)await conn.query('SELECT RELEASE_LOCK(?)',['code_harness_migration_'+expectedDatabase]).catch(()=>{});
    await conn.end();
  }
}
