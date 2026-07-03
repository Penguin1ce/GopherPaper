"use client";

import {
  AlertTriangle,
  ArrowUp,
  ArrowLeft,
  ChevronDown,
  Coffee,
  FileText,
  Loader2,
  Plus,
  Search,
  Sparkles,
  Trash2,
} from "lucide-react";
import Link from "next/link";
import { memo, useCallback, useEffect, useRef, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { PasswordInput } from "@/components/ui/password-input";
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
  AuthUser,
  Message,
  PaperDeleteConfirmPayload,
  PaperFlow,
  PlanStep,
  Session,
  Topic,
} from "@/lib/gopherpaper/types";
import { toolStatusText } from "@/lib/gopherpaper/tool-status";
import { formatTime, messagePlan, metaPlanSteps, sessionTitle } from "@/lib/gopherpaper/utils";
import { cn } from "@/lib/utils";
import { Empty } from "@/components/gopherpaper/app-ui";
import { AgentIntro } from "@/components/gopherpaper/agent-intro";
import { Markdown } from "@/components/gopherpaper/markdown";
import { PaperFlowCard } from "@/components/gopherpaper/paper-flow-card";
import { ProcessTrace } from "@/components/gopherpaper/process-trace";
import { WorkspaceFrame, WorkspacePanel } from "@/components/gopherpaper/workspace-frame";

const AUTH_KEY = "gopherpaper.auth";
const LUCKIN_KEY = "gopherpaper.luckin";
const LUCKIN_TTL_DAYS = 30;
const LUCKIN_HEADER = "X-Luckin-Token";
const DELETE_CONFIRM_HEADER = "X-GopherPaper-Delete-Confirm";
const AGENT_TYPE = "pioneer";
const REFERENCE_DRAFT_KEY = "gopherpaper.pioneer.referenceDraft";

