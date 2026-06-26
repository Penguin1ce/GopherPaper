"use client";

import { AuthView } from "./auth-view";
import { Workspace } from "./workspace";
import { useApp } from "@/lib/gopherpaper/store";

function AppContent() {
  const { authed } = useApp();
  return (
    <>
      {authed ? <Workspace /> : <AuthView />}
    </>
  );
}

export function GopherPaperApp() {
  return <AppContent />;
}
