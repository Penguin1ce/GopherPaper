"use client";

import {
  AlertTriangle,
  ArrowLeft,
  Coffee,
  FileText,
  Loader2,
  Plus,
  Send,
  Trash2,
} from "lucide-react";
import Link from "next/link";
import { memo, useCallback, useEffect, useRef, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverTitle,
  PopoverTrigger,
} from "@/components/ui/popover";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Textarea } from "@/components/ui/textarea";
import * as api from "@/lib/gopherpaper/api";
import type {
  Message,
  PaperDeleteConfirmPayload,
  PlanStep,
  Session,
} from "@/lib/gopherpaper/types";
import { formatTime, sessionTitle } from "@/lib/gopherpaper/utils";
import { cn } from "@/lib/utils";
import { Empty } from "@/components/gopherpaper/app-ui";
import { Markdown } from "@/components/gopherpaper/markdown";
import { WorkspaceFrame, WorkspacePanel } from "@/components/gopherpaper/workspace-frame";
import {
  Plan,
  PlanAction,
  PlanContent,
  PlanHeader,
  PlanTitle,
  PlanTrigger,
} from "@/components/ai-elements/plan";

const AUTH_KEY = "gopherpaper.auth";
const LUCKIN_KEY = "gopherpaper.luckin";
const LUCKIN_TTL_DAYS = 30;
const LUCKIN_HEADER = "X-Luckin-Token";
const DELETE_CONFIRM_HEADER = "X-GopherPaper-Delete-Confirm";
const AGENT_TYPE = "pioneer";

function loadToken(): string {
  if (typeof window === "undefined") return "";
  try {
    const raw = localStorage.getItem(AUTH_KEY);
    if (!raw) return "";
    return (JSON.parse(raw) as { token?: string }).token || "";
  } catch {
    return "";
  }
}

interface LuckinCred {
  token: string;
  savedAt: number;
}

function loadLuckin(): LuckinCred | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = localStorage.getItem(LUCKIN_KEY);
    if (!raw) return null;
    const cred = JSON.parse(raw) as LuckinCred;
    return cred.token ? cred : null;
  } catch {
    return null;
  }
}

function luckinDaysLeft(cred: LuckinCred): number {
  const elapsed = (Date.now() - cred.savedAt) / 86400000;
  return Math.max(0, Math.ceil(LUCKIN_TTL_DAYS - elapsed));
}

