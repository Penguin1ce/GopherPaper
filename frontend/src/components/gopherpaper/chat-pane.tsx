"use client";

import { ArrowUp, BookOpenText, ChevronDown, Loader2 } from "lucide-react";
import { memo, useEffect, useMemo, useRef, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Textarea } from "@/components/ui/textarea";
import { figureUrl } from "@/lib/gopherpaper/api";
import { PAPER_CHAT_DRAFT_KEY } from "@/lib/gopherpaper/navigation-drafts";
import {
  referenceReaderHref,
  referencesUsedBySourceTags,
  sourceTagReaderHref,
} from "@/lib/gopherpaper/source-links";
import { useApp } from "@/lib/gopherpaper/store";
import type { Message, PaperFlow, Reference } from "@/lib/gopherpaper/types";
import {
  formatTime,
  intentLabel,
  messagePlan,
  paperTitle,
  processPlanSteps,
} from "@/lib/gopherpaper/utils";
import { cn } from "@/lib/utils";
import { Empty, useGuard } from "./app-ui";
import { Markdown } from "./markdown";
import { PaperFlowCard } from "./paper-flow-card";
import { ProcessTrace } from "./process-trace";
import { ToolTrace } from "./tool-trace";

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

function referenceName(ref: Reference, index: number) {
  return ref.source_file || ref.source_uri || `片段 ${index + 1}`;
}

function referenceScope(ref: Reference) {
  return ref.knowledge_scope === "public" ? "基础库" : "我的论文";
}

function Sources({
  refs,
  fallbackPaperID,
  allowedPaperIDs,
}: {
  refs: Reference[];
  fallbackPaperID?: string;
  allowedPaperIDs?: Set<string>;
}) {
  const [open, setOpen] = useState(false);

  if (refs.length === 0) return null;
  const firstName = referenceName(refs[0], 0);
  const scopeSummary = Array.from(new Set(refs.map(referenceScope))).join(" / ");

  return (
    <Collapsible
      open={open}
      onOpenChange={setOpen}
      className="mt-3 overflow-hidden rounded-xl border bg-muted/25"
    >
      <CollapsibleTrigger
        render={
          <button
            type="button"
            className="flex w-full items-center gap-2 px-3 py-2 text-left text-xs transition-colors hover:bg-muted/45 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40"
          />
        }
      >
        <BookOpenText className="size-3.5 shrink-0 text-primary" aria-hidden />
        <span className="min-w-0 flex-1 truncate">
          <span className="font-semibold text-foreground/85">引用来源 · {refs.length}</span>
          {!open && (
            <span className="ml-2 font-normal text-muted-foreground">
              {[scopeSummary, `${firstName}${refs.length > 1 ? ` 等 ${refs.length} 条` : ""}`]
                .filter(Boolean)
                .join(" · ")}
            </span>
          )}
        </span>
        <ChevronDown
          className={cn("size-4 shrink-0 text-muted-foreground transition-transform", open && "rotate-180")}
          aria-hidden
        />
      </CollapsibleTrigger>
      <CollapsibleContent>
        <ol className="max-h-72 space-y-2 overflow-y-auto border-t px-3 py-2.5">
          {refs.map((r, i) => {
            const name = referenceName(r, i);
            const scope = referenceScope(r);
            const isImage = r.block_type === "image" && !!r.img_name && !!r.doc_id;
            const src = isImage ? figureUrl(r.doc_id!, r.img_name!) : "";
            const readerHref = referenceReaderHref(r, fallbackPaperID, allowedPaperIDs);
            return (
              <li key={i} className="flex items-start gap-2 rounded-lg px-1 py-1 text-xs">
                <span className="flex size-5 shrink-0 items-center justify-center rounded-full bg-sienna/12 font-mono text-[10px] text-sienna">
                  {i + 1}
                </span>
                {isImage && (
                  <a className="shrink-0" href={src} target="_blank" rel="noreferrer">
                    <img className="size-10 rounded-md border object-cover" src={src} alt={name} loading="lazy" />
                  </a>
                )}
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-medium text-foreground/80" title={name}>
                    {name}
                  </span>
                  <span className="mt-1 flex flex-wrap gap-1.5">
                    {typeof r.page_no === "number" && r.page_no > 0 && (
                      readerHref ? (
                        <Badge
                          variant="secondary"
                          className="rounded-full font-normal hover:bg-secondary/80"
                          render={<a href={readerHref} target="_blank" rel="noopener noreferrer" />}
                        >
                          p.{r.page_no}
                        </Badge>
                      ) : (
                        <Badge variant="secondary" className="rounded-full font-normal">
                          p.{r.page_no}
                        </Badge>
                      )
                    )}
                    <Badge variant="outline" className="rounded-full font-normal">
                      {scope}
                    </Badge>
                    {isImage && (
                      <Badge variant="secondary" className="rounded-full font-normal">
                        图像
                      </Badge>
                    )}
                  </span>
                </span>
              </li>
            );
          })}
        </ol>
      </CollapsibleContent>
    </Collapsible>
  );
}

