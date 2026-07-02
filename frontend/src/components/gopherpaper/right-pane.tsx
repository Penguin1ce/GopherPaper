"use client";

import { BookOpenText, PanelLeftOpen, Plus } from "lucide-react";

import { Button } from "@/components/ui/button";
import { useApp } from "@/lib/gopherpaper/store";
import { paperTitle } from "@/lib/gopherpaper/utils";
import { useGuard } from "./app-ui";
import { ChatPane } from "./chat-pane";

export function RightPane({ onExpandSidebar }: { onExpandSidebar?: () => void }) {
  const { activePaper, activePaperID, createSession } = useApp();
  const guard = useGuard();
  const openReader = () => {
    if (!activePaper) return;
    window.open(`/reader?id=${encodeURIComponent(activePaper.id)}`, "_blank", "noopener,noreferrer");
  };
  const createNewSession = () => {
    guard(async () => {
      await createSession("新会话", activePaperID || undefined);
    });
  };

  return (
    <section className="flex min-h-0 flex-1 flex-col bg-background">
      <div className="flex h-14 shrink-0 items-center justify-between gap-4 border-b px-5">
        <div className="flex min-w-0 items-center gap-2">
          {onExpandSidebar && (
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              onClick={onExpandSidebar}
              title="展开侧栏"
              className="hidden shrink-0 lg:inline-flex"
            >
              <PanelLeftOpen className="size-4" />
            </Button>
          )}
          <div className="min-w-0 truncate text-[15px] font-semibold tracking-tight">
            {activePaper ? paperTitle(activePaper) : "未选择论文"}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="h-8 gap-1.5 px-3 text-sm"
            disabled={!activePaper}
            title={activePaper ? "打开精读页" : "先选择论文"}
            onClick={openReader}
          >
            <BookOpenText className="size-3.5" />
            精读
          </Button>
          <Button
            type="button"
            size="sm"
            className="h-8 gap-1.5 px-3 text-sm"
            onClick={createNewSession}
            title="新建对话"
          >
            <Plus className="size-3.5" />
            新建对话
          </Button>
        </div>
      </div>
      <div className="min-h-0 flex-1">
        <ChatPane />
      </div>
    </section>
  );
}
