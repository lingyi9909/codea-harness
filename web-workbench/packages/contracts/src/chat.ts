export type ChatModel={providerID:string;id:string;name:string};
export type ChatSettings={configured:boolean;directories:string[]};
export type ChatStep={kind:'text';text:string}|{kind:'thinking';at?:number;ended?:number;complete:boolean;hasText:boolean;text?:string}|{kind:'notice';text:string;at?:number}|
  {kind:'tool';name:string;status:string;at?:number;ended?:number;input?:string;progress?:string;result?:string;error?:string};
export type ChatMessage={id:string;role:'user'|'assistant';text:string;steps?:ChatStep[];tools:{name:string;status:string}[];files:string[]};
export type ChatStatus={id:string;model?:Pick<ChatModel,'id'|'providerID'>;directory:string;active:boolean;outcome?:string;messages:ChatMessage[];
  permissions:{id:string;action:string;resources:string[];revision:string}[]};

export type ChatAccessEntry={time:string;requestId:string;source:'工作台'|'OpenCode'|'服务发现';operation:string;result:string;ms:number};
