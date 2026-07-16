"use client";

import { usePathname } from "next/navigation";
import { useEffect, type ReactNode } from "react";

import { AppProvider } from "@/lib/gopherpaper/store";
import { AppToaster, ToastBridge } from "./app-ui";

const SHARED_APP_PATHS = new Set([
  "/",
  "/library",
  "/reader",
  "/reports",
  "/graph",
  "/profile",
]);

export function AppRouteProvider({ children }: { children: ReactNode }) {
  const pathname = usePathname();

  useEffect(() => {
    // 清理旧版本遗留的明文 JWT；当前会话只由 HttpOnly Cookie 承载。
    window.localStorage.removeItem("gopherpaper.auth");
  }, []);

  if (!SHARED_APP_PATHS.has(pathname)) return children;

  return (
    <AppProvider>
      {children}
      <ToastBridge />
      <AppToaster />
    </AppProvider>
  );
}
