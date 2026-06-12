import ReactMarkdown, {
  type Components,
  defaultUrlTransform,
} from "react-markdown";
import { QRCodeSVG } from "qrcode.react";
import remarkGfm from "remark-gfm";

import { figureUrl } from "../api";

const FIGURE_SCHEME = "figure://";

function decodeHTML(value: string): string {
  return value
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&nbsp;/g, " ");
}

function stripHTML(value: string): string {
  return decodeHTML(value.replace(/<[^>]*>/g, "")).trim();
}

function escapeLinkText(value: string): string {
  return value.replace(/\\/g, "\\\\").replace(/\[/g, "\\[").replace(/\]/g, "\\]");
}

function escapeLinkTarget(value: string): string {
  return value.replace(/\(/g, "%28").replace(/\)/g, "%29");
}

function isAllowedRichURL(value: string): boolean {
  const url = decodeHTML(value).trim();
  if (!url) return false;
  try {
    const parsed = new URL(url);
    return ["http:", "https:", "weixin:"].includes(parsed.protocol);
  } catch {
    return false;
  }
}

function normalizeRichMarkdown(value: string): string {
  return value
    .replace(
      /<a\s+[^>]*href\s*=\s*(["'])(.*?)\1[^>]*>([\s\S]*?)<\/a>/gi,
      (raw, _quote, href: string, label: string) => {
        if (!isAllowedRichURL(href)) return raw;
        const text = stripHTML(label) || decodeHTML(href).trim();
        return `[${escapeLinkText(text)}](${escapeLinkTarget(decodeHTML(href).trim())})`;
      },
    )
    .replace(
      /<img\s+[^>]*src\s*=\s*(["'])(.*?)\1[^>]*>/gi,
      (raw, _quote, src: string) => {
        if (!isAllowedRichURL(src)) return raw;
        const alt = /alt\s*=\s*(["'])(.*?)\1/i.exec(raw)?.[2] || "图片";
        return `![${escapeLinkText(stripHTML(alt) || "图片")}](${escapeLinkTarget(
          decodeHTML(src).trim(),
        )})`;
      },
    );
}

function isWeixinURL(value: string): boolean {
  return value.startsWith("weixin://");
}

function isQRCodeURL(value: string): boolean {
  try {
    const url = new URL(value);
    return /qrcode/i.test(url.pathname) || /qrcode/i.test(url.search);
  } catch {
    return false;
  }
}

function isImageURL(value: string): boolean {
  return /\.(png|jpe?g|gif|webp|svg)(?:[?#]|$)/i.test(value) || isQRCodeURL(value);
}

// Markdown 渲染助教回答与研读报告正文。
// 默认不解析裸 HTML(react-markdown 行为),RAG 注入的 PDF 内容也无法 XSS。
// figures 把 figure://文件名 映射到所属论文 docId:模型在正文用 ![](figure://名) 插图,
// 这里解析成带 token 的真实取图 URL,让图片内联出现在回答对应位置。
export function Markdown({
  children,
  figures,
  richLinks = false,
}: {
  children: string;
  figures?: Record<string, string>;
  richLinks?: boolean;
}) {
  const components: Components = {
    a({ href, children }) {
      const url = typeof href === "string" ? href : "";
      // weixin 深链桌面浏览器点不开,其内容即微信支付码,本地渲染成二维码供手机扫,
      // 链接本身保留给移动端直接拉起微信。
      if (richLinks && url && isWeixinURL(url)) {
        return (
          <span className="rich-pay-card">
            <span className="rich-link-kicker">微信扫码支付</span>
            <span className="rich-pay-qr">
              <QRCodeSVG value={url} size={180} marginSize={2} />
            </span>
            <a className="rich-pay-button" href={url}>
              {children}
            </a>
          </span>
        );
      }
      if (richLinks && url && isQRCodeURL(url)) {
        return (
          <a className="rich-qr-card" href={url} target="_blank" rel="noopener noreferrer">
            <span className="rich-link-kicker">扫码支付</span>
            <img className="rich-qr-image" src={url} alt="支付二维码" loading="lazy" />
            <span className="rich-link-action">打开二维码链接</span>
          </a>
        );
      }
      if (richLinks && url && isImageURL(url)) {
        return (
          <a className="rich-image-card" href={url} target="_blank" rel="noopener noreferrer">
            <img className="rich-image-preview" src={url} alt="" loading="lazy" />
            <span className="rich-link-action">打开图片</span>
          </a>
        );
      }
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
          url.startsWith(FIGURE_SCHEME) || (richLinks && isAllowedRichURL(url))
            ? url
            : defaultUrlTransform(url)
        }
      >
        {richLinks ? normalizeRichMarkdown(children ?? "") : children ?? ""}
      </ReactMarkdown>
    </div>
  );
}
