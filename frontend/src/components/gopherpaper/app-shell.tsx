"use client";

import { AuthView } from "./auth-view";
import { AppToaster, ToastBridge } from "./app-ui";
import { Workspace } from "./workspace";
import { AppProvider, useApp } from "@/lib/gopherpaper/store";

function AppContent() {
  const { authed } = useApp();
  return (
    <>
      {authed ? <Workspace /> : <AuthView />}
      <ToastBridge />
      <AppToaster />
    </>
  );
}

export function GopherPaperApp() {
  return (
    <AppProvider>
      <AppContent />
    </AppProvider>
  );
}
