import { useState } from "react";

import { useApp } from "../store";
import { paperTitle } from "../utils";
import { ChatPane } from "./ChatPane";
import { PaperDetail } from "./PaperDetail";
import { ReportPanel } from "./ReportPanel";

type Tab = "chat" | "detail" | "report";

const TABS: { key: Tab; label: string }[] = [
  { key: "chat", label: "问答" },
  { key: "detail", label: "论文详情" },
  { key: "report", label: "研读报告" },
];

// 初始 tab 可经 ?tab= 深链指定,便于分享到具体视图。
function initialTab(): Tab {
  const t = new URLSearchParams(location.search).get("tab");
  return t === "detail" || t === "report" ? t : "chat";
}

export function RightPane() {
  const { activePaper, activeSession } = useApp();
  const [tab, setTab] = useState<Tab>(initialTab);

  const subtitle = activePaper
    ? paperTitle(activePaper)
    : activeSession
      ? activeSession.title
      : "尚未选择论文";

  return (
    <section className="chat-pane">
      <header className="pane-tabs-header">
        <div className="pane-title">
          <p className="eyebrow">Workbench</p>
          <h2>{subtitle}</h2>
        </div>
        <div className="pane-tabs" role="tablist">
          {TABS.map((t) => (
            <button
              key={t.key}
              type="button"
              role="tab"
              aria-selected={tab === t.key}
              className={`pane-tab${tab === t.key ? " active" : ""}`}
              onClick={() => setTab(t.key)}
            >
              {t.label}
            </button>
          ))}
        </div>
      </header>

      <div className="pane-body">
        {tab === "chat" && <ChatPane />}
        {tab === "detail" && <PaperDetail />}
        {tab === "report" && <ReportPanel />}
      </div>
    </section>
  );
}
