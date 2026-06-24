"use client";

import { useState } from "react";

import { cn } from "@/lib/utils";
import { PaperPane } from "./paper-pane";
import { RightPane } from "./right-pane";
import { Sidebar } from "./sidebar";

export function Workspace() {
  const [showSidebar, setShowSidebar] = useState(true);
  const toggle = () => setShowSidebar((v) => !v);
  return (
    <main className="flex h-dvh min-h-[640px] gap-2.5 overflow-hidden bg-muted/50 p-0 lg:p-2.5">
      <aside
        className={cn(
          "hidden overflow-hidden bg-background transition-[width] lg:block lg:rounded-xl lg:border lg:shadow-panel",
          showSidebar ? "w-72" : "w-0 border-0 shadow-none",
        )}
      >
        <Sidebar onCollapse={toggle} />
      </aside>
      <section className="flex min-w-0 flex-1 flex-col overflow-hidden bg-background lg:rounded-xl lg:border lg:shadow-panel">
        <div className="grid min-h-0 flex-1 grid-cols-1 gap-0 lg:grid-cols-[24rem_minmax(0,1fr)]">
          <PaperPane onExpandSidebar={showSidebar ? undefined : toggle} />
          <RightPane />
        </div>
      </section>
    </main>
  );
}
