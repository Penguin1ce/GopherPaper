"use client";

import { Loader2, Plus, Send } from "lucide-react";
import { memo, useEffect, useRef, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Textarea } from "@/components/ui/textarea";
import { figureUrl } from "@/lib/gopherpaper/api";
import { useApp } from "@/lib/gopherpaper/store";
import type { Message, Reference } from "@/lib/gopherpaper/types";
import { formatTime, intentLabel, paperTitle, sessionTitle } from "@/lib/gopherpaper/utils";
import { cn } from "@/lib/utils";
import { Empty, useGuard } from "./app-ui";
import { Markdown } from "./markdown";

function extractSources(meta?: Record<string, unknown>): Reference[] {
  if (!meta) return [];
  const raw = meta.sources;
  if (!Array.isArray(raw)) return [];
  return raw as Reference[];
}

function buildFigureMap(refs: Reference[]): Record<string, string> {
  const map: Record<string, string> = {};
  for (const r of refs) {
    if (r.block_type === "image" && r.img_name && r.doc_id) map[r.img_name] = r.doc_id;
  }
  return map;
}

function Sources({ refs }: { refs: Reference[] }) {
  if (refs.length === 0) return null;
  return (
    <div className="mt-4 rounded-xl border border-sienna/25 bg-sienna/[0.04] p-3">
      <div className="mb-2 text-xs font-semibold tracking-wide text-sienna">
        引用出处 · {refs.length}
      </div>
      <ol className="space-y-2">
        {refs.map((r, i) => {
          const name = r.source_file || r.source_uri || `片段 ${i + 1}`;
          const scope = r.knowledge_scope === "public" ? "基础库" : "我的论文";
          const isImage = r.block_type === "image" && !!r.img_name && !!r.doc_id;
          const src = isImage ? figureUrl(r.doc_id!, r.img_name!) : "";
          return (
            <li key={i} className="flex items-center gap-2 text-xs">
              <span className="flex size-5 shrink-0 items-center justify-center rounded-full bg-sienna/12 font-mono text-[10px] text-sienna">
                {i + 1}
              </span>
              {isImage ? (
                <a className="shrink-0" href={src} target="_blank" rel="noreferrer">
                  <img className="size-10 rounded-md border object-cover" src={src} alt={name} loading="lazy" />
                </a>
              ) : (
                <span className="min-w-0 flex-1 truncate" title={name}>
                  {name}
                </span>
              )}
              {typeof r.page_no === "number" && r.page_no > 0 && (
                <Badge variant="secondary" className="rounded-full font-normal">
                  p.{r.page_no}
                </Badge>
              )}
              <Badge variant="outline" className="rounded-full font-normal">
                {scope}
              </Badge>
            </li>
          );
        })}
      </ol>
    </div>
  );
}

const MessageBubble = memo(function MessageBubble({ message }: { message: Message }) {
  const isAssistant = message.role === "assistant";
  const refs = isAssistant ? extractSources(message.meta) : [];
  const figures = isAssistant ? buildFigureMap(refs) : undefined;
  return (
    <article className={cn("flex flex-col gap-1.5", isAssistant ? "items-start" : "items-end")}>
      {isAssistant ? (
        // 助教答案直接铺在版面上(Perplexity 式),不套气泡框,留白拆开
        <div className="w-full">
          <Markdown figures={figures}>{message.content}</Markdown>
          {refs.length > 0 && <Sources refs={refs} />}
        </div>
      ) : (
        <div className="max-w-[80%] rounded-2xl bg-primary px-4 py-2.5 text-primary-foreground">
          <p className="whitespace-pre-wrap text-sm leading-6">{message.content}</p>
        </div>
      )}
      <div className="flex items-center gap-2 px-0.5 text-xs text-muted-foreground">
        <span>{isAssistant ? "助教" : "我"}</span>
        {isAssistant && message.intent && (
          <Badge variant="secondary" className="h-5 rounded-full px-2 font-normal">
            {intentLabel(message.intent)}
          </Badge>
        )}
        {message.created_at && <span>{formatTime(message.created_at)}</span>}
      </div>
    </article>
  );
});

function ToolStatus({ note }: { note: string }) {
  if (!note) return null;
  return (
    <div className="mb-2 inline-flex items-center gap-2 rounded-full border border-sienna/30 bg-sienna/10 px-3 py-1 text-xs font-medium text-sienna">
      <Loader2 className="size-3 animate-spin" />
      {note}
    </div>
  );
}

const PROMPT_HINTS = [
  "这篇论文的核心贡献是什么?",
  "用了哪些数据集和评测指标?",
  "方法部分的整体流程是怎样的?",
];

