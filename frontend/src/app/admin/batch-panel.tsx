"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Loader2, ListChecks, Search, Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { type BatchPaper, batchDeletePapers, fetchPapersForBatch } from "./console-api";

const PAGE_SIZE = 20;

export function BatchPanel({ token, onChanged }: { token: string; onChanged?: () => void }) {
  const [items, setItems] = useState<BatchPaper[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);

  const load = useCallback(
    async (nextPage = page) => {
      if (!token) return;
      setLoading(true);
      try {
        const res = await fetchPapersForBatch(token, { page: nextPage, page_size: PAGE_SIZE, query });
        setItems(res.items ?? []);
        setTotal(res.total ?? 0);
        setPage(res.page ?? nextPage);
      } catch {
        setItems([]);
      } finally {
        setLoading(false);
      }
    },
    [page, query, token],
  );

  useEffect(() => {
    void load(1);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token]);

  const pageCount = useMemo(() => Math.max(1, Math.ceil(total / PAGE_SIZE)), [total]);
  const allChecked = items.length > 0 && items.every((p) => selected.has(p.id));

  const toggle = (id: string) => {
    setSelected((s) => {
      const next = new Set(s);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const toggleAll = () => {
    setSelected((s) => {
      const next = new Set(s);
      if (allChecked) items.forEach((p) => next.delete(p.id));
      else items.forEach((p) => next.add(p.id));
      return next;
    });
  };

  const onBatchDelete = async () => {
    if (selected.size === 0) return;
    setDeleting(true);
    setNotice(null);
    try {
      const res = await batchDeletePapers(token, Array.from(selected));
      setNotice(`已删除 ${res.succeeded} 篇,失败 ${res.failed} 篇`);
      setSelected(new Set());
      await load(1);
      onChanged?.();
    } catch {
      setNotice("批量删除失败");
    } finally {
      setDeleting(false);
    }
  };

  return (
    <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <ListChecks className="size-4 text-primary" />
          <h3 className="text-sm font-semibold">论文批量操作</h3>
          <span className="text-xs text-muted-foreground">已选 {selected.size} 篇</span>
        </div>
        <div className="flex items-center gap-2">
          <label className="relative">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              className="h-8 w-44 pl-8"
              placeholder="搜索论文"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") void load(1);
              }}
            />
          </label>
          <Button
            size="sm"
            variant="destructive"
            disabled={selected.size === 0 || deleting}
            onClick={() => void onBatchDelete()}
          >
            {deleting ? <Loader2 className="size-4 animate-spin" /> : <Trash2 className="size-4" />}
            批量删除
          </Button>
        </div>
      </div>

      {notice && <p className="mb-2 rounded-lg bg-muted/50 px-3 py-1.5 text-xs text-muted-foreground">{notice}</p>}

      <div className="overflow-x-auto">
        <table className="w-full min-w-[40rem] text-left text-sm">
          <thead className="bg-muted/40 text-xs uppercase text-muted-foreground">
            <tr>
              <th className="px-3 py-2">
                <input type="checkbox" checked={allChecked} onChange={toggleAll} />
              </th>
              <th className="px-3 py-2 font-medium">论文</th>
              <th className="px-3 py-2 font-medium">Owner</th>
              <th className="px-3 py-2 font-medium">状态</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={4} className="px-3 py-10 text-center text-muted-foreground">
                  <Loader2 className="mx-auto mb-2 size-5 animate-spin" />
                  加载中
                </td>
              </tr>
            ) : items.length === 0 ? (
              <tr>
                <td colSpan={4} className="px-3 py-10 text-center text-muted-foreground">
                  暂无论文
                </td>
              </tr>
            ) : (
              items.map((p) => (
                <tr key={p.id} className={cn("border-t border-border", selected.has(p.id) && "bg-primary/5")}>
                  <td className="px-3 py-2.5">
                    <input type="checkbox" checked={selected.has(p.id)} onChange={() => toggle(p.id)} />
                  </td>
                  <td className="max-w-[22rem] px-3 py-2.5">
                    <p className="truncate">{p.title || p.file_name}</p>
                    <p className="truncate font-mono text-[11px] text-muted-foreground">{p.id}</p>
                  </td>
                  <td className="px-3 py-2.5 text-muted-foreground">{p.owner_id}</td>
                  <td className="px-3 py-2.5">
                    <span className="rounded bg-muted px-2 py-0.5 text-xs">{p.status}</span>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      <div className="mt-3 flex items-center justify-between text-sm text-muted-foreground">
        <span>
          第 {page} / {pageCount} 页 · 共 {total} 篇
        </span>
        <div className="flex gap-2">
          <Button size="sm" variant="outline" disabled={page <= 1 || loading} onClick={() => void load(page - 1)}>
            上一页
          </Button>
          <Button size="sm" variant="outline" disabled={page >= pageCount || loading} onClick={() => void load(page + 1)}>
            下一页
          </Button>
        </div>
      </div>
    </div>
  );
}
