import {createHash} from 'node:crypto';
import {ExecutionRegistry} from '../../apps/server/src/execution-registry.ts';
import {mkdtemp,rm} from 'node:fs/promises';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {embedFixture} from './embed-fixture.ts';
import {SqliteEmbedStore} from '../../packages/storage/src/sqlite-embed.ts';
import {EmbeddedAuth} from '../../apps/server/src/embed-auth.ts';
import {createEmbeddedApp} from '../../apps/server/src/embedded-app.ts';
import type {InstitutionQueryAdapter} from '../../apps/server/src/institution-query.ts';
export async function sqliteEmbedFixture(origin='https://workbench.example',parentOrigin='https://host.example',hold=true,adapter?:InstitutionQueryAdapter,allowInstitutionShell=false){
  // Reuse the scripted native peer only. The standalone app receives no MySQL pool.
  const native=await embedFixture(origin,parentOrigin,hold,{anonymous:true,sharedWorkspace:true});await native.app.close();
  const data=await mkdtemp(join(tmpdir(),'sqlite-embed-')),path=join(data,'embedded.sqlite');
  const identity='single:'+createHash('sha256').update(native.workspace).digest('hex');
  native.config.anonymous={parentOrigins:[parentOrigin],executionOwners:[],singleService:{identity}};
  const execution=new ExecutionRegistry([],async()=>native.execution.run(native.owners[0],()=>native.execution.client()),{workspace:native.workspace,singleServiceIdentity:identity,allowInstitutionShell});
  let store=new SqliteEmbedStore(path,identity);
  const makeApp=()=>createEmbeddedApp({auth:new EmbeddedAuth(native.config,store),execution,workspace:native.workspace,bindings:store.bindings,ensureOwner:(id,subject)=>store.ensureOwner(id,subject),storageCheck:()=>store.check(),institutionAdapter:adapter});
  let app=await makeApp();
  return {...native,path,execution,get app(){return app;},get store(){return store;},restart:async()=>{await app.close();await store.close();store=new SqliteEmbedStore(path,identity);app=await makeApp();},close:async()=>{await app.close();await store.close();await native.close();await rm(data,{recursive:true,force:true});}};
}
