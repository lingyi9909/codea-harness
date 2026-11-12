import {useId,useState} from 'react';
import type {FormEvent} from 'react';
import {activeFormFields,formDefaults,type ChatForm,type ChatFormField,type ChatStatus,type FormValue} from '../../../packages/contracts/src/chat.ts';
import './chat-requests.css';

type RequestAction=(kind:'permission'|'form',payload:Record<string,unknown>)=>void;
export function ChatRequests({state,busy,onAction}:{state:ChatStatus;busy:boolean;onAction:RequestAction}){
  const forms=state.forms??[];
  if(!forms.length&&!state.permissions.length)return null;
  return <section className="chat-requests" aria-label="待处理请求">
    <p role="status">OpenCode 正在等待你处理{forms.length+state.permissions.length}个请求</p>
    {state.permissions.map(p=><section key={p.id} className="request-item">
      <strong>OpenCode 请求授权 · {p.action}</strong>{p.message&&<p>{p.message}</p>}<pre>{p.resources.join('\n')}</pre>
      <div className="request-actions">{(['once','reject'] as const).map(decision=>
        <button type="button" disabled={busy} key={decision} onClick={()=>onAction('permission',{requestId:p.id,revision:p.revision,decision})}>
          {decision==='once'?'仅本次允许':'拒绝'}</button>)}</div></section>)}
    {forms.map(form=><RequestForm key={form.id+':'+form.revision} form={form} busy={busy} onAction={onAction}/>)}
  </section>;
}
function RequestForm({form,busy,onAction}:{form:ChatForm;busy:boolean;onAction:RequestAction}){
  const [answers,setAnswers]=useState(()=>formDefaults(form));
  const [message,setMessage]=useState('');
  const fields=activeFormFields(form,answers).filter(f=>!f.hidden);
  const identity={requestId:form.id,revision:form.revision};
  const submit=(event:FormEvent)=>{
    event.preventDefault();
    if(busy)return;
    for(const field of fields){
      if(field.type!=='multiselect')continue;
      const values=answers[field.key],count=Array.isArray(values)?values.length:0;
      if(count<Math.max(field.minItems??0,field.required?1:0)||count>(field.maxItems??Infinity)){
        setMessage('请检查「'+(field.title??field.key)+'」的选项数量。');return;
      }
    }
    const answer=Object.fromEntries(fields.filter(f=>Object.hasOwn(answers,f.key)).map(f=>[f.key,answers[f.key]]));
    onAction('form',{...identity,decision:'reply',answer});
  };
  return <form aria-label={form.title} className="request-item" onSubmit={submit}><strong>{form.title}</strong>
    <fieldset className="request-fields" disabled={busy}>{fields.map(field=>
      <Field key={field.key} field={field} value={Object.hasOwn(answers,field.key)?answers[field.key]:undefined}
        change={value=>{setMessage('');setAnswers(previous=>{
          const next={...previous};
          if(value===undefined)delete next[field.key];else Object.defineProperty(next,field.key,{value,writable:true,configurable:true,enumerable:true});
          return next;
        });}}/>)}</fieldset>
    {message&&<p role="alert">{message}</p>}
    <div className="request-actions"><button type="submit" disabled={busy}>提交回答</button>
      <button type="button" disabled={busy} onClick={()=>onAction('form',{...identity,decision:'cancel'})}>取消请求</button></div>
  </form>;
}
function Field({field,value,change}:{field:ChatFormField;value?:FormValue;change:(v?:FormValue)=>void}){
  const id=useId(),title=(field.title??field.key)+(field.required?' *':'');
  const input={id,required:field.required,'aria-describedby':field.description?id+'-description':undefined};
  const description=field.description&&<small id={id+'-description'}>{field.description}</small>;
  if(field.type==='external')return <div><strong>{title}</strong>{description}
    {field.url?<a href={field.url} target="_blank" rel="noopener noreferrer">打开外部页面 ↗</a>:<span>外部页面链接不可用</span>}
    <label><input type="checkbox" required checked={value===true} onChange={e=>change(e.target.checked||undefined)}/>我已完成此步骤</label></div>;
  if(field.type==='multiselect')return <MultiChoice field={field} title={title} values={Array.isArray(value)?value:[]} change={change}/>;
  if(field.type==='string'&&field.options?.length&&field.options.length<=6)
    return <SingleChoice field={field} title={title} value={typeof value==='string'?value:undefined} change={change}/>;
  let control;
  if(field.type==='boolean'){
    control=<select {...input} value={typeof value==='boolean'?String(value):''}
      onChange={e=>change(e.target.value===''?undefined:e.target.value==='true')}>
      <option value="">请选择</option><option value="true">是</option><option value="false">否</option></select>;
  }else if(field.type==='number'||field.type==='integer'){
    control=<input {...input} type="number" min={field.minimum} max={field.maximum} step={field.type==='integer'?1:'any'}
      value={typeof value==='number'?value:''} onChange={e=>change(Number.isFinite(e.target.valueAsNumber)?e.target.valueAsNumber:undefined)}/>;
  }else if(field.type==='string'){
    if(field.options?.length&&!field.custom){
      control=<select {...input} value={typeof value==='string'?value:''} onChange={e=>change(e.target.value||undefined)}>
        <option value="">请选择</option>{field.options.map(o=><option key={o.value} value={o.value}>{o.label}{o.description?' · '+o.description:''}</option>)}</select>;
    }else{
      const kind=field.format==='uri'?'url':field.format==='date-time'?'text':field.format??'text';
      control=<><input {...input} type={kind} value={typeof value==='string'?value:''} placeholder={field.placeholder??(field.format==='date-time'?'2026-01-01T12:00:00+08:00':undefined)}
        minLength={field.minLength} maxLength={field.maxLength} pattern={field.pattern} onChange={e=>change(e.target.value)}
        list={field.options?.length?id+'-choices':undefined}/>
        {field.options?.length?<datalist id={id+'-choices'}>{field.options.map(o=><option value={o.value} key={o.value}>{o.label}</option>)}</datalist>:null}</>;
    }
  }
  return <div><label htmlFor={id}>{title}</label>{description}{control}</div>;
}
function SingleChoice({field,title,value,change}:{field:Extract<ChatFormField,{type:'string'}>;title:string;value?:string;change:(v?:string)=>void}){
  const name=useId(),options=field.options??[];
  const [custom,setCustom]=useState(value!==undefined&&!options.some(o=>o.value===value));
  const [draft,setDraft]=useState(custom?value??'':'');
  return <fieldset className="request-radio-group"><legend>{title}</legend>{field.description&&<small>{field.description}</small>}
    {options.map(o=><label key={o.value} className="request-radio"><input type="radio" name={name} required={field.required}
      checked={!custom&&value===o.value} onChange={()=>{setCustom(false);change(o.value);}}/>
      <span>{o.label}{o.description&&<small>{o.description}</small>}</span></label>)}
    {field.custom&&<><label className="request-radio"><input type="radio" name={name} required={field.required} checked={custom}
      onChange={()=>{setCustom(true);change(draft||undefined);}}/>其他回答</label>
      {custom&&<input aria-label={title+' · 其他回答'} required={field.required} value={draft} minLength={field.minLength} maxLength={field.maxLength}
        placeholder={field.placeholder} onChange={e=>{setDraft(e.target.value);change(e.target.value||undefined);}}/>}</>}
    {!field.required&&(custom||value!==undefined)&&<button type="button" onClick={()=>{setCustom(false);change(undefined);}}>清除选择</button>}
  </fieldset>;
}
function MultiChoice({field,title,values,change}:{field:Extract<ChatFormField,{type:'multiselect'}>;title:string;values:string[];change:(v:string[])=>void}){
  const [custom,setCustom]=useState(()=>values.filter(v=>!field.options.some(o=>o.value===v)).join('\n'));
  return <fieldset className="request-choices"><legend>{title}</legend>{field.description&&<small>{field.description}</small>}
    {field.options.map(o=><label key={o.value}><input type="checkbox" checked={values.includes(o.value)}
      onChange={e=>change(e.target.checked?[...values,o.value]:values.filter(v=>v!==o.value))}/>{o.label}
      {o.description&&<small>{o.description}</small>}</label>)}
    {field.custom&&<label>其他选项（每行一个）<textarea rows={2} value={custom} onChange={e=>{
      setCustom(e.target.value);
      change([...new Set([...values.filter(v=>field.options.some(o=>o.value===v)),...e.target.value.split('\n').map(s=>s.trim()).filter(Boolean)])]);
    }}/></label>}</fieldset>;
}
