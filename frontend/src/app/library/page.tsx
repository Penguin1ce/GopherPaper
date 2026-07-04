"use client";

import { PaperLibrary } from "@/components/gopherpaper/paper-library";

// 论文库独立页:复用工作台 store,左侧管理栏 + 右侧论文卡片网格。
export default function LibraryPage() {
  return <PaperLibrary />;
}
