"use client";

import { ReportGallery } from "@/components/gopherpaper/report-gallery";
import { AppProvider } from "@/lib/gopherpaper/store";

// 小囊鼠研读报告画廊独立页:复用工作台 store(papers/报告进度/SSE),左论文右报告卡。
export default function ReportsPage() {
  return (
    <AppProvider>
      <ReportGallery />
    </AppProvider>
  );
}
