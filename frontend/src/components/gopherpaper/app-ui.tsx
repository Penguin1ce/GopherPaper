"use client";

import { useEffect, type ReactNode } from "react";
import { toast as sonnerToast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton as ShadcnSkeleton } from "@/components/ui/skeleton";
import { Toaster } from "@/components/ui/sonner";
import { cn } from "@/lib/utils";
import { useApp } from "@/lib/gopherpaper/store";
import type { PaperStatus } from "@/lib/gopherpaper/types";
import { statusLabel, statusTone } from "@/lib/gopherpaper/utils";

export function StatusBadge({ status }: { status?: PaperStatus }) {
  const tone = statusTone(status);
  return (
    <Badge
      variant={tone === "failed" ? "destructive" : tone === "ready" ? "default" : "secondary"}
      className={cn("gap-1.5 rounded-full px-2 py-0.5 font-normal", {
        "bg-muted text-muted-foreground": tone === "working",
      })}
    >
      <span
        className={cn("size-1.5 rounded-full bg-current", tone === "working" && "animate-pulse")}
        aria-hidden
      />
      {statusLabel(status)}
    </Badge>
  );
}

export function Empty({
  title,
  text,
  compact = false,
}: {
  title: string;
  text: string;
  compact?: boolean;
}) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center rounded-lg border border-dashed bg-muted/30 p-8 text-center",
        compact && "p-4",
      )}
    >
      <h3 className="text-sm font-medium">{title}</h3>
      <p className="mt-1 max-w-sm text-sm text-muted-foreground">{text}</p>
    </div>
  );
}

export function SkeletonLines({ lines = 3 }: { lines?: number }) {
  return (
    <div className="space-y-2">
      {Array.from({ length: lines }).map((_, i) => (
        <ShadcnSkeleton
          key={i}
          className="h-4"
          style={{ width: `${88 - i * 10}%` }}
        />
      ))}
    </div>
  );
}

export function AppToaster() {
  return <Toaster richColors closeButton position="top-center" />;
}

export function ToastBridge() {
  const { toasts, dismissToast } = useApp();
  useEffect(() => {
    for (const item of toasts) {
      const fn = item.type === "error" ? sonnerToast.error : sonnerToast.success;
      fn(item.message, { id: item.id, duration: 3200 });
      dismissToast(item.id);
    }
  }, [toasts, dismissToast]);
  return null;
}

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

export function SectionTitle({
  title,
  description,
  action,
}: {
  title: string;
  description?: string;
  action?: ReactNode;
}) {
  return (
    <div className="flex items-start justify-between gap-3">
      <div className="min-w-0">
        <h2 className="text-base font-semibold tracking-tight">{title}</h2>
        {description && <p className="mt-1 text-sm text-muted-foreground">{description}</p>}
      </div>
      {action}
    </div>
  );
}

export function IconButton({
  children,
  label,
  onClick,
  disabled,
}: {
  children: ReactNode;
  label: string;
  onClick?: () => void;
  disabled?: boolean;
}) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      aria-label={label}
      title={label}
      onClick={onClick}
      disabled={disabled}
    >
      {children}
    </Button>
  );
}
