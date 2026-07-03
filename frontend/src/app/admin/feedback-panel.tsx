"use client";

import { useCallback, useEffect, useState } from "react";
import { Inbox, Loader2, Send } from "lucide-react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { type FeedbackItem, fetchFeedbacks, formatDateTime, updateFeedback } from "./console-api";

const STATUS_META: Record<string, { label: string; className: string }> = {
  open: { label: "待处理", className: "bg-amber-500/10 text-amber-600 dark:text-amber-400" },
  resolved: { label: "已解决", className: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400" },
  closed: { label: "已关闭", className: "bg-muted text-muted-foreground" },
};

const CATEGORY_LABEL: Record<string, string> = {
  bug: "缺陷",
  feature: "需求",
  other: "其他",
};

export function FeedbackPanel({ token }: { token: string }) {
  const [items, setItems] = useState<FeedbackItem[]>([]);
  const [status, setStatus] = useState("");
  const [loading, setLoading] = useState(false);
  const [replyDraft, setReplyDraft] = useState<Record<number, string>>({});
  const [savingId, setSavingId] = useState<number | null>(null);

  const load = useCallback(async () => {
    if (!token) return;
    setLoading(true);
    try {
      const res = await fetchFeedbacks(token, { page: 1, page_size: 20, status });
      setItems(res.items ?? []);
    } catch {
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [status, token]);

  useEffect(() => {
    void load();
  }, [load]);

  const onResolve = async (f: FeedbackItem, nextStatus: string) => {
    setSavingId(f.id);
    try {
      await updateFeedback(token, f.id, { status: nextStatus, reply: replyDraft[f.id] ?? f.reply });
      await load();
    } catch {
      // ignore
    } finally {
      setSavingId(null);
    }
  };

  return (
    <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div className="mb-3 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Inbox className="size-4 text-primary" />
          <h3 className="text-sm font-semibold">用户反馈</h3>
          <span className="text-xs text-muted-foreground">共 {items.length} 条</span>
        </div>
        <select
          className="h-8 rounded-lg border border-input bg-background px-2 text-sm outline-none"
          value={status}
          onChange={(e) => setStatus(e.target.value)}
        >
          <option value="">全部状态</option>
          <option value="open">待处理</option>
          <option value="resolved">已解决</option>
          <option value="closed">已关闭</option>
        </select>
      </div>

      {loading ? (
        <div className="py-8 text-center text-muted-foreground">
          <Loader2 className="mx-auto mb-2 size-5 animate-spin" />
          加载中
        </div>
      ) : items.length === 0 ? (
        <p className="py-8 text-center text-sm text-muted-foreground">暂无反馈</p>
      ) : (
        <ul className="space-y-3">
          {items.map((f) => {
            const meta = STATUS_META[f.status] ?? STATUS_META.open;
            return (
              <li key={f.id} className="rounded-lg border border-border p-3">
                <div className="flex items-center gap-2">
                  <span className={cn("rounded px-1.5 py-0.5 text-[11px] font-medium", meta.className)}>{meta.label}</span>
                  <span className="rounded bg-muted px-1.5 py-0.5 text-[11px]">{CATEGORY_LABEL[f.category] ?? f.category}</span>
                  <span className="text-xs text-muted-foreground">{f.student_id}</span>
                  <span className="ml-auto text-[11px] text-muted-foreground">{formatDateTime(f.created_at)}</span>
                </div>
                <p className="mt-2 text-sm">{f.content}</p>
                {f.reply && (
                  <div className="mt-2 rounded-lg bg-muted/40 p-2 text-sm">
                    <span className="text-xs text-muted-foreground">回复({f.handler_name}):</span> {f.reply}
                  </div>
                )}
                <div className="mt-2 flex items-center gap-2">
                  <input
                    className="h-8 flex-1 rounded-lg border border-input bg-background px-2.5 text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    placeholder="回复内容..."
                    value={replyDraft[f.id] ?? ""}
                    onChange={(e) => setReplyDraft((v) => ({ ...v, [f.id]: e.target.value }))}
                  />
                  <Button size="sm" variant="outline" disabled={savingId === f.id} onClick={() => void onResolve(f, "resolved")}>
                    {savingId === f.id ? <Loader2 className="size-4 animate-spin" /> : <Send className="size-4" />}
                    回复并解决
                  </Button>
                  {f.status !== "closed" && (
                    <Button size="sm" variant="ghost" disabled={savingId === f.id} onClick={() => void onResolve(f, "closed")}>
                      关闭
                    </Button>
                  )}
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
