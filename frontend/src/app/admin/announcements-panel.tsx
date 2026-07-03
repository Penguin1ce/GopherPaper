"use client";

import { useCallback, useEffect, useState } from "react";
import { Loader2, Megaphone, Plus, Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import {
  type AnnouncementItem,
  createAnnouncement,
  deleteAnnouncement,
  fetchAnnouncements,
  formatDateTime,
  updateAnnouncement,
} from "./console-api";

const LEVELS: { value: string; label: string; className: string }[] = [
  { value: "info", label: "普通", className: "bg-sky-500/10 text-sky-600 dark:text-sky-400" },
  { value: "warning", label: "警告", className: "bg-amber-500/10 text-amber-600 dark:text-amber-400" },
  { value: "critical", label: "紧急", className: "bg-red-500/10 text-red-600 dark:text-red-400" },
];

function levelMeta(level: string) {
  return LEVELS.find((l) => l.value === level) ?? LEVELS[0];
}

export function AnnouncementsPanel({ token }: { token: string }) {
  const [items, setItems] = useState<AnnouncementItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [level, setLevel] = useState("info");
  const [published, setPublished] = useState(true);
  const [submitting, setSubmitting] = useState(false);

  const load = useCallback(async () => {
    if (!token) return;
    setLoading(true);
    try {
      const res = await fetchAnnouncements(token, { page: 1, page_size: 20 });
      setItems(res.items ?? []);
    } catch {
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [token]);

  useEffect(() => {
    void load();
  }, [load]);

  const onCreate = async () => {
    if (!title.trim()) return;
    setSubmitting(true);
    try {
      await createAnnouncement(token, { title: title.trim(), content, level, published });
      setTitle("");
      setContent("");
      setLevel("info");
      setPublished(true);
      await load();
    } catch {
      // ignore
    } finally {
      setSubmitting(false);
    }
  };

  const onTogglePublish = async (a: AnnouncementItem) => {
    await updateAnnouncement(token, a.id, {
      title: a.title,
      content: a.content,
      level: a.level,
      published: !a.published,
    }).catch(() => {});
    await load();
  };

  const onDelete = async (id: number) => {
    await deleteAnnouncement(token, id).catch(() => {});
    await load();
  };

  return (
    <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div className="mb-3 flex items-center gap-2">
        <Megaphone className="size-4 text-primary" />
        <h3 className="text-sm font-semibold">站内公告</h3>
        <span className="text-xs text-muted-foreground">共 {items.length} 条</span>
      </div>

      {/* 新建表单 */}
      <div className="mb-4 space-y-2 rounded-lg border border-border/60 bg-muted/20 p-3">
        <Input placeholder="公告标题" value={title} onChange={(e) => setTitle(e.target.value)} className="h-8" />
        <textarea
          placeholder="公告内容"
          value={content}
          onChange={(e) => setContent(e.target.value)}
          rows={2}
          className="w-full rounded-lg border border-input bg-background px-2.5 py-1.5 text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring"
        />
        <div className="flex flex-wrap items-center gap-3">
          <select
            className="h-8 rounded-lg border border-input bg-background px-2 text-sm outline-none"
            value={level}
            onChange={(e) => setLevel(e.target.value)}
          >
            {LEVELS.map((l) => (
              <option key={l.value} value={l.value}>
                {l.label}
              </option>
            ))}
          </select>
          <label className="flex items-center gap-1.5 text-sm text-muted-foreground">
            <input type="checkbox" checked={published} onChange={(e) => setPublished(e.target.checked)} />
            立即发布
          </label>
          <Button size="sm" className="ml-auto" onClick={() => void onCreate()} disabled={submitting || !title.trim()}>
            {submitting ? <Loader2 className="size-4 animate-spin" /> : <Plus className="size-4" />}
            发布公告
          </Button>
        </div>
      </div>

      {/* 列表 */}
      {loading ? (
        <div className="py-8 text-center text-muted-foreground">
          <Loader2 className="mx-auto mb-2 size-5 animate-spin" />
          加载中
        </div>
      ) : items.length === 0 ? (
        <p className="py-8 text-center text-sm text-muted-foreground">暂无公告</p>
      ) : (
        <ul className="space-y-2">
          {items.map((a) => {
            const meta = levelMeta(a.level);
            return (
              <li key={a.id} className="rounded-lg border border-border p-3">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <span className={cn("rounded px-1.5 py-0.5 text-[11px] font-medium", meta.className)}>{meta.label}</span>
                      <span className="truncate font-medium">{a.title}</span>
                    </div>
                    {a.content && <p className="mt-1 line-clamp-2 text-sm text-muted-foreground">{a.content}</p>}
                    <p className="mt-1 text-[11px] text-muted-foreground">
                      {a.author_name || "—"} · {formatDateTime(a.created_at)}
                    </p>
                  </div>
                  <div className="flex shrink-0 items-center gap-2">
                    <Button size="sm" variant="outline" onClick={() => void onTogglePublish(a)}>
                      {a.published ? "已发布" : "草稿"}
                    </Button>
                    <Button size="sm" variant="ghost" onClick={() => void onDelete(a.id)}>
                      <Trash2 className="size-4 text-destructive" />
                    </Button>
                  </div>
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
