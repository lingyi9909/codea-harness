export type ChatModel={providerID:string;id:string;name:string;variant?:string;variants?:string[];context?:number};
export type ChatSettings={configured:boolean;directories:string[]};
export type ChatStep=({kind:'text';text:string}|{kind:'thinking';live?:boolean;at?:number;ended?:number;complete:boolean;hasText:boolean;text?:string}|{kind:'notice';text:string;at?:number}|
  {kind:'tool';name:string;status:string;at?:number;ended?:number;input?:string;progress?:string;result?:string;error?:string}) & {ordinal?:number};
export type ChatMessage={id:string;role:'user'|'assistant';complete?:boolean;text:string;steps?:ChatStep[];tools:{name:string;status:string}[];files:string[]};
export type ChatStatus={id:string;model?:Pick<ChatModel,'id'|'providerID'|'variant'>;directory:string;active:boolean;outcome?:string;messages:ChatMessage[];
  forms?:ChatForm[];permissions:{id:string;action:string;resources:string[];message?:string;revision:string}[]};

export type ChatAccessEntry={time:string;requestId:string;source:'工作台'|'OpenCode'|'服务发现';operation:string;result:string;ms:number};

export type ChatFragment={id:string;message:string;ordinal:number;kind:'thinking'|'text';
  phase:'started'|'delta'|'ended';at:number;text:string};
export type ToolProgress={id:string;at:number;text:string};

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
