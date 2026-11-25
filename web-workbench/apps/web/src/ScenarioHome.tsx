/**
 * 研发场景首页
 *
 * 职责：展示可用业务场景与入口，帮助用户定位具体场景。
 * Java 研发阅读提示：场景入口展示与 Skill 实际可用性是两层逻辑。
 *
 * 以下为模块说明注释，不改变原有执行逻辑。
 */
/** Keep legacy scene ids so saved browser navigation survives the visual rename. */
export const scenes={
  home:'工作台',
  chat:'直接对话',
  review:'代码审查',
  logs:'公管问题排查',
  message:'报文问题排查',
  data:'公管数据分析',
  settings:'场景配置',
} as const;
export type Scene=keyof typeof scenes;

export const scenarioGroups=[
  {id:'problem',label:'问题排查',icon:'≡',children:['logs','message']},
  {id:'analysis',label:'数据分析',icon:'▥',children:['data']},
] as const;
export type ScenarioGroup=typeof scenarioGroups[number]['id'];
export function groupForScene(scene:Scene):ScenarioGroup|null{
  return scene==='logs'||scene==='message'?'problem':scene==='data'?'analysis':null;
}

// Kept for compatibility with callers of the old introductory component.
// The current product starts directly in OpenCode chat, not on this card grid.
const cards=[
  {id:'review',icon:'{ }',desc:'固定 Git 版本，审查提交差异与代码风险。'},
  {id:'logs',icon:'≋',desc:'根据 Trace ID、时间范围与证据排查公管问题。'},
  {id:'message',icon:'≋',desc:'基于已有日志工具排查报文相关问题。'},
  {id:'data',icon:'▥',desc:'读取公管 CSV，查看字段与数值统计。'},
] as const;
export function ScenarioHome({open}:{open:(s:Scene)=>void}){
  return <section className="scene-page"><div className="scene-title"><div><h1>智能工作台</h1><p>选择工作场景，或直接与 OpenCode 对话。</p></div></div>
    <button className="chat-hero" onClick={()=>open('chat')}><span className="chat-symbol" aria-hidden="true">✳</span><strong>直接对话</strong>
      <span>在项目目录中提问，让 OpenCode 阅读和修改文件。</span><b>进入对话　↗</b></button>
    <h2 className="scenario-label">专项工作</h2><div className="scene-cards">{cards.map(c=><button key={c.id} className={'scene-card '+c.id} onClick={()=>open(c.id)}>
      <span className="scene-icon" aria-hidden="true">{c.icon}</span><h2>{scenes[c.id]}</h2><p>{c.desc}</p><span className="card-arrow" aria-hidden="true">↗</span></button>)}</div>
  </section>;
}
