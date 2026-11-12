import {memo,useRef,useState,type ComponentPropsWithoutRef} from 'react';
import Markdown,{type Components} from 'react-markdown';
import remarkGfm from 'remark-gfm';
import rehypeHighlight from 'rehype-highlight';
import './chat-markdown.css';

function CodeBlock({children}:ComponentPropsWithoutRef<'pre'>){
  const pre=useRef<HTMLPreElement>(null);
  const [status,setStatus]=useState<'复制'|'已复制'|'复制失败'>('复制');
  async function copy(){
    try{await navigator.clipboard.writeText(pre.current?.textContent??'');setStatus('已复制');}
    catch{setStatus('复制失败');}
  }
  return <div data-component="markdown-code" onPointerLeave={()=>setStatus('复制')}>
    <pre ref={pre} className="highlights">{children}</pre>
    <button type="button" data-slot="markdown-copy-button" aria-label="复制代码" onClick={copy}>{status}</button>
  </div>;
}
const components:Components={
  pre:({children})=><CodeBlock>{children}</CodeBlock>,
  table:({children})=><div data-component="markdown-table"><table>{children}</table></div>,
  a:({href,children})=>href?<a href={href} target="_blank" rel="noopener noreferrer">{children}</a>:<span>{children}</span>,
  // A generated reply may reference an intranet asset. Open it only on the reader's request.
  img:({src,alt})=>typeof src==='string'?<a href={src} target="_blank" rel="noopener noreferrer">{alt||'查看图片'}</a>:<span>{alt}</span>,
};
export const ChatMarkdown=memo(function ChatMarkdown({text}:{text:string}){
  return <div data-component="markdown"><Markdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeHighlight]} components={components} skipHtml>{text}</Markdown></div>;
});
