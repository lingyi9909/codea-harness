import {useEffect,useRef,useState} from 'react';
import {api,ApiError} from './api/client.ts';
import {InstitutionMessageInvestigation} from './InstitutionMessageInvestigation.tsx';
import {ModelPicker} from './ModelPicker.tsx';
import type {ChatModel} from '../../../packages/contracts/src/chat.ts';
import './embedded-message.css';
export function AnonymousInstitutionPage(){
  const opening=useRef<Promise<number>|undefined>(undefined);
  const [ended,setEnded]=useState(false);
  const [ready,setReady]=useState(false),[error,setError]=useState(''),[expires,setExpires]=useState(0),[offline,setOffline]=useState(false);
  const [model,setModel]=useState<ChatModel|undefined>();
  useEffect(()=>{
    let live=true;
    opening.current??=api.openAnonymousEmbedded();
    opening.current.then(expiry=>{if(live){setExpires(expiry);setReady(true);}}).catch(e=>{if(live)setError(e instanceof Error?e.message:'无法建立独立会话，请稍后重新打开页面。');});
    return()=>{live=false;};
  },[]);
  useEffect(()=>{
    if(!ready)return;let live=true;
    api.chatModels().then(models=>{if(live&&models.length===1)setModel(models[0]);}).catch(()=>{});
    let polling=false;
    const timer=setInterval(()=>{
      if(polling)return;polling=true;
      api.embeddedSession().then(()=>{if(live)setOffline(false);}).catch(e=>{
        if(!live)return;
        if(e instanceof ApiError&&e.status===401){api.clearEmbedded();setReady(false);setError('本次会话已结束或过期，请重新打开页面。');}
        else setOffline(true);
      }).finally(()=>{polling=false;});
    },3000);
    const expiry=setTimeout(()=>{api.clearEmbedded();setReady(false);setError('本次会话已过期，请重新打开页面。');},Math.max(0,expires-Date.now()));
    return()=>{live=false;clearInterval(timer);clearTimeout(expiry);};
  },[ready,expires]);
  if(!ready)return <main className="embedded-message-state"><h1>机构报文排查</h1><p role={ended?'status':error?'alert':'status'}>{error||'正在建立独立排查会话…'}</p></main>;
  return <main className="embedded-message-shell anonymous-institution">
    <header className="embedded-message-header"><strong>机构报文排查</strong><div><span>本次模型</span><ModelPicker value={model} loadModels={async()=>api.chatModels()} onSelect={setModel}/><button onClick={()=>{void api.closeEmbedded().then(()=>{setEnded(true);setReady(false);setError('本次会话已结束。');}).catch(()=>{setReady(false);setError('结束请求结果不明，请稍后重新打开页面。');});}}>结束会话</button></div></header>
    {offline&&<p role="status" className="embedded-model-hint">服务暂时不可用，正在重新连接当前会话，请勿重复提交。</p>}
    {error&&<p role="alert">{error}</p>}
    {!model&&<p className="embedded-model-hint">请先选择本次使用的内网模型。</p>}
    <InstitutionMessageInvestigation embedded globalModel={model}/>
  </main>;
}
