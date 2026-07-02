"use client";

import { usePathname } from "next/navigation";
import type { ReactNode } from "react";

import { AppProvider } from "@/lib/gopherpaper/store";
import { AppToaster, ToastBridge } from "./app-ui";

const SHARED_APP_PATHS = new Set(["/", "/reader", "/reports", "/graph", "/profile"]);

export function AppRouteProvider({ children }: { children: ReactNode }) {
  const pathname = usePathname();

  if (!SHARED_APP_PATHS.has(pathname)) return children;

  return (
    <AppProvider>
      {children}
      <ToastBridge />
      <AppToaster />
    </AppProvider>
  );
}