function loadAuth(): { token: string; user: AuthUser | null } {
  if (typeof window === "undefined") return { token: "", user: null };
  try {
    const raw = localStorage.getItem(AUTH_KEY);
    if (!raw) return { token: "", user: null };
    const saved = JSON.parse(raw) as { token?: string; user?: AuthUser | null };
    return { token: saved.token || "", user: saved.user ?? null };
  } catch {
    return { token: "", user: null };
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

function consumeReferenceDraft(): string {
  if (typeof window === "undefined") return "";
  try {
    const draft = sessionStorage.getItem(REFERENCE_DRAFT_KEY) || "";
    if (draft) sessionStorage.removeItem(REFERENCE_DRAFT_KEY);
    return draft;
  } catch {
    return "";
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
          <PasswordInput
            id="luckin-token"
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

const PIONEER_TRACE_LABELS: Record<string, string> = {
  action: "执行",
};

const Bubble = memo(function Bubble({ message }: { message: Message }) {
  const isAssistant = message.role === "assistant";
  // flow 来源:本轮流式挂在 message.flow;刷新/重开会话则从持久化的 meta.flow 还原。
  const flow = message.flow ?? (message.meta?.flow as PaperFlow | undefined);
  const steps = isAssistant ? messagePlan(message) : [];
  return (
    <article className={cn("flex flex-col gap-1.5", isAssistant ? "items-start" : "items-end")}>
      {isAssistant ? (
        <div className="w-full">
          {steps.length > 0 && (
            <ProcessTrace
              steps={steps}
              live={!!message.streaming}
              phaseLabels={PIONEER_TRACE_LABELS}
            />
          )}
          <Markdown richLinks>{message.content}</Markdown>
          {flow && <PaperFlowCard flow={flow} />}
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
  "我在重庆大学虎溪校区，帮我点一杯冰美式",
  "画一个思路流程图",
];

function PioneerComposer({
  input,
  sending,
  toolNote,
  variant = "bottom",
  onInputChange,
  onSubmit,
}: {
  input: string;
  sending: boolean;
  toolNote: string;
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
        <ToolStatus note={toolNote} />
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
            placeholder={variant === "center" ? "问问小云雀" : ""}
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

interface SessionGroup {
  key: string;
  name: string;
  sessions: Session[];
  latest: number;
}

// groupSessionsByTopic 把会话按主题分组:组内会话按时间倒序,组间按各组最新会话时间倒序。
// 未归类(无 topic_id 或主题已被合并删除)的会话归「未归类」组,一同参与时间排序。
function groupSessionsByTopic(sessions: Session[], topics: Topic[]): SessionGroup[] {
  const topicName = new Map(topics.map((t) => [t.id, t.name]));
  const ms = (s: Session) => new Date(s.updated_at || s.created_at).getTime() || 0;
  const buckets = new Map<string, Session[]>();
  for (const s of sessions) {
    const tid = s.topic_id && topicName.has(s.topic_id) ? s.topic_id : "";
    const arr = buckets.get(tid);
    if (arr) arr.push(s);
    else buckets.set(tid, [s]);
  }
  const groups: SessionGroup[] = [];
  for (const [tid, list] of buckets) {
    list.sort((a, b) => ms(b) - ms(a));
    groups.push({
      key: tid || "__none__",
      name: tid ? (topicName.get(tid) ?? "未归类") : "未归类",
      sessions: list,
      latest: ms(list[0]),
    });
  }
  groups.sort((a, b) => b.latest - a.latest);
  return groups;
}

export default function PioneerPage() {
  const [token, setToken] = useState<string | null>(null);
  const [authUser, setAuthUser] = useState<AuthUser | null>(null);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [topics, setTopics] = useState<Topic[]>([]);
  const [topicsSupported, setTopicsSupported] = useState(true);
  const [collapsed, setCollapsed] = useState<Set<string>>(() => new Set());
  const [backfilling, setBackfilling] = useState(false);
  const [search, setSearch] = useState("");
  const [activeID, setActiveID] = useState("");
  const [messages, setMessages] = useState<Message[]>([]);
  const [sending, setSending] = useState(false);
  const [toolNote, setToolNote] = useState("");
  const [input, setInput] = useState("");
  const [error, setError] = useState("");
  const [luckin, setLuckin] = useState<LuckinCred | null>(null);
  const [deleteConfirm, setDeleteConfirm] = useState<PaperDeleteConfirmPayload | null>(null);
  const bottomRef = useRef<HTMLDivElement>(null);
  const mountedRef = useRef(true);
  const incomingReferenceDraftRef = useRef(false);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  useEffect(() => {
    const saved = loadAuth();
    setToken(saved.token);
    setAuthUser(saved.user);
    setLuckin(loadLuckin());
  }, []);

  useEffect(() => {
    const draft = consumeReferenceDraft();
    if (!draft) return;
    incomingReferenceDraftRef.current = true;
    setActiveID("");
    setMessages([]);
    setError("");
    setInput(draft);
  }, []);

  useEffect(() => {
    api.setToken(token ?? "");
  }, [token]);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "instant" });
  }, [messages, sending, toolNote]);

  const fail = useCallback((e: unknown) => {
    if (!mountedRef.current) return;
    setError(e instanceof Error ? e.message : "请求失败，请稍后再试");
  }, []);

  const openSession = useCallback(async (id: string) => {
    setActiveID(id);
    setError("");
    try {
      const msgs = await api.listMessages(id);
      if (mountedRef.current) setMessages(Array.isArray(msgs) ? msgs : []);
    } catch (e) {
      fail(e);
    }
  }, [fail]);

  // reloadSidebar 重拉会话与主题,供归类异步完成后刷新分组。openFirst 仅首次进页时定位首个会话。
  const reloadSidebar = useCallback(
    async (openFirst = false) => {
      try {
        const list = await api.listSessions();
        let topicList: Topic[] = [];
        try {
          topicList = await api.listTopics();
          setTopicsSupported(true);
        } catch (e) {
          if (!(e instanceof api.ApiError && e.status === 404)) throw e;
          setTopicsSupported(false);
        }
        if (!mountedRef.current) return;
        const mine = (Array.isArray(list) ? list : []).filter((s) => s.agent_type === AGENT_TYPE);
        setSessions(mine);
        setTopics(Array.isArray(topicList) ? topicList : []);
        if (openFirst && mine.length > 0) void openSession(mine[0].id);
      } catch (e) {
        fail(e);
      }
    },
    [openSession, fail],
  );

  useEffect(() => {
    if (!token) return;
    void reloadSidebar(!incomingReferenceDraftRef.current);
  }, [token, reloadSidebar]);

  const startNewSession = () => {
    setActiveID("");
    setMessages([]);
    setError("");
    setInput("");
  };

  // runBackfill 触发存量会话回填,后端异步逐个归类,分几次重拉让分组陆续刷新。
  const runBackfill = async () => {
    if (backfilling) return;
    setBackfilling(true);
    try {
      await api.backfillTopics();
      [2000, 5000, 9000].forEach((d) => window.setTimeout(() => void reloadSidebar(), d));
    } catch (e) {
      fail(e);
    } finally {
      window.setTimeout(() => {
        if (mountedRef.current) setBackfilling(false);
      }, 9000);
    }
  };

  // runClear 清空所有主题归类(演示重置),会话退回未归类,随后重拉刷新分组。
  const runClear = async () => {
    try {
      await api.clearTopics();
      await reloadSidebar();
    } catch (e) {
      fail(e);
    }
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
        if (!mountedRef.current) return;
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
      if (!mountedRef.current) return;
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
    // 思路图谱:generate_paper_flow 经 SSE 推达后即上屏占位,done 时一并挂最终消息。
    let capturedFlow: PaperFlow | undefined;
    let planRafID: number | null = null;
    const flushPlan = () => {
      planRafID = null;
      if (!mountedRef.current) return;
      patch((m) => ({ ...m, plan: planSteps.map((s) => ({ ...s })) }));
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
          if (!mountedRef.current) return;
          // 新一轮答案开始:丢弃上一轮已流式正文,气泡只展示末轮。
          if (reset) {
            pending = "";
            patch((m) => ({ ...m, content: "" }));
          }
          pending += text;
          if (rafID === null) rafID = requestAnimationFrame(flush);
        },
        onPlan: (phase, content) => {
          if (!mountedRef.current) return;
          phase = phase.trim();
          if (!phase) return;
          const last = planSteps[planSteps.length - 1];
          if (last && last.phase === phase) last.text += content;
          else planSteps.push({ phase, text: content });
          if (planRafID === null) planRafID = requestAnimationFrame(flushPlan);
        },
        onTool: (tool, done) => {
          if (mountedRef.current) {
            setToolNote(toolStatusText(tool, done));
          }
        },
        onConfirmDeletePaper: (payload) => {
          if (mountedRef.current) setDeleteConfirm(payload);
        },
        onPaperFlow: (payload) => {
          if (!mountedRef.current) return;
          capturedFlow = payload;
          patch((m) => ({ ...m, flow: payload }));
        },
        onPaperFlowNode: (payload) => {
          if (!mountedRef.current || !capturedFlow) return;
          const figures = payload.figure
            ? [...(capturedFlow.figures ?? []), payload.figure]
            : capturedFlow.figures;
          capturedFlow = {
            ...capturedFlow,
            nodes: capturedFlow.nodes.map((n) =>
              n.id === payload.node_id ? { ...n, detail: payload.detail } : n,
            ),
            figures,
          };
          const flow = capturedFlow;
          patch((m) => ({ ...m, flow }));
        },
      });
      cancelFlush();
      if (!mountedRef.current) return;
      const finalMeta = data.meta ?? data.message.meta;
      const persistedPlan = metaPlanSteps(finalMeta);
      const finalPlan = persistedPlan.length > 0 ? persistedPlan : planSteps;
      setMessages((list) => [
        ...list.filter((m) => m.id !== placeholderID),
        {
          ...data.message,
          id: data.message.id || `local-a-${Date.now()}`,
          meta: finalMeta,
          plan: finalPlan.length > 0 ? finalPlan : undefined,
          flow: capturedFlow,
        },
      ]);
      // 主题归类在后端异步进行,延时重拉一次让侧边栏分组刷新。
      window.setTimeout(() => void reloadSidebar(), 1800);
    } catch (e) {
      cancelFlush();
      if (!mountedRef.current) return;
      setMessages((list) => list.filter((m) => m.id !== placeholderID));
      fail(e);
    } finally {
      if (!mountedRef.current) return;
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

  const q = search.trim().toLowerCase();
  const visibleSessions = q
    ? sessions.filter((s) => sessionTitle(s).toLowerCase().includes(q))
    : sessions;
  const sessionGroups = groupSessionsByTopic(visibleSessions, topics);
  const toggleCollapse = (key: string) =>
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  const displayName = authUser?.name || authUser?.student_id || "同学";
  const emptyConversation = messages.length === 0 && !sending;

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
            <AgentIntro kind="pioneer" />
          </div>
          <Button type="button" className="w-full justify-start" onClick={startNewSession}>
            <Plus className="size-4" />
            新会话
          </Button>
          <div className="relative">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              type="search"
              value={search}
              placeholder="搜索会话"
              className="h-9 pl-9"
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>
          {topicsSupported && (
            <div className="flex items-center gap-1">
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="flex-1 justify-start text-muted-foreground"
                disabled={backfilling}
                onClick={() => void runBackfill()}
              >
                {backfilling ? (
                  <Loader2 className="size-3.5 animate-spin" />
                ) : (
                  <Sparkles className="size-3.5" />
                )}
                {backfilling ? "正在整理…" : "整理历史会话"}
              </Button>
              {topics.length > 0 && (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-7 shrink-0 px-2 text-xs text-muted-foreground/45 hover:text-muted-foreground"
                  onClick={() => void runClear()}
                >
                  清除归类
                </Button>
              )}
            </div>
          )}
        </div>
        <div className="relative min-h-0 flex-1">
          <ScrollArea className="absolute! inset-0 px-3">
            <div className="py-3">
              {sessions.length === 0 ? (
                <Empty title="还没有会话" text="发送一条消息后会自动创建云雀会话。" compact />
              ) : sessionGroups.length === 0 ? (
                <Empty title="没有匹配的会话" text={`没有标题包含「${search.trim()}」的会话。`} compact />
              ) : (
                sessionGroups.map((g) => {
                  const isOpen = !collapsed.has(g.key);
                  return (
                    <section key={g.key} className="mb-3 last:mb-0">
                      <button
                        type="button"
                        onClick={() => toggleCollapse(g.key)}
                        className="group/topic flex w-full items-center gap-1.5 rounded-md px-2 py-1.5 text-left transition-colors hover:bg-accent/50"
                      >
                        <ChevronDown
                          className={cn(
                            "size-3 shrink-0 text-muted-foreground/60 transition-transform",
                            !isOpen && "-rotate-90",
                          )}
                          aria-hidden
                        />
                        <span className="min-w-0 flex-1 truncate text-xs font-semibold tracking-wide text-muted-foreground">
                          {g.name}
                        </span>
                        <span className="shrink-0 text-xs tabular-nums text-muted-foreground/60">
                          {g.sessions.length}
                        </span>
                      </button>
                      {isOpen && (
                        <div className="mt-0.5 space-y-1">
                          {g.sessions.map((s) => (
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
                          ))}
                        </div>
                      )}
                    </section>
                  );
                })
              )}
            </div>
          </ScrollArea>
        </div>
        <div className="border-t p-3">
          <LuckinCard cred={luckin} onChange={setLuckin} />
        </div>
      </WorkspacePanel>

      <WorkspacePanel className="flex min-w-0 flex-1 flex-col">
        <div className="relative min-h-0 flex-1">
          <ScrollArea className="absolute! inset-0">
            <div
              className={cn(
                "mx-auto flex max-h-full w-full flex-col px-6 py-5",
                emptyConversation ? "max-w-4xl" : "max-w-3xl gap-6",
              )}
            >
              {emptyConversation ? (
                <div className="mx-auto flex min-h-[calc(100dvh-9rem)] w-full max-w-3xl flex-col justify-center gap-5 pb-16">
                  <div className="text-center">
                    <h1 className="text-xl font-medium leading-8 tracking-tight text-foreground sm:text-2xl">
                      {displayName}，你好
                    </h1>
                  </div>
                  <PioneerComposer
                    input={input}
                    sending={sending}
                    toolNote={toolNote}
                    variant="center"
                    onInputChange={setInput}
                    onSubmit={send}
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
                  {messages.map((m) => (
                    <Bubble key={String(m.id)} message={m} />
                  ))}
                  {sending && !messages.some((m) => m.streaming) && (
                    <article className="flex items-start">
                      <div className="inline-flex items-center gap-2 rounded-xl border bg-card px-4 py-3 text-sm text-muted-foreground">
                        <Loader2 className="size-4 animate-spin" />
                        小云雀正在处理工具与上下文…
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
        {!emptyConversation && (
          <PioneerComposer
            input={input}
            sending={sending}
            toolNote={toolNote}
            onInputChange={setInput}
            onSubmit={send}
          />
        )}
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
    </WorkspaceFrame>
  );
}
