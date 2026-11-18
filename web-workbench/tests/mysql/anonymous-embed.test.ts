import {test} from 'node:test';
import assert from 'node:assert/strict';
import {randomBytes,randomUUID,createHash} from 'node:crypto';
import {fileURLToPath} from 'node:url';
import {configFromEnv,createPool} from '../../packages/storage/src/db.ts';
import {runMigrations} from '../../packages/storage/src/migrate.ts';
import {MysqlEmbedStore} from '../../packages/storage/src/embed.ts';
test('MySQL 8.4: anonymous slot leases and query claims persist, fence replicas, and require cleanup plus reset',{timeout:30000},async()=>{
  const config=configFromEnv(process.env);assert.equal(config.database,'harness_ci_test');
  await runMigrations(config,config.database,fileURLToPath(new URL('../../packages/storage/migrations',import.meta.url)));
  const pool=await createPool(config),locks=await createPool(config),store=new MysqlEmbedStore(pool,locks),replica=new MysqlEmbedStore(pool,locks);
  const slots=[0,1,2].map(i=>({ownerId:randomUUID(),serviceIdentity:'anonymous-ci-'+i+'-'+randomUUID()}));
  const next=()=>({id:randomUUID(),digest:createHash('sha256').update(randomBytes(32)).digest('hex'),expires:Date.now()+60000,parentOrigin:'https://workbench.example'});
  try{
    const results=await Promise.all(Array.from({length:8},(_,i)=>(i%2?store:replica).allocate(next(),slots)));
    const grants=results.filter(g=>g!==undefined);assert.equal(grants.length,3);assert.equal(new Set(grants.map(g=>g.ownerId)).size,3);
    const g=grants[0];assert.equal((await new MysqlEmbedStore(pool,locks).find(g.digest))?.id,g.id);
    const claims=await Promise.all([store.claimQuery(g.id,'a'.repeat(64)),replica.claimQuery(g.id,'a'.repeat(64))]);assert.equal(claims.filter(c=>c.claimed).length,1);
    assert.equal((await replica.claimQuery(g.id,'a'.repeat(64))).state,'RUNNING');await store.finishQuery(g.id,'安全日志证据');
    assert.equal((await replica.claimQuery(g.id,'a'.repeat(64))).evidence,'安全日志证据');
    await store.revoke(g.id);assert.equal(await replica.find(g.digest),undefined);assert.equal(await store.allocate(next(),slots),undefined);
    await store.cleaned(g.id);assert.equal(await replica.allocate(next(),slots),undefined,'cleanup alone must not release the writable environment');
    await store.recycled(g.id);const replacement=await replica.allocate(next(),slots);assert.ok(replacement);assert.equal(replacement.ownerId,g.ownerId);assert.notEqual(replacement.id,g.id);
    // API cleanup and allocation share the same cross-process advisory lock.
    let release!:()=>void,entered!:()=>void;const ready=new Promise<void>(resolve=>{entered=resolve;});
    const held=store.serialize(g.ownerId,async()=>{entered();await new Promise<void>(resolve=>{release=resolve;});});
    await ready;try{assert.equal(await replica.allocate(next(),[slots.find(s=>s.ownerId===g.ownerId)!]),undefined);}finally{release();await held;}
  }finally{
    for(const {ownerId}of slots){await pool.execute('DELETE FROM wh_embed_sessions WHERE owner_id=?',[ownerId]);await pool.execute('DELETE FROM wh_owners WHERE id=?',[ownerId]);}
    await pool.end();await locks.end();
  }
});
