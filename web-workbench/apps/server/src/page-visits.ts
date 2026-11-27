/**
 * 页面访问统计
 *
 * 职责：聚合页面访问次数和访客信息以供管理页展示。
 * Java 研发阅读提示：统计数据不能替代审计凭证；留意用户标识与保留周期。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
import {DatabaseSync} from 'node:sqlite';
import {createHash} from 'node:crypto';
import {existsSync,lstatSync,mkdirSync,chmodSync} from 'node:fs';
import {dirname,isAbsolute,join} from 'node:path';
import {homedir} from 'node:os';
import {AuthProblem} from './auth.ts';

export const PAGE_IDS=['home','review','logs','message','data','institution-message','embedded-message','component-lab','config','analytics'] as const;
export type PageId=typeof PAGE_IDS[number];
export const pageNames:Record<PageId,string>={
  home:'对话首页',review:'代码审查',logs:'公管问题排查',message:'报文问题排查',
  data:'公管数据分析','institution-message':'机构报文排查',
  'embedded-message':'外部嵌入报文排查','component-lab':'组件测试',config:'场景配置',analytics:'页面访问统计',
};
export function defaultPageVisitPath(){
  return join(process.env.LOCALAPPDATA??homedir(),'.smart-workbench','state','page-visits.sqlite');
}
export function validPageId(value:unknown):value is PageId{
  return typeof value==='string'&&PAGE_IDS.includes(value as PageId);
}
export class PageVisitStore{
  private readonly db:DatabaseSync;
  constructor(path=defaultPageVisitPath()){
    if(!isAbsolute(path)||path.startsWith('\\\\')||path.startsWith('//'))throw Error('VISITS_PATH_INVALID');
    mkdirSync(dirname(path),{recursive:true,mode:0o700});
    if(existsSync(path)&&lstatSync(path).isSymbolicLink())throw Error('VISITS_SYMLINK_FORBIDDEN');
    this.db=new DatabaseSync(path);
    chmodSync(path,0o600);
    this.db.exec(`PRAGMA busy_timeout=3000;PRAGMA journal_mode=WAL;
      CREATE TABLE IF NOT EXISTS page_visits (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        page TEXT NOT NULL,
        visitor TEXT NOT NULL,
        at INTEGER NOT NULL,
        day TEXT NOT NULL
      );
      CREATE INDEX IF NOT EXISTS ix_visits_page_day ON page_visits(page,day);
      CREATE INDEX IF NOT EXISTS ix_visits_visitor_at ON page_visits(visitor,page,at);
      CREATE TABLE IF NOT EXISTS user_access_log (
        id TEXT PRIMARY KEY, owner_id TEXT NOT NULL, profile_key TEXT NOT NULL,
        name TEXT NOT NULL, phone_mask TEXT NOT NULL, ip TEXT NOT NULL,
        scope TEXT NOT NULL, logged_in_at INTEGER NOT NULL,
        last_active_at INTEGER NOT NULL, logged_out_at INTEGER
      );
      CREATE INDEX IF NOT EXISTS ix_user_access_login_time ON user_access_log(logged_in_at DESC);
      CREATE INDEX IF NOT EXISTS ix_user_access_profile ON user_access_log(profile_key,logged_in_at DESC);`);
  }
  record(page:PageId,visitorId:string,now=Date.now()){
    if(!/^[a-f0-9-]{36}$/i.test(visitorId))throw new AuthProblem(400,'VISITOR_INVALID');
    const visitor=createHash('sha256').update(visitorId).digest('hex');
    const day=new Date(now).toISOString().slice(0,10);
    // Strict Mode double-mount and fast reloads should not count as two views.
    const previous=this.db.prepare('SELECT at FROM page_visits WHERE visitor=? AND page=? ORDER BY at DESC LIMIT 1').get(visitor,page);
    if(!previous||now-Number(previous.at)>=2000)
      this.db.prepare('INSERT INTO page_visits(page,visitor,at,day) VALUES (?,?,?,?)').run(page,visitor,now,day);
    // Only the last 90 days of aggregate event evidence are retained.
    this.db.prepare('DELETE FROM page_visits WHERE at<?').run(now-90*86400000);
  }
  summary(days=7,now=Date.now()){
    const start=new Date(now-(days-1)*86400000).toISOString().slice(0,10);
    const stats=this.db.prepare(`SELECT page,COUNT(*) AS views,
      COUNT(DISTINCT visitor) AS visitors,MAX(at) AS latest
      FROM page_visits WHERE day>=? GROUP BY page`).all(start);
    const daily=this.db.prepare(`SELECT page,day,COUNT(*) AS views,COUNT(DISTINCT visitor) AS visitors
      FROM page_visits WHERE day>=? GROUP BY page,day ORDER BY day DESC,page`).all(start);
    return {days,rows:PAGE_IDS.map(page=>{
      const row=stats.find(r=>r.page===page);
      return {page,name:pageNames[page],views:Number(row?.views??0),visitors:Number(row?.visitors??0),lastAt:row?.latest?Number(row.latest):null};
    }),daily:daily.map(r=>({page:String(r.page),day:String(r.day),views:Number(r.views),visitors:Number(r.visitors)}))};
  }

  /**
   * Log successful local/LAN logins only. auditId identifies a Cookie lifecycle,
   * and INSERT OR IGNORE avoids duplicate audit rows when reusing a login Cookie.
   * Phone number is masked before writing to disk.
   */
  recordLogin(record:{
    auditId:string;ownerId:string;profileKey:string;name:string;phone:string;
    ip:string;scope:'local'|'lan';
  },now=Date.now()){
    const masked=record.phone?record.phone.slice(0,3)+'****'+record.phone.slice(-4):'';
    this.db.prepare('INSERT OR IGNORE INTO user_access_log '+
      '(id,owner_id,profile_key,name,phone_mask,ip,scope,logged_in_at,last_active_at) '+
      'VALUES (?,?,?,?,?,?,?,?,?)').run(record.auditId,record.ownerId,record.profileKey,
        record.name,masked,record.ip,record.scope,now,now);
    this.db.prepare('DELETE FROM user_access_log WHERE logged_in_at<?').run(now-90*86400000);
  }
  /** Record API activity at most once per 30 seconds per active login Cookie. */
  touch(auditId:string,now=Date.now()){
    this.db.prepare('UPDATE user_access_log SET last_active_at=? WHERE id=? '+
      'AND logged_out_at IS NULL AND last_active_at<=?').run(now,auditId,now-30000);
  }
  recordLogout(auditId:string,now=Date.now()){
    this.db.prepare('UPDATE user_access_log SET logged_out_at=?,last_active_at=? '+
      'WHERE id=? AND logged_out_at IS NULL').run(now,now,auditId);
  }
  /** Only the localhost admin HTTP route may expose these saved login records. */
  users(days:7|30,offset=0,query='',now=Date.now()){
    if(!Number.isSafeInteger(offset)||offset<0||offset>1000000||query.length>128)
      throw new AuthProblem(400,'USER_VISITS_FILTER_INVALID');
    // Retention applies to reads too, even when no fresh login has occurred.
    this.db.prepare('DELETE FROM user_access_log WHERE logged_in_at<?').run(now-90*86400000);
    const search=query.trim(),start=now-days*86400000;
    const where=search?' WHERE logged_in_at>=? AND (name LIKE ? OR phone_mask LIKE ? OR ip LIKE ?)':
      ' WHERE logged_in_at>=?';
    const params:(string|number)[]=[start];
    if(search){const pattern='%'+search+'%';params.push(pattern,pattern,pattern);}
    const total=this.db.prepare('SELECT COUNT(*) AS n FROM user_access_log'+where)
      .get(...params) as {n:number};
    const distinct=this.db.prepare('SELECT COUNT(DISTINCT profile_key) AS n FROM user_access_log'+where)
      .get(...params) as {n:number};
    const rows=this.db.prepare('SELECT id,name,phone_mask,ip,scope,logged_in_at,last_active_at,logged_out_at '+
      'FROM user_access_log'+where+' ORDER BY logged_in_at DESC,id DESC LIMIT 20 OFFSET ?')
      .all(...params,offset);
    return {days,offset,limit:20,total:total.n,distinctUsers:distinct.n,items:rows.map(row=>({
      id:String(row.id),name:String(row.name),phoneMask:String(row.phone_mask),
      ip:String(row.ip),scope:String(row.scope),
      loginAt:new Date(Number(row.logged_in_at)).toISOString(),
      lastActiveAt:new Date(Number(row.last_active_at)).toISOString(),
      logoutAt:row.logged_out_at==null?null:new Date(Number(row.logged_out_at)).toISOString(),
    }))};
  }
  close(){this.db.close();}
}
