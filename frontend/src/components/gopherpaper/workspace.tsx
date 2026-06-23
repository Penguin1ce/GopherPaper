"use client";

import { PanelLeftClose, PanelLeftOpen } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { PaperPane } from "./paper-pane";
import { RightPane } from "./right-pane";
import { Sidebar } from "./sidebar";

export function Workspace() {
  const [showSidebar, setShowSidebar] = useState(true);
  return (
    <main className="flex h-dvh min-h-[640px] gap-2.5 overflow-hidden bg-muted/50 p-0 lg:p-2.5">
      <aside
        className={cn(
          "hidden overflow-hidden bg-background transition-[width] lg:block lg:rounded-xl lg:border lg:shadow-panel",
          showSidebar ? "w-72" : "w-0 border-0 shadow-none",
        )}
      >
        <Sidebar />
      </aside>
      <section className="flex min-w-0 flex-1 flex-col overflow-hidden bg-background lg:rounded-xl lg:border lg:shadow-panel">
        <header className="flex h-16 shrink-0 items-center gap-3 border-b px-5">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={() => setShowSidebar((v) => !v)}
            className="hidden lg:inline-flex"
            title={showSidebar ? "收起侧栏" : "展开侧栏"}
          >
            {showSidebar ? <PanelLeftClose /> : <PanelLeftOpen />}
          </Button>
          <div className="flex items-center gap-2.5">
            <span className="flex size-8 items-center justify-center rounded-lg bg-primary text-base font-bold text-primary-foreground">
              G
            </span>
            <span className="text-base font-bold tracking-tight">GopherPaper</span>
          </div>
        </header>
        <div className="grid min-h-0 flex-1 grid-cols-1 gap-0 lg:grid-cols-[24rem_minmax(0,1fr)]">
          <PaperPane />
          <RightPane />
        </div>
      </section>
    </main>
  );
}