function LuckinCard({
  cred,
  onChange,
}: {
  cred: LuckinCred | null;
  onChange: (c: LuckinCred | null) => void;
}) {
  const [open, setOpen] = useState(false);
  const [input, setInput] = useState("");
  const daysLeft = cred ? luckinDaysLeft(cred) : 0;
  const expired = cred !== null && daysLeft <= 0;

  const save = () => {
    const token = input.trim();
    if (!token) return;
    const next: LuckinCred = { token, savedAt: Date.now() };
    localStorage.setItem(LUCKIN_KEY, JSON.stringify(next));
    onChange(next);
    setInput("");
    setOpen(false);
  };

  const unbind = () => {
    localStorage.removeItem(LUCKIN_KEY);
    onChange(null);
    setInput("");
    setOpen(false);
  };

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        className={cn(
          "flex w-full items-center justify-between gap-3 rounded-lg border bg-card px-3 py-2.5 text-left shadow-sm transition-colors hover:bg-accent/45 focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none",
          open && "bg-accent/45"
        )}
      >
        <span className="flex min-w-0 items-center gap-2.5">
          <span className="flex size-8 shrink-0 items-center justify-center rounded-md bg-accent text-accent-foreground">
            <Coffee className="size-4" />
          </span>
          <span className="min-w-0">
            <span className="block truncate text-sm font-medium">瑞幸点单</span>
            <span className="block truncate text-xs text-muted-foreground">
              {cred ? "点击管理 MCP token" : "点击绑定 MCP token"}
            </span>
          </span>
        </span>
        {cred ? (
          <Badge variant={expired ? "destructive" : "secondary"} className="shrink-0 rounded-full font-normal">
            {expired ? "已过期" : `剩 ${daysLeft} 天`}
          </Badge>
        ) : (
          <Badge variant="outline" className="shrink-0 rounded-full font-normal">
            未绑定
          </Badge>
        )}
      </PopoverTrigger>
      <PopoverContent className="space-y-3" align="start" side="top">
        <div className="space-y-1">
          <PopoverTitle>瑞幸 MCP token</PopoverTitle>
          <PopoverDescription>token 只存在当前浏览器，发消息时透传给工具。</PopoverDescription>
        </div>
        <div className="space-y-2">
          <Label htmlFor="luckin-token">MCP token</Label>
          <Input
            id="luckin-token"
            type="password"
            value={input}
            placeholder={cred ? "粘贴新的瑞幸 MCP token" : "粘贴瑞幸 MCP token"}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && save()}
          />
        </div>
        <div className="flex flex-wrap gap-2">
          <Button type="button" onClick={save} disabled={!input.trim()}>
            {cred ? "保存新 token" : "保存"}
          </Button>
          {cred && (
            <Button type="button" variant="destructive" onClick={unbind}>
              解绑
            </Button>
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}

const PLAN_PHASE_LABEL: Record<string, string> = {
  planning: "规划",
  replanning: "重新规划",
  action: "执行",
  reasoning: "思考",
};

// 按阶段上色, 让计划卡从背景里跳出来并一眼区分: 规划=墨蓝, 执行=赭石, 思考=中性灰。
const PLAN_PHASE_STYLE: Record<string, { card: string; dot: string; title: string }> = {
  planning: { card: "border-l-primary/70 bg-primary/[0.05]", dot: "bg-primary", title: "text-primary" },
  replanning: { card: "border-l-primary/70 bg-primary/[0.05]", dot: "bg-primary", title: "text-primary" },
  action: { card: "border-l-sienna/70 bg-sienna/[0.06]", dot: "bg-sienna", title: "text-sienna" },
  reasoning: { card: "border-l-muted-foreground/50 bg-muted/50", dot: "bg-muted-foreground", title: "text-foreground/80" },
};
const PLAN_PHASE_FALLBACK = PLAN_PHASE_STYLE.reasoning;

const PlanRail = memo(function PlanRail({
  steps,
  live,
  pending,
}: {
  steps: PlanStep[];
  live: boolean;
  pending: boolean;
}) {
  return (
    <WorkspacePanel as="aside" className="hidden w-96 flex-col xl:flex">
      <div className="flex h-16 shrink-0 items-center justify-between border-b px-4">
        <div>
          <div className="text-sm font-medium">执行计划</div>
          <div className="text-xs text-muted-foreground">Plan · Execute</div>
        </div>
        {(live || pending) && (
          <Badge variant="secondary" className="rounded-full font-normal">
            进行中
          </Badge>
        )}
      </div>
      <div className="relative min-h-0 flex-1">
        <ScrollArea className="absolute! inset-0">
        <div className="p-4">
          {steps.length === 0 ? (
            <div className="rounded-lg border border-dashed bg-muted/30 p-6 text-center">
              <div className="text-sm font-medium">
                {pending ? "小云雀正在思考…" : "先规划，再分步执行"}
              </div>
              <p className="mt-1 text-sm text-muted-foreground">
                {pending
                  ? "若本轮需要分步执行，规划会在这里展开。"
                  : "发起任务后，规划与动作会实时展示。"}
              </p>
            </div>
          ) : (
            <div className="space-y-3">
              {steps.map((s, i) => {
                const ps = PLAN_PHASE_STYLE[s.phase] || PLAN_PHASE_FALLBACK;
                return (
                  <Plan
                    key={i}
                    defaultOpen
                    isStreaming={live && i === steps.length - 1}
                    className={cn("border-l-2 shadow-sm", ps.card)}
                  >
                    <PlanHeader>
                      <div className="flex items-center gap-2">
                        <span className={cn("size-1.5 rounded-full", ps.dot)} aria-hidden />
                        <PlanTitle className={cn("font-serif text-sm", ps.title)}>
                          {PLAN_PHASE_LABEL[s.phase] || s.phase}
                        </PlanTitle>
                      </div>
                      <PlanAction>
                        <PlanTrigger />
                      </PlanAction>
                    </PlanHeader>
                    <PlanContent>
                      <p className="whitespace-pre-wrap text-sm leading-6 text-foreground/75">
                        {s.text.trim()}
                      </p>
                    </PlanContent>
                  </Plan>
                );
              })}
            </div>
          )}
        </div>
        </ScrollArea>
      </div>
    </WorkspacePanel>
  );
});

const Bubble = memo(function Bubble({ message }: { message: Message }) {
  const isAssistant = message.role === "assistant";
  return (
    <article className={cn("flex flex-col gap-1.5", isAssistant ? "items-start" : "items-end")}>
      {isAssistant ? (
        <div className="w-full">
          <Markdown richLinks>{message.content}</Markdown>
        </div>
      ) : (
        <div className="max-w-[80%] rounded-2xl bg-primary px-4 py-2.5 text-primary-foreground">
          <p className="whitespace-pre-wrap text-sm leading-6">{message.content}</p>
        </div>
      )}
      <div className="flex gap-2 px-0.5 text-xs text-muted-foreground">
        <span>{isAssistant ? "小云雀" : "我"}</span>
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
  "帮我找几篇关于注意力机制的经典论文",
  "我在国贸，帮我点一杯瑞幸生椰拿铁",
  "查一下我刚才那单咖啡好了没",
];

export default function PioneerPage() {
  const [token, setToken] = useState<string | null>(null);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [activeID, setActiveID] = useState("");
  const [messages, setMessages] = useState<Message[]>([]);
  const [sending, setSending] = useState(false);
  const [toolNote, setToolNote] = useState("");
  const [streamPlan, setStreamPlan] = useState<PlanStep[]>([]);
  const [input, setInput] = useState("");
  const [error, setError] = useState("");
  const [luckin, setLuckin] = useState<LuckinCred | null>(null);
  const [deleteConfirm, setDeleteConfirm] = useState<PaperDeleteConfirmPayload | null>(null);
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setToken(loadToken());
    setLuckin(loadLuckin());
  }, []);

  useEffect(() => {
    api.setToken(token ?? "");
  }, [token]);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "instant" });
  }, [messages, sending, toolNote]);

  const fail = (e: unknown) =>
    setError(e instanceof Error ? e.message : "请求失败，请稍后再试");

  const openSession = useCallback(async (id: string) => {
    setActiveID(id);
    setError("");
    try {
      const msgs = await api.listMessages(id);
      setMessages(Array.isArray(msgs) ? msgs : []);
    } catch (e) {
      fail(e);
    }
  }, []);

  useEffect(() => {
    if (!token) return;
    api
      .listSessions()
      .then((list) => {
        const mine = (Array.isArray(list) ? list : []).filter((s) => s.agent_type === AGENT_TYPE);
        setSessions(mine);
        if (mine.length > 0) void openSession(mine[0].id);
      })
      .catch(fail);
  }, [token, openSession]);

  const startNewSession = () => {
    setActiveID("");
    setMessages([]);
    setStreamPlan([]);
    setError("");
    setInput("");
  };

  const removeSession = async (id: string) => {
    try {
      await api.deleteSession(id);
      setSessions((list) => list.filter((s) => s.id !== id));
      if (activeID === id) {
        setActiveID("");
        setMessages([]);
      }
    } catch (e) {
      fail(e);
    }
  };

  const sendMessageText = async (
    query: string,
    options: { displayText?: string; extraHeaders?: Record<string, string> } = {},
  ) => {
    const q = query.trim();
    if (!q || sending) return;
    const visibleText = options.displayText?.trim() || q;
    setError("");
    let sid = activeID;
    if (!sid) {
      const title = visibleText.length > 24 ? `${visibleText.slice(0, 24)}…` : visibleText;
      try {
        const s = await api.createSession(title, undefined, AGENT_TYPE);
        setSessions((list) => [s, ...list]);
        setActiveID(s.id);
        sid = s.id;
      } catch (e) {
        fail(e);
        return;
      }
    }
    setInput("");
    setSending(true);
    setMessages((list) => [
      ...list,
      {
        id: `local-${Date.now()}`,
        session_id: sid,
        role: "user",
        content: visibleText,
        created_at: new Date().toISOString(),
      },
    ]);

    const placeholderID = `stream-${Date.now()}`;
    let shown = false;
    const patch = (fn: (m: Message) => Message) => {
      if (!shown) {
        shown = true;
        setMessages((list) => [
          ...list,
          fn({
            id: placeholderID,
            session_id: sid,
            role: "assistant",
            content: "",
            created_at: new Date().toISOString(),
            streaming: true,
          }),
        ]);
        return;
      }
      setMessages((list) => list.map((m) => (m.id === placeholderID ? fn(m) : m)));
    };

    let pending = "";
    let rafID: number | null = null;
    const flush = () => {
      rafID = null;
      if (!pending) return;
      const chunk = pending;
      pending = "";
      setToolNote("");
      patch((m) => ({ ...m, content: m.content + chunk }));
    };
    const planSteps: PlanStep[] = [];
    let planRafID: number | null = null;
    const flushPlan = () => {
      planRafID = null;
      setStreamPlan(planSteps.map((s) => ({ ...s })));
    };
    const cancelFlush = () => {
      if (rafID !== null) cancelAnimationFrame(rafID);
      if (planRafID !== null) cancelAnimationFrame(planRafID);
      rafID = null;
      planRafID = null;
    };

    try {
      const headers: Record<string, string> = { ...(options.extraHeaders ?? {}) };
      if (luckin?.token) headers[LUCKIN_HEADER] = luckin.token;
      const data = await api.sendMessage(sid, q, headers, {
        onDelta: (text, reset) => {
          // 新一轮答案开始:丢弃上一轮已流式正文,气泡只展示末轮。
          if (reset) {
            pending = "";
            patch((m) => ({ ...m, content: "" }));
          }
          pending += text;
          if (rafID === null) rafID = requestAnimationFrame(flush);
        },
        onPlan: (phase, content) => {
          const last = planSteps[planSteps.length - 1];
          if (last && last.phase === phase) last.text += content;
          else planSteps.push({ phase, text: content });
          if (planRafID === null) planRafID = requestAnimationFrame(flushPlan);
        },
        onTool: (tool, done) =>
          setToolNote(done ? `${tool} 已返回，正在继续…` : `正在调用 ${tool} …`),
        onConfirmDeletePaper: (payload) => setDeleteConfirm(payload),
      });
      cancelFlush();
      setStreamPlan([]);
      setMessages((list) => [
        ...list.filter((m) => m.id !== placeholderID),
        {
          ...data.message,
          id: data.message.id || `local-a-${Date.now()}`,
          plan: planSteps.length > 0 ? planSteps : undefined,
        },
      ]);
    } catch (e) {
      cancelFlush();
      setStreamPlan([]);
      setMessages((list) => list.filter((m) => m.id !== placeholderID));
      fail(e);
    } finally {
      setSending(false);
      setToolNote("");
    }
  };

  const send = async () => {
    await sendMessageText(input);
  };

  const confirmPaperDelete = async () => {
    if (!deleteConfirm || sending) return;
    const title = deleteConfirm.title || deleteConfirm.file_name || deleteConfirm.paper_id;
    const query = [
      "用户已在前端删除确认弹窗中确认删除论文。",
      `paper_id: ${deleteConfirm.paper_id}`,
      `title: ${title}`,
      "请立即调用 delete_my_paper 完成删除。",
    ].join("\n");
    const token = deleteConfirm.confirmation_token;
    setDeleteConfirm(null);
    await sendMessageText(query, {
      displayText: `已确认删除《${title}》`,
      extraHeaders: { [DELETE_CONFIRM_HEADER]: token },
    });
  };

  if (token === null) {
    return (
      <main className="flex min-h-dvh items-center justify-center bg-muted/40 p-4">
        <Card className="max-w-md text-center">
          <CardHeader>
            <CardTitle>小云雀</CardTitle>
            <CardDescription>正在读取登录态…</CardDescription>
          </CardHeader>
          <CardContent>
            <Loader2 className="mx-auto size-5 animate-spin text-muted-foreground" />
          </CardContent>
        </Card>
      </main>
    );
  }

  if (!token) {
    return (
      <main className="flex min-h-dvh items-center justify-center bg-muted/40 p-4">
        <Card className="max-w-md text-center">
          <CardHeader>
            <CardTitle>小云雀</CardTitle>
            <CardDescription>还没有登录态，请先回 GopherPaper 主页登录。</CardDescription>
          </CardHeader>
          <CardContent>
            <Link className={buttonVariants()} href="/">返回主页</Link>
          </CardContent>
        </Card>
      </main>
    );
  }

  const activeSession = sessions.find((s) => s.id === activeID);
  const lastAssistant = [...messages].reverse().find((m) => m.role === "assistant");
  const railSteps = sending ? streamPlan : (lastAssistant?.plan ?? []);
  const railLive = sending && streamPlan.length > 0;
  const railPending = sending && streamPlan.length === 0;

  return (
    <WorkspaceFrame>
      <WorkspacePanel as="aside" className="hidden w-80 flex-col lg:flex">
        <div className="space-y-3 border-b p-4">
          <div className="flex items-center gap-3">
            <Link
              className={buttonVariants({ variant: "ghost", size: "icon" })}
              href="/"
              aria-label="返回主应用"
            >
              <ArrowLeft className="size-4" />
            </Link>
            <div>
              <div className="text-sm font-medium">小云雀</div>
              <div className="text-xs text-muted-foreground">查论文 · 点咖啡</div>
            </div>
          </div>
          <Button type="button" className="w-full justify-start" onClick={startNewSession}>
            <Plus className="size-4" />
            新会话
          </Button>
        </div>
        <div className="relative min-h-0 flex-1">
        <ScrollArea className="absolute! inset-0 px-3">
          <div className="space-y-1 py-3">
            {sessions.length === 0 ? (
              <Empty title="还没有会话" text="发送一条消息后会自动创建云雀会话。" compact />
            ) : (
              sessions.map((s) => (
                <div
                  key={s.id}
                  className={cn(
                    "group relative flex items-center gap-1 rounded-md p-1",
                    s.id === activeID
                      ? "bg-card shadow-sm before:absolute before:inset-y-1.5 before:left-0 before:w-0.5 before:rounded-full before:bg-sienna"
                      : "hover:bg-accent/60",
                  )}
                >
                  <button
                    type="button"
                    className="min-w-0 flex-1 rounded-md px-2 py-2 text-left"
                    onClick={() => void openSession(s.id)}
                  >
                    <div className="truncate text-sm font-medium">{sessionTitle(s)}</div>
                    <div className="mt-0.5 text-xs text-muted-foreground">
                      {formatTime(s.updated_at || s.created_at)}
                    </div>
                  </button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    className="opacity-0 group-hover:opacity-100"
                    onClick={() => void removeSession(s.id)}
                  >
                    <Trash2 className="size-3.5" />
                  </Button>
                </div>
              ))
            )}
          </div>
        </ScrollArea>
        </div>
        <div className="border-t p-3">
          <LuckinCard cred={luckin} onChange={setLuckin} />
        </div>
      </WorkspacePanel>

      <WorkspacePanel className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-16 shrink-0 items-center justify-between border-b px-5">
          <div className="min-w-0">
            <div className="truncate text-sm font-medium">
              {activeSession ? sessionTitle(activeSession) : "输入消息会自动创建云雀会话"}
            </div>
            <div className="text-xs text-muted-foreground">工具增强会话</div>
          </div>
          <Button type="button" variant="secondary" onClick={startNewSession}>
            <Plus className="size-4" />
            新建
          </Button>
        </header>
        <div className="relative min-h-0 flex-1">
        <ScrollArea className="absolute! inset-0">
          <div className="mx-auto flex max-h-full w-full max-w-3xl flex-col gap-6 px-6 py-5">
            {messages.length === 0 && !sending ? (
              <div className="mx-auto flex min-h-[24rem] w-full max-w-xl flex-col justify-center gap-4">
                <Empty
                  title="嗨，我是小云雀"
                  text="学术问题、找论文、点杯瑞幸，都可以直接说。"
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
                {messages.map((m) => (
                  <Bubble key={String(m.id)} message={m} />
                ))}
                {sending && !messages.some((m) => m.streaming) && (
                  <article className="flex items-start">
                    <div className="inline-flex items-center gap-2 rounded-xl border bg-card px-4 py-3 text-sm text-muted-foreground">
                      <Loader2 className="size-4 animate-spin" />
                      {streamPlan.length > 0 ? "小云雀正在按计划执行…" : "小云雀正在处理工具与上下文…"}
                    </div>
                  </article>
                )}
              </>
            )}
            <div ref={bottomRef} />
          </div>
        </ScrollArea>
        </div>
        {error && (
          <div className="shrink-0 px-6 pt-2">
            <div className="mx-auto w-full max-w-3xl rounded-md border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {error}
            </div>
          </div>
        )}
        <form
          className="shrink-0 px-6 pb-5 pt-2"
          onSubmit={(e) => {
            e.preventDefault();
            void send();
          }}
        >
          <div className="mx-auto w-full max-w-3xl">
            <ToolStatus note={toolNote} />
            <div className="flex items-end gap-2 rounded-[1.625rem] border border-border bg-card py-1.5 pl-2 pr-1.5 shadow-sm transition-[border-color,box-shadow] focus-within:border-ring/50 focus-within:shadow-md">
              <Textarea
                rows={1}
                value={input}
                placeholder="我在国贸，帮我点一杯冰美式"
                disabled={sending}
                className="max-h-44 min-h-9 resize-none overflow-y-auto border-0 bg-transparent px-3 py-1.5 leading-6 shadow-none focus-visible:border-transparent focus-visible:ring-0"
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && !e.shiftKey) {
                    e.preventDefault();
                    void send();
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
      </WorkspacePanel>
      <Dialog
        open={Boolean(deleteConfirm)}
        onOpenChange={(open) => {
          if (!open) setDeleteConfirm(null);
        }}
      >
        {deleteConfirm && (
          <DialogContent className="sm:max-w-md">
            <DialogHeader className="min-w-0 pr-8">
              <div className="flex min-w-0 items-center gap-2">
                <span className="flex size-9 shrink-0 items-center justify-center rounded-md bg-destructive/10 text-destructive">
                  <AlertTriangle className="size-4" />
                </span>
                <DialogTitle className="min-w-0">确认删除论文</DialogTitle>
              </div>
              <DialogDescription>
                删除后会清理这篇论文的绑定会话、报告、图片、向量索引和知识图谱节点。
              </DialogDescription>
            </DialogHeader>
            <div className="min-w-0 overflow-hidden rounded-lg border bg-muted/30 p-3">
              <div className="flex min-w-0 items-start gap-3">
                <span className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-md bg-background text-muted-foreground ring-1 ring-border">
                  <FileText className="size-4" />
                </span>
                <div className="min-w-0 space-y-1">
                  <div className="line-clamp-2 min-w-0 break-words text-sm font-medium [overflow-wrap:anywhere]">
                    {deleteConfirm.title || deleteConfirm.file_name || deleteConfirm.paper_id}
                  </div>
                  {deleteConfirm.file_name && (
                    <div
                      className="min-w-0 truncate text-xs text-muted-foreground"
                      title={deleteConfirm.file_name}
                    >
                      {deleteConfirm.file_name}
                    </div>
                  )}
                  <div className="break-all text-xs text-muted-foreground">
                    ID: {deleteConfirm.paper_id}
                  </div>
                </div>
              </div>
              {deleteConfirm.message && (
                <p className="mt-3 min-w-0 break-words text-xs leading-5 text-muted-foreground [overflow-wrap:anywhere]">
                  {deleteConfirm.message}
                </p>
              )}
            </div>
            <DialogFooter className="sm:flex-nowrap">
              <Button
                type="button"
                variant="outline"
                className="w-full sm:w-auto"
                onClick={() => setDeleteConfirm(null)}
              >
                取消
              </Button>
              <Button
                type="button"
                variant="destructive"
                className="w-full sm:w-auto"
                disabled={sending || !deleteConfirm.confirmation_token}
                onClick={() => void confirmPaperDelete()}
              >
                {sending ? (
                  <Loader2 className="size-4 animate-spin" />
                ) : (
                  <Trash2 className="size-4" />
                )}
                确认删除
              </Button>
            </DialogFooter>
          </DialogContent>
        )}
      </Dialog>
      <PlanRail steps={railSteps} live={railLive} pending={railPending} />
    </WorkspaceFrame>
  );
}
