"use client";

import { usePathname } from "next/navigation";
import type { ReactNode } from "react";

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

function usesSharedAppState(pathname: string) {
  return SHARED_APP_PATHS.has(pathname) || pathname.startsWith("/profile/");
}

export function AppRouteProvider({ children }: { children: ReactNode }) {
  const pathname = usePathname();

  if (!usesSharedAppState(pathname)) return children;

  return (
    <AppProvider>
      {children}
      <ToastBridge />
      <AppToaster />
    </AppProvider>
  );
}