const MessageBubble = memo(function MessageBubble({
  message,
  fallbackPaperID,
  allowedPaperIDs,
}: {
  message: Message;
  fallbackPaperID?: string;
  allowedPaperIDs?: Set<string>;
}) {
  const isAssistant = message.role === "assistant";
  const allRefs = isAssistant ? extractSources(message.meta) : [];
  const refs = isAssistant ? referencesUsedBySourceTags(message.content, allRefs) : [];
  const figures = isAssistant ? buildFigureMap(allRefs) : undefined;
  const steps = isAssistant ? messagePlan(message) : [];
  const processSteps = processPlanSteps(steps);
  const sourceHref = (label: string) =>
    sourceTagReaderHref(label, allRefs, fallbackPaperID, allowedPaperIDs);
  return (
    <article className={cn("flex flex-col gap-1.5", isAssistant ? "items-start" : "items-end")}>
      {isAssistant ? (
        // 助教答案直接铺在版面上(Perplexity 式),不套气泡框,留白拆开
        <div className="w-full">
          {processSteps.length > 0 && (
            <ProcessTrace steps={processSteps} live={!!message.streaming} />
          )}
          <ToolTrace steps={steps} live={!!message.streaming} />
          <Markdown figures={figures} sourceHref={sourceHref}>{message.content}</Markdown>
          {(message.flow ?? (message.meta?.flow as PaperFlow | undefined)) && (
            <PaperFlowCard flow={(message.flow ?? message.meta?.flow) as PaperFlow} />
          )}
          {refs.length > 0 && (
            <Sources
              refs={refs}
              fallbackPaperID={fallbackPaperID}
              allowedPaperIDs={allowedPaperIDs}
            />
          )}
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

const PROMPT_HINTS = [
  "这篇论文的核心贡献是什么?",
  "用了哪些数据集和评测指标?",
  "方法部分的整体流程是怎样的?",
];

function PaperChatComposer({
  input,
  sending,
  variant = "bottom",
  onInputChange,
  onSubmit,
}: {
  input: string;
  sending: boolean;
  variant?: "bottom" | "center";
  onInputChange: (value: string) => void;
  onSubmit: () => void;
}) {
  const hasDraft = input.trim().length > 0;

  return (
    <form
      className={cn(
        variant === "center" ? "w-full" : "shrink-0 px-6 pb-5 pt-2",
      )}
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit();
      }}
    >
      <div className={cn("mx-auto w-full", variant === "center" ? "max-w-2xl" : "max-w-3xl")}>
        <div
          className={cn(
            "flex gap-2 border border-border bg-card py-1.5 pl-2 pr-1.5 shadow-sm transition-[border-color,box-shadow] focus-within:border-ring/50 focus-within:shadow-md",
            variant === "center" ? "items-center" : "items-end",
            variant === "center" ? "rounded-[1.5rem]" : "rounded-[1.625rem]",
          )}
        >
          <Textarea
            rows={1}
            value={input}
            placeholder={variant === "center" ? "问小文鸮" : ""}
            disabled={sending}
            className={cn(
              "max-h-44 min-h-9 resize-none overflow-y-auto border-0 bg-transparent px-3 py-1.5 leading-6 shadow-none focus-visible:border-transparent focus-visible:ring-0",
              variant === "center" && "min-h-10 py-2.5 pl-4 text-sm leading-5",
            )}
            onChange={(e) => onInputChange(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                onSubmit();
              }
            }}
          />
          {(hasDraft || sending) && (
            <Button
              type="submit"
              size="icon"
              className="size-9 shrink-0 rounded-full"
              disabled={sending || !hasDraft}
              title={sending ? "正在发送" : "发送"}
              aria-label={sending ? "正在发送" : "发送"}
            >
              {sending ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                <ArrowUp className="size-4 stroke-[2.6]" />
              )}
            </Button>
          )}
        </div>
      </div>
    </form>
  );
}

export function ChatPane() {
  const { user, messages, activeSession, activePaper, papers, sending, toolNote, sendMessage } = useApp();
  const guard = useGuard();
  const [input, setInput] = useState("");
  const bottomRef = useRef<HTMLDivElement>(null);
  const fallbackPaperID = activeSession?.paper_id || activePaper?.id || "";
  const allowedPaperIDs = useMemo(
    () => (papers.length > 0 ? new Set(papers.map((paper) => paper.id)) : undefined),
    [papers],
  );

  useEffect(() => {
    try {
      const draft = sessionStorage.getItem(PAPER_CHAT_DRAFT_KEY);
      if (!draft) return;
      sessionStorage.removeItem(PAPER_CHAT_DRAFT_KEY);
      setInput(draft);
    } catch {
      // 严格隐私模式下 sessionStorage 可能不可用。
    }
  }, []);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "instant" });
  }, [messages, sending, toolNote]);

  const submit = () => {
    const q = input.trim();
    if (!q || sending) return;
    setInput("");
    guard(() => sendMessage(q));
  };

  const hasContent = Boolean(activeSession) || messages.length > 0;
  const emptyConversation = messages.length === 0 && !sending;
  const displayName = user?.name || user?.student_id || "同学";

  return (
    <div className="flex h-full min-h-0 flex-col">
      <ScrollArea className="min-h-0 flex-1">
        <div
          className={cn(
            "mx-auto flex max-h-full w-full flex-col px-6 py-5",
            emptyConversation ? "max-w-4xl" : "max-w-3xl gap-6",
          )}
        >
          {emptyConversation ? (
            <div className="mx-auto flex min-h-[calc(100dvh-9rem)] w-full max-w-3xl flex-col justify-center gap-5 pb-16">
              <div className="space-y-2 text-center">
                <h1 className="text-xl font-medium leading-8 tracking-tight text-foreground sm:text-2xl">
                  {displayName}，你好
                </h1>
                {activePaper && (
                  <div className="mx-auto max-w-md truncate text-xs text-muted-foreground/75">
                    {paperTitle(activePaper)}
                  </div>
                )}
              </div>
              <PaperChatComposer
                input={input}
                sending={sending}
                variant="center"
                onInputChange={setInput}
                onSubmit={submit}
              />
              <div className="flex flex-wrap justify-center gap-2">
                {PROMPT_HINTS.map((h) => (
                  <Button
                    key={h}
                    type="button"
                    variant="outline"
                    size="sm"
                    className="h-8 rounded-full px-3 text-xs font-normal text-muted-foreground"
                    onClick={() => setInput(h)}
                  >
                    {h}
                  </Button>
                ))}
              </div>
            </div>
          ) : (
            <>
              {!hasContent && (
                <div className="mx-auto w-full max-w-xl">
                  <Empty title="这个会话还没有消息" text="在下方输入问题，开始第一轮论文问答。" />
                </div>
              )}
              {messages.map((m) => (
                <MessageBubble
                  key={String(m.id)}
                  message={m}
                  fallbackPaperID={fallbackPaperID}
                  allowedPaperIDs={allowedPaperIDs}
                />
              ))}
              {sending && !messages.some((m) => m.streaming) && (
                <article className="flex items-start">
                  <div className="inline-flex items-center gap-2 rounded-xl border bg-card px-4 py-3 text-sm text-muted-foreground">
                    <Loader2 className="size-4 animate-spin" />
                    小文鸮正在检索并作答…
                  </div>
                </article>
              )}
            </>
          )}
          <div ref={bottomRef} />
        </div>
      </ScrollArea>
      {!emptyConversation && (
        <PaperChatComposer
          input={input}
          sending={sending}
          onInputChange={setInput}
          onSubmit={submit}
        />
      )}
    </div>
  );
}
