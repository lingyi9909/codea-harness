/**
 * 服务端运行配置
 *
 * 职责：声明供启动与运行流程读取的配置结构。
 * Java 研发阅读提示：配置项与安全约束不应由页面显示状态决定。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
import { resolve } from 'node:path';
import { configFromEnv } from '../../../packages/storage/src/db.ts';

export function runtimeConfig(env:NodeJS.ProcessEnv=process.env){
  const port=Number(env.HARNESS_HTTP_PORT??'4176');
  if(!Number.isInteger(port)||port<1024||port>65535)throw new Error('INVALID_LOOPBACK_PORT');
  return {
    port,
    dataDir:resolve(env.HARNESS_DATA_DIR??'.workbench-local'),
    mysql:configFromEnv(env),
  };
}
