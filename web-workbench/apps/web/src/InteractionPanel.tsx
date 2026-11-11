import { useState } from 'react';
import type { Schema } from './api/client.ts';

type Interaction=Schema<'Interaction'>;
type Answer=Schema<'AnswerRequest'>['answer'];
type Value=string|boolean|string[];
export function InteractionPanel(props:{
  item:Interaction;busy:boolean;onAnswer:(answer:Answer)=>void;
}){
  const {item,busy,onAnswer}=props;
  const [values,setValues]=useState<Record<string,Value>>({});
  const [invalid,setInvalid]=useState('');
  const editable=item.status==='PENDING'&&!busy;
  const update=(id:string,value:Value)=>{
    setValues(prev=>({...prev,[id]:value}));
    setInvalid('');
  };
  const submit=()=>{
    if(item.kind!=='FORM'||!editable)return;
    for(const field of item.fields){
      const v=values[field.id];
      if(field.required&&(v===undefined||
        (typeof v==='string'&&!v.trim())||(Array.isArray(v)&&!v.length))){
        setInvalid('请填写：'+field.label);return;
      }
    }
    onAnswer({kind:'FORM',cancel:false,values});
  };
  return <section className="detail-card permission">
    <div className="card-head">
      <h2>{item.kind==='PERMISSION'?'原生工具授权':'原生补充表单'}</h2>
      <span className="chip">请求 #{item.revision} · {item.status}</span>
    </div>
    <p className="muted">由当前原生执行请求触发，过期时间：{item.expiresAt}。只有 PENDING 可提交。</p>
    {item.kind==='PERMISSION'?<>
      <p>工具：<strong>{item.tool}</strong></p>
      <pre>{item.requestText}</pre>
      <p className="muted">仅本次允许，不修改全局权限。未知结果必须等待服务端核对。</p>
      <div className="button-row">
        {item.allowedDecisions.includes('DENY')&&
          <button disabled={!editable} onClick={()=>onAnswer({kind:'PERMISSION',decision:'DENY'})}>拒绝</button>}
        {item.allowedDecisions.includes('ALLOW_ONCE')&&
          <button className="primary" disabled={!editable}
            onClick={()=>onAnswer({kind:'PERMISSION',decision:'ALLOW_ONCE'})}>仅本次允许</button>}
      </div>
    </>:<>
      <p>{item.title}</p>
      {item.fields.map(field=><div className="native-field" key={field.id}>
        <label htmlFor={'native-'+item.id+'-'+field.id}>
          {field.label}{field.required?' *':''}
        </label>
        {field.type==='TEXT'?<input id={'native-'+item.id+'-'+field.id}
          disabled={!editable} maxLength={field.maxLength}
          value={typeof values[field.id]==='string'?values[field.id] as string:''}
          onChange={e=>update(field.id,e.target.value)}/>
          :field.type==='BOOLEAN'?<select id={'native-'+item.id+'-'+field.id}
            disabled={!editable}
            value={typeof values[field.id]==='boolean'?(values[field.id]?'true':'false'):''}
            onChange={e=>update(field.id,e.target.value==='true')}>
            <option value="">请选择</option><option value="true">是</option><option value="false">否</option>
          </select>
          :field.type==='SINGLE_SELECT'?<select id={'native-'+item.id+'-'+field.id}
            disabled={!editable}
            value={typeof values[field.id]==='string'?values[field.id] as string:''}
            onChange={e=>update(field.id,e.target.value)}>
            <option value="">请选择</option>
            {field.options.map((choice,i)=><option key={i} value={choice}>{choice}</option>)}
          </select>
          :<div className="multi-options" id={'native-'+item.id+'-'+field.id}>
            {field.options.map((option,i)=>{
              const current=Array.isArray(values[field.id])?values[field.id] as string[]:[];
              return <label key={i}><input type="checkbox" disabled={!editable}
                checked={current.includes(option)} onChange={e=>update(field.id,
                  e.target.checked?[...current,option]:current.filter(x=>x!==option))}/>{option}</label>;
            })}
          </div>}
      </div>)}
      {invalid&&<p className="notice" role="alert">{invalid}</p>}
      <div className="button-row">
        <button disabled={!editable} onClick={()=>onAnswer({kind:'FORM',cancel:true,values:{}})}>取消表单</button>
        <button className="primary" disabled={!editable} onClick={submit}>提交原生表单</button>
      </div>
    </>}
  </section>;
}
