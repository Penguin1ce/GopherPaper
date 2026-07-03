"use client";

import { useCallback, useEffect, useState } from "react";
import { Check, Loader2, Pencil, Tag, Trash2, X } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { type TagItem, deleteTag, fetchTags, renameTag } from "./console-api";

export function TagsPanel({ token }: { token: string }) {
  const [items, setItems] = useState<TagItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [draft, setDraft] = useState("");

  const load = useCallback(async () => {
    if (!token) return;
    setLoading(true);
    try {
      const res = await fetchTags(token);
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

  const onRename = async (id: number) => {
    if (!draft.trim()) return;
    await renameTag(token, id, draft.trim()).catch(() => {});
    setEditingId(null);
    setDraft("");
    await load();
  };

  const onDelete = async (id: number) => {
    await deleteTag(token, id).catch(() => {});
    await load();
  };

  return (
    <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div className="mb-3 flex items-center gap-2">
        <Tag className="size-4 text-primary" />
        <h3 className="text-sm font-semibold">标签管理</h3>
        <span className="text-xs text-muted-foreground">共 {items.length} 个</span>
      </div>

      {loading ? (
        <div className="py-8 text-center text-muted-foreground">
          <Loader2 className="mx-auto mb-2 size-5 animate-spin" />
          加载中
        </div>
      ) : items.length === 0 ? (
        <p className="py-8 text-center text-sm text-muted-foreground">暂无标签</p>
      ) : (
        <div className="flex flex-wrap gap-2">
          {items.map((t) => (
            <div
              key={t.id}
              className="flex items-center gap-2 rounded-full border border-border bg-muted/30 py-1 pl-3 pr-1.5 text-sm"
            >
              {editingId === t.id ? (
                <>
                  <Input
                    className="h-6 w-24 px-1.5 text-xs"
                    value={draft}
                    autoFocus
                    onChange={(e) => setDraft(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") void onRename(t.id);
                      if (e.key === "Escape") setEditingId(null);
                    }}
                  />
                  <button type="button" onClick={() => void onRename(t.id)} className="text-emerald-600">
                    <Check className="size-3.5" />
                  </button>
                  <button type="button" onClick={() => setEditingId(null)} className="text-muted-foreground">
                    <X className="size-3.5" />
                  </button>
                </>
              ) : (
                <>
                  <span>{t.name}</span>
                  <span className="rounded-full bg-background px-1.5 text-[11px] text-muted-foreground">
                    {t.paper_count}
                  </span>
                  <button
                    type="button"
                    onClick={() => {
                      setEditingId(t.id);
                      setDraft(t.name);
                    }}
                    className="text-muted-foreground hover:text-foreground"
                  >
                    <Pencil className="size-3" />
                  </button>
                  <button type="button" onClick={() => void onDelete(t.id)} className="text-muted-foreground hover:text-destructive">
                    <Trash2 className="size-3" />
                  </button>
                </>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
