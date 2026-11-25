/**
 * 会话历史存储
 *
 * 职责：维护或整理浏览器侧的历史会话索引与持久化信息。
 * Java 研发阅读提示：本地缓存与服务器实际任务记录可能不同步，不应混用作为权限凭据。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
import {sessionStorage} from './userStorage.ts';
import type {ChatHistoryPage} from '../../../packages/contracts/src/chat.ts';

export type HistoryRow=ChatHistoryPage['items'][number];
export type SceneKey='harness.chat'|'harness.logs'|'harness.message'|'harness.institution-message'|'harness.data';
const prefix=(scene:SceneKey,directory:string)=>scene+'.history.'+JSON.stringify(directory)+'.';
const selection=(scene:SceneKey,directory:string)=>scene+'.selected.'+JSON.stringify(directory);

export function savedChat(durable:Storage,tab:Storage,scene:SceneKey,directory:string){
  const key=selection(scene,directory);
  return tab.getItem(key)??durable.getItem(key)??(!tab.getItem(scene+'.scoped')?tab.getItem(scene):null)??'';
}
export function activateChat(durable:Storage,tab:Storage,scene:SceneKey,directory:string,id:string){
  const key=selection(scene,directory);
  tab.setItem(key,id);
  durable.setItem(key,id);
  tab.setItem(scene,id);
  tab.setItem(scene+'.scoped','1');
}
export function rememberChat(storage:Storage,scene:SceneKey,directory:string,row:HistoryRow){
  storage.setItem(prefix(scene,directory)+row.id,JSON.stringify(row));
}
export function forgetChat(durable:Storage,tab:Storage,scene:SceneKey,directory:string,id:string){
  durable.removeItem(prefix(scene,directory)+id);
  const current=selection(scene,directory);
  if(durable.getItem(current)===id)durable.removeItem(current);
  if(tab.getItem(current)===id)tab.removeItem(current);
  if(tab.getItem(scene)===id)tab.setItem(scene,'');
  tab.removeItem(draftKey(scene,directory,id));
  tab.removeItem(scene+'.range.'+id);
  tab.removeItem(scene+'.filters.'+id);
}
export function uniqueChatTitle(title:string,history:HistoryRow[]){
  const used=new Set(history.map(item=>item.title.trim()));
  if(!used.has(title))return title;
  for(let number=2;number<10000;number++){
    const suffix=' · '+number;
    const available=Array.from(title).slice(0,120-suffix.length).join('')+suffix;
    if(!used.has(available))return available;
  }
  return title;
}
export function readHistory(storage:Storage,scene:SceneKey,directory:string){
  const start=prefix(scene,directory),rows:HistoryRow[]=[];
  for(let i=0;i<storage.length;i++){
    const key=storage.key(i);
    if(!key?.startsWith(start))continue;
    try{
      const row=JSON.parse(storage.getItem(key)??'null');
      if(row&&key===start+row.id&&typeof row.title==='string'&&Number.isFinite(row.updated))rows.push(row);
    }catch{/* Ignore incomplete browser storage entries. */}
  }
  return rows.sort((a,b)=>b.updated-a.updated);
}
export function draftKey(scene:SceneKey,directory:string,id:string){
  return scene+'.composer.'+JSON.stringify([directory,id]);
}
export function readDraft(key:string):{text:string;files:string}{
  try{
    const draft=JSON.parse(sessionStorage.getItem(key)??'null');
    if(typeof draft?.text==='string'&&typeof draft.files==='string')return draft;
  }catch{/* The composer is empty when there is no usable saved draft. */}
  return {text:'',files:''};
}

export function displayChatTitle(native?:string,cached?:string){
  const title=native?.trim();
  return title&&!/^(?:Code Harness|智能工作台) · (直接对话|日志排查|问题排查|专项场景)$/.test(title)?title:cached??'';
}

export function summarizeFirstInput(text='',logInspection=false){
  let identitySuffix='';
  if(logInspection){
    const criteria=text.split(/^导入日志：|^请用当前项目可用的日志工具/m,1)[0];
    const question=criteria.match(/^排查问题：([\s\S]*)/m)?.[1];
    const keyword=criteria.match(/^关键字：(.+)$/m)?.[1],trace=criteria.match(/^Trace ID：(.+)$/m)?.[1];
    const msgSn=criteria.match(/^msgSn：(.+)$/m)?.[1];
    const identity=msgSn?'msgSn '+msgSn:trace?'Trace ID '+trace:keyword?'关键词 '+keyword:'';
    // The first user question is followed by workbench execution instructions.
    // Never use those instructions as the conversation title.
    if(question){text=question.split(/\r?\n/,1)[0].trim();identitySuffix=identity?' · '+identity:'';}
    else if(identity)text='排查 · '+[msgSn?'msgSn '+msgSn:'',trace?'Trace ID '+trace:'',keyword?'关键词 '+keyword:''].filter(Boolean).join(' · ');
  }
  const clean=text.trim().replace(/^(?:你好[，,！!。\s]+|您好[，,！!。\s]+|(?:hi|hello)[!,\s]+)+/i,'')
    .replace(/((?:请(?:帮我)?|帮我|please))\s*\n\s*/gi,'$1 ');
  const clauses=clean.split(/\n+|[。！？!?]+(?:\s|$)?|\.\s+|\.$/).map(line=>line.trim()).filter(Boolean);
  const request=clauses.find(line=>/^(?:请|帮我|麻烦|希望|需求[：:]|目标[：:]|please\b|can you\b|could you\b)/i.test(line));
  const main=request??clauses[0]??text.trim();
  const title=main.replace(/^(?:请(?:问|你)?(?:帮我|协助我)?|麻烦(?:你)?(?:帮我)?|帮我|需求[：:]|目标[：:])\s*/,'')
    .replace(/^(?:(?:can|could) you (?:please )?|please )([a-z])/i,(_,first:string)=>first.toUpperCase())
    .replace(/\s+/g,' ').trim()||main;
  const points=Array.from(title+identitySuffix);
  return points.length>48?points.slice(0,47).join('').trimEnd()+'…':points.join('');
}
