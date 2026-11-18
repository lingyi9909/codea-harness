import {test} from 'node:test';
import assert from 'node:assert/strict';
import {randomBytes,randomUUID,createHash} from 'node:crypto';
import {fileURLToPath} from 'node:url';
import {configFromEnv,createPool} from '../../packages/storage/src/db.ts';
import {runMigrations} from '../../packages/storage/src/migrate.ts';
import {MysqlEmbedStore} from '../../packages/storage/src/embed.ts';
test('MySQL 8.4: atomic ticket consumption, bound grant, revocation, cleanup and independent submission locks',async()=>{
  const config=configFromEnv(process.env);assert.equal(config.database,'harness_ci_test');
  await runMigrations(config,config.database,fileURLToPath(new URL('../../packages/storage/migrations',import.meta.url)));
  const pool=await createPool(config),locks=await createPool(config),store=new MysqlEmbedStore(pool,locks),owner=randomUUID();
  const digest=(s:string)=>createHash('sha256').update(s).digest('hex');
  let release=()=>{};let first:Promise<string>|undefined;
  try{
    await pool.execute("INSERT INTO wh_owners(id,subject,display_name,created_at,updated_at) VALUES (?,?,'Embed CI',UTC_TIMESTAMP(3),UTC_TIMESTAMP(3))",[owner,owner]);
    const ticket=randomBytes(32).toString('base64url');
    await store.ticket({digest:digest(ticket),ownerId:owner,clientId:'ci',parentOrigin:'https://host.example',expires:Date.now()+60000});
    const rawTokens=[randomBytes(32).toString('base64url'),randomBytes(32).toString('base64url')];
    const grants=await Promise.all(rawTokens.map(raw=>store.exchange(digest(ticket),'https://host.example',{id:randomUUID(),digest:digest(raw),expires:Date.now()+60000})));
    assert.equal(grants.filter(Boolean).length,1);const grant=grants.find(Boolean)!;
    assert.deepEqual((await Promise.all([store.reserve(grant.id),store.reserve(grant.id)])).sort(),[false,true]);
    await store.bind(grant.id,'native-ci','ci-service');assert.equal((await store.find(grant.digest))?.nativeId,'native-ci');
    await assert.rejects(store.bind(grant.id,'other-native','ci-service'));
    await store.revoke(grant.id);assert.equal(await store.find(grant.digest),undefined);
    assert.ok((await store.cleanupCandidates()).some(g=>g.id===grant.id));await store.cleaned(grant.id);assert.ok(!(await store.cleanupCandidates()).some(g=>g.id===grant.id));
    let entered!:()=>void;const enteredPromise=new Promise<void>(resolve=>{entered=resolve;});
    first=store.serialize(owner,async()=>{entered();await new Promise<void>(resolve=>{release=resolve;});return 'first';});
    await enteredPromise;
    await assert.rejects(new MysqlEmbedStore(pool,locks).serialize(owner,async()=>{throw Error('should not execute');}),/EMBED_EXECUTOR_BUSY/);
    // Normal queries remain available while advisory lock connections are held.
    await pool.query('SELECT 1');release();assert.equal(await first,'first');
    assert.equal(await store.serialize(owner,async()=>'recovered'),'recovered');
  }finally{
    release();await first?.catch(()=>{});
    await pool.execute('DELETE FROM wh_embed_sessions WHERE owner_id=?',[owner]);await pool.execute('DELETE FROM wh_embed_tickets WHERE owner_id=?',[owner]);await pool.execute('DELETE FROM wh_owners WHERE id=?',[owner]);await pool.end();await locks.end();
  }
});
