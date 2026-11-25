/**
 * 对话业务契约
 *
 * 职责：定义前后端共享的聊天、场景和消息类型。
 * Java 研发阅读提示：类比 Java 公共 DTO；TS 类型只在编译期有效，入站 JSON 仍需校验。
 *
 * 仅补充模块注释，原有代码和接口行为保持不变。
 */
export type ChatModel={providerID:string;id:string;name:string;variant?:string;variants?:string[];context?:number};
export type ChatScenario='direct-chat'|'log-inspection'|'message-inspection'|'institution-message-inspection'|'data-analysis';
export type SceneId='logs'|'message'|'institution-message'|'data';
export type SceneBinding={directory:string;executor:{type:'skill'|'subagent'|'command'|'builtin';name:string}};
export type SceneConfigValue={version:1;scenes:Partial<Record<SceneId,SceneBinding>>};
export const accessToolActions=[
  'shell','execute','skill','subagent',
  'read','write','edit','glob','grep','list','external_directory',
  'webfetch','websearch',
  'dbhub_search_objects','dbhub_execute_sql',
] as const;
export type AccessToolAction=typeof accessToolActions[number];
export type AccessScene='home'|'review'|'logs'|'message'|'institution-message'|'data'|'component-lab'|'analytics';
export type AccessToolChoice='inherit'|'allow'|'ask'|'deny';
export type CustomToolRule={action:string;resource:string;effect:'allow'|'ask'|'deny'};
export type SceneAccessPolicy={lan:boolean;tools:Record<AccessToolAction,AccessToolChoice>;custom:CustomToolRule[]};
export type SceneAccessConfig={version:2;scenes:Record<AccessScene,SceneAccessPolicy>};
export type SceneConfigResponse={workspace:string;configPath:string;accessPath:string;config:SceneConfigValue;revision:string;canEdit:boolean;username:string|null};
export type ChatHistoryPage={items:{id:string;title:string;updated:number}[];next?:string};
export type ChatSettings={configured:boolean;directories:string[]};
export type ChatStep=({kind:'text';text:string}|{kind:'thinking';detailPart?:number;live?:boolean;at?:number;ended?:number;complete:boolean;hasText:boolean;text?:string}|{kind:'notice';text:string;at?:number;problem?:boolean}|
  {kind:'tool';name:string;status:string;at?:number;ended?:number;input?:string;progress?:string;result?:string;error?:string}) & {ordinal?:number};
export type ChatFooter={agent?:string;model?:string;durationMs?:number;tokensPerSecond?:number};
export type ChatIdleOutcome='succeeded'|'failed'|'interrupted';
export type ChatMessage={id:string;role:'user'|'assistant';footer?:ChatFooter;idleOutcome?:ChatIdleOutcome;toolCallTurn?:boolean;queued?:boolean;complete?:boolean;text:string;steps?:ChatStep[];tools:{name:string;status:string}[];files:string[]};
export type ChatStatus={id:string;title?:string;model?:Pick<ChatModel,'id'|'providerID'|'variant'>;directory:string;active:boolean;outcome?:string;toolPolicy?:'deny-all'|'institution-readonly';messages:ChatMessage[];
  forms?:ChatForm[];permissions:{id:string;action:string;resources:string[];message?:string;revision:string}[]};


export type ChatFragment={id:string;message:string;ordinal:number;kind:'thinking'|'text';
  phase:'started'|'delta'|'ended';at:number;text:string};
export type ToolProgress={id:string;message:string;at:number;text:string};
export type ChatLifecycle={id:string;status:'started'|'succeeded'|'failed'|'interrupted';at:number};

export type FormValue=string|number|boolean|string[];
export type FormAnswers=Record<string,FormValue>;
type FormChoice={value:string;label:string;description?:string};
export type ChatFormField={
  key:string;title?:string;description?:string;hidden?:boolean;required?:boolean;
  when?:{key:string;op:'eq'|'neq';value:string|number|boolean}[];
} & (
  {type:'string';default?:string;options?:FormChoice[];custom?:boolean;format?:'email'|'uri'|'date'|'date-time';
    placeholder?:string;pattern?:string;minLength?:number;maxLength?:number} |
  {type:'integer'|'number';default?:number;minimum?:number;maximum?:number} |
  {type:'boolean';default?:boolean} |
  {type:'multiselect';default?:string[];options:FormChoice[];custom?:boolean;minItems?:number;maxItems?:number} |
  {type:'external';url?:string}
);
export type ChatForm={id:string;title:string;fields:ChatFormField[];revision:string};

export function formDefaults(form:ChatForm):FormAnswers{
  return Object.fromEntries(form.fields.flatMap(field=>'default' in field&&field.default!==undefined?[[field.key,field.default]]:[]));
}

export function matchesFormCondition(value:FormValue|undefined,condition:NonNullable<ChatFormField['when']>[number]){
  if(value===undefined)return false;
  const hit=Array.isArray(value)?value.some(item=>item===condition.value):value===condition.value;
  return condition.op==='eq'?hit:!hit;
}

// The native schema requires conditions to reference earlier fields.
export function activeFormFields(form:ChatForm,answers:FormAnswers):ChatFormField[]{
  const values=new Map<string,FormValue>();
  return form.fields.filter(field=>{
    if(field.when?.some(condition=>!matchesFormCondition(values.get(condition.key),condition)))return false;
    if(Object.hasOwn(answers,field.key))values.set(field.key,answers[field.key]);
    return true;
  });
}
