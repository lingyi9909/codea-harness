import { resolve } from 'node:path';
import { configFromEnv } from '../../../../packages/storage/src/db.ts';

export function runtimeConfig(env:NodeJS.ProcessEnv=process.env){
  const port=Number(env.HARNESS_HTTP_PORT??'4176');
  if(!Number.isInteger(port)||port<1024||port>65535)throw new Error('INVALID_LOOPBACK_PORT');
  return {
    port,
    dataDir:resolve(env.HARNESS_DATA_DIR??'.workbench-local'),
    mysql:configFromEnv(env),
  };
}
