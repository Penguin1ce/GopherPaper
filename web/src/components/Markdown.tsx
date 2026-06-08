import ReactMarkdown, {
  type Components,
  defaultUrlTransform,
} from "react-markdown";
import remarkGfm from "remark-gfm";

import { figureUrl } from "../api";

const FIGURE_SCHEME = "figure://";

// Markdown 渲染助教回答与研读报告正文。
// 默认不解析裸 HTML(react-markdown 行为),RAG 注入的 PDF 内容也无法 XSS。
// figures 把 figure://文件名 映射到所属论文 docId:模型在正文用 ![](figure://名) 插图,
// 这里解析成带 token 的真实取图 URL,让图片内联出现在回答对应位置。
export function Markdown({
  children,
  figures,
}: {
  children: string;
  figures?: Record<string, string>;
}) {
  const components: Components = {
    a({ href, children }) {
      return (
        <a href={href} target="_blank" rel="noopener noreferrer">
          {children}
        </a>
      );
    },
    img({ src, alt }) {
      if (typeof src === "string" && src.startsWith(FIGURE_SCHEME)) {
        const name = decodeURIComponent(src.slice(FIGURE_SCHEME.length));
        const docId = figures?.[name];
        if (!docId) return null; // 找不到对应图,丢弃占位不渲染
        const url = figureUrl(docId, name);
        return (
          <a
            className="inline-figure-link"
            href={url}
            target="_blank"
            rel="noreferrer"
          >
            <img className="inline-figure" src={url} alt={alt ?? ""} loading="lazy" />
          </a>
        );
      }
      return <img src={src} alt={alt ?? ""} loading="lazy" />;
    },
  };
  return (
    <div className="md">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={components}
        urlTransform={(url) =>
          url.startsWith(FIGURE_SCHEME) ? url : defaultUrlTransform(url)
        }
      >
        {children ?? ""}
      </ReactMarkdown>
    </div>
  );
}