export function ChatPane() {
  const {
    messages,
    papers,
    activeSession,
    activePaper,
    sending,
    toolNote,
    sendMessage,
    createSession,
  } = useApp();
  const guard = useGuard();
  const [input, setInput] = useState("");
  const listRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = listRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [messages, sending, toolNote]);

  const submit = () => {
    const q = input.trim();
    if (!q || sending) return;
    setInput("");
    guard(() => sendMessage(q));
  };

  const newSession = () =>
    guard(async () => {
      const title = activePaper ? `${paperTitle(activePaper)} 问答` : "新会话";
      await createSession(title, activePaper?.id);
    });

  const sessionPaper = activeSession?.paper_id
    ? papers.find((p) => p.id === activeSession.paper_id) || null
    : null;
  const hasSession = Boolean(activeSession);
  const hasContent = hasSession || messages.length > 0;

  // 论文标题已在右栏头部展示, 这里只给会话上下文, 避免与上方标题重复
  const sessionScopedToActive =
    !!sessionPaper && !!activePaper && sessionPaper.id === activePaper.id;
  const chatContextLabel = !activeSession
    ? activePaper
      ? "向当前论文提问, 自动创建会话"
      : "选择论文或直接提问创建会话"
    : sessionScopedToActive
      ? "围绕当前论文的问答"
      : sessionPaper
        ? `围绕《${paperTitle(sessionPaper)}》`
        : sessionTitle(activeSession);

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="mx-auto flex w-full max-w-3xl shrink-0 items-center justify-between gap-3 px-6 pt-4">
        <span className="min-w-0 truncate text-xs text-muted-foreground">{chatContextLabel}</span>
        <Button type="button" variant="ghost" size="sm" onClick={newSession}>
          <Plus className="size-4" />
          新建
        </Button>
      </div>
      <ScrollArea className="min-h-0 flex-1">
        <div ref={listRef} className="mx-auto flex max-h-full w-full max-w-3xl flex-col gap-6 px-6 py-5">
          {!hasContent ? (
            <div className="mx-auto flex min-h-[24rem] w-full max-w-xl flex-col justify-center gap-4">
              <Empty
                title="准备开始论文问答"
                text={
                  activePaper
                    ? `围绕「${paperTitle(activePaper)}」直接提问，系统会自动创建会话。`
                    : "先在左侧选择一篇论文，或直接输入问题。"
                }
              />
              <div className="flex flex-wrap justify-center gap-2">
                {PROMPT_HINTS.map((h) => (
                  <Button key={h} type="button" variant="outline" onClick={() => setInput(h)}>
                    {h}
                  </Button>
                ))}
              </div>
            </div>
          ) : (
            <>
              {messages.length === 0 && (
                <div className="mx-auto w-full max-w-xl">
                  <Empty title="这个会话还没有消息" text="在下方输入问题，开始第一轮论文问答。" />
                </div>
              )}
              {messages.map((m) => (
                <MessageBubble key={String(m.id)} message={m} />
              ))}
              {sending && !messages.some((m) => m.streaming) && (
                <article className="flex items-start">
                  <div className="inline-flex items-center gap-2 rounded-xl border bg-card px-4 py-3 text-sm text-muted-foreground">
                    <Loader2 className="size-4 animate-spin" />
                    助教正在检索并作答…
                  </div>
                </article>
              )}
            </>
          )}
        </div>
      </ScrollArea>
      <form
        className="shrink-0 px-6 pb-5 pt-2"
        onSubmit={(e) => {
          e.preventDefault();
          submit();
        }}
      >
        <div className="mx-auto w-full max-w-3xl">
        <ToolStatus note={toolNote} />
        <div className="flex items-end gap-2 rounded-[1.625rem] border border-border bg-card py-1.5 pl-2 pr-1.5 shadow-sm transition-[border-color,box-shadow] focus-within:border-ring/50 focus-within:shadow-md">
          <Textarea
            rows={1}
            value={input}
            placeholder="例如：这篇论文的核心贡献是什么?"
            disabled={sending}
            className="max-h-44 min-h-9 resize-none overflow-y-auto border-0 bg-transparent px-3 py-1.5 leading-6 shadow-none focus-visible:border-transparent focus-visible:ring-0"
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                submit();
              }
            }}
          />
          <Button
            type="submit"
            size="icon"
            className="size-9 shrink-0 rounded-full"
            disabled={sending || !input.trim()}
            title="发送"
          >
            {sending ? <Loader2 className="size-4 animate-spin" /> : <Send className="size-4" />}
          </Button>
        </div>
        </div>
      </form>
    </div>
  );
}
