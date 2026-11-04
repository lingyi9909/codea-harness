import mysql, {type RowDataPacket} from 'mysql2/promise';
import { readFile, readdir } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import {config,utcNow} from './db.ts';

async function migrate() {
  const cfg=config(true);
  // Deliberate opt-in. Never run against a corporate business database.
  if (process.env.HARNESS_MIGRATION_APPROVED_DATABASE !== cfg.database ||
      !/_test$|_dev$/.test(cfg.database))
    throw new Error('MIGRATION_NOT_APPROVED_FOR_ISOLATED_DEV_OR_TEST_DATABASE');
  const conn=await mysql.createConnection({...cfg,connectionLimit:undefined,queueLimit:undefined,
    waitForConnections:undefined});
  try{
    const [lock]=await conn.query<RowDataPacket[]>("SELECT GET_LOCK('code_harness_web_migrate',30) AS locked");
    if(lock[0]?.locked!==1)throw new Error('MIGRATION_LOCK_UNAVAILABLE');
    await conn.query(`CREATE TABLE IF NOT EXISTS wh_schema_migrations(
      version VARCHAR(100) PRIMARY KEY,
      checksum CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
      status VARCHAR(16) NOT NULL,
      started_at DATETIME(3) NOT NULL,
      applied_at DATETIME(3) NULL
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci`);
    const directory=new URL('../migrations/',import.meta.url);
    const files=(await readdir(directory)).filter(f=>/^\d{3}_[a-z0-9_]+\.sql$/.test(f)).sort();
    for(const file of files) {
      const sql=await readFile(new URL(file,directory),'utf8');
      const hash=createHash('sha256').update(sql).digest('hex');
      const [existing]=await conn.execute<RowDataPacket[]>(
        'SELECT checksum,status FROM wh_schema_migrations WHERE version=?',[file]);
      if(existing.length){
        if(existing[0].checksum!==hash)throw new Error('MIGRATION_CHECKSUM_MISMATCH: '+file);
        if(existing[0].status!=='APPLIED')throw new Error('PARTIAL_MIGRATION_MANUAL_REPAIR_REQUIRED: '+file);
        console.log('Already applied: '+file);continue;
      }
      await conn.execute('INSERT INTO wh_schema_migrations(version,checksum,status,started_at) VALUES(?,?,?,?)',
        [file,hash,'APPLYING',utcNow()]);
      // Version 001 contains only CREATE/ALTER/INSERT/SET statements; no stored procedures.
      const statements=sql.split(';').map(s=>s.trim()).filter(s=>s && !s.startsWith('--'));
      // Remove comment-only preambles from SQL statements rather than skipping their following DDL.
      const fragments=sql.split(';').map(s=>s.replace(/^\s*(?:--[^\r\n]*(?:\r?\n|$)\s*)*/g,'').trim()).filter(Boolean);
      if(fragments.length<14)throw new Error('MIGRATION_SQL_INCOMPLETE: '+file);
      for(const statement of fragments)await conn.query(statement);
      await conn.execute('UPDATE wh_schema_migrations SET status=?,applied_at=? WHERE version=?',
        ['APPLIED',utcNow(),file]);
      console.log('Applied: '+file+' sha256='+hash);
    }
    await conn.query("SELECT RELEASE_LOCK('code_harness_web_migrate')");
  }finally{await conn.end();}
}
migrate().catch(error=>{console.error('Migration blocked: '+(error instanceof Error?error.message:'UNKNOWN'));process.exitCode=2;});
