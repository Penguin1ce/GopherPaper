"use client";

import type { ElementType, ReactNode } from "react";

import { cn } from "@/lib/utils";

export function WorkspaceFrame({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <main
      className={cn(
        "flex h-dvh min-h-[640px] gap-2.5 overflow-hidden bg-muted/50 p-0 lg:p-2.5",
        className,
      )}
    >
      {children}
    </main>
  );
}

export function WorkspacePanel({
  as: Component = "section",
  children,
  className,
}: {
  as?: ElementType;
  children: ReactNode;
  className?: string;
}) {
  return (
    <Component
      className={cn(
        "min-h-0 overflow-hidden bg-background lg:rounded-xl lg:border lg:shadow-panel",
        className,
      )}
    >
      {children}
    </Component>
  );
}
