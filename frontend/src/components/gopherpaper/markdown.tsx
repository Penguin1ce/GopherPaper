"use client";

import { Check, Copy } from "lucide-react";
import { type ReactNode, useRef, useState } from "react";
import ReactMarkdown, {
  type Components,
  defaultUrlTransform,
} from "react-markdown";
import { QRCodeSVG } from "qrcode.react";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import rehypeKatex from "rehype-katex";
import "katex/dist/katex.min.css";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { figureUrl } from "@/lib/gopherpaper/api";

const FIGURE_SCHEME = "figure://";

async function copyText(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch {
  }
  try {
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.style.position = "fixed";
    ta.style.opacity = "0";
    document.body.appendChild(ta);
    ta.select();
    const ok = document.execCommand("copy");
    document.body.removeChild(ta);
    return ok;
  } catch {
    return false;
  }
}

function CodeBlock({ children }: { children?: ReactNode }) {
  const ref = useRef<HTMLPreElement>(null);
  const [copied, setCopied] = useState(false);
  const onCopy = async () => {
    const ok = await copyText(ref.current?.textContent ?? "");
    if (!ok) return;
    setCopied(true);
    setTimeout(() => setCopied(false), 1600);
  };
  return (
    <div className="group relative my-4 overflow-hidden rounded-lg border bg-muted/40">
      <Button
        type="button"
        variant="secondary"
        size="sm"
        className="absolute right-2 top-2 h-7 gap-1 opacity-0 transition group-hover:opacity-100"
        onClick={onCopy}
        aria-label="复制代码"
      >
        {copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
        {copied ? "已复制" : "复制"}
      </Button>
      <pre ref={ref} className="overflow-x-auto p-4 pr-24 text-xs leading-relaxed">
        {children}
      </pre>
    </div>
  );
}

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

function looksLikeMath(value: string): boolean {
  const trimmed = value.trim();
  return (
    /\\[a-zA-Z]+|\\[{}]|[_^=<>+\-*/]|[\d)]\s*[,，]\s*[\d(]/.test(trimmed) ||
    /^[A-Za-z][A-Za-z0-9']*$/.test(trimmed)
  );
}

function normalizeMathBody(value: string): string {
  return value.replace(
    /\\(notin|in)\s*\{([^{}]+)\}/g,
    (_raw, op: string, body: string) => `\\${op} \\{${body}\\}`,
  );
}

function normalizeMathText(value: string): string {
  return value
    .replace(/\\\$([^$\n]+?)(?:\\\$|\$)/g, (raw, body: string) => {
      if (!looksLikeMath(body)) return raw;
      return `$${normalizeMathBody(body)}$`;
    })
    .replace(/\$([^$\n]+?)\\\$/g, (raw, body: string) => {
      if (!looksLikeMath(body)) return raw;
      return `$${normalizeMathBody(body)}$`;
    })
    .replace(/\$\$([\s\S]+?)\$\$/g, (_raw, body: string) => {
      return `\n\n$$\n${normalizeMathBody(body.trim())}\n$$\n\n`;
    })
    .replace(/\\\[([\s\S]+?)\\\]/g, (raw, body: string) => {
      if (!looksLikeMath(body)) return raw;
      return `\n\n$$\n${normalizeMathBody(body.trim())}\n$$\n\n`;
    })
    .replace(/\\\(([\s\S]+?)\\\)/g, (raw, body: string) => {
      if (!looksLikeMath(body)) return raw;
      return `$${normalizeMathBody(body)}$`;
    })
    .replace(
      /(^|[^\\$])\$([^$\n]+?)\$(?!\$)/g,
      (raw, prefix: string, body: string) => {
        if (!looksLikeMath(body)) return raw;
        return `${prefix}$${normalizeMathBody(body)}$`;
      },
    );
}

function normalizeMathMarkdown(value: string): string {
  let out = "";
  let pos = 0;
  while (pos < value.length) {
    const tickStart = value.indexOf("`", pos);
    if (tickStart < 0) {
      out += normalizeMathText(value.slice(pos));
      break;
    }
    out += normalizeMathText(value.slice(pos, tickStart));

    let tickCount = 1;
    while (value[tickStart + tickCount] === "`") tickCount += 1;
    const ticks = "`".repeat(tickCount);
    const tickEnd = value.indexOf(ticks, tickStart + tickCount);
    if (tickEnd < 0) {
      out += value.slice(tickStart);
      break;
    }
    out += value.slice(tickStart, tickEnd + tickCount);
    pos = tickEnd + tickCount;
  }
  return out;
}

function isFenceLine(line: string): boolean {
  return /^ {0,3}(```|~~~)/.test(line);
}

function isDisplayMathBoundary(line: string): boolean {
  const trimmed = line.trim();
  return trimmed === "$$" || trimmed === "\\]";
}

function isIndentedProse(line: string): boolean {
  if (!/^ {4,}\S/.test(line)) return false;
  const trimmed = line.trim();
  if (/^([-*+]|\d+\.)\s/.test(trimmed)) return false;
  if (
    /^(import|export|const|let|var|function|class|return|if|for|while|switch|try|catch|type|interface|func|package|select|insert|update|delete|create)\b/i.test(
      trimmed,
    )
  ) {
    return false;
  }
  if (/^(\/\/|\/\*|\*\/|<[/!A-Za-z]|[{}[\]();])/.test(trimmed)) return false;
  return /[\u4e00-\u9fff]|[。；，、：？！]/.test(trimmed);
}

function normalizeIndentedProseAfterMath(value: string): string {
  const lines = value.split("\n");
  let inFence = false;
  let lastNonBlank = "";
  let proseBlockAfterMath = false;

  return lines
    .map((line) => {
      if (isFenceLine(line)) {
        inFence = !inFence;
        lastNonBlank = line.trim();
        proseBlockAfterMath = false;
        return line;
      }
      if (inFence) return line;

      const trimmed = line.trim();
      if (!trimmed) {
        proseBlockAfterMath = false;
        return line;
      }

      const shouldDedent =
        isIndentedProse(line) && (isDisplayMathBoundary(lastNonBlank) || proseBlockAfterMath);
      const out = shouldDedent ? line.replace(/^ {4,}/, "") : line;
      lastNonBlank = out.trim();
      proseBlockAfterMath = shouldDedent;
      return out;
    })
    .join("\n");
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

export function Markdown({
  children,
  figures,
  richLinks = false,
  compact = false,
}: {
  children: string;
  figures?: Record<string, string>;
  richLinks?: boolean;
  compact?: boolean;
}) {
  const normalizedChildren = normalizeIndentedProseAfterMath(
    normalizeMathMarkdown(richLinks ? normalizeRichMarkdown(children ?? "") : children ?? ""),
  );
  const components: Components = {
    pre({ children }) {
      return <CodeBlock>{children}</CodeBlock>;
    },
    a({ href, children }) {
      const url = typeof href === "string" ? href : "";
      if (richLinks && url && isWeixinURL(url)) {
        return (
          <span className="my-3 inline-flex flex-col gap-3 rounded-lg border bg-background p-4">
            <span className="text-xs font-medium text-muted-foreground">微信扫码支付</span>
            <span className="rounded-md bg-white p-2">
              <QRCodeSVG value={url} size={180} marginSize={2} />
            </span>
            <a className="text-sm font-medium underline underline-offset-4" href={url}>
              {children}
            </a>
          </span>
        );
      }
      if (richLinks && url && isQRCodeURL(url)) {
        return (
          <a
            className="my-3 inline-flex flex-col gap-3 rounded-lg border bg-background p-4 no-underline"
            href={url}
            target="_blank"
            rel="noopener noreferrer"
          >
            <span className="text-xs font-medium text-muted-foreground">扫码支付</span>
            <img className="size-44 rounded-md bg-white object-contain p-2" src={url} alt="支付二维码" loading="lazy" />
            <span className="text-sm font-medium underline underline-offset-4">打开二维码链接</span>
          </a>
        );
      }
      if (richLinks && url && isImageURL(url)) {
        return (
          <a className="my-3 mx-auto block w-fit max-w-full rounded-lg border bg-background p-2 no-underline" href={url} target="_blank" rel="noopener noreferrer">
            <img className="max-h-80 max-w-full rounded-md" src={url} alt="" loading="lazy" />
            <span className="mt-2 block text-sm font-medium underline underline-offset-4">打开图片</span>
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
      if (richLinks && typeof src === "string" && isQRCodeURL(src)) {
        return (
          <span className="my-3 inline-flex flex-col gap-3 rounded-lg border bg-background p-4">
            <span className="text-xs font-medium text-muted-foreground">扫码支付</span>
            <span className="rounded-md bg-white p-2">
              <img className="size-44 object-contain" src={src} alt={alt || "支付二维码"} loading="lazy" />
            </span>
          </span>
        );
      }
      if (typeof src === "string" && src.startsWith(FIGURE_SCHEME)) {
        const name = decodeURIComponent(src.slice(FIGURE_SCHEME.length));
        const docId = figures?.[name];
        if (!docId) return null;
        const url = figureUrl(docId, name);
        return (
          <a className="my-3 mx-auto block w-fit max-w-full rounded-lg border bg-background p-2" href={url} target="_blank" rel="noreferrer">
            <img className="max-h-96 max-w-full rounded-md" src={url} alt={alt ?? ""} loading="lazy" />
          </a>
        );
      }
      return <img className="my-3 max-w-full rounded-lg border" src={src} alt={alt ?? ""} loading="lazy" />;
    },
  };
  return (
    <div className={cn("gp-markdown", compact && "gp-markdown-compact")}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkMath]}
        rehypePlugins={[[rehypeKatex, { throwOnError: false }]]}
        components={components}
        urlTransform={(url) =>
          url.startsWith(FIGURE_SCHEME) || (richLinks && isAllowedRichURL(url))
            ? url
            : defaultUrlTransform(url)
        }
      >
        {normalizedChildren}
      </ReactMarkdown>
    </div>
  );
}
