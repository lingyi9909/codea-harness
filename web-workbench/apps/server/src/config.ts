import { resolve } from 'node:path';
import { homedir } from 'node:os';
import { mkdirSync } from 'node:fs';

export const apiPort=(()=>{
  const port=Number(process.env.HARNESS_HTTP_PORT??'4176');
  if(!Number.isInteger(port)||port<1||port>65535)throw new Error('HTTP_PORT_INVALID');
  return port;
})();
export const dataDir=resolve(process.env.HARNESS_DATA_DIR ??
  (process.platform==='win32' ? (process.env.LOCALAPPDATA??homedir())+'\\CodeHarness\\Workbench' :
    homedir()+'/.local/share/CodeHarness/Workbench'));
export const allowedRoots=(process.env.HARNESS_REPOSITORY_ROOTS??'').split(';').map(s=>s.trim()).filter(Boolean);
export const allowedOrigin=process.env.HARNESS_ALLOWED_ORIGIN??'';
export function ensureDataDir():void{mkdirSync(dataDir,{recursive:true,mode:0o700});}
