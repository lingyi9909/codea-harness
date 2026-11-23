import {useEffect,useState} from 'react';
import {CalendarDate,SecondClock,localDayBounds,parseSecondDateTime} from './TimeFields.tsx';
import {ModelPicker} from './ModelPicker.tsx';
import {api} from './api/client.ts';
import type {ChatModel} from '../../../packages/contracts/src/chat.ts';
import './component-lab.css';

const today=()=>{
  const d=new Date();
  return [d.getFullYear(),String(d.getMonth()+1).padStart(2,'0'),String(d.getDate()).padStart(2,'0')].join('-');
};
/** Self-contained UI playground: no database query or model execution is initiated. */
export function ComponentLabPage(){
  const [date,setDate]=useState(today),[startClock,setStartClock]=useState('09:00:00'),[endClock,setEndClock]=useState('18:00:00');
  const [text,setText]=useState(''),[checked,setChecked]=useState(false),[selected,setSelected]=useState('normal');
  const [dialog,setDialog]=useState(false),[notice,setNotice]=useState('');
  const [model,setModel]=useState<ChatModel>();
  const bounds=localDayBounds(date),start=parseSecondDateTime(date,startClock),end=parseSecondDateTime(date,endClock);
  useEffect(()=>{api.recordPageVisit('component-lab');},[]);
  return <main className="component-lab">
    <header><div><small>UI COMPONENTS / 组件验证</small><h1>公共组件测试</h1>
      <p>真实复用工作台使用的日期、秒级时间和模型选择组件。所有操作仅在页面内模拟，不发送业务查询。</p></div>
      <nav className="component-admin-links" aria-label="系统管理"><a href="/config">场景配置</a><a href="/institution-tasks">接口任务管理</a><a href="/institution-tasks?view=access">接口接入情况</a><a href="/analytics">访问统计</a><a href="/">返回工作台</a></nav>
    </header>
    <div className="lab-grid">
      <section className="lab-panel">
        <h2>日期组件</h2><p>机构报文与公管排查共用原生日期组件。</p>
        <CalendarDate label="查询日期" value={date} onChange={setDate} invalid={!bounds}/>
        <div className="lab-preview" role="status">
          {bounds?<>选择：{date}　范围：00:00:00 — 23:59:59　实际跨度：{bounds.durationHours} 小时</>:'无效日期'}
        </div>
      </section>
      <section className="lab-panel">
        <h2>时间组件（精确到秒）</h2><p>可手动输入 HH:mm:ss，检查起止时间与边界逻辑。</p>
        <div className="lab-two">
          <SecondClock label="开始时刻" value={startClock} onChange={setStartClock} invalid={start===null}/>
          <SecondClock label="结束时刻" value={endClock} onChange={setEndClock} invalid={end===null||start!==null&&end<=start}/>
        </div>
        <div className="lab-preview" role="status">
          {start!==null&&end!==null&&end>start?'合法时间范围 · '+Math.round((end-start)/1000)+' 秒':'时间格式错误或结束时间早于开始时间'}
        </div>
        <div className="lab-preset">{['00:00:00','09:00:00','23:59:59'].map(clock=>
          <button key={clock} type="button" onClick={()=>setStartClock(clock)}>{clock}</button>)}</div>
      </section>
      <section className="lab-panel"><h2>模型选择器</h2>
        <p>使用真实模型选择 UI 与模拟候选项，不连接模型。</p>
        <ModelPicker value={model} allowDefault loadModels={async()=>[
          {id:'kimi',providerID:'demo',name:'Kimi（模拟）'},
          {id:'model-b',providerID:'demo',name:'测试模型 B'},
        ]} onSelect={setModel}/>
        <div className="lab-preview">{model?.name??'默认模型'}（仅界面测试）</div>
      </section>
      <section className="lab-panel"><h2>表单组件</h2><p>输入、复选框、选择器以及校验反馈。</p>
        <label>关键词<input placeholder="输入关键字" value={text} onChange={e=>setText(e.target.value)}/></label>
        <label className="lab-check"><input type="checkbox" checked={checked} onChange={e=>setChecked(e.target.checked)}/> 启用附加选项</label>
        <label>优先级<select value={selected} onChange={e=>setSelected(e.target.value)}>
          <option value="normal">普通</option><option value="high">高</option></select></label>
        <div className="lab-preview">当前：{text||'未填写'} · {checked?'已勾选':'未勾选'} · {selected==='high'?'高':'普通'}</div>
      </section>
      <section className="lab-panel"><h2>按钮与弹窗</h2><p>点击后仅显示本地反馈。</p>
        <div className="lab-actions">
          <button className="primary" onClick={()=>setNotice('操作已完成（模拟）')}>主要操作</button>
          <button className="secondary" onClick={()=>setDialog(true)}>打开弹窗</button>
          <button disabled>禁用状态</button>
        </div>
        {notice&&<div className="lab-preview" role="status">{notice}</div>}
        {dialog&&<div className="lab-overlay" role="presentation"><section role="dialog" aria-modal="true" aria-label="测试弹窗">
          <h3>弹窗测试</h3><p>这是公共交互样式测试，没有真实业务操作。</p>
          <button onClick={()=>setDialog(false)}>关闭弹窗</button>
        </section></div>}
      </section>
    </div>
  </main>;
}
