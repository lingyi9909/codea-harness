import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {randomUUID} from 'node:crypto';
import {DatabaseSync} from 'node:sqlite';
import {PageVisitStore} from '../../apps/server/src/page-visits.ts';
import {createApp} from '../../apps/server/src/app.ts';
import {LocalAuth} from '../../apps/server/src/auth.ts';

test('login audit persists, masks phone, tracks activity/logout and paginates',async()=>{
  const root=await mkdtemp(join(tmpdir(),'user-access-audit-'));
  const path=join(root,'visits.sqlite'),store=new PageVisitStore(path);
  const now=Date.now();
  try{
    for(let i=0;i<25;i++){
      store.recordLogin({auditId:'audit-'+i,ownerId:'lan:'+i,profileKey:'key-'+i,
        name:i===0?'张三':'用户'+i,phone:'13812345678',ip:'192.168.1.'+(i+1),scope:'lan'},now+i);
    }
    assert.equal(store.users(7,0,'',now+50).total,25);
    assert.equal(store.users(7,0,'',now+50).items.length,20);
    assert.equal(store.users(7,20,'',now+50).items.length,5);
    assert.equal(store.users(7,0,'张三',now+50).total,1);
    assert.equal(store.users(7,0,'192.168.1.2',now+50).total,1);
    const first=store.users(7,0,'张三',now+50).items[0];
    assert.equal(first.phoneMask,'138****5678');
    assert.equal(first.logoutAt,null);
    store.touch('audit-0',now+60000);
    store.recordLogout('audit-0',now+70000);
    assert.equal(store.users(7,0,'张三',now+70001).items[0].lastActiveAt,
      new Date(now+70000).toISOString());
    assert.equal(store.users(7,0,'张三',now+70001).items[0].logoutAt,
      new Date(now+70000).toISOString());
    store.recordLogin({auditId:'audit-0',ownerId:'lan:0',profileKey:'key-0',
      name:'张三',phone:'13812345678',ip:'192.168.1.1',scope:'lan'},now+80000);
    assert.equal(store.users(7,0,'张三',now+80001).total,1,'same Cookie is idempotent');
    assert.throws(()=>store.users(7,-1),/USER_VISITS_FILTER_INVALID/);
  }finally{store.close();}
  const db=new DatabaseSync(path);
  assert.equal((db.prepare('SELECT phone_mask FROM user_access_log WHERE id=?').get('audit-0')
    as {phone_mask:string}).phone_mask,'138****5678');
  db.close();
  const reopened=new PageVisitStore(path);
  assert.equal(reopened.users(7,0,'',now+100000).total,25,'audit survives process restart');
  reopened.close();
  await rm(root,{recursive:true,force:true});
});

test('only localhost config editor can read user access audit; LAN can log in but cannot inspect peers',async t=>{
  const root=await mkdtemp(join(tmpdir(),'user-visits-api-'));
  const visits=new PageVisitStore(join(root,'page-visits.sqlite'));
  const auth=new LocalAuth('unused',4176,undefined,undefined,{origin:'http://192.168.1.66:4176'});
  const app=await createApp({localOnly:true,workspace:root,auth,pageVisits:visits});
  t.after(async()=>{await app.close();visits.close();await rm(root,{recursive:true,force:true});});
  const lan={host:'192.168.1.66:4176',origin:'http://192.168.1.66:4176'};
  const local={host:'127.0.0.1:4176',origin:'http://127.0.0.1:4176'};
  const a=await app.inject({url:'/api/v1/auth/local-session',method:'POST',
    remoteAddress:'192.168.1.51',headers:lan,payload:{name:'李四',phone:'13800000051'}});
  assert.equal(a.statusCode,200,a.body);
  const remoteCookie=String(a.headers['set-cookie']).split(';')[0];
  const denied=await app.inject({url:'/api/v1/user-visits',remoteAddress:'192.168.1.51',
    headers:{...lan,cookie:remoteCookie}});
  assert.equal(denied.statusCode,403,denied.body);
  const admin=await app.inject({url:'/api/v1/auth/local-session',method:'POST',
    remoteAddress:'127.0.0.1',headers:local,payload:{name:'zhangzhanhui',phone:'13800000000'}});
  assert.equal(admin.statusCode,200,admin.body);
  const cookie=String(admin.headers['set-cookie']).split(';')[0];
  const allowed=await app.inject({url:'/api/v1/user-visits?days=7&offset=0',
    remoteAddress:'127.0.0.1',headers:{...local,cookie}});
  assert.equal(allowed.statusCode,200,allowed.body);
  const body=allowed.json().data;
  assert.equal(body.total,2);
  assert.equal(body.distinctUsers,2);
  assert.ok(body.items.some((row:{name:string;phoneMask:string;ip:string})=>
    row.name==='李四'&&row.phoneMask==='138****0051'&&row.ip==='192.168.1.51'));
  assert.equal(JSON.stringify(body).includes('13800000051'),false,'never return raw phone');
  const badLimit=await app.inject({url:'/api/v1/user-visits?days=365',
    remoteAddress:'127.0.0.1',headers:{...local,cookie}});
  assert.equal(badLimit.statusCode,400);
  const logout=await app.inject({method:'DELETE',url:'/api/v1/auth/session',
    remoteAddress:'127.0.0.1',headers:{...local,cookie,'x-csrf-token':admin.json().data.csrfToken}});
  assert.equal(logout.statusCode,204,logout.body);
  assert.equal(visits.users(7,0,'zhangzhanhui').items[0].logoutAt!==null,true);
});
