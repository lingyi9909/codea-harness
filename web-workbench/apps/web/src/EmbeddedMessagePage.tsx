import {useEffect,useState} from 'react';
import {api} from './api/client.ts';
import {LogInvestigation} from './LogInvestigation.tsx';
import {ModelPicker} from './ModelPicker.tsx';
import type {ChatModel} from '../../../packages/contracts/src/chat.ts';
import './embedded-message.css';
type LaunchMessage={type:'workbench.embed.launch';nonce:string;ticket:string};
export function EmbeddedMessagePage(){
  useEffect(()=>{api.recordPageVisit('embedded-message');},[]);
  const [ready,setReady]=useState(false),[error,setError]=useState(''),[expires,setExpires]=useState(0);
  const [model,setModel]=useState<ChatModel|undefined>();
  const [modelError,setModelError]=useState('');
  useEffect(()=>{
    if(!ready)return;
    let live=true;
    api.chatModels().then(models=>{if(live&&models.length===1)setModel(models[0]);}).catch(()=>{if(live)setModelError('无法读取当前可用模型，请从右上角重新选择。');});
    return()=>{live=false;};
  },[ready]);
  useEffect(()=>{
    let live=true,claimed=false;const nonce=crypto.randomUUID();
    const parentOrigin=new URLSearchParams(window.location.search).get('parentOrigin')??'';
    const receive=async(event:MessageEvent)=>{
      if(!live||claimed||event.source!==window.parent||event.origin!==parentOrigin)return;
      const data=event.data as Partial<LaunchMessage>|null;
      if(!data||data.type!=='workbench.embed.launch'||data.nonce!==nonce||typeof data.ticket!=='string')return;
      claimed=true;
      try{
        const expiry=await api.openEmbedded(data.ticket,parentOrigin);
        if(live){setExpires(expiry);setReady(true);}
      }catch{if(live)setError('无法打开排查会话，接入凭证可能已过期。请重新打开此页面。');}
    };
    const init=async()=>{
      if(window.parent===window){setError('请从已接入的业务系统打开此页面。');return;}
      try{
        const response=await fetch('/api/v1/embed/config?parentOrigin='+encodeURIComponent(parentOrigin),{credentials:'omit',cache:'no-store',redirect:'error'});
        if(!response.ok)throw new Error();
        const body=await response.json();if(body.data?.parentOrigin!==parentOrigin)throw new Error();
        if(!live)return;
        window.addEventListener('message',receive);
        window.parent.postMessage({type:'workbench.embed.ready',version:1,nonce},parentOrigin);
      }catch{if(live)setError('当前系统尚未获准嵌入报文排查页面。');}
    };
    void init();
    return()=>{live=false;window.removeEventListener('message',receive);api.clearEmbedded();};
  },[]);
  useEffect(()=>{
    if(!ready||!expires)return;
    const timer=setTimeout(()=>{api.clearEmbedded();setReady(false);setError('本次排查会话已过期，请重新打开页面。');},Math.max(0,expires-Date.now()));
    return()=>clearTimeout(timer);
  },[ready,expires]);
  if(!ready)return <main className="embedded-message-state"><h1>报文问题排查</h1><p role={error?'alert':'status'}>{error||'正在建立独立排查会话…'}</p></main>;
  return <main className="embedded-message-shell">
    <header className="embedded-message-header"><strong>报文问题排查</strong><div><span>本次模型</span><ModelPicker value={model} loadModels={async()=>api.chatModels()} onSelect={setModel}/><button type="button" onClick={()=>{void api.closeEmbedded().finally(()=>{setReady(false);setError('本次会话已结束，可从业务系统重新打开。');});}}>结束会话</button></div></header>
    {(!model||modelError)&&<p className="embedded-model-hint" role={modelError?'alert':'status'}>{modelError||'请先选择本次排查使用的内网模型。'}</p>}
    <LogInvestigation mode="message" globalModel={model} embedded/>
  </main>;
}
