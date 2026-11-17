import {test} from 'node:test';
import assert from 'node:assert/strict';
import {randomUUID} from 'node:crypto';
import {fileURLToPath} from 'node:url';
import type {FastifyRequest,FastifyReply} from 'fastify';
import {createPool,configFromEnv} from '../../packages/storage/src/db.ts';
import {runMigrations} from '../../packages/storage/src/migrate.ts';
import {MysqlIdentityStore} from '../../packages/storage/src/identity.ts';
import {MysqlChatBindings} from '../../packages/storage/src/chat-bindings.ts';
import {MultiUserAuth} from '../../apps/server/src/multi-user-auth.ts';
test('MySQL 8.4: migrations, concurrent unique identities, persisted auth and binding ownership',async()=>{
  const config=configFromEnv(process.env);assert.equal(config.database,'harness_ci_test','DEDICATED_CI_DATABASE_REQUIRED');
  const directory=fileURLToPath(new URL('../../packages/storage/migrations',import.meta.url));
  await runMigrations(config,config.database,directory);await runMigrations(config,config.database,directory);
  const pool=await createPool(config),store=new MysqlIdentityStore(pool),bindings=new MysqlChatBindings(pool),ids=new Set<string>();
  const suffix=randomUUID().replace(/\D/g,'').padEnd(12,'0').slice(0,12),phone='+9'+suffix;
  try{
    const owners=await Promise.all([0,1,2].map(()=>store.identify('Concurrent user',phone)));owners.forEach(o=>ids.add(o.id));assert.equal(ids.size,1);
    const others=await Promise.all([0,1].map(i=>store.identify('Other '+i,'+8'+i+suffix)));others.forEach(o=>ids.add(o.id));assert.equal(ids.size,3);
    const req={ip:'127.0.0.1',headers:{host:'workbench.example',origin:'https://workbench.example'}} as unknown as FastifyRequest;
    let cookie='';const reply={header:(_key:string,value:string)=>{cookie=value.split(';')[0];}} as FastifyReply;
    const auth=new MultiUserAuth(store,'https://workbench.example'),session=await auth.login(req,reply,{username:'Concurrent user',phone});
    const restoredReq={...req,headers:{...req.headers,cookie,'x-csrf-token':session.csrf}} as FastifyRequest;
    assert.equal((await new MultiUserAuth(new MysqlIdentityStore(pool),'https://workbench.example').session(restoredReq)).ownerId,owners[0].id);
    const binding={ownerId:owners[0].id,serviceIdentity:'mysql-fixture-'+suffix,id:randomUUID(),scenario:'direct-chat',directory:'/isolated/fixture'};
    await bindings.create(binding);assert.deepEqual(await bindings.get(binding.serviceIdentity,binding.id,binding.ownerId),binding);
    assert.equal(await bindings.get(binding.serviceIdentity,binding.id,others[0].id),undefined);
    await bindings.remove(binding.serviceIdentity,binding.id,binding.ownerId);assert.equal(await bindings.get(binding.serviceIdentity,binding.id,binding.ownerId),undefined);
    await auth.logout(restoredReq,reply);await assert.rejects(auth.session(restoredReq),/AUTH_REQUIRED/);
  }finally{
    for(const owner of ids){await pool.execute('DELETE FROM wh_chat_session_bindings WHERE owner_id=?',[owner]);await pool.execute('DELETE FROM wh_web_sessions WHERE owner_id=?',[owner]);await pool.execute('DELETE FROM wh_user_profiles WHERE owner_id=?',[owner]);await pool.execute('DELETE FROM wh_owners WHERE id=?',[owner]);}
    await pool.end();
  }
});
