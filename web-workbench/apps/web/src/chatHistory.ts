import type {ChatHistoryPage} from '../../../packages/contracts/src/chat.ts';

export type HistoryRow=ChatHistoryPage['items'][number];
export type SceneKey='harness.chat'|'harness.logs';
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
