"use client";

import type { CSSProperties, ReactNode } from "react";

import { cn } from "@/lib/utils";

type GlassSurfaceProps = {
  children: ReactNode;
  className?: string;
  contentClassName?: string;
  width?: number | string;
  height?: number | string;
};

export function GlassSurface({
  children,
  className,
  contentClassName,
  width = "auto",
  height = "auto",
}: GlassSurfaceProps) {
  return (
    <div
      className={cn(
        "relative overflow-hidden rounded-2xl border border-white/50 bg-white/[0.35] shadow-[0_22px_70px_-38px_oklch(0.2_0.02_220/0.55)] backdrop-blur-2xl",
        "before:pointer-events-none before:absolute before:inset-0 before:bg-[linear-gradient(135deg,rgba(255,255,255,0.65),rgba(255,255,255,0.12)_42%,rgba(255,255,255,0.38))]",
        "after:pointer-events-none after:absolute after:inset-px after:rounded-[inherit] after:shadow-[inset_0_1px_0_rgba(255,255,255,0.62),inset_0_-1px_0_rgba(255,255,255,0.18)]",
        "dark:border-white/10 dark:bg-white/[0.055] dark:before:bg-[linear-gradient(135deg,rgba(255,255,255,0.14),rgba(255,255,255,0.035)_45%,rgba(255,255,255,0.10))] dark:after:shadow-[inset_0_1px_0_rgba(255,255,255,0.16),inset_0_-1px_0_rgba(255,255,255,0.06)]",
        className,
      )}
      style={{ width, height } as CSSProperties}
    >
      <div className={cn("relative z-10 h-full w-full", contentClassName)}>{children}</div>
    </div>
  );
}
