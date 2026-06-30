"use client";

import { useEffect, useMemo, useState, type FormEvent } from "react";
import {
  ArrowLeft,
  Check,
  ChevronDown,
  Pin,
  Plus,
  Search,
  Tags,
  Trash2,
} from "lucide-react";
import Link from "next/link";

import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Textarea } from "@/components/ui/textarea";
import * as api from "@/lib/gopherpaper/api";
import { useApp } from "@/lib/gopherpaper/store";
import type { MemoryItem, MemoryPayload, MemoryType } from "@/lib/gopherpaper/types";
import { cn } from "@/lib/utils";
import { Empty } from "./app-ui";
import { WorkspaceFrame, WorkspacePanel } from "./workspace-frame";

const MEMORY_TYPES = [
  "研究方向",
  "论文笔记",
  "问题清单",
  "术语解释",
  "方法想法",
  "待办事项",
];

const emptyDraft: MemoryPayload = {
  type: "论文笔记",
  title: "",
  content: "",
  tags: [],
  pinned: false,
};

function splitTags(value: string) {
  return value
    .split(/[,，\s]+/)
    .map((tag) => tag.trim())
    .filter(Boolean);
}

export function MemoryCenter() {
  const { authed } = useApp();
  const [items, setItems] = useState<MemoryItem[]>([]);
  const [active, setActive] = useState<MemoryItem | null>(null);
  const [draft, setDraft] = useState<MemoryPayload>(emptyDraft);
  const [tagText, setTagText] = useState("");
  const [query, setQuery] = useState("");
  const [typeFilter, setTypeFilter] = useState<MemoryType | "all">("all");
  const [typeOpen, setTypeOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const visibleItems = useMemo(() => {
    const q = query.trim().toLowerCase();
    return items.filter((item) => {
      if (typeFilter !== "all" && item.type !== typeFilter) return false;
      if (!q) return true;
      const haystack = [
        item.title,
        item.content,
        ...(item.tags ?? []),
        item.type,
      ]
        .join(" ")
        .toLowerCase();
      return haystack.includes(q);
    });
  }, [items, query, typeFilter]);

  const typeOptions = useMemo(() => {
    const set = new Set<string>(MEMORY_TYPES);
    for (const item of items) {
      if (item.type) set.add(item.type);
    }
    return Array.from(set);
  }, [items]);

  useEffect(() => {
    if (!authed) return;
    void load();
  }, [authed]);

  function startCreate() {
    setActive(null);
    setDraft(emptyDraft);
    setTagText("");
    setError("");
  }

  function edit(item: MemoryItem) {
    setActive(item);
    setDraft({
      type: item.type,
      title: item.title,
      content: item.content,
      tags: item.tags ?? [],
      source_paper_id: item.source_paper_id || "",
      pinned: item.pinned,
    });
    setTagText((item.tags ?? []).join(" "));
    setError("");
  }

  async function load() {
    setLoading(true);
    setError("");
    try {
      const res = await api.listMemories();
      setItems(res);
      if (active && !res.some((item) => item.id === active.id)) {
        startCreate();
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "读取记忆失败");
    } finally {
      setLoading(false);
    }
  }

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSaving(true);
    setError("");
    const payload: MemoryPayload = {
      ...draft,
      title: draft.title.trim(),
      content: draft.content.trim(),
      source_paper_id: draft.source_paper_id?.trim() || "",
      tags: splitTags(tagText),
      pinned: Boolean(draft.pinned),
    };
    try {
      const saved = active
        ? await api.updateMemory(active.id, payload)
        : await api.createMemory(payload);
      setItems((prev) => {
        const next = prev.filter((item) => item.id !== saved.id);
        return [saved, ...next].sort((a, b) => Number(b.pinned) - Number(a.pinned));
      });
      edit(saved);
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存记忆失败");
    } finally {
      setSaving(false);
    }
  }

  async function removeActive() {
    if (!active) return;
    setSaving(true);
    setError("");
    try {
      await api.deleteMemory(active.id);
      setItems((prev) => prev.filter((item) => item.id !== active.id));
      startCreate();
    } catch (err) {
      setError(err instanceof Error ? err.message : "删除记忆失败");
    } finally {
      setSaving(false);
    }
  }

  if (!authed) {
    return (
      <main className="flex h-dvh items-center justify-center bg-muted/50 p-6">
        <div className="text-center">
          <Empty title="请先登录" text="登录后即可管理个人记忆。" />
          <Link href="/" className={cn(buttonVariants({ variant: "outline" }), "mt-4")}>
            返回首页
          </Link>
        </div>
      </main>
    );
  }

  return (
    <WorkspaceFrame className="bg-muted/40">
      <WorkspacePanel as="aside" className="hidden w-64 flex-col lg:flex">
        <div className="border-b p-4">
          <div className="flex items-center gap-3">
            <Link href="/" className={buttonVariants({ variant: "ghost", size: "icon" })}>
              <ArrowLeft className="size-4" />
            </Link>
            <div className="min-w-0">
              <div className="truncate text-sm font-medium">小囊鼠记忆</div>
              <div className="truncate text-xs text-muted-foreground">{items.length} 条卡片</div>
            </div>
          </div>
        </div>
        <nav className="space-y-1 p-3">
          <button
            type="button"
            className={cn(
              "flex w-full items-center justify-between rounded-lg px-3 py-2 text-left text-sm transition-colors",
              typeFilter === "all" ? "bg-primary text-primary-foreground" : "hover:bg-muted",
            )}
            onClick={() => setTypeFilter("all")}
          >
            <span>全部</span>
            <span className="text-xs opacity-70">{items.length}</span>
          </button>
          {typeOptions.map((type) => (
            <button
              key={type}
              type="button"
              className={cn(
                "flex w-full items-center justify-between rounded-lg px-3 py-2 text-left text-sm transition-colors",
                typeFilter === type ? "bg-primary text-primary-foreground" : "hover:bg-muted",
              )}
              onClick={() => setTypeFilter(type)}
            >
              <span>{type}</span>
              <span className="text-xs opacity-70">
                {items.filter((memory) => memory.type === type).length}
              </span>
            </button>
          ))}
        </nav>
      </WorkspacePanel>

      <WorkspacePanel className="min-w-0 flex-1">
        <section className="grid h-full min-h-0 lg:grid-cols-[minmax(16rem,0.72fr)_minmax(0,1.28fr)]">
          <div className="flex min-w-0 flex-col border-r">
            <header className="shrink-0 border-b p-5">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <h1 className="font-serif text-2xl font-semibold tracking-tight">记忆库</h1>
                  <p className="mt-1 text-xs text-muted-foreground">{visibleItems.length} / {items.length} 条</p>
                </div>
                <Button type="button" size="sm" onClick={startCreate}>
                  <Plus className="size-4" />
                  新建
                </Button>
              </div>
              <div className="mt-4 grid items-end gap-3 sm:grid-cols-[minmax(0,1fr)_10rem]">
                <div className="relative min-w-0">
                  <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    value={query}
                    onChange={(event) => setQuery(event.target.value)}
                    placeholder="搜索标题、内容或标签"
                    className="h-10 pl-9"
                  />
                </div>
                <TypeFilter value={typeFilter} options={typeOptions} onChange={setTypeFilter} />
              </div>
            </header>

            <div className="min-h-0 flex-1 overflow-auto p-3">
              {loading ? (
                <div className="p-4 text-sm text-muted-foreground">加载中...</div>
              ) : visibleItems.length === 0 ? (
                <div className="grid h-full place-items-center p-8">
                  <Empty title="还没有记忆" text="先创建一张卡片，把研究方向或论文笔记沉淀下来。" />
                </div>
              ) : (
                <div className="divide-y">
                  {visibleItems.map((item) => (
                    <button
                      key={item.id}
                      type="button"
                      className={cn(
                        "w-full px-2 py-3 text-left transition hover:bg-muted/50",
                        active?.id === item.id && "bg-primary/5",
                      )}
                      onClick={() => edit(item)}
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0">
                          <div className="flex flex-wrap items-center gap-2">
                            <Badge variant={item.pinned ? "default" : "secondary"}>
                              {item.pinned && <Pin className="size-3" />}
                              {item.type || "论文笔记"}
                            </Badge>
                            <h2 className="truncate text-base font-semibold">{item.title}</h2>
                          </div>
                          <p className="mt-2 line-clamp-2 text-sm leading-relaxed text-muted-foreground">
                            {item.content}
                          </p>
                        </div>
                      </div>
                      {item.tags && item.tags.length > 0 && (
                        <div className="mt-3 flex flex-wrap gap-1.5">
                          {item.tags.map((tag) => (
                            <span key={tag} className="rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground">
                              #{tag}
                            </span>
                          ))}
                        </div>
                      )}
                    </button>
                  ))}
                </div>
              )}
            </div>
          </div>

          <form className="flex min-w-0 flex-col overflow-hidden" onSubmit={save}>
            <div className="shrink-0 border-b p-5">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <Badge variant="secondary">{active ? "编辑" : "新建"}</Badge>
                  <h2 className="mt-2 text-lg font-semibold">{draft.title || "未命名卡片"}</h2>
                </div>
                <Button
                  type="button"
                  variant={draft.pinned ? "default" : "outline"}
                  size="sm"
                  onClick={() => setDraft((prev) => ({ ...prev, pinned: !prev.pinned }))}
                >
                  <Pin className="size-4" />
                  {draft.pinned ? "已置顶" : "置顶"}
                </Button>
              </div>
            </div>

            <div className="min-h-0 flex-1 space-y-5 overflow-auto p-6">
              <div className="grid gap-4 sm:grid-cols-2">
                <div className="space-y-2">
                  <Label>类型</Label>
                  <div className="flex h-10 overflow-hidden rounded-lg border border-input bg-background transition-colors focus-within:border-ring focus-within:ring-3 focus-within:ring-ring/50">
                    <Input
                      value={draft.type}
                      maxLength={32}
                      placeholder="论文笔记"
                      className="h-full min-w-0 flex-1 rounded-none border-0 bg-transparent px-3 shadow-none focus-visible:ring-0"
                      onChange={(event) => setDraft((prev) => ({ ...prev, type: event.target.value }))}
                    />
                    <Popover open={typeOpen} onOpenChange={setTypeOpen}>
                      <PopoverTrigger className="inline-flex h-full w-10 shrink-0 items-center justify-center border-l bg-background text-muted-foreground transition-colors hover:bg-muted focus-visible:outline-none">
                        <ChevronDown className="size-4" />
                      </PopoverTrigger>
                      <PopoverContent align="start" className="w-56 p-1">
                        {typeOptions.map((type) => (
                          <button
                            key={type}
                            type="button"
                            className="flex w-full items-center rounded-md px-2 py-1.5 text-left text-sm hover:bg-muted"
                            onClick={() => {
                              setDraft((prev) => ({ ...prev, type }));
                              setTypeOpen(false);
                            }}
                          >
                            {type}
                          </button>
                        ))}
                      </PopoverContent>
                    </Popover>
                  </div>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="memory-title">标题</Label>
                  <Input
                    id="memory-title"
                    value={draft.title}
                    maxLength={160}
                    placeholder="例如: 多 Agent 文献检索方向"
                    className="h-10"
                    onChange={(event) => setDraft((prev) => ({ ...prev, title: event.target.value }))}
                  />
                </div>
              </div>

              <div className="space-y-2">
                <Label htmlFor="memory-content">内容</Label>
                <Textarea
                  id="memory-content"
                  value={draft.content}
                  placeholder="写下你的研究判断、论文笔记、问题或后续计划"
                  className="min-h-[26rem] resize-y"
                  onChange={(event) => setDraft((prev) => ({ ...prev, content: event.target.value }))}
                />
              </div>

              <div className="grid gap-4 sm:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="memory-tags">标签</Label>
                  <div className="relative">
                    <Tags className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                      id="memory-tags"
                      value={tagText}
                      placeholder="agent RAG 综述"
                      className="pl-9"
                      onChange={(event) => setTagText(event.target.value)}
                    />
                  </div>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="memory-source">关联论文 ID 可选</Label>
                  <Input
                    id="memory-source"
                    value={draft.source_paper_id ?? ""}
                    placeholder="从论文沉淀时再填写"
                    onChange={(event) => setDraft((prev) => ({ ...prev, source_paper_id: event.target.value }))}
                  />
                </div>
              </div>

              {error && (
                <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</p>
              )}
            </div>

            <div className="flex shrink-0 flex-col gap-2 border-t p-5 sm:flex-row sm:justify-between">
              <Button type="button" variant="destructive" disabled={!active || saving} onClick={removeActive}>
                <Trash2 className="size-4" />
                删除
              </Button>
              <div className="flex gap-2">
                <Button type="button" variant="outline" disabled={saving} onClick={startCreate}>
                  <Plus className="size-4" />
                  新卡片
                </Button>
                <Button type="submit" disabled={saving}>
                  <Check className="size-4" />
                  {saving ? "保存中..." : "保存记忆"}
                </Button>
              </div>
            </div>
          </form>
        </section>
      </WorkspacePanel>
    </WorkspaceFrame>
  );
}

function TypeFilter({
  value,
  options,
  onChange,
}: {
  value: MemoryType | "all";
  options: string[];
  onChange: (value: MemoryType | "all") => void;
}) {
  const [open, setOpen] = useState(false);
  const label = value === "all" ? "全部类型" : value;
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger className="inline-flex h-10 w-full items-center justify-between rounded-lg border bg-background px-3 text-sm font-normal transition-colors hover:bg-muted focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none">
        <span className="truncate">{label}</span>
        <ChevronDown className="size-4 text-muted-foreground" />
      </PopoverTrigger>
      <PopoverContent align="start" className="w-44 p-1">
        <button
          type="button"
          className="flex w-full items-center rounded-md px-2 py-1.5 text-left text-sm hover:bg-muted"
          onClick={() => {
            onChange("all");
            setOpen(false);
          }}
        >
          全部类型
        </button>
        {options.map((type) => (
          <button
            key={type}
            type="button"
            className="flex w-full items-center rounded-md px-2 py-1.5 text-left text-sm hover:bg-muted"
            onClick={() => {
              onChange(type);
              setOpen(false);
            }}
          >
            {type}
          </button>
        ))}
      </PopoverContent>
    </Popover>
  );
}
