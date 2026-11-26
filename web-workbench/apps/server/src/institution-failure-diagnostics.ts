/**
 * 机构任务失败诊断：管理员可查看原始错误对象 + 分类建议。
 *
 * 维护人员需要原生 error.message/stack/SQL 返回错误等原文来排障，
 * 只采集失败时原生提供的 error 对象（不主动收集 Prompt/整段模型思考）。
 * 原始错误可能包含凭证与业务数据，故仅保存在七日留存的本地任务库，
 * 通过原有仅本机管理员可访问的诊断 API 展示；B 对外 API 绝不返回。
 * 本模块只是记录观察结果，不改变原本的任务成功、失败或重试语义。
 */
export type FailureCategory='TIMEOUT'|'NETWORK'|'AUTH'|'PERMISSION'|'MCP'|'SQL'|'TOOL_MISSING'|'EXECUTION'|'UNKNOWN';
export type FailureOrigin='native-event'|'session-snapshot'|'worker';
export type SafeFailure={
  version:1;
  toolName:string|null;
  toolCallId:string|null;
  category:FailureCategory;
  errorCode:string|null;
  nativeType:string|null;
  reason:string;
  suggestion:string;
  origin:FailureOrigin;
  /** Original native error, including message/stack/data when present. Never redacted. */
  rawError?:string|null;
  /** Large native errors are size-capped for database stability, not sanitized. */
  rawTruncated?:boolean;
};
const asObject=(value:unknown):Record<string,unknown>|undefined=>
  value!==null&&typeof value==='object'&&!Array.isArray(value)?value as Record<string,unknown>:undefined;
const safeIdentifier=(value:unknown,max=80):string|null=>
  typeof value==='string'&&value.length<=max&&/^[a-zA-Z0-9_.:-]+$/.test(value)?value:null;
const safeType=(value:unknown)=>safeIdentifier(value,64);
const guidelines:Record<FailureCategory,{reason:string;suggestion:string}>={
  TIMEOUT:{reason:'工具调用或依赖服务响应超时',suggestion:'检查 OpenCode/DBHub/MCP 连接和超时配置，并核对下游服务耗时。'},
  NETWORK:{reason:'工具与依赖服务之间的连接异常',suggestion:'检查服务地址、网络连通性、DNS、端口和依赖服务状态。'},
  AUTH:{reason:'依赖接口返回身份验证或授权错误',suggestion:'核对服务账号、访问令牌有效期与下游接口的访问权限。'},
  PERMISSION:{reason:'执行权限不足或操作被策略拒绝',suggestion:'核对工作台、OpenCode、MCP 及执行目录的权限规则。'},
  MCP:{reason:'MCP 服务或工具适配器发生异常',suggestion:'检查 MCP 服务进程、注册配置、工具名称及其后端连接状态。'},
  SQL:{reason:'数据库或 SQL 查询执行出现异常',suggestion:'用 DBHub 只读检查数据库连接、表结构、字段、SQL 方言和查询超时。'},
  TOOL_MISSING:{reason:'工具、命令或可执行文件未找到',suggestion:'确认 Skill/MCP 工具配置与可执行文件是否已安装、加载。'},
  EXECUTION:{reason:'原生执行或工具返回异常',suggestion:'结合工具调用 ID、执行时间和 OpenCode 原生日志继续核查。'},
  UNKNOWN:{reason:'原生工具报告失败，但未提供可识别的错误分类',suggestion:'按调用 ID 查询 OpenCode 原生会话与对应 MCP/Shell 服务日志。'},
};
const diagnosticCode=(value:unknown):string|null=>{
  const v=safeIdentifier(value,64);
  // A arbitrary uppercase 'code' can itself be a business secret. Admit only
  // recognized system/SQL codes, never an unbounded opaque application value.
  return v&&/^(?:ETIMEDOUT|ECONNREFUSED|ECONNRESET|ENOTFOUND|EAI_AGAIN|EACCES|EPERM|ENOENT|EHOSTUNREACH|ECONNABORTED|ER_[A-Z0-9_]+|SQLITE_[A-Z0-9_]+|HTTP_[1-5]\d\d|[1-5]\d\d|DEADLINE_EXCEEDED|PERMISSION_DENIED|UNAUTHENTICATED|UNAVAILABLE)$/.test(v)?v:null;
};
/**
 * Preserve raw native error fields, including non-enumerable Error.message/stack.
 * Size cap (128 Ki characters) prevents one broken tool from filling task storage.
 * This is not redaction: error content is preserved verbatim up to the cap.
 */
