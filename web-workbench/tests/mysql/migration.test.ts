import { describe,it } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { migrationStatements } from '../../packages/storage/src/migrate.ts';
describe('MySQL 8.4 migration static contract; not a real DB test',()=>{
  it('contains all 14 design business tables, no DROP, and one execution slot seed',async()=>{
    const sql=await readFile(new URL('../../packages/storage/migrations/001_init.sql',import.meta.url),'utf8');
    const statements=migrationStatements(sql);
    const tables=statements.filter(s=>s.startsWith('CREATE TABLE')).map(s=>s.match(/^CREATE TABLE (\w+)/)?.[1]);
    assert.equal(tables.length,14);
    assert.equal(new Set(tables).size,14);
    assert.ok(statements.some(s=>s.includes("VALUES ('local-default'")));
    assert.ok(statements.some(s=>s.includes('uq_active_task')));
    assert.ok(statements.some(s=>s.includes('wh_outbox')));
    assert.throws(()=>migrationStatements('DROP TABLE wh_tasks;'),/DROP_MIGRATION_NOT_ALLOWED/);
  });
});
