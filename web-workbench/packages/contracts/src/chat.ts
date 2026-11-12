export type ChatModel={providerID:string;id:string;name:string};
export type ChatSettings={configured:boolean;directories:string[]};
export type ChatMessage={id:string;role:'user'|'assistant';text:string;tools:{name:string;status:string}[];files:string[]};
export type ChatStatus={id:string;directory:string;active:boolean;outcome?:string;messages:ChatMessage[];
  permissions:{id:string;action:string;resources:string[];revision:string}[]};

export type ChatAccessEntry={time:string;requestId:string;source:'工作台'|'OpenCode'|'服务发现';operation:string;result:string;ms:number};
