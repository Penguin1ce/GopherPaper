import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";

// 链接强制新标签打开,并断开 opener 引用。
const components: Components = {
  a({ href, children }) {
    return (
      <a href={href} target="_blank" rel="noopener noreferrer">
        {children}
      </a>
    );
  },
};

// Markdown 渲染助教回答与研读报告正文。
// 默认不解析裸 HTML(react-markdown 行为),RAG 注入的 PDF 内容也无法 XSS。
export function Markdown({ children }: { children: string }) {
  return (
    <div className="md">
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
        {children ?? ""}
      </ReactMarkdown>
    </div>
  );
}
