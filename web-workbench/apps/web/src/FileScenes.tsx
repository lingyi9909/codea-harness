import {useMemo,useRef,useState} from 'react';

// CSV accepts escaped quotes, quoted newlines, CRLF and a UTF-8 BOM.
export function csvTable(input:string){
  const rows:string[][]=[];let row:string[]=[],field='',quoted=false,closed=false;
  const text=input.replace(/^\uFEFF/,'');
  for(let i=0;i<text.length;i++){
    const ch=text[i];
    if(quoted){if(ch!=='"')field+=ch;else if(text[i+1]==='"'){field+='"';i++;}else{quoted=false;closed=true;}continue;}
    if(ch===','||ch==='\n'||ch==='\r'){
      row.push(field);field='';closed=false;
      if(ch!==','){if(ch==='\r'&&text[i+1]==='\n')i++;rows.push(row);row=[];}continue;
    }
    if(closed||ch==='"'&&field)throw Error('CSV 引号格式无效');
    if(ch==='"')quoted=true;else field+=ch;
  }
  if(quoted)throw Error('CSV 中存在未闭合的引号');
  if(row.length||field||closed){row.push(field);rows.push(row);}
  if(!rows.length||rows.some(r=>r.length!==rows[0].length))throw Error('请使用包含表头且各行列数一致的 CSV');
  return rows;
}
export function FileScene(){
  const [name,setName]=useState(''),[table,setTable]=useState<string[][]>([]);
  const [col,setCol]=useState(0),[error,setError]=useState('');
  const generation=useRef(0);
  const stats=useMemo(()=>{let sum=0,count=0,min=Infinity,max=-Infinity;
    for(const r of table.slice(1)){const v=r[col]?.trim();if(!v||!Number.isFinite(Number(v)))continue;const n=Number(v);sum+=n;count++;min=Math.min(min,n);max=Math.max(max,n);}
    return {sum,count,min,max};},[table,col]);
  async function load(file?:File){
    const key=++generation.current;setError('');setName('');setTable([]);setCol(0);
    if(!file)return;
    try{if(file.size>2*1024*1024)throw Error('请选择 2 MiB 以内的文件。');
      const raw=new TextDecoder('utf-8',{fatal:true}).decode(await file.arrayBuffer());if(key!==generation.current)return;
      if(raw.includes('\0'))throw Error('请选择 UTF-8 文本文件。');
      setTable(csvTable(raw));setName(file.name);
    }catch(e){if(key===generation.current)setError(e instanceof Error?e.message:String(e));}
  }
  return <section className="scene-page"><div className="scene-title"><div><h1>数据分析</h1><p>导入 CSV 查看数据预览，选择数值列计算统计指标。</p></div></div>
    <div className="file-panel"><label className="file-pick"><strong>{name|| '选择 CSV 文件'}</strong><span>UTF-8 · 最大 2 MiB · 在浏览器内处理</span><input type="file" accept=".csv,text/csv" onChange={e=>void load(e.target.files?.[0])}/></label>
    {error&&<p role="alert">{error}</p>}
    {table.length>0&&<><label>统计字段<select value={col} onChange={e=>setCol(Number(e.target.value))}>{table[0].map((h,i)=><option key={i} value={i}>{h||'列 '+(i+1)}</option>)}</select></label>
      <p>{table.length-1} 行 · {table[0].length} 列 · {stats.count} 个数值</p>
      {stats.count>0&&<dl className="stat-line">{[['总和',stats.sum],['平均值',stats.sum/stats.count],['最小值',stats.min],['最大值',stats.max]].map(([k,v])=><div key={k}><dt>{k}</dt><dd>{Number(v).toLocaleString('zh-CN',{maximumFractionDigits:4})}</dd></div>)}</dl>}
      <div className="data-table"><table><caption>前 50 行</caption><thead><tr>{table[0].map((h,i)=><th key={i}>{h}</th>)}</tr></thead><tbody>{table.slice(1,51).map((r,i)=><tr key={i}>{r.map((v,j)=><td key={j}>{v}</td>)}</tr>)}</tbody></table></div></>}
    </div></section>;
}
