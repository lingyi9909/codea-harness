export const scenes={home:'工作台',chat:'直接对话',review:'代码审查',logs:'日志排查',data:'数据分析'} as const;
export type Scene=keyof typeof scenes;
const cards=[
  {id:'review',icon:'{ }',desc:'固定 Git 版本，审查提交差异与代码风险。'},
  {id:'logs',icon:'≋',desc:'按请求入参、时间和 Trace ID 排查异常。'},
  {id:'data',icon:'▥',desc:'读取 CSV，查看字段分布与数值统计。'},
] as const;
export function ScenarioHome({open}:{open:(s:Scene)=>void}){
  return <section className="scene-page"><div className="scene-title"><div><h1>研发工作台</h1><p>选择工作场景，或直接与 OpenCode 对话。</p></div></div>
    <button className="chat-hero" onClick={()=>open('chat')}><span className="chat-symbol" aria-hidden="true">✳</span><strong>直接对话</strong>
      <span>在项目目录中提问，让 OpenCode 阅读和修改文件。</span><b>进入对话　↗</b></button>
    <h2 className="scenario-label">专项工作</h2><div className="scene-cards">{cards.map(c=><button key={c.id} className={'scene-card '+c.id} onClick={()=>open(c.id)}>
      <span className="scene-icon" aria-hidden="true">{c.icon}</span><h2>{scenes[c.id]}</h2><p>{c.desc}</p><span className="card-arrow" aria-hidden="true">↗</span></button>)}</div>
  </section>;
}
