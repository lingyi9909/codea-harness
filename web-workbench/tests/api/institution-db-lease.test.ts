import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {InstitutionTaskStore} from '../../apps/server/src/institution-task-store.ts';

test('second workbench instance cannot reset another live institution task in shared SQLite',async()=>{
  const root=await mkdtemp(join(tmpdir(),'institution-db-lease-'));
  const a=join(root,'project-a'),b=join(root,'project-b'),path=join(root,'shared.sqlite');
  const {mkdir}=await import('node:fs/promises');
  await Promise.all([mkdir(a),mkdir(b)]);
  const first=new InstitutionTaskStore(path,a);
  try{
    const record=first.create('platform-b','hash','request-1','2026-11-25','MSG-001','customer-001');
    assert.equal(first.take()?.id,record.row.id);
    assert.equal(first.get('platform-b','hash',record.row.id)?.status,'RUNNING');

    assert.throws(()=>new InstitutionTaskStore(path,a),/INSTITUTION_API_DB_ALREADY_IN_USE/,
      'even a different port on the same workspace must not open a second worker');
    assert.throws(()=>new InstitutionTaskStore(path,b),/INSTITUTION_API_DB_WORKSPACE_CONFLICT/,
      'other workspace must use a different institution API database');
    assert.equal(first.get('platform-b','hash',record.row.id)?.status,'RUNNING',
      'second instance must never turn a live first-instance task into UNKNOWN');
  }finally{first.close();}
  const reopened=new InstitutionTaskStore(path,a);
  try{
    const row=reopened.listAdmin(undefined,undefined,0).items[0];
    assert.equal(row.status,'UNKNOWN','only actual same-workspace restart marks unresolved work UNKNOWN');
    assert.equal(row.errorCode,'PROCESS_RESTARTED');
  }finally{reopened.close();}
  assert.throws(()=>new InstitutionTaskStore(path,b),/INSTITUTION_API_DB_WORKSPACE_CONFLICT/,
    'cross-workspace isolation also applies after first instance stopped');
  await rm(root,{recursive:true,force:true});
});

test('dead owner PID lease can be reclaimed without manual database deletion',async()=>{
  const root=await mkdtemp(join(tmpdir(),'institution-crash-lease-'));
  const path=join(root,'tasks.sqlite');
  const workspace=join(root,'project');
  const {mkdir}=await import('node:fs/promises');await mkdir(workspace);
  const first=new InstitutionTaskStore(path,workspace);
  first.close();
  const {DatabaseSync}=await import('node:sqlite');
  const db=new DatabaseSync(path);
  db.prepare("INSERT INTO institution_api_settings(key,value) VALUES('active_instance',?)")
    .run(JSON.stringify({pid:987654321,token:'crashed-process'}));
  db.close();
  const restarted=new InstitutionTaskStore(path,workspace);
  try{assert.equal(restarted.queueCounts().running,0);}finally{restarted.close();}
  await rm(root,{recursive:true,force:true});
});
