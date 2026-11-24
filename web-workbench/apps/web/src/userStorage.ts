/**
 * 用户相关浏览器存储
 *
 * 职责：集中管理客户端可保存的用户或会话相关数据。
 * Java 研发阅读提示：浏览器存储可能被用户修改，不应承载机密信息或授权事实。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
// All browser caches are inaccessible until bootstrap establishes an owner.
let scope:string|undefined;
export function configureUserStorage(ownerId:string,multiUser:boolean){scope=multiUser?'harness.user.'+encodeURIComponent(ownerId)+'.':'';}
export function resetUserStorage(){scope=undefined;}
export function scopedStorage(storage:()=>Storage):Storage{
  const keys=()=>{if(scope===undefined)return [];const raw=storage(),out:string[]=[];for(let i=0;i<raw.length;i++){const key=raw.key(i);if(key?.startsWith(scope))out.push(key.slice(scope.length));}return out;};
  return {
    get length(){return keys().length;},key:(i:number)=>keys()[i]??null,
    getItem:(key:string)=>scope===undefined?null:storage().getItem(scope+key),
    setItem:(key:string,value:string)=>{if(scope!==undefined)storage().setItem(scope+key,value);},
    removeItem:(key:string)=>{if(scope!==undefined)storage().removeItem(scope+key);},
    clear:()=>{for(const key of keys())storage().removeItem(scope+key);},
  };
}
export const localStorage=scopedStorage(()=>window.localStorage);
export const sessionStorage=scopedStorage(()=>window.sessionStorage);
