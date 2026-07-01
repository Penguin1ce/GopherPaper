"use client";

import type { CSSProperties, ReactNode } from "react";
import { useRef } from "react";

import { cn } from "@/lib/utils";

type SpotlightCardProps = {
  children: ReactNode;
  className?: string;
  spotlightColor?: string;
};

type SpotlightStyle = CSSProperties & {
  "--spotlight-x": string;
  "--spotlight-y": string;
  "--spotlight-opacity": number;
  "--spotlight-color": string;
};

export function SpotlightCard({
  children,
  className,
  spotlightColor = "rgba(30, 141, 150, 0.22)",
}: SpotlightCardProps) {
  const ref = useRef<HTMLDivElement>(null);

  const setSpotlight = (opacity: number) => {
    const el = ref.current;
    if (!el) return;
    el.style.setProperty("--spotlight-opacity", String(opacity));
  };

  return (
    <div
      ref={ref}
      onPointerMove={(event) => {
        const rect = event.currentTarget.getBoundingClientRect();
        event.currentTarget.style.setProperty("--spotlight-x", `${event.clientX - rect.left}px`);
        event.currentTarget.style.setProperty("--spotlight-y", `${event.clientY - rect.top}px`);
      }}
      onPointerEnter={() => setSpotlight(1)}
      onPointerLeave={() => setSpotlight(0)}
      onFocus={() => setSpotlight(1)}
      onBlur={() => setSpotlight(0)}
      className={cn(
        "group relative overflow-hidden rounded-xl border border-white/40 bg-white/[0.42] shadow-[0_18px_50px_-30px_oklch(0.24_0.02_220/0.45)] outline-none backdrop-blur-xl transition-[border-color,box-shadow,transform] duration-300 focus-within:ring-2 focus-within:ring-ring/60 dark:border-white/10 dark:bg-white/[0.055]",
        "before:pointer-events-none before:absolute before:inset-0 before:opacity-[var(--spotlight-opacity)] before:transition-opacity before:duration-500 before:[background:radial-gradient(360px_circle_at_var(--spotlight-x)_var(--spotlight-y),var(--spotlight-color),transparent_68%)]",
        className,
      )}
      style={
        {
          "--spotlight-x": "50%",
          "--spotlight-y": "50%",
          "--spotlight-opacity": 0,
          "--spotlight-color": spotlightColor,
        } as SpotlightStyle
      }
    >
      <div className="relative z-10">{children}</div>
    </div>
  );
}
