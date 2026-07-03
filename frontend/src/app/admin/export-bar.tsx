"use client";

import { useState } from "react";
import { Download, Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { downloadExport } from "./console-api";

const KINDS: { kind: "papers" | "users" | "logs"; label: string }[] = [
  { kind: "papers", label: "导出论文" },
  { kind: "users", label: "导出用户" },
  { kind: "logs", label: "导出日志" },
];

export function ExportBar({ token }: { token: string }) {
  const [busy, setBusy] = useState<string | null>(null);

  const onExport = async (kind: "papers" | "users" | "logs") => {
    setBusy(kind);
    try {
      await downloadExport(token, kind);
    } catch {
      // 静默失败,按钮恢复即可
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="flex flex-wrap items-center gap-2 rounded-xl border border-border bg-card p-4 shadow-sm">
      <span className="flex items-center gap-2 text-sm font-semibold">
        <Download className="size-4 text-primary" />
        数据导出
      </span>
      <span className="mr-2 text-xs text-muted-foreground">导出为 CSV(带 UTF-8 BOM,Excel 直接打开)</span>
      {KINDS.map((k) => (
        <Button
          key={k.kind}
          size="sm"
          variant="outline"
          disabled={busy !== null}
          onClick={() => void onExport(k.kind)}
        >
          {busy === k.kind ? <Loader2 className="size-4 animate-spin" /> : <Download className="size-4" />}
          {k.label}
        </Button>
      ))}
    </div>
  );
}
