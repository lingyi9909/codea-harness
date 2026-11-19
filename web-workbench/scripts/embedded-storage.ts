import {parseArgs} from 'node:util';
import {existsSync} from 'node:fs';
import {SqliteEmbedStore} from '../packages/storage/src/sqlite-embed.ts';
import {defaultEmbeddedDatabase} from '../apps/server/src/embedded-start.ts';
async function main(){
  const {values}=parseArgs({options:{backup:{type:'string'},'list-locks':{type:'boolean'},'recover-lock':{type:'string'},nonce:{type:'string'},'confirm-stopped':{type:'boolean'}}});
  if([Boolean(values.backup),Boolean(values['list-locks']),Boolean(values['recover-lock'])].filter(Boolean).length!==1)throw Error('SPECIFY_BACKUP_LIST_OR_RECOVER');
  const path=process.env.HARNESS_SQLITE_PATH??defaultEmbeddedDatabase();if(!existsSync(path))throw Error('SQLITE_FILE_NOT_FOUND');
  const store=new SqliteEmbedStore(path);
  try{
    if(values.backup){await store.backup(values.backup);console.log('SQLite backup completed.');}
    else if(values['list-locks'])console.log(JSON.stringify(store.recoveryLocks(),null,2));
    else{store.recoverLock(values['recover-lock']!,values.nonce??'',values['confirm-stopped']===true);console.log('Confirmed stopped execution lock recovered.');}
  }finally{await store.close();}
}
main().catch(e=>{console.error(e instanceof Error?e.message:'STORAGE_OPERATION_FAILED');process.exitCode=2;});
