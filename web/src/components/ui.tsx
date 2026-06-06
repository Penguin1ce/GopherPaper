import type { ReactNode } from "react";

import { useApp } from "../store";
import { statusLabel, statusTone } from "../utils";
import type { PaperStatus } from "../types";

// 状态徽标:可提问 / 处理中 / 失败 三态配色。
export function StatusBadge({ status }: { status?: PaperStatus }) {
  const tone = statusTone(status);
  return (
    <span className={`status-badge ${tone}`}>
      <span className="status-dot" aria-hidden />
      {statusLabel(status)}
    </span>
  );
}

export function Chip({
  children,
  tone = "neutral",
}: {
  children: ReactNode;
  tone?: "neutral" | "accent" | "info";
}) {
  return <span className={`chip ${tone}`}>{children}</span>;
}

export function Empty({
  title,
  text,
  inline = false,
}: {
  title: string;
  text: string;
  inline?: boolean;
}) {
  return (
    <div className={`empty-state${inline ? " inline" : ""}`}>
      <h3>{title}</h3>
      <p>{text}</p>
    </div>
  );
}

// 三行骨架占位,用于加载态。
export function Skeleton({ lines = 3 }: { lines?: number }) {
  return (
    <div className="skeleton" aria-hidden>
      {Array.from({ length: lines }).map((_, i) => (
        <span key={i} style={{ width: `${88 - i * 12}%` }} />
      ))}
    </div>
  );
}

export function ToastStack() {
  const { toasts, dismissToast } = useApp();
  return (
    <div className="toast-stack" aria-live="polite">
      {toasts.map((t) => (
        <button
          key={t.id}
          type="button"
          className={`toast ${t.type}`}
          onClick={() => dismissToast(t.id)}
        >
          {t.message}
        </button>
      ))}
    </div>
  );
}

// 包一层 try/catch 的异步动作,失败弹 toast。
export function useGuard() {
  const { toast } = useApp();
  return async (fn: () => Promise<void>) => {
    try {
      await fn();
    } catch (err) {
      toast((err as Error)?.message || "操作失败", "error");
    }
  };
}
