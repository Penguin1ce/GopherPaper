"use client";

import { MessageSquareText, PanelLeftOpen } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useApp } from "@/lib/gopherpaper/store";
import { paperTitle } from "@/lib/gopherpaper/utils";
import { ChatPane } from "./chat-pane";

export function RightPane({ onExpandSidebar }: { onExpandSidebar?: () => void }) {
  const { activePaper } = useApp();
  const [tab, setTab] = useState("chat");
  return (
    <section className="flex min-h-0 flex-1 flex-col bg-background">
      <Tabs value={tab} onValueChange={setTab} className="flex min-h-0 flex-1 flex-col">
        <div className="flex h-16 shrink-0 items-center justify-between gap-4 border-b px-5">
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
          <TabsList>
            <TabsTrigger value="chat" className="gap-1.5">
              <MessageSquareText className="size-3.5" />
              问答
            </TabsTrigger>
          </TabsList>
        </div>
        <TabsContent value="chat" className="m-0 min-h-0 flex-1">
          <ChatPane />
        </TabsContent>
      </Tabs>
    </section>
  );
}
