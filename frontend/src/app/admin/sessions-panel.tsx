"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Loader2, MessagesSquare, Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { type SessionItem, deleteSession, fetchSessions, formatDateTime } from "./console-api";

const PAGE_SIZE = 15;

const AGENT_LABEL: Record<string, string> = {
  "": "小文鸮",
  pioneer: "小云雀",
  maodie: "小耄耋",
};

export function SessionsPanel({ token }: { token: string }) {
  const [items, setItems] = useState<SessionItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [agentType, setAgentType] = useState("");
  const [loading, setLoading] = useState(false);

  const load = useCallback(
    async (nextPage = page) => {
      if (!token) return;
      setLoading(true);
      try {
        const res = await fetchSessions(token, { page: nextPage, page_size: PAGE_SIZE, agent_type: agentType });
        setItems(res.items ?? []);
        setTotal(res.total ?? 0);
        setPage(res.page ?? nextPage);
      } catch {
        setItems([]);
      } finally {
        setLoading(false);
      }
    },
    [agentType, page, token],
  );

  useEffect(() => {
    void load(1);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token, agentType]);

  const pageCount = useMemo(() => Math.max(1, Math.ceil(total / PAGE_SIZE)), [total]);

  const onDelete = async (id: string) => {
    await deleteSession(token, id).catch(() => {});
    await load(page);
  };

  return (
    <div className="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div className="mb-3 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <MessagesSquare className="size-4 text-primary" />
          <h3 className="text-sm font-semibold">会话管理</h3>
          <span className="text-xs text-muted-foreground">共 {total} 个</span>
        </div>
        <select
          className="h-8 rounded-lg border border-input bg-background px-2 text-sm outline-none"
          value={agentType}
          onChange={(e) => setAgentType(e.target.value)}
        >
          <option value="">全部智能体</option>
          <option value="pioneer">小云雀</option>
          <option value="maodie">小耄耋</option>
        </select>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full min-w-[40rem] text-left text-sm">
          <thead className="bg-muted/40 text-xs uppercase text-muted-foreground">
            <tr>
              <th className="px-3 py-2 font-medium">标题</th>
              <th className="px-3 py-2 font-medium">用户</th>
              <th className="px-3 py-2 font-medium">智能体</th>
              <th className="px-3 py-2 font-medium">时间</th>
              <th className="px-3 py-2 text-right font-medium">操作</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={5} className="px-3 py-10 text-center text-muted-foreground">
                  <Loader2 className="mx-auto mb-2 size-5 animate-spin" />
                  加载中
                </td>
              </tr>
            ) : items.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-3 py-10 text-center text-muted-foreground">
                  暂无会话
                </td>
              </tr>
            ) : (
              items.map((s) => (
                <tr key={s.id} className="border-t border-border">
                  <td className="max-w-[18rem] px-3 py-2.5">
                    <p className="truncate">{s.title || "(未命名会话)"}</p>
                    <p className="truncate font-mono text-[11px] text-muted-foreground">{s.id}</p>
                  </td>
                  <td className="px-3 py-2.5 text-muted-foreground">{s.student_id}</td>
                  <td className="px-3 py-2.5">
                    <span className="rounded bg-muted px-2 py-0.5 text-xs">
                      {AGENT_LABEL[s.agent_type ?? ""] ?? s.agent_type}
                    </span>
                  </td>
                  <td className="px-3 py-2.5 text-xs text-muted-foreground">{formatDateTime(s.created_at)}</td>
                  <td className="px-3 py-2.5 text-right">
                    <Button size="sm" variant="ghost" onClick={() => void onDelete(s.id)}>
                      <Trash2 className="size-4 text-destructive" />
                    </Button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      <div className="mt-3 flex items-center justify-between text-sm text-muted-foreground">
        <span>
          第 {page} / {pageCount} 页
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