function originalError(error:unknown){
  if(error===undefined||error===null)return {text:null,truncated:false};
  let raw:string;
  try{
    if(typeof error==='string')raw=error;
    else{
      const seen=new WeakSet<object>();
      const serialized=JSON.stringify(error,(_key,value:unknown)=>{
        if(value instanceof Error){
          const e=value as Error&{cause?:unknown};
          return {...Object.fromEntries(Object.entries(e)),name:e.name,message:e.message,
            stack:e.stack,...(e.cause===undefined?{}:{cause:e.cause})};
        }
        if(typeof value==='bigint')return value.toString();
        if(value&&typeof value==='object'){
          if(seen.has(value))return '[Circular]';
          seen.add(value);
        }
        return value;
      },2);
      raw=serialized??String(error);
    }
  }catch{
    raw=String(error);
  }
  const truncated=raw.length>131072;
  return {text:truncated?raw.slice(0,131072):raw,truncated};
}
/** Classification uses raw strings in memory and does not rewrite originalError(). */
function classify(raw:string):FailureCategory{
  if(/\b(?:401|403|UNAUTHENTICATED|UNAUTHORIZED|AUTHENTICATION|TOKEN_EXPIRED|INVALID_TOKEN)\b/i.test(raw))
    return 'AUTH';
  if(/\b(?:EACCES|EPERM|PERMISSION_DENIED)\b|permission denied|not permitted|forbidden/i.test(raw))
    return 'PERMISSION';
  if(/\b(?:ENOENT|COMMAND_NOT_FOUND|TOOL_NOT_FOUND|EXEC_NOT_FOUND)\b|command not found|tool (?:was )?not found|spawn .*enoent/i.test(raw))
    return 'TOOL_MISSING';
  if(/\b(?:ETIMEDOUT|DEADLINE_EXCEEDED|TIMEOUT)\b|timed? out|deadline exceeded|aborterror/i.test(raw))
    return 'TIMEOUT';
  if(/\b(?:ECONNREFUSED|ECONNRESET|ENOTFOUND|EAI_AGAIN|EHOSTUNREACH|ECONNABORTED)\b|connection refused|network unreachable|dns failure|socket hang up/i.test(raw))
    return 'NETWORK';
  if(/\b(?:SQLSTATE|SQLITE_|ER_BAD_FIELD_ERROR|ER_NO_SUCH_TABLE|ER_PARSE_ERROR|ER_ACCESS_DENIED_ERROR)\b|unknown column|sql syntax|relation .*does not exist|table .*doesn't exist/i.test(raw))
    return 'SQL';
  if(/\b(?:MCP|MODEL_CONTEXT_PROTOCOL)\b/i.test(raw))return 'MCP';
  if(/\b(?:TOOL_EXECUTION|TOOL_ERROR|EXECUTION_FAILED)\b/i.test(raw))return 'EXECUTION';
  return 'UNKNOWN';
}
export function makeSafeFailure(error:unknown,toolName:unknown,toolCallId:unknown,origin:FailureOrigin):SafeFailure{
  const outer=asObject(error);
  const nested=asObject(outer?.data);
  const explicitCode=diagnosticCode(outer?.code)??diagnosticCode(nested?.code);
  const raw=[
    typeof error==='string'?error:'',
    typeof outer?.message==='string'?outer.message:'',
    typeof nested?.message==='string'?nested.message:'',
    typeof outer?.type==='string'?outer.type:'',
    typeof outer?.name==='string'?outer.name:'',
    explicitCode??'',
  ].join(' ').slice(0,4096);
  const category=classify(raw);
  const observed=explicitCode??[
    ...raw.matchAll(/\b(?:ETIMEDOUT|ECONNREFUSED|ECONNRESET|ENOTFOUND|EAI_AGAIN|EACCES|EPERM|ER_[A-Z0-9_]+|SQLITE_[A-Z0-9_]+)\b/g),
  ].map(m=>diagnosticCode(m[0])).find((v):v is string=>Boolean(v))??null;
  let text=guidelines[category];
  // More specific, still fully controlled descriptions: SQL names, paths and
  // actual command arguments never leave this classification function.
  if(category==='SQL'&&/unknown column|ER_BAD_FIELD_ERROR/i.test(raw))
    text={reason:'SQL 引用了数据库中不存在的字段',suggestion:'通过 DBHub 只读核对目标表结构、字段拼写及映射关系。'};
  else if(category==='SQL'&&/ER_NO_SUCH_TABLE|table .*doesn't exist|relation .*does not exist/i.test(raw))
    text={reason:'SQL 引用了数据库中不存在的表',suggestion:'核对目标数据源、Schema、表名及环境绑定。'};
  else if(category==='SQL'&&/ER_PARSE_ERROR|sql syntax/i.test(raw))
    text={reason:'SQL 语法解析失败',suggestion:'核对执行 SQL 的数据库方言和语法规则，不要直接重试写操作。'};
  else if(category==='NETWORK'&&/ECONNREFUSED|connection refused/i.test(raw))
    text={reason:'目标服务拒绝网络连接',suggestion:'检查目标服务是否启动、监听端口、网络 ACL 与 MCP/数据库地址。'};
  else if(category==='TIMEOUT'&&/MCP|DBHUB/i.test(raw))
    text={reason:'MCP 或 DBHub 工具请求超时',suggestion:'检查工具调用超时、DBHub query_timeout 与目标数据库查询耗时。'};
  const original=originalError(error);
  return {
    version:1,
    toolName:safeIdentifier(toolName),toolCallId:safeIdentifier(toolCallId),
    category,errorCode:observed,
    nativeType:safeType(outer?.type)??safeType(outer?.name),
    reason:text.reason,suggestion:text.suggestion,origin,
    rawError:original.text,rawTruncated:original.truncated,
  };
}
const PREFIX='SAFE_FAILURE_V1:';
/** Raw errors are intentionally stored for the local administrator only. */
export function encodeSafeFailure(error:unknown,toolName:unknown,toolCallId:unknown,origin:FailureOrigin){
  return PREFIX+JSON.stringify(makeSafeFailure(error,toolName,toolCallId,origin));
}
export function decodeSafeFailure(value:unknown):SafeFailure|null{
  if(typeof value!=='string'||!value.startsWith(PREFIX))return null;
  try{
    const x=JSON.parse(value.slice(PREFIX.length)) as SafeFailure;
    if(x.version!==1||typeof x.reason!=='string'||typeof x.suggestion!=='string'||
      !Object.hasOwn(guidelines,x.category))return null;
    return x;
  }catch{return null;}
}
