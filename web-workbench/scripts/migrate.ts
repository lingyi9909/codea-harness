/**
 * 数据库迁移启动脚本
 *
 * 职责：触发既有版本化数据库升级流程。
 * Java 研发阅读提示：操作前应确认数据库目标和权限，避免误对生产库执行迁移。
 *
 * 仅补充模块注释，原有代码和接口行为保持不变。
 */
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
