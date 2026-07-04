"use client";

import { useEffect, useRef, useState } from "react";

import { Separator } from "@/components/ui/separator";
import { useApp } from "@/lib/gopherpaper/store";
import { cn } from "@/lib/utils";
import { PaperPane } from "./paper-pane";
import { RightPane } from "./right-pane";
import { Sidebar, SidebarUserCard } from "./sidebar";
import { WorkspaceFrame, WorkspacePanel } from "./workspace-frame";

export function Workspace() {
  const { papers, activePaperID, selectPaper } = useApp();
  const [showSidebar, setShowSidebar] = useState(true);
  const routedPaperRef = useRef("");
  const toggle = () => setShowSidebar((v) => !v);

  useEffect(() => {
    const paperID = new URLSearchParams(window.location.search)
      .get("paper_id")
      ?.trim();
    if (
      !paperID ||
      routedPaperRef.current === paperID ||
      activePaperID === paperID ||
      !papers.some((paper) => paper.id === paperID)
    ) {
      return;
    }
    routedPaperRef.current = paperID;
    selectPaper(paperID);
  }, [activePaperID, papers, selectPaper]);

  return (
    <WorkspaceFrame>
      <WorkspacePanel
        as="aside"
        className={cn(
          "hidden transition-[width] lg:flex lg:flex-col",
          showSidebar ? "w-96" : "w-0 border-0 shadow-none",
        )}
      >
        <Sidebar />
        <Separator />
        <PaperPane />
        <SidebarUserCard onCollapse={toggle} />
      </WorkspacePanel>
      <WorkspacePanel className="flex min-w-0 flex-1 flex-col">
        <RightPane onExpandSidebar={showSidebar ? undefined : toggle} />
      </WorkspacePanel>
    </WorkspaceFrame>
  );
}
