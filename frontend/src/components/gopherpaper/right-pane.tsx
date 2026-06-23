"use client";

import { FileText, MessageSquareText, NotebookText } from "lucide-react";
import { useState } from "react";

import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useApp } from "@/lib/gopherpaper/store";
import { paperTitle } from "@/lib/gopherpaper/utils";
import { ChatPane } from "./chat-pane";
import { PaperDetail } from "./paper-detail";
import { ReportPanel } from "./report-panel";

export function RightPane() {
  const { activePaper } = useApp();
  const [tab, setTab] = useState("chat");
  return (
    <section className="flex min-h-0 flex-col bg-background">
      <Tabs value={tab} onValueChange={setTab} className="flex min-h-0 flex-1 flex-col">
        <div className="flex h-16 shrink-0 items-center justify-between gap-4 border-b px-5">
          <div className="min-w-0 truncate text-[15px] font-semibold tracking-tight">
            {activePaper ? paperTitle(activePaper) : "未选择论文"}
          </div>
          <TabsList>
            <TabsTrigger value="chat" className="gap-1.5">
              <MessageSquareText className="size-3.5" />
              问答
            </TabsTrigger>
            <TabsTrigger value="detail" className="gap-1.5">
              <FileText className="size-3.5" />
              详情
            </TabsTrigger>
            <TabsTrigger value="report" className="gap-1.5">
              <NotebookText className="size-3.5" />
              报告
            </TabsTrigger>
          </TabsList>
        </div>
        <TabsContent value="chat" className="m-0 min-h-0 flex-1">
          <ChatPane />
        </TabsContent>
        <TabsContent value="detail" className="m-0 min-h-0 flex-1">
          <PaperDetail />
        </TabsContent>
        <TabsContent value="report" className="m-0 min-h-0 flex-1">
          <ReportPanel />
        </TabsContent>
      </Tabs>
    </section>
  );
}
