import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import { configFromEnv } from '../packages/storage/src/db.ts';
import { runMigrations } from '../packages/storage/src/migrate.ts';

async function main(){
  const database=process.env.HARNESS_MYSQL_DATABASE;
  if(!database||process.env.HARNESS_MIGRATION_CONFIRM_DATABASE!==database)
    throw new Error('MIGRATION_CONFIRM_DATABASE_REQUIRED');
  const cfg=configFromEnv({
    ...process.env,
    HARNESS_MYSQL_USER:process.env.HARNESS_MIGRATION_USER,
    HARNESS_MYSQL_PASSWORD:process.env.HARNESS_MIGRATION_PASSWORD,
  });
  // Never run migrations on app startup or on an unconfirmed database.
  await runMigrations(cfg,database,resolve('packages/storage/migrations'));
  console.log('Schema migrations verified for approved database '+database);
}
main().catch(e=>{console.error('Migration blocked:',e instanceof Error?e.message:'UNKNOWN');process.exitCode=2});
