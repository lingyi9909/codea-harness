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
      CREATE INDEX IF NOT EXISTS ix_visits_visitor_at ON page_visits(visitor,page,at);`);
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
  close(){this.db.close();}
}
